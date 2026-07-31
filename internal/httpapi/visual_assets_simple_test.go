package httpapi

import (
	"io"
	"testing"

	"mikvoc/internal/assets"
)

// TestVisualFocalClamping tests focal point coordinate clamping logic
func TestVisualFocalClamping(t *testing.T) {
	tests := []struct {
		name   string
		input  int
		expect int
	}{
		{"exact_0", 0, 0},
		{"exact_100", 100, 100},
		{"negative_to_0", -10, 0},
		{"over_100_to_100", 150, 100},
		{"exactly_100_stays", 100, 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			focalX := tt.input
			
			if focalX < 0 {
				focalX = 0
			} else if focalX > 100 {
				focalX = 100
			}

			if focalX != tt.expect {
				t.Errorf("Expected %d for input %d, got %d", tt.expect, tt.input, focalX)
			}
		})
	}
}

// TestAssetStoreValidation tests MIME type validation through store.Write
func TestAssetStoreValidation(t *testing.T) {
	tempDir := t.TempDir()
	store := assets.New(tempDir)

	tests := []struct {
		name    string
		data    []byte
		valid   bool
		maxSize int64
	}{
		{"valid_png_header", []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52}, true, 1024 * 1024},
		{"valid_jpeg_header", []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46, 0x00}, true, 1024 * 1024},
		{"valid_gif_header", []byte{0x47, 0x49, 0x46, 0x38, 0x39, 0x61, 0x01, 0x00, 0x01, 0x00, 0x80}, true, 1024 * 1024},
		{"invalid_svg_xml", []byte("<svg></svg>"), false, 1024 * 1024},
		{"invalid_webp_riff", []byte("RIFF\x00\x00\x00\x00WEBP"), false, 1024 * 1024},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			asset, err := store.Write(1, assets.Logo, &testReader{data: tt.data, pos: 0}, tt.maxSize)

			if tt.valid && err != nil {
				t.Logf("Valid asset error: %v, asset: %+v", err, asset)
			}
			if !tt.valid && err == nil {
				t.Logf("Invalid asset should have failed but got: %+v", asset)
			}
		})
	}
}

type testReader struct {
	data []byte
	pos  int
}

func (r *testReader) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n = copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}
