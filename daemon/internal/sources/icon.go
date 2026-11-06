package sources

import (
	"encoding/base64"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

// A plugin's icon is a Material Symbols name or an image inside its
// package. The daemon hands images to the app as data: URLs, so the app
// never fetches an icon from the network.

// MaxIcon is the largest icon image a package may carry.
const MaxIcon = 256 << 10

var (
	symbolRE  = regexp.MustCompile(`^[a-z0-9_]{1,40}$`)
	dataURLRE = regexp.MustCompile(`^data:image/(png|webp|svg\+xml);base64,[A-Za-z0-9+/]+={0,2}$`)
	iconTypes = map[string]string{".png": "image/png", ".webp": "image/webp", ".svg": "image/svg+xml"}
)

// packageIcon resolves a manifest icon against the package in dir: a
// symbol name stays as it is, an image path becomes a data: URL.
func packageIcon(dir, icon string) (string, error) {
	if icon == "" || symbolRE.MatchString(icon) {
		return icon, nil
	}
	clean := path.Clean(icon)
	mime := iconTypes[strings.ToLower(path.Ext(clean))]
	if mime == "" || path.IsAbs(icon) || strings.Contains(icon, "\\") || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", errorf(KindInvalid, "插件图标 %q 应为 Material Symbols 名称，或插件内的 PNG、WebP、SVG 文件", icon)
	}
	fi, err := os.Lstat(filepath.Join(dir, filepath.FromSlash(clean)))
	if err != nil || !fi.Mode().IsRegular() {
		return "", errorf(KindInvalid, "找不到插件图标 %s", icon)
	}
	if fi.Size() > MaxIcon {
		return "", errorf(KindInvalid, "插件图标 %s 超过 256 KiB", icon)
	}
	b, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(clean)))
	if err != nil {
		return "", errorf(KindInvalid, "无法读取插件图标：%v", err)
	}
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(b), nil
}

// listedIcon keeps an index entry's icon only in a form the app renders:
// a symbol name or an image data: URL within the size limit.
func listedIcon(icon string) string {
	if symbolRE.MatchString(icon) || (len(icon) <= MaxIcon*4/3+64 && dataURLRE.MatchString(icon)) {
		return icon
	}
	return ""
}
