package routeros

import (
	"testing"
	"time"
)

func TestIsZeroHotspotValue(t *testing.T) {
	for _, s := range []string{"", "0", "0s", "00:00:00", " 0 "} {
		if !IsZeroHotspotValue(s) {
			t.Fatalf("expected zero for %q", s)
		}
	}
	if IsZeroHotspotValue("1s") {
		t.Fatal("expected non-zero")
	}
}

func TestIsUnusedHotspotUser(t *testing.T) {
	if !IsUnusedHotspotUser(HotspotUser{Uptime: "0s", BytesIn: "0", BytesOut: "0"}) {
		t.Fatal("expected unused")
	}
	if IsUnusedHotspotUser(HotspotUser{Uptime: "1h", BytesIn: "0", BytesOut: "0"}) {
		t.Fatal("expected used")
	}
}

func TestIsExpiredHotspotUser(t *testing.T) {
	now := time.Date(2020, 1, 1, 0, 0, 0, 0, time.Local)
	if !IsExpiredHotspotUser(HotspotUser{Comment: "2000-01-01 00:00:00"}, now) {
		t.Fatal("expected expired")
	}
	if IsExpiredHotspotUser(HotspotUser{Comment: "2999-01-01 00:00:00"}, now) {
		t.Fatal("expected not expired")
	}
	if IsExpiredHotspotUser(HotspotUser{Comment: "vc-05.04.26"}, now) {
		t.Fatal("batch comment should not expire")
	}
}
