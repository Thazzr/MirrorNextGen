package api

import (
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/thazzr/mirrornextgen/server/internal/update"
)

const (
	logbookPath = "/api/v1/logbook"
	updatePath  = "/api/v1/system/update"
)

func TestLogbookNeedsSession(t *testing.T) {
	e := newTestEnv(t)
	e.setup(t)
	wantProblem(t, e.do(http.MethodGet, logbookPath, nil, opts{}), http.StatusUnauthorized, "")
}

func TestLogbookFiltersByLevelAndFlattensAttrs(t *testing.T) {
	e := newTestEnv(t)
	c := e.setup(t)
	log := slog.New(e.book.Handler(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelDebug})))
	log.Debug("d")
	log.Info("i", "k", "v")
	log.Warn("w", slog.Group("g", slog.Int("n", 1)))
	log.Error("e")

	get := func(query string) Logbook {
		rec := e.do(http.MethodGet, logbookPath+query, nil, opts{cookie: c.cookie})
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d: %s", rec.Code, rec.Body)
		}
		return decode[Logbook](t, rec)
	}
	msgs := func(l Logbook) string {
		var m []string
		for _, e := range l.Entries {
			m = append(m, e.Message+":"+string(e.Level))
		}
		return strings.Join(m, ",")
	}

	def := get("")
	if def.Capacity != 500 {
		t.Errorf("capacity = %d", def.Capacity)
	}
	if got := msgs(def); got != "e:error,w:warn,i:info" {
		t.Errorf("default (info) = %s", got)
	}
	if got := msgs(get("?minLevel=debug")); got != "e:error,w:warn,i:info,d:debug" {
		t.Errorf("debug = %s", got)
	}
	if got := msgs(get("?minLevel=warn")); got != "e:error,w:warn" {
		t.Errorf("warn = %s", got)
	}
	if got := msgs(get("?minLevel=error")); got != "e:error" {
		t.Errorf("error = %s", got)
	}
	warn := get("?minLevel=warn")
	if a := warn.Entries[1].Attrs; len(a) != 1 || a[0].Key != "g.n" || a[0].Value != "1" {
		t.Errorf("attrs = %+v", a)
	}
	wantProblem(t, e.do(http.MethodGet, logbookPath+"?minLevel=trace", nil, opts{cookie: c.cookie}), http.StatusBadRequest, "")
}

func TestLogbookCapacityIsFiveHundredAndAFullRingAnswersExactlyThat(t *testing.T) {
	e := newTestEnv(t)
	c := e.setup(t)
	log := slog.New(e.book.Handler(slog.NewTextHandler(io.Discard, nil)))
	for i := 0; i < 501; i++ {
		log.Info("fill")
	}
	l := decode[Logbook](t, e.do(http.MethodGet, logbookPath, nil, opts{cookie: c.cookie}))
	if l.Capacity != 500 || len(l.Entries) != 500 {
		t.Errorf("capacity = %d, entries = %d", l.Capacity, len(l.Entries))
	}
}

func TestLogbookIsEmptyArrayNotNull(t *testing.T) {
	e := newTestEnv(t)
	c := e.setup(t)
	rec := e.do(http.MethodGet, logbookPath, nil, opts{cookie: c.cookie})
	if !strings.Contains(rec.Body.String(), `"entries":[]`) {
		t.Errorf("body = %s", rec.Body)
	}
}

func TestSystemUpdateNeedsSessionAndCSRF(t *testing.T) {
	e := newTestEnv(t)
	c := e.setup(t)
	wantProblem(t, e.do(http.MethodGet, updatePath, nil, opts{}), http.StatusUnauthorized, "")
	wantProblem(t, e.do(http.MethodPost, updatePath, nil, opts{}), http.StatusUnauthorized, "")
	wantProblem(t, e.do(http.MethodPost, updatePath, nil, opts{cookie: c.cookie}), http.StatusForbidden, "")
	wantProblem(t, e.do(http.MethodPost, updatePath, nil, opts{cookie: c.cookie, csrf: "wrong"}), http.StatusForbidden, "")
	if _, err := os.Stat(filepath.Join(e.controlDir, "update-request")); err == nil {
		t.Error("a refused POST wrote a request")
	}
}

func TestSystemUpdateStatusAndRequest(t *testing.T) {
	e := newTestEnv(t)
	c := e.setup(t)

	st := decode[UpdateStatus](t, e.do(http.MethodGet, updatePath, nil, opts{cookie: c.cookie}))
	if !st.Available || st.Pending || st.State != Idle {
		t.Errorf("initial status = %+v", st)
	}

	rec := e.do(http.MethodPost, updatePath, nil, opts{cookie: c.cookie, csrf: c.csrf})
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body)
	}
	st = decode[UpdateStatus](t, rec)
	if !st.Available || !st.Pending {
		t.Errorf("status after request = %+v", st)
	}
	if _, err := os.Stat(filepath.Join(e.controlDir, "update-request")); err != nil {
		t.Errorf("no request file: %v", err)
	}

	wantProblem(t, e.do(http.MethodPost, updatePath, nil, opts{cookie: c.cookie, csrf: c.csrf}), http.StatusConflict, "Ein Update läuft bereits.")
}

