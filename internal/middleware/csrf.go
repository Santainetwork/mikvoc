package middleware

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
)

const (
	CSRFCookieName = "mikvoc_csrf"
	CSRFHeaderName = "X-CSRF-Token"
	CSRFFormField  = "csrf_token"
	csrfTokenBytes = 32
	csrfSessionKey = "csrf_token"
)

func CSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if csrfSafeMethod(r.Method) || csrfExemptPath(r.URL.Path) {
			_ = EnsureCSRFToken(w, r)
			next.ServeHTTP(w, r)
			return
		}

		// Unsafe methods: never mint a new token before validation.
		expected := readCSRFToken(r)
		if !csrfTokenValid(r, expected) {
			writeCSRFError(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// EnsureCSRFToken returns the session-backed CSRF token, creating it if needed,
// and mirrors it into a readable cookie for JS.
// If the session is missing the token but the cookie has one, the cookie value
// is promoted into the session so later sess.Save() calls keep it.
func EnsureCSRFToken(w http.ResponseWriter, r *http.Request) string {
	if sessTok := sessionCSRFToken(r); sessTok != "" {
		setCSRFCookie(w, r, sessTok)
		return sessTok
	}
	if cookieTok := cookieCSRFToken(r); cookieTok != "" {
		// Promote cookie → session so auth/login Save does not drop CSRF.
		writeCSRFToken(w, r, cookieTok)
		return cookieTok
	}
	token := newCSRFToken()
	writeCSRFToken(w, r, token)
	return token
}

func sessionCSRFToken(r *http.Request) string {
	if Store == nil {
		return ""
	}
	sess, err := Store.Get(r, SessionName)
	if err != nil {
		return ""
	}
	t, ok := sess.Values[csrfSessionKey].(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(t)
}

func cookieCSRFToken(r *http.Request) string {
	c, err := r.Cookie(CSRFCookieName)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(c.Value)
}

func readCSRFToken(r *http.Request) string {
	if t := sessionCSRFToken(r); t != "" {
		return t
	}
	return cookieCSRFToken(r)
}

func writeCSRFToken(w http.ResponseWriter, r *http.Request, token string) {
	if Store != nil {
		if sess, err := Store.Get(r, SessionName); err == nil {
			sess.Values[csrfSessionKey] = token
			_ = sess.Save(r, w)
		}
	}
	setCSRFCookie(w, r, token)
	// So later reads in the same request see the token.
	r.AddCookie(&http.Cookie{Name: CSRFCookieName, Value: token})
}

func setCSRFCookie(w http.ResponseWriter, r *http.Request, token string) {
	secure := r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
	http.SetCookie(w, &http.Cookie{
		Name:     CSRFCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   86400 * 7,
		HttpOnly: true, // Changed from false to true for security
		SameSite: http.SameSiteStrictMode, // Changed from Lax to Strict for security
		Secure:   secure,
	})
}

func newCSRFToken() string {
	b := make([]byte, csrfTokenBytes)
	if _, err := rand.Read(b); err != nil {
		panic("csrf: crypto/rand failed: " + err.Error())
	}
	return hex.EncodeToString(b)
}

func csrfSafeMethod(method string) bool {
	switch strings.ToUpper(method) {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return true
	default:
		return false
	}
}

func csrfExemptPath(path string) bool {
	switch path {
	case "/login", "/logout", "/healthz", "/metrics":
		return true
	}
	if strings.HasPrefix(path, "/static/") {
		return true
	}
	return false
}

func csrfTokenValid(r *http.Request, expected string) bool {
	expected = strings.TrimSpace(expected)
	if expected == "" {
		return false
	}

	provided := firstNonEmpty(
		r.Header.Get(CSRFHeaderName),
		r.Header.Get("X-CSRF-TOKEN"),
		r.Header.Get("X-XSRF-TOKEN"),
	)
	if provided == "" {
		provided = csrfTokenFromBody(r)
	}
	provided = strings.TrimSpace(provided)
	if provided == "" {
		return false
	}
	if len(provided) != len(expected) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) == 1
}

func csrfTokenFromBody(r *http.Request) string {
	ct := r.Header.Get("Content-Type")
	if strings.HasPrefix(ct, "multipart/form-data") {
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			return ""
		}
		return r.FormValue(CSRFFormField)
	}
	if err := r.ParseForm(); err != nil {
		return ""
	}
	return r.FormValue(CSRFFormField)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func writeCSRFError(w http.ResponseWriter, r *http.Request) {
	accept := r.Header.Get("Accept")
	xrw := r.Header.Get("X-Requested-With")
	if strings.Contains(accept, "application/json") ||
		strings.EqualFold(xrw, "fetch") ||
		strings.EqualFold(xrw, "XMLHttpRequest") {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"ok":false,"message":"CSRF token invalid or expired. Reload page and try again."}`))
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusForbidden)
	_, _ = w.Write([]byte(`<!DOCTYPE html><html><head><meta charset="utf-8"><title>CSRF</title></head><body style="font-family:system-ui;padding:2rem">
<h1>403 — CSRF token invalid</h1>
<p>The security token does not match or has expired. <a href="javascript:location.reload()">Reload page</a> and try again.</p>
<p><a href="/">Back</a></p>
</body></html>`))
}

// GetCSRFTokenHandler returns a new or existing CSRF token without requiring authentication.
// This allows frontend to fetch the token before submitting forms.
func GetCSRFTokenHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := EnsureCSRFToken(w, r)
		
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf(`{"csrf_token":%q}`, token)))
	})
}
