// Package sgd speaks Zebra Set-Get-Do and a few ZPL host queries over raw TCP
// port 9100.
//
// Command framing is from the ZPL Programming Guide, SGD overview:
//
//	! U1 getvar "name"\r\n
//	! U1 setvar "name" "value"\r\n
//	! U1 do "name" "value"\r\n
//
// The guide's getvar page says the printer returns the value in double quotes.
// Those replies are not terminated (no CR/LF), so this client stops at the
// closing quote and uses a read deadline so a missing quote cannot hang.
// setvar and do usually return nothing; a short quiet period treats silence
// as success. ZPL host replies such as ~HI are framed with STX/ETX and are
// read separately.
package sgd

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	// DefaultReadTimeout is long enough for a LAN getvar and short enough
	// that a dead printer does not freeze the screen.
	DefaultReadTimeout = 4 * time.Second
	DefaultDialTimeout = 5 * time.Second
	maxValueLen        = 4096
	maxRawLen          = 16 * 1024
)

var (
	ErrRefused     = errors.New("connection refused")
	ErrDialTimeout = errors.New("dial timeout")
	ErrReadTimeout = errors.New("read timeout")
	ErrUnreachable = errors.New("unreachable")
	ErrUnexpected  = errors.New("unexpected printer response")
	ErrClosed      = errors.New("connection closed")
)

// Client is one TCP session to a printer. Methods are safe to call from one
// goroutine at a time; a mutex also serializes commands on the connection.
type Client struct {
	conn        net.Conn
	readTimeout time.Duration
	mu          sync.Mutex
}

// Dial opens a TCP connection to host:port. host must already be an IP;
// this function does not resolve names.
func Dial(host string, port int, dialTimeout, readTimeout time.Duration) (*Client, error) {
	if dialTimeout <= 0 {
		dialTimeout = DefaultDialTimeout
	}
	if readTimeout <= 0 {
		readTimeout = DefaultReadTimeout
	}
	addr := fmt.Sprintf("%s:%d", host, port)
	conn, err := net.DialTimeout("tcp", addr, dialTimeout)
	if err != nil {
		return nil, mapDialError(err)
	}
	return &Client{conn: conn, readTimeout: readTimeout}, nil
}

// New wraps an existing connection. Tests use this with net.Pipe.
func New(conn net.Conn, readTimeout time.Duration) *Client {
	if readTimeout <= 0 {
		readTimeout = DefaultReadTimeout
	}
	return &Client{conn: conn, readTimeout: readTimeout}
}

// SetReadTimeout changes the deadline used for later reads.
func (c *Client) SetReadTimeout(d time.Duration) {
	if d > 0 {
		c.readTimeout = d
	}
}

// ReadTimeout returns the current read deadline duration.
func (c *Client) ReadTimeout() time.Duration { return c.readTimeout }

// Close closes the TCP connection.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return nil
	}
	err := c.conn.Close()
	c.conn = nil
	return err
}

// Get reads one SGD variable. The returned string has the surrounding quotes removed.
func (c *Client) Get(name string) (string, error) {
	vals, err := c.GetMany([]string{name})
	if err != nil {
		return "", err
	}
	return vals[0], nil
}

