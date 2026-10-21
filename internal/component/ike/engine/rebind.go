// Design: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md -- NAT traversal, the rebind
// Related: mobike.go -- the per-child migration loop both paths share
// Related: established.go -- serviceChildRekeyTrigger raises the owed rekey
// RFC: rfc/short/rfc3948.md
// RFC: rfc/short/rfc7296.md

package engine

import (
	"errors"
	"fmt"
	"log/slog"
	"net"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
)

// Reasons logged when a moved Child SA is rekeyed rather than migrated. The interop
// checker compares the attribute, so each is a distinct, fixed text.
const (
	rebindReasonTransport = "transport mode"
	rebindReasonDataplane = "the dataplane cannot migrate Child SAs"
	rebindReasonPending   = "a new IKE SA with the peer is being set up"
	rebindReasonBareESP   = "the Child SA does not encapsulate ESP in UDP"
)

// rebindChildSAs moves the installed Child SAs after adoptAuthenticatedEndpoint has
// moved the IKE SA's peer endpoint: a NAT rebinding after establishment, with MOBIKE
// not negotiated and this node not behind a NAT. Until it runs, every installed state
// still encapsulates ESP to the old endpoint.
//
// RFC 7296 Section 2.23: an endpoint not behind a NAT that receives a validated packet
// from a changed address or port "SHOULD send all packets (including retransmission
// packets) to the IP address and port in the validated packet,".
// RFC 3948 Section 2.1: "the Source Port and Destination Port MUST be the same as that
// used by IKE traffic."
//
// Mode and backend are per IKE SA, so it decides once for the SA:
//
//   - A transport-mode Child SA whose peer changed ADDRESS cannot be migrated or
//     rekeyed onto it, because its selectors name the old address: the IKE SA closes,
//     and only the configured remote address sets it up again.
//   - The Child SAs can be migrated (childMigrationAvailable): the current pair and a
//     distinct superseded pair move in place, keeping their keys, replay window and
//     sequence (TunnelMigrator contract).
//   - They cannot (transport mode, a pair that does not encapsulate ESP in UDP, a
//     dataplane without a TunnelMigrator, or a pending child that a parallel IKE SA is
//     setting up): a Child SA rekey is owed. The rekeyed child takes the IKE SA's ports
//     (childEncapPorts) and, in tunnel mode, its peer address (takePeerTunnelAddress),
//     so it lands on the new endpoint.
//   - The migration failed with any error but ErrTunnelMigrationLost: the contract says
//     the old endpoints still stand, so the rekey is owed as above. Most such errors wrap
//     no sentinel, so the test is for the lost case only.
//   - The migration lost the tunnel: the keys MUST NOT be reinstalled, so the IKE SA
//     closes and is set up again, as a MOBIKE SA does on the same error.
//
// The caller holds childLifecycleMu, as it does for MOBIKE. Owner-loop state only.
func (ps *PeerSession) rebindChildSAs(sa *SA, dp dataplane.Dataplane, oldEndpoint *net.UDPAddr, log *slog.Logger) ownedOutcome {
	current, superseded := ps.getChildSA(), ps.supersededChild
	if current == nil && superseded == nil {
		return ownedOutcome{}
	}
	attrs := []any{"peer", sa.PeerName, "old-endpoint", oldEndpoint, "new-endpoint", sa.peerEndpoint}

	// RFC 7296 Section 2.23.1: "The TSi entries MUST have exactly one IP address, and
	// that MUST match the source address of the IKE SA." A transport-mode Child SA's
	// selectors name the old peer address, and a rekey keeps that scope as its floor
	// (RFC 7296 Section 2.9.2), so no rekey can follow a peer that changed ADDRESS.
	// The IKE SA closes rather than keep ESP flowing to an address the peer left. It
	// is set up again only toward, or from, the CONFIGURED remote address (the
	// initiator redials it, matchResponderPeer accepts only it), so a peer that keeps
	// the new address needs it configured. A port-only move keeps the address and
	// takes the owed rekey below. The mode is the installed pair's, not the
	// configured one: a peer that declined transport mode runs a tunnel-mode pair.
	anchor := current
	if anchor == nil {
		anchor = superseded
	}
	if anchor.Mode == modeTransport && !oldEndpoint.IP.Equal(sa.peerEndpoint.IP) {
		log.Info("ike: peer endpoint moved to a new address, a transport-mode Child SA cannot follow it, so the IKE SA is closed. If the peer keeps the new address, set it as the peer's remote address in the IPsec configuration",
			attrs...)
		sa.State = StateDead
		return ownedOutcome{reestablish: true}
	}

	if reason, ok := rebindRekeyReason(ps, sa, anchor); !ok {
		ps.rebindRekeyOwed = true
		log.Info("ike: peer endpoint moved, Child SA cannot be migrated here, so it is rekeyed onto the new address and port",
			append(attrs, "reason", reason)...)
		return ownedOutcome{}
	}

	moved, err := ps.migrateRebound(sa, dp, current, superseded)
	if err == nil {
		log.Info("ike: peer endpoint moved, Child SAs migrated to the new address and port",
			append(attrs, "child-sas", moved)...)
		return ownedOutcome{}
	}
	if errors.Is(err, dataplane.ErrTunnelMigrationLost) {
		log.Error("ike: peer endpoint moved, the Child SA was lost while migrating it, so the IKE SA is closed and set up again. Check the kernel IPsec state with: ze doctor",
			append(attrs, "error", err)...)
		sa.State = StateDead
		return ownedOutcome{reestablish: true}
	}
	ps.rebindRekeyOwed = true
	log.Info("ike: peer endpoint moved, migrating the Child SA failed and the old endpoints stay in place, so it is rekeyed onto the new address and port",
		append(attrs, "error", err)...)
	return ownedOutcome{}
}

