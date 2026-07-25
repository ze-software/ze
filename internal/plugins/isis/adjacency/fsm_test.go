// Design: plan/spec-isis-5-adjacency.md -- adjacency FSM transition tests.
//
// VALIDATES: the ISO/IEC 10589 section 8.2 state machine -- Down/Init/Up
// transitions, the LAN three-way check (our SNPA echoed in the neighbor's TLV
// 6), the RFC 5303 P2P three-way handshake plus the legacy fall-back, the L1
// area-address match (section 8.2.2), and the hold-timer timeout (section
// 8.2.3). Also that the received TLV 132/232 address is stored as the SPF
// next-hop.
// PREVENTS: an adjacency reaching Up without bidirectional proof, an L1
// adjacency forming across mismatched areas, or a stale adjacency never timing
// out.

package adjacency

import (
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/types"
)

var (
	t0       = time.Unix(1_000_000, 0)
	localSys = types.SystemID{0, 0, 0, 0, 0, 1}
	peerSys  = types.SystemID{0, 0, 0, 0, 0, 2}
	localMAC = SNPA{0x02, 0, 0, 0, 0, 1}
	peerMAC  = SNPA{0x02, 0, 0, 0, 0, 2}
)

func mustArea(t *testing.T, hex ...byte) types.AreaID {
	t.Helper()
	a, err := types.AreaIDFromBytes(hex)
	if err != nil {
		t.Fatalf("AreaIDFromBytes(%x): %v", hex, err)
	}
	return a
}

func lanLocal(t *testing.T) Local {
	t.Helper()
	return Local{
		SystemID: localSys,
		SNPA:     localMAC,
		Areas:    []types.AreaID{mustArea(t, 0x49, 0x00, 0x01)},
		Kind:     KindBroadcast,
	}
}

func p2pLocal(t *testing.T) Local {
	t.Helper()
	l := lanLocal(t)
	l.Kind = KindP2P
	l.SNPA = SNPA{}
	return l
}

// TestISISAdjFSMDownToInit: a first Hello from a new neighbor that does NOT echo
// our SNPA moves the adjacency Down -> Initializing (not Up).
func TestISISAdjFSMDownToInit(t *testing.T) {
	adj := &Adjacency{State: StateDown}
	in := HelloInput{
		SystemID:      peerSys,
		SNPA:          peerMAC,
		Level:         Level1,
		HoldTime:      30,
		Areas:         []types.AreaID{mustArea(t, 0x49, 0x00, 0x01)},
		NeighborSNPAs: nil, // our SNPA NOT echoed yet
	}
	tr := ReceiveHello(adj, lanLocal(t), in, t0)
	if tr.State != StateInitializing {
		t.Fatalf("state = %v, want initializing", tr.State)
	}
	if tr.SessionUp || tr.SessionDown {
		t.Errorf("unexpected session event on Down->Init: %+v", tr)
	}
	if adj.SystemID != peerSys {
		t.Errorf("neighbor SystemID not recorded")
	}
}

// TestISISAdjFSMInitToUp: once the neighbor echoes our SNPA (LAN three-way), the
// adjacency reaches Up and a session-up event fires.
func TestISISAdjFSMInitToUp(t *testing.T) {
	adj := &Adjacency{State: StateInitializing, SystemID: peerSys}
	in := HelloInput{
		SystemID:      peerSys,
		SNPA:          peerMAC,
		Level:         Level1,
		HoldTime:      30,
		Areas:         []types.AreaID{mustArea(t, 0x49, 0x00, 0x01)},
		NeighborSNPAs: []SNPA{localMAC}, // our SNPA echoed
	}
	tr := ReceiveHello(adj, lanLocal(t), in, t0)
	if tr.State != StateUp {
		t.Fatalf("state = %v, want up", tr.State)
	}
	if !tr.SessionUp {
		t.Errorf("expected SessionUp on Init->Up")
	}
}

