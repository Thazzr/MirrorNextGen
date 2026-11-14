package auth

import (
	"testing"
	"time"
)

func TestThrottleBlocksAfterFiveFailures(t *testing.T) {
	clock := newClock()
	th := NewThrottle(clock.now)
	for i := range MaxFailedLogins {
		if _, ok := th.Attempt("1.2.3.4"); !ok {
			t.Fatalf("attempt %d blocked", i+1)
		}
		clock.add(time.Minute)
	}
	// First attempt was 5 minutes ago, so 10 minutes remain.
	wait, ok := th.Attempt("1.2.3.4")
	if ok || wait != 10*time.Minute {
		t.Fatalf("Attempt = %v, %v; want blocked for 10m", wait, ok)
	}
	// Being blocked must not extend the block.
	clock.add(time.Minute)
	if wait, ok := th.Attempt("1.2.3.4"); ok || wait != 9*time.Minute {
		t.Fatalf("Attempt = %v, %v; want blocked for 9m", wait, ok)
	}
}

func TestThrottleWindowSlides(t *testing.T) {
	clock := newClock()
	th := NewThrottle(clock.now)
	for range MaxFailedLogins {
		th.Attempt("ip")
		clock.add(time.Minute) // failures at t=0..4, now t=5
	}
	clock.add(ThrottleWindow - 5*time.Minute - time.Second) // t = 15m - 1s
	if _, ok := th.Attempt("ip"); ok {
		t.Fatal("oldest failure left the window too early")
	}
	clock.add(time.Second) // t = 15m: the failure at t=0 is out
	if _, ok := th.Attempt("ip"); !ok {
		t.Fatal("still blocked after the oldest failure left the window")
	}
	// That attempt counts again: 4 old + 1 new = 5, so blocked right away.
	if _, ok := th.Attempt("ip"); ok {
		t.Fatal("window did not keep the newer failures")
	}
}

func TestThrottleSuccessResets(t *testing.T) {
	th := NewThrottle(newClock().now)
	for range MaxFailedLogins - 1 {
		th.Attempt("ip")
	}
	th.Succeed("ip")
	for i := range MaxFailedLogins {
		if _, ok := th.Attempt("ip"); !ok {
			t.Fatalf("attempt %d blocked after a successful login", i+1)
		}
	}
}

func TestThrottleIPsAreIndependent(t *testing.T) {
	th := NewThrottle(newClock().now)
	for range MaxFailedLogins {
		th.Attempt("a")
	}
	if _, ok := th.Attempt("a"); ok {
		t.Fatal("a must be blocked")
	}
	if _, ok := th.Attempt("b"); !ok {
		t.Fatal("b must not be affected by a")
	}
	th.Succeed("b")
	if _, ok := th.Attempt("a"); ok {
		t.Fatal("success of b must not clear a")
	}
}

func TestThrottleForgetsIdleKeys(t *testing.T) {
	clock := newClock()
	th := NewThrottle(clock.now)
	th.Attempt("old")
	clock.add(ThrottleWindow)
	th.Attempt("new")
	if _, ok := th.attempts["old"]; ok {
		t.Error("expired key was not swept")
	}
}
