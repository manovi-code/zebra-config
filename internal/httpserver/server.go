// Package httpserver is the loopback UI and the only place that turns button
// presses into printer commands.
package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/manovi-code/zebra-config/internal/catalog"
	"github.com/manovi-code/zebra-config/internal/hipaa"
	"github.com/manovi-code/zebra-config/internal/printer"
	"github.com/manovi-code/zebra-config/internal/recent"
	"github.com/manovi-code/zebra-config/internal/sgd"
	"github.com/manovi-code/zebra-config/internal/web"
)

// Server is one local UI process and at most one printer session.
type Server struct {
	timeouts printer.Timeouts
	recent   *recent.Store
	mu       sync.Mutex
	sess     *session
	http     *http.Server
	ln       net.Listener
}

type session struct {
	client *sgd.Client
	ip     string
	port   int
	snap   printer.Snapshot
}

// New builds a server. recentPath may be empty to use the user config dir.
func New(recentPath string, t printer.Timeouts) (*Server, error) {
	if t.Dial == 0 && t.Read == 0 {
		t = printer.DefaultTimeouts()
	}
	st, err := recent.Open(recentPath)
	if err != nil {
		return nil, err
	}
	s := &Server{timeouts: t, recent: st}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.serveFile("index.html", "text/html; charset=utf-8"))
	mux.HandleFunc("GET /app.css", s.serveFile("app.css", "text/css; charset=utf-8"))
	mux.HandleFunc("GET /app.js", s.serveFile("app.js", "text/javascript; charset=utf-8"))
	mux.HandleFunc("GET /favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /api/bootstrap", s.handleBootstrap)
	mux.HandleFunc("POST /api/connect", s.handleConnect)
	mux.HandleFunc("POST /api/disconnect", s.handleDisconnect)
	mux.HandleFunc("POST /api/quit", s.handleQuit)
	mux.HandleFunc("POST /api/preview", s.handlePreview)
	mux.HandleFunc("POST /api/apply", s.handleApply)
	mux.HandleFunc("POST /api/action", s.handleAction)
	mux.HandleFunc("POST /api/console", s.handleConsole)
	mux.HandleFunc("GET /api/export", s.handleExport)
	mux.HandleFunc("POST /api/import", s.handleImport)
	mux.HandleFunc("DELETE /api/recent", s.handleForget)
	s.http = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	return s, nil
}

// Listen binds a loopback address. Other addresses are refused.
func (s *Server) Listen(addr string) (string, error) {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return "", err
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return "", errors.New("the UI only listens on 127.0.0.1")
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return "", err
	}
	s.ln = ln
	return ln.Addr().String(), nil
}

// Serve accepts HTTP connections until Close.
func (s *Server) Serve() error {
	err := s.http.Serve(s.ln)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// Close stops the UI and the printer session immediately.
func (s *Server) Close() error {
	s.closePrinter()
	if s.http != nil {
		_ = s.http.Close()
	}
	return nil
}

// Shutdown closes the printer connection, then stops the page once the
// current request has been answered. Quit and a process signal use this.
func (s *Server) Shutdown(ctx context.Context) error {
	s.closePrinter()
	if s.http == nil {
		return nil
	}
	return s.http.Shutdown(ctx)
}

func (s *Server) closePrinter() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sess != nil && s.sess.client != nil {
		_ = s.sess.client.Close()
		s.sess = nil
	}
}

func (s *Server) handleQuit(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = s.Shutdown(ctx)
	}()
}

