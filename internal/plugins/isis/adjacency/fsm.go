// Design: docs/architecture/isis/isis-5-adjacency.md -- adjacency finite state machine.
// ISO/IEC 10589 section 8.2 (Down/Initializing/Up), section 8.2.2 (L1 area
// match), section 8.2.3 (hold-timer timeout).
//
// RFC: rfc/short/rfc5303.md -- P2P three-way handshake (TLV 240) + legacy fall-back
// RFC: rfc/short/rfc1195.md -- TLV 1/129/132 carried in an IIH
//
// The FSM is pure: it takes a parsed HelloInput plus the local identity and the
// current time, mutates an Adjacency in place, and returns whether a session
// up/down transition occurred so the circuit (the only I/O owner) can emit the
// event and update metrics. It never touches the wire or the clock directly.

package adjacency

import (
	"net/netip"
	"slices"
	"time"

	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// CircuitKind is the medium of the circuit the FSM runs on. The LAN and P2P
// paths reach Up by different bidirectionality proofs (TLV 6 echo vs TLV 240).
type CircuitKind uint8

// Circuit kinds.
const (
	KindBroadcast CircuitKind = iota
	KindP2P
)

// DefaultMaxAreaAddresses is the protocol default Maximum Area Addresses used
// when the IIH common-header field is 0 (ISO/IEC 10589 clause 8.4.1: "the value
// zero ... is interpreted as 3"). It bounds how many area addresses a received
// TLV-1 may carry before the Hello is rejected.
const DefaultMaxAreaAddresses = 3

// Local is the immutable per-circuit local identity the FSM matches against a
// received Hello. It is supplied by the circuit and does not change for the life
// of the circuit (a config change rebuilds the circuit).
type Local struct {
	// SystemID is our own System ID (echoed by the neighbor's TLV 6 on a LAN, or
	// in the neighbor's TLV 240 on P2P, to prove bidirectionality).
	SystemID types.SystemID
	// SNPA is our own source MAC; the LAN three-way check looks for THIS in the
	// neighbor's TLV 6 IS-Neighbors list.
	SNPA SNPA
	// Areas are our configured area addresses; the L1 match requires an overlap
	// with the neighbor's advertised areas (ISO/IEC 10589 section 8.2.2).
	Areas []types.AreaID
	// Kind is the circuit medium.
	Kind CircuitKind
}

// HelloInput is the parsed, FSM-relevant content of one received IIH. The
// circuit decodes the PDU with the spec-isis-2 codec and the frame source MAC,
// fills this struct, and hands it to the FSM. The FSM never parses bytes.
type HelloInput struct {
	// SystemID is the sender's System ID (from the IIH fixed header).
	SystemID types.SystemID
	// SNPA is the sender's source MAC (from the 802.3 frame). Zero on P2P.
	SNPA SNPA
	// Level is the routing level this Hello forms an adjacency at (1 or 2).
	Level Level
	// HoldTime is the sender's advertised holding time in seconds (the IIH fixed
	// header holding-time field).
	HoldTime uint16
	// Priority is the sender's DIS election priority (0..127) from the LAN IIH
	// fixed header (ISO/IEC 10589 clause 8.4.5). It is meaningful only on a LAN
	// IIH; a P2P IIH carries no priority field, so the circuit leaves it 0.
	Priority uint8
	// MaxAreaAddresses is the sender's Maximum Area Addresses common-header field
	// (ISO/IEC 10589 clause 8.4.1). It bounds how many area addresses the sender's
	// TLV-1 may carry; the value 0 on the wire means the protocol default of 3
	// (DefaultMaxAreaAddresses). The FSM rejects a Hello whose TLV-1 area count
	// exceeds this effective maximum.
	MaxAreaAddresses uint8
	// Areas are the sender's area addresses (TLV 1). Empty on a P2P IIH that
	// omits them (an L2-only adjacency forms regardless).
	Areas []types.AreaID
	// NeighborSNPAs is the sender's TLV 6 IS-Neighbors list (LAN only). The LAN
	// three-way check looks for OUR SNPA in this list.
	NeighborSNPAs []SNPA
	// IPv4 is the sender's first TLV 132 IPv4 interface address (the SPF
	// next-hop source). Invalid when the IIH carried no TLV 132.
	IPv4 netip.Addr
	// IPv6 is the sender's first TLV 232 IPv6 interface address. Invalid when
	// absent.
	IPv6 netip.Addr
	// Protocols is the received TLV 129 set; absence means CLNP-only.
	Protocols Protocols

	// ThreeWay is the sender's TLV 240 (P2P three-way) state when present.
	// HasThreeWay reports whether the IIH carried a TLV 240 at all; its absence
	// selects the legacy implicit fall-back (RFC 5303 sec 3.2).
	HasThreeWay bool
	ThreeWay    packet.P2PThreeWayTLV
}

// Transition reports the effect of feeding a Hello to the FSM: the resulting
// state and whether a session up/down event must be emitted by the circuit.
type Transition struct {
	// State is the adjacency state after the event.
	State State
	// SessionUp is true exactly on a transition INTO Up (Down/Init -> Up).
	SessionUp bool
	// SessionDown is true exactly on a transition OUT of Up (Up -> Down/Init).
	SessionDown bool
	// ForwardingChanged reports a capability, address or SNPA change while Up.
	ForwardingChanged bool
	// Rejected is true when the Hello was rejected (e.g. L1 area mismatch) and no
	// state change happened; the circuit logs the reason.
	Rejected bool
	// RejectReason is a short stable reason token when Rejected is set.
	RejectReason string
}

// ReceiveHello applies a received Hello to adj for the given local identity and
// current time. It implements the ISO/IEC 10589 section 8.2 state machine:
//
//   - L1 (ISO/IEC 10589 section 8.2.2): the neighbor must share at least one
//     area address, else the Hello is rejected and no adjacency forms. L2 forms
//     regardless of area.
//   - A first accepted Hello moves Down -> Initializing.
//   - The adjacency reaches Up only once bidirectionality is proven:
//     LAN -- our SNPA is echoed in the neighbor's TLV 6 (IS Neighbors);
//     P2P -- the neighbor's TLV 240 reports Up/Initializing AND echoes our
//     System ID (RFC 5303 sec 3.2), OR the neighbor sent no TLV 240 and we fall
//     back to the implicit (two-way) adjacency for a legacy peer.
//
// On every accepted Hello the hold timer is (re)armed to now + HoldTime, the
// neighbor addresses (TLV 132/232) are stored as the SPF next-hop source, and
// the area list is refreshed. The function mutates adj in place and returns the
// resulting Transition.
func ReceiveHello(adj *Adjacency, local Local, in HelloInput, now time.Time) Transition {
	// RFC 5303 Section 3.2: "If the option is present and contains invalid
	// Adjacency Three-Way State, the PDU SHALL be discarded and no further
	// action is taken." Guard the raw HelloInput boundary before even the
	// neighbor identity or hold deadline can change.
	if local.Kind == KindP2P {
		if in.HasThreeWay {
			switch in.ThreeWay.State {
			case packet.AdjThreeWayDown, packet.AdjThreeWayInitializing, packet.AdjThreeWayUp:
				// Every named wire state has a column in the RFC table.
			default:
				// The received state is an open wire value.
				return Transition{State: adj.State, Rejected: true, RejectReason: "invalid-three-way-state"}
			}
		}
	}

	// ISO/IEC 10589 section 8.2: an IS forms adjacencies with OTHER intermediate
	// systems; it must never form an adjacency with itself. A Hello carrying our own System
	// ID is a looped-back or spoofed frame (e.g. a LAN hairpin or a duplicate
	// System ID), so reject it before any adjacency mutation -- accepting it would
	// fabricate a phantom self-neighbor that corrupts the LSDB and SPF.
	if in.SystemID == local.SystemID {
		return Transition{State: adj.State, Rejected: true, RejectReason: "own-system-id"}
	}
	// ISO/IEC 10589 clause 8.4.1: a received IIH whose TLV-1 carries more area
	// addresses than the sender's advertised Maximum Area Addresses is malformed
	// (the field 0 means the default of 3). Reject it before any mutation rather
	// than store an over-long area list that a misconfigured or hostile neighbor
	// could use to bloat the adjacency record.
	if maxAreas := effectiveMaxAreas(in.MaxAreaAddresses); len(in.Areas) > maxAreas {
		return Transition{State: adj.State, Rejected: true, RejectReason: "too-many-areas"}
	}

	// ISO/IEC 10589 section 8.2.2: "a Level 1 adjacency may be formed ... only if
	// at least one Area Address ... is common to both systems." L2 forms across
	// areas (RFC 1195); only L1 enforces the overlap, on both LAN and P2P.
	if in.Level == Level1 && !areasOverlap(local.Areas, in.Areas) {
		return Transition{State: adj.State, Rejected: true, RejectReason: "l1-area-mismatch"}
	}

	prev := adj.State
	forwardingChanged := adj.Protocols != in.Protocols ||
		adj.IPv4 != in.IPv4 || adj.IPv6 != in.IPv6 || adj.SNPA != in.SNPA

	// Record neighbor identity and the SPF next-hop addresses on every Hello.
	adj.SystemID = in.SystemID
	adj.ISHOnly = false
	adj.SNPA = in.SNPA
	adj.Level = in.Level
	adj.Areas = in.Areas
	adj.IPv4 = in.IPv4
	adj.IPv6 = in.IPv6
	adj.Protocols = in.Protocols
	adj.HoldTime = in.HoldTime
	// ISO/IEC 10589 clause 8.4.5: record the neighbor's advertised DIS priority so
	// the broadcast circuit's election (isis-8) can compare it. On P2P the field is
	// 0 (no DIS), which is harmless because P2P circuits never run the election.
	adj.Priority = in.Priority
	adj.LastSeen = now
	// ISO/IEC 10589 section 8.2.3: the adjacency holding time governs the timeout;
	// arm the hold timer to now + the neighbor's advertised holding time.
	adj.HoldExpiry = now.Add(time.Duration(in.HoldTime) * time.Second)
	adj.deleteAt = time.Time{} // a fresh Hello cancels any pending grace deletion

	// Track the RFC 5303 three-way state for the P2P decision.
	if local.Kind == KindP2P {
		updateThreeWay(adj, local, in)
	}

	// Decide the new state.
	switch {
	case local.Kind == KindP2P && adj.sawTLV240:
		applyThreeWayAction(adj, threeWayTableAction(adj), now)
	case bidirectional(adj, local, in):
		adj.State = StateUp
	default:
		// We have heard a Hello but cannot yet prove the neighbor heard us.
		adj.State = StateInitializing
	}

	tr := classify(prev, adj.State)
	if prev == StateUp {
		if adj.State == StateUp {
			tr.ForwardingChanged = forwardingChanged
		}
	}
	return tr
}

// bidirectional reports whether the neighbor has proven it can hear us, which is
// the condition for the Up state.
//
//   - LAN: our SNPA appears in the neighbor's TLV 6 IS-Neighbors list.
//   - P2P legacy (no TLV 240 ever seen): the implicit two-way adjacency forms on
//     the first Hello (a single Hello in each direction is enough).
//
// A P2P adjacency that has seen a TLV 240 never reaches here: the RFC 5303
// sec 3.2 state table (threeWayTableAction) decides it.
func bidirectional(adj *Adjacency, local Local, in HelloInput) bool {
	switch local.Kind {
	case KindBroadcast:
		return slices.Contains(in.NeighborSNPAs, local.SNPA)
	case KindP2P:
		// RFC 5303 Section 3.2: "If it does not, the link is assumed to be
		// functional in both directions, and the procedures described in
		// section 8.2.4.2 are followed."
		return !adj.sawTLV240
	default:
		// Local.Kind is caller-supplied; unknown media prove no bidirectionality.
		return false
	}
}

// threeWayAction is one action of the RFC 5303 sec 3.2 Adjacency Three-Way
// State Table. The zero value is not an action: it is what the table answers
// for a received state it has no column for.
type threeWayAction uint8

// Three-way state table actions.
const (
	threeWayActionUnspecified threeWayAction = iota
	threeWayInitialize
	threeWayUp
	threeWayDown
	threeWayAccept
)

// threeWayTableAction looks up the RFC 5303 sec 3.2 state table for adj: the
// row is our current three-way state, which is the adjacency state on a P2P
// circuit, and the column the neighbor's reported state from its last TLV 240.
//
// RFC 5303 Section 3.2: "In section 8.2.4.2 a and b, the action "Up" from state
// tables 5, 6, 7, and 8 may create a new adjacency but the three-way state of
// the adjacency SHALL be Down." A new adjacency is created in StateDown, so its
// first Hello reads the Down row.
//
// RFC 5303 Section 3.2: "If the action taken from section 8.2.4.2 a or b is
// "Up" or "Accept", the IS SHALL perform the action indicated by the new
// adjacency three-way state table below, based on the current adjacency
// three-way state and the received Adjacency Three-Way State value from the
// option."
//
//	                      Received Adjacency Three-Way State
//	                         Down       Initializing    Up
//	                    --------------------------------------
//	Down         |  Initialize        Up         Down
//	Initializing |  Initialize        Up         Up
//	Up           |  Initialize        Accept     Accept
func threeWayTableAction(adj *Adjacency) threeWayAction {
	// A Neighbor System ID that is not ours proves the neighbor heard someone
	// else. RFC 5303 sec 3.2 discards such a PDU (RFC5303-3.2-9, a gap); until
	// that discard exists the Hello proves nothing, so it takes the Initialize
	// action it took before the table existed rather than any Up.
	if adj.neighborNamesOther {
		return threeWayInitialize
	}
	switch adj.reportedState {
	case packet.AdjThreeWayDown:
		return threeWayInitialize
	case packet.AdjThreeWayInitializing:
		if adj.State == StateUp {
			return threeWayAccept
		}
		return threeWayUp
	case packet.AdjThreeWayUp:
		return threeWayOnReceivedUp(adj.State)
	default:
		// The reported state is an open wire value. ReceiveHello rejects
		// invalid input before mutation; direct table callers get no action.
		return threeWayActionUnspecified
	}
}

// threeWayOnReceivedUp is the "Up" column of the RFC 5303 sec 3.2 table.
func threeWayOnReceivedUp(current State) threeWayAction {
	switch current {
	case StateDown:
		return threeWayDown
	case StateInitializing:
		return threeWayUp
	case StateUp:
		return threeWayAccept
	default:
		// Adjacency.State is caller-owned and can contain an unknown state.
		return threeWayActionUnspecified
	}
}

// applyThreeWayAction performs one RFC 5303 sec 3.2 table action on adj.
func applyThreeWayAction(adj *Adjacency, action threeWayAction, now time.Time) {
	switch action {
	case threeWayDown:
		// RFC 5303 Section 3.2: "If the new action is "Down", an
		// adjacencyStateChange(Down) event is generated with the reason
		// "Neighbor restarted" and the adjacency SHALL be deleted." The
		// deletion is the Reap at the next sweep; the event is RFC5303-3.2-12,
		// a gap. The row is Down, so no session was Up and no SessionDown fires.
		adj.State = StateDown
		adj.deleteAt = now
	case threeWayUp:
		// RFC 5303 Section 3.2: "If the new action is "Up", an
		// adjacencyStateChange(Up) event is generated." classify reports it
		// as SessionUp.
		adj.State = StateUp
	case threeWayAccept:
		// Accept is reached only from the Up row: the adjacency stays Up.
		adj.State = StateUp
	case threeWayInitialize:
		// RFC 5303 Section 3.2: "If the new action is "Initialize", no event is
		// generated and the adjacency three-way state SHALL be set to
		// "Initializing"."
		// On the Up row classify still reports SessionDown. That is not an
		// adjacencyStateChange event: it is Ze's internal notice that the
		// adjacency left Up, which the LSP owner needs. rfc/full/iso-iec-10589.txt
		// Section 9.4 lists among the events that "require regeneration": "an
		// adjacency or circuit enters or leaves the Up state", and Section 7.5:
		// "An Intermediate System that considers a link down MUST omit that
		// link from its LSPs." The RFC's "no event is generated" is about the
		// adjacencyStateChange event, not about withdrawing a non-Up adjacency.
		adj.State = StateInitializing
	case threeWayActionUnspecified:
		// An unknown input has no table action and cannot initialize a peer.
		return
	default:
		panic("BUG: invalid three-way adjacency action")
	}
}

// updateThreeWay folds the neighbor's TLV 240 into the adjacency's reported
// state (RFC 5303 sec 3.1/3.2). A neighbor that has never sent a TLV 240 keeps
// sawTLV240 false so the legacy fall-back applies; once it sends one the
// sec 3.2 state table decides every later Hello. neighborNamesOther is set
// when the neighbor's TLV 240 carries a Neighbor System ID that is not ours.
func updateThreeWay(adj *Adjacency, local Local, in HelloInput) {
	if !in.HasThreeWay {
		return
	}
	adj.sawTLV240 = true
	adj.reportedState = in.ThreeWay.State
	adj.neighborNamesOther = in.ThreeWay.HasNeighbor && in.ThreeWay.NeighborID != local.SystemID
}

// classify maps a (previous, next) state pair to the session event flags.
func classify(prev, next State) Transition {
	t := Transition{State: next}
	switch {
	case prev != StateUp && next == StateUp:
		t.SessionUp = true
	case prev == StateUp && next != StateUp:
		t.SessionDown = true
	}
	return t
}

// effectiveMaxAreas resolves the advertised Maximum Area Addresses to its
// effective value: the wire value 0 means the protocol default of 3 (ISO/IEC
// 10589 clause 8.4.1). Any non-zero value is taken as-is.
func effectiveMaxAreas(maxAreaAddresses uint8) int {
	if maxAreaAddresses == 0 {
		return DefaultMaxAreaAddresses
	}
	return int(maxAreaAddresses)
}

// areasOverlap reports whether the two area-address lists share at least one
// area (ISO/IEC 10589 section 8.2.2 L1 match).
func areasOverlap(a, b []types.AreaID) bool {
	for _, x := range a {
		if slices.ContainsFunc(b, x.Equal) {
			return true
		}
	}
	return false
}

// Expire transitions adj to Down if the hold timer has elapsed (no Hello within
// the neighbor's advertised holding time, ISO/IEC 10589 section 8.2.3). It
// returns a Transition with SessionDown set when an Up adjacency drops, and
// arms the grace-period deletion. A non-expired adjacency is left unchanged and
// returns its current state with no flags.
func Expire(adj *Adjacency, now time.Time, grace time.Duration) Transition {
	if adj.State == StateDown {
		return Transition{State: StateDown}
	}
	if !adj.HoldExpiry.IsZero() && now.Before(adj.HoldExpiry) {
		return Transition{State: adj.State}
	}
	return drop(adj, now, grace)
}

// Down forces the adjacency Down regardless of the hold timer (used on a
// circuit-down event). It emits SessionDown when an Up adjacency drops and arms
// the grace-period deletion.
func Down(adj *Adjacency, now time.Time, grace time.Duration) Transition {
	if adj.State == StateDown {
		return Transition{State: StateDown}
	}
	return drop(adj, now, grace)
}

// drop moves adj to Down, arms the grace-period deletion, and classifies the
// session event.
func drop(adj *Adjacency, now time.Time, grace time.Duration) Transition {
	prev := adj.State
	adj.State = StateDown
	adj.deleteAt = now.Add(grace)
	return classify(prev, StateDown)
}
