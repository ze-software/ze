// Design: docs/exabgp/exabgp-migration.md -- ExaBGP 6.0 receive policy
// Related: migrate.go -- migrateSingleNeighbor calls applyReceivePolicy
// RFC: rfc/short/rfc9774.md
// RFC: rfc/short/rfc9012.md

package migration

import (
	"fmt"

	"github.com/ze-software/ze/internal/component/config"
)

// The ExaBGP receive-policy values, each spelled once. exabgp.yang declares the
// same words as the enumerations of the neighbor leaves `as-set` and
// `tunnel-encapsulation`, and TestReceivePolicyValuesMatchTheExaBGPModel holds
// the two in agreement: the parser refuses a word the model does not declare,
// so a constant spelled differently would never match.
const (
	exabgpASSetWithdraw     = "withdraw"
	exabgpASSetAccept       = "accept"
	exabgpTunnelEncapAuto   = "auto"
	exabgpTunnelEncapFilter = "filter"
	exabgpTunnelEncapAccept = "accept"
)

// zeASSetAccept is the ze peer leaf value `session { as-set accept; }` the
// migration writes. ze-bgp-conf.yang declares it, and
// TestMigrateASSetAcceptKeepsTheSet parses the migrated config against that
// model.
const zeASSetAccept = "accept"

// applyReceivePolicy judges the two receive-policy leaves ExaBGP 6.0 added to
// a neighbor (f9bfb853b, configuration/grammar/tree/neighbor.py) and writes
// what each needs on the migrated ze peer:
//
//	as-set               withdraw | accept         (RFC 9774)
//	tunnel-encapsulation auto | filter | accept    (RFC 9012 Section 11)
//
// A value is accepted where ze gives the received routes the same treatment,
// with ze's own leaf when one is needed. Any other value stops the migration:
// a peer that withdraws routes ExaBGP kept, or passes on an attribute ExaBGP
// dropped, is not a migration.
func applyReceivePolicy(neighbor, peer *config.Tree) error {
	if err := applyASSetPolicy(neighbor, peer); err != nil {
		return err
	}
	return checkTunnelEncapsulationPolicy(neighbor)
}

// applyASSetPolicy writes nothing for `as-set withdraw`, ExaBGP's default,
// because it is ze's default too (message/rfc7606.go validateASPathAttr,
// message/rfc7607.go validateAS4PathAttr).
//
// `as-set accept` keeps such a route in ExaBGP (bgp/message/update/
// collection.py, UpdateCollection._withdrawn_in_context), and becomes the ze
// peer leaf `session { as-set accept; }`, the same operator leaf any ze
// config may set. RFC 9774 Section 3: "Unless explicitly configured by a
// network operator to do otherwise (e.g., during a transition phase), BGP
// speakers:" ... "MUST use the "treat-as-withdraw" error handling behavior per
// [RFC7606] upon reception of BGP UPDATE messages containing AS_SETs or
// AS_CONFED_SETs in the AS_PATH or AS4_PATH [RFC6793]." The ExaBGP line is that
// operator configuration.
func applyASSetPolicy(neighbor, peer *config.Tree) error {
	value, ok := neighbor.Get("as-set")
	if !ok {
		return nil
	}
	if value == exabgpASSetWithdraw {
		return nil
	}
	if value == exabgpASSetAccept {
		peerSession(peer).Set("as-set", zeASSetAccept)
		return nil
	}
	return fmt.Errorf("as-set %s: this neighbor cannot be migrated\n"+
		"why: ExaBGP knows withdraw and accept\n"+
		"fix: use as-set withdraw or as-set accept", value)
}

// peerSession answers the peer's session container, creating it when the
// migration wrote none.
func peerSession(peer *config.Tree) *config.Tree {
	session := peer.GetContainer("session")
	if session == nil {
		session = config.NewTree()
		peer.SetContainer("session", session)
	}
	return session
}

// checkTunnelEncapsulationPolicy accepts `accept` and `auto`. Ze keeps a
// received Tunnel Encapsulation attribute on every session, which is what
// `accept` asks for, and what `auto` asks for inside the AS. On an external
// neighbor ExaBGP's `auto` drops it; ze does not yet, with the leaf or
// without it, which is ze's RFC 9012 Section 11 gap (rfc/short/rfc9012.md,
// RFC9012-11-5), not a property of the migrated line.
//
// `filter` drops the attribute from that neighbor on every session type, and
// ze has no attribute 23 filter to write, so it is refused.
func checkTunnelEncapsulationPolicy(neighbor *config.Tree) error {
	value, ok := neighbor.Get("tunnel-encapsulation")
	if !ok {
		return nil
	}
	if value == exabgpTunnelEncapAccept {
		return nil
	}
	if value == exabgpTunnelEncapAuto {
		return nil
	}
	if value == exabgpTunnelEncapFilter {
		return fmt.Errorf("tunnel-encapsulation filter: this neighbor cannot be migrated\n" +
			"why: ze cannot yet drop a received Tunnel Encapsulation attribute " +
			"(RFC 9012 Section 11) from one peer\n" +
			"fix: remove the tunnel-encapsulation line to migrate a peer that keeps the " +
			"attribute, as tunnel-encapsulation accept does")
	}
	return fmt.Errorf("tunnel-encapsulation %s: this neighbor cannot be migrated\n"+
		"why: ExaBGP knows auto, filter and accept\n"+
		"fix: use tunnel-encapsulation auto", value)
}
