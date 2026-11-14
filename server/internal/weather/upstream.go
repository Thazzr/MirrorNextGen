package weather

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
	"time"
)

// DefaultBaseURL is the OpenWeather API (MIRROR_OPENWEATHER_URL overrides it).
const DefaultBaseURL = "https://api.openweathermap.org/"

// maxBody bounds what is read from OpenWeather; real answers are far smaller.
const maxBody = 4 << 20

var errLocationNotFound = errors.New("location not found")

// statusError is an unexpected HTTP status of OpenWeather.
type statusError struct{ Code int }

func (e *statusError) Error() string { return "unexpected HTTP status " + strconv.Itoa(e.Code) }

type upstreamWeather struct {
	Description string `json:"description"`
	Icon        string `json:"icon"`
}

type upstreamRain struct {
	OneHour float64 `json:"1h"`
}

type upstreamGeo struct {
	Name string  `json:"name"`
	Lat  float64 `json:"lat"`
	Lon  float64 `json:"lon"`
}

type upstreamCurrent struct {
	Sunrise   int64             `json:"sunrise"`
	Sunset    int64             `json:"sunset"`
	Temp      float64           `json:"temp"`
	FeelsLike float64           `json:"feels_like"`
	Humidity  int               `json:"humidity"`
	UVI       float64           `json:"uvi"`
	WindSpeed float64           `json:"wind_speed"`
	Weather   []upstreamWeather `json:"weather"`
	Rain      upstreamRain      `json:"rain"`
}

type upstreamHourly struct {
	Time      int64             `json:"dt"`
	Temp      float64           `json:"temp"`
	FeelsLike float64           `json:"feels_like"`
	Weather   []upstreamWeather `json:"weather"`
	Pop       float64           `json:"pop"`
	WindSpeed float64           `json:"wind_speed"`
	WindGust  *float64          `json:"wind_gust"`
	Rain      upstreamRain      `json:"rain"`
}

type upstreamDaily struct {
	Time int64 `json:"dt"`
	Temp struct {
		Min float64 `json:"min"`
		Max float64 `json:"max"`
	} `json:"temp"`
	Weather   []upstreamWeather `json:"weather"`
	Pop       float64           `json:"pop"`
	WindSpeed float64           `json:"wind_speed"`
	Rain      float64           `json:"rain"`
}

type upstreamOneCall struct {
	Current *upstreamCurrent `json:"current"`
	Hourly  []upstreamHourly `json:"hourly"`
	Daily   []upstreamDaily  `json:"daily"`
}

type upstreamHistoryPoint struct {
	Temp      float64           `json:"temp"`
	FeelsLike float64           `json:"feels_like"`
	Weather   []upstreamWeather `json:"weather"`
	WindSpeed float64           `json:"wind_speed"`
	WindGust  *float64          `json:"wind_gust"`
	Rain      upstreamRain      `json:"rain"`
}

type upstreamTimeMachine struct {
	Data []upstreamHistoryPoint `json:"data"`
}

// client talks to OpenWeather. The API key is only ever put into request
// URLs; errors are stripped of the URL before they leave this file.
type client struct {
	http    *http.Client
	baseURL string
	apiKey  string
}

// get fetches path with the query and decodes the JSON answer into out.
func (c *client) get(ctx context.Context, path string, q url.Values, out any) error {
	q.Set("appid", c.apiKey)
	u := strings.TrimRight(c.baseURL, "/") + "/" + path + "?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return fmt.Errorf("build request for %s: %w", path, stripURL(err))
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("request %s: %w", path, stripURL(err))
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
		return fmt.Errorf("request %s: %w", path, &statusError{Code: resp.StatusCode})
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxBody)).Decode(out); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	return nil
}

// stripURL removes the request URL (which carries the API key) from a
// *url.Error and keeps the underlying cause.
func stripURL(err error) error {
	var ue *url.Error
	if errors.As(err, &ue) {
		return ue.Err
	}
	return err
}

func (c *client) geocode(ctx context.Context, location string) (upstreamGeo, error) {
	var res []upstreamGeo
	q := url.Values{"q": {location}, "limit": {"1"}}
	if err := c.get(ctx, "geo/1.0/direct", q, &res); err != nil {
		return upstreamGeo{}, err
	}
	if len(res) == 0 {
		return upstreamGeo{}, errLocationNotFound
	}
	return res[0], nil
}

func coords(g upstreamGeo) url.Values {
	return url.Values{
		"lat":   {strconv.FormatFloat(g.Lat, 'f', -1, 64)},
		"lon":   {strconv.FormatFloat(g.Lon, 'f', -1, 64)},
		"units": {"metric"},
		"lang":  {"de"},
	}
}

func (c *client) oneCall(ctx context.Context, g upstreamGeo) (upstreamOneCall, error) {
	q := coords(g)
	q.Set("exclude", "minutely,alerts")
	var res upstreamOneCall
	if err := c.get(ctx, "data/3.0/onecall", q, &res); err != nil {
		return upstreamOneCall{}, err
	}
	if res.Current == nil {
		return upstreamOneCall{}, errors.New("one call answer has no current weather")
	}
	return res, nil
}

// timeMachine returns the weather at the past instant at.
func (c *client) timeMachine(ctx context.Context, g upstreamGeo, at time.Time) (upstreamHistoryPoint, bool, error) {
	q := coords(g)
	q.Set("dt", strconv.FormatInt(at.Unix(), 10))
	var res upstreamTimeMachine
	if err := c.get(ctx, "data/3.0/onecall/timemachine", q, &res); err != nil {
		return upstreamHistoryPoint{}, false, err
	}
	if len(res.Data) == 0 {
		return upstreamHistoryPoint{}, false, nil
	}
	return res.Data[0], true, nil
}
