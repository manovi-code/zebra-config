package httpserver

import (
	"bytes"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/manovi-code/zebra-config/internal/printer"
	"github.com/manovi-code/zebra-config/internal/sim"
)

func startUI(t *testing.T, timeouts printer.Timeouts) string {
	t.Helper()
	if timeouts.Read == 0 {
		timeouts = printer.Timeouts{Dial: time.Second, Read: 2 * time.Second, Status: 500 * time.Millisecond}
	}
	srv, err := New(filepath.Join(t.TempDir(), "recent.json"), timeouts)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := srv.Listen("0.0.0.0:0"); err == nil {
		t.Fatal("UI listened on a non-loopback address")
	}
	addr, err := srv.Listen("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = srv.Serve() }()
	t.Cleanup(func() { _ = srv.Close() })
	return "http://" + addr
}

func postJSON(t *testing.T, url string, body any) (int, map[string]any) {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	res, err := http.Post(url, "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &out); err != nil {
			t.Fatalf("status %d body %s", res.StatusCode, raw)
		}
	}
	return res.StatusCode, out
}

func TestQuitStopsTheServer(t *testing.T) {
	base := startUI(t, printer.Timeouts{})
	status, out := postJSON(t, base+"/api/quit", map[string]any{})
	if status != 200 || out["ok"] != true {
		t.Fatalf("quit %d %+v", status, out)
	}
	deadline := time.Now().Add(3 * time.Second)
	client := &http.Client{Timeout: 200 * time.Millisecond}
	for time.Now().Before(deadline) {
		res, err := client.Get(base + "/")
		if err != nil {
			return
		}
		res.Body.Close()
		time.Sleep(30 * time.Millisecond)
	}
	t.Fatal("the page was still up after Quit")
}

