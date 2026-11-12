// Package printer reads and writes the allowlisted settings on one Zebra printer.
package printer

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/manovi-code/zebra-config/internal/catalog"
	"github.com/manovi-code/zebra-config/internal/sgd"
)

// ErrBadResponse means the printer answered, but not with a model.
var ErrBadResponse = errors.New("bad response")

// Version is shown on the page. Release builds replace it with the tag
// (for example 0.1.1) using -X.
var Version = "0.1.1"

// Identity names are read on every connect and are not editable.
var identityNames = []string{
	"device.product_name",
	"appl.name",
	"head.resolution.in_dpi",
}

// Identity is what the printer reported about itself, plus the support decision.
type Identity struct {
	ProductName string `json:"productName"`
	Firmware    string `json:"firmware"`
	DPI         int    `json:"dpi"`
	DPIRaw      string `json:"dpiRaw"`
	DotsPerMM   int    `json:"dotsPerMM"`
	HostID      string `json:"hostId"`
	Status      string `json:"status"`
	Supported   bool   `json:"supported"`
	Readonly    bool   `json:"readonly"`
	Message     string `json:"message"`
}

// Snapshot is the screen's picture of one connected printer.
type Snapshot struct {
	Identity    Identity              `json:"identity"`
	ConnectedTo string                `json:"connectedTo"`
	Sections    []catalog.SectionView `json:"sections"`
	Values      map[string]string     `json:"-"`
	Readonly    bool                  `json:"readonly"`
	Message     string                `json:"message"`
}

// Mismatch is a setting whose re-read did not match what we sent.
type Mismatch struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Wanted  string `json:"wanted"`
	Actual  string `json:"actual"`
	Display string `json:"display"`
}

// ApplyResult is the outcome of sending a reviewed diff.
type ApplyResult struct {
	Mismatches []Mismatch `json:"mismatches"`
	Saved      bool       `json:"saved"`
	Snapshot   Snapshot   `json:"snapshot"`
}

// ActionResult is the outcome of a one-click button.
type ActionResult struct {
	Message      string    `json:"message"`
	Disconnected bool      `json:"disconnected"`
	Snapshot     *Snapshot `json:"snapshot,omitempty"`
}

// DialTimeout and read timeouts used by the service.
type Timeouts struct {
	Dial   time.Duration
	Read   time.Duration
	Status time.Duration
}

// DefaultTimeouts are the interactive defaults.
func DefaultTimeouts() Timeouts {
	return Timeouts{Dial: 5 * time.Second, Read: 4 * time.Second, Status: 1200 * time.Millisecond}
}

// Open dials the printer and reads identity plus the allowlisted settings.
func Open(ip string, port int, t Timeouts) (*sgd.Client, Snapshot, error) {
	c, err := sgd.Dial(ip, port, t.Dial, t.Read)
	if err != nil {
		return nil, Snapshot{}, err
	}
	snap, err := Read(c, fmt.Sprintf("%s:%d", ip, port), t)
	if err != nil {
		_ = c.Close()
		return nil, Snapshot{}, err
	}
	return c, snap, nil
}

// Read queries the printer. A missing ~HI or a quiet ~HS does not fail the
// read when device.product_name already identified the printer. The ~HS page
// says the printer sends nothing when it is out of paper, out of ribbon, the
// head is open, or the rewinder is full.
func Read(c *sgd.Client, connectedTo string, t Timeouts) (Snapshot, error) {
	names := append(append([]string{}, identityNames...), catalog.IDs()...)
	vals, err := c.GetMany(names)
	if err != nil {
		return Snapshot{}, err
	}
	got := map[string]string{}
	for i, name := range names {
		got[name] = vals[i]
	}
	hiRaw, hiErr := c.Send("~HI\r\n", true)
	model, fw, dpm := "", "", 0
	hostPretty := ""
	if hiErr == nil {
		model, fw, dpm, hostPretty = ParseHI(hiRaw)
	}
	product := clean(got["device.product_name"])
	if product == "" {
		product = model
	}
	firmware := clean(got["appl.name"])
	if firmware == "" {
		firmware = fw
	}
	dpiRaw := clean(got["head.resolution.in_dpi"])
	if product == "" && model == "" {
		return Snapshot{}, fmt.Errorf("%w", ErrBadResponse)
	}
	id := Decide(product, dpiRaw, dpm, firmware, hostPretty)
	status := "No status reply. The printer stays quiet when it is out of paper, out of ribbon, or the head is open."
	prev := c.ReadTimeout()
	if t.Status > 0 {
		c.SetReadTimeout(t.Status)
	}
	hs, hsErr := c.Send("~HS\r\n", true)
	c.SetReadTimeout(prev)
	if hsErr == nil {
		if parsed := ParseHS(hs); parsed != "" {
			status = parsed
		}
	}
	id.Status = status
	values := map[string]string{}
	for _, name := range catalog.IDs() {
		values[name] = got[name]
	}
	snap := Snapshot{
		Identity:    id,
		ConnectedTo: connectedTo,
		Values:      values,
		Readonly:    id.Readonly,
		Message:     id.Message,
		Sections: catalog.BuildView(values, []catalog.Fact{
			{Label: "Model", Value: blank(id.ProductName)},
			{Label: "Firmware", Value: blank(id.Firmware)},
			{Label: "Resolution", Value: dpiText(id)},
			{Label: "Status", Value: status},
			{Label: "Identification", Value: blank(id.HostID)},
		}),
	}
	return snap, nil
}

