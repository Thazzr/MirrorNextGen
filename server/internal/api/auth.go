package api

import (
	"context"
	"crypto/subtle"
	"errors"
	"math"
	"net"
	"net/http"
	"time"

	"github.com/thazzr/mirrornextgen/server/internal/auth"
)

const (
	// SessionCookieName is the cookie that carries the session token.
	SessionCookieName = "mirror_session"
	csrfHeader        = "X-CSRF-Token"

	msgLoginRequired   = "Bitte melde dich an."
	msgBadCSRF         = "Ungültiges Sicherheits-Token."
	msgWrongPassword   = "Das Passwort stimmt nicht."
	msgWrongCurrent    = "Das aktuelle Passwort stimmt nicht."
	msgAlreadySetUp    = "Der Zugang ist bereits eingerichtet."
	msgNotSetUp        = "Der Zugang ist noch nicht eingerichtet."
	msgTooManyAttempts = "Zu viele Fehlversuche. Versuch es später noch einmal."
)

// publicOperations are the only operations reachable without a session.
// Everything else is denied by default, so a new endpoint is protected
// until someone deliberately adds it here. The strict middleware receives
// the operation ID in the generator's exported form ("GetHealth" for the
// spec's getHealth), so the names are written that way.
var publicOperations = map[string]bool{
	"GetHealth":  true,
	"GetInfo":    true,
	"GetSession": true,
	"SetupAdmin": true,
	"Login":      true,
	// Read by the kiosk container and the dashboard without a session; the
	// state (rotation, output, test pattern) contains no secrets.
	"GetKioskState": true,
	// The dashboard reads weather and the guest Wi-Fi QR code without a
	// session. The weather carries no secrets (the API key stays on the
	// server); the QR code encodes the guest Wi-Fi password, which is the
	// purpose of that widget.
	"GetDashboardWeather": true,
	"GetDashboardWifi":    true,
	// The dashboard reads the layout without a session; it contains only
	// module keys, filtered to the modules that exist.
	"GetDashboardLayout": true,
	// The dashboard reads calendar sections without a session. They carry
	// event titles and places of the household's own calendars (the purpose
	// of the module) but never the calendar addresses, which stay secret.
	"GetDashboardCalendarSection": true,
	// The dashboard loads uploaded icons without a session; the files are
	// validated at upload and served with a locked-down CSP.
	"GetIconFile": true,
}

type ctxKey struct{}

// requestInfo is what the middleware tells the handlers about a request.
type requestInfo struct {
	session *auth.Session // nil without a valid session
	ip      string
	secure  bool // the request came over TLS
}

func infoFrom(ctx context.Context) *requestInfo {
	info, _ := ctx.Value(ctxKey{}).(*requestInfo)
	if info == nil {
		return &requestInfo{}
	}
	return info
}

// clientIP is the host part of the peer address. X-Forwarded-For is ignored
// on purpose: nothing trusted sits in front of the mirror, so the header is
// attacker-controlled and would let anyone dodge the login throttle.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// authMiddleware resolves the session, enforces default-deny by operation ID
// and checks the CSRF token of unsafe methods.
func (s *Server) authMiddleware(next StrictHandlerFunc, operationID string) StrictHandlerFunc {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request, request any) (any, error) {
		info := &requestInfo{ip: clientIP(r), secure: r.TLS != nil}
		if c, err := r.Cookie(SessionCookieName); err == nil {
			if sess, ok := s.sessions.Lookup(c.Value); ok {
				info.session = &sess
			}
		}
		if !publicOperations[operationID] {
			if info.session == nil {
				WriteProblem(w, http.StatusUnauthorized, msgLoginRequired)
				return nil, nil
			}
			if r.Method != http.MethodGet && r.Method != http.MethodHead &&
				subtle.ConstantTimeCompare([]byte(r.Header.Get(csrfHeader)), []byte(info.session.CSRFToken)) != 1 {
				WriteProblem(w, http.StatusForbidden, msgBadCSRF)
				return nil, nil
			}
		}
		return next(context.WithValue(ctx, ctxKey{}, info), w, r, request)
	}
}

func problem(status int, detail string) Problem {
	p := Problem{Title: http.StatusText(status), Status: status}
	if detail != "" {
		p.Detail = &detail
	}
	return p
}

func fieldProblem(field string, err error) Problem {
	p := problem(http.StatusBadRequest, "")
	p.Errors = &map[string]string{field: err.Error()}
	return p
}

func isPasswordRuleError(err error) bool {
	return errors.Is(err, auth.ErrPasswordTooShort) || errors.Is(err, auth.ErrPasswordTooLong)
}

func sessionCookie(token string, secure bool) *string {
	c := http.Cookie{
		Name:     SessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   secure,
	}
	v := c.String()
	return &v
}

// expiredSessionCookie tells the browser to drop the cookie (Max-Age=0).
func expiredSessionCookie(secure bool) *string {
	c := http.Cookie{
		Name:     SessionCookieName,
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   secure,
	}
	v := c.String()
	return &v
}

