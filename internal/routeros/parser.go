// Package api - date/time adaptive parser for RouterOS v6 and v7.10+
package routeros

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ParseDate parses a RouterOS date string, supporting both:
//   - RouterOS v6 format: "may/02/2026" (mon/DD/YYYY)
//   - RouterOS v7.10+ format: "2026-05-02" (YYYY-MM-DD, ISO 8601)
func ParseDate(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, nil
	}
	// Try ISO 8601 first (v7.10+)
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t, nil
	}
	// Try ISO with time
	if t, err := time.Parse("2006-01-02 15:04:05", s); err == nil {
		return t, nil
	}
	// Try legacy format (v6): "jan/02/2006"
	if t, err := time.Parse("Jan/02/2006", strings.Title(s)); err == nil {
		return t, nil
	}
	// Legacy with time: "jan/02/2006 15:04:05"
	if t, err := time.Parse("Jan/02/2006 15:04:05", strings.Title(s)); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("cannot parse date: %q", s)
}

// FormatDateForRouter formats a time.Time into the appropriate date string
// for writing to the router, based on the detected RouterOS version.
func FormatDateForRouter(t time.Time, rosVersion string) string {
	if isV7Plus(rosVersion) {
		return t.Format("2006-01-02 15:04:05")
	}
	// v6 format: "jan/02/2006 15:04:05"
	return t.Format("Jan/02/2006 15:04:05")
}

// isV7Plus returns true if the version string indicates RouterOS v7.10+.
func isV7Plus(version string) bool {
	if version == "" {
		return false
	}
	parts := strings.Split(version, ".")
	if len(parts) < 2 {
		return false
	}
	major, _ := strconv.Atoi(parts[0])
	if major > 7 {
		return true
	}
	if major == 7 {
		minor, _ := strconv.Atoi(strings.TrimRight(parts[1], " (abcdefghijklmnopqrstuvwxyz)"))
		return minor >= 10
	}
	return false
}

// ParseDuration parses a RouterOS duration string like "1d05h20m10s", "3h", "00:05:00", etc.
func ParseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" || s == "0" {
		return 0, nil
	}

	// Format: "1d05h20m10s"
	re := regexp.MustCompile(`(?:(\d+)w)?(?:(\d+)d)?(?:(\d+)h)?(?:(\d+)m)?(?:(\d+)s)?`)
	if re.MatchString(s) && strings.ContainsAny(s, "wdhms") {
		m := re.FindStringSubmatch(s)
		weeks, _ := strconv.Atoi(m[1])
		days, _ := strconv.Atoi(m[2])
		hours, _ := strconv.Atoi(m[3])
		mins, _ := strconv.Atoi(m[4])
		secs, _ := strconv.Atoi(m[5])
		total := time.Duration(weeks)*7*24*time.Hour +
			time.Duration(days)*24*time.Hour +
			time.Duration(hours)*time.Hour +
			time.Duration(mins)*time.Minute +
			time.Duration(secs)*time.Second
		return total, nil
	}

	// Format: "HH:MM:SS"
	parts := strings.Split(s, ":")
	if len(parts) == 3 {
		h, _ := strconv.Atoi(parts[0])
		m, _ := strconv.Atoi(parts[1])
		sec, _ := strconv.Atoi(parts[2])
		return time.Duration(h)*time.Hour + time.Duration(m)*time.Minute + time.Duration(sec)*time.Second, nil
	}

	return 0, fmt.Errorf("cannot parse duration: %q", s)
}

// FormatDuration formats a duration as a human-readable string.
func FormatDuration(d time.Duration) string {
	if d <= 0 {
		return "-"
	}
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	mins := int(d.Minutes()) % 60
	secs := int(d.Seconds()) % 60

	var parts []string
	if days > 0 {
		parts = append(parts, fmt.Sprintf("%dd", days))
	}
	if hours > 0 {
		parts = append(parts, fmt.Sprintf("%dh", hours))
	}
	if mins > 0 {
		parts = append(parts, fmt.Sprintf("%dm", mins))
	}
	if secs > 0 || len(parts) == 0 {
		parts = append(parts, fmt.Sprintf("%ds", secs))
	}
	return strings.Join(parts, "")
}

// FormatBytes formats bytes into human-readable form.
func FormatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
