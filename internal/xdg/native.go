package xdg

import (
	"bufio"
	"encoding/xml"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type nativeApp struct {
	Path string
	ID   string
	Name string
	Icon string
}

func (f *Finder) findNativeApp(query string) (nativeApp, bool) {
	q := strings.TrimSpace(query)
	for _, app := range f.listNativeApps() {
		bundleName := strings.TrimSuffix(filepath.Base(app.Path), ".app")
		if strings.EqualFold(q, app.Name) || strings.EqualFold(q, app.ID) || strings.EqualFold(strings.TrimSuffix(q, ".app"), bundleName) {
			return app, true
		}
	}
	return nativeApp{}, false
}

func (f *Finder) listNativeApps() []nativeApp {
	var out []nativeApp
	seen := map[string]struct{}{}
	for _, root := range f.NativeAppDirs {
		_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			var app nativeApp
			var ok bool
			switch {
			case d.IsDir() && strings.EqualFold(filepath.Ext(d.Name()), ".app"):
				app, ok = readAppBundle(path)
				if !ok {
					return filepath.SkipDir
				}
			case !d.IsDir() && strings.EqualFold(filepath.Ext(d.Name()), ".url"):
				app, ok = readInternetShortcut(path)
			default:
				return nil
			}
			if ok {
				key := strings.ToLower(app.Path)
				if _, exists := seen[key]; !exists {
					seen[key] = struct{}{}
					out = append(out, app)
				}
			}
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		})
	}
	return out
}

func readAppBundle(bundle string) (nativeApp, bool) {
	values, err := plistStrings(filepath.Join(bundle, "Contents", "Info.plist"))
	if err != nil {
		return nativeApp{}, false
	}
	iconName := values["CFBundleIconFile"]
	if iconName == "" {
		return nativeApp{}, false
	}
	if filepath.Ext(iconName) == "" {
		iconName += ".icns"
	}
	icon := filepath.Join(bundle, "Contents", "Resources", filepath.Base(iconName))
	if st, err := os.Stat(icon); err != nil || !st.Mode().IsRegular() {
		return nativeApp{}, false
	}
	name := values["CFBundleDisplayName"]
	if name == "" {
		name = values["CFBundleName"]
	}
	if name == "" {
		name = strings.TrimSuffix(filepath.Base(bundle), ".app")
	}
	return nativeApp{Path: bundle, ID: values["CFBundleIdentifier"], Name: name, Icon: icon}, true
}

func plistStrings(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	dec := xml.NewDecoder(io.LimitReader(f, 2<<20))
	out := map[string]string{}
	var key string
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return out, nil
		}
		if err != nil {
			return nil, err
		}
		start, ok := tok.(xml.StartElement)
		if !ok || (start.Name.Local != "key" && start.Name.Local != "string") {
			continue
		}
		var text string
		if err := dec.DecodeElement(&text, &start); err != nil {
			return nil, err
		}
		if start.Name.Local == "key" {
			key = strings.TrimSpace(text)
		} else if key != "" {
			out[key] = strings.TrimSpace(text)
			key = ""
		}
	}
}

func readInternetShortcut(path string) (nativeApp, bool) {
	f, err := os.Open(path)
	if err != nil {
		return nativeApp{}, false
	}
	defer func() { _ = f.Close() }()
	icon := ""
	sc := bufio.NewScanner(io.LimitReader(f, 1<<20))
	for sc.Scan() {
		key, value, ok := strings.Cut(sc.Text(), "=")
		if ok && strings.EqualFold(strings.TrimSpace(key), "IconFile") {
			icon = strings.Trim(strings.TrimSpace(value), `"`)
		}
	}
	if sc.Err() != nil || icon == "" || !filepath.IsAbs(icon) {
		return nativeApp{}, false
	}
	if st, err := os.Stat(icon); err != nil || !st.Mode().IsRegular() {
		return nativeApp{}, false
	}
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	return nativeApp{Path: path, Name: name, Icon: icon}, true
}
