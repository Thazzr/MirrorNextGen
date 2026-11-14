package auth

import (
	"testing"
	"time"
)

type fakeClock struct{ t time.Time }

func newClock() *fakeClock               { return &fakeClock{t: time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)} }
func (c *fakeClock) now() time.Time      { return c.t }
func (c *fakeClock) add(d time.Duration) { c.t = c.t.Add(d) }

func TestSessionCreateAndLookup(t *testing.T) {
	m := NewSessions(newClock().now)
	a, err := m.Create()
	if err != nil {
		t.Fatal(err)
	}
	b, _ := m.Create()
	if len(a.Token) != 43 || len(a.CSRFToken) != 43 { // 32 bytes, unpadded base64url
		t.Errorf("token lengths %d/%d, want 43", len(a.Token), len(a.CSRFToken))
	}
	if a.Token == b.Token || a.CSRFToken == b.CSRFToken || a.Token == a.CSRFToken {
		t.Error("tokens must be unique, and the CSRF token distinct from the session token")
	}
	got, ok := m.Lookup(a.Token)
	if !ok || got != a {
		t.Errorf("Lookup = %+v, %v", got, ok)
	}
	if _, ok := m.Lookup("unknown"); ok {
		t.Error("unknown token must not resolve")
	}
	if _, ok := m.Lookup(""); ok {
		t.Error("empty token must not resolve")
	}
}

func TestSessionSlidingExpiry(t *testing.T) {
	clock := newClock()
	m := NewSessions(clock.now)
	s, _ := m.Create()

	clock.add(SessionTTL - time.Second)
	if _, ok := m.Lookup(s.Token); !ok {
		t.Fatal("expired just before 8h")
	}
	// The lookup above extended the session: another almost-8h is fine.
	clock.add(SessionTTL - time.Second)
	if _, ok := m.Lookup(s.Token); !ok {
		t.Fatal("lookup did not extend the session")
	}
	clock.add(SessionTTL)
	if _, ok := m.Lookup(s.Token); ok {
		t.Fatal("session alive after 8h without use")
	}
	clock.add(-SessionTTL)
	if _, ok := m.Lookup(s.Token); ok {
		t.Fatal("expired session must be removed, not revived")
	}
}

func TestSessionLRUEviction(t *testing.T) {
	clock := newClock()
	m := NewSessions(clock.now)
	var all []Session
	for range MaxSessions {
		s, _ := m.Create()
		all = append(all, s)
		// Same instant for all: eviction must not depend on clock resolution.
	}
	// Use the oldest, so the second-oldest becomes least recently used.
	if _, ok := m.Lookup(all[0].Token); !ok {
		t.Fatal("first session missing")
	}
	extra, _ := m.Create() // the 21st
	if _, ok := m.Lookup(all[1].Token); ok {
		t.Error("least recently used session survived")
	}
	for _, s := range append([]Session{all[0], extra}, all[2:]...) {
		if _, ok := m.Lookup(s.Token); !ok {
			t.Errorf("session %q evicted wrongly", s.Token)
		}
	}
	if len(m.entries) != MaxSessions {
		t.Errorf("table holds %d sessions, want %d", len(m.entries), MaxSessions)
	}
}

func TestSessionDelete(t *testing.T) {
	m := NewSessions(newClock().now)
	a, _ := m.Create()
	b, _ := m.Create()
	c, _ := m.Create()

	m.Delete(a.Token)
	if _, ok := m.Lookup(a.Token); ok {
		t.Error("deleted session still valid")
	}
	m.Delete("unknown") // no-op

	m.DeleteAllExcept(b.Token)
	if _, ok := m.Lookup(b.Token); !ok {
		t.Error("kept session was removed")
	}
	if _, ok := m.Lookup(c.Token); ok {
		t.Error("other session survived DeleteAllExcept")
	}
}
