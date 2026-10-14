// Design: docs/architecture/wire/capabilities.md -- WalkOptionalParams, the one reader of the parameter framing
// Overview: optional_params.go -- WalkOptionalParams, ParseFromOptionalParams

package capability

import (
	"bytes"
	"errors"
	"testing"
)

// TestWalkOptionalParams proves the one framing reader hands every parameter to
// its caller, in wire order and of every type, under the framing the caller
// names, and refuses a parameter that runs past the field.
//
// VALIDATES: spec-bgp-open-session-rfc-defects D1: WalkOptionalParams reads a
// one-octet Parameter Length in the RFC 4271 form and a two-octet one in the
// RFC 9072 Section 2 form; a non-capability parameter reaches the callback.
// PREVENTS: a reader that needs raw capability octets falling back to a
// one-octet loop of its own because the walker hides parameters it skips.
func TestWalkOptionalParams(t *testing.T) {
	type param struct {
		kind  uint8
		value []byte
	}
	want := []param{{2, []byte{9, 1, 3}}, {7, []byte{0xaa}}}
	for _, tc := range []struct {
		name     string
		extended bool
		wire     []byte
	}{
		{"RFC 4271 form", false, []byte{2, 3, 9, 1, 3, 7, 1, 0xaa}},
		{"RFC 9072 form", true, []byte{2, 0, 3, 9, 1, 3, 7, 0, 1, 0xaa}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got []param
			err := WalkOptionalParams(tc.wire, tc.extended, func(kind uint8, value []byte) error {
				got = append(got, param{kind, value})
				return nil
			})
			if err != nil {
				t.Fatalf("walk: %v", err)
			}
			if len(got) != len(want) {
				t.Fatalf("parameters %v, want %v", got, want)
			}
			for i := range want {
				if got[i].kind != want[i].kind {
					t.Errorf("parameter %d type %d, want %d", i, got[i].kind, want[i].kind)
				}
				if !bytes.Equal(got[i].value, want[i].value) {
					t.Errorf("parameter %d value %x, want %x", i, got[i].value, want[i].value)
				}
			}
		})
	}

	for _, tc := range []struct {
		name     string
		extended bool
		wire     []byte
	}{
		{"one-octet length past the field", false, []byte{2, 4, 9, 1, 3}},
		{"two-octet length past the field", true, []byte{2, 1, 0, 9, 1, 3}},
		{"two-octet header cut short", true, []byte{2, 0}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := WalkOptionalParams(tc.wire, tc.extended, func(uint8, []byte) error { return nil })
			if !errors.Is(err, ErrShortRead) {
				t.Fatalf("error %v, want ErrShortRead", err)
			}
		})
	}

	t.Run("callback error stops the walk", func(t *testing.T) {
		stop := errors.New("stop")
		calls := 0
		err := WalkOptionalParams([]byte{2, 1, 9, 2, 1, 9}, false, func(uint8, []byte) error {
			calls++
			return stop
		})
		if !errors.Is(err, stop) {
			t.Fatalf("error %v, want the callback's", err)
		}
		if calls != 1 {
			t.Errorf("callback ran %d times, want 1", calls)
		}
	})
}
