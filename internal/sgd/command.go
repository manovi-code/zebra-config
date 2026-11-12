package sgd

import (
	"fmt"
	"strings"
	"unicode"
)

// GetVar formats one getvar command, including the CR/LF the guide requires.
func GetVar(name string) (string, error) {
	if err := ValidateName(name); err != nil {
		return "", err
	}
	return fmt.Sprintf("! U1 getvar \"%s\"\r\n", name), nil
}

// SetVar formats one setvar command.
func SetVar(name, value string) (string, error) {
	if err := ValidateName(name); err != nil {
		return "", err
	}
	if err := ValidateValue(value); err != nil {
		return "", err
	}
	return fmt.Sprintf("! U1 setvar \"%s\" \"%s\"\r\n", name, value), nil
}

// DoCmd formats one do command.
func DoCmd(name, value string) (string, error) {
	if err := ValidateName(name); err != nil {
		return "", err
	}
	if err := ValidateValue(value); err != nil {
		return "", err
	}
	return fmt.Sprintf("! U1 do \"%s\" \"%s\"\r\n", name, value), nil
}

// ValidateName allows only the dotted SGD names published in the guide.
func ValidateName(name string) error {
	if name == "" || len(name) > 80 {
		return fmt.Errorf("invalid setting name")
	}
	for i, r := range name {
		if i == 0 && !unicode.IsLetter(r) {
			return fmt.Errorf("invalid setting name %q", name)
		}
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '.' && r != '_' {
			return fmt.Errorf("invalid setting name %q", name)
		}
	}
	return nil
}

// Visible turns printer bytes into text a technician can read.
// Control bytes are named so a reply is never mistaken for a blank screen.
func Visible(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case 0x02:
			b.WriteString("<STX>")
		case 0x03:
			b.WriteString("<ETX>")
		case '\r':
			b.WriteString("<CR>")
		case '\n':
			b.WriteByte('\n')
		default:
			if c < 32 || c > 126 {
				fmt.Fprintf(&b, "<%02X>", c)
			} else {
				b.WriteByte(c)
			}
		}
	}
	return b.String()
}

// ValidateValue rejects quotes and control characters so a value cannot
// escape the setvar quotes or look like a second command.
func ValidateValue(value string) error {
	if len(value) > 200 {
		return fmt.Errorf("value is too long")
	}
	if strings.ContainsAny(value, "\"\r\n\x00") {
		return fmt.Errorf("value contains a character this tool will not send")
	}
	for _, r := range value {
		if r < 32 || r == 127 {
			return fmt.Errorf("value contains a control character")
		}
	}
	return nil
}
