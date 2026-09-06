// Design: docs/architecture/l2tp/bng-1-radius-attributes.md -- Filter-Id rate parsing tests

package traffic

import "testing"

// TestParseFilterIDRateForms walks the four accepted spellings of a RADIUS
// Filter-Id rate. The asymmetric forms are the reason this parser exists: a
// caller that hands the whole string to ParseRateBps accepts the symmetric ones
// and rejects the asymmetric ones, which is how the CoA listener came to NAK a
// value the shaper accepts at Access-Accept.
func TestParseFilterIDRateForms(t *testing.T) {
	cases := []struct {
		filterID   string
		download   uint64
		upload     uint64
		wantParsed bool
	}{
		{"10mbit", 10_000_000, 10_000_000, true},
		{"20mbit/5mbit", 20_000_000, 5_000_000, true},
		{"rate:100mbit/50mbit", 100_000_000, 50_000_000, true},
		{"rate:10mbit", 10_000_000, 10_000_000, true},
		{"1gbit", 1_000_000_000, 1_000_000_000, true},
	}
	for _, c := range cases {
		down, up, ok := ParseFilterIDRate(c.filterID)
		if ok != c.wantParsed {
			t.Fatalf("ParseFilterIDRate(%q) parsed = %v, want %v", c.filterID, ok, c.wantParsed)
		}
		if down != c.download {
			t.Errorf("ParseFilterIDRate(%q) download = %d, want %d", c.filterID, down, c.download)
		}
		if up != c.upload {
			t.Errorf("ParseFilterIDRate(%q) upload = %d, want %d", c.filterID, up, c.upload)
		}
	}
}

// TestParseFilterIDRateRejectsNonRates checks the other polarity. A Filter-Id
// is a free-form identifier, so a value that is not a rate must report so
// rather than yield a zero a caller would read as a rate of zero.
func TestParseFilterIDRateRejectsNonRates(t *testing.T) {
	for _, s := range []string{"", "rate:", "not-a-rate", "10", "abc/def"} {
		down, up, ok := ParseFilterIDRate(s)
		if ok {
			t.Errorf("ParseFilterIDRate(%q) reported a rate: %d/%d", s, down, up)
		}
	}
}
