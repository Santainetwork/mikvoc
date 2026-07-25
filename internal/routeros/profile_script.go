package routeros

import (
	"fmt"
	"strings"
)

func buildProfileOnLoginScript(profileName, expiredMode, validity, lockMac, price, sellingPrice, gracePeriod string) string {
	profileName = strings.TrimSpace(profileName)
	expiredMode = strings.TrimSpace(expiredMode)
	validity = strings.TrimSpace(validity)
	lockMac = strings.TrimSpace(lockMac)
	price = strings.TrimSpace(price)
	sellingPrice = strings.TrimSpace(sellingPrice)
	gracePeriod = strings.TrimSpace(gracePeriod)
	if sellingPrice == "" {
		sellingPrice = "0"
	}
	if gracePeriod == "" {
		gracePeriod = "5m"
	}

	lines := []string{
		fmt.Sprintf("# mikvoc-config: price=%s sprice=%s validity=%s expired=%s grace=%s lock_mac=%s",
			price, sellingPrice, validity, expiredMode, gracePeriod, lockMac),
	}

	hasExpiry := profileScriptNeedsExpiry(expiredMode, validity)
	hasRecord := profileScriptNeedsRecord(expiredMode, price)
	hasLock := lockMac == "1"
	if !hasExpiry && !hasRecord && !hasLock {
		return lines[0]
	}

	if hasRecord || hasLock {
		lines = append(lines, `:local mac $"mac-address";`)
	}
	if hasExpiry || hasRecord {
		lines = append(lines,
			`:local date [/system clock get date];`,
			`:local year [:pick $date 7 11];`,
			`:local month [:pick $date 0 3];`,
			`:if ([:pick $date 4 5] = "-") do={`,
			`  :set year [:pick $date 0 4];`,
			`  :set month [:pick $date 5 7];`,
			`}`,
		)
	}
	if hasRecord {
		lines = append(lines, `:local time [/system clock get time];`)
	}
	if hasExpiry || hasRecord {
		lines = append(lines, `:local comment [/ip hotspot user get [/ip hotspot user find where name="$user"] comment];`)
	}

	recordLine := buildProfileRecordScriptLine(profileName, validity, price)
	if hasExpiry {
		lines = append(lines,
			`:local ucode [:pick $comment 0 2];`,
			`:if (($ucode = "vc") or ($ucode = "up") or ($comment = "")) do={`,
			fmt.Sprintf(`  /system scheduler add name="$user" disabled=no start-date=$date interval="%s";`, routerOSScriptString(validity)),
			`  :delay 5s;`,
			`  :local exp [/system scheduler get [/system scheduler find where name="$user"] next-run];`,
			`  :local getxp [:len $exp];`,
			`  :if ($getxp = 15) do={`,
			`    :local d [:pick $exp 0 6];`,
			`    :local t [:pick $exp 7 16];`,
			`    :local s "/";`,
			`    :set exp ("$d$s$year $t");`,
			`    /ip hotspot user set comment="$exp" [/ip hotspot user find where name="$user"];`,
			`  }`,
			`  :if ($getxp = 8) do={`,
			`    /ip hotspot user set comment="$date $exp" [/ip hotspot user find where name="$user"];`,
			`  }`,
			`  :if ($getxp > 15) do={`,
			`    /ip hotspot user set comment="$exp" [/ip hotspot user find where name="$user"];`,
			`  }`,
			`  :delay 5s;`,
			`  /system scheduler remove [/system scheduler find where name="$user"];`,
		)
		if hasRecord {
			lines = append(lines, "  "+recordLine)
		}
		lines = append(lines, `}`)
	} else if hasRecord {
		lines = append(lines, recordLine)
	}

	if hasLock {
		lines = append(lines,
			`:if ([/ip hotspot user get [/ip hotspot user find where name="$user"] mac-address] = "") do={`,
			`  /ip hotspot user set mac-address=$mac [/ip hotspot user find where name="$user"];`,
			`}`,
		)
	}

	return strings.Join(lines, "\n")
}

func profileScriptNeedsExpiry(expiredMode, validity string) bool {
	if validity == "" {
		return false
	}

	switch expiredMode {
	case "remove", "remove_record", "notice", "notice_record", "disable":
		return true
	default:
		return false
	}
}

func profileScriptNeedsRecord(expiredMode, price string) bool {
	return expiredMode == "remove_record" || expiredMode == "notice_record" || profileScriptHasNonZeroPrice(price)
}

func profileScriptHasNonZeroPrice(price string) bool {
	price = strings.TrimSpace(price)
	if price == "" {
		return false
	}
	for _, r := range price {
		if r >= '1' && r <= '9' {
			return true
		}
	}
	return false
}

func buildProfileRecordScriptLine(profileName, validity, price string) string {
	if strings.TrimSpace(price) == "" {
		price = "0"
	}
	return fmt.Sprintf(`/system script add name="$date-|-$time-|-$user-|-%s-|-$address-|-$mac-|-%s-|-%s-|-$comment" owner="$month$year" source="$date" comment="mikhmon";`,
		routerOSScriptString(price),
		routerOSScriptString(validity),
		routerOSScriptString(profileName),
	)
}

func routerOSScriptString(s string) string {
	return strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s)
}

func applyProfileConfigFromOnLogin(p *HotspotUserProfile) {
	if p == nil || p.OnLogin == "" {
		return
	}
	for _, line := range strings.Split(p.OnLogin, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# mikvoc-config: ") {
			configStr := strings.TrimPrefix(line, "# mikvoc-config: ")
			for _, part := range strings.Fields(configStr) {
				kv := strings.SplitN(part, "=", 2)
				if len(kv) != 2 {
					continue
				}
				switch kv[0] {
				case "price":
					p.Price = kv[1]
				case "sprice":
					p.SellingPrice = kv[1]
				case "validity":
					p.Validity = kv[1]
				case "expired":
					p.ExpiredMode = normalizeExpiredMode(kv[1])
				case "grace":
					p.GracePeriod = kv[1]
				case "lock_mac":
					p.LockMac = kv[1] == "1" || strings.EqualFold(kv[1], "Enable")
				}
			}
			return
		}
	}
	if i := strings.Index(p.OnLogin, `:put (",`); i >= 0 {
		rest := p.OnLogin[i+len(`:put (",`):]
		if j := strings.Index(rest, `")`); j >= 0 {
			parts := strings.Split(rest[:j], ",")
			if len(parts) >= 1 {
				p.ExpiredMode = normalizeExpiredMode(parts[0])
			}
			if len(parts) >= 2 && parts[1] != "0" {
				p.Price = parts[1]
			}
			if len(parts) >= 3 {
				p.Validity = parts[2]
			}
			if len(parts) >= 4 && parts[3] != "0" {
				p.SellingPrice = parts[3]
			}
			if len(parts) >= 6 {
				p.LockMac = strings.EqualFold(parts[5], "Enable") || parts[5] == "1"
			}
			if p.GracePeriod == "" {
				p.GracePeriod = "5m"
			}
		}
	}
}

func normalizeExpiredMode(mode string) string {
	switch strings.TrimSpace(mode) {
	case "rem", "remove":
		return "remove"
	case "ntf", "notice":
		return "notice"
	case "remc", "remove_record":
		return "remove_record"
	case "ntfc", "notice_record":
		return "notice_record"
	case "disable":
		return "disable"
	case "0", "", "none":
		return "none"
	default:
		return mode
	}
}