func (s *Server) serveFile(name, contentType string) http.HandlerFunc {
	body, err := web.Files.ReadFile(name)
	return func(w http.ResponseWriter, r *http.Request) {
		setHeaders(w)
		if err != nil {
			http.Error(w, "missing page", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", contentType)
		_, _ = w.Write(body)
	}
}

func (s *Server) handleBootstrap(w http.ResponseWriter, r *http.Request) {
	list, recentErr := s.recent.List()
	if list == nil {
		list = []recent.Printer{}
	}
	s.mu.Lock()
	var snap any
	if s.sess != nil {
		snap = s.sess.snap
	}
	s.mu.Unlock()
	out := map[string]any{
		"ok":      true,
		"version": printer.Version,
		"recent":  list,
		"session": snap,
	}
	if recentErr != nil {
		out["recentError"] = "The recent printers list could not be read."
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleConnect(w http.ResponseWriter, r *http.Request) {
	var body struct {
		IP   string `json:"ip"`
		Port int    `json:"port"`
	}
	if err := decode(w, r, &body); err != nil {
		return
	}
	if body.Port == 0 {
		body.Port = 9100
	}
	if err := validateTarget(body.IP, body.Port); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_target", err.Error())
		return
	}
	c, snap, err := printer.Open(body.IP, body.Port, s.timeouts)
	if err != nil {
		writePrinterErr(w, body.IP, body.Port, err)
		return
	}
	s.mu.Lock()
	if s.sess != nil && s.sess.client != nil {
		_ = s.sess.client.Close()
	}
	s.sess = &session{client: c, ip: body.IP, port: body.Port, snap: snap}
	s.mu.Unlock()
	name := strings.TrimSpace(snap.Values["device.friendly_name"])
	if name == "" || name == "?" {
		name = snap.Identity.ProductName
	}
	_ = s.recent.Add(recent.Printer{
		IP:       body.IP,
		Name:     name,
		Model:    snap.Identity.ProductName,
		LastUsed: time.Now().UTC(),
	})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "snapshot": snap})
}

func (s *Server) handleDisconnect(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	if s.sess != nil && s.sess.client != nil {
		_ = s.sess.client.Close()
	}
	s.sess = nil
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handlePreview(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Changes []catalog.Change `json:"changes"`
	}
	if err := decode(w, r, &body); err != nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sess == nil {
		writeErr(w, http.StatusConflict, "not_connected", "Connect to a printer first.")
		return
	}
	rows, err := catalog.Diff(s.sess.snap.Values, body.Changes)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "bad_value", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, reviewPayload(rows, nil))
}

func (s *Server) handleApply(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Changes      []catalog.Change `json:"changes"`
		Confirm      bool             `json:"confirm"`
		ExtraConfirm bool             `json:"extraConfirm"`
		Save         bool             `json:"save"`
	}
	if err := decode(w, r, &body); err != nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sess == nil {
		writeErr(w, http.StatusConflict, "not_connected", "Connect to a printer first.")
		return
	}
	if s.sess.snap.Readonly {
		writeErr(w, http.StatusConflict, "readonly", "This printer is read-only. Changes are turned off.")
		return
	}
	rows, err := catalog.Diff(s.sess.snap.Values, body.Changes)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "bad_value", err.Error())
		return
	}
	if len(rows) == 0 {
		writeErr(w, http.StatusBadRequest, "no_change", "Nothing is different.")
		return
	}
	if !body.Confirm {
		writeErr(w, http.StatusBadRequest, "needs_confirm", "Review the changes and confirm before they are sent.")
		return
	}
	if catalog.NeedsExtra(rows) && !body.ExtraConfirm {
		writeErr(w, http.StatusBadRequest, "needs_extra", "Network changes need the extra confirmation. The printer may drop off the network.")
		return
	}
	result, err := printer.Apply(s.sess.client, s.sess.snap.Values, body.Changes, body.Save, s.timeouts, s.sess.snap.ConnectedTo)
	if result.Snapshot.ConnectedTo != "" {
		s.sess.snap = result.Snapshot
		s.rememberLocked()
	}
	if err != nil {
		writePrinterErr(w, s.sess.ip, s.sess.port, err)
		return
	}
	msg := "Settings were sent and the printer reports the new values."
	if result.Saved {
		msg = "Settings were sent, checked, and saved on the printer."
	}
	if len(result.Mismatches) > 0 {
		msg = "Some settings did not stick. The table shows what the printer reports now."
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":         true,
		"message":    msg,
		"mismatches": result.Mismatches,
		"saved":      result.Saved,
		"snapshot":   result.Snapshot,
	})
}