func blank(s string) string {
	if strings.TrimSpace(s) == "" {
		return "Not reported"
	}
	return s
}

func dpiText(id Identity) string {
	if id.DPI == 0 && id.DPIRaw == "" && id.DotsPerMM == 0 {
		return "Not reported"
	}
	if id.DPI > 0 {
		extra := ""
		if id.DotsPerMM > 0 {
			extra = fmt.Sprintf(" (%d dots/mm)", id.DotsPerMM)
		}
		return fmt.Sprintf("%d dpi%s", id.DPI, extra)
	}
	if id.DPIRaw != "" {
		return id.DPIRaw + " (unrecognized)"
	}
	return fmt.Sprintf("%d dots/mm", id.DotsPerMM)
}

func clean(v string) string {
	v = strings.TrimSpace(v)
	if v == "?" || v == "" {
		return ""
	}
	return v
}

// Apply sends a reviewed diff, re-reads those settings, and optionally saves
// with ^JUS. ^JUS is the programming guide's "save current settings" command.
func Apply(c *sgd.Client, current map[string]string, changes []catalog.Change, save bool, t Timeouts, connectedTo string) (ApplyResult, error) {
	rows, err := catalog.Diff(current, changes)
	if err != nil {
		return ApplyResult{}, err
	}
	if len(rows) == 0 {
		return ApplyResult{}, fmt.Errorf("nothing is different")
	}
	for _, row := range rows {
		wire, err := catalog.Encode(row.ID, row.After)
		if err != nil {
			return ApplyResult{}, err
		}
		if err := c.Set(row.ID, wire); err != nil {
			return ApplyResult{}, fmt.Errorf("sending %s: %w", row.Label, err)
		}
	}
	ids := make([]string, len(rows))
	for i, row := range rows {
		ids[i] = row.ID
	}
	got, err := c.GetMany(ids)
	var mismatches []Mismatch
	if err != nil {
		for _, row := range rows {
			mismatches = append(mismatches, Mismatch{
				ID: row.ID, Label: row.Label, Wanted: row.AfterDisplay,
				Actual: "could not re-read", Display: "could not re-read",
			})
		}
	} else {
		for i, row := range rows {
			if !catalog.Same(row.ID, got[i], row.After) {
				mismatches = append(mismatches, Mismatch{
					ID:      row.ID,
					Label:   row.Label,
					Wanted:  row.AfterDisplay,
					Actual:  strings.TrimSpace(got[i]),
					Display: catalog.Display(row.ID, got[i]),
				})
			}
		}
	}
	saved := false
	if save && len(mismatches) == 0 {
		if _, err := c.Send("^XA^JUS^XZ\r\n", false); err != nil {
			return ApplyResult{}, fmt.Errorf("settings were sent, but saving them failed: %w", err)
		}
		saved = true
	}
	snap, rerr := Read(c, connectedTo, t)
	if rerr != nil {
		return ApplyResult{Mismatches: mismatches, Saved: saved}, fmt.Errorf("settings were sent, but re-reading the printer failed: %w", rerr)
	}
	return ApplyResult{Mismatches: mismatches, Saved: saved, Snapshot: snap}, nil
}

