// Design: docs/architecture/ospf/ospf-1-types.md -- Options bit-field tests

package types

import "testing"

// VALIDATES: AC-9 - each Options bit is set, cleared, tested, formatted, and serialized independently.
// PREVENTS: Hello/LSA capability bits from being conflated across area types.
func TestOptionsBits(t *testing.T) {
	var opts Options
	for _, bit := range []Options{OptionE, OptionMC, OptionNP, OptionL, OptionDC, OptionO, OptionDN} {
		if opts.Has(bit) {
			t.Fatalf("zero options unexpectedly has bit %#x", bit)
		}
		opts = opts.Set(bit)
		if !opts.Has(bit) {
			t.Fatalf("Set(%#x) did not set bit", bit)
		}
		opts = opts.Clear(bit)
		if opts.Has(bit) {
			t.Fatalf("Clear(%#x) did not clear bit", bit)
		}
	}
	opts = opts.Set(OptionE).Set(OptionNP).Set(OptionDC).Set(OptionO).Set(OptionDN)
	for _, bit := range []Options{OptionE, OptionNP, OptionDC, OptionO, OptionDN} {
		if !opts.Has(bit) {
			t.Fatalf("combined options missing bit %#x", bit)
		}
	}
	if got := opts.String(); got != "E,N/P,DC,O,DN" {
		t.Fatalf("Options.String() = %q, want E,N/P,DC,O,DN", got)
	}
	fromBytes, err := OptionsFromBytes([]byte{byte(opts)})
	if err != nil {
		t.Fatalf("OptionsFromBytes returned error: %v", err)
	}
	if fromBytes != opts {
		t.Fatalf("OptionsFromBytes = %#x, want %#x", byte(fromBytes), byte(opts))
	}
	var buf [1]byte
	if n := opts.WriteTo(buf[:], 0); n != OptionsLen || buf[0] != byte(opts) {
		t.Fatalf("Options.WriteTo n=%d byte=%#x", n, buf[0])
	}
	if _, err := OptionsFromBytes(nil); err == nil {
		t.Fatalf("OptionsFromBytes nil succeeded")
	}
}
