package routeros

import "testing"

func TestFormatPPPSecretName(t *testing.T) {
	if got := FormatPPPSecretName("user", 1, 3); got != "user001" {
		t.Fatalf("got %q", got)
	}
	if got := FormatPPPSecretName("a", 12, 2); got != "a12" {
		t.Fatalf("got %q", got)
	}
	if got := FormatPPPSecretName("x", 5, 0); got != "x5" {
		t.Fatalf("pad<1 -> %q", got)
	}
}

func TestPPPBatchComment(t *testing.T) {
	c := PPPBatchComment("10M", "")
	if c == "" || c[:4] != "ppp-" {
		t.Fatalf("unexpected comment %q", c)
	}
	c2 := PPPBatchComment("10M", "batchA")
	if c2[len(c2)-6:] != "batchA" {
		t.Fatalf("label missing: %q", c2)
	}
}

func TestPPPSecretFromRow(t *testing.T) {
	s := pppSecretFromRow(map[string]string{
		".id": "*1", "name": "u1", "password": "u1", "service": "pppoe",
		"profile": "default", "disabled": "true", "comment": "c",
	})
	if s.ID != "*1" || s.Name != "u1" || !s.Disabled || s.Service != "pppoe" {
		t.Fatalf("%+v", s)
	}
}

func TestPPPProfileFromRowIncludesMikhmonFields(t *testing.T) {
	p := pppProfileFromRow(map[string]string{
		".id": "*1", "name": "10M", "local-address": "10.0.0.1",
		"remote-address": "pool-10m", "bridge": "bridge1", "rate-limit": "10M/10M",
		"only-one": "yes", "incoming-filter": "input", "outgoing-filter": "forward",
		"address-list": "pppoe", "dns-server": "1.1.1.1", "wins-server": "10.0.0.2",
		"change-tcp-mss": "yes", "use-upnp": "no",
	})
	if p.IncomingFilter != "input" || p.OutgoingFilter != "forward" || p.WINSServer != "10.0.0.2" || p.UseUPnP != "no" {
		t.Fatalf("%+v", p)
	}
}
