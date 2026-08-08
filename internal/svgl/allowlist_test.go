package svgl_test

import (
	"errors"
	"testing"

	"github.com/bolens/appicon/internal/svgl"
)

func TestAssertAllowedURL(t *testing.T) {
	t.Parallel()
	cases := []struct {
		url   string
		allow bool
	}{
		{"https://svgl.app/library/firefox.svg", true},
		{"https://api.svgl.app/", true},
		{"http://svgl.app/x.svg", false},
		{"https://evil.example/x.svg", false},
		{"https://svgl.app.evil.com/x.svg", false},
		{"https://user@svgl.app/x.svg", true}, // host is still svgl.app
	}
	for _, tc := range cases {
		err := svgl.AssertAllowedURL(tc.url)
		if tc.allow && err != nil {
			t.Fatalf("%s: unexpected err %v", tc.url, err)
		}
		if !tc.allow && !errors.Is(err, svgl.ErrHostNotAllowed) {
			t.Fatalf("%s: want ErrHostNotAllowed got %v", tc.url, err)
		}
	}
}

func TestAssetFileName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		url  string
		want string
	}{
		{name: "plain", url: "https://svgl.app/vscode-dark.svg", want: "visual-studio-code-dark.svg"},
		{name: "query and fragment", url: "https://svgl.app/vscode-dark.svg?v=2#icon", want: "visual-studio-code-dark.svg"},
		{name: "extensionless", url: "https://svgl.app/icon", want: "visual-studio-code-dark.svg"},
		{name: "unsafe encoded extension", url: "https://svgl.app/icon.%3F", want: "visual-studio-code-dark.svg"},
		{name: "oversized extension", url: "https://svgl.app/icon.abcdefghijkl", want: "visual-studio-code-dark.svg"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := svgl.AssetFileName("Visual Studio Code", "dark", tt.url)
			if got != tt.want {
				t.Fatalf("got %q want %q", got, tt.want)
			}
		})
	}
}
