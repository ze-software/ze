// Design: docs/architecture/core-design.md -- a library chain member drops NLRI by offset
// Related: filter_library.go -- libraryNLRIOps applies the drops to the UPDATE
// Related: filter_library_intents_test.go -- runLibraryImport, the intent member

package reactor

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/internal/core/family"
)

// nlriDrop is one entry a member asks out: its section and its offset there.
type nlriDrop struct {
	section policychain.NLRISection
	offset  uint16
}

// dropAt answers an evaluator body that drops exactly the given entries.
func dropAt(drops ...nlriDrop) func(*policyview.View, *policychain.Sets) error {
	return func(_ *policyview.View, sets *policychain.Sets) error {
		for _, d := range drops {
			if err := sets.DropNLRI(d.section, d.offset); err != nil {
				return err
			}
		}
		return nil
	}
}

// TestLibraryNLRIDropRewritesMPSections proves a library member's NLRI drop
// removes the entry from the MP section that carries it, at UPDATE bytes.
//
// VALIDATES: the MP_REACH_NLRI is rebuilt with the kept NLRI, AFI, SAFI and
// next hop unchanged and every length recomputed, beside a legacy NLRI field
// it leaves alone; an MP_UNREACH_NLRI loses the dropped withdrawal; a rebuilt
// MP_REACH_NLRI longer than 255 octets keeps the Extended Length flag (0x90)
// and a two-octet length. Each want is the body the text path's prefix-list
// answer built for the same drop (TestFilterModifyRewritesMPReach,
// TestFilterModifyMPReachAboveOneOctetLength), so both paths agree.
// PREVENTS: the MP_REACH_NLRI copied verbatim, so a prefix the import chain
// refused reached the RIB (plan/journal/guard-added-to-one-half-of-a-pair.md),
// and a rebuilt attribute written with a one-octet length that truncates it.
func TestLibraryNLRIDropRewritesMPSections(t *testing.T) {
	prefixes := func(skip int) string {
		var b strings.Builder
		for i := 1; i <= 40; i++ {
			if i != skip {
				b.WriteString(hex.EncodeToString([]byte{0x30, 0x20, 0x01, 0x0d, 0xb8, 0x00, byte(i)}))
			}
		}
		return b.String()
	}
	head := " 0002 01 10 20010db8000000000000000000000001 00 "
	tests := []struct {
		name string
		body string
		drop []nlriDrop
		want string
	}{
		{
			// IPv6 unicast: 2001:db8:1::/48 kept, 2001:db8:2::/48 (offset 7) dropped.
			name: "ipv6-unicast",
			body: "0000 0033 40010100 40020602010000FDE9" +
				" 800E23 0002 01 10 20010db8000000000000000000000001 00 3020010db80001 3020010db80002",
			drop: []nlriDrop{{policychain.NLRISectionMPReach, 7}},
			// RFC 7606 Section 5.1: the rebuilt MP_REACH_NLRI leads the block.
			want: "0000 002c 800E1c 0002 01 10 20010db8000000000000000000000001 00 3020010db80001" +
				" 40010100 40020602010000FDE9",
		},
		{
			// IPv4 unicast in MP_REACH_NLRI beside a legacy NLRI field: the
			// dropped 192.168.1.0/24 sits in the MP block only, and the legacy
			// 10.0.1.0/24 stays.
			name: "mp-ipv4-unicast-beside-legacy",
			body: "0000 0028 40010100 40020602010000FDE9 40030401010101" +
				" 800E11 0001 01 04 01010101 00 18c0a801 180a0002 180a0001",
			drop: []nlriDrop{{policychain.NLRISectionMPReach, 0}},
			want: "0000 0024 800E0d 0001 01 04 01010101 00 180a0002" +
				" 40010100 40020602010000FDE9 40030401010101 180a0001",
		},
		{
			// MP_UNREACH_NLRI: the withdrawal of 2001:db8:2::/48 is dropped.
			name: "mp-unreach-ipv6",
			body: "0000 0014 800F11 0002 01 3020010db80001 3020010db80002",
			drop: []nlriDrop{{policychain.NLRISectionMPUnreach, 7}},
			want: "0000 000d 800F0a 0002 01 3020010db80001",
		},
		{
			// 40 IPv6 /48 prefixes (a 301-octet value) less the first leave a
			// 294-octet value, written with flags 0x90 and length 0x0126.
			name: "mp-reach-above-one-octet-length",
			body: "0000 013e 40010100 40020602010000FDE9 900E012d" + head + prefixes(0),
			drop: []nlriDrop{{policychain.NLRISectionMPReach, 0}},
			want: "0000 0137 900E0126" + head + prefixes(1) + " 40010100 40020602010000FDE9",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			libraryIntentEval = dropAt(tt.drop...)
			res := runLibraryImport(t, nlriHex(t, tt.body))
			require.True(t, res.accept, "a drop the engine can apply accepts the route")
			require.NotNil(t, res.modifiedPayload, "the dropped entry must be removed from the payload")
			assert.Equal(t, strings.ReplaceAll(strings.ToLower(tt.want), " ", ""), hex.EncodeToString(res.modifiedPayload))
		})
	}
}