func signedInSession(sess auth.Session) Session {
	return Session{Authenticated: true, CsrfToken: &sess.CSRFToken}
}

// GetSession reports whether setup is needed and whether the caller is signed in.
func (s *Server) GetSession(ctx context.Context, _ GetSessionRequestObject) (GetSessionResponseObject, error) {
	out := Session{SetupRequired: !s.admin.IsConfigured()}
	if sess := infoFrom(ctx).session; sess != nil {
		out.Authenticated = true
		out.CsrfToken = &sess.CSRFToken
	}
	return GetSession200JSONResponse(out), nil
}

// SetupAdmin sets the first password and signs the caller in.
func (s *Server) SetupAdmin(ctx context.Context, request SetupAdminRequestObject) (SetupAdminResponseObject, error) {
	err := s.admin.Setup(request.Body.Password)
	switch {
	case errors.Is(err, auth.ErrAlreadyConfigured):
		return SetupAdmin409ApplicationProblemPlusJSONResponse{ConflictApplicationProblemPlusJSONResponse(problem(http.StatusConflict, msgAlreadySetUp))}, nil
	case isPasswordRuleError(err):
		return SetupAdmin400ApplicationProblemPlusJSONResponse{BadRequestApplicationProblemPlusJSONResponse(fieldProblem("password", err))}, nil
	case err != nil:
		return nil, err
	}
	sess, err := s.sessions.Create()
	if err != nil {
		return nil, err
	}
	return SetupAdmin200JSONResponse{
		Body:    signedInSession(sess),
		Headers: SetupAdmin200ResponseHeaders{SetCookie: sessionCookie(sess.Token, infoFrom(ctx).secure)},
	}, nil
}

// Login checks the password behind the per-IP throttle.
func (s *Server) Login(ctx context.Context, request LoginRequestObject) (LoginResponseObject, error) {
	info := infoFrom(ctx)
	if !s.admin.IsConfigured() {
		return Login409ApplicationProblemPlusJSONResponse{ConflictApplicationProblemPlusJSONResponse(problem(http.StatusConflict, msgNotSetUp))}, nil
	}
	if wait, ok := s.throttle.Attempt(info.ip); !ok {
		secs := retryAfterSeconds(wait)
		return Login429ApplicationProblemPlusJSONResponse{
			Body:    problem(http.StatusTooManyRequests, msgTooManyAttempts),
			Headers: Login429ResponseHeaders{RetryAfter: &secs},
		}, nil
	}
	err := s.admin.Verify(request.Body.Password)
	switch {
	case errors.Is(err, auth.ErrWrongPassword):
		return Login401ApplicationProblemPlusJSONResponse{UnauthorizedApplicationProblemPlusJSONResponse(problem(http.StatusUnauthorized, msgWrongPassword))}, nil
	case errors.Is(err, auth.ErrNotConfigured):
		return Login409ApplicationProblemPlusJSONResponse{ConflictApplicationProblemPlusJSONResponse(problem(http.StatusConflict, msgNotSetUp))}, nil
	case err != nil:
		return nil, err
	}
	s.throttle.Succeed(info.ip)
	sess, err := s.sessions.Create()
	if err != nil {
		return nil, err
	}
	return Login200JSONResponse{
		Body:    signedInSession(sess),
		Headers: Login200ResponseHeaders{SetCookie: sessionCookie(sess.Token, info.secure)},
	}, nil
}

// Logout ends the current session. The middleware guarantees one exists.
func (s *Server) Logout(ctx context.Context, _ LogoutRequestObject) (LogoutResponseObject, error) {
	info := infoFrom(ctx)
	s.sessions.Delete(info.session.Token)
	return Logout204Response{Headers: Logout204ResponseHeaders{SetCookie: expiredSessionCookie(info.secure)}}, nil
}

// ChangePassword replaces the password and ends all other sessions. A wrong
// current password is not a login attempt (the caller already holds a
// session), so it does not feed the throttle.
func (s *Server) ChangePassword(ctx context.Context, request ChangePasswordRequestObject) (ChangePasswordResponseObject, error) {
	err := s.admin.Change(request.Body.CurrentPassword, request.Body.NewPassword)
	switch {
	case errors.Is(err, auth.ErrWrongPassword):
		return ChangePassword401ApplicationProblemPlusJSONResponse{UnauthorizedApplicationProblemPlusJSONResponse(problem(http.StatusUnauthorized, msgWrongCurrent))}, nil
	case isPasswordRuleError(err):
		return ChangePassword400ApplicationProblemPlusJSONResponse{BadRequestApplicationProblemPlusJSONResponse(fieldProblem("newPassword", err))}, nil
	case err != nil:
		return nil, err
	}
	s.sessions.DeleteAllExcept(infoFrom(ctx).session.Token)
	return ChangePassword204Response{}, nil
}

// retryAfterSeconds rounds up so a client never retries a moment too early,
// and never answers 0, which would invite an immediate retry.
func retryAfterSeconds(d time.Duration) int { return max(1, int(math.Ceil(d.Seconds()))) }
