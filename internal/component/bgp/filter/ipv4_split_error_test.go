// Design: docs/architecture/api/json-format.md -- the error convention of a family operation
// Related: filter.go -- AnnouncedByFamily, WithdrawnByFamily

package filter

import (
	"bytes"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/family"
)

// TestBodyIPv4SplitErrorIsReturned groups a FilterResult whose body NLRI or
// Withdrawn Routes field does not split: a /24 carrying two of its three
// octets, after a whole 10.0.0.0/8.
//
// VALIDATES: AnnouncedByFamily and WithdrawnByFamily return one ipv4/unicast
// entry with SplitErr set, no NLRIs and the whole field in Raw, which is what
// json-format.md says the `format full` producer renders as the error member.
// PREVENTS: the body IPv4 branch skipping a field that did not split, or
// returning the routes before the break as if they were the whole field.
func TestBodyIPv4SplitErrorIsReturned(t *testing.T) {
	t.Parallel()

	field := []byte{8, 10, 24, 0x0b, 0x0c}
	cases := []struct {
		name    string
		result  FilterResult
		grouped func(FilterResult) []FamilyNLRI
	}{
		{
			name:    "announced",
			result:  FilterResult{IPv4Announced: wireu.NewIPv4Reach([]byte{192, 0, 2, 1}, field)},
			grouped: func(r FilterResult) []FamilyNLRI { return r.AnnouncedByFamily(nil) },
		},
		{
			name:    "withdrawn",
			result:  FilterResult{IPv4Withdrawn: wireu.NewIPv4Withdraw(field)},
			grouped: func(r FilterResult) []FamilyNLRI { return r.WithdrawnByFamily(nil) },
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			entries := tt.grouped(tt.result)
			if len(entries) != 1 {
				t.Fatalf("got %d entries, want 1: %+v", len(entries), entries)
			}
			entry := entries[0]
			if entry.Family != family.IPv4Unicast {
				t.Errorf("family = %s, want ipv4/unicast", entry.Family)
			}
			if entry.SplitErr == nil {
				t.Fatal("SplitErr is nil for a field that does not split")
			}
			if entry.NLRIs != nil {
				t.Errorf("NLRIs = %v, want nil beside SplitErr", entry.NLRIs)
			}
			if !bytes.Equal(entry.Raw, field) {
				t.Errorf("Raw = %X, want %X", entry.Raw, field)
			}
		})
	}
}
