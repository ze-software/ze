package reactor

import (
	"math"
	"testing"
)

// VALIDATES: AC-28 — pool maximum dynamically tracks peer set via weight
// PREVENTS: fixed pool sizes that ignore actual peer workload

func TestBurstFraction(t *testing.T) {
	// Boundary testing: last valid in each tier, first in next tier.
	tests := []struct {
		name      string
		prefixMax uint32
		want      float64
	}{
		{"zero", 0, 1.0},
		{"one", 1, 1.0},
		{"tier1-last", 499, 1.0},
		{"tier2-first", 500, 0.5},
		{"tier2-mid", 5000, 0.5},
		{"tier2-last", 9999, 0.5},
		{"tier3-first", 10000, 0.3},
		{"tier3-mid", 50000, 0.3},
		{"tier3-last", 99999, 0.3},
		{"tier4-first", 100000, 0.15},
		{"tier4-mid", 300000, 0.15},
		{"tier4-last", 499999, 0.15},
		{"tier5-first", 500000, 0.1},
		{"tier5-full-table", 1000000, 0.1},
		{"tier5-large", 2000000, 0.1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := burstFraction(tt.prefixMax)
			if got != tt.want {
				t.Errorf("burstFraction(%d) = %v, want %v", tt.prefixMax, got, tt.want)
			}
		})
	}
}

func TestBurstWeight(t *testing.T) {
	tests := []struct {
		name      string
		prefixMax uint32
		want      int
	}{
		{"small-200", 200, 200},         // 200 * 1.0
		{"medium-1000", 1000, 500},      // 1000 * 0.5
		{"medium-5000", 5000, 2500},     // 5000 * 0.5
		{"transit-50000", 50000, 15000}, // 50000 * 0.3
		{"large-100000", 100000, 15000}, // 100000 * 0.15
		{"large-300000", 300000, 45000}, // 300000 * 0.15
		{"full-table", 1000000, 100000}, // 1000000 * 0.1
		{"zero", 0, 0},                  // 0 * 1.0
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := burstWeight(tt.prefixMax)
			if got != tt.want {
				t.Errorf("burstWeight(%d) = %d, want %d", tt.prefixMax, got, tt.want)
			}
		})
	}
}

func TestBuffersNeeded(t *testing.T) {
	// buffersNeeded divides by nlriPerMessage (20) and rounds up.
	tests := []struct {
		name  string
		nlris int
		want  int
	}{
		{"negative", -5, 0},
		{"zero", 0, 0},
		{"one", 1, 1},
		{"exact-20", 20, 1},
		{"21-rounds-up", 21, 2},
		{"100", 100, 5},
		{"101-rounds-up", 101, 6},
		{"1000", 1000, 50},
		{"100000", 100000, 5000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buffersNeeded(tt.nlris)
			if got != tt.want {
				t.Errorf("buffersNeeded(%d) = %d, want %d", tt.nlris, got, tt.want)
			}
		})
	}
}

func TestPeerBufferDemand(t *testing.T) {
	// Pre-EOR: full table. Post-EOR: burst weight only.
	tests := []struct {
		name      string
		prefixMax uint32
		preEOR    int // expected pre-EOR buffers
		postEOR   int // expected post-EOR buffers
	}{
		{"small-200", 200, 10, 10},           // 200/20=10, burstWeight=200, 200/20=10
		{"medium-1000", 1000, 50, 25},        // 1000/20=50, burstWeight=500, 500/20=25
		{"transit-50000", 50000, 2500, 750},  // 50000/20=2500, burstWeight=15000, 15000/20=750
		{"full-table", 1000000, 50000, 5000}, // 1M/20=50000, burstWeight=100000, 100000/20=5000
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pre := peerBufferDemand(tt.prefixMax, true)
			post := peerBufferDemand(tt.prefixMax, false)
			if pre != tt.preEOR {
				t.Errorf("peerBufferDemand(%d, preEOR=true) = %d, want %d", tt.prefixMax, pre, tt.preEOR)
			}
			if post != tt.postEOR {
				t.Errorf("peerBufferDemand(%d, preEOR=false) = %d, want %d", tt.prefixMax, post, tt.postEOR)
			}
		})
	}
}

