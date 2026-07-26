package service

import (
	"errors"
	"testing"

	"mikvoc/internal/core"
)

func TestRouterManagementValidationBeforePoolLookup(t *testing.T) {
	svc := NewRouterManagement(NewPool())
	checks := []struct {
		name string
		fn   func() error
	}{
		{"host id", func() error { return svc.MakeHostBinding(1, " ", "regular") }},
		{"host binding type", func() error { return svc.MakeHostBinding(1, "*1", "invalid") }},
		{"binding name fields", func() error { return svc.AddIPBinding(1, core.IPBinding{}) }},
		{"binding type", func() error {
			return svc.AddIPBinding(1, core.IPBinding{MACAddress: "AA:BB:CC:DD:EE:FF", Type: "invalid"})
		}},
		{"binding ip", func() error { return svc.AddIPBinding(1, core.IPBinding{Address: "bad", Type: "regular"}) }},
		{"binding mac", func() error { return svc.AddIPBinding(1, core.IPBinding{MACAddress: "bad", Type: "regular"}) }},
		{"binding id", func() error { return svc.SetIPBinding(1, core.IPBinding{Type: "regular"}) }},
		{"cookie id", func() error { return svc.RemoveCookie(1, "") }},
		{"server name", func() error { return svc.AddHotspotServer(1, core.HotspotServer{}) }},
		{"server interface", func() error { return svc.AddHotspotServer(1, core.HotspotServer{Name: "hs1"}) }},
		{"server profile", func() error { return svc.AddHotspotServer(1, core.HotspotServer{Name: "hs1", Interface: "bridge1"}) }},
		{"server id", func() error { return svc.SetHotspotServer(1, core.HotspotServer{Name: "hs"}) }},
		{"profile name", func() error { return svc.AddHotspotServerProfile(1, core.HotspotServerProfile{}) }},
		{"profile ip", func() error {
			return svc.AddHotspotServerProfile(1, core.HotspotServerProfile{Name: "p", HotspotAddress: "bad"})
		}},
		{"profile login", func() error {
			return svc.AddHotspotServerProfile(1, core.HotspotServerProfile{Name: "p", LoginBy: "http-chap,unknown"})
		}},
		{"profile cookie alone", func() error {
			return svc.AddHotspotServerProfile(1, core.HotspotServerProfile{Name: "p", LoginBy: "cookie"})
		}},
		{"profile ipv6", func() error {
			return svc.AddHotspotServerProfile(1, core.HotspotServerProfile{Name: "p", HotspotAddress: "2001:db8::1", LoginBy: "http-chap"})
		}},
	}
	for _, tc := range checks {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.fn(); !errors.Is(err, core.ErrInvalidInput) {
				t.Fatalf("got %v", err)
			}
		})
	}
}

func TestNormalizeLogLimit(t *testing.T) {
	for _, tc := range []struct{ in, want int }{{0, 100}, {1, 1}, {200, 200}} {
		got, err := normalizeLogLimit(tc.in)
		if err != nil || got != tc.want {
			t.Fatalf("limit %d: got %d, %v", tc.in, got, err)
		}
	}
	for _, limit := range []int{-1, 201} {
		if _, err := normalizeLogLimit(limit); !errors.Is(err, core.ErrInvalidInput) {
			t.Fatalf("limit %d: %v", limit, err)
		}
	}
}

func TestRouterManagementAcceptsWhitelistsThenRequiresConnection(t *testing.T) {
	svc := NewRouterManagement(NewPool())
	for _, typ := range []string{"regular", "bypassed", "blocked"} {
		err := svc.AddIPBinding(1, core.IPBinding{MACAddress: "AA:BB:CC:DD:EE:FF", Type: typ})
		if !errors.Is(err, core.ErrNotConnected) {
			t.Fatalf("type %q: %v", typ, err)
		}
	}
	for _, loginBy := range []string{"http-chap", "http-pap,https,cookie,mac-cookie,mac,trial"} {
		err := svc.AddHotspotServerProfile(1, core.HotspotServerProfile{Name: "profile", LoginBy: loginBy})
		if !errors.Is(err, core.ErrNotConnected) {
			t.Fatalf("login-by %q: %v", loginBy, err)
		}
	}
}