func TestSystemUpdateRunningBlocksAndHostMessageIsShown(t *testing.T) {
	e := newTestEnv(t)
	c := e.setup(t)
	status := `{"state":"running","startedAt":"2026-11-14T11:50:00Z"}`
	if err := os.WriteFile(filepath.Join(e.controlDir, "update-status.json"), []byte(status), 0o644); err != nil {
		t.Fatal(err)
	}
	wantProblem(t, e.do(http.MethodPost, updatePath, nil, opts{cookie: c.cookie, csrf: c.csrf}), http.StatusConflict, "Ein Update läuft bereits.")

	failed := `{"state":"failed","startedAt":"2026-11-14T10:00:00Z","finishedAt":"2026-11-14T10:01:00Z","message":"pull access denied"}`
	if err := os.WriteFile(filepath.Join(e.controlDir, "update-status.json"), []byte(failed), 0o644); err != nil {
		t.Fatal(err)
	}
	st := decode[UpdateStatus](t, e.do(http.MethodGet, updatePath, nil, opts{cookie: c.cookie}))
	if st.State != Failed || st.Message == nil || *st.Message != "pull access denied" || st.FinishedAt == nil {
		t.Errorf("status = %+v", st)
	}
}

func TestSystemUpdateUnavailable(t *testing.T) {
	e := newTestEnvWithSettingsDir(t, t.TempDir(), func(d *Deps) { d.Update = update.New("") })
	c := e.setup(t)
	st := decode[UpdateStatus](t, e.do(http.MethodGet, updatePath, nil, opts{cookie: c.cookie}))
	if st.Available || st.Pending || st.State != Idle {
		t.Errorf("status = %+v", st)
	}
	wantProblem(t, e.do(http.MethodPost, updatePath, nil, opts{cookie: c.cookie, csrf: c.csrf}), http.StatusServiceUnavailable, "Updates per Knopf sind nicht eingerichtet.")
}

func TestSystemUpdateRequestIsLogged(t *testing.T) {
	var buf syncBuffer
	e := newTestEnvWithSettingsDir(t, t.TempDir(), func(d *Deps) { d.Logger = slog.New(slog.NewTextHandler(&buf, nil)) })
	c := e.setup(t)
	if rec := e.do(http.MethodPost, updatePath, nil, opts{cookie: c.cookie, csrf: c.csrf}); rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body)
	}
	if out := buf.String(); !strings.Contains(out, `msg="update requested"`) {
		t.Errorf("log = %s", out)
	}
}

func TestSystemUpdateStalledRequestIsReportedAndReplaced(t *testing.T) {
	e := newTestEnv(t)
	c := e.setup(t)
	old := e.clock.now().Add(-time.Hour).Format(time.RFC3339)
	if err := os.WriteFile(filepath.Join(e.controlDir, "update-request"), []byte(old+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	st := decode[UpdateStatus](t, e.do(http.MethodGet, updatePath, nil, opts{cookie: c.cookie}))
	if !st.Pending || !st.Stalled {
		t.Errorf("status = %+v", st)
	}
	rec := e.do(http.MethodPost, updatePath, nil, opts{cookie: c.cookie, csrf: c.csrf})
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body)
	}
	if st = decode[UpdateStatus](t, rec); !st.Pending || st.Stalled {
		t.Errorf("status after new request = %+v", st)
	}
}

func TestSystemUpdateNotWritableAnswers503AndLogsTheDirectory(t *testing.T) {
	var buf syncBuffer
	missing := filepath.Join(t.TempDir(), "nope")
	e := newTestEnvWithSettingsDir(t, t.TempDir(), func(d *Deps) {
		d.Logger = slog.New(slog.NewTextHandler(&buf, nil))
		d.Update = update.New(missing)
	})
	c := e.setup(t)
	wantProblem(t, e.do(http.MethodPost, updatePath, nil, opts{cookie: c.cookie, csrf: c.csrf}), http.StatusServiceUnavailable,
		"Der Spiegel kann nicht in das Steuerverzeichnis schreiben. Richte den Update-Dienst ein (deploy/INSTALL.md, Abschnitt „Updates per Knopf“).")
	if out := buf.String(); !strings.Contains(out, "level=WARN") || !strings.Contains(out, missing) {
		t.Errorf("log = %s", out)
	}
}
