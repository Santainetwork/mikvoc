package routeros

import (
	"reflect"
	"testing"

	"mikvoc/internal/core"
)

func TestHotspotHostFromRow(t *testing.T) {
	got := hotspotHostFromRow(map[string]string{
		".id": "*1", "mac-address": "AA:BB:CC:DD:EE:FF", "address": "10.0.0.2",
		"to-address": "10.0.0.3", "server": "hotspot1", "uptime": "1h", "idle-time": "2m",
	})
	if got.ID != "*1" || got.MACAddress != "AA:BB:CC:DD:EE:FF" || got.ToAddress != "10.0.0.3" || got.IdleTime != "2m" {
		t.Fatalf("%+v", got)
	}
}

func TestIPBindingFromRow(t *testing.T) {
	got := ipBindingFromRow(map[string]string{
		".id": "*2", "mac-address": "AA:BB:CC:DD:EE:FF", "address": "10.0.0.2",
		"to-address": "10.0.0.3", "server": "all", "type": "bypassed", "comment": "printer", "disabled": "true",
	})
	if got.ID != "*2" || got.Type != "bypassed" || !got.Disabled || got.Comment != "printer" {
		t.Fatalf("%+v", got)
	}
}

func TestCookieAndHotspotServerRows(t *testing.T) {
	cookie := hotspotCookieFromRow(map[string]string{".id": "*3", "user": "alice", "mac-address": "AA:BB:CC:DD:EE:FF", "expires-in": "1d"})
	if cookie.User != "alice" || cookie.ExpiresIn != "1d" {
		t.Fatalf("%+v", cookie)
	}
	server := hotspotServerFromRow(map[string]string{".id": "*4", "name": "hs1", "interface": "bridge", "address-pool": "pool1", "profile": "default", "idle-timeout": "5m", "keepalive-timeout": "2m", "disabled": "true"})
	if server.Name != "hs1" || server.AddressPool != "pool1" || !server.Disabled {
		t.Fatalf("%+v", server)
	}
	profile := hotspotServerProfileFromRow(map[string]string{".id": "*5", "name": "prof1", "hotspot-address": "10.0.0.1", "dns-name": "login.test", "html-directory": "hotspot", "login-by": "http-chap,cookie", "http-cookie-lifetime": "3d", "rate-limit": "10M/10M"})
	if profile.DNSName != "login.test" || profile.CookieLifetime != "3d" || profile.RateLimit != "10M/10M" {
		t.Fatalf("%+v", profile)
	}
}

func TestFilterSystemLogsFiltersAndCaps(t *testing.T) {
	rows := []map[string]string{
		{".id": "*1", "topics": "system,info", "message": "router started"},
		{".id": "*2", "topics": "hotspot,info", "message": "alice logged in"},
		{".id": "*3", "topics": "hotspot,error", "message": "login failed"},
	}
	got := filterSystemLogs(rows, "hotspot", "login", 1)
	want := []core.SystemLog{{ID: "*3", Topics: "hotspot,error", Message: "login failed"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v want %+v", got, want)
	}
}

func TestRouterManagementRejectsMissingIDs(t *testing.T) {
	c := &Client{}
	checks := []struct {
		name string
		fn   func() error
	}{
		{"make binding", func() error { return c.MakeHotspotHostBinding("", "regular") }},
		{"set binding", func() error { return c.SetIPBinding(core.IPBinding{}) }},
		{"remove binding", func() error { return c.RemoveIPBinding("") }},
		{"remove cookie", func() error { return c.RemoveHotspotCookie("") }},
		{"set server", func() error { return c.SetHotspotServer(core.HotspotServer{}) }},
		{"remove server", func() error { return c.RemoveHotspotServer("") }},
		{"set profile", func() error { return c.SetHotspotServerProfile(core.HotspotServerProfile{}) }},
		{"remove profile", func() error { return c.RemoveHotspotServerProfile("") }},
	}
	for _, tc := range checks {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.fn(); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestRouterManagementProplistsAreFixed(t *testing.T) {
	want := map[string]string{
		"host": hotspotHostProplist, "binding": ipBindingProplist, "cookie": hotspotCookieProplist,
		"log": systemLogProplist, "server": hotspotServerProplist, "profile": hotspotServerProfileProplist,
	}
	for name, value := range want {
		if value == "" {
			t.Fatalf("%s proplist empty", name)
		}
	}
}

func TestRouterManagementParamsApplySafeDefaults(t *testing.T) {
	server := hotspotServerParams(core.HotspotServer{Name: "hs1", Interface: "bridge1", Profile: "default"})
	if server["address-pool"] != "none" || server["idle-timeout"] != "5m" || server["keepalive-timeout"] != "none" {
		t.Fatalf("server params = %#v", server)
	}
	profile := hotspotServerProfileParams(core.HotspotServerProfile{Name: "default"})
	if profile["hotspot-address"] != "0.0.0.0" || profile["html-directory"] != "hotspot" || profile["login-by"] != "http-chap,cookie" || profile["http-cookie-lifetime"] != "3d" {
		t.Fatalf("profile params = %#v", profile)
	}
	binding := ipBindingParams(core.IPBinding{Address: "10.0.0.2", Type: "regular"})
	if binding["server"] != "all" {
		t.Fatalf("binding params = %#v", binding)
	}
}
