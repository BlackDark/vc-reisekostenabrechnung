package httpapi

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type limiter struct {
	mu sync.Mutex
	m  map[string]*rate.Limiter
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
	lim, ok := l.m[key]
	if !ok {
		lim = rate.NewLimiter(rate.Every(per/time.Duration(count)), count)
		l.m[key] = lim
	}
	return lim.Allow()
}
