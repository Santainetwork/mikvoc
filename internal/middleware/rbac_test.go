package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"mikvoc/internal/core"
)

func init() {
	InitSession("test-secret-rbac")
}

func withRole(r *http.Request, role core.AuditRole) *http.Request {
	sess, _ := Store.Get(r, SessionName)
	sess.Values["authenticated"] = true
	sess.Values["admin_role"] = string(role)
	sess.Values["admin_id"] = 1
	sess.Values["admin_user"] = "tester"
	_ = sess.Save(r, httptest.NewRecorder())
	return r
}

func TestRoleLevel(t *testing.T) {
	if RoleLevel(core.RoleOwner) <= RoleLevel(core.RoleOperator) {
		t.Fatal("owner should > operator")
	}
	if RoleLevel(core.RoleOperator) <= RoleLevel(core.RoleViewer) {
		t.Fatal("operator should > viewer")
	}
}

func TestBlockViewerWrites_AllowsGET(t *testing.T) {
	called := false
	h := BlockViewerWrites(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	req := httptest.NewRequest(http.MethodGet, "/hotspot/users", nil)
	req = withRole(req, core.RoleViewer)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if !called {
		t.Fatal("viewer GET should pass")
	}
}

func TestBlockViewerWrites_BlocksPOST(t *testing.T) {
	called := false
	h := BlockViewerWrites(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	req := httptest.NewRequest(http.MethodPost, "/hotspot/users/remove", nil)
	req = withRole(req, core.RoleViewer)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if called {
		t.Fatal("viewer POST should be blocked")
	}
	if w.Code != http.StatusForbidden {
		t.Fatalf("code=%d", w.Code)
	}
}

func TestBlockViewerWrites_OperatorPOST(t *testing.T) {
	called := false
	h := BlockViewerWrites(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	req := httptest.NewRequest(http.MethodPost, "/hotspot/generate", nil)
	req = withRole(req, core.RoleOperator)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if !called {
		t.Fatal("operator POST should pass")
	}
}

func TestRequireOwner_BlocksOperator(t *testing.T) {
	called := false
	h := RequireOwner(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	req := httptest.NewRequest(http.MethodGet, "/settings/admins", nil)
	req = withRole(req, core.RoleOperator)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if called {
		t.Fatal("operator should not access owner-only")
	}
	if w.Code != http.StatusForbidden {
		t.Fatalf("code=%d", w.Code)
	}
}

func TestRequireOwner_AllowsOwner(t *testing.T) {
	called := false
	h := RequireOwner(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	req := httptest.NewRequest(http.MethodGet, "/settings/admins", nil)
	req = withRole(req, core.RoleOwner)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if !called {
		t.Fatal("owner should pass")
	}
}
