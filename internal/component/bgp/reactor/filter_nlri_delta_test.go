package reactor

import (
	"encoding/hex"
	"net/netip"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/family"
)

// dropPrefixFilter returns a seam that answers like a prefix-list filter: a
// modify whose NLRI value is the subject's, with every token in drop removed.
func dropPrefixFilter(drop ...string) PolicyFilterFunc {
	return func(_, _, _, _ string, _ uint32, update string) PolicyResponse {
		field := nlriValue(update)
		if field == "" {
			return PolicyResponse{Action: PolicyAccept}
		}
		kept := make([]string, 0, 8)
		for tok := range strings.FieldsSeq(field) {
			if !slices.Contains(drop, tok) {
				kept = append(kept, tok)
			}
		}
		return PolicyResponse{Action: PolicyModify, Delta: strings.Join(kept, " ")}
	}
}

// answerFilter returns a seam that answers every UPDATE with a modify whose
// NLRI value is nlri.
func answerFilter(nlri string) PolicyFilterFunc {
	return func(_, _, _, _ string, _ uint32, _ string) PolicyResponse {
		return PolicyResponse{Action: PolicyModify, Delta: nlri}
	}
}

// nlriValue returns the NLRI value of a filter text: everything from its first
// "nlri" or "mp" keyword, or "".
func nlriValue(text string) string {
	for i, tok := range strings.Fields(text) {
		if isNLRIKeyword(tok) {
			return strings.Join(strings.Fields(text)[i:], " ")
		}
	}
	return ""
}

// runImportDrop drives the real import chain over body with a filter that
// drops the given prefixes, and returns its result.
func runImportDrop(t *testing.T, body []byte, drop ...string) ingressStepResult {
	t.Helper()
	family.RegisterTestFamilies()
	addr := netip.MustParseAddr("10.0.0.9")
	peer := NewPeer(&PeerSettings{
		Address:      addr,
		LocalAS:      65000,
		PeerAS:       65001,
		ImportPolicy: mustPolicy(t, []filterapi.FilterRef{{Name: "prefix-list"}}, filterapi.FinalAccept),
	})
	r := &Reactor{
		api:              &pluginserver.Server{}, // non-nil: past the r.api guard
		policyFilterSeam: dropPrefixFilter(drop...),
		attrModHandlers:  attrModHandlersWithDefaults(),
	}
	return r.runIngressPolicyChain(peer, addr, 65001, testWireUpdate(body), body)
}

func nlriHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(strings.ReplaceAll(s, " ", ""))
	require.NoError(t, err)
	return b
}

