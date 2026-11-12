// Package sim is an in-memory ZD410/ZD411-style printer on a TCP port.
// It answers SGD getvar/setvar/do and the host queries this tool uses.
// Responses follow the ZPL Programming Guide: quoted SGD values with no
// terminator, and STX/ETX host replies for ~HI and ~HS.
// Nothing here stores a label, a graphic, or a file list.
package sim

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Options selects the printer the simulator pretends to be.
type Options struct {
	// Model is the device.product_name string, such as "ZD411" or "ZD621".
	Model string
	// DPI is head.resolution.in_dpi. 203 and 200 are the 8 dots/mm class.
	DPI int
	// Delay is inserted before each reply. Use it to exercise timeouts.
	Delay time.Duration
	// Hang accepts the TCP connection and never answers.
	Hang bool
	// Firmware is the appl.name / ~HI version string.
	Firmware string
}

// Server is a started simulator.
type Server struct {
	ln     net.Listener
	opt    Options
	state  *state
	mu     sync.Mutex
	log    []string
	closed chan struct{}
}

type state struct {
	mu       sync.Mutex
	values   map[string]string
	saved    map[string]string
	factory  map[string]string
	reject   map[string]bool
	model    string
	dpi      int
	dpm      int
	firmware string
}

var sgdLine = regexp.MustCompile(`(?i)^!\s*U1\s+(getvar|setvar|do)\s+"([^"]+)"(?:\s+"([^"]*)")?\s*$`)

// Listen starts the simulator on addr (for example "127.0.0.1:9101" or "127.0.0.1:0").
func Listen(addr string, opt Options) (*Server, error) {
	if opt.Model == "" {
		opt.Model = "ZD411"
	}
	if opt.DPI == 0 {
		opt.DPI = 203
	}
	if opt.Firmware == "" {
		opt.Firmware = "V84.20.20Z"
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	s := &Server{
		ln:     ln,
		opt:    opt,
		state:  newState(opt),
		closed: make(chan struct{}),
	}
	go s.accept()
	return s, nil
}

// Addr is the host:port the simulator bound.
func (s *Server) Addr() string { return s.ln.Addr().String() }

// Port is the TCP port.
func (s *Server) Port() int {
	return s.ln.Addr().(*net.TCPAddr).Port
}

// Close stops the listener.
func (s *Server) Close() error {
	select {
	case <-s.closed:
	default:
		close(s.closed)
	}
	return s.ln.Close()
}

// Commands returns the command lines the simulator actually received.
func (s *Server) Commands() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, len(s.log))
	copy(out, s.log)
	return out
}

// Get returns the current getvar text for name.
func (s *Server) Get(name string) string {
	return s.state.get(name)
}

// SetSticky controls whether a later setvar changes name.
// Sticky false is how tests force "the printer did not keep this setting".
func (s *Server) SetSticky(name string, sticky bool) {
	s.state.mu.Lock()
	defer s.state.mu.Unlock()
	if s.state.reject == nil {
		s.state.reject = map[string]bool{}
	}
	s.state.reject[name] = !sticky
}

func (s *Server) accept() {
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			return
		}
		go s.handle(conn)
	}
}

func (s *Server) handle(conn net.Conn) {
	defer conn.Close()
	if s.opt.Hang {
		_, _ = io.Copy(io.Discard, conn)
		return
	}
	r := bufio.NewReader(conn)
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		s.mu.Lock()
		s.log = append(s.log, strings.TrimRight(line, "\r\n"))
		s.mu.Unlock()
		resp, closeConn := s.dispatch(line)
		if s.opt.Delay > 0 {
			time.Sleep(s.opt.Delay)
		}
		if len(resp) > 0 {
			_ = conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
			if _, err := conn.Write(resp); err != nil {
				return
			}
		}
		if closeConn {
			return
		}
	}
}

