// Package catalog is the allowlist of printer settings the main screen may
// read or write. Names and allowed values come from Zebra's ZPL Programming
// Guide (SGD command pages) and the ZD411 user guide cross-reference.
// See docs/sgd-variables.md for the table and the guide URLs.
//
// A few names in the original support notes are not the names in the guide.
// This list uses the guide:
//
//	media.type            -> ezpl.media_type
//	sensor.select         -> device.sensor_select
//	media.tear_off        -> ezpl.tear_off
//	ip.protocol / ip.addr -> internal_wired.ip.protocol / internal_wired.ip.addr
//	                         (wired print server; ip.dhcp.enable is the legacy
//	                         branch and is not sent, so a Wi-Fi setting is not
//	                         changed by mistake)
package catalog

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

// Kind is how the screen edits a setting.
type Kind string

const (
	KindSelect Kind = "select"
	KindNumber Kind = "number"
	KindText   Kind = "text"
	KindIP     Kind = "ip"
)

// Option is one dropdown entry. Value is what getvar returns (or the canonical
// form of it). Label is the plain words shown to a technician.
type Option struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// Setting describes one allowlisted SGD variable.
type Setting struct {
	ID       string
	Label    string
	Section  string
	Help     string
	Kind     Kind
	Options  []Option
	Min      float64
	Max      float64
	Decimals int
	Network  bool
	Step     string
}

// Change is a proposed new value from the screen or an import file.
type Change struct {
	ID    string `json:"id"`
	Value string `json:"value"`
}

// Row is one line of a before/after review.
type Row struct {
	ID            string `json:"id"`
	Label         string `json:"label"`
	Before        string `json:"before"`
	After         string `json:"after"`
	BeforeDisplay string `json:"beforeDisplay"`
	AfterDisplay  string `json:"afterDisplay"`
	Network       bool   `json:"network"`
}

// FieldView is one editable control.
type FieldView struct {
	ID      string   `json:"id"`
	Label   string   `json:"label"`
	Kind    Kind     `json:"kind"`
	Value   string   `json:"value"`
	Help    string   `json:"help"`
	Options []Option `json:"options,omitempty"`
	Min     float64  `json:"min"`
	Max     float64  `json:"max"`
	Step    string   `json:"step,omitempty"`
	Network bool     `json:"network"`
}

// Fact is a read-only line in a section.
type Fact struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// SectionView is one card on the screen.
type SectionView struct {
	Name   string      `json:"name"`
	Facts  []Fact      `json:"facts,omitempty"`
	Fields []FieldView `json:"fields"`
}

