package middleware

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSecurityHeaders(t *testing.T) {
	h := SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if got := w.Header().Get("X-Frame-Options"); got != "DENY" {
		t.Fatalf("X-Frame-Options: %q", got)
	}
	if got := w.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("X-Content-Type-Options: %q", got)
	}
	if got := w.Header().Get("Referrer-Policy"); got != "strict-origin-when-cross-origin" {
		t.Fatalf("Referrer-Policy: %q", got)
	}
	if got := w.Header().Get("X-XSS-Protection"); got != "0" {
		t.Fatalf("X-XSS-Protection: %q", got)
	}
}

func TestRequestIDHeader(t *testing.T) {
	h := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if GetRequestID(r) == "" {
			t.Error("request id missing from context")
		}
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if got := w.Header().Get("X-Request-ID"); got == "" {
		t.Fatal("X-Request-ID missing")
	}
	if len(w.Header().Get("X-Request-ID")) != 32 {
		t.Fatalf("unexpected request id length: %s", w.Header().Get("X-Request-ID"))
	}
}

func TestRequestIDPassthrough(t *testing.T) {
	h := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", "abc123custom")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if got := w.Header().Get("X-Request-ID"); got != "abc123custom" {
		t.Fatalf("got %q", got)
	}
}

func TestCSRFAllowsGET(t *testing.T) {
	InitSession("csrf-test-secret-key-32bytes!!")
	called := false
	h := CSRF(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/users", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if !called {
		t.Fatal("GET should pass")
	}
	if w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}
	found := false
	for _, c := range w.Result().Cookies() {
		if c.Name == CSRFCookieName && c.Value != "" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected CSRF cookie on GET")
	}
}

