package bring

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// DefaultBaseURL is the Bring! REST API (MIRROR_BRING_URL overrides it).
const DefaultBaseURL = "https://api.getbring.com/rest/v2/"

// apiKey is the public key of Bring!'s own web app. Open-source clients use it
// as well; it identifies the client program, not the user, and is no secret.
const apiKey = "cof4Nc6D8saplXjE3h3HXqHH8m7VU2i1Gs0g85Sp"

// maxBody bounds what is read from Bring!; real answers are far smaller.
const maxBody = 4 << 20

// sessionMargin is how long before its expiry a session is no longer used.
const sessionMargin = time.Minute

var (
	// ErrLoginFailed means Bring! refused the e-mail and password.
	ErrLoginFailed = errors.New("bring login failed")
	// errUnauthorized is a 401 on an authenticated call: the token is stale.
	errUnauthorized = errors.New("bring session rejected")
)

// statusError is an unexpected HTTP status of Bring!.
type statusError struct{ Code int }

func (e *statusError) Error() string { return "unexpected HTTP status " + strconv.Itoa(e.Code) }

// Credentials are the login of a Bring! account.
type Credentials struct {
	Email    string
	Password string
}

// List is one shopping list of an account.
type List struct {
	UUID string
	Name string
}

// Content is the open part of one list.
type Content struct {
	Items []Item
	// UserIcons maps lower-cased catalogue ids to the picture id the user chose.
	UserIcons map[string]string
}

type session struct {
	creds       Credentials
	accessToken string
	userUUID    string
	defaultList string
	expiresAt   time.Time
}

// Client talks to Bring!. It keeps one session, shared by all calls and
// renewed when it expires, its token is rejected or the credentials change.
type Client struct {
	http    *http.Client
	baseURL string
	now     func() time.Time

	mu   sync.Mutex
	sess *session
}

// NewClient returns a Client. A nil httpClient gets a timeout of 15 seconds,
// an empty baseURL means DefaultBaseURL and a nil now means time.Now.
func NewClient(httpClient *http.Client, baseURL string, now func() time.Time) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	if now == nil {
		now = time.Now
	}
	return &Client{http: httpClient, baseURL: baseURL, now: now}
}

func (c *Client) endpoint(path string) string {
	return strings.TrimRight(c.baseURL, "/") + "/" + path
}

func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("X-BRING-API-KEY", apiKey)
	req.Header.Set("Accept-Language", "de-DE,de;q=0.8,en;q=0.5")
}

// do sends req and decodes a 200 answer into out (nil skips decoding).
func (c *Client) do(req *http.Request, out any) (int, error) {
	c.setHeaders(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, fmt.Errorf("request %s: %w", req.URL.Path, stripURL(err))
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
		return resp.StatusCode, nil
	}
	if out != nil {
		if err := json.NewDecoder(io.LimitReader(resp.Body, maxBody)).Decode(out); err != nil {
			return resp.StatusCode, fmt.Errorf("decode %s: %w", req.URL.Path, err)
		}
	}
	return resp.StatusCode, nil
}

func stripURL(err error) error {
	var ue *url.Error
	if errors.As(err, &ue) {
		return ue.Err
	}
	return err
}

type authAnswer struct {
	UUID        string `json:"uuid"`
	ListUUID    string `json:"bringListUUID"`
	AccessToken string `json:"access_token"`
	ExpiresIn   int64  `json:"expires_in"`
}

// login performs the authentication; the form body carries the password and
// goes nowhere else.
func (c *Client) login(ctx context.Context, creds Credentials) (*session, error) {
	form := url.Values{"email": {creds.Email}, "password": {creds.Password}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint("bringauth"), strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("build login request: %w", stripURL(err))
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	var ans authAnswer
	code, err := c.do(req, &ans)
	if err != nil {
		return nil, fmt.Errorf("login: %w", err)
	}
	switch code {
	case http.StatusOK:
	case http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden:
		return nil, ErrLoginFailed
	default:
		return nil, fmt.Errorf("login: %w", &statusError{Code: code})
	}
	if ans.AccessToken == "" {
		return nil, fmt.Errorf("login: %w", ErrLoginFailed)
	}
	ttl := 10 * time.Minute
	if ans.ExpiresIn > 60 {
		ttl = time.Duration(ans.ExpiresIn-30) * time.Second
	}
	return &session{
		creds:       creds,
		accessToken: ans.AccessToken,
		userUUID:    ans.UUID,
		defaultList: ans.ListUUID,
		expiresAt:   c.now().Add(ttl),
	}, nil
}

// session returns the usable session for creds, logging in when there is
// none. Logins are serialised, so concurrent callers share one.
func (c *Client) session(ctx context.Context, creds Credentials) (*session, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if s := c.sess; s != nil && s.creds == creds && s.expiresAt.After(c.now().Add(sessionMargin)) {
		return s, nil
	}
	s, err := c.login(ctx, creds)
	if err != nil {
		c.sess = nil
		return nil, err
	}
	c.sess = s
	return s, nil
}

// drop forgets s if it is still the current session.
func (c *Client) drop(s *session) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.sess == s {
		c.sess = nil
	}
}

