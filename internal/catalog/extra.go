package catalog

import (
	"fmt"
	"strconv"
	"strings"
)

// extraSettings are DHCP client ID, DHCP host name, and Energy Star.
// Names and values are from the ZPL Programming Guide. See docs/sgd-variables.md.
func extraSettings() []Setting {
	var out []Setting
	out = append(out, powerSettings()...)
	out = append(out, cidGroup("internal_wired", "Wired")...)
	out = append(out, cidGroup("wlan", "Wi-Fi")...)
	out = append(out, hostnameSettings()...)
	return out
}

func powerSettings() []Setting {
	return []Setting{
		{
			ID: "power.energy_star.enable", Label: "Keep printer always on", Section: "Power",
			Kind: KindCheck, CheckedValue: "off", UncheckedValue: "on",
			Help: "Checked turns off Energy Star sleep (power.energy_star.enable off). Unchecked lets the printer sleep after the time below. The programming guide does not list power.sleep.enable for desktop printers; that command is for mobile printers and is not sent.",
		},
		{
			ID: "power.energy_star.timeout", Label: "Sleep after (seconds)", Section: "Power",
			Kind: KindNumber, Min: 180, Max: 65535, Decimals: 0, Step: "1",
			Help: "Idle time before Energy Star sleep, in seconds. 180 is 3 minutes. The guide allows 180 to 65535. This is used when the printer is allowed to sleep.",
		},
	}
}

func cidGroup(branch, label string) []Setting {
	id := func(leaf string) string { return branch + ".ip.dhcp." + leaf }
	return []Setting{
		{
			ID: id("cid_enable"), Label: label + " DHCP client ID", Section: "Network",
			Kind: KindSelect, Network: true,
			Help:    "Sends a DHCP client identifier (option 61) when on. Changing it can make the printer get a different address. Default in the guide is off.",
			Options: []Option{{Value: "on", Label: "On"}, {Value: "off", Label: "Off"}},
		},
		{
			ID: id("cid_type"), Label: label + " client ID type", Section: "Network",
			Kind: KindSelect, Network: true,
			Help: "0 is text. 1 is the printer's MAC address. 2 is hexadecimal. Prefix and suffix are used for 0 and 2.",
			Options: []Option{
				{Value: "0", Label: "Text"},
				{Value: "1", Label: "MAC address"},
				{Value: "2", Label: "Hexadecimal"},
			},
		},
		{
			ID: id("cid_prefix"), Label: label + " client ID prefix", Section: "Network",
			Kind: KindText, MaxLen: 20, Network: true,
			Help: "Text added in front of the client ID. The guide allows 10 characters for text, or 20 for hex. Leave blank if you are not using a custom ID.",
		},
		{
			ID: id("cid_suffix"), Label: label + " client ID suffix", Section: "Network",
			Kind: KindText, MaxLen: 120, Network: true,
			Help: "Text added at the end of the client ID. The guide allows 60 characters for text, or 120 for hex.",
		},
	}
}

func hostnameSettings() []Setting {
	return []Setting{
		{
			ID: "internal_wired.ip.dhcp.option12", Label: "Send wired DHCP host name", Section: "Network",
			Kind: KindSelect, Network: true,
			Help:    "DHCP option 12. On sends a host name when the printer asks for an address. Default in the guide is on.",
			Options: []Option{{Value: "on", Label: "On"}, {Value: "off", Label: "Off"}},
		},
		{
			ID: "internal_wired.ip.dhcp.option12_format", Label: "Wired DHCP host name", Section: "Network",
			Kind: KindText, MaxLen: 80, Network: true,
			Help: "The host name to send, or <device.friendly_name> to use the printer's friendly name.",
		},
		{
			ID: "internal_wired.ip.dhcp.option12_value", Label: "Wired DHCP host name in use", Section: "Network",
			Kind: KindText, ReadOnly: true, MaxLen: 80,
			Help: "The host name the printer will actually send. The guide only allows reading this.",
		},
		{
			ID: "ip.dhcp.option12", Label: "Send Wi-Fi DHCP host name", Section: "Network",
			Kind: KindSelect, Network: true,
			Help:    "DHCP option 12 on the wireless print server. Leave this if the printer says Not reported.",
			Options: []Option{{Value: "on", Label: "On"}, {Value: "off", Label: "Off"}},
		},
		{
			ID: "ip.dhcp.option12_format", Label: "Wi-Fi DHCP host name", Section: "Network",
			Kind: KindText, MaxLen: 80, Network: true,
			Help: "Host name for the wireless print server. Leave this if the printer says Not reported.",
		},
	}
}

// PowerSummary is the plain sentence for the Power card.
func PowerSummary(enable, timeout string) string {
	if strings.EqualFold(strings.TrimSpace(enable), "off") {
		return "Always on"
	}
	sec, err := strconv.Atoi(strings.TrimSpace(timeout))
	if err != nil || sec <= 0 {
		return "Sleeps (time not reported)"
	}
	if sec%60 != 0 {
		return fmt.Sprintf("Sleeps after %d sec", sec)
	}
	min := sec / 60
	if min == 1 {
		return "Sleeps after 1 min"
	}
	return fmt.Sprintf("Sleeps after %d min", min)
}

// StaticIP reports settings that would give the new printer the old address.
func StaticIP(id string) bool {
	switch id {
	case "internal_wired.ip.addr", "internal_wired.ip.netmask", "internal_wired.ip.gateway", "internal_wired.ip.protocol":
		return true
	default:
		return false
	}
}
