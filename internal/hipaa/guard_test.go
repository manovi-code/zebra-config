package hipaa

import "testing"

func TestAllowsStatusAndSettings(t *testing.T) {
	ok := []string{
		`! U1 getvar "print.tone"`,
		`! U1 getvar "appl.name"`,
		`! U1 getvar "internal_wired.ip.addr"`,
		`~HI`,
		`~HS`,
		`^HH`,
		`^XA^HH^XZ`,
		`~WC`,
		`~JC`,
		`! U1 setvar "print.tone" "10.0"`,
		`^XA^JUR^XZ`,
		`^XA^JUS^XZ`,
	}
	for _, cmd := range ok {
		d := Inspect(cmd)
		if !d.Allow {
			t.Fatalf("blocked %q: %s", cmd, d.Reason)
		}
	}
	if Inspect(`! U1 getvar "print.tone"`).Write {
		t.Fatal("getvar should be a query")
	}
	if !Inspect(`~WC`).Write {
		t.Fatal("~WC should require confirm")
	}
	if Inspect(`~WC`).Extra {
		t.Fatal("~WC is not a network reset")
	}
	if !Inspect(`! U1 setvar "internal_wired.ip.addr" "10.1.1.9"`).Extra {
		t.Fatal("wired IP should require the extra warning")
	}
	if !Inspect(`^XA^JUF^XZ`).Extra || !Inspect(`^XA^JUN^XZ`).Extra || !Inspect(`^XA^JUA^XZ`).Extra {
		t.Fatal("factory and network resets need the extra warning")
	}
	if Inspect(`^XA^JUR^XZ`).Extra {
		t.Fatal("recall is a normal confirm, not the factory warning")
	}
}

func TestBlocksFileFormatAndLabelData(t *testing.T) {
	bad := []string{
		`! U1 getvar "file.dir"`,
		`! U1 getvar "file.type"`,
		`! U1 getvar "File.Dir"`,
		`! U1 getvar "allcv"`,
		`! U1 getvar "all"`,
		`! U1 getvar "capture.channel1.data.raw"`,
		`^XA^HF^XZ`,
		`^XA^HG^XZ`,
		`~DY`,
		`~DG`,
		`~DB`,
		`^WD`,
		`~HD`,
		`^XA^DFE:LABEL.ZPL^FS`,
		`^XA^XFE:LABEL.ZPL^XZ`,
		`^XA^XGIMAGE.GRF^XZ`,
		`^GF`,
		`^FDPatient^FS`,
		`! U1 do "file.delete" "E:FOO"`,
		"~HI\n~HS",
		"! U1 getvar \"print.tone\"\r\n! U1 getvar \"appl.name\"",
	}
	for _, cmd := range bad {
		d := Inspect(cmd)
		if d.Allow {
			t.Fatalf("allowed %q", cmd)
		}
	}
}