// Action is one of the fixed buttons. The command strings are not taken from
// the user.
type Action string

const (
	ActionCalibrate       Action = "calibrate"
	ActionConfigLabel     Action = "config_label"
	ActionRecall          Action = "recall"
	ActionFactory         Action = "factory"
	ActionNetworkDefaults Action = "network_defaults"
	ActionRestart         Action = "restart"
)

// ActionSpec describes a button: the bytes to send, whether the extra
// warning is required, and the words shown in the confirm dialog.
type ActionSpec struct {
	Command     string
	ExpectReply bool
	Extra       bool
	Title       string
	Warning     string
	Result      string
	Reread      bool
	Disconnect  bool
	OfferSave   bool
}

// LookupAction returns the fixed command for a button.
func LookupAction(name string) (ActionSpec, error) {
	switch Action(name) {
	case ActionCalibrate:
		return ActionSpec{
			Command: "~JC\r\n",
			Title:   "Calibrate media",
			Warning: "The printer will feed a few labels and measure the sensor. Send calibration?",
			Result:  "Calibration was sent. The printer should feed a few labels. This screen cannot see when that finishes.",
			Reread:  true,
		}, nil
	case ActionConfigLabel:
		return ActionSpec{
			Command: "~WC\r\n",
			Title:   "Print configuration label",
			Warning: "The printer will print its configuration label. That label lists settings, not your label designs. Send it?",
			Result:  "The configuration label was sent. It prints only when the printer is idle.",
		}, nil
	case ActionRecall:
		return ActionSpec{
			Command: "^XA^JUR^XZ\r\n",
			Title:   "Reset to last saved settings",
			Warning: "This puts back the last settings saved on the printer. If the saved network settings are different, the printer may drop off the network. Send it?",
			Result:  "The printer was told to restore its last saved settings.",
			Reread:  true,
		}, nil
	case ActionFactory:
		return ActionSpec{
			Command:   "^XA^JUF^XZ\r\n",
			Extra:     true,
			OfferSave: true,
			Title:     "Reset printer settings to factory",
			Warning:   "This reloads factory printer settings (not the wired network). The programming guide says a factory reload is lost at power-off unless it is saved. This cannot be undone from here except by setting the values again.",
			Result:    "Factory printer settings were reloaded.",
			Reread:    true,
		}, nil
	case ActionNetworkDefaults:
		return ActionSpec{
			Command:   "^XA^JUN^XZ\r\n",
			Extra:     true,
			OfferSave: true,
			Title:     "Reset network settings to factory",
			Warning:   "This reloads factory network settings. The printer may drop off the network, and you may have to reconnect at a different address. The programming guide says this reload is lost at power-off unless it is saved.",
			Result:    "Factory network settings were reloaded. The printer may no longer be at this address.",
			Reread:    true,
		}, nil
	case ActionRestart:
		return ActionSpec{
			Command:    "! U1 setvar \"device.reset\" \"\"\r\n",
			Extra:      true,
			Title:      "Restart printer",
			Warning:    "The printer will reboot and drop off the network for a minute. Unsaved setting changes may be discarded. Restart it?",
			Result:     "Restart was sent. Wait about a minute, then connect again. If the address changed, use the new one.",
			Disconnect: true,
		}, nil
	default:
		return ActionSpec{}, fmt.Errorf("unknown action")
	}
}

// RunAction sends a fixed button command.
func RunAction(c *sgd.Client, spec ActionSpec, save bool, t Timeouts, connectedTo string) (ActionResult, error) {
	_, err := c.Send(spec.Command, spec.ExpectReply)
	if err != nil && spec.Disconnect {
		// device.reset often drops the socket as the printer reboots.
		return ActionResult{Message: spec.Result, Disconnected: true}, nil
	}
	if err != nil {
		return ActionResult{}, err
	}
	if spec.Disconnect {
		return ActionResult{Message: spec.Result, Disconnected: true}, nil
	}
	if save && spec.OfferSave {
		if _, err := c.Send("^XA^JUS^XZ\r\n", false); err != nil {
			return ActionResult{}, fmt.Errorf("the reset was sent, but saving it failed: %w", err)
		}
	}
	res := ActionResult{Message: spec.Result}
	if spec.Reread {
		snap, rerr := Read(c, connectedTo, t)
		if rerr != nil {
			res.Message = spec.Result + " Re-reading the printer afterwards failed, so the screen may be stale. Refresh in a moment."
			return res, nil
		}
		res.Snapshot = &snap
	}
	return res, nil
}

