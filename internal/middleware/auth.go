package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/sessions"
	"golang.org/x/time/rate"
)

var Store *sessions.CookieStore

const SessionName = "mikvoc_session"

func InitSession(secret string) {
	Store = sessions.NewCookieStore([]byte(secret))
	Store.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   86400 * 7, // 7 days
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}
}

// RequireAuth middleware: redirects to /login if no valid session.
func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess, err := Store.Get(r, SessionName)
		if err != nil || sess.Values["authenticated"] != true {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// LoginLimiter membatasi percobaan login per IP: 5 per menit, burst 5.
type LoginLimiter struct {
	mu       sync.Mutex
	visitors map[string]*visitorEntry
	rate     rate.Limit
	burst    int
	ttl      time.Duration
}

type visitorEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

func NewLoginLimiter() *LoginLimiter {
	l := &LoginLimiter{
		visitors: make(map[string]*visitorEntry),
		rate:     rate.Every(time.Minute / 5), // 5 per minute
		burst:    5,
		ttl:      5 * time.Minute,
	}
	go l.cleanup()
	return l
}

func (l *LoginLimiter) Wrap(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			ip := clientIP(r)
			if !l.allow(ip) {
				w.Header().Set("Retry-After", "60")
				http.Error(w, "Terlalu banyak percobaan login. Coba lagi dalam 1 menit.", http.StatusTooManyRequests)
				return
			}
		}
		next.ServeHTTP(w, r)
	}
}

func (l *LoginLimiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	v, exists := l.visitors[ip]
	if !exists {
		v = &visitorEntry{limiter: rate.NewLimiter(l.rate, l.burst)}
		l.visitors[ip] = v
	}
	v.lastSeen = time.Now()
	return v.limiter.Allow()
}

func (l *LoginLimiter) cleanup() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		l.mu.Lock()
		for ip, v := range l.visitors {
			if time.Since(v.lastSeen) > l.ttl {
				delete(l.visitors, ip)
			}
		}
		l.mu.Unlock()
	}
}

func clientIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		for i := 0; i < len(fwd); i++ {
			if fwd[i] == ',' {
				return fwd[:i]
			}
		}
		return fwd
	}
	if r.RemoteAddr == "" {
		return "unknown"
	}
	for i := len(r.RemoteAddr) - 1; i >= 0; i-- {
		if r.RemoteAddr[i] == ':' {
			return r.RemoteAddr[:i]
		}
	}
	return r.RemoteAddr
}
