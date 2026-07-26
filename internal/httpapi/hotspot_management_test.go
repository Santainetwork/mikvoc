package httpapi

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gorilla/mux"

	"mikvoc/internal/core"
	"mikvoc/internal/middleware"
	"mikvoc/internal/service"
)

func TestManagementFormValues(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(url.Values{
		"id": {"*1"}, "mac_address": {"AA:BB:CC:DD:EE:FF"}, "address": {"10.0.0.2"},
		"to_address": {"10.0.0.9"}, "server": {"hotspot1"}, "type": {"blocked"},
		"comment": {"uji"}, "disabled": {"on"}, "name": {"hs1"}, "interface": {"bridge1"},
		"address_pool": {"pool1"}, "profile": {"default"}, "idle_timeout": {"5m"},
		"keepalive_timeout": {"2m"}, "hotspot_address": {"10.0.0.1"}, "dns_name": {"login.test"},
		"html_directory": {"hotspot"}, "login_by": {"cookie", "http-chap"},
		"cookie_lifetime": {"3d"}, "rate_limit": {"10M/10M"},
	}.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	b := ipBindingFromRequest(r)
	if b != (core.IPBinding{ID: "*1", MACAddress: "AA:BB:CC:DD:EE:FF", Address: "10.0.0.2", ToAddress: "10.0.0.9", Server: "hotspot1", Type: "blocked", Comment: "uji", Disabled: true}) {
		t.Fatalf("binding = %#v", b)
	}
	s := hotspotServerFromRequest(r)
	if s.ID != "*1" || s.Name != "hs1" || s.Interface != "bridge1" || s.AddressPool != "pool1" || s.Profile != "default" || !s.Disabled {
		t.Fatalf("server = %#v", s)
	}
	p := hotspotServerProfileFromRequest(r)
	if p.ID != "*1" || p.LoginBy != "cookie,http-chap" || p.HotspotAddress != "10.0.0.1" || p.RateLimit != "10M/10M" {
		t.Fatalf("profile = %#v", p)
	}
}

func TestFilterHosts(t *testing.T) {
	hosts := []core.HotspotHost{{ID: "1", Server: "hs-a", MACAddress: "AA"}, {ID: "2", Server: "hs-b", Address: "10.0.0.2"}}
	got := filterHosts(hosts, "hs-b", "10.0")
	if len(got) != 1 || got[0].ID != "2" {
		t.Fatalf("hosts = %#v", got)
	}
}

func TestServerProfileFormKeepsAllLoginMethods(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/hotspot/server-profiles/create", strings.NewReader("name=default&login_by=cookie&login_by=http-chap"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if got := hotspotServerProfileFromRequest(r).LoginBy; got != "cookie,http-chap" {
		t.Fatalf("LoginBy = %q", got)
	}
}

func TestManagementRoutesAndSystemLogRole(t *testing.T) {
	withTestDB(t)
	middleware.InitSession("management-route-secret")
	app := NewApp(nil, nil, nil, nil, nil, nil, nil)
	router := mux.NewRouter()
	app.RegisterRoutes(router)

	for _, path := range []string{"/hotspot/hosts", "/hotspot/ip-bindings", "/hotspot/cookies", "/hotspot/servers", "/hotspot/server-profiles"} {
		req := authenticatedManagementRequest(t, path, core.RoleViewer)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("GET %s = %d, body %q", path, rec.Code, rec.Body.String())
		}
		if strings.Contains(rec.Body.String(), `method="POST"`) {
			t.Errorf("viewer GET %s exposes mutation form", path)
		}
	}

	req := authenticatedManagementRequest(t, "/system/log", core.RoleViewer)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("viewer GET /system/log = %d", rec.Code)
	}
	req = authenticatedManagementRequest(t, "/system/log?limit=999", core.RoleOperator)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `value="200" selected`) {
		t.Fatalf("operator log = %d, body %q", rec.Code, rec.Body.String())
	}

	req = authenticatedManagementRequest(t, "/hotspot/cookies/remove", core.RoleViewer)
	req.Method = http.MethodPost
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("viewer POST = %d", rec.Code)
	}

	app.RouterManagement = service.NewRouterManagement(nil)
	req = authenticatedManagementRequest(t, "/hotspot/ip-bindings/create", core.RoleOperator)
	req.Method = http.MethodPost
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/hotspot/ip-bindings" {
		t.Fatalf("invalid POST = %d Location %q", rec.Code, rec.Header().Get("Location"))
	}
}

func authenticatedManagementRequest(t *testing.T, path string, role core.AuditRole) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	sess, err := middleware.Store.Get(req, middleware.SessionName)
	if err != nil {
		t.Fatal(err)
	}
	sess.Values["authenticated"] = true
	sess.Values["admin_role"] = string(role)
	sess.Values["router_id"] = 0
	if err := sess.Save(req, rec); err != nil {
		t.Fatal(err)
	}
	for _, cookie := range rec.Result().Cookies() {
		req.AddCookie(cookie)
	}
	return req
}