func TestOverflowPeerCount(t *testing.T) {
	// K = max(1, sqrt(N))
	tests := []struct {
		name  string
		peers int
		want  int
	}{
		{"zero", 0, 1},
		{"one", 1, 1},
		{"four", 4, 2},
		{"nine", 9, 3},
		{"twenty-five", 25, 5},
		{"fifty", 50, 7},
		{"hundred", 100, 10},
		{"two-hundred", 200, 14},
		{"five-hundred", 500, 22},
		{"thousand", 1000, 31},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := overflowPeerCount(tt.peers)
			if got != tt.want {
				t.Errorf("overflowPeerCount(%d) = %d, want %d", tt.peers, got, tt.want)
			}
			// Verify against direct sqrt calculation.
			expected := max(1, int(math.Sqrt(float64(tt.peers))))
			if got != expected {
				t.Errorf("overflowPeerCount(%d) = %d, expected max(1, sqrt(%d)) = %d",
					tt.peers, got, tt.peers, expected)
			}
		})
	}
}

func TestPoolBudget(t *testing.T) {
	// Verifies guaranteed + overflow calculation.
	tests := []struct {
		name           string
		peers          []uint32 // prefix maximums
		preEOR         bool     // all peers in same phase for simplicity
		wantGuaranteed int
		wantOverflow   int
	}{
		{
			name:           "single-peer",
			peers:          []uint32{1000},
			preEOR:         false,
			wantGuaranteed: 25, // burstWeight=500, 500/20=25
			wantOverflow:   25, // K=1, top 1 = 25
		},
		{
			name:           "two-peers-same-size",
			peers:          []uint32{1000, 1000},
			preEOR:         false,
			wantGuaranteed: 50, // 25 + 25
			wantOverflow:   25, // K=max(1,sqrt(2))=1, top 1 = 25
		},
		{
			name:   "four-peers-mixed",
			peers:  []uint32{200, 1000, 10000, 100000},
			preEOR: false,
			// burstWeights: 200, 500, 3000, 15000
			// buffers: 10, 25, 150, 750
			wantGuaranteed: 935, // 10 + 25 + 150 + 750
			wantOverflow:   900, // K=2, top 2 = 750 + 150
		},
		{
			name:           "pre-eor-single",
			peers:          []uint32{100000},
			preEOR:         true,
			wantGuaranteed: 5000, // 100000/20
			wantOverflow:   5000, // K=1, top 1 = 5000
		},
		{
			name:           "empty",
			peers:          []uint32{},
			preEOR:         false,
			wantGuaranteed: 0,
			wantOverflow:   0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			demands := make([]int, len(tt.peers))
			for i, pm := range tt.peers {
				demands[i] = peerBufferDemand(pm, tt.preEOR)
			}
			g, o := calculatePoolBudget(demands)
			if g != tt.wantGuaranteed {
				t.Errorf("guaranteed = %d, want %d", g, tt.wantGuaranteed)
			}
			if o != tt.wantOverflow {
				t.Errorf("overflow = %d, want %d", o, tt.wantOverflow)
			}
		})
	}
}

