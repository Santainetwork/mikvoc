package httpapi

import "strings"

type HotspotVariant struct {
	ID          string
	Name        string
	Description string
	Custom      bool
}

var BuiltinHotspotVariants = []HotspotVariant{
	{ID: "modern", Name: "Modern", Description: "Kartu login bersih dengan aksen warna utama"},
	{ID: "informative", Name: "Informatif", Description: "Login plus panel paket, harga, dan kontak"},
	{ID: "minimal", Name: "Minimal", Description: "Ringan dan kontras tinggi untuk perangkat lambat"},
	{ID: "cafe", Name: "Cafe / Gaming", Description: "Tema gelap energik untuk cafe dan game center"},
	{ID: "custom", Name: "Custom HTML", Description: "Edit login/status/logout sendiri, bisa plus aset ZIP", Custom: true},
}

func validHotspotVariant(id string) bool {
	for _, variant := range BuiltinHotspotVariants {
		if variant.ID == id {
			return true
		}
	}
	return false
}

func normalizeHotspotVariant(id string) string {
	id = strings.ToLower(strings.TrimSpace(id))
	if validHotspotVariant(id) {
		return id
	}
	return "modern"
}
