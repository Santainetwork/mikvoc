package service

import (
	"testing"
	"time"

	"mikvoc/internal/core"
	"mikvoc/internal/routeros"
)

func sampleUsers() []routeros.HotspotUser {
	return []routeros.HotspotUser{
		{ID: "*1", Name: "alice", Profile: "1hari", Comment: "vc-05.04.26", Uptime: "0s", BytesIn: "0", BytesOut: "0"},
		{ID: "*2", Name: "bob", Profile: "2hari", Comment: "2000-01-01 00:00:00", Uptime: "1h", BytesIn: "100", BytesOut: "200"},
		{ID: "*3", Name: "carol", Profile: "1hari", Comment: "2999-01-01 00:00:00", Uptime: "0s", BytesIn: "0", BytesOut: "0"},
		{ID: "*4", Name: "dave", Profile: "vip", Comment: "special", Uptime: "5m", BytesIn: "1", BytesOut: "1"},
	}
}

func TestFilterUsers_ByProfile(t *testing.T) {
	got := filterUsers(sampleUsers(), core.UserFilters{Profile: "1hari"})
	if len(got) != 2 {
		t.Fatalf("len=%d want 2", len(got))
	}
	for _, u := range got {
		if u.Profile != "1hari" {
			t.Fatalf("unexpected profile %q", u.Profile)
		}
	}
}

func TestFilterUsers_BySearch(t *testing.T) {
	got := filterUsers(sampleUsers(), core.UserFilters{Search: "ali"})
	if len(got) != 1 || got[0].Name != "alice" {
		t.Fatalf("got=%v", got)
	}
}

func TestFilterUsers_Expired(t *testing.T) {
	got := filterUsers(sampleUsers(), core.UserFilters{Expired: true})
	if len(got) != 1 || got[0].Name != "bob" {
		t.Fatalf("got=%v want bob only", got)
	}
}

func TestFilterUsers_ByIDsOrder(t *testing.T) {
	got := filterUsers(sampleUsers(), core.UserFilters{IDs: []string{"*4", "*1"}})
	if len(got) != 2 {
		t.Fatalf("len=%d", len(got))
	}
	if got[0].ID != "*4" || got[1].ID != "*1" {
		t.Fatalf("order wrong: %s, %s", got[0].ID, got[1].ID)
	}
}

func TestFilterUsers_ByComment(t *testing.T) {
	got := filterUsers(sampleUsers(), core.UserFilters{Comment: "special"})
	if len(got) != 1 || got[0].Name != "dave" {
		t.Fatalf("got=%v", got)
	}
}

func TestIsExpiredUser(t *testing.T) {
	now := time.Date(2020, 1, 1, 0, 0, 0, 0, time.Local)
	if !routeros.IsExpiredHotspotUser(routeros.HotspotUser{Comment: "2000-01-01 00:00:00"}, now) {
		t.Fatal("expected expired")
	}
	if routeros.IsExpiredHotspotUser(routeros.HotspotUser{Comment: "2999-01-01 00:00:00"}, now) {
		t.Fatal("expected not expired")
	}
	if routeros.IsExpiredHotspotUser(routeros.HotspotUser{Comment: "vc-05.04.26"}, now) {
		t.Fatal("batch comment should not expire")
	}
	if !routeros.IsExpiredHotspotUser(routeros.HotspotUser{LimitUptime: "1s"}, now) {
		t.Fatal("limit-uptime 1s is expired")
	}
}

func TestIsUnusedUser(t *testing.T) {
	if !routeros.IsUnusedHotspotUser(routeros.HotspotUser{Uptime: "0s", BytesIn: "0", BytesOut: "0"}) {
		t.Fatal("expected unused")
	}
	if routeros.IsUnusedHotspotUser(routeros.HotspotUser{Uptime: "1h", BytesIn: "0", BytesOut: "0"}) {
		t.Fatal("expected used")
	}
}

func TestContainsIgnoreCase(t *testing.T) {
	if !core.ContainsIgnoreCase("Alice", "ali") {
		t.Fatal("should match")
	}
	if core.ContainsIgnoreCase("bob", "alice") {
		t.Fatal("should not match")
	}
}

func TestPageSlice(t *testing.T) {
	items := []int{1, 2, 3, 4, 5}
	got := PageSlice(items, 2, 1)
	if len(got) != 2 || got[0] != 2 || got[1] != 3 {
		t.Fatalf("got=%v", got)
	}
	got = PageSlice(items, 0, 0)
	if len(got) != 5 {
		t.Fatalf("limit 0 should return all, got %d", len(got))
	}
	got = PageSlice(items, 10, 10)
	if len(got) != 0 {
		t.Fatalf("want empty got %v", got)
	}
}

func TestClampLimitOffset(t *testing.T) {
	if ClampLimit(0, 100, 500) != 100 {
		t.Fatal("default")
	}
	if ClampLimit(999, 100, 500) != 500 {
		t.Fatal("max")
	}
	if ClampLimit(50, 100, 500) != 50 {
		t.Fatal("ok")
	}
	if ClampOffset(-3) != 0 {
		t.Fatal("offset")
	}
}
