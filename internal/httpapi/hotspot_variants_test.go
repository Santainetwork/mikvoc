package httpapi

import "testing"

func TestNormalizeHotspotVariantDefaultsToModern(t *testing.T) {
	for _, in := range []string{"", "bogus", "MODERN "} {
		if got := normalizeHotspotVariant(in); got != "modern" {
			t.Fatalf("normalizeHotspotVariant(%q) = %q, want modern", in, got)
		}
	}
}

func TestNormalizeHotspotVariantKeepsKnownIDs(t *testing.T) {
	for _, id := range []string{"modern", "informative", "minimal", "cafe", "custom"} {
		if got := normalizeHotspotVariant(id); got != id {
			t.Fatalf("normalizeHotspotVariant(%q) = %q", id, got)
		}
	}
}

func TestValidHotspotVariantRejectsUnknown(t *testing.T) {
	if validHotspotVariant("bogus") {
		t.Fatal("expected bogus to be rejected on save")
	}
	if !validHotspotVariant("cafe") {
		t.Fatal("expected cafe to be accepted on save")
	}
}

func TestBuiltinHotspotVariantsExposeFiveChoices(t *testing.T) {
	if len(BuiltinHotspotVariants) != 5 {
		t.Fatalf("expected 5 variants, got %d", len(BuiltinHotspotVariants))
	}
	seen := map[string]bool{}
	for _, v := range BuiltinHotspotVariants {
		if v.ID == "" || v.Name == "" || v.Description == "" {
			t.Fatalf("variant %+v has empty field", v)
		}
		if seen[v.ID] {
			t.Fatalf("duplicate variant ID %q", v.ID)
		}
		if v.ID == "custom" && !v.Custom {
			t.Fatal("expected custom variant to be marked custom")
		}
		seen[v.ID] = true
	}
	if !seen["custom"] {
		t.Fatal("expected a custom variant")
	}
}