// TestISISLANThreeWay: the LAN adjacency reaches Up ONLY when our SNPA appears in
// the neighbor's TLV 6; a TLV 6 without our SNPA keeps it Initializing.
func TestISISLANThreeWay(t *testing.T) {
	local := lanLocal(t)
	area := []types.AreaID{mustArea(t, 0x49, 0x00, 0x01)}

	// TLV 6 lists a different SNPA: not us -> stay Initializing.
	adj := &Adjacency{State: StateDown}
	other := SNPA{0x02, 0, 0, 0, 0, 9}
	tr := ReceiveHello(adj, local, HelloInput{
		SystemID: peerSys, SNPA: peerMAC, Level: Level1, HoldTime: 30,
		Areas: area, NeighborSNPAs: []SNPA{other},
	}, t0)
	if tr.State != StateInitializing {
		t.Fatalf("with our SNPA absent, state = %v, want initializing", tr.State)
	}

	// Now our SNPA appears -> Up.
	tr = ReceiveHello(adj, local, HelloInput{
		SystemID: peerSys, SNPA: peerMAC, Level: Level1, HoldTime: 30,
		Areas: area, NeighborSNPAs: []SNPA{other, localMAC},
	}, t0)
	if tr.State != StateUp || !tr.SessionUp {
		t.Fatalf("with our SNPA echoed, state = %v sessionUp = %v, want up/true", tr.State, tr.SessionUp)
	}
}

// TestISISAdjacencyNextHopStored: the received TLV 132 (IPv4) and TLV 232 (IPv6)
// interface addresses are stored on the adjacency as the SPF next-hop source.
func TestISISAdjacencyNextHopStored(t *testing.T) {
	adj := &Adjacency{State: StateDown}
	v4 := netip.MustParseAddr("192.0.2.2")
	v6 := netip.MustParseAddr("fe80::2")
	in := HelloInput{
		SystemID: peerSys, SNPA: peerMAC, Level: Level1, HoldTime: 30,
		Areas: []types.AreaID{mustArea(t, 0x49, 0x00, 0x01)},
		IPv4:  v4, IPv6: v6, NeighborSNPAs: []SNPA{localMAC},
	}
	ReceiveHello(adj, lanLocal(t), in, t0)
	if adj.IPv4 != v4 {
		t.Errorf("IPv4 next-hop = %v, want %v", adj.IPv4, v4)
	}
	if adj.IPv6 != v6 {
		t.Errorf("IPv6 next-hop = %v, want %v", adj.IPv6, v6)
	}
}

// TestISISAdjFSMUpToDownOnTimeout: an Up adjacency whose hold timer has elapsed
// transitions to Down and emits session-down.
func TestISISAdjFSMUpToDownOnTimeout(t *testing.T) {
	adj := &Adjacency{State: StateUp, SystemID: peerSys, HoldExpiry: t0.Add(30 * time.Second)}
	// Before expiry: no change.
	if tr := Expire(adj, t0.Add(10*time.Second), DefaultGracePeriod); tr.State != StateUp {
		t.Fatalf("pre-expiry state = %v, want up", tr.State)
	}
	// After expiry: Down + session-down.
	tr := Expire(adj, t0.Add(31*time.Second), DefaultGracePeriod)
	if tr.State != StateDown || !tr.SessionDown {
		t.Fatalf("post-expiry state = %v sessionDown = %v, want down/true", tr.State, tr.SessionDown)
	}
	if adj.deleteAt.IsZero() {
		t.Errorf("grace-period deletion not armed on timeout")
	}
}

// TestISISAdjFSMInitToDownOnTimeout: an Initializing adjacency that times out
// goes Down WITHOUT a session-down (it was never Up).
func TestISISAdjFSMInitToDownOnTimeout(t *testing.T) {
	adj := &Adjacency{State: StateInitializing, SystemID: peerSys, HoldExpiry: t0.Add(30 * time.Second)}
	tr := Expire(adj, t0.Add(31*time.Second), DefaultGracePeriod)
	if tr.State != StateDown {
		t.Fatalf("state = %v, want down", tr.State)
	}
	if tr.SessionDown {
		t.Errorf("unexpected SessionDown from Init->Down (was never Up)")
	}
}

