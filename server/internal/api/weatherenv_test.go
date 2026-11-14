package api

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/thazzr/mirrornextgen/server/internal/settings"
	"github.com/thazzr/mirrornextgen/server/internal/weather"
)

// fakeOW is a fake OpenWeather serving the recorded fixtures of the weather
// package.
type fakeOW struct {
	server *httptest.Server
	calls  atomic.Int32
	// status, when non-zero, is answered to every request instead of data.
	status atomic.Int32
}

func newFakeOW(t *testing.T) *fakeOW {
	t.Helper()
	f := &fakeOW{}
	read := func(name string) []byte {
		b, err := os.ReadFile(filepath.Join("..", "weather", "testdata", name))
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.calls.Add(1)
		if st := f.status.Load(); st != 0 {
			w.WriteHeader(int(st))
			return
		}
		switch r.URL.Path {
		case "/geo/1.0/direct":
			_, _ = w.Write(read("geocode_berlin.json"))
		case "/data/3.0/onecall":
			_, _ = w.Write(read("onecall_berlin.json"))
		case "/data/3.0/onecall/timemachine":
			_, _ = w.Write(read("timemachine_clear.json"))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(f.server.Close)
	return f
}

// newTestWeather returns a weather service reading the store's settings and
// talking to a fake OpenWeather.
func newTestWeather(t *testing.T, store *settings.Store, now func() time.Time) (*weather.Service, *fakeOW) {
	t.Helper()
	loc, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatal(err)
	}
	ow := newFakeOW(t)
	svc := weather.New(weather.Options{
		HTTPClient: ow.server.Client(),
		BaseURL:    ow.server.URL,
		Config: func() weather.Config {
			w := store.Get().Weather
			return weather.Config{Location: w.Location, APIKey: w.APIKey, ShowIcons: w.ShowIcons, RefreshMinutes: w.RefreshMinutes}
		},
		Now:      now,
		Location: loc,
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	return svc, ow
}
