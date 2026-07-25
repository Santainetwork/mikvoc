package routeros

import "testing"

func TestEnabledInterfacesExcludesDisabledAndNameless(t *testing.T) {
	rows := []map[string]string{
		{"name": "ether1", "type": "ether", "disabled": "false", "running": "true"},
		{"name": "bridge-hotspot", "type": "bridge", "disabled": "false", "running": "true"},
		{"name": "vlan20", "type": "vlan", "disabled": "true", "running": "false"},
		{"type": "ether", "disabled": "false"},
	}

	got := enabledInterfaces(rows)
	if len(got) != 2 {
		t.Fatalf("enabled interfaces = %v, want 2 entries", got)
	}
	if got[0]["name"] != "ether1" || got[1]["name"] != "bridge-hotspot" {
		t.Fatalf("enabled interfaces = %v", got)
	}
}