// TestISISP2PThreeWay: with a TLV 240 reporting Up and echoing our System ID, the
// P2P adjacency reaches Up; a TLV 240 reporting Down keeps it Initializing.
func TestISISP2PThreeWay(t *testing.T) {
	local := p2pLocal(t)
	area := []types.AreaID{mustArea(t, 0x49, 0x00, 0x01)}

	// Neighbor reports Down (it has not heard us): stay Initializing.
	adj := &Adjacency{State: StateDown}
	tr := ReceiveHello(adj, local, HelloInput{
		SystemID: peerSys, Level: Level1, HoldTime: 30, Areas: area,
		HasThreeWay: true,
		ThreeWay:    packet.P2PThreeWayTLV{State: packet.AdjThreeWayDown, HasCircuitID: true},
	}, t0)
	if tr.State != StateInitializing {
		t.Fatalf("neighbor Down -> state %v, want initializing", tr.State)
	}

	// Neighbor reports Up AND echoes our System ID: reach Up.
	tr = ReceiveHello(adj, local, HelloInput{
		SystemID: peerSys, Level: Level1, HoldTime: 30, Areas: area,
		HasThreeWay: true,
		ThreeWay: packet.P2PThreeWayTLV{
			State: packet.AdjThreeWayUp, HasCircuitID: true,
			HasNeighbor: true, NeighborID: localSys,
		},
	}, t0)
	if tr.State != StateUp || !tr.SessionUp {
		t.Fatalf("neighbor Up+echo -> state %v sessionUp %v, want up/true", tr.State, tr.SessionUp)
	}
}

// TestISISP2PThreeWayNoEcho: a TLV 240 reporting Up but NOT echoing our System ID
// keeps the adjacency Initializing (the neighbor has not proven it heard us).
func TestISISP2PThreeWayNoEcho(t *testing.T) {
	adj := &Adjacency{State: StateDown}
	area := []types.AreaID{mustArea(t, 0x49, 0x00, 0x01)}
	tr := ReceiveHello(adj, p2pLocal(t), HelloInput{
		SystemID: peerSys, Level: Level1, HoldTime: 30, Areas: area,
		HasThreeWay: true,
		ThreeWay: packet.P2PThreeWayTLV{
			State: packet.AdjThreeWayUp, HasCircuitID: true,
			HasNeighbor: true, NeighborID: types.SystemID{9, 9, 9, 9, 9, 9}, // not us
		},
	}, t0)
	if tr.State != StateInitializing {
		t.Fatalf("no echo -> state %v, want initializing", tr.State)
	}
}

// TestISISP2PLegacyNoTLV240: a P2P peer that sends no TLV 240 reaches Up via the
// implicit (two-way) fall-back on the first Hello (RFC 5303 sec 3.2).
func TestISISP2PLegacyNoTLV240(t *testing.T) {
	adj := &Adjacency{State: StateDown}
	area := []types.AreaID{mustArea(t, 0x49, 0x00, 0x01)}
	tr := ReceiveHello(adj, p2pLocal(t), HelloInput{
		SystemID: peerSys, Level: Level1, HoldTime: 30, Areas: area,
		HasThreeWay: false, // legacy peer
	}, t0)
	if tr.State != StateUp || !tr.SessionUp {
		t.Fatalf("legacy P2P -> state %v sessionUp %v, want up/true", tr.State, tr.SessionUp)
	}
}

// TestISISL1AreaMatch: an L1 Hello with an overlapping area address forms an
// adjacency.
func TestISISL1AreaMatch(t *testing.T) {
	adj := &Adjacency{State: StateInitializing, SystemID: peerSys}
	tr := ReceiveHello(adj, lanLocal(t), HelloInput{
		SystemID: peerSys, SNPA: peerMAC, Level: Level1, HoldTime: 30,
		Areas:         []types.AreaID{mustArea(t, 0x49, 0x00, 0x01)},
		NeighborSNPAs: []SNPA{localMAC},
	}, t0)
	if tr.Rejected {
		t.Fatalf("matching area was rejected: %s", tr.RejectReason)
	}
	if tr.State != StateUp {
		t.Fatalf("matching L1 area -> state %v, want up", tr.State)
	}
}

// TestISISL1AreaMismatch: an L1 Hello with a non-overlapping area address is
// rejected; no adjacency forms.
func TestISISL1AreaMismatch(t *testing.T) {
	adj := &Adjacency{State: StateDown}
	tr := ReceiveHello(adj, lanLocal(t), HelloInput{
		SystemID: peerSys, SNPA: peerMAC, Level: Level1, HoldTime: 30,
		Areas:         []types.AreaID{mustArea(t, 0x49, 0x99, 0x99)}, // different area
		NeighborSNPAs: []SNPA{localMAC},
	}, t0)
	if !tr.Rejected || tr.RejectReason != "l1-area-mismatch" {
		t.Fatalf("expected l1-area-mismatch rejection, got %+v", tr)
	}
	if adj.State == StateUp {
		t.Errorf("adjacency reached Up despite area mismatch")
	}
}

