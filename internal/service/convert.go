package service

import (
	"mikvoc/internal/core"
	"mikvoc/internal/routeros"
)

func toCoreUser(u routeros.HotspotUser) core.User {
	return core.User{
		ID:              u.ID,
		Name:            u.Name,
		Password:        u.Password,
		Profile:         u.Profile,
		Server:          u.Server,
		Comment:         u.Comment,
		LimitUptime:     u.LimitUptime,
		LimitBytesTotal: u.LimitBytesTotal,
		Uptime:          u.Uptime,
		BytesIn:         u.BytesIn,
		BytesOut:        u.BytesOut,
		Disabled:        u.Disabled,
		MacAddress:      u.MacAddress,
	}
}

func fromCoreUser(u core.User) routeros.HotspotUser {
	return routeros.HotspotUser{
		ID:              u.ID,
		Name:            u.Name,
		Password:        u.Password,
		Profile:         u.Profile,
		Server:          u.Server,
		Comment:         u.Comment,
		LimitUptime:     u.LimitUptime,
		LimitBytesTotal: u.LimitBytesTotal,
		Uptime:          u.Uptime,
		BytesIn:         u.BytesIn,
		BytesOut:        u.BytesOut,
		Disabled:        u.Disabled,
		MacAddress:      u.MacAddress,
	}
}

func toCoreProfile(p routeros.HotspotUserProfile) core.UserProfile {
	return core.UserProfile{
		ID:           p.ID,
		Name:         p.Name,
		RateLimit:    p.RateLimit,
		SharedUsers:  p.SharedUsers,
		OnLogin:      p.OnLogin,
		AddressPool:  p.AddressPool,
		ParentQueue:  p.ParentQueue,
		Validity:     p.Validity,
		ExpiredMode:  p.ExpiredMode,
		Price:        p.Price,
		SellingPrice: p.SellingPrice,
		GracePeriod:  p.GracePeriod,
		LockMac:      p.LockMac,
		HasMonitor:   p.HasMonitor,
		IsDefault:    p.IsDefault,
	}
}

func fromCoreProfile(p core.UserProfile) routeros.HotspotUserProfile {
	return routeros.HotspotUserProfile{
		ID:           p.ID,
		Name:         p.Name,
		RateLimit:    p.RateLimit,
		SharedUsers:  p.SharedUsers,
		OnLogin:      p.OnLogin,
		AddressPool:  p.AddressPool,
		ParentQueue:  p.ParentQueue,
		Validity:     p.Validity,
		ExpiredMode:  p.ExpiredMode,
		Price:        p.Price,
		SellingPrice: p.SellingPrice,
		GracePeriod:  p.GracePeriod,
		LockMac:      p.LockMac,
		HasMonitor:   p.HasMonitor,
		IsDefault:    p.IsDefault,
	}
}

func FromCoreProfiles(in []core.UserProfile) []routeros.HotspotUserProfile {
	out := make([]routeros.HotspotUserProfile, len(in))
	for i, p := range in {
		out[i] = fromCoreProfile(p)
	}
	return out
}

func FromCoreUsers(in []core.User) []routeros.HotspotUser {
	out := make([]routeros.HotspotUser, len(in))
	for i, u := range in {
		out[i] = fromCoreUser(u)
	}
	return out
}

func toCoreActive(a routeros.HotspotActive) core.ActiveSession {
	return core.ActiveSession{
		ID:       a.ID,
		User:     a.User,
		Server:   a.Server,
		IP:       a.IP,
		MacAddr:  a.MacAddr,
		Uptime:   a.Uptime,
		IdleTime: a.IdleTime,
		BytesIn:  a.BytesIn,
		BytesOut: a.BytesOut,
		Comment:  a.Comment,
	}
}

func toCoreResource(r routeros.SystemResource) core.SystemResource {
	return core.SystemResource{
		Version:     r.Version,
		BoardName:   r.BoardName,
		Uptime:      r.Uptime,
		CPULoad:     r.CPULoad,
		FreeMemory:  r.FreeMemory,
		TotalMemory: r.TotalMemory,
	}
}

func toROSGenerateOpts(spec core.VoucherSpec) routeros.GenerateOptions {
	return routeros.GenerateOptions{
		Qty:            spec.Qty,
		Profile:        spec.Profile,
		Server:         spec.Server,
		Mode:           spec.Mode,
		Prefix:         spec.Prefix,
		Length:         spec.Length,
		CharMode:       spec.CharMode,
		TimeLimitStr:   spec.TimeLimitStr,
		DataLimitBytes: spec.DataLimitBytes,
		Comment:        spec.Comment,
	}
}
