package routeros

import (
	"strings"
	"testing"
)

func TestBuildProfileOnLoginScript(t *testing.T) {
	script := buildProfileOnLoginScript("  VIP 2H  ", " remove_record ", " 2h ", " 1 ", " 5000 ", "6000", "5m")
	if !strings.Contains(script, "# mikvoc-config:") {
		t.Fatalf("missing config line: %s", script)
	}
	if !strings.Contains(script, "price=5000") || !strings.Contains(script, "sprice=6000") {
		t.Fatalf("price fields missing: %s", script)
	}
	if !strings.Contains(script, "grace=5m") || !strings.Contains(script, "lock_mac=1") {
		t.Fatalf("grace/lock missing: %s", script)
	}
	if !strings.Contains(script, "validity=2h") {
		t.Fatalf("validity missing: %s", script)
	}
}

func TestBuildProfileOnLoginScriptMinimal(t *testing.T) {
	script := buildProfileOnLoginScript("basic", "none", "", "0", "", "", "")
	if !strings.Contains(script, "# mikvoc-config:") {
		t.Fatalf("expected config-only script, got %q", script)
	}
	if strings.Contains(script, "/system scheduler") {
		t.Fatalf("did not expect scheduler for none mode: %s", script)
	}
}

func TestBuildProfileOnLoginScriptPriceOnly(t *testing.T) {
	script := buildProfileOnLoginScript("Daily", "none", "", "0", "3000", "0", "5m")
	if !strings.Contains(script, "price=3000") {
		t.Fatalf("price missing: %s", script)
	}
}

func TestApplyProfileConfigMikhmonLegacy(t *testing.T) {
	p := HotspotUserProfile{
		OnLogin: `:put (",rem,2000,1d,2500,,Enable,"); {:local comment ...`,
	}
	applyProfileConfigFromOnLogin(&p)
	if p.ExpiredMode != "remove" {
		t.Fatalf("expired=%q", p.ExpiredMode)
	}
	if p.Price != "2000" || p.SellingPrice != "2500" || p.Validity != "1d" {
		t.Fatalf("%+v", p)
	}
	if !p.LockMac {
		t.Fatal("expected lock mac")
	}
}

func TestApplyProfileConfigMikvoc(t *testing.T) {
	p := HotspotUserProfile{
		OnLogin: "# mikvoc-config: price=1000 sprice=1500 validity=12h expired=notice grace=10m lock_mac=1\n:local mac",
	}
	applyProfileConfigFromOnLogin(&p)
	if p.Price != "1000" || p.SellingPrice != "1500" || p.Validity != "12h" || p.ExpiredMode != "notice" || p.GracePeriod != "10m" || !p.LockMac {
		t.Fatalf("%+v", p)
	}
}

func TestNoticeRecordNeedsExpiryAndRecord(t *testing.T) {
	if !profileScriptNeedsExpiry("notice_record", "1d") {
		t.Fatal("notice_record must schedule expiry")
	}
	if !profileScriptNeedsRecord("notice_record", "0") {
		t.Fatal("notice_record must create sales record")
	}
}