func (s *Server) handleAction(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Action       string `json:"action"`
		Confirm      bool   `json:"confirm"`
		ExtraConfirm bool   `json:"extraConfirm"`
		Save         bool   `json:"save"`
	}
	if err := decode(w, r, &body); err != nil {
		return
	}
	spec, err := printer.LookupAction(body.Action)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "bad_action", "That button is not a known action.")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sess == nil {
		writeErr(w, http.StatusConflict, "not_connected", "Connect to a printer first.")
		return
	}
	if s.sess.snap.Readonly {
		writeErr(w, http.StatusConflict, "readonly", "This printer is read-only. Actions that change it are turned off.")
		return
	}
	if !body.Confirm {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok": false, "code": "needs_confirm", "needsExtra": spec.Extra,
			"error": spec.Warning, "title": spec.Title, "offerSave": spec.OfferSave,
			"extraLabel": extraLabel(spec),
		})
		return
	}
	if spec.Extra && !body.ExtraConfirm {
		writeErr(w, http.StatusBadRequest, "needs_extra", "This action needs the extra confirmation.")
		return
	}
	res, err := printer.RunAction(s.sess.client, spec, body.Save, s.timeouts, s.sess.snap.ConnectedTo)
	if res.Disconnected {
		_ = s.sess.client.Close()
		s.sess = nil
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": res.Message, "disconnected": true})
		return
	}
	if err != nil {
		writePrinterErr(w, s.sess.ip, s.sess.port, err)
		return
	}
	out := map[string]any{"ok": true, "message": res.Message, "disconnected": false}
	if res.Snapshot != nil {
		s.sess.snap = *res.Snapshot
		s.rememberLocked()
		out["snapshot"] = res.Snapshot
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleConsole(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Command      string `json:"command"`
		Confirm      bool   `json:"confirm"`
		ExtraConfirm bool   `json:"extraConfirm"`
	}
	if err := decode(w, r, &body); err != nil {
		return
	}
	decision := hipaa.Inspect(body.Command)
	if !decision.Allow {
		writeErr(w, http.StatusBadRequest, "blocked", decision.Reason)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sess == nil {
		writeErr(w, http.StatusConflict, "not_connected", "Connect to a printer first.")
		return
	}
	if decision.Write && s.sess.snap.Readonly {
		writeErr(w, http.StatusConflict, "readonly", "This printer is read-only. Commands that change it are turned off.")
		return
	}
	if decision.Write && !body.Confirm {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok": false, "code": "needs_confirm", "needsExtra": decision.Extra,
			"error":      "This command changes the printer. Confirm to send it.",
			"extraLabel": "I understand the printer may drop off the network.",
		})
		return
	}
	if decision.Extra && !body.ExtraConfirm {
		writeErr(w, http.StatusBadRequest, "needs_extra", "This command can drop the printer off the network. Check the warning, then send it again.")
		return
	}
	line := strings.TrimSpace(body.Command)
	reply, err := s.sess.client.Send(line, !decision.Write)
	if err != nil {
		writePrinterErr(w, s.sess.ip, s.sess.port, err)
		return
	}
	text := sgd.Visible(reply)
	if strings.TrimSpace(text) == "" {
		text = "The printer returned no data. That is normal for a setting change, a calibration, or a configuration label."
	}
	if len(text) > 8000 {
		text = text[:8000] + "\n… response cut off. Configuration text only is kept on screen, and it is not saved."
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "response": text})
}