// TestFilterModifyRewritesMPReach proves a modify answer that drops prefixes
// from an MP_REACH_NLRI block removes them from the UPDATE the RIB receives.
//
// VALIDATES: the MP block is rebuilt with the kept NLRI, AFI, SAFI and next hop
// unchanged and every length recomputed; asserted as wire hex.
// PREVENTS: the MP_REACH_NLRI copied verbatim, so a prefix the import filter
// refused reached the RIB (plan/journal/guard-added-to-one-half-of-a-pair.md).
func TestFilterModifyRewritesMPReach(t *testing.T) {
	tests := []struct {
		name string
		body string
		drop []string
		want string
	}{
		{
			// IPv6 unicast: 2001:db8:1::/48 kept, 2001:db8:2::/48 refused.
			name: "ipv6-unicast",
			body: "0000 0033 40010100 40020602010000FDE9" +
				" 800E23 0002 01 10 20010db8000000000000000000000001 00 3020010db80001 3020010db80002",
			drop: []string{"2001:db8:2::/48"},
			// RFC 7606 Section 5.1: the rebuilt MP_REACH_NLRI leads the block.
			want: "0000 002c 800E1c 0002 01 10 20010db8000000000000000000000001 00 3020010db80001" +
				" 40010100 40020602010000FDE9",
		},
		{
			// IPv4 unicast in MP_REACH_NLRI (RFC 4760) beside a legacy NLRI
			// field: the refused prefix sits in the MP block only, and the
			// legacy 10.0.1.0/24 stays.
			name: "mp-ipv4-unicast-beside-legacy",
			body: "0000 0028 40010100 40020602010000FDE9 40030401010101" +
				" 800E11 0001 01 04 01010101 00 18c0a801 180a0002 180a0001",
			drop: []string{"192.168.1.0/24"},
			want: "0000 0024 800E0d 0001 01 04 01010101 00 180a0002" +
				" 40010100 40020602010000FDE9 40030401010101 180a0001",
		},
		{
			// Legacy and MP blocks both trimmed in one UPDATE.
			name: "legacy-and-mp-ipv6",
			body: "0000 003a 40010100 40020602010000FDE9 40030401010101" +
				" 800E23 0002 01 10 20010db8000000000000000000000001 00 3020010db80001 3020010db80002" +
				" 180a0001 180a0009",
			drop: []string{"2001:db8:1::/48", "10.0.9.0/24"},
			want: "0000 0033 800E1c 0002 01 10 20010db8000000000000000000000001 00 3020010db80002" +
				" 40010100 40020602010000FDE9 40030401010101 180a0001",
		},
		{
			// Every MP prefix refused: the attribute goes, never an empty
			// MP_REACH_NLRI, and the legacy prefix stays.
			name: "mp-emptied-drops-attribute",
			body: "0000 0033 40010100 40020602010000FDE9 40030401010101" +
				" 800E1c 0002 01 10 20010db8000000000000000000000001 00 3020010db80001 180a0001",
			drop: []string{"2001:db8:1::/48"},
			want: "0000 0014 40010100 40020602010000FDE9 40030401010101 180a0001",
		},
		{
			// MP_UNREACH_NLRI: the withdrawal of a refused prefix is dropped.
			name: "mp-unreach-ipv6",
			body: "0000 0014 800F11 0002 01 3020010db80001 3020010db80002",
			drop: []string{"2001:db8:2::/48"},
			want: "0000 000d 800F0a 0002 01 3020010db80001",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := runImportDrop(t, nlriHex(t, tt.body), tt.drop...)
			require.True(t, res.accept, "a modify the engine can apply accepts the route")
			require.NotNil(t, res.modifiedPayload, "the refused prefix must be removed from the payload")
			assert.Equal(t, strings.ReplaceAll(strings.ToLower(tt.want), " ", ""), hex.EncodeToString(res.modifiedPayload))
		})
	}
}

// TestFilterModifyRefusesNonCIDRFamily proves a modify answer that changes the
// block of a family the engine cannot rewrite drops the route.
//
// VALIDATES: an ipv4/flow block whose destination prefix the filter removed is
// refused, fail-closed, rather than the MP_REACH_NLRI being copied verbatim.
// PREVENTS: a FlowSpec rule the import filter refused reaching the RIB.
func TestFilterModifyRefusesNonCIDRFamily(t *testing.T) {
	// MP_REACH_NLRI ipv4/flow (AFI 1, SAFI 133), no next hop, one NLRI whose
	// destination component is 10.1.0.0/24.
	body := nlriHex(t, "0000 001b 40010100 40020602010000FDE9 800E0b 0001 85 00 00 05 01 18 0a0100")
	res := runImportDrop(t, body, "10.1.0.0/24")
	assert.False(t, res.accept, "a modify the engine cannot apply must drop the route")
	assert.Nil(t, res.modifiedPayload)
}

// TestFilterModifyRefusesAddedMPPrefix proves an answer may only remove: a
// prefix the MP section does not carry is refused, never encoded.
func TestFilterModifyRefusesAddedMPPrefix(t *testing.T) {
	family.RegisterTestFamilies()
	body := nlriHex(t, "0000 0033 40010100 40020602010000FDE9"+
		" 800E23 0002 01 10 20010db8000000000000000000000001 00 3020010db80001 3020010db80002")
	text := string(AppendUpdateForFilter(nil, nil, testWireUpdate(body), nil))
	require.Equal(t, "mp ipv6/unicast add 2001:db8:1::/48 2001:db8:2::/48", text)

	var mods filterapi.ModAccumulator
	values := acquireValueScratch()
	defer releaseValueScratch(values)
	_, refusal, ok := sectionNLRIDelta(values, text, "mp ipv6/unicast add 2001:db8:3::/48", testWireUpdate(body), &mods)
	assert.False(t, ok)
	assert.Equal(t, "ipv6/unicast", refusal.family)
	assert.Equal(t, refuseNotSubset, refusal.reason)
}

