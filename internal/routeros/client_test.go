package routeros

import "testing"

func TestRouterOSCommandWordsNormalizesLegacyEqualKeys(t *testing.T) {
	words := routerOSCommandWords("/ip/hotspot/user/remove", map[string]string{
		"=.id":      "*1",
		"=disabled": "false",
		"?profile":  "1d",
	})

	for _, want := range []string{"/ip/hotspot/user/remove", "=.id=*1", "=disabled=false", "?profile=1d"} {
		if !hasWord(words, want) {
			t.Fatalf("expected words %v to contain %q", words, want)
		}
	}
	for _, bad := range []string{"==.id=*1", "==disabled=false"} {
		if hasWord(words, bad) {
			t.Fatalf("expected words %v not to contain %q", words, bad)
		}
	}
}

func hasWord(words []string, want string) bool {
	for _, word := range words {
		if word == want {
			return true
		}
	}
	return false
}
