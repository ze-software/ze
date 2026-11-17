// Design: docs/architecture/edge-cases/as4.md -- selecting the received aggregating node.
// Related: aspath_collapse.go -- prepared reconciliation and rebased attribute spans.

package wireu

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/wire"
)

// TestSelectedAggregatorPartialSurvivesCollapse follows the selected information
// source's Partial bit, independently of the ignored attribute's flags.
// RFC 6793 Section 4.2.3 makes a non-AS_TRANS AGGREGATOR authoritative; otherwise
// "the AS4_AGGREGATOR attribute SHALL be taken as the information about the
// aggregating node". Section 4.1 discards AS4 attributes from a NEW speaker.
// RFC 4271 Section 5: "If a path with a recognized, transitive optional attribute
// is accepted and passed along to other BGP peers and the Partial bit in the
// Attribute Flags octet is set to 1 by some previous AS, it MUST NOT be set back
// to 0 by the current AS." Ignored Partial bits must not contaminate a complete
// selected attribute. These are direct collapse/plan tests, not Session proofs.
func TestSelectedAggregatorPartialSurvivesCollapse(t *testing.T) {
	oldPath := []byte{2, 1, 0xfd, 0xea}
	transPath := []byte{2, 1, 0x5b, 0xa0}
	widePath := []byte{2, 1, 0, 0, 0xfd, 0xea}
	selectedPath := []byte{2, 1, 0, 1, 0x11, 0x71} // AS 70001
	oldNode := []byte{0xfd, 0xe9, 192, 0, 2, 9}
	transNode := []byte{0x5b, 0xa0, 192, 0, 2, 9}
	wideNode := []byte{0, 0, 0xfd, 0xe9, 192, 0, 2, 9}
	selectedNode := []byte{0, 1, 0x11, 0x70, 192, 0, 2, 10} // AS 70000
	cases := []selectedAggregatorCase{
		{
			name: "old-mappable-original-partial", originalFlags: 0xe0, companionFlags: 0xc0,
			path: oldPath, node: oldNode, expectedPath: widePath, expectedNode: wideNode,
			expectedFlags: 0xe0, discards: []attribute.AttributeCode{attribute.AttrAS4Aggregator},
		},
		{
			name: "old-mappable-ignored-companion-partial", originalFlags: 0xc0, companionFlags: 0xe0,
			path: oldPath, node: oldNode, expectedPath: widePath, expectedNode: wideNode,
			expectedFlags: 0xc0, discards: []attribute.AttributeCode{attribute.AttrAS4Aggregator},
		},
		{
			name: "old-trans-selected-companion-partial", originalFlags: 0xc0, companionFlags: 0xe0,
			path: transPath, node: transNode, expectedPath: selectedPath, expectedNode: selectedNode,
			expectedFlags: 0xe0, rebase: true,
		},
		{
			name: "old-trans-ignored-original-partial", originalFlags: 0xe0, companionFlags: 0xc0,
			path: transPath, node: transNode, expectedPath: selectedPath, expectedNode: selectedNode,
			expectedFlags: 0xc0, rebase: true,
		},
		{
			name: "new-original-partial", wide: true, originalFlags: 0xe0, companionFlags: 0xc0,
			path: widePath, node: wideNode, expectedPath: widePath, expectedNode: wideNode,
			expectedFlags: 0xe0,
			discards:      []attribute.AttributeCode{attribute.AttrAS4Path, attribute.AttrAS4Aggregator},
		},
		{
			name: "new-ignored-companion-partial", wide: true, originalFlags: 0xc0, companionFlags: 0xe0,
			path: widePath, node: wideNode, expectedPath: widePath, expectedNode: wideNode,
			expectedFlags: 0xc0,
			discards:      []attribute.AttributeCode{attribute.AttrAS4Path, attribute.AttrAS4Aggregator},
		},
	}
	for _, tc := range cases {
		attrs := concatAttrs(
			buildOriginAttr(),
			collapseAttrWire(attribute.FlagTransitive, attribute.AttrASPath, tc.path),
			[]byte{0x40, 3, 4, 192, 0, 2, 254},
			[]byte{0x80, 4, 4, 0, 0, 0, 10},
			collapseAttrWire(tc.originalFlags, attribute.AttrAggregator, tc.node),
			collapseAttrWire(0xc0, attribute.AttrAS4Path, selectedPath),
			collapseAttrWire(tc.companionFlags, attribute.AttrAS4Aggregator, selectedNode),
			[]byte{0xe0, 250, 1, 0x42},
		)
		t.Run(tc.name+"/collapse", func(t *testing.T) {
			payload := buildPayload([]byte{24, 198, 51, 100}, attrs, []byte{24, 203, 0, 113})
			original := bytes.Clone(payload)
			dst := make([]byte, CollapseAS4FamilySize(payload))
			n, discards, err := CollapseAS4Family(dst, payload, tc.wide)
			require.NoError(t, err)
			require.Positive(t, n, "a carried AS4 companion requires reconciliation")
			require.LessOrEqual(t, n, len(dst))
			require.Equal(t, original, payload, "the borrowed receive buffer stays unchanged")
			selectedAggregatorDiscards(t, discards, tc.discards)
			selectedAggregatorProjection(t, dst[:n], tc)
		})
		if tc.rebase {
			t.Run(tc.name+"/rebased-plan", func(t *testing.T) {
				original := buildPayload(nil, attrs, []byte{24, 203, 0, 113})
				plan, err := PrepareAS4Family(original, tc.wide)
				require.NoError(t, err)
				require.Equal(t, tc.expectedPath, plan.ASPath(original))
				// WriteTo MUST receive the prepared attributes, but may receive
				// a different withdrawal section. The old buffer is recycled.
				clear(original)
				rebound := buildPayload([]byte{24, 198, 51, 100}, attrs, []byte{24, 203, 0, 113})
				before := bytes.Clone(rebound)
				dst := make([]byte, CollapseAS4FamilySize(rebound))
				n, discards, err := plan.WriteTo(dst, rebound)
				require.NoError(t, err)
				require.Positive(t, n, "the prepared companion selection still requires encoding")
				require.LessOrEqual(t, n, len(dst))
				require.Equal(t, before, rebound, "the replacement input stays borrowed")
				require.Equal(t, make([]byte, len(original)), original, "encoding never revives the old buffer")
				selectedAggregatorDiscards(t, discards, tc.discards)
				selectedAggregatorProjection(t, dst[:n], tc)
			})
		}
	}
}