// Settings is the allowlist, in screen order.
func Settings() []Setting {
	return []Setting{
		{
			ID: "device.friendly_name", Label: "Friendly name", Section: "Printer",
			Kind: KindText,
			Help: "A short name for this printer. It is saved in the recent list on this computer. 1 to 32 letters, numbers, spaces, or dashes.",
		},
		{
			ID: "device.languages", Label: "Command language", Section: "Printer",
			Kind: KindSelect,
			Help: "Leave this on ZPL. EPL only will stop ZPL labels from printing.",
			Options: []Option{
				{Value: "hybrid_xml_zpl", Label: "ZPL"},
				{Value: "epl_zpl", Label: "EPL and ZPL"},
				{Value: "epl", Label: "EPL only"},
			},
		},
		{
			ID: "ezpl.media_type", Label: "Media type", Section: "Media",
			Kind: KindSelect,
			Help: "Gap or notch is a normal die-cut label. Continuous is receipt paper. Black mark uses marks on the back of the media.",
			Options: []Option{
				{Value: "gap/notch", Label: "Gap or notch (most labels)"},
				{Value: "continuous", Label: "Continuous (receipts)"},
				{Value: "mark", Label: "Black mark"},
			},
		},
		{
			ID: "media.printmode", Label: "Print mode", Section: "Media",
			Kind: KindSelect,
			Help: "How the printer handles the label after it prints. Tear-off is the usual setting. Peel, cutter, and rewind need the matching hardware.",
			Options: []Option{
				{Value: "tear off", Label: "Tear-off"},
				{Value: "peel off", Label: "Peel-off"},
				{Value: "cutter", Label: "Cutter"},
				{Value: "delayed cut", Label: "Delayed cutter"},
				{Value: "rewind", Label: "Rewind"},
			},
		},
		{
			ID: "ezpl.print_method", Label: "Print method", Section: "Media",
			Kind: KindSelect,
			Help: "Direct thermal uses heat-sensitive labels and no ribbon. Thermal transfer uses a ribbon. A direct-thermal-only printer will reject thermal transfer.",
			Options: []Option{
				{Value: "direct thermal", Label: "Direct thermal (no ribbon)"},
				{Value: "thermal trans", Label: "Thermal transfer (ribbon)"},
			},
		},
		{
			ID: "device.sensor_select", Label: "Media sensor", Section: "Sensor",
			Kind: KindSelect,
			Help: "Transmissive looks through the liner at the gap. Reflective sees black marks on the back. The ZD411 report often shows AUTO; the programming guide only lists these two values.",
			Options: []Option{
				{Value: "transmissive", Label: "Transmissive (gap / see-through)"},
				{Value: "reflective", Label: "Reflective (black mark)"},
			},
		},
		{
			ID: "ezpl.power_up_action", Label: "When the printer turns on", Section: "Sensor",
			Kind:    KindSelect,
			Help:    "What the media does at power-on. No motion is the ZD411 report default. The programming guide's own default line says calibrate.",
			Options: motionOptions(),
		},
		{
			ID: "ezpl.head_close_action", Label: "When the printhead is closed", Section: "Sensor",
			Kind:    KindSelect,
			Help:    "What the media does after the head is closed. Feed is the ZD411 report default. The programming guide's own default line says calibrate.",
			Options: motionOptions(),
		},
		{
			ID: "print.tone", Label: "Darkness", Section: "Print",
			Kind: KindNumber, Min: 0, Max: 30, Decimals: 1, Step: "0.1",
			Help: "Higher is darker. 0 to 30. A ZD411 often ships at 10. The programming guide lists a default of 4.0, which may not match your printer.",
		},
		{
			ID: "media.speed", Label: "Print speed (inches per second)", Section: "Print",
			Kind: KindNumber, Min: 2, Max: 12, Decimals: 0, Step: "1",
			Help: "2 to 12. A 203 dpi ZD410 or ZD411 usually tops out at 6 (some ZD411 notes say 8). The printer rejects a speed it cannot do.",
		},
		{
			ID: "ezpl.print_width", Label: "Print width (dots)", Section: "Print",
			Kind: KindNumber, Min: 1, Max: 832, Decimals: 0, Step: "1",
			Help: "Dots across the label. A 203 dpi ZD410 or ZD411 is 448 dots wide (about 2.2 inches).",
		},
		{
			ID: "zpl.label_length", Label: "Label length (dots)", Section: "Print",
			Kind: KindNumber, Min: 1, Max: 32000, Decimals: 0, Step: "1",
			Help: "Dots down the label. About 203 dots per inch. The ZD411 report often starts at 1225. The guide allows 1 to 32000; the printer may allow less.",
		},
		{
			ID: "ezpl.tear_off", Label: "Tear-off position (dots)", Section: "Print",
			Kind: KindNumber, Min: -120, Max: 120, Decimals: 0, Step: "1",
			Help: "Shifts where the label stops at the tear bar. -120 to 120. The ZD411 report default is 0.",
		},
		{
			ID: "internal_wired.ip.protocol", Label: "Wired addressing", Section: "Network",
			Kind: KindSelect, Network: true,
			Help: "Permanent uses the address below. Any other choice gets an address automatically. A new permanent address usually applies only after a restart.",
			Options: []Option{
				{Value: "all", Label: "Automatic (try every method)"},
				{Value: "dhcp", Label: "DHCP"},
				{Value: "bootp", Label: "BOOTP"},
				{Value: "rarp", Label: "RARP"},
				{Value: "glean", Label: "Glean from a ping"},
				{Value: "permanent", Label: "Permanent (manual address)"},
			},
		},
		{
			ID: "internal_wired.ip.addr", Label: "Wired IP address", Section: "Network",
			Kind: KindIP, Network: true,
			Help: "Used when addressing is Permanent. While the printer is on DHCP this may show the leased address instead.",
		},
		{
			ID: "internal_wired.ip.netmask", Label: "Wired subnet mask", Section: "Network",
			Kind: KindIP, Network: true,
			Help: "Usually 255.255.255.0. Ignored unless addressing is Permanent.",
		},
		{
			ID: "internal_wired.ip.gateway", Label: "Wired gateway", Section: "Network",
			Kind: KindIP, Network: true,
			Help: "Usually your router. Use 0.0.0.0 if you are not sure. Ignored unless addressing is Permanent.",
		},
		{
			ID: "internal_wired.ip.port", Label: "Raw printing port", Section: "Network",
			Kind: KindNumber, Min: 1, Max: 65535, Decimals: 0, Step: "1", Network: true,
			Help: "Leave this at 9100 unless a network admin changed it. The guide excludes ports 21, 23, 80, and 515. If you change it, reconnect on the new port.",
		},
	}
}

