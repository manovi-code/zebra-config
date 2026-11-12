package sgd

import (
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

func TestReadQuotedHasNoTerminator(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()
	go func() {
		buf := make([]byte, 128)
		_, _ = io.ReadAtLeast(server, buf, 8)
		// The guide's getvar reply is a quoted string and nothing after it.
		_, _ = server.Write([]byte(`"ZD411"`))
	}()
	c := New(client, time.Second)
	got, err := c.Get("device.product_name")
	if err != nil {
		t.Fatal(err)
	}
	if got != "ZD411" {
		t.Fatalf("got %q", got)
	}
}

func TestReadQuotedTimesOutWithoutCloseQuote(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()
	go func() {
		buf := make([]byte, 128)
		_, _ = io.ReadAtLeast(server, buf, 8)
		_, _ = server.Write([]byte(`"ZD411`))
	}()
	c := New(client, 200*time.Millisecond)
	_, err := c.Get("device.product_name")
	if err == nil || !strings.Contains(err.Error(), "timeout") && !isTimeout(err) && err != ErrReadTimeout {
		// errors.Is through the return path: readQuoted returns ErrReadTimeout directly.
		if err != ErrReadTimeout {
			t.Fatalf("err = %v", err)
		}
	}
}

func TestGetManyBackToBackQuotes(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()
	go func() {
		buf := make([]byte, 512)
		_, _ = io.ReadAtLeast(server, buf, 20)
		_, _ = server.Write([]byte(`"ZD411""203"`))
	}()
	c := New(client, time.Second)
	got, err := c.GetMany([]string{"device.product_name", "head.resolution.in_dpi"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "ZD411" || got[1] != "203" {
		t.Fatalf("got %#v", got)
	}
}

func TestRejectsQuoteInValue(t *testing.T) {
	if _, err := SetVar("device.friendly_name", `a"b`); err == nil {
		t.Fatal("expected rejection")
	}
}
