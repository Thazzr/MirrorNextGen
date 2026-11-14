package auth

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"sync"
	"time"
)

const (
	// SessionTTL is the idle time after which a session expires; every
	// authenticated request starts it anew.
	SessionTTL = 8 * time.Hour
	// MaxSessions bounds memory; the least recently used session makes room.
	MaxSessions = 20
)

// Session is a signed-in admin. CSRFToken belongs to this session only.
type Session struct {
	Token     string
	CSRFToken string
}

type sessionEntry struct {
	Session
	lastSeen time.Time
	// seq orders entries by recency; the clock alone can tie.
	seq uint64
}

// Sessions is the in-memory session table.
type Sessions struct {
	mu      sync.Mutex
	now     func() time.Time
	entries map[string]*sessionEntry
	seq     uint64
}

// NewSessions returns an empty table that reads time from now.
func NewSessions(now func() time.Time) *Sessions {
	return &Sessions{now: now, entries: map[string]*sessionEntry{}}
}

// Create starts a session, evicting the least recently used one at the cap.
func (m *Sessions) Create() (Session, error) {
	token, err := randomToken()
	if err != nil {
		return Session{}, err
	}
	csrf, err := randomToken()
	if err != nil {
		return Session{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for len(m.entries) >= MaxSessions {
		var oldest *sessionEntry
		for _, e := range m.entries {
			if oldest == nil || e.seq < oldest.seq {
				oldest = e
			}
		}
		delete(m.entries, oldest.Token)
	}
	e := &sessionEntry{Session: Session{Token: token, CSRFToken: csrf}}
	m.touch(e)
	m.entries[token] = e
	return e.Session, nil
}

// Lookup returns the session for token and extends its life (sliding
// expiry). An expired session is removed.
func (m *Sessions) Lookup(token string) (Session, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.entries[token]
	if !ok {
		return Session{}, false
	}
	if !m.now().Before(e.lastSeen.Add(SessionTTL)) {
		delete(m.entries, token)
		return Session{}, false
	}
	m.touch(e)
	return e.Session, true
}

// Delete ends the session with token, if any.
func (m *Sessions) Delete(token string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.entries, token)
}

// DeleteAllExcept ends every session but keep.
func (m *Sessions) DeleteAllExcept(keep string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for token := range m.entries {
		if token != keep {
			delete(m.entries, token)
		}
	}
}

// touch marks e as just used. Callers hold m.mu.
func (m *Sessions) touch(e *sessionEntry) {
	m.seq++
	e.seq = m.seq
	e.lastSeen = m.now()
}

func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