func (s *Server) dispatch(line string) ([]byte, bool) {
	trim := strings.TrimSpace(line)
	low := strings.ToLower(trim)
	// No file listing exists. A direct query still must not invent one.
	if strings.Contains(low, "file.") {
		return []byte("\"?\""), false
	}
	if m := sgdLine.FindStringSubmatch(trim); m != nil {
		op, name, value := strings.ToLower(m[1]), m[2], m[3]
		switch op {
		case "getvar":
			return []byte(`"` + s.state.get(name) + `"`), false
		case "setvar":
			closeConn := s.state.set(name, value)
			return nil, closeConn
		case "do":
			s.state.do(name, value)
			return nil, false
		}
	}
	compact := strings.ToUpper(strings.ReplaceAll(trim, " ", ""))
	switch {
	case strings.Contains(compact, "~HI"):
		return s.state.hostID(), false
	case strings.Contains(compact, "~HS"):
		return s.state.hostStatus(), false
	case strings.Contains(compact, "^HH") || strings.Contains(compact, "~HH"):
		return s.state.configText(), false
	case strings.Contains(compact, "~JC"):
		return nil, false
	case strings.Contains(compact, "~WC"):
		return nil, false
	case strings.Contains(compact, "^JU"):
		s.state.ju(compact)
		return nil, false
	default:
		return nil, false
	}
}

func newState(opt Options) *state {
	dpm := 8
	switch opt.DPI {
	case 300:
		dpm = 12
	case 600:
		dpm = 24
	case 152:
		dpm = 6
	}
	width := "448"
	if opt.DPI == 300 {
		width = "640"
	}
	factory := map[string]string{
		"device.friendly_name":       "ZD411",
		"device.languages":           "hybrid_xml_zpl",
		"ezpl.media_type":            "gap/notch",
		"media.printmode":            "tear off",
		"ezpl.print_method":          "direct thermal",
		"device.sensor_select":       "transmissive",
		"ezpl.power_up_action":       "no motion",
		"ezpl.head_close_action":     "feed",
		"print.tone":                 "10.0",
		"media.speed":                "6",
		"ezpl.print_width":           width,
		"zpl.label_length":           "1225",
		"ezpl.tear_off":              "0",
		"internal_wired.ip.protocol": "all",
		"internal_wired.ip.addr":     "0.0.0.0",
		"internal_wired.ip.netmask":  "255.255.255.0",
		"internal_wired.ip.gateway":  "0.0.0.0",
		"internal_wired.ip.port":     "9100",
	}
	// A printer that already has a name and a leased address is more like
	// the one a tech sits down in front of. Factory reset returns to the
	// map above. ^JU rules: current starts equal to saved.
	current := copyMap(factory)
	current["device.friendly_name"] = "Shipping desk"
	current["internal_wired.ip.protocol"] = "dhcp"
	current["internal_wired.ip.addr"] = "192.168.1.50"
	current["internal_wired.ip.gateway"] = "192.168.1.1"
	if opt.Model != "ZD411" && opt.Model != "ZD410" {
		current["device.friendly_name"] = opt.Model
	}
	return &state{
		values:   current,
		saved:    copyMap(current),
		factory:  factory,
		model:    opt.Model,
		dpi:      opt.DPI,
		dpm:      dpm,
		firmware: opt.Firmware,
	}
}

func (s *state) get(name string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch name {
	case "device.product_name":
		return s.model
	case "appl.name":
		return s.firmware
	case "head.resolution.in_dpi":
		return strconv.Itoa(s.dpi)
	}
	if v, ok := s.values[name]; ok {
		return v
	}
	return "?"
}

func (s *state) set(name, value string) (closeConn bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if name == "device.reset" {
		s.values = copyMap(s.saved)
		return true
	}
	if name == "device.restore_defaults" {
		s.restoreBranch(value)
		return false
	}
	if s.reject[name] {
		return false
	}
	switch name {
	case "media.printmode":
		if word, ok := canonicalPrintMode(value); ok {
			s.values[name] = word
		}
		return false
	case "device.languages":
		v := strings.ToLower(strings.TrimSpace(value))
		if v == "zpl" {
			v = "hybrid_xml_zpl"
		}
		switch v {
		case "epl", "epl_zpl", "hybrid_xml_zpl":
			s.values[name] = v
		}
		return false
	case "device.product_name", "appl.name", "head.resolution.in_dpi":
		return false
	default:
		if _, ok := s.values[name]; ok {
			s.values[name] = value
		}
		return false
	}
}

