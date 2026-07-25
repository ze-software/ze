// VALIDATES: spec-ospf-ext-2 TE opaque consumer wiring -- registerTEConsumer registers
// Opaque type 1 (RFC 3630) and type 6 (RFC 5392) with the carrier; a duplicate Opaque
// Type is rejected; reception of a malformed TE body never panics and stores nothing;
// reception of a type-6 LSA carrying the prohibited Link ID sub-TLV is skipped.
// PREVENTS: the TE consumer compiling but never registering, and untrusted-input crashes.
package ospf

import (
	"errors"
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

func TestTEConsumerRegistered(t *testing.T) {
	resetOpaqueConsumers()
	t.Cleanup(resetOpaqueConsumers)

	eng, _ := newRedistEngine(t, teCfgJSON)
	if err := registerTEConsumer(eng); err != nil {
		t.Fatalf("registerTEConsumer: %v", err)
	}
	// AC-1: Opaque type 1 (area) and type 6 are both registered.
	if _, ok := lookupOpaqueConsumer(packet.TEOpaqueType); !ok {
		t.Fatalf("Opaque type 1 (RFC 3630 TE) not registered")
	}
	if _, ok := lookupOpaqueConsumer(packet.InterAsTEOpaqueType); !ok {
		t.Fatalf("Opaque type 6 (RFC 5392 inter-AS TE) not registered")
	}
	// The carrier rejects a duplicate Opaque Type registration (RFC 5250 sec 9).
	if err := registerOpaqueConsumer(packet.TEOpaqueType, OpaqueScopeArea, nil, nil); !errors.Is(err, ErrOpaqueTypeRegistered) {
		t.Fatalf("duplicate registration err = %v, want ErrOpaqueTypeRegistered", err)
	}
}

func TestTEReceiveMalformedNoEntry(t *testing.T) {
	eng, _ := newRedistEngine(t, teCfgJSON)
	adv := mustRouterID(t, "2.2.2.2")
	// AC-18: a truncated body never panics and adds no TED entry.
	eng.teOnReceive(teReceived(OpaqueScopeArea, packet.TEOpaqueType, 1, adv, []byte{0x00, 0x02, 0x00, 0xff}, true, false))
	if n := len(eng.ted.Snapshot().Links); n != 0 {
		t.Fatalf("malformed TE body stored %d links, want 0", n)
	}
	// A type-1 Link TLV missing the mandatory Link ID sub-TLV is skipped (RFC 3630 sec 2.4.2).
	noLinkID := packet.TELSA{IsLink: true, Link: packet.TELink{HasLinkType: true, LinkType: packet.TELinkTypePointToPoint}}.Encode()
	eng.teOnReceive(teReceived(OpaqueScopeArea, packet.TEOpaqueType, 2, adv, noLinkID, true, false))
	if n := len(eng.ted.Snapshot().Links); n != 0 {
		t.Fatalf("type-1 link without Link ID stored, want skipped")
	}
	// A type-6 Link TLV carrying the prohibited Link ID sub-TLV is skipped (RFC 5392 sec 3.2.1).
	// RFC requirement: RFC5392-3.2.1-1 negative -- reception rejects a type-6 inter-AS Link TLV
	// that carries the prohibited Link ID sub-TLV; it is parsed but produces no TED entry (§3.2.1).
	withLinkID := packet.TELSA{IsLink: true, Link: packet.TELink{HasLinkType: true, LinkType: packet.TELinkTypePointToPoint, HasLinkID: true, LinkID: [4]byte{2, 2, 2, 2}, HasRemoteAS: true, RemoteAS: 65001, HasRemoteASBRv4: true, RemoteASBRv4: [4]byte{203, 0, 113, 9}}}.Encode()
	eng.teOnReceive(teReceived(OpaqueScopeArea, packet.InterAsTEOpaqueType, 3, adv, withLinkID, true, false))
	if n := len(eng.ted.Snapshot().Links); n != 0 {
		t.Fatalf("type-6 link with prohibited Link ID stored, want skipped")
	}
}

func TestTEReceiveType6MissingRemoteASSkipped(t *testing.T) {
	// RFC 5392 sec 3.2.1/3.3.1: the Remote AS Number sub-TLV (21) is REQUIRED in a type-6
	// inter-AS Link TLV. A received type-6 Link TLV that lacks it is a spec violation: parsed,
	// counted, and skipped, leaving no TED entry (validateReceivedTELink).
	eng, _ := newRedistEngine(t, teCfgJSON)
	adv := mustRouterID(t, "2.2.2.2")
	// A well-formed type-6 Link TLV with a Link Type and an IPv4 remote ASBR ID but NO Remote AS
	// Number sub-TLV.
	noRemoteAS := packet.TELSA{IsLink: true, Link: packet.TELink{
		HasLinkType: true, LinkType: packet.TELinkTypePointToPoint,
		HasRemoteASBRv4: true, RemoteASBRv4: [4]byte{203, 0, 113, 9},
	}}.Encode()
	// RFC requirement: RFC5392-3.2.1-4 negative -- reception rejects a type-6 inter-AS Link TLV
	// that lacks the REQUIRED Remote AS Number sub-TLV; the guard produces no TED entry (§3.2.1).
	eng.teOnReceive(teReceived(OpaqueScopeArea, packet.InterAsTEOpaqueType, 7, adv, noRemoteAS, true, false))
	if n := len(eng.ted.Snapshot().Links); n != 0 {
		t.Fatalf("type-6 inter-AS link without the Remote AS sub-TLV stored, want skipped (RFC 5392 sec 3.2.1)")
	}
}

func TestTEReceiveIgnoresForeignRouterID(t *testing.T) {
	// A well-formed type-1 Router-Address plus Link from a peer installs cleanly; this
	// guards the happy path used by the other reception tests.
	eng, _ := newRedistEngine(t, teCfgJSON)
	adv := types.RouterID{5, 6, 7, 8}
	eng.teOnReceive(teReceived(OpaqueScopeArea, packet.TEOpaqueType, 1, adv, teLinkBody(t), true, false))
	if len(eng.ted.Snapshot().Links) != 1 {
		t.Fatalf("well-formed link not installed")
	}
}