// GetMany writes several getvar commands and reads that many quoted replies, in order.
func (c *Client) GetMany(names []string) ([]string, error) {
	if len(names) == 0 {
		return nil, nil
	}
	var b strings.Builder
	for _, name := range names {
		line, err := GetVar(name)
		if err != nil {
			return nil, err
		}
		b.WriteString(line)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.writeLocked(b.String()); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(names))
	for range names {
		v, err := c.readQuotedLocked()
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

// Set writes one setvar. Silence from the printer is success.
// value may be empty (used by device.reset). Quotes and newlines are rejected
// so the framing cannot be broken by a setting value.
func (c *Client) Set(name, value string) error {
	line, err := SetVar(name, value)
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.writeLocked(line); err != nil {
		return err
	}
	c.drainLocked(120 * time.Millisecond)
	return nil
}

// Do writes one SGD do command. Silence is success.
func (c *Client) Do(name, value string) error {
	line, err := DoCmd(name, value)
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.writeLocked(line); err != nil {
		return err
	}
	c.drainLocked(120 * time.Millisecond)
	return nil
}

// Send writes one already-checked command line. When wantResponse is false,
// silence is success. When it is true, bytes are read until a short quiet
// period (ZPL host replies such as ~HI and ~HS).
func (c *Client) Send(line string, wantResponse bool) (string, error) {
	if !strings.HasSuffix(line, "\n") {
		line += "\r\n"
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.writeLocked(line); err != nil {
		return "", err
	}
	if !wantResponse {
		c.drainLocked(200 * time.Millisecond)
		return "", nil
	}
	buf, err := c.readRawLocked()
	if err != nil {
		return "", err
	}
	return string(buf), nil
}

func (c *Client) writeLocked(s string) error {
	if c.conn == nil {
		return ErrClosed
	}
	_ = c.conn.SetWriteDeadline(time.Now().Add(c.readTimeout))
	_, err := io.WriteString(c.conn, s)
	if err != nil {
		return fmt.Errorf("write to printer: %w", err)
	}
	return nil
}

// readQuotedLocked reads a single double-quoted SGD value. There is no
// terminator after the closing quote; see the package comment.
func (c *Client) readQuotedLocked() (string, error) {
	if c.conn == nil {
		return "", ErrClosed
	}
	deadline := time.Now().Add(c.readTimeout)
	if err := c.conn.SetReadDeadline(deadline); err != nil {
		return "", err
	}
	var b strings.Builder
	tmp := []byte{0}
	started := false
	for {
		_, err := c.conn.Read(tmp)
		if err != nil {
			if started && errors.Is(err, io.EOF) {
				return "", ErrReadTimeout
			}
			if isTimeout(err) || errors.Is(err, io.EOF) {
				return "", ErrReadTimeout
			}
			return "", err
		}
		ch := tmp[0]
		if !started {
			switch ch {
			case ' ', '\n', '\r', '\t':
				continue
			case '"':
				started = true
				continue
			default:
				return "", fmt.Errorf("%w: got byte 0x%02X before a quote", ErrUnexpected, ch)
			}
		}
		if ch == '"' {
			return b.String(), nil
		}
		if b.Len() >= maxValueLen {
			return "", fmt.Errorf("%w: value longer than %d bytes", ErrUnexpected, maxValueLen)
		}
		b.WriteByte(ch)
	}
}

func (c *Client) readRawLocked() ([]byte, error) {
	if c.conn == nil {
		return nil, ErrClosed
	}
	var out []byte
	buf := make([]byte, 512)
	first := true
	for len(out) < maxRawLen {
		wait := c.readTimeout
		if !first {
			wait = 200 * time.Millisecond
		}
		_ = c.conn.SetReadDeadline(time.Now().Add(wait))
		n, err := c.conn.Read(buf)
		if n > 0 {
			out = append(out, buf[:n]...)
			first = false
			continue
		}
		if err != nil {
			if !first && (isTimeout(err) || errors.Is(err, io.EOF)) {
				return out, nil
			}
			if isTimeout(err) || errors.Is(err, io.EOF) {
				return nil, ErrReadTimeout
			}
			return nil, err
		}
	}
	return out, nil
}

func (c *Client) drainLocked(d time.Duration) {
	if c.conn == nil {
		return
	}
	buf := make([]byte, 256)
	_ = c.conn.SetReadDeadline(time.Now().Add(d))
	for {
		_, err := c.conn.Read(buf)
		if err != nil {
			return
		}
		_ = c.conn.SetReadDeadline(time.Now().Add(80 * time.Millisecond))
	}
}

func mapDialError(err error) error {
	if errors.Is(err, syscall.ECONNREFUSED) {
		return fmt.Errorf("%w", ErrRefused)
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return fmt.Errorf("%w", ErrDialTimeout)
	}
	if errors.Is(err, syscall.EHOSTUNREACH) || errors.Is(err, syscall.ENETUNREACH) {
		return fmt.Errorf("%w", ErrUnreachable)
	}
	return fmt.Errorf("%w: %v", ErrUnreachable, err)
}

func isTimeout(err error) bool {
	if errors.Is(err, os.ErrDeadlineExceeded) {
		return true
	}
	var ne net.Error
	return errors.As(err, &ne) && ne.Timeout()
}