func motionOptions() []Option {
	return []Option{
		{Value: "no motion", Label: "No motion"},
		{Value: "feed", Label: "Feed to the next label"},
		{Value: "calibrate", Label: "Calibrate (measure the media)"},
		{Value: "length", Label: "Measure label length"},
		{Value: "short cal", Label: "Short calibration"},
	}
}

var byID = func() map[string]Setting {
	m := make(map[string]Setting)
	for _, s := range Settings() {
		m[s.ID] = s
	}
	return m
}()

// ByID returns the setting and true if name is allowlisted.
func ByID(id string) (Setting, bool) {
	s, ok := byID[id]
	return s, ok
}

// IDs is the allowlist in a stable order, for getvar.
func IDs() []string {
	all := Settings()
	out := make([]string, len(all))
	for i, s := range all {
		out[i] = s.ID
	}
	return out
}

// IsNetwork reports whether changing this setting can drop the printer off the network.
func IsNetwork(id string) bool {
	s, ok := byID[id]
	return ok && s.Network
}

// KnownID reports whether id is on the allowlist.
func KnownID(id string) bool {
	_, ok := byID[id]
	return ok
}

// printModeWire maps the getvar words to the setvar codes on the media.printmode page.
// The guide's example sets "T" and the getvar result is "tear off".
var printModeWire = map[string]string{
	"tear off":         "T",
	"peel off":         "P",
	"rewind":           "R",
	"applicator":       "A",
	"cutter":           "C",
	"delayed cut":      "D",
	"rfid":             "F",
	"linerless peel":   "L",
	"linerless rewind": "U",
	"kiosk":            "K",
	"linerless tear":   "V",
	"stream":           "S",
}

var printModeWord = map[string]string{
	"T": "tear off",
	"P": "peel off",
	"R": "rewind",
	"A": "applicator",
	"C": "cutter",
	"D": "delayed cut",
	"F": "rfid",
	"L": "linerless peel",
	"U": "linerless rewind",
	"K": "kiosk",
	"V": "linerless tear",
	"S": "stream",
}

// Canonical folds a printer reply or a typed value into the form we compare.
// The second return is false when value is not a legal value for id.
func Canonical(id, value string) (string, bool) {
	s, ok := byID[id]
	if !ok {
		return "", false
	}
	value = strings.TrimSpace(value)
	switch id {
	case "media.printmode":
		return canonicalPrintMode(value)
	case "device.languages":
		v := strings.ToLower(value)
		if v == "zpl" {
			// Guide: setvar "zpl" reads back as "hybrid_xml_zpl".
			v = "hybrid_xml_zpl"
		}
		if !hasOption(s, v) {
			return "", false
		}
		return v, true
	}
	switch s.Kind {
	case KindSelect:
		v := strings.ToLower(value)
		if !hasOption(s, v) {
			return "", false
		}
		return v, true
	case KindNumber:
		return canonicalNumber(s, value)
	case KindIP:
		ip, err := ParseIPv4(value)
		if err != nil {
			return "", false
		}
		if id == "internal_wired.ip.netmask" && !contiguousMask(ip) {
			return "", false
		}
		return ip.String(), true
	case KindText:
		if id == "device.friendly_name" {
			if !validFriendly(value) {
				return "", false
			}
			return value, true
		}
		return "", false
	default:
		return "", false
	}
}

func canonicalPrintMode(value string) (string, bool) {
	v := strings.ToLower(strings.TrimSpace(value))
	if len(v) == 1 {
		word, ok := printModeWord[strings.ToUpper(v)]
		if !ok {
			return "", false
		}
		return word, true
	}
	if _, ok := printModeWire[v]; !ok {
		return "", false
	}
	return v, true
}

