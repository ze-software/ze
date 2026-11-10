// VALIDATES: the peer leaf icmp-error-check reaches SiteToSitePeer, with the RFC 4301
// Section 6.2 processing on when the leaf is absent or true and off when it is false.
// PREVENTS: the leaf parsed and dropped, which leaves the operator no way to turn the
// check off (Section 6.2: "it is also necessary to allow a local administrator to
// configure an implementation to NOT perform such checks").

package ipsec

import (
	"testing"

	"github.com/ze-software/ze/internal/component/config"
)

// RFC requirement: RFC4301-6.2-1 positive -- the peer's icmp-error-check leaf turns the
// ICMP error processing off when false and leaves it on when absent or true.
func TestPeerICMPErrorCheckLeafIsParsed(t *testing.T) {
	cases := []struct {
		name     string
		value    string
		set      bool
		wantSkip bool
	}{
		{name: "absent leaf keeps the check", set: false, wantSkip: false},
		{name: "true keeps the check", value: "true", set: true, wantSkip: false},
		{name: "false turns the check off", value: "false", set: true, wantSkip: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree := config.NewTree()
			if tc.set {
				tree.Set("icmp-error-check", tc.value)
			}
			peer, err := parseSiteToSitePeer("icmp", tree)
			if err != nil {
				t.Fatalf("parse peer: %v", err)
			}
			if peer.SkipICMPErrorCheck != tc.wantSkip {
				t.Errorf("SkipICMPErrorCheck = %v, want %v", peer.SkipICMPErrorCheck, tc.wantSkip)
			}
		})
	}
}