func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sess == nil {
		writeErr(w, http.StatusConflict, "not_connected", "Connect to a printer first.")
		return
	}
	doc := exportDoc{
		ZebraConfig: 1,
		ExportedAt:  time.Now().UTC().Format(time.RFC3339),
		Tool:        "zebra-config " + printer.Version,
		Printer: exportPrinter{
			ProductName: s.sess.snap.Identity.ProductName,
			Firmware:    s.sess.snap.Identity.Firmware,
			DPI:         s.sess.snap.Identity.DPI,
		},
		Settings: cloneValues(s.sess.snap.Values),
	}
	body, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "export", "Could not build the export file.")
		return
	}
	setHeaders(w)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="zebra-config.json"`)
	_, _ = w.Write(append(body, '\n'))
}

func (s *Server) handleImport(w http.ResponseWriter, r *http.Request) {
	var body struct {
		IncludeNetwork bool            `json:"includeNetwork"`
		Document       json.RawMessage `json:"document"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	if err := decode(w, r, &body); err != nil {
		return
	}
	doc, err := parseExport(body.Document)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "bad_file", err.Error())
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sess == nil {
		writeErr(w, http.StatusConflict, "not_connected", "Connect to a printer first.")
		return
	}
	if s.sess.snap.Readonly {
		writeErr(w, http.StatusConflict, "readonly", "This printer is read-only. Import is turned off.")
		return
	}
	var changes []catalog.Change
	var notes []string
	skippedNetwork := false
	for id, value := range doc.Settings {
		if dangerousKey(id) {
			writeErr(w, http.StatusBadRequest, "bad_file", "This file is not a settings export. It names printer files or label data, which this tool will not send.")
			return
		}
		if !catalog.KnownID(id) {
			notes = append(notes, fmt.Sprintf("Skipped unknown setting %q.", id))
			continue
		}
		if catalog.IsNetwork(id) && !body.IncludeNetwork {
			skippedNetwork = true
			continue
		}
		changes = append(changes, catalog.Change{ID: id, Value: value})
	}
	if skippedNetwork {
		notes = append(notes, "Network settings in the file were left out. Check the box to include them.")
	}
	rows, err := catalog.Diff(s.sess.snap.Values, changes)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "bad_value", err.Error())
		return
	}
	payload := reviewPayload(rows, notes)
	payload["changes"] = changesFromRows(rows)
	writeJSON(w, http.StatusOK, payload)
}

