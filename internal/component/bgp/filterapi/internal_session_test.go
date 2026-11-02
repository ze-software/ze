package filterapi

import "testing"

// TestInternalSession checks the one internal-session rule over its four
// shapes: the plain equality, a different AS with no migration ASN, a session
// formed under the RFC 7705 Section 4.2 migration ASN (internal), and a third
// AS beside a migration ASN (external). Method: a table of verdicts.
func TestInternalSession(t *testing.T) {
	cases := []struct {
		name                         string
		peerAS, localAS, migrationAS uint32
		want                         bool
	}{
		{"same AS", 65000, 65000, 0, true},
		{"other AS", 65001, 65000, 0, false},
		{"migration AS", 64999, 65000, 64999, true},
		{"third AS beside migration", 65001, 65000, 64999, false},
	}
	for _, tc := range cases {
		if got := InternalSession(tc.peerAS, tc.localAS, tc.migrationAS); got != tc.want {
			t.Errorf("%s: InternalSession(%d, %d, %d) = %v, want %v",
				tc.name, tc.peerAS, tc.localAS, tc.migrationAS, got, tc.want)
		}
	}
}