// TestISISL2FormsAcrossAreas: an L2 Hello forms an adjacency regardless of area
// (RFC 1195 -- L2 forms across areas).
func TestISISL2FormsAcrossAreas(t *testing.T) {
	adj := &Adjacency{State: StateInitializing, SystemID: peerSys}
	tr := ReceiveHello(adj, lanLocal(t), HelloInput{
		SystemID: peerSys, SNPA: peerMAC, Level: Level2, HoldTime: 30,
		Areas:         []types.AreaID{mustArea(t, 0x49, 0x99, 0x99)}, // different area
		NeighborSNPAs: []SNPA{localMAC},
	}, t0)
	if tr.Rejected {
		t.Fatalf("L2 adjacency rejected on area mismatch: %s", tr.RejectReason)
	}
	if tr.State != StateUp {
		t.Fatalf("L2 across areas -> state %v, want up", tr.State)
	}
}

// TestISISAdjRejectsOwnSystemID: a Hello whose System ID equals our own (a
// looped-back or spoofed frame) is rejected with reason "own-system-id" and
// mutates no adjacency state. An IS must never form an adjacency with itself
// (ISO/IEC 10589 section 8.2): doing so would let a loop or a spoof create a
// phantom neighbor.
func TestISISAdjRejectsOwnSystemID(t *testing.T) {
	local := lanLocal(t)
	adj := &Adjacency{State: StateDown}
	in := HelloInput{
		SystemID:      local.SystemID, // our own System ID echoed back
		SNPA:          localMAC,
		Level:         Level1,
		HoldTime:      30,
		Areas:         local.Areas,
		NeighborSNPAs: []SNPA{localMAC}, // even with our SNPA echoed, must not reach Up
	}
	tr := ReceiveHello(adj, local, in, t0)
	if !tr.Rejected || tr.RejectReason != "own-system-id" {
		t.Fatalf("expected own-system-id rejection, got %+v", tr)
	}
	if tr.SessionUp || tr.SessionDown {
		t.Errorf("self-adjacency must emit no session event: %+v", tr)
	}
	// No adjacency mutation: the record stays Down with no neighbor identity.
	if adj.State != StateDown {
		t.Errorf("adjacency state = %v, want unchanged Down", adj.State)
	}
	if adj.SystemID != (types.SystemID{}) {
		t.Errorf("adjacency recorded a neighbor SystemID %v on a self-Hello", adj.SystemID)
	}
	if !adj.HoldExpiry.IsZero() {
		t.Errorf("self-Hello armed the hold timer (%v); it must not mutate state", adj.HoldExpiry)
	}
}

// TestISISAdjRejectsTooManyAreas: an IIH whose TLV-1 carries more area addresses
// than the effective Max Area Addresses is rejected with reason "too-many-areas"
// and mutates no adjacency state (ISO/IEC 10589 clause 8.4.1; the header value 0
// means the default 3). A count within the limit is accepted.
func TestISISAdjRejectsTooManyAreas(t *testing.T) {
	local := lanLocal(t)
	areasN := func(n int) []types.AreaID {
		out := make([]types.AreaID, 0, n)
		for i := range n {
			out = append(out, mustArea(t, 0x49, 0x00, byte(i+1)))
		}
		return out
	}

	cases := []struct {
		name       string
		maxArea    uint8
		areaCount  int
		wantReject bool
	}{
		{"default-limit-exceeded", 0, 4, true},  // 0 => default 3; 4 areas exceeds it
		{"default-limit-at-cap", 0, 3, false},   // exactly 3 is allowed
		{"explicit-limit-exceeded", 2, 3, true}, // header says 2; 3 areas exceeds it
		{"explicit-limit-at-cap", 5, 5, false},  // header says 5; 5 areas allowed
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			adj := &Adjacency{State: StateDown}
			in := HelloInput{
				SystemID:         peerSys,
				SNPA:             peerMAC,
				Level:            Level2, // L2 forms across areas, isolating the count check
				HoldTime:         30,
				MaxAreaAddresses: tc.maxArea,
				Areas:            areasN(tc.areaCount),
				NeighborSNPAs:    []SNPA{localMAC},
			}
			tr := ReceiveHello(adj, local, in, t0)
			if tc.wantReject {
				if !tr.Rejected || tr.RejectReason != "too-many-areas" {
					t.Fatalf("expected too-many-areas rejection, got %+v", tr)
				}
				if adj.State != StateDown || adj.SystemID != (types.SystemID{}) {
					t.Errorf("rejected over-limit Hello still mutated adjacency: state=%v sys=%v", adj.State, adj.SystemID)
				}
				return
			}
			if tr.Rejected {
				t.Fatalf("within-limit Hello rejected: %+v", tr)
			}
		})
	}
}