// legacyBody is an UPDATE whose legacy NLRI carries 10.0.0.0/24, 10.0.1.0/24
// and 192.168.1.0/24, rendered "nlri add 10.0.0.0/24 10.0.1.0/24 192.168.1.0/24".
const legacyBody = "0000 0014 40010100 40020602010000FDE9 40030401010101 180a0000 180a0001 18c0a801"

// TestSectionNLRIDeltaLegacyAnnounce proves the legacy announce block is held
// to the same rule as every other section: an answer may only remove prefixes,
// in order, and anything else drops the route.
//
// VALIDATES: the override is the subset of the source NLRI the answer kept, an
// emptied block yields an empty non-nil override, and an added, widened,
// reordered or unparsable prefix, a duplicate block, the retired
// "nlri <family>" form and an mp block with no section are each refused.
// PREVENTS: the legacy block encoded from the answer's own tokens, which let a
// filter ADD a prefix the peer never sent, dropped every ADD-PATH path
// identifier, and copied the source verbatim on a token that did not parse.
func TestSectionNLRIDeltaLegacyAnnounce(t *testing.T) {
	family.RegisterTestFamilies()
	body := nlriHex(t, legacyBody)
	subject := string(AppendUpdateForFilter(nil, nil, testWireUpdate(body), nil))
	require.Equal(t, "nlri add 10.0.0.0/24 10.0.1.0/24 192.168.1.0/24", subject)

	tests := []struct {
		name     string
		modified string
		want     string // hex of the override; "nil" means nil
		refusal  string // "" means accepted
	}{
		{name: "unchanged", modified: subject, want: "nil"},
		{name: "subset", modified: "nlri add 10.0.0.0/24 192.168.1.0/24", want: "180a0000 18c0a801"},
		{name: "every-prefix-refused", modified: "origin igp", want: ""},
		{name: "empty-block", modified: "nlri add", want: ""},
		{name: "added-default", modified: "nlri add 0.0.0.0/0", refusal: refuseNotSubset},
		{name: "widened", modified: "nlri add 10.0.0.0/20", refusal: refuseNotSubset},
		{name: "reordered", modified: "nlri add 192.168.1.0/24 10.0.0.0/24", refusal: refuseNotSubset},
		{name: "not-a-prefix", modified: "nlri add not-a-prefix", refusal: refuseNotSubset},
		{name: "ipv6-in-legacy", modified: "nlri add 2001:db8::/32", refusal: refuseNotSubset},
		{name: "duplicate-block", modified: "nlri add 10.0.0.0/24 nlri add 10.0.1.0/24", refusal: refuseDuplicate},
		{name: "retired-family-form", modified: "nlri ipv4/unicast add 10.0.0.0/24", refusal: refuseSyntax},
		{name: "mp-without-section", modified: subject + " mp ipv6/unicast add 2001:db8::/32", refusal: refuseNoSection},
		// Nine blocks overflow the table of maxNLRIBlocks (eight): the answer is
		// refused as a whole before any block is read, not judged on its first eight.
		{name: "more-blocks-than-table", modified: strings.Repeat("nlri del ", maxNLRIBlocks+1), refusal: refuseSyntax},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var mods filterapi.ModAccumulator
			override, refusal, ok := sectionNLRIDelta(newTestScratch(t), subject, tt.modified, testWireUpdate(body), &mods)
			if tt.refusal != "" {
				assert.False(t, ok, "the answer must be refused")
				assert.Equal(t, tt.refusal, refusal.reason)
				return
			}
			require.True(t, ok, "refused: %s", refusal.reason)
			if tt.want == "nil" {
				assert.Nil(t, override)
				return
			}
			require.NotNil(t, override, "a changed block yields a non-nil override")
			assert.Equal(t, strings.ReplaceAll(tt.want, " ", ""), hex.EncodeToString(override))
		})
	}
}

