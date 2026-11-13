package catalog

import "testing"

func TestPowerSummaryAndClientIDNetwork(t *testing.T) {
	if got := PowerSummary("off", "180"); got != "Always on" {
		t.Fatalf("summary %q", got)
	}
	if got := PowerSummary("on", "180"); got != "Sleeps after 3 min" {
		t.Fatalf("summary %q", got)
	}
	if !IsNetwork("internal_wired.ip.dhcp.cid_prefix") || !IsNetwork("wlan.ip.dhcp.cid_suffix") {
		t.Fatal("client ID changes must require the network warning")
	}
	if IsNetwork("power.energy_star.enable") {
		t.Fatal("sleep is not a network change")
	}
	if _, ok := Canonical("power.energy_star.timeout", "179"); ok {
		t.Fatal("timeout below 180 should be rejected")
	}
	if _, ok := Canonical("internal_wired.ip.dhcp.cid_type", "1"); !ok {
		t.Fatal("cid type 1 should be allowed")
	}
	if _, ok := Canonical("internal_wired.ip.dhcp.cid_prefix", "LAB"); !ok {
		t.Fatal("prefix should be allowed")
	}
}

func TestReplacementNameAndPlan(t *testing.T) {
	if got := SuggestReplacementName("PRN-LAB2"); got != "PRN-LAB2_" {
		t.Fatalf("suggest %q", got)
	}
	long := "ABCDEFGHIJKLMNOPQRSTUVWXYZ123456"
	if len(long) != 32 {
		t.Fatalf("fixture length %d", len(long))
	}
	got := SuggestReplacementName(long)
	if len(got) != 32 || got[len(got)-1] != '_' {
		t.Fatalf("truncated %q", got)
	}
	settings := map[string]string{
		"device.friendly_name":                   "PRN-LAB2",
		"internal_wired.ip.dhcp.cid_prefix":      "LAB",
		"internal_wired.ip.addr":                 "10.1.1.9",
		"internal_wired.ip.dhcp.option12_format": "PRN-LAB2",
		"internal_wired.ip.dhcp.option12_value":  "PRN-LAB2",
		"print.tone":                             "10.0",
	}
	changes, notes, old, suggested := ReplacementPlan(settings, "", false)
	if old != "PRN-LAB2" || suggested != "PRN-LAB2_" {
		t.Fatalf("names %q %q", old, suggested)
	}
	gotVal := map[string]string{}
	for _, ch := range changes {
		gotVal[ch.ID] = ch.Value
		if ch.ID == "internal_wired.ip.addr" {
			t.Fatal("static IP was included")
		}
		if ch.ID == "internal_wired.ip.dhcp.option12_value" {
			t.Fatal("read-only host name was included")
		}
	}
	if gotVal["device.friendly_name"] != "PRN-LAB2_" {
		t.Fatalf("name %q", gotVal["device.friendly_name"])
	}
	if gotVal["internal_wired.ip.dhcp.cid_prefix"] != "LAB" {
		t.Fatal("client ID prefix missing")
	}
	if gotVal["internal_wired.ip.dhcp.option12_format"] != "PRN-LAB2_" {
		t.Fatalf("hostname %q", gotVal["internal_wired.ip.dhcp.option12_format"])
	}
	if len(notes) == 0 {
		t.Fatal("expected a note about the IP address")
	}
	withIP, _, _, _ := ReplacementPlan(settings, "FRONT_", true)
	found := false
	for _, ch := range withIP {
		if ch.ID == "internal_wired.ip.addr" && ch.Value == "10.1.1.9" {
			found = true
		}
		if ch.ID == "device.friendly_name" && ch.Value != "FRONT_" {
			t.Fatalf("custom name %q", ch.Value)
		}
	}
	if !found {
		t.Fatal("opt-in static IP was missing")
	}
}
