package httpapi

import (
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"mikvoc/internal/routeros"
)

func TestLogoDataURLAcceptsUploadedImage(t *testing.T) {
	png := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0x00, 0x00, 0x00, 0x0d, 'I', 'H', 'D', 'R'}

	dataURL, err := logoDataURL("logo.png", png)
	if err != nil {
		t.Fatalf("logoDataURL returned error: %v", err)
	}

	if !strings.HasPrefix(dataURL, "data:image/png;base64,") {
		t.Fatalf("expected PNG data URL, got %q", dataURL)
	}
}

func TestLogoDataURLRejectsNonImage(t *testing.T) {
	_, err := logoDataURL("logo.txt", []byte("not an image"))
	if err == nil {
		t.Fatal("expected non-image upload to be rejected")
	}
}

func TestFormIDsReadsMultipleSelectedUsers(t *testing.T) {
	form := url.Values{}
	form.Add("id", "*1")
	form.Add("id", "*2")
	req := httptest.NewRequest("POST", "/hotspot/users/remove", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if err := req.ParseForm(); err != nil {
		t.Fatalf("ParseForm: %v", err)
	}

	ids := formIDs(req, "id")
	if got := strings.Join(ids, ","); got != "*1,*2" {
		t.Fatalf("expected both selected ids, got %q", got)
	}
}

func TestQueryIDsAcceptsRepeatedAndCommaSeparatedValues(t *testing.T) {
	ids := queryIDs([]string{"*1,*2", "*3", "*2"})
	if got := strings.Join(ids, ","); got != "*1,*2,*3" {
		t.Fatalf("expected ids to preserve first-seen order, got %q", got)
	}
}

func TestUserListPrintURLHonorsSavedTemplateByDefaultAndSelectedIDs(t *testing.T) {
	filters := userListFilters{Comment: "batch-1", IDs: []string{"*1", "*2"}}
	defaultURL, qrURL, smallURL := userListPrintURLs(filters)

	if strings.Contains(defaultURL, "template=classic") {
		t.Fatalf("default print URL should use saved voucher_template setting, got %q", defaultURL)
	}
	for _, want := range []string{"comment=batch-1", "ids=%2A1", "ids=%2A2", "qr=no"} {
		if !strings.Contains(defaultURL, want) {
			t.Fatalf("expected default print URL %q to contain %q", defaultURL, want)
		}
	}
	if !strings.Contains(qrURL, "qr=yes") {
		t.Fatalf("expected QR print URL to request QR, got %q", qrURL)
	}
	if !strings.Contains(smallURL, "template=compact") {
		t.Fatalf("expected small print URL to request compact template, got %q", smallURL)
	}
}

func TestCommentRemovalCandidatesOnlyUnusedMatchingUsers(t *testing.T) {
	users := []routeros.HotspotUser{
		{ID: "*1", Comment: "batch-a", Uptime: ""},
		{ID: "*2", Comment: "batch-a", Uptime: "0s"},
		{ID: "*3", Comment: "batch-a", Uptime: "00:00:00"},
		{ID: "*4", Comment: "batch-a", Uptime: "1m"},
		{ID: "*5", Comment: "batch-b", Uptime: ""},
	}

	candidates := commentRemovalCandidates(users, "batch-a", "")
	ids := make([]string, 0, len(candidates))
	for _, u := range candidates {
		ids = append(ids, u.ID)
	}
	if got := strings.Join(ids, ","); got != "*1,*2,*3" {
		t.Fatalf("expected only unused matching users, got %q", got)
	}
}

func TestCommentRemovalCandidatesCanScopeByProfile(t *testing.T) {
	users := []routeros.HotspotUser{
		{ID: "*1", Profile: "default", Comment: "batch-a", Uptime: ""},
		{ID: "*2", Profile: "premium", Comment: "batch-a", Uptime: ""},
		{ID: "*3", Profile: "premium", Comment: "batch-a", Uptime: "2m"},
		{ID: "*4", Profile: "premium", Comment: "batch-b", Uptime: ""},
	}

	candidates := commentRemovalCandidates(users, "batch-a", "premium")
	ids := make([]string, 0, len(candidates))
	for _, u := range candidates {
		ids = append(ids, u.ID)
	}
	if got := strings.Join(ids, ","); got != "*2" {
		t.Fatalf("expected only unused matching users in profile premium, got %q", got)
	}
}

func TestCommentCandidateProfilesOnlyUsesUnusedMatchingCandidates(t *testing.T) {
	users := []routeros.HotspotUser{
		{ID: "*1", Profile: "default", Comment: "batch-a", Uptime: ""},
		{ID: "*2", Profile: "premium", Comment: "batch-a", Uptime: "0s"},
		{ID: "*3", Profile: "used", Comment: "batch-a", Uptime: "5m"},
		{ID: "*4", Profile: "premium", Comment: "batch-b", Uptime: ""},
	}

	profiles := commentCandidateProfiles(commentRemovalCandidates(users, "batch-a", ""))
	if got := strings.Join(profiles, ","); got != "default,premium" {
		t.Fatalf("expected only profiles with unused matching users, got %q", got)
	}
}

func TestUsersTemplateDefaultsToSelectionColumn(t *testing.T) {
	b, err := os.ReadFile("../../web/templates/pages/users.html")
	if err != nil {
		t.Fatalf("read users template: %v", err)
	}
	html := string(b)

	for _, want := range []string{
		`id="select-all-users"`,
		`class="user-select rounded`,
		`ajaxRemoveSelectedUsers()`,
		`ajaxRemoveExpiredUsers()`,
		`refreshUsersTable()`,
		`printSelectedOrBatch(`,
		`perPageSelect: [10, 50, 100, 500, 1000]`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("expected users template to contain %q", want)
		}
	}
}

func TestUsersTemplateUsesSafeAjaxCommentRemoval(t *testing.T) {
	b, err := os.ReadFile("../../web/templates/pages/users.html")
	if err != nil {
		t.Fatalf("read users template: %v", err)
	}
	html := string(b)

	for _, want := range []string{
		`data-comment="{{.Comment}}"`,
		`data-profile="{{.Profile}}"`,
		`name="profile" value="{{.Profile}}"`,
		`params.append('profile', profile)`,
		`selectedCommentProfile()`,
		`ajaxRemoveByComment(this)`,
		`afterUserAction`,
		`refreshUsersTable()`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("expected users template to contain %q", want)
		}
	}
	if strings.Contains(html, `confirm('Hapus user belum terpakai dengan comment {{.Comment}}?')`) {
		t.Fatal("delete-by-comment should not interpolate comments inside a single-quoted JS string")
	}
}