// TestSectionNLRIDeltaLegacyWithdrawn proves an answer that drops a prefix from
// the "nlri del" block rewrites the legacy Withdrawn Routes, and no other
// section.
func TestSectionNLRIDeltaLegacyWithdrawn(t *testing.T) {
	family.RegisterTestFamilies()
	body := nlriHex(t, "0008 180a0000 180a0001 0000")
	subject := string(AppendUpdateForFilter(nil, nil, testWireUpdate(body), nil))
	require.Equal(t, "nlri del 10.0.0.0/24 10.0.1.0/24", subject)

	var mods filterapi.ModAccumulator
	override, refusal, ok := sectionNLRIDelta(newTestScratch(t), subject, "nlri del 10.0.1.0/24", testWireUpdate(body), &mods)
	require.True(t, ok, "refused: %s", refusal.reason)
	assert.Nil(t, override, "the legacy announce section is untouched")
	assert.Equal(t, "180a0001", hex.EncodeToString(mods.WithdrawnRewrite()))
	assert.Equal(t, 0, mods.Len(), "no attribute operation")

	_, refusal, ok = sectionNLRIDelta(newTestScratch(t), subject, "nlri del 10.0.2.0/24", testWireUpdate(body), &mods)
	assert.False(t, ok, "a withdrawal the source never carried is refused")
	assert.Equal(t, refuseNotSubset, refusal.reason)
}

// TestFilterModifyRefusesReorderedMPAnswer proves the order of the subject is
// part of the contract on an MP section too, through the real import chain.
func TestFilterModifyRefusesReorderedMPAnswer(t *testing.T) {
	body := nlriHex(t, "0000 0033 40010100 40020602010000FDE9"+
		" 800E23 0002 01 10 20010db8000000000000000000000001 00 3020010db80001 3020010db80002")
	family.RegisterTestFamilies()
	addr := netip.MustParseAddr("10.0.0.9")
	peer := NewPeer(&PeerSettings{Address: addr, LocalAS: 65000, PeerAS: 65001, ImportPolicy: mustPolicy(t, []filterapi.FilterRef{{Name: "prefix-list"}}, filterapi.FinalAccept)})
	r := &Reactor{
		api:              &pluginserver.Server{},
		policyFilterSeam: answerFilter("mp ipv6/unicast add 2001:db8:2::/48 2001:db8:1::/48"),
		attrModHandlers:  attrModHandlersWithDefaults(),
	}
	res := r.runIngressPolicyChain(peer, addr, 65001, testWireUpdate(body), body)
	assert.False(t, res.accept, "a reordered answer must drop the route")
	assert.Nil(t, res.modifiedPayload)
}

// TestExportFilterModifyRefusalSuppresses proves the export chain fails closed
// on an answer the engine cannot apply, and reports it as a failure rather than
// a policy decision.
//
// VALIDATES: an export answer adding a prefix the UPDATE does not carry yields
// accept == false and failed == true (filter_ordered.go).
// PREVENTS: the route sent to the peer with the prefixes the export policy
// refused, or the drop counted as a deliberate reject.
func TestExportFilterModifyRefusalSuppresses(t *testing.T) {
	family.RegisterTestFamilies()
	body := nlriHex(t, legacyBody)
	r := &Reactor{
		api:              &pluginserver.Server{},
		policyFilterSeam: answerFilter("nlri add 10.0.0.0/24 172.16.0.0/24"),
		attrModHandlers:  attrModHandlersWithDefaults(),
	}
	refs := []filterapi.FilterRef{{Name: "prefix-list"}}
	got := r.runEgressPolicyChainASN4(mustPolicy(t, refs, filterapi.FinalAccept), "10.0.0.2", 65002, 65000, false, testWireUpdate(body), true)
	assert.False(t, got.accept, "fail-closed: the route is withheld")
	assert.True(t, got.failed, "the step must report that it could not apply the answer")
}