// ParseHI splits a ~HI reply. The guide's format is
// model,version,dots-per-mm,memory[,options], often wrapped in STX and ETX.
func ParseHI(raw string) (model, version string, dpm int, pretty string) {
	s := strings.Trim(raw, "\x02\x03\r\n ")
	pretty = s
	parts := strings.Split(s, ",")
	if len(parts) >= 1 {
		model = strings.TrimSpace(parts[0])
	}
	if len(parts) >= 2 {
		version = strings.TrimSpace(parts[1])
	}
	if len(parts) >= 3 {
		dpm, _ = strconv.Atoi(strings.TrimSpace(parts[2]))
	}
	return model, version, dpm, pretty
}

// ParseHS turns the three ~HS strings into a short status line.
// Flags follow the ~HS page: string 1 paper-out and pause, string 2 head-up
// and ribbon-out.
func ParseHS(raw string) string {
	parts := strings.Split(raw, "\x03")
	if len(parts) < 2 {
		return ""
	}
	var notes []string
	s1 := strings.Trim(parts[0], "\x02\r\n ")
	f1 := strings.Split(s1, ",")
	if len(f1) > 2 {
		if f1[1] == "1" {
			notes = append(notes, "Out of paper")
		}
		if f1[2] == "1" {
			notes = append(notes, "Paused")
		}
	}
	if len(parts) > 1 {
		s2 := strings.Trim(parts[1], "\x02\r\n ")
		f2 := strings.Split(s2, ",")
		if len(f2) > 3 {
			if f2[2] == "1" {
				notes = append(notes, "Printhead is open")
			}
			if f2[3] == "1" {
				notes = append(notes, "Out of ribbon")
			}
		}
	}
	if len(notes) == 0 {
		return "Ready"
	}
	return strings.Join(notes, ", ")
}

// Decide says whether this tool may change settings.
// Supported means a ZD410 or ZD411 at 203 dpi (8 dots/mm). The programming
// guide lists head.resolution.in_dpi as 200, 300, or 600, while the ZD411
// user guide calls the same head 203 dpi. Both 200 and 203 are accepted.
func Decide(product, dpiRaw string, dpm int, firmware, hostID string) Identity {
	id := Identity{
		ProductName: strings.TrimSpace(product),
		Firmware:    strings.TrimSpace(firmware),
		DPIRaw:      strings.TrimSpace(dpiRaw),
		DotsPerMM:   dpm,
		HostID:      hostID,
	}
	family := familyOf(id.ProductName)
	dpi, dpiKnown := dpiOf(id.DPIRaw, dpm)
	id.DPI = dpi
	switch {
	case family == "":
		id.Readonly = true
		id.Message = fmt.Sprintf("This printer reports as %s. Zebra Config only changes settings on a ZD410 or ZD411 at 203 dpi. You can look, but changes are turned off.", orUnknown(id.ProductName))
	case !dpiKnown:
		id.Readonly = true
		id.Message = fmt.Sprintf("This looks like a %s, but the resolution could not be read. Changes are turned off until the printer reports 203 dpi.", family)
	case dpi != 203:
		id.Readonly = true
		id.Message = fmt.Sprintf("This is a %s at %d dpi. Zebra Config only changes settings on the 203 dpi models. You can look, but changes are turned off.", family, dpi)
	default:
		id.Supported = true
	}
	return id
}

func familyOf(product string) string {
	u := strings.ToUpper(product)
	switch {
	case strings.Contains(u, "ZD411"):
		return "ZD411"
	case strings.Contains(u, "ZD410"):
		return "ZD410"
	default:
		return ""
	}
}

func dpiOf(raw string, dpm int) (dpi int, known bool) {
	if n, err := strconv.Atoi(strings.TrimSpace(raw)); err == nil && n > 0 {
		switch n {
		case 200, 203:
			return 203, true
		default:
			return n, true
		}
	}
	switch dpm {
	case 8:
		return 203, true
	case 12:
		return 300, true
	case 6:
		return 152, true
	case 24:
		return 600, true
	default:
		return 0, false
	}
}

func orUnknown(s string) string {
	if strings.TrimSpace(s) == "" {
		return "an unknown model"
	}
	return s
}