// rebindRekeyReason answers whether the moved Child SAs can be migrated, and when they
// cannot, the reason a rekey replaces the migration. A pending child means a parallel
// IKE SA has already transferred the shared policy templates, so the retiring SA moves
// nothing (migrateMobikeChild refuses the same case for MOBIKE).
//
// A Child SA that is not UDP-encapsulated is rekeyed too. A migration carries the
// NAT-T socket's port and encapsulates the moved pair (migrateMobikePair), so it
// would turn a bare-ESP pair into an encapsulated one, and the kernel refuses its
// port pair outright when IKE runs on 500 (validateXFRMMigration). The rekeyed child
// keeps UDPEncap from the retired pair and takes the new address
// (takePeerTunnelAddress).
func rebindRekeyReason(ps *PeerSession, sa *SA, anchor *ChildSA) (string, bool) {
	if childTransportMode(sa) {
		return rebindReasonTransport, false
	}
	if !anchor.UDPEncap {
		return rebindReasonBareESP, false
	}
	if !childMigrationAvailable(sa) {
		return rebindReasonDataplane, false
	}
	if ps.getPendingChild() != nil {
		return rebindReasonPending, false
	}
	return "", true
}

// migrateRebound migrates the installed pairs to the new peer endpoint. The local side
// keeps its address and takes the port childEncapPorts names, the NAT-T socket's.
func (ps *PeerSession) migrateRebound(sa *SA, dp dataplane.Dataplane, current, superseded *ChildSA) (int, error) {
	localPort, _, err := childEncapPorts(sa)
	if err != nil {
		return 0, fmt.Errorf("ike: rebind has no encapsulation port: %w", err)
	}
	anchor := current
	if anchor == nil {
		anchor = superseded
	}
	local := &net.UDPAddr{IP: append(net.IP(nil), anchor.LocalAddr...), Port: int(localPort)}
	return ps.migrateMobikeChild(sa, dp, local)
}