// TestCalculatePoolBudget_MinFloorApplied verifies that when peers have very low
// prefix maximums, the auto-sized budget from calculatePoolBudget is small enough
// that the minPoolBudget floor in reactor.go is necessary.
//
// VALIDATES: The minPoolBudget floor (bufMuxBlockSize * MaxMsgLen = 128 * 4096 = 524288)
//
//	is needed because calculatePoolBudget can return values that produce budgets
//	smaller than a single block allocation.
//
// PREVENTS: Removing the minPoolBudget floor under the assumption it is never reached.
func TestCalculatePoolBudget_MinFloorApplied(t *testing.T) {
	// A peer with prefixMax=2 and familyCount=1:
	//   burstWeight(2) = 2 (tier 1: 2 * 1.0)
	//   peerBufferDemand(2, false) = buffersNeeded(2) = 1 (ceil(2/20))
	//
	// With a single peer, guaranteed=1, overflow=1.
	demand := peerBufferDemand(2, false)
	if demand != 1 {
		t.Fatalf("peerBufferDemand(2, false) = %d, want 1", demand)
	}

	g, o := calculatePoolBudget([]int{demand})
	if g != 1 {
		t.Errorf("guaranteed = %d, want 1", g)
	}
	if o != 1 {
		t.Errorf("overflow = %d, want 1", o)
	}

	// The reactor computes: total = max((g+o)*MaxMsgLen, minPoolBudget)
	// where minPoolBudget = bufMuxBlockSize * MaxMsgLen = 128 * 4096 = 524288.
	//
	// Without the floor: (1+1) * 4096 = 8192 -- too small for a single
	// block allocation (128 * 4096 = 524288). The floor is necessary.
	const maxMsgLen = 4096 // message.MaxMsgLen
	autoSized := int64(g+o) * maxMsgLen
	minPoolBudget := int64(bufMuxBlockSize) * maxMsgLen // 128 * 4096 = 524288

	if autoSized >= minPoolBudget {
		t.Errorf("auto-sized budget %d >= minPoolBudget %d; floor would be unnecessary",
			autoSized, minPoolBudget)
	}

	// Verify the floor produces a usable budget.
	total := max(autoSized, minPoolBudget)
	if total != minPoolBudget {
		t.Errorf("total = %d, want minPoolBudget %d", total, minPoolBudget)
	}
}

func TestOverflowFanOut(t *testing.T) {
	// fanOut = min(N-1, floor(2*sqrt(N))), floor 1
	tests := []struct {
		name  string
		peers int
		want  int
	}{
		{"zero", 0, 1},
		{"one", 1, 1},
		{"two", 2, 1},     // min(1, 2*1.41)=1
		{"three", 3, 2},   // min(2, 2*1.73)=2
		{"four", 4, 3},    // min(3, 2*2)=3 -- but 2*sqrt(4)=4, min(3,4)=3
		{"five", 5, 4},    // min(4, 2*2.23)=4
		{"ten", 10, 6},    // min(9, 2*3.16)=6
		{"fifty", 50, 14}, // min(49, 2*7.07)=14
		{"hundred", 100, 20},
		{"two-hundred", 200, 28},
		{"thousand", 1000, 63},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := overflowFanOut(tt.peers)
			if got != tt.want {
				t.Errorf("overflowFanOut(%d) = %d, want %d", tt.peers, got, tt.want)
			}
		})
	}
}

func TestOverflowPoolBudget(t *testing.T) {
	// Pure function: largest peer restart burst * fan-out + 10% steady-state.
	tests := []struct {
		name      string
		peers     []overflowPeerInput
		wantSlots int
	}{
		{
			name:      "empty",
			peers:     nil,
			wantSlots: 64, // floor
		},
		{
			name: "single-peer-10K-pfx",
			// peerBufferDemand(10000, true) = 10000/20 = 500
			// fanOut(1) = 1, restartBurst = 500*1 = 500
			// no other peers for steady contrib
			// total = 500, floor 64 -> 500
			peers:     []overflowPeerInput{{prefixMax: 10000, extMsg: false}},
			wantSlots: 500,
		},
		{
			name: "chaos-default-4-peers-10K",
			// All 4 peers: prefixMax=10000
			// largest demand (preEOR): peerBufferDemand(10000, true) = 500
			// fanOut(4) = min(3, floor(2*2)) = 3
			// restartBurst = 500 * 3 = 1500
			// steady: 3 other peers * peerBufferDemand(10000, false) * 0.1
			//   peerBufferDemand(10000, false) = buffersNeeded(burstWeight(10000))
			//   burstWeight(10000) = 10000*0.3 = 3000
			//   buffersNeeded(3000) = ceil(3000/20) = 150
			//   steady = 3 * 150 * 0.1 = 45
			// total = 1500 + 45 = 1545
			peers: []overflowPeerInput{
				{prefixMax: 10000, extMsg: false},
				{prefixMax: 10000, extMsg: false},
				{prefixMax: 10000, extMsg: false},
				{prefixMax: 10000, extMsg: false},
			},
			wantSlots: 1545,
		},
		{
			name: "two-peers-different-sizes",
			// peer1: prefixMax=1000, peer2: prefixMax=100000
			// largest preEOR demand: peerBufferDemand(100000, true) = 100000/20 = 5000
			// fanOut(2) = min(1, floor(2*1.41)) = 1
			// restartBurst = 5000 * 1 = 5000
			// steady: 1 other peer (the 1000 one)
			//   peerBufferDemand(1000, false) = buffersNeeded(burstWeight(1000))
			//   burstWeight(1000) = 1000*0.5 = 500
			//   buffersNeeded(500) = ceil(500/20) = 25
			//   steady = 1 * 25 * 0.1 = 2 (truncated to int)
			// total = 5000 + 2 = 5002
			peers: []overflowPeerInput{
				{prefixMax: 1000, extMsg: false},
				{prefixMax: 100000, extMsg: false},
			},
			wantSlots: 5002,
		},
		{
			name: "floor-applied",
			// Single peer with very low prefix max
			// peerBufferDemand(2, true) = ceil(2/20) = 1
			// fanOut(1) = 1, restartBurst = 1
			// total = 1, floor 64 -> 64
			peers:     []overflowPeerInput{{prefixMax: 2, extMsg: false}},
			wantSlots: 64,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := overflowPoolBudget(tt.peers)
			if got.slots != tt.wantSlots {
				t.Errorf("overflowPoolBudget() slots = %d, want %d", got.slots, tt.wantSlots)
			}
		})
	}
}

