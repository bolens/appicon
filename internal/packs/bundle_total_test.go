package packs

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallBundleEnforcesTotalSize(t *testing.T) {
	tests := []struct {
		name    string
		files   map[string]string
		wantErr bool
	}{
		{name: "exact limit", files: map[string]string{"pack/a.svg": "123", "pack/b.svg": "45"}},
		{name: "one byte over", files: map[string]string{"pack/a.svg": "123", "pack/b.svg": "456"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dataHome := t.TempDir()
			configDir := t.TempDir()
			t.Setenv("XDG_DATA_HOME", dataHome)
			bundle := writeBundleFixture(t, tt.files)

			err := installBundle(configDir, bundle, 5)
			if tt.wantErr {
				if err == nil || !strings.Contains(err.Error(), "uncompressed size exceeds limit") {
					t.Fatalf("err=%v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(filepath.Join(dataHome, "appicon", "packs", "pack", "b.svg")); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func writeBundleFixture(t *testing.T, files map[string]string) string {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, body := range files {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(body)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	bundle := filepath.Join(t.TempDir(), "bundle.tar.gz")
	if err := os.WriteFile(bundle, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return bundle
}
