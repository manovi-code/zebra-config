package catalog

import (
	"strings"
	"testing"
)

func TestMap2824KeepsNameAndSkipsMedia(t *testing.T) {
	values := map[string]string{
		"device.friendly_name":                   "PRN-LAB2",
		"print.tone":                             "15",
		"media.speed":                            "4",
		"ezpl.media_type":                        "gap/notch",
		"external_wired.ip.dhcp.cid_enable":      "on",
		"external_wired.ip.dhcp.cid_type":        "0",
		"external_wired.ip.dhcp.cid_prefix":      "?",
		"external_wired.ip.dhcp.cid_suffix":      "2824",
		"internal_wired.ip.dhcp.option12_format": "?",
	}
	settings, gaps := Map2824(values)
	if settings["device.friendly_name"] != "PRN-LAB2" {
		t.Fatalf("name %#v", settings)
	}
	if settings["internal_wired.ip.dhcp.cid_suffix"] != "2824" || settings["internal_wired.ip.dhcp.cid_enable"] != "on" {
		t.Fatalf("cid %#v", settings)
	}
	if _, ok := settings["print.tone"]; ok {
		t.Fatal("darkness was copied")
	}
	if _, ok := settings["internal_wired.ip.dhcp.cid_prefix"]; ok {
		t.Fatal("unreadable prefix was copied")
	}
	labels := map[string]bool{}
	for _, gap := range gaps {
		labels[gap.Label] = true
	}
	if !labels["DHCP client ID prefix"] || !labels["DHCP host name"] {
		t.Fatalf("gaps %+v", gaps)
	}
}

func TestMap2824NameMissing(t *testing.T) {
	settings, gaps := Map2824(map[string]string{
		"device.friendly_name":              "?",
		"external_wired.ip.dhcp.cid_suffix": "2824",
		"external_wired.ip.dhcp.cid_enable": "on",
		"external_wired.ip.dhcp.cid_type":   "0",
	})
	if _, ok := settings["device.friendly_name"]; ok {
		t.Fatal("missing name was copied")
	}
	found := false
	for _, gap := range gaps {
		if gap.Label == "Printer name" {
			found = true
		}
	}
	if !found {
		t.Fatalf("gaps %+v", gaps)
	}
	if settings["internal_wired.ip.dhcp.cid_suffix"] != "2824" {
		t.Fatalf("suffix %#v", settings)
	}
}

func TestLegacyRenameSkipsWhenNameMissing(t *testing.T) {
	_, _, _, _, skip := LegacyRename(map[string]string{
		"device.friendly_name":              "?",
		"external_wired.ip.dhcp.cid_enable": "on",
		"external_wired.ip.dhcp.cid_suffix": "2824",
		"internal_wired.ip.dhcp.cid_enable": "?",
	}, nil)
	if skip == "" || !strings.Contains(skip, "not renamed") {
		t.Fatalf("skip %q", skip)
	}
}

func TestLegacyRenameWritesInternalSuffixOnly(t *testing.T) {
	values := map[string]string{
		"device.friendly_name":                   "PRN-LAB2",
		"internal_wired.ip.dhcp.cid_enable":      "on",
		"internal_wired.ip.dhcp.cid_type":        "0",
		"internal_wired.ip.dhcp.cid_suffix":      "2824",
		"internal_wired.ip.dhcp.option12_format": "<device.friendly_name>",
		"external_wired.ip.dhcp.cid_suffix":      "EXT",
	}
	changes, notes, choices, retired, skip := LegacyRename(values, nil)
	if skip != "" || retired != "PRN-LAB2_" {
		t.Fatalf("skip %q retired %q", skip, retired)
	}
	got := map[string]string{}
	for _, ch := range changes {
		got[ch.ID] = ch.Value
	}
	if got["device.friendly_name"] != "PRN-LAB2_" || got["internal_wired.ip.dhcp.cid_suffix"] != "2824_" {
		t.Fatalf("changes %#v", got)
	}
	if _, ok := got["external_wired.ip.dhcp.cid_suffix"]; ok {
		t.Fatal("external suffix was queued for a write")
	}
	if len(choices) != 1 || !choices[0].Checked {
		t.Fatalf("choices %+v", choices)
	}
	if len(notes) == 0 {
		t.Fatal("expected a host-name note")
	}
	if !LegacyWriteOK(values, changes) {
		t.Fatal("rename changes should be allowed")
	}
	if LegacyWriteOK(values, []Change{{ID: "print.tone", Value: "10"}}) {
		t.Fatal("darkness must stay blocked on a 2824")
	}
}
