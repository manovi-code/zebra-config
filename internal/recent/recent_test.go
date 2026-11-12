package recent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRecentStoresOnlyIPNameAndModel(t *testing.T) {
	path := filepath.Join(t.TempDir(), "zebra-config", "recent.json")
	st, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)
	if err := st.Add(Printer{IP: "192.168.1.50", Name: "Shipping desk", Model: "ZD411", LastUsed: now}); err != nil {
		t.Fatal(err)
	}
	if err := st.Add(Printer{IP: "192.168.1.51", Name: "Pharmacy", Model: "ZD410", LastUsed: now.Add(time.Minute)}); err != nil {
		t.Fatal(err)
	}
	if err := st.Add(Printer{IP: "192.168.1.50", Name: "Shipping desk", Model: "ZD411", LastUsed: now.Add(2 * time.Minute)}); err != nil {
		t.Fatal(err)
	}
	list, err := st.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].IP != "192.168.1.50" || list[1].IP != "192.168.1.51" {
		t.Fatalf("%+v", list)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if stringsContains(string(body), "firmware") || stringsContains(string(body), "file.dir") || stringsContains(string(body), "settings") {
		t.Fatalf("recent file has extra data:\n%s", body)
	}
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatal(err)
	}
	if _, ok := raw["printers"]; !ok || len(raw) != 1 {
		t.Fatalf("top-level keys: %v", raw)
	}
	first := raw["printers"].([]any)[0].(map[string]any)
	for _, key := range []string{"ip", "name", "model", "lastUsed"} {
		if _, ok := first[key]; !ok {
			t.Fatalf("missing %s in %v", key, first)
		}
	}
	if len(first) != 4 {
		t.Fatalf("entry keys: %v", first)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode %v", info.Mode().Perm())
	}
}

func stringsContains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || (len(s) > 0 && contains(s, sub)))
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