func (s *Server) handleForget(w http.ResponseWriter, r *http.Request) {
	ip := r.URL.Query().Get("ip")
	if err := validateTarget(ip, 9100); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_target", "That recent printer does not have a usable IP address.")
		return
	}
	if err := s.recent.Remove(ip); err != nil {
		writeErr(w, http.StatusInternalServerError, "recent", "Could not update the recent printers list.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) rememberLocked() {
	name := strings.TrimSpace(s.sess.snap.Values["device.friendly_name"])
	if name == "" || name == "?" {
		name = s.sess.snap.Identity.ProductName
	}
	_ = s.recent.Add(recent.Printer{
		IP:       s.sess.ip,
		Name:     name,
		Model:    s.sess.snap.Identity.ProductName,
		LastUsed: time.Now().UTC(),
	})
}

func reviewPayload(rows []catalog.Row, notes []string) map[string]any {
	warning := ""
	extra := "I understand the printer may drop off the network."
	if rows == nil {
		rows = []catalog.Row{}
	}
	if catalog.NeedsExtra(rows) {
		warning = "These changes can make the printer drop off the network. You may have to reconnect at a new address. A new wired address usually applies only after a restart, and only when addressing is Permanent."
	}
	if notes == nil {
		notes = []string{}
	}
	return map[string]any{
		"ok":         true,
		"rows":       rows,
		"needsExtra": catalog.NeedsExtra(rows),
		"warning":    warning,
		"extraLabel": extra,
		"offerSave":  true,
		"notes":      notes,
	}
}

func changesFromRows(rows []catalog.Row) []catalog.Change {
	out := make([]catalog.Change, len(rows))
	for i, row := range rows {
		out[i] = catalog.Change{ID: row.ID, Value: row.After}
	}
	return out
}

func extraLabel(spec printer.ActionSpec) string {
	if spec.Command == "^XA^JUF^XZ\r\n" {
		return "I understand this reloads factory printer settings."
	}
	return "I understand the printer may drop off the network."
}

type exportPrinter struct {
	ProductName string `json:"productName"`
	Firmware    string `json:"firmware"`
	DPI         int    `json:"dpi"`
}

type exportDoc struct {
	ZebraConfig int               `json:"zebraConfig"`
	ExportedAt  string            `json:"exportedAt"`
	Tool        string            `json:"tool"`
	Printer     exportPrinter     `json:"printer"`
	Settings    map[string]string `json:"settings"`
}

func parseExport(raw json.RawMessage) (exportDoc, error) {
	if len(raw) == 0 {
		return exportDoc{}, errors.New("The file is empty.")
	}
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	var doc exportDoc
	if err := dec.Decode(&doc); err != nil {
		return exportDoc{}, errors.New("That file is not a Zebra Config export.")
	}
	if dec.More() {
		return exportDoc{}, errors.New("That file is not a Zebra Config export.")
	}
	if doc.ZebraConfig != 1 {
		return exportDoc{}, errors.New("That file is from a different version of Zebra Config.")
	}
	if doc.Settings == nil {
		return exportDoc{}, errors.New("That file has no settings.")
	}
	return doc, nil
}

func dangerousKey(id string) bool {
	low := strings.ToLower(id)
	return strings.Contains(low, "file.") || strings.Contains(low, "capture.") || strings.Contains(low, "format")
}

func cloneValues(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func validateTarget(ip string, port int) error {
	if port < 1 || port > 65535 {
		return errors.New("Port must be a number from 1 to 65535. Printers use 9100 unless someone changed it.")
	}
	parsed, err := catalog.ParseIPv4(ip)
	if err != nil {
		return errors.New("That doesn't look like an IPv4 address. Example: 192.168.1.50")
	}
	if parsed.IsUnspecified() || parsed.IsMulticast() || parsed.Equal(net.IPv4bcast) {
		return errors.New("That address cannot be a printer. Example: 192.168.1.50")
	}
	return nil
}

func writePrinterErr(w http.ResponseWriter, ip string, port int, err error) {
	switch {
	case errors.Is(err, sgd.ErrRefused):
		writeErr(w, http.StatusBadGateway, "refused",
			fmt.Sprintf("Can't reach the printer at %s port %d. The connection was refused. Check the IP address, that the printer is on, and that port %d is the raw printer port (usually 9100).", ip, port, port))
	case errors.Is(err, sgd.ErrDialTimeout):
		writeErr(w, http.StatusGatewayTimeout, "timeout",
			fmt.Sprintf("Timed out trying to reach %s port %d. The printer did not answer. Check the IP address and that this computer is on the same network.", ip, port))
	case errors.Is(err, sgd.ErrReadTimeout):
		writeErr(w, http.StatusGatewayTimeout, "timeout",
			fmt.Sprintf("The printer at %s port %d accepted the connection but did not answer in time. It may be busy, out of paper, or not a Zebra printer.", ip, port))
	case errors.Is(err, sgd.ErrUnreachable):
		writeErr(w, http.StatusBadGateway, "unreachable",
			fmt.Sprintf("Can't reach the printer at %s port %d. Check the IP address, the cable or Wi-Fi, and that the printer is on.", ip, port))
	case errors.Is(err, printer.ErrBadResponse), errors.Is(err, sgd.ErrUnexpected):
		writeErr(w, http.StatusBadGateway, "bad_response",
			fmt.Sprintf("The printer at %s port %d answered, but not in a way this tool understands. It may not be a Zebra printer, or it may be busy.", ip, port))
	default:
		writeErr(w, http.StatusBadGateway, "printer", err.Error())
	}
}

func decode(w http.ResponseWriter, r *http.Request, dest any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dest); err != nil {
		msg := "The request was not understood."
		if errors.Is(err, io.EOF) {
			msg = "The request was empty."
		}
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			msg = "That file is too large. Exports from this tool are a few kilobytes."
		}
		writeErr(w, http.StatusBadRequest, "bad_request", msg)
		return err
	}
	return nil
}

func writeErr(w http.ResponseWriter, _ int, code, msg string) {
	// Expected failures (bad IP, printer timeout, "please confirm") are HTTP 200
	// with ok:false. The browser prints a red console line for every 4xx/5xx
	// fetch, and this page is used by people who will look at that console.
	writeJSON(w, http.StatusOK, map[string]any{"ok": false, "code": code, "error": msg})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	setHeaders(w)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(true)
	_ = enc.Encode(body)
}

func setHeaders(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self'; script-src 'self'; img-src 'self' data:; connect-src 'self'; base-uri 'none'; form-action 'self'")
}