func (s *state) do(name, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if strings.EqualFold(name, "device.restore_defaults") {
		s.restoreBranch(value)
	}
}

func (s *state) restoreBranch(value string) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "internal_wired", "ip":
		s.restore(true)
	}
}

func (s *state) ju(compact string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch {
	case strings.Contains(compact, "^JUS"):
		s.saved = copyMap(s.values)
	case strings.Contains(compact, "^JUR"):
		s.values = copyMap(s.saved)
	case strings.Contains(compact, "^JUA"):
		s.values = copyMap(s.factory)
	case strings.Contains(compact, "^JUF"):
		s.restore(false)
	case strings.Contains(compact, "^JUN"):
		s.restore(true)
	}
}

func (s *state) restore(network bool) {
	for k, v := range s.factory {
		isNet := strings.HasPrefix(k, "internal_wired.") || strings.HasPrefix(k, "ip.") || strings.HasPrefix(k, "wlan.")
		if isNet == network {
			s.values[k] = v
		}
	}
}

func (s *state) hostID() []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	// ~HI: STX + model,version,dots-per-mm,memory + ETX.
	// Programming Guide ~HI: XXXXXX,V1.0.0,dpm,000KB,X
	body := fmt.Sprintf("%s,%s,%d,8192KB", s.model, s.firmware, s.dpm)
	return []byte("\x02" + body + "\x03")
}

func (s *state) hostStatus() []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	// ~HS returns three STX...ETX LF strings. See the ~HS page.
	// String 2 field r (print mode): 2 = tear-off. Field q: 1 = thermal transfer.
	length := s.values["zpl.label_length"]
	if length == "" {
		length = "1225"
	}
	mode := "2"
	switch s.values["media.printmode"] {
	case "rewind":
		mode = "0"
	case "peel off":
		mode = "1"
	case "tear off":
		mode = "2"
	case "cutter":
		mode = "3"
	case "delayed cut":
		mode = "5"
	}
	thermal := "0"
	if s.values["ezpl.print_method"] == "thermal trans" {
		thermal = "1"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "\x02030,0,0,%s,000,0,0,0,000,0,0,0\x03\n", pad4(length))
	fmt.Fprintf(&b, "\x02000,0,0,0,%s,%s,0,0,00000000,1,000\x03\n", thermal, mode)
	b.WriteString("\x020000,1\x03\n")
	return []byte(b.String())
}

func (s *state) configText() []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	// ^HH is the configuration label returned to the host: settings only.
	var b strings.Builder
	fmt.Fprintf(&b, "MODEL: %s\nFIRMWARE: %s\nDPI: %d\n", s.model, s.firmware, s.dpi)
	for _, k := range []string{
		"ezpl.media_type", "media.printmode", "print.tone", "media.speed",
		"ezpl.print_width", "internal_wired.ip.protocol", "internal_wired.ip.addr",
	} {
		fmt.Fprintf(&b, "%s: %s\n", k, s.values[k])
	}
	return []byte(b.String())
}

func pad4(s string) string {
	if len(s) >= 4 {
		return s
	}
	return strings.Repeat("0", 4-len(s)) + s
}

func copyMap(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func canonicalPrintMode(value string) (string, bool) {
	v := strings.ToLower(strings.TrimSpace(value))
	words := map[string]string{
		"t": "tear off", "tear off": "tear off",
		"p": "peel off", "peel off": "peel off",
		"r": "rewind", "rewind": "rewind",
		"a": "applicator", "applicator": "applicator",
		"c": "cutter", "cutter": "cutter",
		"d": "delayed cut", "delayed cut": "delayed cut",
		"f": "rfid", "rfid": "rfid",
		"l": "linerless peel", "linerless peel": "linerless peel",
		"u": "linerless rewind", "linerless rewind": "linerless rewind",
		"k": "kiosk", "kiosk": "kiosk",
		"v": "linerless tear", "linerless tear": "linerless tear",
		"s": "stream", "stream": "stream",
	}
	w, ok := words[v]
	return w, ok
}
