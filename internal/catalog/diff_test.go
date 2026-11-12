package catalog

import "testing"

func TestPrintModeCanonicalAndEncode(t *testing.T) {
	for _, in := range []string{"T", "t", "tear off", "TEAR OFF"} {
		got, ok := Canonical("media.printmode", in)
		if !ok || got != "tear off" {
			t.Fatalf("Canonical(%q) = %q, %v", in, got, ok)
		}
	}
	wire, err := Encode("media.printmode", "peel off")
	if err != nil || wire != "P" {
		t.Fatalf("Encode peel off = %q, %v", wire, err)
	}
	if !Same("media.printmode", "TEAR OFF", "tear off") {
		t.Fatal("tear off forms should compare equal")
	}
	if Same("media.printmode", "tear off", "peel off") {
		t.Fatal("tear off and peel off should differ")
	}
}

func TestLanguageZPLAlias(t *testing.T) {
	got, ok := Canonical("device.languages", "zpl")
	if !ok || got != "hybrid_xml_zpl" {
		t.Fatalf("zpl alias = %q, %v", got, ok)
	}
	if !Same("device.languages", "hybrid_xml_zpl", "zpl") {
		t.Fatal("zpl and hybrid_xml_zpl should compare equal")
	}
}

func TestToneNumericSame(t *testing.T) {
	if !Same("print.tone", "10", "10.0") {
		t.Fatal("10 and 10.0 should compare equal")
	}
	c, ok := Canonical("print.tone", "15")
	if !ok || c != "15.0" {
		t.Fatalf("tone canonical = %q, %v", c, ok)
	}
	if _, ok := Canonical("print.tone", "31"); ok {
		t.Fatal("31 should be rejected")
	}
}

func TestDiffSkipsUnchangedAndFlagsNetwork(t *testing.T) {
	current := map[string]string{
		"print.tone":                 "10.0",
		"ezpl.media_type":            "gap/notch",
		"internal_wired.ip.addr":     "192.168.1.50",
		"internal_wired.ip.protocol": "dhcp",
	}
	rows, err := Diff(current, []Change{
		{ID: "print.tone", Value: "10"},
		{ID: "ezpl.media_type", Value: "continuous"},
		{ID: "internal_wired.ip.addr", Value: "192.168.1.60"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %+v", rows)
	}
	if rows[0].ID != "ezpl.media_type" || rows[0].AfterDisplay != "Continuous (receipts)" {
		t.Fatalf("first row = %+v", rows[0])
	}
	if !NeedsExtra(rows) {
		t.Fatal("expected network confirm")
	}
}

func TestDiffRejectsUnknownAndBadValue(t *testing.T) {
	if _, err := Diff(map[string]string{}, []Change{{ID: "file.dir", Value: "E:"}}); err == nil {
		t.Fatal("file.dir must not be a setting")
	}
	_, err := Diff(map[string]string{"print.tone": "10.0"}, []Change{{ID: "print.tone", Value: "dark"}})
	if err == nil {
		t.Fatal("expected invalid darkness")
	}
}

func TestNetmaskAndPort(t *testing.T) {
	if _, ok := Canonical("internal_wired.ip.netmask", "255.255.255.0"); !ok {
		t.Fatal("mask should be accepted")
	}
	if _, ok := Canonical("internal_wired.ip.netmask", "255.0.255.0"); ok {
		t.Fatal("non-contiguous mask should be rejected")
	}
	if _, ok := Canonical("internal_wired.ip.port", "9100"); !ok {
		t.Fatal("9100 should be accepted")
	}
	if _, ok := Canonical("internal_wired.ip.port", "80"); ok {
		t.Fatal("port 80 is excluded by the guide")
	}
	if _, ok := Canonical("ezpl.print_width", "449"); ok {
		t.Fatal("width above 448 should be rejected for a 203 dpi write")
	}
}

func TestFriendlyName(t *testing.T) {
	if _, ok := Canonical("device.friendly_name", "Shipping desk"); !ok {
		t.Fatal("expected a normal name to pass")
	}
	if _, ok := Canonical("device.friendly_name", "a\"b"); ok {
		t.Fatal("quotes must be rejected")
	}
}