func TestOverflowPoolBudgetBytes(t *testing.T) {
	// Byte budget accounts for mixed sizes: 4K per standard slot, 64K per ExtMsg slot.
	tests := []struct {
		name      string
		peers     []overflowPeerInput
		wantBytes int64
	}{
		{
			name: "all-standard",
			// Single peer, 10K prefixes, standard (4K)
			// slots = 500 (from TestOverflowPoolBudget)
			// bytes = 500 * 4096
			peers:     []overflowPeerInput{{prefixMax: 10000, extMsg: false}},
			wantBytes: 500 * 4096,
		},
		{
			name: "single-extmsg-peer",
			// Single ExtMsg peer, 10K prefixes
			// slots = 500, bytes = 500 * 65535
			peers:     []overflowPeerInput{{prefixMax: 10000, extMsg: true}},
			wantBytes: 500 * 65535,
		},
		{
			name: "mixed-standard-and-extmsg",
			// 2 standard (10K pfx) + 1 ExtMsg (100K pfx, largest)
			// largest preEOR: peerBufferDemand(100000, true) = 100000/20 = 5000
			// fanOut(3) = min(2, floor(2*1.73)) = 2
			// restartBurst = 5000 * 2 = 10000
			// steady: 2 standard peers, peerBufferDemand(10000, false) = 150
			//   each: 150 * 0.1 = 15 slots * 4096 bytes
			// totalSlots = 10000 + 30 = 10030
			// restartBurst bytes: 10000 * 65535 (ExtMsg peer)
			// steady bytes: 15*4096 + 15*4096
			peers: []overflowPeerInput{
				{prefixMax: 10000, extMsg: false},
				{prefixMax: 10000, extMsg: false},
				{prefixMax: 100000, extMsg: true},
			},
			wantBytes: 10000*65535 + 30*4096,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := overflowPoolBudget(tt.peers)
			if got.bytes != tt.wantBytes {
				t.Errorf("overflowPoolBudget() bytes = %d, want %d", got.bytes, tt.wantBytes)
			}
		})
	}
}

func TestTotalPrefixMax(t *testing.T) {
	tests := []struct {
		name string
		m    map[string]uint32
		want uint32
	}{
		{"nil", nil, 0},
		{"empty", map[string]uint32{}, 0},
		{"single", map[string]uint32{"ipv4/unicast": 100000}, 100000},
		{"multi", map[string]uint32{"ipv4/unicast": 100000, "ipv6/unicast": 50000}, 150000},
		{"overflow-saturates", map[string]uint32{"a": math.MaxUint32, "b": 1}, math.MaxUint32},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := totalPrefixMax(tt.m)
			if got != tt.want {
				t.Errorf("totalPrefixMax() = %d, want %d", got, tt.want)
			}
		})
	}
}
