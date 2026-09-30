// VALIDATES: the RIB declares two extended communities equal only when all 8
// octets match (RFC 4360 Section 2).
// PREVENTS: a comparator that ignores an octet and hides a changed community
// on replay.

package rib

import (
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// TestRFC4360ExtendedCommunitiesEqualOnlyOnAllEightOctets drives the
// comparator the RIB uses to decide whether a replayed route's Extended
// Communities changed (extCommunitiesEqual, rib_replay.go).
// Method: a community equals its octet-for-octet copy; then each of the 8
// octets in turn is changed, and the comparator must declare the pair unequal
// for every position, including the value octets after Type and Sub-Type and
// the last one.
//
// RFC requirement: RFC4360-2-1 positive -- two extended communities with all 8 octets equal are declared equal by the RIB comparator.
// RFC requirement: RFC4360-2-1 negative -- a difference in any one of the 8 octets, first through last, makes the comparator declare them unequal.
func TestRFC4360ExtendedCommunitiesEqualOnlyOnAllEightOctets(t *testing.T) {
	base := attribute.ExtendedCommunity{0x00, 0x02, 0xFD, 0xE8, 0x00, 0x00, 0x00, 0x64}
	copyOf := base
	if !extCommunitiesEqual([]attribute.ExtendedCommunity{base}, []attribute.ExtendedCommunity{copyOf}) {
		t.Fatal("communities equal in all 8 octets were declared unequal")
	}
	for i := range base {
		changed := base
		changed[i] ^= 0x01
		if extCommunitiesEqual([]attribute.ExtendedCommunity{base}, []attribute.ExtendedCommunity{changed}) {
			t.Errorf("octet %d differs but the communities were declared equal", i)
		}
	}
}
