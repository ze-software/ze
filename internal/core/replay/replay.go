// Design: DESIGN-REVIEW.md finding 2 (late-join replay) and section 5
//
//	(protocol-agnostic core carrying protocol-specific shape)
//
// Package replay holds the ONE vocabulary every late-join replay hop shares: a
// value-typed request payload carrying an opaque correlation token, a reserved
// broadcast sentinel, and the token-derived "is this a replay" predicate.
//
// Three hops speak it, each keeping its own distinct (namespace, eventType)
// typed handle but binding it to *replay.Request:
//
//   - bgp-rib/replay-request     sysrib asks the BGP RIB to replay its best-path table
//   - system-rib/replay-request  a FIB backend asks sysrib to replay the system-best table
//   - redistribute/replay-request the redistribute orchestrator asks producers to re-emit
//
// The first two are BROADCAST: the token is Broadcast and the handler ignores
// it, walking its whole table for every subscriber. The third is TARGETED: the
// orchestrator allocates a fresh per-peer token, records token -> peer, and maps
// the returning batch back to the one new peer. Broadcast is simply the case
// where the token addresses everyone; a payload-carrying request absorbs both,
// which a payload-less signal cannot (it could not carry the per-peer token).
//
// This package is a true leaf: it imports only the standard library and must
// never spell a BGP, peer, or any protocol concept. The token is opaque; only
// the orchestrator that minted a targeted token knows what it maps to.
package replay

import "math"

// Request is the payload of every replay-request event. It carries an opaque
// correlation token. json tag "replay-id" is the external wire contract that
// forked plugin producers decode; it must not move.
type Request struct {
	// ReplayID is the opaque correlation token.
	//
	//	0          -> never emitted as a request; on a change batch it means a
	//	              normal incremental change (see IsReplay).
	//	Broadcast  -> addresses every consumer (the full-table replay case).
	//	any other  -> a per-target token the emitter maps back to one consumer.
	ReplayID uint64 `json:"replay-id"`
}

// Broadcast is the reserved token that addresses every consumer. It is the
// token the two broadcast hops (bgp-rib, system-rib) put on their request and
// echo onto the replay batch so IsReplay() reports true.
//
// It is the top of the uint64 range so it is disjoint from both 0 (incremental)
// and any per-target token: the redistribute orchestrator allocates targeted
// tokens from a monotonic counter starting at 1, which cannot reach MaxUint64
// in any realistic run. Keeping 0 as "not a replay" and Broadcast as a distinct
// reserved value means incremental, broadcast, and targeted are three disjoint
// cases (spec-unify-replay A-3/R-2).
const Broadcast uint64 = math.MaxUint64

// IsReplay reports whether a token denotes a replay (any nonzero token) rather
// than a normal incremental change (token 0). It is the single source of truth
// for the replay marker: every change-batch type derives its marker from this
// rather than storing an independent boolean.
func IsReplay(token uint64) bool { return token != 0 }
