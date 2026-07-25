package routeros

import (
	"strings"
	"time"
)

func IsZeroHotspotValue(s string) bool {
	s = strings.TrimSpace(s)
	return s == "" || s == "0" || s == "0s" || s == "00:00:00"
}

func IsUnusedHotspotUser(u HotspotUser) bool {
	return IsZeroHotspotValue(u.Uptime) && IsZeroHotspotValue(u.BytesIn) && IsZeroHotspotValue(u.BytesOut)
}

func IsExpiredHotspotUser(u HotspotUser, now time.Time) bool {
	return HotspotUserExpiredAt(u, now)
}
