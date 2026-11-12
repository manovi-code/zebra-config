package printer

import (
	"strings"
	"testing"
	"time"

	"github.com/manovi-code/zebra-config/internal/catalog"
	"github.com/manovi-code/zebra-config/internal/sim"
)

func startSim(t *testing.T, opt sim.Options) *sim.Server {
	t.Helper()
	s, err := sim.Listen("127.0.0.1:0", opt)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestReadAndApplyAgainstSimulator(t *testing.T) {
	s := startSim(t, sim.Options{})
	c, snap, err := Open("127.0.0.1", s.Port(), DefaultTimeouts())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	if !snap.Identity.Supported || snap.Readonly {
		t.Fatalf("identity %+v", snap.Identity)
	}
	if snap.Identity.ProductName != "ZD411" || snap.Identity.DPI != 203 {
		t.Fatalf("identity %+v", snap.Identity)
	}
	if !strings.Contains(snap.Identity.HostID, "ZD411") || !strings.Contains(snap.Identity.HostID, ",8,") {
		t.Fatalf("host id %q", snap.Identity.HostID)
	}
	if snap.Identity.Status != "Ready" {
		t.Fatalf("status %q", snap.Identity.Status)
	}
	if snap.Values["print.tone"] != "10.0" || snap.Values["ezpl.media_type"] != "gap/notch" {
		t.Fatalf("values %+v", snap.Values)
	}

	result, err := Apply(c, snap.Values, []catalog.Change{
		{ID: "print.tone", Value: "15"},
		{ID: "ezpl.media_type", Value: "continuous"},
		{ID: "media.printmode", Value: "peel off"},
	}, true, DefaultTimeouts(), snap.ConnectedTo)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Mismatches) != 0 {
		t.Fatalf("mismatches %+v", result.Mismatches)
	}
	if !result.Saved {
		t.Fatal("expected save")
	}
	if result.Snapshot.Values["print.tone"] != "15.0" || result.Snapshot.Values["ezpl.media_type"] != "continuous" {
		t.Fatalf("after %+v", result.Snapshot.Values)
	}
	if result.Snapshot.Values["media.printmode"] != "peel off" && result.Snapshot.Values["media.printmode"] != "P" {
		t.Fatalf("print mode %q", result.Snapshot.Values["media.printmode"])
	}
	joined := strings.Join(s.Commands(), "\n")
	if !strings.Contains(joined, `! U1 setvar "media.printmode" "P"`) {
		t.Fatalf("print mode was not sent as the guide's setvar code:\n%s", joined)
	}
	if !strings.Contains(joined, "^XA^JUS^XZ") {
		t.Fatal("save command was not sent")
	}
}

func TestApplyReportsValueThatDidNotStick(t *testing.T) {
	s := startSim(t, sim.Options{})
	s.SetSticky("print.tone", false)
	c, snap, err := Open("127.0.0.1", s.Port(), DefaultTimeouts())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	result, err := Apply(c, snap.Values, []catalog.Change{{ID: "print.tone", Value: "18"}}, false, DefaultTimeouts(), snap.ConnectedTo)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Mismatches) != 1 || result.Mismatches[0].ID != "print.tone" {
		t.Fatalf("mismatches %+v", result.Mismatches)
	}
	if s.Get("print.tone") != "10.0" {
		t.Fatalf("tone changed to %s", s.Get("print.tone"))
	}
}

func TestRestartDropsUnsavedSettings(t *testing.T) {
	s := startSim(t, sim.Options{})
	c, snap, err := Open("127.0.0.1", s.Port(), DefaultTimeouts())
	if err != nil {
		t.Fatal(err)
	}
	result, err := Apply(c, snap.Values, []catalog.Change{{ID: "print.tone", Value: "12"}}, false, DefaultTimeouts(), snap.ConnectedTo)
	if err != nil {
		t.Fatal(err)
	}
	spec, err := LookupAction("restart")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := RunAction(c, spec, false, DefaultTimeouts(), snap.ConnectedTo); err != nil {
		t.Fatal(err)
	}
	_ = c.Close()
	if result.Snapshot.Values["print.tone"] != "12.0" {
		t.Fatalf("tone before restart %s", result.Snapshot.Values["print.tone"])
	}
	c2, again, err := Open("127.0.0.1", s.Port(), DefaultTimeouts())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c2.Close() })
	if again.Values["print.tone"] != "10.0" {
		t.Fatalf("unsaved tone survived restart: %s", again.Values["print.tone"])
	}
}

func TestDecide(t *testing.T) {
	ok := Decide("ZD411", "203", 8, "V84.20.20Z", "ZD411,V84.20.20Z,8,8192KB")
	if !ok.Supported || ok.Readonly {
		t.Fatalf("%+v", ok)
	}
	// The programming guide lists 200 for this head. Treat it as 203 dpi.
	also := Decide("ZD410", "200", 8, "V1", "ZD410,V1,8,8192KB")
	if !also.Supported || also.DPI != 203 {
		t.Fatalf("%+v", also)
	}
	dpi := Decide("ZD411", "300", 12, "V1", "ZD411,V1,12,8192KB")
	if dpi.Supported || !strings.Contains(dpi.Message, "300 dpi") {
		t.Fatalf("%+v", dpi)
	}
	other := Decide("ZD621", "203", 8, "V1", "ZD621,V1,8,8192KB")
	if other.Supported || !strings.Contains(other.Message, "ZD621") {
		t.Fatalf("%+v", other)
	}
	unknown := Decide("ZD411", "", 0, "V1", "")
	if unknown.Supported || !strings.Contains(unknown.Message, "resolution") {
		t.Fatalf("%+v", unknown)
	}
}

func TestParseHostReplies(t *testing.T) {
	_, _, dpm, pretty := ParseHI("\x02ZD411,V84.20.20Z,8,8192KB\x03")
	if dpm != 8 || !strings.Contains(pretty, "ZD411") {
		t.Fatalf("dpm %d pretty %q", dpm, pretty)
	}
	if got := ParseHS("\x02030,1,0,1225,000,0,0,0,000,0,0,0\x03\n\x02000,0,1,0,0,2,0,0,00000000,1,000\x03\n\x020000,1\x03\n"); got != "Out of paper, Printhead is open" {
		t.Fatalf("status %q", got)
	}
	if got := ParseHS("\x02030,0,0,1225,000,0,0,0,000,0,0,0\x03\n\x02000,0,0,0,0,2,0,0,00000000,1,000\x03\n"); got != "Ready" {
		t.Fatalf("status %q", got)
	}
}

func TestQuotedGetvarAgainstSimulator(t *testing.T) {
	s := startSim(t, sim.Options{Model: "ZD410", DPI: 203})
	c, snap, err := Open("127.0.0.1", s.Port(), Timeouts{Dial: time.Second, Read: time.Second, Status: 500 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	if snap.Identity.ProductName != "ZD410" || !snap.Identity.Supported {
		t.Fatalf("%+v", snap.Identity)
	}
}