func canonicalNumber(s Setting, value string) (string, bool) {
	f, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil {
		return "", false
	}
	if f < s.Min || f > s.Max {
		return "", false
	}
	if s.Decimals == 0 && f != float64(int(f)) {
		return "", false
	}
	if s.ID == "internal_wired.ip.port" {
		p := int(f)
		switch p {
		case 21, 23, 80, 515:
			return "", false
		}
	}
	if s.ID == "ezpl.print_width" && f > 448 {
		// 448 is the 203 dpi ZD410/ZD411 head width from the ZD411 user guide.
		// Wider values are left for read-only display of other models; writes
		// of those widths are rejected here because this tool only writes to
		// 203 dpi ZD410/ZD411 printers.
		return "", false
	}
	return strconv.FormatFloat(f, 'f', s.Decimals, 64), true
}

func hasOption(s Setting, value string) bool {
	for _, o := range s.Options {
		if strings.EqualFold(o.Value, value) {
			return true
		}
	}
	// Print mode accepts the full guide list even when the dropdown is shorter.
	if s.ID == "media.printmode" {
		_, ok := printModeWire[strings.ToLower(value)]
		return ok
	}
	return false
}

func validFriendly(value string) bool {
	if len(value) < 1 || len(value) > 32 {
		return false
	}
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == ' ' || r == '-' || r == '_' || r == '.':
		default:
			return false
		}
	}
	return true
}

func contiguousMask(ip net.IP) bool {
	mask := net.IPMask(ip.To4())
	ones, bits := mask.Size()
	return bits == 32 && ones >= 0
}

// ParseIPv4 parses a dotted address and rejects leading zeros.
func ParseIPv4(s string) (net.IP, error) {
	parts := strings.Split(strings.TrimSpace(s), ".")
	if len(parts) != 4 {
		return nil, fmt.Errorf("enter an address like 192.168.1.50")
	}
	out := make(net.IP, 4)
	for i, p := range parts {
		if p == "" || (len(p) > 1 && p[0] == '0') {
			return nil, fmt.Errorf("enter the address without leading zeros, such as 192.168.1.50")
		}
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 || n > 255 {
			return nil, fmt.Errorf("enter an address like 192.168.1.50")
		}
		out[i] = byte(n)
	}
	return out, nil
}

// Encode converts a canonical value to the bytes the printer expects.
// media.printmode is the important case: the screen says "tear off" and
// setvar must send "T".
func Encode(id, canonical string) (string, error) {
	if _, ok := Canonical(id, canonical); !ok {
		return "", fmt.Errorf("%s: %q is not an allowed value", labelOf(id), canonical)
	}
	if id == "media.printmode" {
		word, ok := canonicalPrintMode(canonical)
		if !ok {
			return "", fmt.Errorf("print mode %q is not an allowed value", canonical)
		}
		return printModeWire[word], nil
	}
	c, ok := Canonical(id, canonical)
	if !ok {
		return "", fmt.Errorf("%s: %q is not an allowed value", labelOf(id), canonical)
	}
	return c, nil
}

// Same reports whether current (a printer reply) and next (a screen value)
// mean the same setting. Unknown current values compare as trimmed text so a
// surprise reply is left alone until someone picks a new value.
func Same(id, current, next string) bool {
	a, aok := Canonical(id, current)
	b, bok := Canonical(id, next)
	if aok && bok {
		return a == b
	}
	return strings.TrimSpace(current) == strings.TrimSpace(next)
}

// Display returns the plain label for a value, or the value itself.
func Display(id, value string) string {
	if c, ok := Canonical(id, value); ok {
		value = c
	}
	if s, ok := byID[id]; ok {
		for _, o := range s.Options {
			if strings.EqualFold(o.Value, value) {
				return o.Label
			}
		}
		if s.ID == "media.printmode" {
			if word, ok := canonicalPrintMode(value); ok {
				for _, o := range s.Options {
					if o.Value == word {
						return o.Label
					}
				}
				return word
			}
		}
	}
	return strings.TrimSpace(value)
}

