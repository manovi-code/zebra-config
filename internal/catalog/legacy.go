package catalog

import "strings"

// Gap is a naming field a 2824 did not report. Target is the ZD410/ZD411
// setting a technician can fill in on import.
type Gap struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Target string `json:"target"`
}

// LegacyProbeIDs are extra getvars for a 2824. They are not editable settings
// on a ZD410 or ZD411. The programming guide's external_wired names are the
// external ZebraNet print server. The unqualified ip.dhcp.cid_* names are the
// older print-server commands. Option 12 is documented for internal_wired, ip,
// and wlan, not for external_wired, so that probe often answers "?".
func LegacyProbeIDs() []string {
	leaves := []string{"cid_enable", "cid_type", "cid_prefix", "cid_suffix"}
	var ids []string
	for _, leaf := range leaves {
		ids = append(ids, "external_wired.ip.dhcp."+leaf)
		ids = append(ids, "ip.dhcp."+leaf)
	}
	ids = append(ids,
		"external_wired.ip.dhcp.option12",
		"external_wired.ip.dhcp.option12_format",
		"external_wired.ip.dhcp.option12_value",
		"ip.dhcp.option12_value",
	)
	return ids
}

// Answered reports whether getvar returned a value. "?" means the setting
// does not exist, per the getvar page.
func Answered(value string) bool {
	return strings.TrimSpace(value) != "?" && strings.TrimSpace(value) != ""
}

// LegacyMediaNote explains why print settings are not copied from a 2824.
const LegacyMediaNote = "Media type, darkness, speed, print width, label length, and sensor settings were not copied. A 2824 is 203 dpi, the same head class as a ZD410 or ZD411, but the 2824 Plus tops out at 4 inches per second and its darkness scale is not the ZD410/ZD411 scale. Set media and darkness on the new printer."

// Map2824 keeps the naming scheme and drops everything else.
// Client ID is copied onto the ZD410/ZD411 wired names. The first print
// server that actually answers is used: internal_wired, then external_wired,
// then the older ip.dhcp names.
func Map2824(values map[string]string) (settings map[string]string, gaps []Gap) {
	settings = map[string]string{}
	name := strings.TrimSpace(values["device.friendly_name"])
	if Answered(name) {
		settings["device.friendly_name"] = name
	} else {
		gaps = append(gaps, Gap{ID: "device.friendly_name", Label: "Printer name", Target: "device.friendly_name"})
	}
	branch := firstCIDBranch(values)
	leaves := []struct{ leaf, label string }{
		{"cid_enable", "DHCP client ID on/off"},
		{"cid_type", "DHCP client ID type"},
		{"cid_prefix", "DHCP client ID prefix"},
		{"cid_suffix", "DHCP client ID suffix"},
	}
	if branch == "" {
		gaps = append(gaps,
			Gap{ID: "internal_wired.ip.dhcp.cid_prefix", Label: "DHCP client ID prefix", Target: "internal_wired.ip.dhcp.cid_prefix"},
			Gap{ID: "internal_wired.ip.dhcp.cid_suffix", Label: "DHCP client ID suffix", Target: "internal_wired.ip.dhcp.cid_suffix"},
		)
	} else {
		for _, leaf := range leaves {
			src := cidID(branch, leaf.leaf)
			dst := "internal_wired.ip.dhcp." + leaf.leaf
			value := strings.TrimSpace(values[src])
			if value == "?" || value == "" {
				gaps = append(gaps, Gap{ID: src, Label: leaf.label, Target: dst})
				continue
			}
			settings[dst] = value
		}
	}
	if format, ok := firstHostName(values); ok {
		settings["internal_wired.ip.dhcp.option12_format"] = format
	} else {
		gaps = append(gaps, Gap{ID: "internal_wired.ip.dhcp.option12_format", Label: "DHCP host name", Target: "internal_wired.ip.dhcp.option12_format"})
	}
	return settings, gaps
}

func firstCIDBranch(values map[string]string) string {
	for _, branch := range []string{"internal_wired", "external_wired", "ip"} {
		for _, leaf := range []string{"cid_enable", "cid_type", "cid_prefix", "cid_suffix"} {
			if Answered(values[cidID(branch, leaf)]) {
				return branch
			}
		}
	}
	return ""
}

func cidID(branch, leaf string) string {
	if branch == "ip" {
		return "ip.dhcp." + leaf
	}
	return branch + ".ip.dhcp." + leaf
}

