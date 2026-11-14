// Package logbook keeps the most recent log records in memory, so the admin
// can read them without access to the container's stdout.
package logbook

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"
)

// Capacity is the number of records the logbook keeps.
const Capacity = 500

// Attr is one flattened attribute of a record.
type Attr struct {
	Key   string
	Value string
}

// Entry is one recorded log record.
type Entry struct {
	Time    time.Time
	Level   slog.Level
	Message string
	Attrs   []Attr
}

// Logbook is a ring buffer of the last Capacity records. The zero value is
// not usable; create it with New.
type Logbook struct {
	mu      sync.Mutex
	entries []Entry // ring storage, at most Capacity long
	next    int     // index the next record is written to once the ring is full
}

// New returns an empty logbook.
func New() *Logbook { return &Logbook{} }

func (l *Logbook) add(e Entry) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.entries) < Capacity {
		l.entries = append(l.entries, e)
		return
	}
	l.entries[l.next] = e
	l.next = (l.next + 1) % Capacity
}

// Entries returns the records at or above minLevel, newest first.
func (l *Logbook) Entries(minLevel slog.Level) []Entry {
	l.mu.Lock()
	defer l.mu.Unlock()
	n := len(l.entries)
	out := make([]Entry, 0, n)
	for i := 0; i < n; i++ {
		// Oldest record sits at l.next once the ring is full, else at 0;
		// l.next stays 0 until then, so one formula serves both.
		e := l.entries[(l.next+n-1-i)%n]
		if e.Level >= minLevel {
			out = append(out, e)
		}
	}
	return out
}

// Counts returns how many records of each level class the buffer holds.
func (l *Logbook) Counts() (warn, errs int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, e := range l.entries {
		switch {
		case e.Level >= slog.LevelError:
			errs++
		case e.Level >= slog.LevelWarn:
			warn++
		}
	}
	return warn, errs
}

// Handler returns a slog.Handler that passes every record to next and also
// records it in the logbook. Enabled follows next.
func (l *Logbook) Handler(next slog.Handler) slog.Handler {
	return &handler{book: l, next: next}
}

type handler struct {
	book   *Logbook
	next   slog.Handler
	groups []string // open groups, outermost first
	attrs  []Attr   // attributes added by WithAttrs, already flattened
}

func (h *handler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *handler) Handle(ctx context.Context, r slog.Record) error {
	e := Entry{Time: r.Time, Level: r.Level, Message: r.Message}
	e.Attrs = append(e.Attrs, h.attrs...)
	r.Attrs(func(a slog.Attr) bool {
		e.Attrs = flatten(e.Attrs, strings.Join(h.groups, "."), a)
		return true
	})
	h.book.add(e)
	return h.next.Handle(ctx, r)
}

func (h *handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	c := *h
	c.attrs = append([]Attr(nil), h.attrs...)
	prefix := strings.Join(h.groups, ".")
	for _, a := range attrs {
		c.attrs = flatten(c.attrs, prefix, a)
	}
	c.next = h.next.WithAttrs(attrs)
	return &c
}

func (h *handler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	c := *h
	c.groups = append(append([]string(nil), h.groups...), name)
	c.next = h.next.WithGroup(name)
	return &c
}

// flatten appends a to out as key/value strings; group values become
// "group.key" and empty attributes are dropped, as slog does.
func flatten(out []Attr, prefix string, a slog.Attr) []Attr {
	a.Value = a.Value.Resolve()
	if a.Equal(slog.Attr{}) {
		return out
	}
	key := a.Key
	if prefix != "" && key != "" {
		key = prefix + "." + key
	} else if prefix != "" {
		key = prefix
	}
	if a.Value.Kind() == slog.KindGroup {
		for _, g := range a.Value.Group() {
			out = flatten(out, key, g)
		}
		return out
	}
	return append(out, Attr{Key: key, Value: a.Value.String()})
}