type selectedAggregatorCase struct {
	name           string
	wide           bool
	originalFlags  attribute.AttributeFlags
	companionFlags attribute.AttributeFlags
	path           []byte
	node           []byte
	expectedPath   []byte
	expectedNode   []byte
	expectedFlags  byte
	discards       []attribute.AttributeCode
	rebase         bool
}

func selectedAggregatorDiscards(t *testing.T, discards []attribute.ASPathDiscard, expected []attribute.AttributeCode) {
	t.Helper()
	var codes []attribute.AttributeCode
	for _, discard := range discards {
		require.Error(t, discard.Reason, "the existing discard report must retain its reason")
		codes = append(codes, discard.Code)
	}
	require.Equal(t, expected, codes)
}

// selectedAggregatorProjection pins useful canonical output before the flag
// assertion. Internal canonicalization removes AS4 companions even when the
// selected node or path contains a non-mappable AS number.
func selectedAggregatorProjection(t *testing.T, payload []byte, expected selectedAggregatorCase) {
	t.Helper()
	sections, err := wire.ParseUpdateSections(payload)
	require.NoError(t, err)
	require.Equal(t, []byte{24, 198, 51, 100}, sections.Withdrawn(payload))
	require.Equal(t, []byte{24, 203, 0, 113}, sections.NLRI(payload))
	attrs := sections.Attrs(payload)
	for _, control := range []struct {
		code  attribute.AttributeCode
		flags byte
		value []byte
	}{
		{attribute.AttrOrigin, 0x40, []byte{0}},
		{attribute.AttrASPath, 0x40, expected.expectedPath},
		{attribute.AttrNextHop, 0x40, []byte{192, 0, 2, 254}},
		{attribute.AttrMED, 0x80, []byte{0, 0, 0, 10}},
		{attribute.AttributeCode(250), 0xe0, []byte{0x42}},
	} {
		_, flags, value, found := attribute.AttrFind(attrs, control.code)
		require.True(t, found, "useful attribute %d survives", control.code)
		require.Equal(t, control.flags, byte(flags), "attribute %d flags", control.code)
		require.Equal(t, control.value, value, "attribute %d value", control.code)
	}
	for _, code := range []attribute.AttributeCode{attribute.AttrAS4Path, attribute.AttrAS4Aggregator} {
		_, _, _, found := attribute.AttrFind(attrs, code)
		require.False(t, found, "AS4 companion %d must not survive canonicalization", code)
	}
	_, flags, value, found := attribute.AttrFind(attrs, attribute.AttrAggregator)
	require.True(t, found, "the selected aggregating node survives")
	require.Equal(t, expected.expectedNode, value, "the selected AS and IPv4 address survive")
	require.Equal(t, expected.expectedFlags, byte(flags), "flags follow only the selected information source")
}
