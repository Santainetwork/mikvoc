package middleware

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func TestLoginLimiterAllowsGET(t *testing.T) {
	lim := NewLoginLimiter()
	called := false
	h := lim.Wrap(func(w http.ResponseWriter, r *http.Request) { called = true })
	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if !called {
		t.Fatal("GET should not be rate-limited")
	}
}

func TestLoginLimiterBlocksAfterBurst(t *testing.T) {
	lim := NewLoginLimiter()
	hits := 0
	h := lim.Wrap(func(w http.ResponseWriter, r *http.Request) { hits++ })
	blocked := 0
	for i := 0; i < 20; i++ {
		req := httptest.NewRequest(http.MethodPost, "/login", nil)
		req.RemoteAddr = "1.2.3.4:1234"
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code == http.StatusTooManyRequests {
			blocked++
		}
	}
	if blocked == 0 {
		t.Fatalf("expected some requests blocked, got %d", blocked)
	}
	if hits > 5 {
		t.Fatalf("expected max 5 hits (burst), got %d", hits)
	}
}

func TestLoginLimiterSeparatesIPs(t *testing.T) {
	lim := NewLoginLimiter()
	hits := 0
	h := lim.Wrap(func(w http.ResponseWriter, r *http.Request) { hits++ })
	for _, ip := range []string{"1.1.1.1:1", "2.2.2.2:2", "3.3.3.3:3"} {
		for i := 0; i < 3; i++ {
			req := httptest.NewRequest(http.MethodPost, "/login", nil)
			req.RemoteAddr = ip
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)
		}
	}
	if hits != 9 {
		t.Fatalf("expected 9 hits (3 IPs * 3), got %d", hits)
	}
}

func TestClientIPFromXForwardedFor(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "9.9.9.9, 10.0.0.1")
	if got := clientIP(req); got != "9.9.9.9" {
		t.Fatalf("got %q", got)
	}
}

func TestLoginLimiterConcurrent(t *testing.T) {
	lim := NewLoginLimiter()
	var wg sync.WaitGroup
	var mu sync.Mutex
	blocked := 0
	h := lim.Wrap(func(w http.ResponseWriter, r *http.Request) {})
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodPost, "/login", nil)
			req.RemoteAddr = "5.5.5.5:5"
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)
			if w.Code == http.StatusTooManyRequests {
				mu.Lock()
				blocked++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if blocked == 0 {
		t.Fatal("expected some concurrent requests blocked")
	}
}
