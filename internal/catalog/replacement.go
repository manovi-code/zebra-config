package catalog

import "strings"

// SuggestReplacementName is the old name plus one underscore, trimmed to 32 characters.
func SuggestReplacementName(old string) string {
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

// ReplacementPlan builds the changes for loading one printer's settings onto another.
// Client ID and the rest of the config are included. The new friendly name defaults
// to the old name plus "_". Static IP settings are included only when includeStatic is set.
func ReplacementPlan(settings map[string]string, newName string, includeStatic bool) (changes []Change, notes []string, oldName, suggested string) {
	oldName = strings.TrimSpace(settings["device.friendly_name"])
	suggested = SuggestReplacementName(oldName)
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