// TestISISDownOnCircuitDown: Down() forces an Up adjacency to Down and emits
// session-down regardless of the hold timer.
func TestISISDownOnCircuitDown(t *testing.T) {
	adj := &Adjacency{State: StateUp, SystemID: peerSys, HoldExpiry: t0.Add(time.Hour)}
	tr := Down(adj, t0, DefaultGracePeriod)
	if tr.State != StateDown || !tr.SessionDown {
		t.Fatalf("Down() -> state %v sessionDown %v, want down/true", tr.State, tr.SessionDown)
	}
}

// RFC requirement: RFC5303-3.1-6 positive -- a system able to process the option
// follows the three-way procedure: with a TLV 240 present, ReceiveHello
// (fsm.go:207 bidirectional) withholds Up until the neighbor proves it heard us,
// so an Up-reporting-but-no-echo neighbor stays Initializing.
// RFC requirement: RFC5303-3.1-6 negative -- the three-way procedure is engaged
// by the option, not applied blanket: with NO TLV 240 the legacy ISO 10589
// two-way adjacency forms on the first Hello with no echo at all.
func TestISISThreeWayProceduresEngagedByOption(t *testing.T) {
	area := []types.AreaID{mustArea(t, 0x49, 0x00, 0x01)}

	withOpt := &Adjacency{State: StateDown}
	tr := ReceiveHello(withOpt, p2pLocal(t), HelloInput{
		SystemID: peerSys, Level: Level1, HoldTime: 30, Areas: area,
		HasThreeWay: true,
		ThreeWay:    packet.P2PThreeWayTLV{State: packet.AdjThreeWayUp, HasCircuitID: true}, // no neighbor echo
	}, t0)
	if tr.State != StateInitializing {
		t.Fatalf("option present, no echo -> state %v, want initializing (procedure engaged)", tr.State)
	}

	noOpt := &Adjacency{State: StateDown}
	tr = ReceiveHello(noOpt, p2pLocal(t), HelloInput{
		SystemID: peerSys, Level: Level1, HoldTime: 30, Areas: area,
		HasThreeWay: false,
	}, t0)
	if tr.State != StateUp {
		t.Fatalf("no option -> legacy two-way should reach up, got %v", tr.State)
	}
}

// RFC requirement: RFC5303-3.2-13 positive -- when the received option does not
// yet prove bidirectionality the "Initialize" action applies: the adjacency is
// set to Initializing and NO session event is generated (classify at fsm.go:244).
// RFC requirement: RFC5303-3.2-13 negative -- the event-less Initializing outcome
// is specific to "Initialize": once the neighbor reports Up and echoes us the
// action is "Up", so the adjacency goes Up WITH a session event.
func TestISISThreeWayInitializeAction(t *testing.T) {
	area := []types.AreaID{mustArea(t, 0x49, 0x00, 0x01)}

	adj := &Adjacency{State: StateDown}
	tr := ReceiveHello(adj, p2pLocal(t), HelloInput{
		SystemID: peerSys, Level: Level1, HoldTime: 30, Areas: area,
		HasThreeWay: true,
		ThreeWay:    packet.P2PThreeWayTLV{State: packet.AdjThreeWayDown, HasCircuitID: true},
	}, t0)
	if tr.State != StateInitializing {
		t.Fatalf("state = %v, want initializing", tr.State)
	}
	if tr.SessionUp || tr.SessionDown {
		t.Fatalf("Initialize action generated a session event: %+v", tr)
	}

	tr = ReceiveHello(adj, p2pLocal(t), HelloInput{
		SystemID: peerSys, Level: Level1, HoldTime: 30, Areas: area,
		HasThreeWay: true,
		ThreeWay: packet.P2PThreeWayTLV{
			State: packet.AdjThreeWayUp, HasCircuitID: true,
			HasNeighbor: true, NeighborID: localSys,
		},
	}, t0)
	if tr.State != StateUp || !tr.SessionUp {
		t.Fatalf("state=%v sessionUp=%v, want up with session event", tr.State, tr.SessionUp)
	}
}
