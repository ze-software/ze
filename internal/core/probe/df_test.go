package probe

import (
	"context"
	"errors"
	"net/netip"
	"testing"
)

// TestDFModeZeroIsUnspecified proves the typed mode's zero value is not a
// valid DF setting: it names itself "unspecified", and OpenICMP refuses it by
// name before any socket is opened, so a caller that never chose a mode can
// not probe with the DF bit silently clear.
func TestDFModeZeroIsUnspecified(t *testing.T) {
	var zero DFMode
	if zero != DFUnspecified {
		t.Fatalf("zero DFMode = %v, want DFUnspecified", zero)
	}
	if got := zero.String(); got != "unspecified" {
		t.Fatalf("zero DFMode.String() = %q, want %q", got, "unspecified")
	}
	for _, valid := range []DFMode{DFOff, DFHonorCache, DFBypassCache} {
		if valid == DFUnspecified {
			t.Fatalf("%v collides with the zero value", valid)
		}
	}

	conn, err := OpenICMP(context.Background(), FamilyIPv4, netip.Addr{}, zero)
	if !errors.Is(err, ErrDFUnspecified) {
		t.Fatalf("OpenICMP(zero mode) err = %v, want ErrDFUnspecified", err)
	}
	if conn != nil {
		t.Fatalf("OpenICMP(zero mode) returned a conn beside the refusal")
	}
}

// TestOpenICMPRefusesFamilyAny proves the constructor needs one family: an
// ICMP socket is ICMPv4 or ICMPv6, and FamilyAny names neither.
func TestOpenICMPRefusesFamilyAny(t *testing.T) {
	conn, err := OpenICMP(context.Background(), FamilyAny, netip.Addr{}, DFOff)
	if !errors.Is(err, ErrFamilyRequired) {
		t.Fatalf("OpenICMP(FamilyAny) err = %v, want ErrFamilyRequired", err)
	}
	if conn != nil {
		t.Fatalf("OpenICMP(FamilyAny) returned a conn beside the refusal")
	}
}

// TestDFModeOfValue pins the CLI spelling: each of the two value words names
// its mode, and any other word, the empty word of a bare keyword included, is
// refused with ErrDFValueUnknown and the zero mode, so no parser can turn a
// typo or a bare keyword into a probe.
func TestDFModeOfValue(t *testing.T) {
	cases := []struct {
		word string
		mode DFMode
		err  error
	}{
		{"honor-cache", DFHonorCache, nil},
		{"bypass-cache", DFBypassCache, nil},
		{"count", DFUnspecified, ErrDFValueUnknown},
		{"", DFUnspecified, ErrDFValueUnknown},
	}
	for _, tc := range cases {
		mode, err := DFModeOfValue(tc.word)
		if mode != tc.mode || !errors.Is(err, tc.err) {
			t.Errorf("DFModeOfValue(%q) = (%v, %v), want (%v, %v)", tc.word, mode, err, tc.mode, tc.err)
		}
	}
}
