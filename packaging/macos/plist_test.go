package macos

import (
	"os"
	"strings"
	"testing"
)

func TestInfoPlistIsADoubleClickApp(t *testing.T) {
	body, err := os.ReadFile("Info.plist")
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, needle := range []string{
		"<key>CFBundleExecutable</key>",
		"<string>zebra-config</string>",
		"<key>CFBundleIdentifier</key>",
		"<string>com.manovi.zebraconfig</string>",
		"<key>CFBundleName</key>",
		"<string>Zebra Config</string>",
		"<key>CFBundlePackageType</key>",
		"<string>APPL</string>",
		"<key>LSUIElement</key>",
		"<true/>",
		"__VERSION__",
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("Info.plist missing %s", needle)
		}
	}
}
