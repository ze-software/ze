package storage

import (
	"testing"

	"github.com/ze-software/ze/internal/core/family"
)

// Identical wire attributes do not make a new received UPDATE the old owner.
func TestInsertEntryRefreshesReceivedGeneration(t *testing.T) {
	for _, tc := range []struct {
		name    string
		fam     family.Family
		addPath bool
		nlri    []byte
	}{
		{"cidr", family.IPv4Unicast, false, []byte{24, 192, 0, 2}},
		{"addpath", family.IPv4Unicast, true, []byte{0, 0, 0, 7, 24, 192, 0, 2}},
		{"opaque", family.Family{AFI: family.AFIL2VPN, SAFI: family.SAFIEVPN}, false, []byte{1, 1, 0}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			peer := NewPeerRIB("192.0.2.10")
			defer peer.Release()
			peer.SetAddPath(tc.fam, tc.addPath)
			entry, fp, size, err := ParseRouteEntry([]byte{0x40, 1, 1, 0})
			if err != nil {
				t.Fatal(err)
			}
			defer entry.Release()
			entry.MsgID = 91
			peer.InsertEntry(tc.fam, entry, fp, size, tc.nlri)
			entry.MsgID = 92
			peer.InsertEntry(tc.fam, entry, fp, size, tc.nlri)
			got, ok := peer.Lookup(tc.fam, tc.nlri)
			if !ok || got.MsgID != 92 {
				t.Fatalf("new generation lost on fingerprint hit: found=%t MsgID=%d", ok, got.MsgID)
			}
		})
	}
}
