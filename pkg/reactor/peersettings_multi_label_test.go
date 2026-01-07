package reactor

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestStaticRoute_MultiLabel verifies StaticRoute multi-label support.
//
// VALIDATES: StaticRoute.Labels field and IsLabeledUnicast() with multiple labels.
// PREVENTS: Regression where multi-label routes are incorrectly classified.
func TestStaticRoute_MultiLabel(t *testing.T) {
	tests := []struct {
		name            string
		labels          []uint32
		rd              string
		wantIsVPN       bool
		wantIsLabeled   bool
		wantSingleLabel uint32
	}{
		{
			name:            "single label, no RD = labeled unicast",
			labels:          []uint32{1000},
			rd:              "",
			wantIsVPN:       false,
			wantIsLabeled:   true,
			wantSingleLabel: 1000,
		},
		{
			name:            "two labels, no RD = labeled unicast",
			labels:          []uint32{1000, 2000},
			rd:              "",
			wantIsVPN:       false,
			wantIsLabeled:   true,
			wantSingleLabel: 1000,
		},
		{
			name:            "single label with RD = VPN",
			labels:          []uint32{1000},
			rd:              "100:100",
			wantIsVPN:       true,
			wantIsLabeled:   false,
			wantSingleLabel: 1000,
		},
		{
			name:            "two labels with RD = VPN",
			labels:          []uint32{1000, 2000},
			rd:              "100:100",
			wantIsVPN:       true,
			wantIsLabeled:   false,
			wantSingleLabel: 1000,
		},
		{
			name:            "no labels, no RD = plain unicast",
			labels:          nil,
			rd:              "",
			wantIsVPN:       false,
			wantIsLabeled:   false,
			wantSingleLabel: 0,
		},
		{
			name:            "empty labels slice, no RD = plain unicast",
			labels:          []uint32{},
			rd:              "",
			wantIsVPN:       false,
			wantIsLabeled:   false,
			wantSingleLabel: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			route := StaticRoute{
				Prefix: netip.MustParsePrefix("10.0.0.0/8"),
				Labels: tt.labels,
				RD:     tt.rd,
			}

			assert.Equal(t, tt.wantIsVPN, route.IsVPN(), "IsVPN mismatch")
			assert.Equal(t, tt.wantIsLabeled, route.IsLabeledUnicast(), "IsLabeledUnicast mismatch")
			assert.Equal(t, tt.wantSingleLabel, route.SingleLabel(), "SingleLabel mismatch")
		})
	}
}

// TestStaticRoute_SingleLabel verifies SingleLabel() helper.
//
// VALIDATES: SingleLabel() returns first label from stack for backward compatibility.
// PREVENTS: Breakage when code expects single label value from multi-label route.
func TestStaticRoute_SingleLabel(t *testing.T) {
	tests := []struct {
		name   string
		labels []uint32
		want   uint32
	}{
		{name: "nil labels", labels: nil, want: 0},
		{name: "empty labels", labels: []uint32{}, want: 0},
		{name: "single label", labels: []uint32{100}, want: 100},
		{name: "two labels", labels: []uint32{100, 200}, want: 100},
		{name: "three labels", labels: []uint32{100, 200, 300}, want: 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			route := StaticRoute{
				Prefix: netip.MustParsePrefix("10.0.0.0/8"),
				Labels: tt.labels,
			}
			assert.Equal(t, tt.want, route.SingleLabel())
		})
	}
}
