// Package hipaa decides which console commands may be sent.
//
// The main screen never asks the printer for an arbitrary variable: it only
// getvars the allowlist in the catalog package. The console is a single
// command box, so it uses a denylist. Anything that would read or dump stored
// formats, graphics, the file directory, captured data, or label field data
// is rejected before it is written to the socket. Rejected text is not logged.
package hipaa

import (
	"strings"
	"unicode"
)

const maxCommandLen = 500

// Decision is the policy result for one console command.
type Decision struct {
	Allow  bool
	Write  bool
	Extra  bool // network change, factory reset, or restart
	Reason string
}

// Inspect returns whether command may be sent. The command is not retained.
func Inspect(command string) Decision {
	cmd := strings.TrimSpace(command)
	if cmd == "" {
		return Decision{Reason: "Type one command first."}
	}
	if len(cmd) > maxCommandLen {
		return Decision{Reason: "That is too long. This tool sends one short setting or status command, not a label."}
	}
	for _, r := range cmd {
		if r == 0 {
			return Decision{Reason: "That command contains a character this tool will not send."}
		}
	}
	if !oneCommand(cmd) {
		return Decision{Reason: "Send one command at a time."}
	}
	low := strings.ToLower(cmd)
	if reason, blocked := blocked(low); blocked {
		return Decision{Reason: reason}
	}
	if isQuery(cmd) {
		return Decision{Allow: true}
	}
	return Decision{
		Allow:  true,
		Write:  true,
		Extra:  needsExtra(low),
		Reason: "",
	}
}

func oneCommand(cmd string) bool {
	low := strings.ToLower(cmd)
	if strings.Count(low, "! u1") > 1 {
		return false
	}
	tildes := strings.Count(cmd, "~")
	carets := strings.Count(cmd, "^")
	bangs := strings.Count(low, "! u1")
	kinds := 0
	if tildes > 0 {
		kinds++
	}
	if carets > 0 {
		kinds++
	}
	if bangs > 0 {
		kinds++
	}
	if kinds > 1 {
		return false
	}
	if tildes > 1 {
		return false
	}
	return true
}

func blocked(low string) (string, bool) {
	const refuse = "This tool does not read, store, or send label formats, graphics, or the printer file list."
	needles := []string{
		"file.",
		"capture.channel",
		"allcv",
		"usb.mirror",
		"job_log",
		"logging.entries",
		"~dy", "~dg", "~db", "~hd", "~hf", "~hg", "~hy", "~wd", "~dt", "~hw",
		"^hf", "^hg", "^hy", "^wd", "^df", "^xf", "^xg", "^id", "^is", "^il",
		"^im", "^gf", "^dg", "^dy", "^fd", "^fn", "^sn", "^hw",
	}
	for _, n := range needles {
		if strings.Contains(low, n) {
			return refuse, true
		}
	}
	// getvar "all" dumps every config variable. Require the name to be all,
	// not a prefix of another name.
	if strings.Contains(low, `getvar "all"`) || strings.Contains(low, "getvar \"all\"") {
		return refuse, true
	}
	return "", false
}

func isQuery(cmd string) bool {
	low := strings.ToLower(strings.TrimSpace(cmd))
	if strings.HasPrefix(low, "! u1 getvar ") {
		return true
	}
	compact := compact(cmd)
	switch strings.ToUpper(compact) {
	case "~HI", "~HS", "^HH", "~HH", "^XA^HH^XZ", "^XA~HH^XZ":
		return true
	default:
		return false
	}
}

func needsExtra(low string) bool {
	if strings.Contains(low, "internal_wired.") || strings.Contains(low, "wlan.") {
		return true
	}
	for _, n := range []string{
		"ip.dhcp", "ip.addr", "ip.netmask", "ip.gateway", "ip.protocol", "ip.port",
		"device.reset", "device.restore_defaults",
		"^juf", "^jun", "^jua",
	} {
		if strings.Contains(low, n) {
			return true
		}
	}
	return false
}

func compact(s string) string {
	var b strings.Builder
	for _, r := range s {
		if !unicode.IsSpace(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}
