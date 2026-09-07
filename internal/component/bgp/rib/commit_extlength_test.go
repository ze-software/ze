// Related: commit.go — packAttributesWithASPath, the rail this sizes for
//
// VALIDATES: the commit rail allocates the buffer WriteAttrTo actually fills,
// for an attribute whose flags already carry the Extended Length bit over a
// value of 255 octets or fewer.
// PREVENTS: a one-octet under-allocation. RFC 4271 Section 4.3 makes the length
// field two octets whenever the Extended Length bit is set, and WriteHeaderTo
// emits the four-octet header on the FLAG as well as on the length, so a sizer
// that tests only "value exceeds 255" answers three where the writer writes
// four. Until 2026-09-07 the rail carried its own attrSize and attrSizeWithContext
// that did exactly that, and the block ran one octet past its own buffer.
package rib

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// extLengthShortValueAttr is the shape a peer can put on the wire and ze relays:
// an unknown optional-transitive attribute whose sender set Extended Length even
// though four octets of value do not require it. RFC 4271 Section 4.3 permits it,
// so ze must size for what it will write rather than for what the length implies.
func extLengthShortValueAttr() attribute.Attribute {
	return attribute.NewOpaqueAttribute(
		attribute.FlagOptional|attribute.FlagTransitive|attribute.FlagExtLength,
		attribute.AttributeCode(233),
		[]byte{0xDE, 0xAD, 0xBE, 0xEF},
	)
}

// TestPackAttributesSizesTheExtLengthHeaderItWrites walks the packed block with
// the same rules a peer's parser uses. A block sized three where four were
// written either panics inside the pack or hands back a buffer whose last
// attribute runs past its end, and both are read here rather than assumed.
func TestPackAttributesSizesTheExtLengthHeaderItWrites(t *testing.T) {
	cs := NewCommitService(&mockUpdateSender{}, testContext(65000, 65001, true), true)

	block, err := cs.packAttributesWithASPath(
		[]attribute.Attribute{extLengthShortValueAttr()}, nil,
		netip.MustParseAddr("10.0.0.1"),
		newIPv4NLRI("192.168.1.0/24").Family(),
		nil,
	)
	require.NoError(t, err)

	// attrsOnTheWire fails the test if any attribute runs past the block, which
	// is the observable an under-allocation produces when it does not panic.
	onWire := attrsOnTheWire(t, block)

	value, carried := onWire[233]
	require.True(t, carried, "the relayed attribute reached the block")
	require.Equal(t, []byte{0xDE, 0xAD, 0xBE, 0xEF}, value,
		"an under-sized header shifts every octet after it")
}