// Diff compares proposed values with what the printer last reported.
// Unchanged rows are dropped. The first invalid change returns an error.
func Diff(current map[string]string, changes []Change) ([]Row, error) {
	seen := map[string]bool{}
	var rows []Row
	for _, ch := range changes {
		s, ok := byID[ch.ID]
		if !ok {
			return nil, fmt.Errorf("unknown setting %q", ch.ID)
		}
		if seen[ch.ID] {
			return nil, fmt.Errorf("setting %s was sent twice", s.Label)
		}
		seen[ch.ID] = true
		cur := current[ch.ID]
		if Same(ch.ID, cur, ch.Value) {
			continue
		}
		next, ok := Canonical(ch.ID, ch.Value)
		if !ok {
			return nil, fmt.Errorf("%s: %s", s.Label, explainInvalid(s, ch.Value))
		}
		rows = append(rows, Row{
			ID:            s.ID,
			Label:         s.Label,
			Before:        strings.TrimSpace(cur),
			After:         next,
			BeforeDisplay: Display(s.ID, cur),
			AfterDisplay:  Display(s.ID, next),
			Network:       s.Network,
		})
	}
	return rows, nil
}

// NeedsExtra is true when any row can drop the printer off the network.
func NeedsExtra(rows []Row) bool {
	for _, r := range rows {
		if r.Network {
			return true
		}
	}
	return false
}

func explainInvalid(s Setting, value string) string {
	switch s.Kind {
	case KindNumber:
		if s.Decimals == 1 {
			return fmt.Sprintf("enter a number from %g to %g", s.Min, s.Max)
		}
		if s.ID == "internal_wired.ip.port" {
			return "enter a port from 1 to 65535, other than 21, 23, 80, or 515"
		}
		if s.ID == "ezpl.print_width" {
			return "enter a width from 1 to 448 dots"
		}
		return fmt.Sprintf("enter a whole number from %g to %g", s.Min, s.Max)
	case KindIP:
		if s.ID == "internal_wired.ip.netmask" {
			return "enter a subnet mask such as 255.255.255.0"
		}
		return "enter an address such as 192.168.1.50, without leading zeros"
	case KindSelect:
		return "pick one of the listed choices"
	case KindText:
		return "use 1 to 32 letters, numbers, spaces, or dashes"
	default:
		return fmt.Sprintf("%q is not an allowed value", value)
	}
}

func labelOf(id string) string {
	if s, ok := byID[id]; ok {
		return s.Label
	}
	return id
}

// BuildView turns a value map into the cards the screen draws.
// facts are read-only lines for the Printer card (model, firmware, and so on).
func BuildView(values map[string]string, facts []Fact) []SectionView {
	order := []string{"Printer", "Media", "Print", "Sensor", "Network"}
	bySection := map[string][]FieldView{}
	for _, s := range Settings() {
		raw := values[s.ID]
		shown := raw
		if c, ok := Canonical(s.ID, raw); ok {
			shown = c
		}
		fv := FieldView{
			ID:      s.ID,
			Label:   s.Label,
			Kind:    s.Kind,
			Value:   shown,
			Help:    s.Help,
			Min:     s.Min,
			Max:     s.Max,
			Step:    s.Step,
			Network: s.Network,
		}
		if s.Kind == KindSelect {
			fv.Options = optionsFor(s, shown, raw)
		}
		bySection[s.Section] = append(bySection[s.Section], fv)
	}
	out := make([]SectionView, 0, len(order))
	for _, name := range order {
		sec := SectionView{Name: name, Fields: bySection[name]}
		if name == "Printer" {
			sec.Facts = facts
		}
		out = append(out, sec)
	}
	return out
}

func optionsFor(s Setting, shown, raw string) []Option {
	opts := append([]Option{}, s.Options...)
	if shown == "" && strings.TrimSpace(raw) == "" {
		return opts
	}
	target := shown
	if _, ok := Canonical(s.ID, raw); !ok {
		target = strings.TrimSpace(raw)
	}
	for _, o := range opts {
		if strings.EqualFold(o.Value, target) {
			return opts
		}
	}
	label := target
	if label == "?" || label == "" {
		label = "Not reported"
		target = strings.TrimSpace(raw)
		if target == "" {
			target = "?"
		}
	} else {
		label = target + " (reported by the printer)"
	}
	return append(opts, Option{Value: target, Label: label})
}
