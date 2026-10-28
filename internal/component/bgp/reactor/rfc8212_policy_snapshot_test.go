// Design: docs/architecture/api/architecture.md -- originated export admission before forwarding publication.
// RFC naming: untagged -- red existing-entry probe; discrimination follows implementation.
package reactor

import (
	"bytes"
	"encoding/hex"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
)

// TestRFC8212PolicyBeforeSnapshot drives the real originated-route gate before
// refreshForwardFacts, where startup must not silently turn absence into accept.
func TestRFC8212PolicyBeforeSnapshot(t *testing.T) {
	for _, policyCase := range []struct {
		name   string
		remote uint32
		final  filterapi.FinalDisposition
		deny   bool
	}{
		{name: "absent-ebgp", remote: 65001, deny: true},
		{name: "absent-ibgp", remote: 65000},
		{name: "explicit-accept", remote: 65001, final: filterapi.FinalAccept},
		{name: "explicit-reject", remote: 65001, final: filterapi.FinalReject, deny: true},
	} {
		t.Run(policyCase.name, func(t *testing.T) {
			var policy filterapi.Policy
			if policyCase.final != 0 {
				policy = mustPolicy(t, nil, policyCase.final)
			}
			peer := NewPeer(&PeerSettings{Address: netip.MustParseAddr("192.0.2.1"),
				LocalAS: 65000, PeerAS: policyCase.remote, ExportPolicy: policy})
			if peer.forwardFacts() != nil {
				t.Fatal("probe requires the unpublished startup forwarding snapshot")
			}
			for _, update := range []struct{ name, body, withdrawals string }{
				{"announce", "0000000e40010100400200400304c0000201180a5201", ""},
				{"withdraw", "0004180a52c80000", "0004180a52c80000"},
				{"mixed", "0004180a52c8000e40010100400200400304c0000201180a5201", "0004180a52c80000"},
				{"mp-withdraw", "0000000d800f0a0002013020010db80001", "0000000d800f0a0002013020010db80001"},
			} {
				t.Run(update.name, func(t *testing.T) {
					body, err := hex.DecodeString(update.body)
					if err != nil {
						t.Fatal(err)
					}
					want := body
					if policyCase.deny {
						want, err = hex.DecodeString(update.withdrawals)
						if err != nil {
							t.Fatal(err)
						}
					}
					suppress, override := (&Reactor{}).exportFilterForBody(peer, body)
					if suppress != (len(want) == 0) {
						t.Fatalf("suppressed=%t, want %t", suppress, len(want) == 0)
					}
					if suppress {
						return
					}
					got := body
					if override != nil {
						got = override
					}
					if !bytes.Equal(got, want) {
						t.Fatalf("export body=%x, want %x", got, want)
					}
				})
			}
		})
	}
}
