package httpapi

import (
	"os"
	"strings"
	"testing"
)

func TestPPPProfilesTemplateHasMikhmonFields(t *testing.T) {
	b, err := os.ReadFile("../../web/templates/pages/ppp_profiles.html")
	if err != nil {
		t.Fatal(err)
	}
	html := string(b)
	for _, field := range []string{
		`name="local_address"`,
		`name="remote_address"`,
		`name="bridge"`,
		`name="incoming_filter"`,
		`name="outgoing_filter"`,
		`name="address_list"`,
		`name="dns_server"`,
		`name="wins_server"`,
		`name="change_tcp_mss"`,
		`name="use_upnp"`,
		`name="rate_limit"`,
		`name="only_one"`,
	} {
		if !strings.Contains(html, field) {
			t.Errorf("missing Mikhmon PPP profile field %s", field)
		}
	}
}
