package ci

import (
	"errors"
	"strings"
	"testing"
)

// TestParseKVPairsRefusesMalformedSegments drives the three shapes the splitter
// used to settle in silence.
//
// VALIDATES: a segment with no `=`, a key given twice, and a key that cannot
// start a key token (so it was glued into the previous value) are each an
// error that names the offending text and wraps the sentinel for its shape,
// so a caller can tell the causes apart with errors.Is.
// PREVENTS: `option=asn:65000` parsing as an option with no value, and
// `option=timeout:value=1s:value=9s` keeping whichever value came last.
// docs/architecture/testing/ci-format.md promises an unknown key is a parse
// error; a key the splitter never surfaces cannot be checked by anyone above it.
func TestParseKVPairsRefusesMalformedSegments(t *testing.T) {
	cases := []struct {
		name   string
		parts  []string
		wantIn []string
		wantIs error
	}{
		{"segment with no equals sign", []string{"65000"}, []string{`"65000"`, "key=value"}, ErrSegmentWithoutKey},
		{"key-less segment before a key", []string{"stdout", "contains=x"}, []string{`"stdout"`}, ErrSegmentWithoutKey},
		{"duplicate key", []string{"value=1s", "value=9s"}, []string{`"value"`, "twice"}, ErrKeyGivenTwice},
		{"duplicate key among others", []string{"conn=1", "seq=1", "conn=2"}, []string{`"conn"`, "twice"}, ErrKeyGivenTwice},
		{"non-letter key glued to a value", []string{"path=out.txt", "!contains=x"}, []string{`"!contains"`, "path"}, ErrKeyGluedToValue},
		{"digit-led key glued to a value", []string{"contains=a", "2nd=b"}, []string{`"2nd"`, "contains"}, ErrKeyGluedToValue},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseKVPairs(tc.parts)
			if err == nil {
				t.Fatalf("ParseKVPairs(%q) = %v with no error; it must be refused", tc.parts, got)
			}
			if !errors.Is(err, tc.wantIs) {
				t.Errorf("error %q does not wrap %q", err, tc.wantIs)
			}
			for _, want := range tc.wantIn {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not carry %q", err, want)
				}
			}
		})
	}
}

// TestParseKVPairsStrictKeepsValidShapes is the other polarity: the shapes the
// corpus writes still parse, including a colon inside a value and a complex key.
func TestParseKVPairsStrictKeepsValidShapes(t *testing.T) {
	for _, parts := range [][]string{
		{"contains=error", " no such peer"},
		{"contains=http", "//host", "8080/p"},
		{"conn=1", "seq=1", "hex=FF:FF"},
		{"conn=1", "seq=1", "pattern=a=1:a=2"},
		{"key="},
	} {
		if _, err := ParseKVPairs(parts); err != nil {
			t.Errorf("ParseKVPairs(%q): %v", parts, err)
		}
	}
}