// TestLibraryNLRIDropRefusesNonCIDRFamily proves a drop from the section of a
// family the engine cannot frame entry by entry drops the route.
//
// VALIDATES: an ipv4/flow MP_REACH_NLRI from which a member drops the entry at
// offset 0 is refused by libraryNLRIOps with refuseNonCIDR, fail-closed,
// rather than cut as if it held prefixes or copied verbatim; and the import
// step over that UPDATE does not accept it.
// PREVENTS: a FlowSpec rule the import chain refused reaching the RIB, or a
// rule mangled by a CIDR-framed cut.
func TestLibraryNLRIDropRefusesNonCIDRFamily(t *testing.T) {
	// MP_REACH_NLRI ipv4/flow (AFI 1, SAFI 133), no next hop, one NLRI whose
	// destination component is 10.1.0.0/24.
	body := nlriHex(t, "0000 001b 40010100 40020602010000FDE9 800E0b 0001 85 00 00 05 01 18 0a0100")
	family.RegisterTestFamilies()

	var folded policychain.Sets
	require.NoError(t, folded.DropNLRI(policychain.NLRISectionMPReach, 0))
	var mods filterapi.ModAccumulator
	_, refusal, ok := libraryNLRIOps(newTestScratch(t), &folded, testWireUpdate(body), &mods)
	assert.False(t, ok, "a drop from a FlowSpec section must drop the route")
	assert.Equal(t, refuseNonCIDR, refusal.reason)
	assert.Zero(t, mods.Len(), "a refused drop records no rewrite")

	libraryIntentEval = dropAt(nlriDrop{policychain.NLRISectionMPReach, 0})
	res := runLibraryImport(t, body)
	assert.False(t, res.accept, "a drop the engine cannot apply must drop the route")
	assert.Nil(t, res.modifiedPayload)
}

// TestDropNLRIKeepsPathIDAndLabels proves the library drop copies each kept
// NLRI whole.
//
// VALIDATES: dropNLRI over an ADD-PATH section keeps each kept NLRI's path
// identifier, and of a prefix announced under two identifiers drops only the
// entry at the offset; a labeled (RFC 8277) announcement keeps its label stack
// and a labeled withdrawal its Compatibility field. The wants are the ones
// TestSubsetNLRIKeepsPathIDAndLabels asserted for the text path's subset.
// PREVENTS: a rewrite that re-encoded the prefix alone, losing the path
// identifier or the label.
func TestDropNLRIKeepsPathIDAndLabels(t *testing.T) {
	family.RegisterTestFamilies()
	labeled := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIMPLSLabel}
	tests := []struct {
		name     string
		fam      family.Family
		section  string
		addPath  bool
		withdraw bool
		drops    []uint16
		want     string
	}{
		{
			name: "add-path", fam: family.IPv4Unicast, addPath: true,
			section: "00000001 180a0000 00000002 180a0001 00000003 180a0000",
			drops:   []uint16{0},
			want:    "00000002180a0001 00000003180a0000",
		},
		{
			name: "labeled", fam: labeled,
			section: "30 000641 0a0000 30 000651 0a0100",
			drops:   []uint16{0},
			want:    "30000651 0a0100",
		},
		{
			name: "labeled-add-path", fam: labeled, addPath: true,
			section: "00000007 30 000641 0a0000 00000008 30 000651 0a0100",
			drops:   []uint16{11},
			want:    "00000007 30000641 0a0000",
		},
		{
			name: "labeled-withdrawal", fam: labeled, withdraw: true,
			section: "30 800000 0a0000 30 800000 0a0100",
			drops:   []uint16{0},
			want:    "30800000 0a0100",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := nlriSource{section: sectionMPReach, fam: tt.fam, raw: nlriHex(t, tt.section), addPath: tt.addPath, present: true}
			if tt.withdraw {
				source.section = sectionMPUnreach
			}
			got, ok := dropNLRI(newTestScratch(t), source, tt.drops)
			require.True(t, ok)
			assert.Equal(t, strings.ReplaceAll(tt.want, " ", ""), hex.EncodeToString(got))
		})
	}
}
