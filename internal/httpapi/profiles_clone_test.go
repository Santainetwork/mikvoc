package httpapi

import (
	"os"
	"strings"
	"testing"
)

func TestBuildProfileCloneParamsCopiesWritableConfigToNewName(t *testing.T) {
	source := map[string]string{
		".id":          "*7",
		"name":         "1d",
		"shared-users": "1",
		"rate-limit":   "2M/2M",
		"address-pool": "pool-hotspot",
		"parent-queue": "parent-main",
		"on-login":     "# mikvoc-config: price=2000 validity=1d expired=remove lock_mac=1\n:local mac $\"mac-address\";",
		"comment":      "source note",
		"numbers":      "0",
		"default":      "true",
		"dynamic":      "false",
		"invalid":      "false",
	}

	params, err := buildProfileCloneParams(source, "1d-copy")
	if err != nil {
		t.Fatalf("buildProfileCloneParams returned error: %v", err)
	}

	if params["name"] != "1d-copy" {
		t.Fatalf("expected cloned profile name to be 1d-copy, got %q", params["name"])
	}
	for _, key := range []string{"shared-users", "rate-limit", "address-pool", "parent-queue", "on-login", "comment"} {
		if params[key] != source[key] {
			t.Fatalf("expected %s to be copied as %q, got %q", key, source[key], params[key])
		}
	}
	for _, key := range []string{".id", "numbers", "default", "dynamic", "invalid"} {
		if _, ok := params[key]; ok {
			t.Fatalf("expected read-only key %q not to be cloned; params=%v", key, params)
		}
	}
}

func TestBuildProfileCloneParamsRejectsBlankNewName(t *testing.T) {
	_, err := buildProfileCloneParams(map[string]string{"name": "1d"}, "   ")
	if err == nil {
		t.Fatal("expected blank clone name to be rejected")
	}
}

func TestProfilesTemplateIncludesCloneProfileAction(t *testing.T) {
	b, err := os.ReadFile("../../web/templates/pages/profiles.html")
	if err != nil {
		t.Fatalf("read profiles template: %v", err)
	}
	html := string(b)

	for _, want := range []string{
		`action="/hotspot/profiles/clone"`,
		`name="source_id" value="{{$p.ID}}"`,
		`openClone('{{$p.ID}}')`,
		`row-clone-{{$p.ID}}`,
		`content_copy`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("expected profiles template to contain %q", want)
		}
	}
}
