// RFC: rfc/short/rfc9552.md -- Section 8.2.2 fault management for BGP-LS
// Overview: rfc7606_bgpls_nlri.go -- the walk this target drives
// Related: fuzz_test.go -- the other receive-path targets of this package

package message

import (
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// FuzzRetainWellFormedNLRI drives the RFC 9552 Section 8.2.2 Link-State NLRI walk with
// arbitrary bytes.
//
// VALIDATES: the walk terminates and answers, whatever a peer sends. It also checks the one
// property the walk owes its caller beyond not crashing: what it keeps must itself be clean,
// so a second pass over the survivors drops nothing.
// PREVENTS: a peer panicking the daemon with a length field, which every octet of an NLRI
// section is. Each length here is attacker-controlled and each loop is bounded by a length
// read before the loop starts, and a fuzz target is how that claim stays true.
// SECURITY: this walk runs on the receive path, before any policy, for every UPDATE a BGP-LS
// peer sends.
func FuzzRetainWellFormedNLRI(f *testing.F) {
	f.Add([]byte{}, false)
	f.Add([]byte{0x00}, false)
	// A well-formed Node NLRI: Protocol-ID, Identifier, TLV 256 holding sub-TLV 512.
	f.Add([]byte{
		0x00, 0x01, 0x00, 0x15,
		0x02,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x01, 0x00, 0x00, 0x08,
		0x02, 0x00, 0x00, 0x04, 0x00, 0x00, 0xfd, 0xe9,
	}, false)
	// The same NLRI with a Total NLRI Length that overruns the section.
	f.Add([]byte{0x00, 0x01, 0xff, 0xff, 0x02}, false)
	// A Node Descriptor TLV whose length overruns the NLRI body.
	f.Add([]byte{
		0x00, 0x01, 0x00, 0x0d,
		0x02,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x01, 0x00, 0xff, 0xff,
	}, false)
	// A Node Descriptor carrying sub-TLV 512 twice.
	f.Add([]byte{
		0x00, 0x01, 0x00, 0x1d,
		0x02,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x01, 0x00, 0x00, 0x10,
		0x02, 0x00, 0x00, 0x04, 0x00, 0x00, 0xfd, 0xe9,
		0x02, 0x00, 0x00, 0x04, 0x00, 0x00, 0xfd, 0xea,
	}, false)
	// An unknown NLRI type, which Section 5.2 makes opaque, behind a path identifier.
	f.Add([]byte{0x00, 0x00, 0x00, 0x07, 0x00, 0x63, 0x00, 0x02, 0xde, 0xad}, true)

	f.Fuzz(func(t *testing.T, section []byte, addPath bool) {
		for _, safi := range []attribute.SAFI{safiBGPLS, safiBGPLSVPN} {
			kept, dropped, framed := RetainWellFormedNLRI(afiBGPLS, safi, section, addPath)
			if !framed {
				if dropped != 0 {
					t.Fatalf("an unwalkable section reported %d discards", dropped)
				}
				continue
			}
			if len(kept) > len(section) {
				t.Fatalf("the walk kept %d octets of a %d octet section", len(kept), len(section))
			}
			again, droppedAgain, framedAgain := RetainWellFormedNLRI(afiBGPLS, safi, kept, addPath)
			if !framedAgain {
				t.Fatalf("the survivors of a walkable section did not walk: %x", kept)
			}
			if droppedAgain != 0 {
				t.Fatalf("a second pass discarded %d more of %x", droppedAgain, kept)
			}
			if len(again) != len(kept) {
				t.Fatalf("a second pass changed the section from %x to %x", kept, again)
			}
			// The framing walk judges the same octets, so it must agree that a section
			// this one could walk is framed.
			if result := validateBGPLSNLRISyntax(14, section, addPath); result != nil {
				t.Fatalf("the framing walk called %x malformed where the discard walk did not", section)
			}
			_ = dropped
		}
	})
}
