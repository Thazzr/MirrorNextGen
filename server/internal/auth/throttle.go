package auth

import (
	"sync"
	"time"
)

const (
	// MaxFailedLogins failed attempts within ThrottleWindow block a client.
	MaxFailedLogins = 5
	ThrottleWindow  = 15 * time.Minute
)

// Throttle limits password guessing per client key (the IP address).
type Throttle struct {
	mu       sync.Mutex
	now      func() time.Time
	attempts map[string][]time.Time
}

// NewThrottle returns a throttle that reads time from now.
func NewThrottle(now func() time.Time) *Throttle {
	return &Throttle{now: now, attempts: map[string][]time.Time{}}
}

// Attempt registers a login attempt for key before the password is checked.
// If the key is blocked it returns the time to wait and ok=false and records
// nothing. Recording up front (and clearing on success) instead of recording
// after a failed check keeps parallel requests from all slipping past the
// limit before the first failure is noted.
func (t *Throttle) Attempt(key string) (retryAfter time.Duration, ok bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := t.now()
	t.sweep(now)
	recent := t.attempts[key]
	if len(recent) >= MaxFailedLogins {
		return recent[0].Add(ThrottleWindow).Sub(now), false
	}
	t.attempts[key] = append(recent, now)
	return 0, true
}

// Succeed forgets the attempts of key after a correct password.
func (t *Throttle) Succeed(key string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.attempts, key)
}

// sweep drops attempts that left the window and keys without attempts, so
// the map cannot grow with every address that ever failed. Callers hold t.mu.
func (t *Throttle) sweep(now time.Time) {
	for key, list := range t.attempts {
		i := 0
		for i < len(list) && !now.Before(list[i].Add(ThrottleWindow)) {
			i++
		}
		if i == len(list) {
			delete(t.attempts, key)
		} else if i > 0 {
			t.attempts[key] = list[i:]
		}
	}
}