func TestPageAndErrors(t *testing.T) {
	base := startUI(t, printer.Timeouts{})
	res, err := http.Get(base + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != 200 || !strings.Contains(string(body), "Zebra Config") || !strings.Contains(string(body), `id="quit-btn"`) {
		t.Fatalf("index %d %s", res.StatusCode, body)
	}
	for _, path := range []string{"/app.css", "/app.js"} {
		r, err := http.Get(base + path)
		if err != nil {
			t.Fatal(err)
		}
		r.Body.Close()
		if r.StatusCode != 200 {
			t.Fatalf("%s -> %d", path, r.StatusCode)
		}
	}
	fav, err := http.Get(base + "/favicon.ico")
	if err != nil {
		t.Fatal(err)
	}
	fav.Body.Close()
	if fav.StatusCode != 204 {
		t.Fatalf("favicon %d", fav.StatusCode)
	}

	status, out := postJSON(t, base+"/api/connect", map[string]any{"ip": "not-an-ip", "port": 9100})
	if status != 200 || out["ok"] != false || !strings.Contains(out["error"].(string), "IPv4") {
		t.Fatalf("invalid ip %d %+v", status, out)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	refused := ln.Addr().String()
	ln.Close()
	host, port, _ := net.SplitHostPort(refused)
	status, out = postJSON(t, base+"/api/connect", map[string]any{"ip": host, "port": atoi(port)})
	if status != 200 || out["code"] != "refused" || !strings.Contains(strings.ToLower(out["error"].(string)), "refused") {
		t.Fatalf("refused %d %+v", status, out)
	}
}

func TestHangTimesOut(t *testing.T) {
	hang, err := sim.Listen("127.0.0.1:0", sim.Options{Hang: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = hang.Close() })
	base := startUI(t, printer.Timeouts{Dial: 300 * time.Millisecond, Read: 300 * time.Millisecond, Status: 200 * time.Millisecond})
	status, out := postJSON(t, base+"/api/connect", map[string]any{"ip": "127.0.0.1", "port": hang.Port()})
	if status != 200 || out["code"] != "timeout" || !strings.Contains(strings.ToLower(out["error"].(string)), "did not answer") {
		t.Fatalf("timeout %d %+v", status, out)
	}
}

func TestConnectApplyImportAndConsole(t *testing.T) {
	printerSim, err := sim.Listen("127.0.0.1:0", sim.Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = printerSim.Close() })
	base := startUI(t, printer.Timeouts{})

	status, out := postJSON(t, base+"/api/connect", map[string]any{"ip": "127.0.0.1", "port": printerSim.Port()})
	if status != 200 || out["ok"] != true {
		t.Fatalf("connect %d %+v", status, out)
	}
	snap := out["snapshot"].(map[string]any)
	if snap["readonly"] != false {
		t.Fatalf("snapshot %+v", snap)
	}
	names := sectionNames(snap)
	for _, want := range []string{"Printer", "Media", "Print", "Sensor", "Network"} {
		if !names[want] {
			t.Fatalf("sections %+v", names)
		}
	}

	status, out = postJSON(t, base+"/api/apply", map[string]any{
		"changes": []map[string]string{{"id": "print.tone", "value": "16"}},
		"confirm": false,
	})
	if status != 200 || out["code"] != "needs_confirm" || printerSim.Get("print.tone") != "10.0" {
		t.Fatalf("unconfirmed apply %d tone %s %+v", status, printerSim.Get("print.tone"), out)
	}

	status, out = postJSON(t, base+"/api/apply", map[string]any{
		"changes": []map[string]string{
			{"id": "print.tone", "value": "16"},
			{"id": "ezpl.media_type", "value": "continuous"},
		},
		"confirm": true,
		"save":    true,
	})
	if status != 200 || printerSim.Get("print.tone") != "16.0" || printerSim.Get("ezpl.media_type") != "continuous" {
		t.Fatalf("apply %d tone %s media %s %+v", status, printerSim.Get("print.tone"), printerSim.Get("ezpl.media_type"), out)
	}
	if mismatches, _ := out["mismatches"].([]any); len(mismatches) != 0 {
		t.Fatalf("mismatches %+v", out["mismatches"])
	}

	status, out = postJSON(t, base+"/api/apply", map[string]any{
		"changes":      []map[string]string{{"id": "internal_wired.ip.addr", "value": "10.1.2.3"}},
		"confirm":      true,
		"extraConfirm": false,
	})
	if status != 200 || out["code"] != "needs_extra" || !strings.Contains(strings.ToLower(out["error"].(string)), "network") {
		t.Fatalf("network gate %d %+v", status, out)
	}
	if printerSim.Get("internal_wired.ip.addr") != "192.168.1.50" {
		t.Fatalf("addr changed without extra confirm: %s", printerSim.Get("internal_wired.ip.addr"))
	}

	status, blocked := postJSON(t, base+"/api/console", map[string]any{"command": `! U1 getvar "file.dir"`})
	if status != 200 || blocked["code"] != "blocked" {
		t.Fatalf("file.dir %d %+v", status, blocked)
	}
	for _, line := range printerSim.Commands() {
		if strings.Contains(strings.ToLower(line), "file.") {
			t.Fatalf("file command reached the printer: %q", line)
		}
	}
	status, hi := postJSON(t, base+"/api/console", map[string]any{"command": "~HI"})
	if status != 200 || !strings.Contains(hi["response"].(string), "ZD411") || !strings.Contains(hi["response"].(string), "<STX>") {
		t.Fatalf("console ~HI %d %+v", status, hi)
	}

	res, err := http.Get(base + "/api/export")
	if err != nil {
		t.Fatal(err)
	}
	exportBody, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if strings.Contains(string(exportBody), "file.dir") || strings.Contains(string(exportBody), "^FD") {
		t.Fatalf("export leaked non-settings: %s", exportBody)
	}
	var doc map[string]any
	if err := json.Unmarshal(exportBody, &doc); err != nil {
		t.Fatal(err)
	}
	if doc["zebraConfig"] != float64(1) {
		t.Fatalf("export %+v", doc)
	}
	settings := doc["settings"].(map[string]any)
	if _, ok := doc["ip"]; ok {
		t.Fatal("export included a connection ip field")
	}
	settings["print.tone"] = "10.0"
	settings["ezpl.media_type"] = "gap/notch"
	settings["internal_wired.ip.addr"] = "10.9.9.9"
	status, imported := postJSON(t, base+"/api/import", map[string]any{
		"includeNetwork": false,
		"document":       doc,
	})
	if status != 200 {
		t.Fatalf("import %d %+v", status, imported)
	}
	rows := imported["rows"].([]any)
	sawTone := false
	for _, row := range rows {
		id := row.(map[string]any)["id"]
		if id == "internal_wired.ip.addr" {
			t.Fatal("network setting was included without the opt-in")
		}
		if id == "print.tone" {
			sawTone = true
		}
	}
	if !sawTone {
		t.Fatalf("import rows %+v", rows)
	}
	if imported["needsExtra"] != false {
		t.Fatalf("import extra %+v", imported)
	}

	status, withNet := postJSON(t, base+"/api/import", map[string]any{
		"includeNetwork": true,
		"document":       doc,
	})
	if status != 200 || withNet["needsExtra"] != true {
		t.Fatalf("import network %d %+v", status, withNet)
	}

	badDoc := map[string]any{
		"zebraConfig": 1,
		"exportedAt":  "2026-11-12T00:00:00Z",
		"tool":        "zebra-config",
		"printer":     map[string]any{"productName": "ZD411", "firmware": "V", "dpi": 203},
		"settings":    map[string]string{"file.dir": "E:"},
	}
	status, bad := postJSON(t, base+"/api/import", map[string]any{"document": badDoc})
	if status != 200 || bad["code"] != "bad_file" {
		t.Fatalf("bad import %d %+v", status, bad)
	}
}

func TestUnsupportedIsReadOnly(t *testing.T) {
	other, err := sim.Listen("127.0.0.1:0", sim.Options{Model: "ZD621", DPI: 300})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = other.Close() })
	base := startUI(t, printer.Timeouts{})
	status, out := postJSON(t, base+"/api/connect", map[string]any{"ip": "127.0.0.1", "port": other.Port()})
	if status != 200 {
		t.Fatalf("connect %d %+v", status, out)
	}
	snap := out["snapshot"].(map[string]any)
	if snap["readonly"] != true || !strings.Contains(snap["message"].(string), "ZD621") {
		t.Fatalf("snapshot %+v", snap)
	}
	before := other.Get("print.tone")
	status, denied := postJSON(t, base+"/api/apply", map[string]any{
		"changes": []map[string]string{{"id": "print.tone", "value": "20"}},
		"confirm": true, "extraConfirm": true, "save": true,
	})
	if status != 200 || denied["code"] != "readonly" || other.Get("print.tone") != before {
		t.Fatalf("readonly apply %d tone %s %+v", status, other.Get("print.tone"), denied)
	}
	status, action := postJSON(t, base+"/api/action", map[string]any{"action": "calibrate", "confirm": true})
	if status != 200 || action["code"] != "readonly" {
		t.Fatalf("readonly action %d %+v", status, action)
	}
}

func sectionNames(snap map[string]any) map[string]bool {
	out := map[string]bool{}
	for _, raw := range snap["sections"].([]any) {
		out[raw.(map[string]any)["name"].(string)] = true
	}
	return out
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		n = n*10 + int(c-'0')
	}
	return n
}
