package routeros

import (
	"testing"
	"time"
)

func TestHotspotUserExpiredAtLimitUptimeOneSecond(t *testing.T) {
	now := time.Date(2026, 5, 4, 13, 10, 0, 0, time.UTC)
	u := HotspotUser{LimitUptime: " 1s "}

	if !HotspotUserExpiredAt(u, now) {
		t.Fatal("expected limit-uptime 1s to be expired")
	}
}

func TestHotspotUserExpiredAtCommentTime(t *testing.T) {
	now := time.Date(2026, 5, 4, 13, 10, 0, 0, time.UTC)

	tests := []struct {
		name    string
		comment string
		want    bool
	}{
		{name: "future comment is not expired", comment: "2026-05-04 13:10:01", want: false},
		{name: "equal comment is not expired", comment: "2026-05-04 13:10:00", want: false},
		{name: "past comment is expired", comment: "2026-05-04 13:09:59", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := HotspotUser{Comment: tt.comment}
			if got := HotspotUserExpiredAt(u, now); got != tt.want {
				t.Fatalf("HotspotUserExpiredAt() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHotspotUserExpiredAtLegacyV6Comment(t *testing.T) {
	now := time.Date(2026, 5, 4, 13, 10, 1, 0, time.UTC)
	u := HotspotUser{Comment: " may/04/2026 13:10:00 "}

	if !HotspotUserExpiredAt(u, now) {
		t.Fatal("expected legacy v6 expiry comment to be expired")
	}
}

func TestExpiryTimeFromCommentIgnoresBatchVoucherComments(t *testing.T) {
	for _, comment := range []string{"vc-05.04.26", "1d-05.04.26-batch"} {
		t.Run(comment, func(t *testing.T) {
			if _, ok := ExpiryTimeFromComment(comment); ok {
				t.Fatalf("expected %q not to parse as an expiry time", comment)
			}
		})
	}
}
