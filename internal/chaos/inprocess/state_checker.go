// Design: docs/architecture/chaos-web-dashboard.md -- online reactor invariant checker (StateChecker)

package inprocess

import (
	"hash/fnv"
	"slices"

	bgpreactor "github.com/ze-software/ze/internal/component/bgp/reactor"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	fsmIdle        = "IDLE"
	fsmConnect     = "CONNECT"
	fsmActive      = "ACTIVE"
	fsmOpenSent    = "OPENSENT"
	fsmOpenConfirm = "OPENCONFIRM"
	fsmEstablished = "ESTABLISHED"
)

// legalFSMTransitions enumerates every valid (From -> To) BGP FSM state edge
// permitted by RFC 4271 Section 8.2.2. State strings match fsm.State.String().
var legalFSMTransitions = map[string]map[string]bool{
	fsmIdle: {
		fsmIdle:    true,
		fsmConnect: true,
		fsmActive:  true,
	},
	fsmConnect: {
		fsmIdle:     true,
		fsmConnect:  true,
		fsmActive:   true,
		fsmOpenSent: true,
	},
	fsmActive: {
		fsmIdle:     true,
		fsmConnect:  true,
		fsmActive:   true,
		fsmOpenSent: true,
	},
	fsmOpenSent: {
		fsmIdle:        true,
		fsmActive:      true,
		fsmOpenConfirm: true,
	},
	fsmOpenConfirm: {
		fsmIdle:        true,
		fsmOpenConfirm: true,
		fsmEstablished: true,
	},
	fsmEstablished: {
		fsmIdle:        true,
		fsmEstablished: true,
	},
}

// IsLegalFSMTransition reports whether moving from state `from` to state `to`
// is allowed by RFC 4271 Section 8.2.2.
func IsLegalFSMTransition(from, to string) bool {
	next, ok := legalFSMTransitions[from]
	if !ok {
		return false
	}
	return next[to]
}

type seenTransitionKey struct {
	timestamp int64
	from      string
	to        string
	reason    string
}

// StateChecker performs TigerBeetle VOPR-style online invariant checks directly
// against the live in-process reactor on every simulation tick and at settle:
//   - RFC 4271 Section 8.2.2 FSM transition legality across every peer's history
//   - RFC 4271 Section 4.2 non-zero remote BGP Identifier while Established
//   - Deterministic FNV-64a checkpoint state hashing across sorted peers
type StateChecker struct {
	seenByPeer      map[string]map[seenTransitionKey]struct{}
	reported        map[string]struct{}
	violations      []string
	transitionCount int
	lastHash        uint64
}

// NewStateChecker constructs an online reactor invariant checker.
func NewStateChecker() *StateChecker {
	return &StateChecker{
		seenByPeer: make(map[string]map[seenTransitionKey]struct{}),
		reported:   make(map[string]struct{}),
	}
}

// Sample inspects all peers in r, validates newly observed FSM transitions and
// established-session invariants, and updates the checkpoint state hash.
func (sc *StateChecker) Sample(r *bgpreactor.Reactor) uint64 {
	if r == nil {
		return 0
	}
	peers := r.Peers()
	slices.SortFunc(peers, func(a, b *bgpreactor.Peer) int {
		return a.Settings().Address.Compare(b.Settings().Address)
	})

	h := fnv.New64a()
	var tb textbuf.Buffer

	for _, p := range peers {
		addr := p.Settings().Address.String()
		state := p.State()

		// Check all transitions in chronological order (FSMHistory is newest-first).
		history := p.FSMHistory()
		seen := sc.seenByPeer[addr]
		if seen == nil {
			seen = make(map[seenTransitionKey]struct{})
			sc.seenByPeer[addr] = seen
		}
		for _, tr := range slices.Backward(history) {
			key := seenTransitionKey{
				timestamp: tr.Timestamp.UnixNano(),
				from:      tr.From,
				to:        tr.To,
				reason:    tr.Reason,
			}
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			sc.transitionCount++

			if !IsLegalFSMTransition(tr.From, tr.To) {
				msg := tb.Reset().
					Str("fsm-transition: peer ").Str(addr).
					Str(" illegal RFC 4271 transition ").Str(tr.From).
					Str(" -> ").Str(tr.To).
					Str(" (").Str(tr.Reason).Byte(')').String()
				sc.recordViolation(msg)
			}
		}

		// RFC 4271 Section 4.2: an Established peer must hold a non-zero BGP Identifier.
		if state == bgpreactor.PeerStateEstablished && p.RemoteRouterID() == 0 {
			msg := tb.Reset().
				Str("remote-router-id: peer ").Str(addr).
				Str(" is Established with zero RemoteRouterID").String()
			sc.recordViolation(msg)
		}

		// Feed deterministic peer snapshot into checkpoint hash.
		tb.Reset().Str(addr).Byte('=').Str(state.String()).Byte('|')
		_, _ = h.Write(tb.Bytes())
	}

	sc.lastHash = h.Sum64()
	return sc.lastHash
}

func (sc *StateChecker) recordViolation(msg string) {
	if _, dup := sc.reported[msg]; dup {
		return
	}
	sc.reported[msg] = struct{}{}
	sc.violations = append(sc.violations, msg)
}

// Violations returns a copy of all invariant violations observed so far.
func (sc *StateChecker) Violations() []string {
	if len(sc.violations) == 0 {
		return nil
	}
	out := make([]string, len(sc.violations))
	copy(out, sc.violations)
	return out
}

// TransitionCount returns the total number of distinct FSM transitions validated.
func (sc *StateChecker) TransitionCount() int {
	return sc.transitionCount
}

// LastHash returns the most recent checkpoint state hash.
func (sc *StateChecker) LastHash() uint64 {
	return sc.lastHash
}

// ValidateFSMTransitions checks an arbitrary transition slice against RFC 4271
// Section 8.2.2 and returns human-readable violation strings (used by tests and
// offline checkers).
func ValidateFSMTransitions(peerLabel string, transitions []bgpreactor.FSMTransition) []string {
	var violations []string
	var tb textbuf.Buffer
	for _, tr := range transitions {
		if !IsLegalFSMTransition(tr.From, tr.To) {
			msg := tb.Reset().
				Str("fsm-transition: peer ").Str(peerLabel).
				Str(" illegal RFC 4271 transition ").Str(tr.From).
				Str(" -> ").Str(tr.To).
				Str(" (").Str(tr.Reason).Byte(')').String()
			violations = append(violations, msg)
		}
	}
	return violations
}
