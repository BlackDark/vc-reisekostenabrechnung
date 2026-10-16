package httpapi

import (
	"fmt"
	"testing"
	"time"
)

// The limiter holds one entry per key, and keys are client addresses and user
// IDs, so the map has to shed the keys that are idle again or it grows for the
// life of the process. A bucket that has refilled is indistinguishable from one
// that was never created, which is what makes dropping it safe; a bucket that is
// still spending tokens has to stay, or its caller would get a fresh burst.
func TestLimiterShedsIdleKeysAndKeepsBusyOnes(t *testing.T) {
	l := limiter{}
	const window = time.Hour
	if !l.allow("busy", 1, window) {
		t.Fatal("the first request of a window was refused")
	}
	// A one-nanosecond window refills immediately, so these keys are idle again by
	// the time the sweep reaches them.
	for i := range limiterMaxKeys + 1 {
		l.allow(fmt.Sprintf("idle-%d", i), 1, time.Nanosecond)
	}

	if got := len(l.m); got > limiterMaxKeys {
		t.Fatalf("the limiter kept %d keys, so idle ones are never shed", got)
	}
	if l.allow("busy", 1, window) {
		t.Fatal("the sweep handed an in-use key a second request in its window")
	}
}