// authed runs fn with a session; a rejected token means one fresh login and
// one more try.
func (c *Client) authed(ctx context.Context, creds Credentials, fn func(*session) error) error {
	for attempt := 0; ; attempt++ {
		s, err := c.session(ctx, creds)
		if err != nil {
			return err
		}
		err = fn(s)
		if errors.Is(err, errUnauthorized) && attempt == 0 {
			c.drop(s)
			continue
		}
		return err
	}
}

// get performs an authenticated GET; a 401 is errUnauthorized.
func (c *Client) get(ctx context.Context, s *session, path string, out any) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.endpoint(path), nil)
	if err != nil {
		return 0, fmt.Errorf("build request for %s: %w", path, stripURL(err))
	}
	req.Header.Set("Authorization", "Bearer "+s.accessToken)
	code, err := c.do(req, out)
	if err != nil {
		return code, err
	}
	if code == http.StatusUnauthorized {
		return code, errUnauthorized
	}
	return code, nil
}

type listAnswer struct {
	Purchase []struct {
		Name          string `json:"name"`
		Specification string `json:"specification"`
	} `json:"purchase"`
}

type detailAnswer struct {
	ItemID         string `json:"itemId"`
	UserIconItemID string `json:"userIconItemId"`
}

type userListsAnswer struct {
	Lists []struct {
		ListUUID string `json:"listUuid"`
		Name     string `json:"name"`
	} `json:"lists"`
}

// Content fetches the open items of listUUID; "" means the account's default
// list. The picture ids of the details are best effort: when they cannot be
// read, the items' names decide the pictures.
func (c *Client) Content(ctx context.Context, creds Credentials, listUUID string) (Content, error) {
	var out Content
	err := c.authed(ctx, creds, func(s *session) error {
		id := listUUID
		if id == "" {
			id = s.defaultList
		}
		if id == "" {
			return errors.New("the account has no default list")
		}
		var list listAnswer
		path := "bringlists/" + url.PathEscape(id)
		if code, err := c.get(ctx, s, path, &list); err != nil {
			return fmt.Errorf("get list: %w", err)
		} else if code != http.StatusOK {
			return fmt.Errorf("get list: %w", &statusError{Code: code})
		}
		out = Content{Items: make([]Item, 0, len(list.Purchase)), UserIcons: map[string]string{}}
		for _, p := range list.Purchase {
			out.Items = append(out.Items, Item{Name: p.Name, Specification: p.Specification})
		}
		var details []detailAnswer
		code, err := c.get(ctx, s, path+"/details", &details)
		if errors.Is(err, errUnauthorized) {
			return err
		}
		if err != nil {
			return fmt.Errorf("get list details: %w", err)
		}
		if code == http.StatusOK {
			for _, d := range details {
				key := strings.ToLower(strings.TrimSpace(d.ItemID))
				if _, taken := out.UserIcons[key]; key != "" && !taken {
					out.UserIcons[key] = d.UserIconItemID
				}
			}
		}
		return nil
	})
	return out, err
}

// Lists fetches the shopping lists of the account.
func (c *Client) Lists(ctx context.Context, creds Credentials) ([]List, error) {
	var out []List
	err := c.authed(ctx, creds, func(s *session) error {
		var ans userListsAnswer
		code, err := c.get(ctx, s, "bringusers/"+url.PathEscape(s.userUUID)+"/lists", &ans)
		if err != nil {
			return fmt.Errorf("get user lists: %w", err)
		}
		if code != http.StatusOK {
			return fmt.Errorf("get user lists: %w", &statusError{Code: code})
		}
		out = make([]List, 0, len(ans.Lists))
		for _, l := range ans.Lists {
			out = append(out, List{UUID: l.ListUUID, Name: l.Name})
		}
		return nil
	})
	return out, err
}