func firstHostName(values map[string]string) (string, bool) {
	for _, id := range []string{
		"internal_wired.ip.dhcp.option12_format",
		"ip.dhcp.option12_format",
		"external_wired.ip.dhcp.option12_format",
	} {
		if Answered(values[id]) {
			return strings.TrimSpace(values[id]), true
		}
	}
	return "", false
}

// LegacyRename builds the confirm list for a 2824 that is still responding.
// The name is renamed only when device.friendly_name actually answered.
// The client ID suffix is changed only on internal_wired, whose guide page
// documents setvar. external_wired is not written: SEH PS102-Z and PS105-Z
// accept getvar only. skip is a plain message when nothing can be written.
func LegacyRename(values map[string]string, includeSuffix map[string]bool) (changes []Change, notes []string, choices []SuffixChoice, retired, skip string) {
	name := strings.TrimSpace(values["device.friendly_name"])
	nameOK := Answered(name)
	choice, suffixOK := internalSuffixChoice(values)
	if !nameOK && !suffixOK {
		skip = "This 2824 was not renamed. It did not report a printer name this tool can change, and its client ID is not on a command this tool can write. Zebra's friendly-name note excludes the 2824 series, and SEH external print servers accept getvar only. If the printer is still on the network, change the name on the printer or the print server."
		return nil, nil, nil, "", skip
	}
	if nameOK {
		retired = RetiredName(name)
		changes = append(changes, Change{ID: "device.friendly_name", Value: retired})
		format := strings.TrimSpace(values["internal_wired.ip.dhcp.option12_format"])
		if format == "<device.friendly_name>" {
			notes = append(notes, "The DHCP host name follows the printer name, so it becomes "+retired+" when the name changes.")
		} else if format == name {
			changes = append(changes, Change{ID: "internal_wired.ip.dhcp.option12_format", Value: retired})
		}
	} else {
		notes = append(notes, "The printer name was not reported, so only the client ID suffix can be changed.")
	}
	if answeredExternal(values) && !suffixOK {
		notes = append(notes, "The client ID was read from the external print server. The guide says SEH PS102-Z and PS105-Z accept getvar only, so the suffix was not changed.")
	}
	if suffixOK {
		use := choice.Checked
		if includeSuffix != nil {
			use = includeSuffix[choice.ID]
			choice.Checked = use
		}
		choices = []SuffixChoice{choice}
		if use {
			changes = append(changes, Change{ID: choice.ID, Value: choice.To})
		}
	}
	return changes, notes, choices, retired, ""
}

func internalSuffixChoice(values map[string]string) (SuffixChoice, bool) {
	enable := strings.TrimSpace(values["internal_wired.ip.dhcp.cid_enable"])
	typ := strings.TrimSpace(values["internal_wired.ip.dhcp.cid_type"])
	suffix := values["internal_wired.ip.dhcp.cid_suffix"]
	if !Answered(enable) || !Answered(typ) || strings.TrimSpace(suffix) == "?" {
		return SuffixChoice{}, false
	}
	if typ != "0" {
		return SuffixChoice{}, false
	}
	next := strings.TrimSpace(suffix) + "_"
	if len(next) > 60 {
		return SuffixChoice{}, false
	}
	choice := SuffixChoice{
		ID:      "internal_wired.ip.dhcp.cid_suffix",
		Label:   "Wired client ID suffix",
		From:    strings.TrimSpace(suffix),
		To:      next,
		Enabled: true,
		Checked: strings.EqualFold(enable, "on"),
	}
	if choice.Checked {
		choice.Reason = "This 2824 is sending a text client ID on the internal print server. The new printer would send the same ID. Adding _ here lets the new one keep it."
	} else {
		choice.Reason = "The client ID is off, so DHCP is not using this suffix. Leave this unchecked unless you turn the client ID on."
	}
	return choice, true
}

func answeredExternal(values map[string]string) bool {
	for _, leaf := range []string{"cid_enable", "cid_type", "cid_prefix", "cid_suffix"} {
		if Answered(values["external_wired.ip.dhcp."+leaf]) {
			return true
		}
	}
	return false
}

// LegacyWriteOK reports whether every change is a 2824 rename this tool may send.
func LegacyWriteOK(values map[string]string, changes []Change) bool {
	if len(changes) == 0 {
		return false
	}
	for _, ch := range changes {
		switch ch.ID {
		case "device.friendly_name":
			if !Answered(values["device.friendly_name"]) {
				return false
			}
		case "internal_wired.ip.dhcp.option12_format":
			if !Answered(values[ch.ID]) {
				return false
			}
		case "internal_wired.ip.dhcp.cid_suffix":
			if _, ok := internalSuffixChoice(values); !ok {
				return false
			}
		default:
			return false
		}
	}
	return true
}