// TestSubsetNLRIKeepsPathIDAndLabels proves the rewritten section is carved
// from the source NLRI, so what rides beside the prefix survives.
//
// VALIDATES: an ADD-PATH section keeps each kept NLRI's path identifier and a
// prefix announced twice under two identifiers is matched in order; a labeled
// (RFC 8277) announcement keeps its label stack and a labeled withdrawal its
// Compatibility field; the subject renders the prefix, never the label bits.
// PREVENTS: a rewrite that re-encoded the prefix alone, losing the path
// identifier or the label, or a subject that read label bits as prefix bits.
func TestSubsetNLRIKeepsPathIDAndLabels(t *testing.T) {
	family.RegisterTestFamilies()
	labeled := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIMPLSLabel}
	tests := []struct {
		name     string
		fam      family.Family
		section  string
		addPath  bool
		withdraw bool
		subject  string
		keep     string
		want     string
	}{
		{
			name: "add-path", fam: family.IPv4Unicast, addPath: true,
			section: "00000001 180a0000 00000002 180a0001 00000003 180a0000",
			subject: " 10.0.0.0/24 10.0.1.0/24 10.0.0.0/24",
			keep:    "10.0.1.0/24 10.0.0.0/24",
			want:    "00000002180a0001 00000003180a0000",
		},
		{
			name: "labeled", fam: labeled,
			section: "30 000641 0a0000 30 000651 0a0100",
			subject: " 10.0.0.0/24 10.1.0.0/24",
			keep:    "10.1.0.0/24",
			want:    "30000651 0a0100",
		},
		{
			name: "labeled-add-path", fam: labeled, addPath: true,
			section: "00000007 30 000641 0a0000 00000008 30 000651 0a0100",
			subject: " 10.0.0.0/24 10.1.0.0/24",
			keep:    "10.0.0.0/24",
			want:    "00000007 30000641 0a0000",
		},
		{
			name: "labeled-withdrawal", fam: labeled, withdraw: true,
			section: "30 800000 0a0000 30 800000 0a0100",
			subject: " 10.0.0.0/24 10.1.0.0/24",
			keep:    "10.1.0.0/24",
			want:    "30800000 0a0100",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			section := nlriHex(t, tt.section)
			assert.Equal(t, tt.subject, string(appendCIDRPrefixes(nil, tt.fam, section, tt.addPath, tt.withdraw)))
			got, ok := subsetNLRI(newTestScratch(t), tt.fam, section, tt.addPath, tt.withdraw, tt.keep)
			require.True(t, ok)
			assert.Equal(t, strings.ReplaceAll(tt.want, " ", ""), hex.EncodeToString(got))
		})
	}
}

// TestFilterModifyMPReachAboveOneOctetLength proves a rewritten MP_REACH_NLRI
// longer than 255 octets keeps the Extended Length flag and a two-octet length.
//
// VALIDATES: 40 IPv6 /48 prefixes (301-octet value) trimmed by one leave a
// 294-octet value written with flags 0x90 and length 0x0126, every other octet
// of the UPDATE unchanged.
// PREVENTS: the rebuilt attribute written with a one-octet length that
// truncates it, which a peer reads as a malformed UPDATE.
func TestFilterModifyMPReachAboveOneOctetLength(t *testing.T) {
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
	body := "0000 013e 40010100 40020602010000FDE9 900E012d" + head + prefixes(0)
	// RFC 7606 Section 5.1: the rebuilt MP_REACH_NLRI leads the block.
	want := "0000 0137 900E0126" + head + prefixes(1) + " 40010100 40020602010000FDE9"

	res := runImportDrop(t, nlriHex(t, body), "2001:db8:1::/48")
	require.True(t, res.accept)
	require.NotNil(t, res.modifiedPayload)
	assert.Equal(t, strings.ReplaceAll(strings.ToLower(want), " ", ""), hex.EncodeToString(res.modifiedPayload))
}
