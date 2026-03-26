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
