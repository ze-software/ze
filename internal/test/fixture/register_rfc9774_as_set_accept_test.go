package fixture

import (
	"bytes"
	"testing"
)

// TestPathAttributeValueWalksPackedAttributes proves the observer's attribute
// walk finds the AS_PATH value behind a one-octet and an extended-length
// header, and refuses a truncated attribute or an absent code rather than
// answering an empty value the .ci would then compare.
//
// VALIDATES: ORIGIN then AS_PATH answers the AS_PATH value bytes; the same
// AS_PATH behind an Extended Length header answers the same bytes; a value
// shorter than its length, a header cut short, and a list with no AS_PATH each
// return an error.
// PREVENTS: the .ci reading a wrong slice as the received segment bytes.
func TestPathAttributeValueWalksPackedAttributes(t *testing.T) {
	asPath := []byte{0x01, 0x02, 0x00, 0x00, 0xFD, 0xF2, 0x00, 0x00, 0xFD, 0xF3}
	origin := []byte{0x40, 0x01, 0x01, 0x00}
	short := append(append([]byte{}, origin...), 0x40, 0x02, byte(len(asPath)))
	short = append(short, asPath...)
	extended := append(append([]byte{}, origin...), 0x50, 0x02, 0x00, byte(len(asPath)))
	extended = append(extended, asPath...)

	for name, attrs := range map[string][]byte{"one-octet": short, "extended": extended} {
		got, err := pathAttributeValue(attrs, attrCodeASPath)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !bytes.Equal(got, asPath) {
			t.Fatalf("%s: AS_PATH value % X, want % X", name, got, asPath)
		}
	}

	refused := map[string][]byte{
		"value truncated":  short[:len(short)-1],
		"header truncated": append(append([]byte{}, origin...), 0x40, 0x02),
		"no AS_PATH":       origin,
	}
	for name, attrs := range refused {
		if got, err := pathAttributeValue(attrs, attrCodeASPath); err == nil {
			t.Fatalf("%s: answered % X, want an error", name, got)
		}
	}
}
