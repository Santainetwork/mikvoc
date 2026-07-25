package httpapi

import "fmt"

const routerFileContentLimit = 4096

type templateFile struct {
	Name    string
	Content []byte
	Asset   bool
}

type templateFileSet struct {
	Files []templateFile
}

func (s templateFileSet) Get(name string) []byte {
	for _, file := range s.Files {
		if file.Name == name {
			return file.Content
		}
	}
	return nil
}

func (s templateFileSet) HasAssets() bool {
	for _, file := range s.Files {
		if file.Asset {
			return true
		}
	}
	return false
}

func (s templateFileSet) Oversized() []templateFile {
	var oversized []templateFile
	for _, file := range s.Files {
		if len(file.Content) >= routerFileContentLimit {
			oversized = append(oversized, file)
		}
	}
	return oversized
}

func (s templateFileSet) PushCheck() error {
	if s.HasAssets() {
		return fmt.Errorf("aset tidak dapat dikirim langsung; pilih Unduh Paket ZIP lalu unggah melalui Winbox/WebFig")
	}
	if oversized := s.Oversized(); len(oversized) > 0 {
		file := oversized[0]
		return fmt.Errorf("file %s berukuran %d byte, batas kirim langsung %d byte; gunakan Unduh Paket ZIP lalu unggah melalui Winbox/WebFig", file.Name, len(file.Content), routerFileContentLimit)
	}
	return nil
}
