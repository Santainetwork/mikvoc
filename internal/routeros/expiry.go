package routeros

import (
	"strings"
	"time"
)

// HotspotUserExpiredAt reports whether a hotspot user is expired according to
// RouterOS/Mikhmon conventions.
func HotspotUserExpiredAt(u HotspotUser, now time.Time) bool {
	if strings.TrimSpace(u.LimitUptime) == "1s" {
		return true
	}

	expiresAt, ok := ExpiryTimeFromComment(u.Comment)
	return ok && now.After(expiresAt)
}

// ExpiryTimeFromComment parses a Mikhmon/MikVoc expiry comment.
func ExpiryTimeFromComment(comment string) (time.Time, bool) {
	comment = strings.TrimSpace(comment)
	if comment == "" || isBatchVoucherComment(comment) {
		return time.Time{}, false
	}

	expiresAt, err := ParseDate(comment)
	if err != nil || expiresAt.IsZero() {
		return time.Time{}, false
	}
	return expiresAt, true
}

func isBatchVoucherComment(comment string) bool {
	if strings.HasPrefix(comment, "vc-") || strings.HasPrefix(comment, "up-") {
		return true
	}

	parts := strings.SplitN(comment, "-", 2)
	return len(parts) == 2 && len(parts[1]) >= len("05.04.26") && strings.Count(parts[1], ".") >= 2
}
