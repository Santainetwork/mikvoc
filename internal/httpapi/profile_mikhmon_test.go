package httpapi

import (
	"os"
	"strings"
	"testing"

	"mikvoc/internal/routeros"
)

func TestProfilesTemplateHasMikhmonFields(t *testing.T) {
	b, err := os.ReadFile("../../web/templates/pages/profiles.html")
	if err != nil {
		t.Fatal(err)
	}
	html := string(b)
	for _, field := range []string{
		`name="address_pool"`,
		`name="shared_users"`,
		`name="rate_limit"`,
		`name="expired_mode"`,
		`name="validity"`,
		`name="grace_period"`,
		`name="price"`,
		`name="sprice"`,
		`name="lock_mac"`,
		`name="parent_queue"`,
		`value="notice_record"`,
	} {
		if !strings.Contains(html, field) {
			t.Errorf("missing Mikhmon profile field %s", field)
		}
	}
}

func TestProfileVoucherPricePrefersMikhmonSellingPrice(t *testing.T) {
	p := routeros.HotspotUserProfile{Price: "2000", SellingPrice: "3000"}
	if got := profileVoucherPrice(p); got != "3000" {
		t.Fatalf("got %q", got)
	}
	p.SellingPrice = "0"
	if got := profileVoucherPrice(p); got != "2000" {
		t.Fatalf("fallback got %q", got)
	}
}
