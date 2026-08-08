package xdg

import (
	"bufio"
	"crypto/sha256"
	"encoding/xml"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

type nativeApp struct {
	Path string
	ID   string
	Name string
	Icon string
}

type nativeAppIndexEntry struct {
	signature string
	apps      []nativeApp
}

var nativeAppIndex = struct {
	sync.Mutex
	entries map[string]nativeAppIndexEntry
}{entries: map[string]nativeAppIndexEntry{}}

func (f *Finder) findNativeApp(query string) (nativeApp, bool) {
	q := strings.TrimSpace(query)
	for _, app := range f.listNativeApps() {
		fileName := trimNativeAppExtension(filepath.Base(app.Path))
		if strings.EqualFold(q, app.Name) || strings.EqualFold(q, app.ID) || strings.EqualFold(trimNativeAppExtension(q), fileName) {
			return app, true
		}
	}
	return nativeApp{}, false
}

func (f *Finder) listNativeApps() []nativeApp {
	key, signature := nativeAppIndexIdentity(f.NativeAppDirs)
	nativeAppIndex.Lock()
	defer nativeAppIndex.Unlock()
	if cached, ok := nativeAppIndex.entries[key]; ok && cached.signature == signature {
		return append([]nativeApp(nil), cached.apps...)
	}
	apps := scanNativeApps(f.NativeAppDirs)
	nativeAppIndex.entries[key] = nativeAppIndexEntry{signature: signature, apps: append([]nativeApp(nil), apps...)}
	return apps
}

func scanNativeApps(roots []string) []nativeApp {
	var out []nativeApp
	seen := map[string]struct{}{}
	for _, root := range roots {
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

func trimNativeAppExtension(name string) string {
	ext := strings.ToLower(filepath.Ext(name))
	if ext == ".app" || ext == ".url" {
		return strings.TrimSuffix(name, filepath.Ext(name))
	}
	return name
}

func nativeAppIndexIdentity(roots []string) (string, string) {
	var key, signature strings.Builder
	for _, root := range roots {
		clean := filepath.Clean(root)
		key.WriteString(clean)
		key.WriteByte(0)
		signature.WriteString(clean)
		if _, err := os.Stat(clean); err != nil {
			signature.WriteString("|missing")
		} else {
			_ = filepath.WalkDir(clean, func(path string, d os.DirEntry, walkErr error) error {
				if walkErr != nil {
					signature.WriteByte('|')
					signature.WriteString(path)
					signature.WriteString("|unreadable")
					return nil
				}
				if d.IsDir() {
					appendNativePathSignature(&signature, clean, path, 0)
					if strings.EqualFold(filepath.Ext(d.Name()), ".app") {
						appendAppBundleSignature(&signature, clean, path)
						return filepath.SkipDir
					}
				} else if strings.EqualFold(filepath.Ext(d.Name()), ".url") {
					appendNativePathSignature(&signature, clean, path, 1<<20)
				}
				return nil
			})
		}
		signature.WriteByte(0)
	}
	return key.String(), signature.String()
}

func appendAppBundleSignature(signature *strings.Builder, root, bundle string) {
	plist := filepath.Join(bundle, "Contents", "Info.plist")
	appendNativePathSignature(signature, root, plist, 2<<20)
	values, err := plistStrings(plist)
	if err != nil {
		return
	}
	iconName := values["CFBundleIconFile"]
	if iconName == "" {
		return
	}
	if filepath.Ext(iconName) == "" {
		iconName += ".icns"
	}
	appendNativePathSignature(signature, root, filepath.Join(bundle, "Contents", "Resources", filepath.Base(iconName)), 0)
}

func appendNativePathSignature(signature *strings.Builder, root, path string, hashLimit int64) {
	signature.WriteByte('|')
	if rel, err := filepath.Rel(root, path); err == nil {
		signature.WriteString(rel)
	} else {
		signature.WriteString(path)
	}
	info, err := os.Stat(path)
	if err != nil {
		signature.WriteString("|missing")
		return
	}
	signature.WriteByte('|')
	signature.WriteString(info.ModTime().UTC().String())
	signature.WriteByte('|')
	signature.WriteString(info.Mode().String())
	signature.WriteByte('|')
	signature.WriteString(strconv.FormatInt(info.Size(), 10))
	if hashLimit <= 0 || !info.Mode().IsRegular() {
		return
	}
	f, err := os.Open(path)
	if err != nil {
		signature.WriteString("|unreadable")
		return
	}
	data, readErr := io.ReadAll(io.LimitReader(f, hashLimit+1))
	_ = f.Close()
	if readErr != nil {
		signature.WriteString("|unreadable")
		return
	}
	sum := sha256.Sum256(data)
	signature.WriteByte('|')
	_, _ = signature.Write(sum[:])
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
