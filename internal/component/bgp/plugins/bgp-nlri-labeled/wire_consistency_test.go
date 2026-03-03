package bgp_nlri_labeled_test

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"

	labeled "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/bgp-nlri-labeled"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/bgp/nlri"
)

// TestLabeledUnicastWireConsistency verifies two code paths produce identical wire format.
//
// VALIDATES: Immediate send and queued replay produce identical NLRI bytes.
// - Path 1: BuildLabeledUnicast → BuildLabeledUnicastNLRIBytes (immediate send)
// - Path 2: buildRIBRouteUpdate → nlri.LabeledUnicast.WriteTo (queued replay)
//
// PREVENTS: Route replay producing different wire encoding than original announcement.
func TestLabeledUnicastWireConsistency(t *testing.T) {
	tests := []struct {
		name    string
		prefix  netip.Prefix
		label   uint32
		pathID  uint32
		addPath bool
	}{
		{
			name:    "10.0.0.0/8 label=100 no-addpath",
			prefix:  netip.MustParsePrefix("10.0.0.0/8"),
			label:   100,
			pathID:  0,
			addPath: false,
		},
		{
			name:    "192.168.1.0/24 label=16000 no-addpath",
			prefix:  netip.MustParsePrefix("192.168.1.0/24"),
			label:   16000,
			pathID:  0,
			addPath: false,
		},
		{
			name:    "10.0.0.0/8 label=100 pathid=42 addpath",
			prefix:  netip.MustParsePrefix("10.0.0.0/8"),
			label:   100,
			pathID:  42,
			addPath: true,
		},
		{
			name:    "0.0.0.0/0 label=3 no-addpath",
			prefix:  netip.MustParsePrefix("0.0.0.0/0"),
			label:   3, // implicit null
			pathID:  0,
			addPath: false,
		},
		{
			name:    "10.1.2.3/32 label=1048575 no-addpath",
			prefix:  netip.MustParsePrefix("10.1.2.3/32"),
			label:   1048575, // max label
			pathID:  0,
			addPath: false,
		},
		{
			name:    "2001:db8::/32 label=100 no-addpath",
			prefix:  netip.MustParsePrefix("2001:db8::/32"),
			label:   100,
			pathID:  0,
			addPath: false,
		},
		{
			name:    "2001:db8::/32 label=100 pathid=1 addpath",
			prefix:  netip.MustParsePrefix("2001:db8::/32"),
			label:   100,
			pathID:  1,
			addPath: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Path 1: Build via UpdateBuilder (immediate send path)
			ub := message.NewUpdateBuilder(65001, false, true, tt.addPath)
			params := message.LabeledUnicastParams{
				Prefix: tt.prefix,
				PathID: tt.pathID,
				Labels: []uint32{tt.label},
			}
			expected := ub.BuildLabeledUnicastNLRIBytes(&params)

			// Path 2: Build via nlri.LabeledUnicast (queued replay path)
			family := nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIMPLSLabel}
			if tt.prefix.Addr().Is6() {
				family.AFI = nlri.AFIIPv6
			}
			n := labeled.NewLabeledUnicast(family, tt.prefix, []uint32{tt.label}, tt.pathID)
			actual := func() []byte {
				b := make([]byte, nlri.LenWithContext(n, tt.addPath))
				nlri.WriteNLRI(n, b, 0, tt.addPath)
				return b
			}()

			assert.Equal(t, expected, actual,
				"Wire format mismatch: BuildLabeledUnicastNLRIBytes vs nlri.LabeledUnicast.WriteTo")
		})
	}
}

// TestLabeledUnicastWireConsistency_AddPathZero verifies ADD-PATH with pathID=0.
//
// RFC 7911: Path Identifier MUST be present when ADD-PATH is negotiated,
// even if the value is 0. Both code paths now correctly include NOPATH.
func TestLabeledUnicastWireConsistency_AddPathZero(t *testing.T) {
	prefix := netip.MustParsePrefix("10.0.0.0/8")
	label := uint32(100)
	pathID := uint32(0) // Path ID is 0

	ub := message.NewUpdateBuilder(65001, false, true, true)
	params := message.LabeledUnicastParams{
		Prefix: prefix,
		PathID: pathID,
		Labels: []uint32{label},
	}
	builderBytes := ub.BuildLabeledUnicastNLRIBytes(&params)

	family := nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIMPLSLabel}
	n := labeled.NewLabeledUnicast(family, prefix, []uint32{label}, pathID)
	nlriBytes := func() []byte {
		b := make([]byte, nlri.LenWithContext(n, true))
		nlri.WriteNLRI(n, b, 0, true)
		return b
	}()

	// RFC 7911: Both should include 4-byte path ID (even if 0)
	// Format: [0,0,0,0][length][label][prefix]
	assert.Equal(t, builderBytes, nlriBytes,
		"Wire format must match: both include NOPATH when ADD-PATH negotiated")

	// Verify NOPATH prefix is present
	assert.Equal(t, []byte{0, 0, 0, 0}, builderBytes[:4],
		"First 4 bytes should be NOPATH (path ID = 0)")
}
