package engine

import (
	"net/netip"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/plugins/bfd/api"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bfd/transport"
)

// VALIDATES: RFC 5881 Section 5 -- single-hop GTSM discards every packet
// whose TTL is not exactly 255. Last-valid is 255; first-invalid is 254.
// PREVENTS: regression where a single-hop session accepts spoofed traffic
// from an off-link source (TTL decremented by at least one router).
func TestTTLGateSingleHop(t *testing.T) {
	tests := []struct {
		name string
		ttl  uint8
		want bool
	}{
		{"exactly 255", 255, true},
		{"one below", 254, false},
		{"kernel could not extract", 0, false},
		{"two below", 253, false},
		{"half the space", 128, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := transport.Inbound{
				From: netip.MustParseAddr("203.0.113.1"),
				Mode: api.SingleHop,
				TTL:  tt.ttl,
			}
			// MinTTL is irrelevant for single-hop; passing a non-zero
			// value verifies the dispatch does not consult it.
			got := passesTTLGate(in, 1)
			if got != tt.want {
				t.Fatalf("passesTTLGate(ttl=%d, single-hop) = %v, want %v", tt.ttl, got, tt.want)
			}
		})
	}
}

// VALIDATES: RFC 5883 Section 5 -- multi-hop minimum-TTL check is
// inclusive. A session with MinTTL=254 accepts exactly 254 but rejects
// 253. A session with MinTTL=1 accepts exactly 1 (boundary).
// PREVENTS: off-by-one regressions in the multi-hop gate.
func TestTTLGateMultiHop(t *testing.T) {
	tests := []struct {
		name   string
		ttl    uint8
		minTTL uint8
		want   bool
	}{
		{"default min 254 accept boundary", 254, 254, true},
		{"default min 254 reject one below", 253, 254, false},
		{"default min 254 reject two below", 252, 254, false},
		{"min 1 accept boundary", 1, 1, true},
		{"min 1 reject zero", 0, 1, false},
		{"min 10 accept", 64, 10, true},
		{"min 10 reject", 9, 10, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := transport.Inbound{
				From: netip.MustParseAddr("203.0.113.1"),
				Mode: api.MultiHop,
				TTL:  tt.ttl,
			}
			got := passesTTLGate(in, tt.minTTL)
			if got != tt.want {
				t.Fatalf("passesTTLGate(ttl=%d, min=%d) = %v, want %v", tt.ttl, tt.minTTL, got, tt.want)
			}
		})
	}
}

// VALIDATES: unknown modes fail closed.
// PREVENTS: regression where adding a new HopMode accidentally defaults
// to accept.
func TestTTLGateUnknownMode(t *testing.T) {
	in := transport.Inbound{
		From: netip.MustParseAddr("203.0.113.1"),
		Mode: api.HopMode(99),
		TTL:  255,
	}
	if passesTTLGate(in, 1) {
		t.Fatal("unknown mode should fail closed, got accept")
	}
}
