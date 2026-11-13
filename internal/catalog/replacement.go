package catalog

import "strings"

// RetiredName is the name left on the old printer: the current name plus one underscore.
func RetiredName(old string) string {
	old = strings.TrimSpace(old)
	if old == "" || old == "?" {
		return "Printer_"
	}
	name := old + "_"
	if len(name) <= 32 {
		return name
	}
	return old[:31] + "_"
}

// SuggestReplacementName is kept for callers that mean the name left on the old printer.
func SuggestReplacementName(old string) string {
	return RetiredName(old)
}

// ReplacementPlan builds the changes for loading one printer's settings onto another.
// The new friendly name defaults to the old name exactly. The underscore is offered
// on the old printer, in a separate confirm, when that printer is still connected.
// Static IP settings are included only when includeStatic is set.
func ReplacementPlan(settings map[string]string, newName string, includeStatic bool) (changes []Change, notes []string, oldName, suggested string) {
	oldName = strings.TrimSpace(settings["device.friendly_name"])
	suggested = oldName
	if suggested == "" || suggested == "?" {
		suggested = "Printer"
	}
	if strings.TrimSpace(newName) == "" {
		newName = suggested
	}
	for id, value := range settings {
		s, ok := byID[id]
		if !ok {
			notes = append(notes, "Skipped unknown setting "+id+".")
			continue
		}
		if s.ReadOnly {
			continue
		}
		if StaticIP(id) && !includeStatic {
			continue
		}
		if id == "device.friendly_name" {
			value = newName
		}
		if strings.HasSuffix(id, "option12_format") && strings.TrimSpace(value) == oldName && oldName != "" {
			value = newName
		}
		changes = append(changes, Change{ID: id, Value: value})
	}
	hasName := false
	for _, ch := range changes {
		if ch.ID == "device.friendly_name" {
			hasName = true
		}
	}
	if !hasName {
		changes = append(changes, Change{ID: "device.friendly_name", Value: newName})
	}
	if !includeStatic {
		notes = append(notes, "The old IP address, mask, gateway, and addressing method were left out. Check the box only if the old printer is off the network.")
	}
	return changes, notes, oldName, suggested
}

// SuffixChoice is one DHCP client-ID suffix the old printer could change.
type SuffixChoice struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	From    string `json:"from"`
	To      string `json:"to"`
	Checked bool   `json:"checked"`
	Enabled bool   `json:"enabled"`
	Reason  string `json:"reason"`
}

// OldRenamePlan is the confirm list for the printer that is still responding.
// includeSuffix nil means "use each choice's default". An empty map means the
// tech turned every suffix off. A set map includes only those ids.
func OldRenamePlan(values map[string]string, includeSuffix map[string]bool) (changes []Change, notes []string, choices []SuffixChoice, retired string) {
	oldName := strings.TrimSpace(values["device.friendly_name"])
	retired = RetiredName(oldName)
	changes = append(changes, Change{ID: "device.friendly_name", Value: retired})
	for _, host := range []struct{ id, label string }{
		{"internal_wired.ip.dhcp.option12_format", "Wired"},
		{"ip.dhcp.option12_format", "Wi-Fi"},
	} {
		format := strings.TrimSpace(values[host.id])
		if format == "<device.friendly_name>" {
			notes = append(notes, host.label+" DHCP host name follows the friendly name, so it becomes "+retired+" when the name changes.")
			continue
		}
		if oldName != "" && format == oldName {
			changes = append(changes, Change{ID: host.id, Value: retired})
		}
	}
	choices = suffixChoices(values)
	for i := range choices {
		choice := &choices[i]
		if !choice.Enabled {
			notes = append(notes, choice.Reason)
			continue
		}
		use := choice.Checked
		if includeSuffix != nil {
			use = includeSuffix[choice.ID]
			choice.Checked = use
		}
		if use {
			changes = append(changes, Change{ID: choice.ID, Value: choice.To})
		}
	}
	return changes, notes, choices, retired
}

func suffixChoices(values map[string]string) []SuffixChoice {
	branches := []struct{ id, label string }{
		{"internal_wired", "Wired"},
		{"wlan", "Wi-Fi"},
	}
	var out []SuffixChoice
	for _, branch := range branches {
		enable := strings.TrimSpace(values[branch.id+".ip.dhcp.cid_enable"])
		typ := strings.TrimSpace(values[branch.id+".ip.dhcp.cid_type"])
		suffix := values[branch.id+".ip.dhcp.cid_suffix"]
		if enable == "?" || enable == "" && typ == "" && strings.TrimSpace(suffix) == "" {
			continue
		}
		choice := SuffixChoice{
			ID:    branch.id + ".ip.dhcp.cid_suffix",
			Label: branch.label + " client ID suffix",
			From:  suffix,
		}
		switch typ {
		case "2":
			choice.Reason = branch.label + " client ID type is hexadecimal. The guide only allows hex characters in the suffix, and _ is not one of them, so it is not added."
		case "1":
			choice.Reason = branch.label + " client ID is the printer's MAC address. The guide does not send the suffix for that type, so adding _ would not change the ID DHCP sees. Each printer already has its own MAC."
		case "0":
			next := suffix + "_"
			if len(next) > 60 {
				choice.Reason = branch.label + " client ID suffix is already at the guide's 60 character limit for text, so _ was not added."
				break
			}
			choice.Enabled = true
			choice.To = next
			choice.Checked = strings.EqualFold(enable, "on")
			if choice.Checked {
				choice.Reason = branch.label + " client ID is on. The new printer would send the same ID. Adding _ on this printer lets the new one keep it."
			} else {
				choice.Reason = branch.label + " client ID is off, so DHCP is not using this suffix. Leave this unchecked unless you turn the client ID on."
			}
		default:
			choice.Reason = branch.label + " client ID type was not 0, 1, or 2, so the suffix was left alone."
		}
		out = append(out, choice)
	}
	return out
}
