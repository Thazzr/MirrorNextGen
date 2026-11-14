package logbook

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"
)

func newLogger(t *testing.T, level slog.Level) (*slog.Logger, *Logbook, *bytes.Buffer) {
	t.Helper()
	book := New()
	var out bytes.Buffer
	h := slog.NewJSONHandler(&out, &slog.HandlerOptions{Level: level})
	return slog.New(book.Handler(h)), book, &out
}

func TestRingKeepsNewestCapacityEntriesNewestFirst(t *testing.T) {
	log, book, _ := newLogger(t, slog.LevelInfo)
	total := Capacity + 7
	for i := 0; i < total; i++ {
		log.Info(fmt.Sprintf("m%d", i))
	}
	got := book.Entries(slog.LevelDebug)
	if len(got) != Capacity {
		t.Fatalf("len = %d, want %d", len(got), Capacity)
	}
	if got[0].Message != fmt.Sprintf("m%d", total-1) {
		t.Errorf("newest = %q", got[0].Message)
	}
	if got[Capacity-1].Message != "m7" {
		t.Errorf("oldest = %q, want m7", got[Capacity-1].Message)
	}
}

func TestOrderBeforeTheRingIsFull(t *testing.T) {
	log, book, _ := newLogger(t, slog.LevelInfo)
	log.Info("a")
	log.Info("b")
	got := book.Entries(slog.LevelDebug)
	if len(got) != 2 || got[0].Message != "b" || got[1].Message != "a" {
		t.Errorf("entries = %+v", got)
	}
}

func TestMinLevelFilterAndCounts(t *testing.T) {
	log, book, _ := newLogger(t, slog.LevelDebug)
	log.Debug("d")
	log.Info("i")
	log.Warn("w1")
	log.Warn("w2")
	log.Error("e")
	tests := []struct {
		min  slog.Level
		want []string
	}{
		{slog.LevelDebug, []string{"e", "w2", "w1", "i", "d"}},
		{slog.LevelInfo, []string{"e", "w2", "w1", "i"}},
		{slog.LevelWarn, []string{"e", "w2", "w1"}},
		{slog.LevelError, []string{"e"}},
	}
	for _, tt := range tests {
		var msgs []string
		for _, e := range book.Entries(tt.min) {
			msgs = append(msgs, e.Message)
		}
		if strings.Join(msgs, ",") != strings.Join(tt.want, ",") {
			t.Errorf("min %v: %v, want %v", tt.min, msgs, tt.want)
		}
	}
	if w, e := book.Counts(); w != 2 || e != 1 {
		t.Errorf("counts = %d warn, %d error", w, e)
	}
}

func TestEnabledFollowsWrappedHandlerAndStdoutStillWritten(t *testing.T) {
	log, book, out := newLogger(t, slog.LevelWarn)
	log.Info("quiet")
	log.Warn("loud")
	if got := book.Entries(slog.LevelDebug); len(got) != 1 || got[0].Message != "loud" {
		t.Errorf("entries = %+v", got)
	}
	if !strings.Contains(out.String(), `"msg":"loud"`) || strings.Contains(out.String(), "quiet") {
		t.Errorf("stdout = %s", out.String())
	}
}

func TestAttrsAreFlattenedWithGroupsAndWith(t *testing.T) {
	log, book, _ := newLogger(t, slog.LevelInfo)
	l := log.With("svc", "weather").WithGroup("req").With("id", 7)
	l.Info("hello", "path", "/x", slog.Group("h", slog.String("a", "b"), slog.Int("n", 2)), slog.Any("err", fmt.Errorf("boom")))

	got := book.Entries(slog.LevelInfo)
	if len(got) != 1 {
		t.Fatal("no entry")
	}
	want := []Attr{
		{"svc", "weather"},
		{"req.id", "7"},
		{"req.path", "/x"},
		{"req.h.a", "b"},
		{"req.h.n", "2"},
		{"req.err", "boom"},
	}
	if fmt.Sprint(got[0].Attrs) != fmt.Sprint(want) {
		t.Errorf("attrs = %v, want %v", got[0].Attrs, want)
	}
}

func TestWithAttrsDoesNotShareStateBetweenDerivedLoggers(t *testing.T) {
	log, book, _ := newLogger(t, slog.LevelInfo)
	a := log.With("x", "1")
	b := log.With("x", "2")
	a.Info("a")
	b.Info("b")
	got := book.Entries(slog.LevelInfo)
	if got[0].Attrs[0].Value != "2" || got[1].Attrs[0].Value != "1" {
		t.Errorf("entries = %+v", got)
	}
}

func TestConcurrentWritesAndReads(t *testing.T) {
	log, book, _ := newLogger(t, slog.LevelInfo)
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				log.With("g", g).Warn("x", "i", i)
				book.Entries(slog.LevelInfo)
				book.Counts()
			}
		}()
	}
	wg.Wait()
	if got := book.Entries(slog.LevelInfo); len(got) != Capacity {
		t.Errorf("len = %d, want %d", len(got), Capacity)
	}
}

func TestCapacityIsFiveHundred(t *testing.T) {
	if Capacity != 500 {
		t.Errorf("Capacity = %d", Capacity)
	}
}

func TestDerivedLoggersOfALoggerWithAttrsDoNotAlias(t *testing.T) {
	log, book, _ := newLogger(t, slog.LevelInfo)
	// Three attributes built one by one leave spare capacity (len 3, cap 4):
	// a derived handler that appended in place would share the fourth slot.
	base := log.With("a", "1", "c", "2", "d", "3")
	x := base.With("b", "2")
	y := base.With("b", "3")
	x.Info("x")
	y.Info("y")
	got := book.Entries(slog.LevelInfo) // newest first: y, x
	last := func(e Entry) Attr { return e.Attrs[len(e.Attrs)-1] }
	if len(got) != 2 || last(got[0]) != (Attr{"b", "3"}) || last(got[1]) != (Attr{"b", "2"}) {
		t.Errorf("entries = %+v", got)
	}
}

func TestHandlerEmptyGroupNameAddsNoPrefixAndEmptyAttrsAreDropped(t *testing.T) {
	book := New()
	h := book.Handler(slog.NewTextHandler(&bytes.Buffer{}, nil)).WithGroup("").WithAttrs([]slog.Attr{{}, slog.Group("empty"), slog.String("w", "1")})
	h = h.WithGroup("g") // an empty name before must not leave a leading dot
	r := slog.NewRecord(time.Time{}, slog.LevelInfo, "m", 0)
	r.AddAttrs(slog.Attr{}, slog.Group("none"), slog.String("k", "v"))
	if err := h.Handle(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	got := book.Entries(slog.LevelInfo)
	want := []Attr{{"w", "1"}, {"g.k", "v"}}
	if len(got) != 1 || fmt.Sprint(got[0].Attrs) != fmt.Sprint(want) {
		t.Errorf("entries = %+v, want attrs %v", got, want)
	}
}