func TestCSRFRejectsPOSTWithoutToken(t *testing.T) {
	h := CSRF(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodPost, "/users", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestCSRFAllowsPOSTWithHeader(t *testing.T) {
	InitSession("csrf-test-secret-key-32bytes!!")
	token := "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
	// Seed session token via GET
	getReq := httptest.NewRequest(http.MethodGet, "/users", nil)
	getW := httptest.NewRecorder()
	CSRF(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// force known token into session
		sess, _ := Store.Get(r, SessionName)
		sess.Values[csrfSessionKey] = token
		_ = sess.Save(r, w)
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(getW, getReq)
	var sessionCookie *http.Cookie
	for _, c := range getW.Result().Cookies() {
		if c.Name == SessionName {
			sessionCookie = c
		}
	}

	h := CSRF(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodPost, "/users", nil)
	if sessionCookie != nil {
		req.AddCookie(sessionCookie)
	}
	req.AddCookie(&http.Cookie{Name: CSRFCookieName, Value: token})
	req.Header.Set(CSRFHeaderName, token)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestCSRFAllowsPOSTWithFormField(t *testing.T) {
	InitSession("csrf-test-secret-key-32bytes!!")
	token := "aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899"
	getReq := httptest.NewRequest(http.MethodGet, "/settings", nil)
	getW := httptest.NewRecorder()
	CSRF(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess, _ := Store.Get(r, SessionName)
		sess.Values[csrfSessionKey] = token
		_ = sess.Save(r, w)
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(getW, getReq)
	var sessionCookie *http.Cookie
	for _, c := range getW.Result().Cookies() {
		if c.Name == SessionName {
			sessionCookie = c
		}
	}

	h := CSRF(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	body := strings.NewReader(CSRFFormField + "=" + token)
	req := httptest.NewRequest(http.MethodPost, "/settings", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if sessionCookie != nil {
		req.AddCookie(sessionCookie)
	}
	req.AddCookie(&http.Cookie{Name: CSRFCookieName, Value: token})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestCSRFExemptLogin(t *testing.T) {
	called := false
	h := CSRF(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodPost, "/login", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if !called || w.Code != http.StatusOK {
		t.Fatalf("login POST should be exempt, called=%v code=%d", called, w.Code)
	}
}

func TestCSRFExemptHealthz(t *testing.T) {
	h := CSRF(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodPost, "/healthz", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("healthz should be exempt, got %d", w.Code)
	}
}

func TestCSRFExemptLogout(t *testing.T) {
	h := CSRF(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodPost, "/logout", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("logout should be exempt, got %d", w.Code)
	}
}

func TestCSRFAllowsMultipartForm(t *testing.T) {
	InitSession("csrf-test-secret-key-32bytes!!")
	token := "aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899"
	getReq := httptest.NewRequest(http.MethodGet, "/", nil)
	getW := httptest.NewRecorder()
	CSRF(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess, _ := Store.Get(r, SessionName)
		sess.Values[csrfSessionKey] = token
		_ = sess.Save(r, w)
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(getW, getReq)
	var sessionCookie *http.Cookie
	for _, c := range getW.Result().Cookies() {
		if c.Name == SessionName {
			sessionCookie = c
		}
	}

	h := CSRF(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	_ = mw.WriteField(CSRFFormField, token)
	_ = mw.WriteField("csv_file", "x")
	_ = mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/hotspot/users/import-csv", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	if sessionCookie != nil {
		req.AddCookie(sessionCookie)
	}
	req.AddCookie(&http.Cookie{Name: CSRFCookieName, Value: token})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 multipart, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestTemplateUploadLimitRunsBeforeCSRF(t *testing.T) {
	InitSession("template-limit-test-secret-key!!")
	token := strings.Repeat("a", 64)
	request := func(path string, filler int) *http.Request {
		var body bytes.Buffer
		mw := multipart.NewWriter(&body)
		if err := mw.WriteField(CSRFFormField, token); err != nil {
			t.Fatal(err)
		}
		part, err := mw.CreateFormFile("tpl_assets_zip", "assets.zip")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write(bytes.Repeat([]byte("x"), filler)); err != nil {
			t.Fatal(err)
		}
		if err := mw.Close(); err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodPost, path, &body)
		req.Header.Set("Content-Type", mw.FormDataContentType())
		req.AddCookie(&http.Cookie{Name: CSRFCookieName, Value: token})
		return req
	}

	called := false
	h := TemplateUploadLimit(CSRF(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	})))

	w := httptest.NewRecorder()
	h.ServeHTTP(w, request("/template", int(TemplateUploadMaxBytes)))
	if w.Code != http.StatusRequestEntityTooLarge || called {
		t.Fatalf("oversized /template status=%d called=%v, want 413/false", w.Code, called)
	}

	called = false
	w = httptest.NewRecorder()
	h.ServeHTTP(w, request("/template", 1024))
	if w.Code != http.StatusNoContent || !called {
		t.Fatalf("small /template status=%d called=%v, want 204/true", w.Code, called)
	}

	called = false
	w = httptest.NewRecorder()
	h.ServeHTTP(w, request("/settings/restore", int(TemplateUploadMaxBytes)))
	if w.Code != http.StatusNoContent || !called {
		t.Fatalf("restore status=%d called=%v, want 204/true", w.Code, called)
	}
}

func TestCSRFPromotesCookieIntoSession(t *testing.T) {
	InitSession("csrf-test-secret-key-32bytes!!")
	token := "11223344556677889900aabbccddeeff11223344556677889900aabbccddeeff"

	// Session without csrf_token, only cookie present.
	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	req.AddCookie(&http.Cookie{Name: CSRFCookieName, Value: token})
	w := httptest.NewRecorder()
	got := EnsureCSRFToken(w, req)
	if got != token {
		t.Fatalf("expected promote cookie token, got %q", got)
	}
	sess, err := Store.Get(req, SessionName)
	if err != nil {
		t.Fatal(err)
	}
	st, _ := sess.Values[csrfSessionKey].(string)
	if st != token {
		t.Fatalf("session missing promoted csrf, got %q", st)
	}

	// POST with form token must pass using session-backed expected.
	var sessionCookie *http.Cookie
	for _, c := range w.Result().Cookies() {
		if c.Name == SessionName {
			sessionCookie = c
		}
	}
	body := strings.NewReader(CSRFFormField + "=" + token)
	post := httptest.NewRequest(http.MethodPost, "/settings", body)
	post.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if sessionCookie != nil {
		post.AddCookie(sessionCookie)
	}
	post.AddCookie(&http.Cookie{Name: CSRFCookieName, Value: token})
	postW := httptest.NewRecorder()
	CSRF(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		rw.WriteHeader(http.StatusOK)
	})).ServeHTTP(postW, post)
	if postW.Code != http.StatusOK {
		t.Fatalf("expected 200 after promote, got %d body=%s", postW.Code, postW.Body.String())
	}
}

func TestCSRFRejectsPOSTWhenTokenMissingEvenIfWouldMint(t *testing.T) {
	InitSession("csrf-test-secret-key-32bytes!!")
	// GET first to create session token
	getReq := httptest.NewRequest(http.MethodGet, "/hotspot/generate", nil)
	getW := httptest.NewRecorder()
	CSRF(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(getW, getReq)
	var cookies []*http.Cookie
	for _, c := range getW.Result().Cookies() {
		cookies = append(cookies, c)
	}
	h := CSRF(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodPost, "/hotspot/generate", strings.NewReader("qty=1"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	// no csrf_token form field, no header → 403
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 without form token, got %d", w.Code)
	}
}

func TestCSRFJSONErrorOnFetch(t *testing.T) {
	h := CSRF(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodPost, "/hotspot/users/remove", nil)
	req.Header.Set("X-Requested-With", "fetch")
	req.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Fatalf("expected json content-type, got %q", ct)
	}
	if !strings.Contains(w.Body.String(), "CSRF") {
		t.Fatalf("expected CSRF message, got %s", w.Body.String())
	}
}

func TestGzipCompressesHTML(t *testing.T) {
	body := strings.Repeat("<html>hello</html>", 50)
	h := Gzip(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(body))
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Header().Get("Content-Encoding") != "gzip" {
		t.Fatalf("expected gzip, got %q", w.Header().Get("Content-Encoding"))
	}
	if w.Body.Len() == 0 || w.Body.Len() >= len(body) {
		t.Fatalf("expected compressed body smaller than original: %d vs %d", w.Body.Len(), len(body))
	}
}
