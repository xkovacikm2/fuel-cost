package api

import (
	"net/http"
	"net/netip"
	"sync"
	"time"
)

type attemptWindow struct {
	started time.Time
	count   int
}

type rateLimiter struct {
	mu      sync.Mutex
	limit   int
	window  time.Duration
	clients map[string]attemptWindow
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{
		limit:   limit,
		window:  window,
		clients: make(map[string]attemptWindow),
	}
}

func (l *rateLimiter) allow(ip string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	for key, entry := range l.clients {
		if now.Sub(entry.started) >= l.window {
			delete(l.clients, key)
		}
	}
	entry := l.clients[ip]
	if entry.started.IsZero() || now.Sub(entry.started) >= l.window {
		l.clients[ip] = attemptWindow{started: now, count: 1}
		return true
	}
	if entry.count >= l.limit {
		return false
	}
	entry.count++
	l.clients[ip] = entry
	return true
}

func (l *rateLimiter) middleware(next http.Handler, trusted ...netip.Prefix) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientAddress(r, trusted)
		if !l.allow(ip, time.Now()) {
			writeError(w, http.StatusTooManyRequests, "Skúste to neskôr.")
			return
		}
		next.ServeHTTP(w, r)
	})
}
