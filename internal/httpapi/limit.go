package httpapi

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

const (
	// limiterMaxKeys is the size at which the sweep starts looking. There is one
	// entry per client address and per user, and an instance reachable from the
	// internet sees a new address with every scan, so the map cannot be left to
	// grow for the life of the process.
	limiterMaxKeys = 4096
	// limiterSweepEvery is the shortest gap between two sweeps. A map that is
	// large and cannot shed anything would otherwise turn every request into a
	// walk over all of it.
	limiterSweepEvery = time.Minute
)

type limiter struct {
	mu      sync.Mutex
	m       map[string]*rate.Limiter
	sweptAt time.Time
}

func (l *limiter) allow(key string, count int, per time.Duration) bool {
	if count < 1 || per <= 0 {
		return true
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.m == nil {
		l.m = map[string]*rate.Limiter{}
	}
	if now := time.Now(); len(l.m) > limiterMaxKeys && now.Sub(l.sweptAt) >= limiterSweepEvery {
		l.sweep(now)
	}
	lim, ok := l.m[key]
	if !ok {
		lim = rate.NewLimiter(rate.Every(per/time.Duration(count)), count)
		l.m[key] = lim
	}
	return lim.Allow()
}

// sweep forgets the keys whose bucket is full again. Such an entry behaves like
// one that was never created, so dropping it gives no caller anything it would
// not have had; an entry that is still spending tokens is kept, because dropping
// it would hand back a burst the caller has not earned.
func (l *limiter) sweep(now time.Time) {
	l.sweptAt = now
	for key, lim := range l.m {
		if lim.Tokens() >= float64(lim.Burst()) {
			delete(l.m, key)
		}
	}
}
