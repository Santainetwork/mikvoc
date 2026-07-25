package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"mikvoc/internal/middleware"
	"mikvoc/internal/routeros"
	"mikvoc/internal/service"
)

func TestIsExpiredHotspotUserUsesMikhmonExpiryComment(t *testing.T) {
	now := time.Now()
	if !routeros.IsExpiredHotspotUser(routeros.HotspotUser{Comment: "2000-01-01 00:00:00"}, now) {
		t.Fatal("expected past expiry comment to be treated as expired")
	}

	if routeros.IsExpiredHotspotUser(routeros.HotspotUser{Comment: "2999-01-01 00:00:00"}, now) {
		t.Fatal("expected future expiry comment not to be treated as expired")
	}

	if routeros.IsExpiredHotspotUser(routeros.HotspotUser{Comment: "vc-05.04.26"}, now) {
		t.Fatal("expected batch voucher comment not to be treated as expired")
	}
}

func TestPageHotspotUsers(t *testing.T) {
	users := make([]routeros.HotspotUser, 5)
	for i := range users {
		users[i] = routeros.HotspotUser{ID: string(rune('a' + i)), Name: string(rune('a' + i))}
	}
	page := service.PageSlice(users, 2, 1)
	if len(page) != 2 {
		t.Fatalf("len=%d want 2", len(page))
	}
	if page[0].Name != "b" || page[1].Name != "c" {
		t.Fatalf("got %s,%s", page[0].Name, page[1].Name)
	}
	empty := service.PageSlice(users, 10, 100)
	if len(empty) != 0 {
		t.Fatalf("want empty got %d", len(empty))
	}
}

func TestParseUsersJSONPagination(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/hotspot/users", nil)
	limit, offset := parseUsersJSONPagination(req)
	if limit != usersJSONDefaultLimit || offset != 0 {
		t.Fatalf("default limit=%d offset=%d", limit, offset)
	}
	req = httptest.NewRequest(http.MethodGet, "/api/hotspot/users?limit=50&offset=10", nil)
	limit, offset = parseUsersJSONPagination(req)
	if limit != 50 || offset != 10 {
		t.Fatalf("got limit=%d offset=%d", limit, offset)
	}
	req = httptest.NewRequest(http.MethodGet, "/api/hotspot/users?limit=9999", nil)
	limit, _ = parseUsersJSONPagination(req)
	if limit != usersJSONMaxLimit {
		t.Fatalf("cap limit=%d want %d", limit, usersJSONMaxLimit)
	}
}

func TestHandleUsersJSONUnavailable(t *testing.T) {
	middleware.InitSession("test-secret-users-json")
	app := NewApp(nil, nil, nil, nil, nil, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/hotspot/users?limit=10&offset=0", nil)
	rr := httptest.NewRecorder()
	app.HandleUsersJSON(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d want 503", rr.Code)
	}
}

func TestUsersJSONEnvelopeShape(t *testing.T) {
	payload := map[string]any{
		"ok":     true,
		"total":  3,
		"limit":  2,
		"offset": 1,
		"users":  []routeros.HotspotUser{{Name: "a"}, {Name: "b"}},
	}
	b, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"ok", "total", "limit", "offset", "users"} {
		if _, ok := got[k]; !ok {
			t.Fatalf("missing key %s", k)
		}
	}
}
