// Design: docs/exabgp/exabgp-migration.md -- ExaBGP 6.0 receive policy
// Related: migrate.go -- migrateSingleNeighbor calls checkReceivePolicy
// RFC: rfc/short/rfc9774.md
// RFC: rfc/short/rfc9012.md

package migration

import (
	"fmt"

	"github.com/ze-software/ze/internal/component/config"
)

// checkReceivePolicy judges the two receive-policy leaves ExaBGP 6.0 added to
// a neighbor (f9bfb853b, configuration/grammar/tree/neighbor.py):
//
//	as-set               withdraw | accept         (RFC 9774)
//	tunnel-encapsulation auto | filter | accept    (RFC 9012 Section 11)
//
// Neither has a ze leaf, so a value is accepted only where ze already gives
// the received routes the same treatment, and nothing is written for it. Any
// other value stops the migration: a peer that withdraws routes ExaBGP kept,
// or passes on an attribute ExaBGP dropped, is not a migration.
func checkReceivePolicy(neighbor *config.Tree) error {
	if err := checkASSetPolicy(neighbor); err != nil {
		return err
	}
	return checkTunnelEncapsulationPolicy(neighbor)
}

// checkASSetPolicy accepts `as-set withdraw`, ExaBGP's default, because ze
// applies the same verdict to every peer (message/rfc7606.go validateASPath,
// message/rfc7607.go validateAS4PathAttr).
//
// RFC 9774 Section 3: "MUST use the "treat-as-withdraw" error handling behavior per
// [RFC7606] upon reception of BGP UPDATE messages containing AS_SETs
// or AS_CONFED_SETs in the AS_PATH or AS4_PATH [RFC6793]."
//
// `as-set accept` keeps such a route in ExaBGP (bgp/message/update/
// collection.py, UpdateCollection._withdrawn_in_context). Ze has no peer leaf
// that does so yet, so it is refused.
func checkASSetPolicy(neighbor *config.Tree) error {
	value, ok := neighbor.Get("as-set")
	if !ok {
		return nil
	}
	if value == "withdraw" {
		return nil
	}
	if value == "accept" {
		return fmt.Errorf("as-set accept: ze withdraws every received route whose AS_PATH "+
			"or AS4_PATH holds an AS_SET (RFC 9774 Section 3) and has no setting to keep it; "+
			"remove the line to migrate with ze's behavior, which is ExaBGP's default %q", "as-set withdraw")
	}
	return fmt.Errorf("as-set %s: ExaBGP knows withdraw and accept; use as-set withdraw", value)
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
	if value == "accept" {
		return nil
	}
	if value == "auto" {
		return nil
	}
	if value == "filter" {
		return fmt.Errorf("tunnel-encapsulation filter: ze cannot yet drop a received " +
			"Tunnel Encapsulation attribute (RFC 9012 Section 11) from one peer; remove the " +
			"line to migrate a peer that keeps it, as tunnel-encapsulation accept does")
	}
	return fmt.Errorf("tunnel-encapsulation %s: ExaBGP knows auto, filter and accept; "+
		"use tunnel-encapsulation auto", value)
}
