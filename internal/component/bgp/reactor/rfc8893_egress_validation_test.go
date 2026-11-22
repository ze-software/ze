// RFC 8893 conformance tests: RPKI Origin Validation for BGP Export.
//
// Related: filter_ordered.go -- runEgressPolicyChainASN4
// Related: filter_library.go -- runPostEgressRPKIValidation
// Related: stale_readvertise_chain.go -- runStaleChain
// Related: policy_dryrun_library.go -- traceLibraryChain
// Related: ../policyview/rpki_origin.go -- View.EffectiveOriginAS
// Related: ../policychain/match_rpki.go -- compileBGPRPKIValidationState

package reactor

import (
	"net/netip"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"
	ribpolicy "github.com/ze-software/ze/internal/core/rib/policy"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

const (
	rfc8893ImportAttachment = "bgp import"
	rfc8893ExportAttachment = "bgp export"
)

type rfc8893VRP struct {
	prefix netip.Prefix
	maxLen int
	asn    uint32
}

// installRFC8893Validator installs an RFC 6811 Section 2 VRP validator on the
// shared ribpolicy seam for the duration of t, recording every (prefix, originAS)
// query it evaluates.
func installRFC8893Validator(t *testing.T, vrps []rfc8893VRP) *[]struct {
	prefix   netip.Prefix
	originAS uint32
} {
	t.Helper()
	var mu sync.Mutex
	var queries []struct {
		prefix   netip.Prefix
		originAS uint32
	}
	ribpolicy.SetOriginValidator(func(prefix netip.Prefix, originAS uint32) ribpolicy.ValidationState {
		mu.Lock()
		queries = append(queries, struct {
			prefix   netip.Prefix
			originAS uint32
		}{prefix: prefix, originAS: originAS})
		mu.Unlock()

		covered := false
		for _, vrp := range vrps {
			if vrp.prefix.Addr().Is4() != prefix.Addr().Is4() {
				continue
			}
			if prefix.Bits() < vrp.prefix.Bits() {
				continue
			}
			if !vrp.prefix.Contains(prefix.Addr()) {
				continue
			}
			covered = true
			if originAS != ribpolicy.OriginNone && prefix.Bits() <= vrp.maxLen && vrp.asn == originAS && vrp.asn != 0 {
				return ribpolicy.ValidationValid
			}
		}
		if covered {
			return ribpolicy.ValidationInvalid
		}
		return ribpolicy.ValidationNotFound
	})
	t.Cleanup(func() { ribpolicy.SetOriginValidator(nil) })
	return &queries
}

// compileRFC8893ExportChain builds a real BGP export policychain.Chain against
// the real "bgp export" attachment from policy registry instances and optional
// external members.
func compileRFC8893ExportChain(t *testing.T, reg *policy.Registry, members []policychain.Member, final policy.Final) filterapi.Policy {
	t.Helper()
	chain, err := policychain.Compile(rfc8893ExportAttachment, reg, members, final, policychain.Bases{})
	require.NoError(t, err)
	refs := make([]policy.Ref, len(members))
	for i, m := range members {
		name := m.Name
		if m.Plugin != "" {
			name = m.Plugin + ":" + m.Name
		}
		refs[i] = policy.Ref{Name: name}
	}
	return mustPolicy(t, refs, final).WithLibrary(chain)
}

func rfc8893IPv4Update(t *testing.T, asPath []byte, nlri ...[]byte) (*wireu.WireUpdate, bgpctx.ContextID) {
	t.Helper()
	ctx := bgpctx.NewEncodingContext(
		&capability.PeerIdentity{LocalASN: 65000, PeerASN: 65001},
		&capability.EncodingCaps{ASN4: true},
		bgpctx.DirectionRecv,
	)
	ctxID, err := bgpctx.Registry.Register(ctx)
	require.NoError(t, err)
	attrs := makeAttr(0x40, byte(attribute.AttrOrigin), []byte{0})
	attrs = append(attrs, makeAttr(0x40, byte(attribute.AttrASPath), asPath)...)
	attrs = append(attrs, makeAttr(0x40, byte(attribute.AttrNextHop), []byte{192, 0, 2, 254})...)
	var nlriBytes []byte
	for _, n := range nlri {
		nlriBytes = append(nlriBytes, n...)
	}
	body := buildUpdatePayload(attrs, nlriBytes)
	return wireu.NewWireUpdate(body, ctxID), ctxID
}

func rfc8893RemovePrivateTransport() *externalTransport {
	return &externalTransport{
		reads: []string{"as-path"},
		sets:  []string{"as-path"},
		reply: rpc.FilterUpdateOutput{
			Action: rpc.FilterModify,
			Sets:   []rpc.FilterSet{intentSet("as-path", rpc.FilterSetActionRemove, `"private"`)},
		},
	}
}

func rfc8893RemovePrivatePeerASTransport() *externalTransport {
	return &externalTransport{
		reads: []string{"as-path"},
		sets:  []string{"as-path"},
		reply: rpc.FilterUpdateOutput{
			Action: rpc.FilterModify,
			Sets:   []rpc.FilterSet{intentSet("as-path", rpc.FilterSetActionRemove, `"private-peer-as"`)},
		},
	}
}

// RFC requirement: RFC8893-1-1 positive -- "A validating BGP speaker MUST apply
// Route Origin Validation policy semantics (see Section 2 of [RFC6811] and
// Section 4 of [RFC8481]) after applying any egress configuration and policy."
// Verifies post-egress validation across:
//  1. Export chain ordering (remove-private-as runs before rpki-validation-state
//     even when listed after it).
//  2. Live forwarding with as-override egress configuration (real
//     peer.refreshForwardFacts and live Session socket write).
//  3. Multi-prefix UPDATE dropping only the Invalid prefix.
//  4. Stale re-advertisement (runStaleChain with judgeAll=true).
//  5. Policy dry-run (traceLibraryChain on export).
//  6. SessionFacts and captureOriginatedFacts carrying ASOverride and SecondaryAS.
func TestRFC8893EgressValidationAppliesAfterEgressConfigAndPolicy(t *testing.T) {
	installRFC8893Validator(t, []rfc8893VRP{
		{prefix: netip.MustParsePrefix("198.51.100.0/24"), maxLen: 24, asn: 3356},
		{prefix: netip.MustParsePrefix("203.0.113.0/24"), maxLen: 24, asn: 65000},
		{prefix: netip.MustParsePrefix("192.0.2.0/24"), maxLen: 24, asn: 64496},
	})

	reg, err := policy.NewRegistry(policy.Instance{
		Name:   "drop-invalid",
		Type:   policy.MatchFilterType,
		Config: map[string]any{"rpki-validation-state": []any{"invalid"}, "action": "reject"},
	})
	require.NoError(t, err)

	t.Run("remove-private-as runs before RPKI validation even when listed after it", func(t *testing.T) {
		r := &Reactor{attrModHandlers: attrModHandlersWithDefaults(), filterTransportSeam: rfc8893RemovePrivateTransport()}

		// Intentionally place drop-invalid BEFORE probe:f (remove-private-as) to prove
		// export RPKI validation always runs after non-validating egress policy.
		expPol := compileRFC8893ExportChain(t, reg, []policychain.Member{
			{Name: "drop-invalid"},
			{Plugin: "probe", Name: "f"},
		}, policy.FinalAccept)

		// Public AS 3356 + private AS 65500: remove-private-as strips 65500 leaving [3356] (Valid).
		wu, _ := rfc8893IPv4Update(t, asSequence4(3356, 65500), []byte{24, 198, 51, 100})
		dest := (&PeerSettings{Address: netip.MustParseAddr("10.0.0.2"), LocalAS: 65000, PeerAS: 65002}).SessionFacts(65002)
		session := policyview.ExportSession(dest, filterapi.PeerFilterInfo{}, false, filterapi.ExportExplicit)

		res := r.runEgressPolicyChainASN4(expPol, "10.0.0.2", 65002, 65000, false, session, wu, true)
		require.True(t, res.accept, "route whose private origin 65500 is stripped to authorized 3356 must be accepted")
		require.NotNil(t, res.wireOverride, "remove-private-as rewrite must be preserved in wireOverride")
		u, err := message.UnpackUpdate(res.wireOverride.Payload())
		require.NoError(t, err)
		_, _, rawAS, found := attribute.AttrFind(u.PathAttributes, attribute.AttrASPath)
		require.True(t, found)
		parsed, err := attribute.ParseASPath(rawAS, true)
		require.NoError(t, err)
		require.Equal(t, []uint32{3356}, flatASNs(parsed))
	})

	t.Run("live forward with as-override applies egress config before RPKI validation", func(t *testing.T) {
		f := newAIGPReplayFixture(t, nil)
		f.r.orderedEgressSteps = buildOrderedEgressSteps()
		peer := f.destination
		peer.settings.LocalAS = 65000
		peer.settings.PeerAS = 65002
		peer.settings.ASOverride = true
		peer.settings.NextHopMode = NextHopUnchanged
		peer.settings.ExportPolicy = compileRFC8893ExportChain(t, reg, []policychain.Member{{Name: "drop-invalid"}}, policy.FinalAccept)
		peer.refreshForwardFacts()

		// Received origin AS is 65002 (Invalid for 203.0.113.0/24), but as-override
		// rewrites PeerAS 65002 -> LocalAS 65000 (Valid for 203.0.113.0/24).
		attrs := makeAttr(0x40, byte(attribute.AttrOrigin), []byte{0})
		attrs = append(attrs, makeAttr(0x40, byte(attribute.AttrASPath), asSequence4(65001, 65002))...)
		attrs = append(attrs, makeAttr(0x40, byte(attribute.AttrNextHop), []byte{192, 0, 2, 254})...)
		body := buildUpdatePayload(attrs, []byte{24, 203, 0, 113})

		id := f.receive(t, body)
		f.forward(t, id)
		forwardSocketBarrier(t, f.r)

		bodies := aigpSocketBodies(t, f.conn)
		require.Len(t, bodies, 1, "as-override changing effective origin 65002 -> 65000 (Valid) must be advertised")
	})

	t.Run("multi-prefix UPDATE drops only Invalid prefix after egress policy", func(t *testing.T) {
		r := &Reactor{attrModHandlers: attrModHandlersWithDefaults()}
		expPol := compileRFC8893ExportChain(t, reg, []policychain.Member{{Name: "drop-invalid"}}, policy.FinalAccept)

		// 198.51.100.0/24 is Valid for AS 3356; 192.0.2.0/24 is Invalid for AS 3356 (VRP is 64496).
		wu, _ := rfc8893IPv4Update(t, asSequence4(3356), []byte{24, 198, 51, 100}, []byte{24, 192, 0, 2})
		dest := (&PeerSettings{Address: netip.MustParseAddr("10.0.0.2"), LocalAS: 65000, PeerAS: 65002}).SessionFacts(65002)
		session := policyview.ExportSession(dest, filterapi.PeerFilterInfo{}, false, filterapi.ExportExplicit)

		res := r.runEgressPolicyChainASN4(expPol, "10.0.0.2", 65002, 65000, false, session, wu, true)
		require.True(t, res.accept)
		require.NotNil(t, res.wireOverride)
		u, err := message.UnpackUpdate(res.wireOverride.Payload())
		require.NoError(t, err)
		require.Equal(t, []byte{24, 198, 51, 100}, u.NLRI, "only Valid prefix 198.51.100.0/24 may remain")
	})

	t.Run("stale readvertise chain runs RPKI validation after non-validating egress policy", func(t *testing.T) {
		r := &Reactor{attrModHandlers: attrModHandlersWithDefaults(), filterTransportSeam: rfc8893RemovePrivateTransport()}
		expPol := compileRFC8893ExportChain(t, reg, []policychain.Member{
			{Name: "drop-invalid"},
			{Plugin: "probe", Name: "f"},
		}, policy.FinalAccept)
		wu, ctxID := rfc8893IPv4Update(t, asSequence4(3356, 65500), []byte{24, 198, 51, 100})
		dest := (&PeerSettings{Address: netip.MustParseAddr("10.0.0.2"), LocalAS: 65000, PeerAS: 65002}).SessionFacts(65002)

		res, ran := r.runStaleChain(staleChain{policy: expPol, ctxID: ctxID, asn4: true, judgeAll: true}, dest, wu.Payload(), 1)
		require.True(t, ran)
		require.Equal(t, policychain.OutcomeAccept, res.outcome.Action)
		require.False(t, res.failed)
		require.NotNil(t, res.modifiedPayload, "stale readvertise must preserve remove-private-as rewrite")
	})

	t.Run("policy dry-run traces and runs RPKI validation after non-validating egress policy", func(t *testing.T) {
		r := &Reactor{attrModHandlers: attrModHandlersWithDefaults(), filterTransportSeam: rfc8893RemovePrivateTransport()}
		expPol := compileRFC8893ExportChain(t, reg, []policychain.Member{
			{Name: "drop-invalid"},
			{Plugin: "probe", Name: "f"},
		}, policy.FinalAccept)
		wu, ctxID := rfc8893IPv4Update(t, asSequence4(3356, 65500), []byte{24, 198, 51, 100})
		dest := (&PeerSettings{Address: netip.MustParseAddr("10.0.0.2"), LocalAS: 65000, PeerAS: 65002}).SessionFacts(65002)
		before, err := displayUpdate(wu)
		require.NoError(t, err)
		dry := &libraryDryRun{
			wireUpdate: wu,
			payload:    wu.Payload(),
			ctxID:      ctxID,
			direction:  directionExport,
			peer:       "10.0.0.2",
			session:    policyview.ExportSession(dest, filterapi.PeerFilterInfo{}, false, filterapi.ExportExplicit),
			intent:     libraryIntentContext{asn4: true, localAS: 65000, peerAS: 65002, asPathEditable: true},
		}

		got, _, trace, _ := r.traceLibraryChain(expPol.Library(), dry, &before)
		require.Equal(t, PolicyAccept, got.Action, "dry-run export must accept after remove-private-as yields Valid 3356")
		require.Len(t, trace, 2)
		require.Equal(t, "probe:f", trace[0].Canonical, "non-validating egress policy must trace before RPKI validation")
		require.Equal(t, "drop-invalid", trace[1].Canonical, "rpki-validation-state member must trace after egress policy")
	})

	t.Run("SessionFacts and captureOriginatedFacts populate ASOverride and SecondaryAS", func(t *testing.T) {
		ps := &PeerSettings{
			Address:       netip.MustParseAddr("10.0.0.2"),
			LocalAS:       65200,
			GlobalLocalAS: 65100,
			PeerAS:        65002,
			ASOverride:    true,
		}
		sf := ps.SessionFacts(65002)
		require.True(t, sf.ASOverride)
		require.Equal(t, uint32(65100), sf.SecondaryAS)

		p := NewPeer(ps)
		var captured peerForwardFacts
		p.captureOriginatedFacts(&captured)
		require.True(t, captured.asOverride)
		require.Equal(t, uint32(65100), captured.secondaryAS)
		require.True(t, captured.filterInfo.ASOverride)
		require.Equal(t, uint32(65100), captured.filterInfo.SecondaryAS)
	})
}

// RFC requirement: RFC8893-1-1 negative -- validation AFTER egress configuration
// and policy rejects routes that become Invalid once egress policy or
// configuration runs (across live forwarding, runEgressPolicyChainASN4,
// runStaleChain, and traceLibraryChain), even if their pre-egress origin AS was
// Valid, and rejects AS_SET (OriginNone) routes covered by a VRP per RFC 6811
// Section 2 / RFC 8481 Section 4.
func TestRFC8893EgressValidationRejectsRouteInvalidAfterEgressConfigAndPolicy(t *testing.T) {
	installRFC8893Validator(t, []rfc8893VRP{
		{prefix: netip.MustParsePrefix("198.51.100.0/24"), maxLen: 24, asn: 65500},
		{prefix: netip.MustParsePrefix("203.0.113.0/24"), maxLen: 24, asn: 65002},
	})

	reg, err := policy.NewRegistry(policy.Instance{
		Name:   "drop-invalid",
		Type:   policy.MatchFilterType,
		Config: map[string]any{"rpki-validation-state": []any{"invalid"}, "action": "reject"},
	})
	require.NoError(t, err)

	t.Run("remove-private-as turning Valid origin into Invalid rejects", func(t *testing.T) {
		r := &Reactor{attrModHandlers: attrModHandlersWithDefaults(), filterTransportSeam: rfc8893RemovePrivateTransport()}

		expPol := compileRFC8893ExportChain(t, reg, []policychain.Member{
			{Name: "drop-invalid"},
			{Plugin: "probe", Name: "f"},
		}, policy.FinalAccept)

		// Pre-egress origin 65500 is Valid, but after stripping 65500 the
		// effective origin is public AS 64497 which is Invalid.
		wu, _ := rfc8893IPv4Update(t, asSequence4(64497, 65500), []byte{24, 198, 51, 100})
		dest := (&PeerSettings{Address: netip.MustParseAddr("10.0.0.2"), LocalAS: 65000, PeerAS: 65002}).SessionFacts(65002)
		session := policyview.ExportSession(dest, filterapi.PeerFilterInfo{}, false, filterapi.ExportExplicit)

		res := r.runEgressPolicyChainASN4(expPol, "10.0.0.2", 65002, 65000, false, session, wu, true)
		require.False(t, res.accept, "route whose post-policy origin AS 64497 is Invalid must be rejected")
	})

	t.Run("live forward with as-override turning Valid origin into Invalid rejects", func(t *testing.T) {
		f := newAIGPReplayFixture(t, nil)
		f.r.orderedEgressSteps = buildOrderedEgressSteps()
		peer := f.destination
		peer.settings.LocalAS = 65000
		peer.settings.PeerAS = 65002
		peer.settings.ASOverride = true
		peer.settings.NextHopMode = NextHopUnchanged
		peer.settings.ExportPolicy = compileRFC8893ExportChain(t, reg, []policychain.Member{{Name: "drop-invalid"}}, policy.FinalAccept)
		peer.refreshForwardFacts()

		// Pre-egress origin 65002 is Valid, but as-override rewrites it to 65000 (Invalid).
		attrs := makeAttr(0x40, byte(attribute.AttrOrigin), []byte{0})
		attrs = append(attrs, makeAttr(0x40, byte(attribute.AttrASPath), asSequence4(65001, 65002))...)
		attrs = append(attrs, makeAttr(0x40, byte(attribute.AttrNextHop), []byte{192, 0, 2, 254})...)
		body := buildUpdatePayload(attrs, []byte{24, 203, 0, 113})

		id := f.receive(t, body)
		f.forward(t, id)
		forwardSocketBarrier(t, f.r)

		require.Empty(t, aigpSocketBodies(t, f.conn), "as-override rewriting Valid 65002 to unauthorized 65000 must not be advertised")
	})

	t.Run("AS_SET trailing segment yields OriginNone and Invalid under covering VRP", func(t *testing.T) {
		r := &Reactor{attrModHandlers: attrModHandlersWithDefaults()}
		expPol := compileRFC8893ExportChain(t, reg, []policychain.Member{{Name: "drop-invalid"}}, policy.FinalAccept)

		// AS_SET {65002} is OriginNone per RFC 6811 Section 2, so Invalid when covered.
		asSet := []byte{byte(attribute.ASSet), 1, 0, 0, 0xFD, 0xEA}
		wu, _ := rfc8893IPv4Update(t, asSet, []byte{24, 203, 0, 113})
		dest := (&PeerSettings{Address: netip.MustParseAddr("10.0.0.2"), LocalAS: 65000, PeerAS: 65003}).SessionFacts(65003)
		session := policyview.ExportSession(dest, filterapi.PeerFilterInfo{}, false, filterapi.ExportExplicit)

		res := r.runEgressPolicyChainASN4(expPol, "10.0.0.2", 65003, 65000, false, session, wu, true)
		require.False(t, res.accept, "AS_SET origin NONE covered by VRP must be rejected as Invalid")
	})

	t.Run("stale readvertise chain rejects route turned Invalid by egress policy", func(t *testing.T) {
		r := &Reactor{attrModHandlers: attrModHandlersWithDefaults(), filterTransportSeam: rfc8893RemovePrivateTransport()}
		expPol := compileRFC8893ExportChain(t, reg, []policychain.Member{
			{Name: "drop-invalid"},
			{Plugin: "probe", Name: "f"},
		}, policy.FinalAccept)
		wu, ctxID := rfc8893IPv4Update(t, asSequence4(64497, 65500), []byte{24, 198, 51, 100})
		dest := (&PeerSettings{Address: netip.MustParseAddr("10.0.0.2"), LocalAS: 65000, PeerAS: 65002}).SessionFacts(65002)

		res, ran := r.runStaleChain(staleChain{policy: expPol, ctxID: ctxID, asn4: true, judgeAll: true}, dest, wu.Payload(), 1)
		require.True(t, ran)
		require.Equal(t, policychain.OutcomeReject, res.outcome.Action)
	})

	t.Run("policy dry-run rejects route turned Invalid by egress policy", func(t *testing.T) {
		r := &Reactor{attrModHandlers: attrModHandlersWithDefaults(), filterTransportSeam: rfc8893RemovePrivateTransport()}
		expPol := compileRFC8893ExportChain(t, reg, []policychain.Member{
			{Name: "drop-invalid"},
			{Plugin: "probe", Name: "f"},
		}, policy.FinalAccept)
		wu, ctxID := rfc8893IPv4Update(t, asSequence4(64497, 65500), []byte{24, 198, 51, 100})
		dest := (&PeerSettings{Address: netip.MustParseAddr("10.0.0.2"), LocalAS: 65000, PeerAS: 65002}).SessionFacts(65002)
		before, err := displayUpdate(wu)
		require.NoError(t, err)
		dry := &libraryDryRun{
			wireUpdate: wu,
			payload:    wu.Payload(),
			ctxID:      ctxID,
			direction:  directionExport,
			peer:       "10.0.0.2",
			session:    policyview.ExportSession(dest, filterapi.PeerFilterInfo{}, false, filterapi.ExportExplicit),
			intent:     libraryIntentContext{asn4: true, localAS: 65000, peerAS: 65002, asPathEditable: true},
		}

		got, _, _, _ := r.traceLibraryChain(expPol.Library(), dry, &before)
		require.Equal(t, PolicyReject, got.Action)
	})
}

// RFC requirement: RFC8893-1-2 positive -- "Any AS_PATH modifications resulting
// in effective origin AS change MUST be taken into account." Covers all five
// effective-origin-changing mechanisms: remove-private-as strip to empty
// (LocalAS becomes origin), remove-private-as peer-as replacement (PeerAS
// becomes origin), live-forwarded as-override (PeerAS -> LocalAS), RFC 7705
// migration SecondaryAS on empty AS_PATH via PeerSettings.SessionFacts, and
// external raw AS_PATH rewrite.
func TestRFC8893ASPathModificationsChangingEffectiveOriginAS(t *testing.T) {
	installRFC8893Validator(t, []rfc8893VRP{
		{prefix: netip.MustParsePrefix("198.51.100.0/24"), maxLen: 24, asn: 65000},
		{prefix: netip.MustParsePrefix("203.0.113.0/24"), maxLen: 24, asn: 65100},
		{prefix: netip.MustParsePrefix("192.0.2.0/24"), maxLen: 24, asn: 65001},
		{prefix: netip.MustParsePrefix("198.18.0.0/24"), maxLen: 24, asn: 65002},
	})

	reg, err := policy.NewRegistry(policy.Instance{
		Name:   "only-valid",
		Type:   policy.MatchFilterType,
		Config: map[string]any{"rpki-validation-state": []any{"valid"}, "action": "accept"},
	})
	require.NoError(t, err)

	t.Run("remove-private-as emptying path makes LocalAS the effective origin", func(t *testing.T) {
		r := &Reactor{attrModHandlers: attrModHandlersWithDefaults(), filterTransportSeam: rfc8893RemovePrivateTransport()}

		expPol := compileRFC8893ExportChain(t, reg, []policychain.Member{
			{Plugin: "probe", Name: "f"},
			{Name: "only-valid"},
		}, policy.FinalAccept)

		// AS_PATH=[65500] -> stripped to [] -> eBGP prepends LocalAS 65000 -> Valid for 198.51.100.0/24.
		wu, _ := rfc8893IPv4Update(t, asSequence4(65500), []byte{24, 198, 51, 100})
		dest := (&PeerSettings{Address: netip.MustParseAddr("10.0.0.2"), LocalAS: 65000, PeerAS: 65002}).SessionFacts(65002)
		session := policyview.ExportSession(dest, filterapi.PeerFilterInfo{}, false, filterapi.ExportExplicit)

		res := r.runEgressPolicyChainASN4(expPol, "10.0.0.2", 65002, 65000, false, session, wu, true)
		require.True(t, res.accept)
	})

	t.Run("remove-private-as peer-as replacement makes PeerAS the effective origin", func(t *testing.T) {
		r := &Reactor{attrModHandlers: attrModHandlersWithDefaults(), filterTransportSeam: rfc8893RemovePrivatePeerASTransport()}

		expPol := compileRFC8893ExportChain(t, reg, []policychain.Member{
			{Plugin: "probe", Name: "f"},
			{Name: "only-valid"},
		}, policy.FinalAccept)

		// AS_PATH=[3356, 65500] -> private 65500 replaced with PeerAS 65002 -> Valid for 198.18.0.0/24.
		wu, _ := rfc8893IPv4Update(t, asSequence4(3356, 65500), []byte{24, 198, 18, 0})
		dest := (&PeerSettings{Address: netip.MustParseAddr("10.0.0.2"), LocalAS: 65000, PeerAS: 65002}).SessionFacts(65002)
		session := policyview.ExportSession(dest, filterapi.PeerFilterInfo{}, false, filterapi.ExportExplicit)

		res := r.runEgressPolicyChainASN4(expPol, "10.0.0.2", 65002, 65000, false, session, wu, true)
		require.True(t, res.accept)
	})

	t.Run("as-override changes effective origin AS from PeerAS to LocalAS through live forward", func(t *testing.T) {
		f := newAIGPReplayFixture(t, nil)
		f.r.orderedEgressSteps = buildOrderedEgressSteps()
		peer := f.destination
		peer.settings.LocalAS = 65000
		peer.settings.PeerAS = 65002
		peer.settings.ASOverride = true
		peer.settings.NextHopMode = NextHopUnchanged
		peer.settings.ExportPolicy = compileRFC8893ExportChain(t, reg, []policychain.Member{{Name: "only-valid"}}, policy.FinalAccept)
		peer.refreshForwardFacts()

		// Received AS_PATH=[65001, 65002] (unauthorized 65002 for 198.51.100.0/24);
		// as-override rewrites 65002 -> 65000 (Valid for 198.51.100.0/24).
		attrs := makeAttr(0x40, byte(attribute.AttrOrigin), []byte{0})
		attrs = append(attrs, makeAttr(0x40, byte(attribute.AttrASPath), asSequence4(65001, 65002))...)
		attrs = append(attrs, makeAttr(0x40, byte(attribute.AttrNextHop), []byte{192, 0, 2, 254})...)
		body := buildUpdatePayload(attrs, []byte{24, 198, 51, 100})

		id := f.receive(t, body)
		f.forward(t, id)
		forwardSocketBarrier(t, f.r)

		bodies := aigpSocketBodies(t, f.conn)
		require.Len(t, bodies, 1, "as-override rewriting origin 65002 -> Valid 65000 must be advertised")
	})

	t.Run("RFC 7705 migration SecondaryAS is innermost effective origin on empty AS_PATH", func(t *testing.T) {
		r := &Reactor{attrModHandlers: attrModHandlersWithDefaults()}
		expPol := compileRFC8893ExportChain(t, reg, []policychain.Member{{Name: "only-valid"}}, policy.FinalAccept)

		// Empty AS_PATH with LocalAS=65200, GlobalLocalAS=65100 -> SecondaryAS=65100 -> announced [65200, 65100] -> Valid.
		wu, _ := rfc8893IPv4Update(t, nil, []byte{24, 203, 0, 113})
		dest := (&PeerSettings{
			Address: netip.MustParseAddr("10.0.0.2"), LocalAS: 65200, GlobalLocalAS: 65100, PeerAS: 65002,
		}).SessionFacts(65002)
		session := policyview.ExportSession(dest, filterapi.PeerFilterInfo{}, false, filterapi.ExportExplicit)

		res := r.runEgressPolicyChainASN4(expPol, "10.0.0.2", 65002, 65200, false, session, wu, true)
		require.True(t, res.accept)
	})

	t.Run("external raw AS_PATH rewrite changes effective origin AS", func(t *testing.T) {
		rewritten, _ := rfc8893IPv4Update(t, asSequence4(65001), []byte{24, 192, 0, 2})
		r := &Reactor{
			attrModHandlers: attrModHandlersWithDefaults(),
			filterTransportSeam: &externalTransport{
				reads: []string{"as-path", rpc.FilterFieldRaw},
				sets:  []string{"as-path"},
				reply: rpc.FilterUpdateOutput{Action: rpc.FilterModify, Raw: rewritten.Payload()},
			},
		}

		expPol := compileRFC8893ExportChain(t, reg, []policychain.Member{
			{Plugin: "probe", Name: "f"},
			{Name: "only-valid"},
		}, policy.FinalAccept)

		// Input has unauthorized AS 65099; external filter rewrites AS_PATH to [65001] (Valid).
		wu, _ := rfc8893IPv4Update(t, asSequence4(65099), []byte{24, 192, 0, 2})
		dest := (&PeerSettings{Address: netip.MustParseAddr("10.0.0.2"), LocalAS: 65000, PeerAS: 65002}).SessionFacts(65002)
		session := policyview.ExportSession(dest, filterapi.PeerFilterInfo{}, false, filterapi.ExportExplicit)

		res := r.runEgressPolicyChainASN4(expPol, "10.0.0.2", 65002, 65000, false, session, wu, true)
		require.True(t, res.accept)
	})
}

// RFC requirement: RFC8893-1-2 negative -- AS_PATH modifications that flip a
// route to an unauthorized origin AS (as-override through live forwarding and
// RFC 7705 SecondaryAS) are rejected, while transit-only as-override that
// leaves the rightmost origin AS unchanged keeps it Valid.
func TestRFC8893ASPathModificationToUnauthorizedOriginIsRejected(t *testing.T) {
	installRFC8893Validator(t, []rfc8893VRP{
		{prefix: netip.MustParsePrefix("203.0.113.0/24"), maxLen: 24, asn: 65200},
		{prefix: netip.MustParsePrefix("192.0.2.0/24"), maxLen: 24, asn: 65001},
		{prefix: netip.MustParsePrefix("198.51.100.0/24"), maxLen: 24, asn: 65002},
	})

	reg, err := policy.NewRegistry(policy.Instance{
		Name:   "only-valid",
		Type:   policy.MatchFilterType,
		Config: map[string]any{"rpki-validation-state": []any{"valid"}, "action": "accept"},
	})
	require.NoError(t, err)

	t.Run("as-override flipping Valid PeerAS to unauthorized LocalAS rejects through live forward", func(t *testing.T) {
		f := newAIGPReplayFixture(t, nil)
		f.r.orderedEgressSteps = buildOrderedEgressSteps()
		peer := f.destination
		peer.settings.LocalAS = 65000
		peer.settings.PeerAS = 65002
		peer.settings.ASOverride = true
		peer.settings.NextHopMode = NextHopUnchanged
		peer.settings.ExportPolicy = compileRFC8893ExportChain(t, reg, []policychain.Member{{Name: "only-valid"}}, policy.FinalAccept)
		peer.refreshForwardFacts()

		// VRP authorizes 65002, NOT 65000; as-override rewrites 65002 -> 65000 (Invalid -> rejected).
		attrs := makeAttr(0x40, byte(attribute.AttrOrigin), []byte{0})
		attrs = append(attrs, makeAttr(0x40, byte(attribute.AttrASPath), asSequence4(65001, 65002))...)
		attrs = append(attrs, makeAttr(0x40, byte(attribute.AttrNextHop), []byte{192, 0, 2, 254})...)
		body := buildUpdatePayload(attrs, []byte{24, 198, 51, 100})

		id := f.receive(t, body)
		f.forward(t, id)
		forwardSocketBarrier(t, f.r)

		require.Empty(t, aigpSocketBodies(t, f.conn))
	})

	t.Run("RFC 7705 SecondaryAS overrides LocalAS as origin so unauthorized SecondaryAS rejects", func(t *testing.T) {
		r := &Reactor{attrModHandlers: attrModHandlersWithDefaults()}
		expPol := compileRFC8893ExportChain(t, reg, []policychain.Member{{Name: "only-valid"}}, policy.FinalAccept)

		// VRP authorizes LocalAS 65200, NOT SecondaryAS 65100; announced path is
		// [65200, 65100], so effective origin is 65100 (Invalid -> rejected).
		wu, _ := rfc8893IPv4Update(t, nil, []byte{24, 203, 0, 113})
		dest := (&PeerSettings{
			Address: netip.MustParseAddr("10.0.0.2"), LocalAS: 65200, GlobalLocalAS: 65100, PeerAS: 65002,
		}).SessionFacts(65002)
		session := policyview.ExportSession(dest, filterapi.PeerFilterInfo{}, false, filterapi.ExportExplicit)

		res := r.runEgressPolicyChainASN4(expPol, "10.0.0.2", 65002, 65200, false, session, wu, true)
		require.False(t, res.accept)
	})

	t.Run("as-override affecting only transit AS leaves Valid rightmost origin unchanged", func(t *testing.T) {
		r := &Reactor{attrModHandlers: attrModHandlersWithDefaults()}
		expPol := compileRFC8893ExportChain(t, reg, []policychain.Member{{Name: "only-valid"}}, policy.FinalAccept)

		// AS_PATH=[65002, 65001] with PeerAS=65002, LocalAS=65000, as-override=true
		// announces [65000, 65000, 65001]: rightmost origin is still 65001 (Valid).
		wu, _ := rfc8893IPv4Update(t, asSequence4(65002, 65001), []byte{24, 192, 0, 2})
		dest := (&PeerSettings{
			Address: netip.MustParseAddr("10.0.0.2"), LocalAS: 65000, PeerAS: 65002, ASOverride: true,
		}).SessionFacts(65002)
		session := policyview.ExportSession(dest, filterapi.PeerFilterInfo{}, false, filterapi.ExportExplicit)

		res := r.runEgressPolicyChainASN4(expPol, "10.0.0.2", 65002, 65000, false, session, wu, true)
		require.True(t, res.accept)
	})
}

// RFC requirement: RFC8893-3-1 positive -- "BGP implementations supporting
// RPKI-based origin validation MUST provide the same policy configuration
// primitives for decisions based on the validation state available for use in
// ingress, redistribution, and egress policies."
// Compiles the IDENTICAL match config (`rpki-validation-state`) across real
// "bgp import", route-policy CompileRouteMatch (paired with redistribute's
// "redistribute import" CompileRouteChain + handleBatch test), and real
// "bgp export", proving all three accept Valid/NotFound and reject Invalid.
func TestRFC8893SamePolicyPrimitiveAcrossIngressRedistributionAndEgress(t *testing.T) {
	installRFC8893Validator(t, []rfc8893VRP{
		{prefix: netip.MustParsePrefix("198.51.100.0/24"), maxLen: 24, asn: 65001},
	})

	// One identical match configuration primitive used across all three policy points.
	dropInvalidCfg := map[string]any{
		"rpki-validation-state": []any{"invalid"},
		"action":                "reject",
	}
	reg, err := policy.NewRegistry(policy.Instance{
		Name:   "drop-invalid",
		Type:   policy.MatchFilterType,
		Config: dropInvalidCfg,
	})
	require.NoError(t, err)

	validWU, _ := rfc8893IPv4Update(t, asSequence4(65001), []byte{24, 198, 51, 100})
	invalidWU, _ := rfc8893IPv4Update(t, asSequence4(65099), []byte{24, 198, 51, 100})
	notFoundWU, _ := rfc8893IPv4Update(t, asSequence4(65099), []byte{24, 203, 0, 113})

	// 1. BGP Ingress (real "bgp import" attachment)
	impChain, err := policychain.Compile(rfc8893ImportAttachment, reg, []policychain.Member{{Name: "drop-invalid"}}, policy.FinalAccept, policychain.Bases{})
	require.NoError(t, err)
	impPeer := NewPeer(&PeerSettings{
		Address:      netip.MustParseAddr("10.0.0.1"),
		LocalAS:      65000,
		PeerAS:       65001,
		ImportPolicy: mustPolicy(t, []policy.Ref{{Name: "drop-invalid"}}, policy.FinalAccept).WithLibrary(impChain),
	})
	r := &Reactor{attrModHandlers: attrModHandlersWithDefaults()}
	srcInfo := &filterapi.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.1"), LocalAS: 65000, PeerAS: 65001}
	var meta map[string]any
	require.True(t, r.runIngressPolicyChainMeta(impPeer, srcInfo.Address, srcInfo, &meta, validWU, validWU.Payload()).accept, "ingress must accept Valid")
	require.True(t, r.runIngressPolicyChainMeta(impPeer, srcInfo.Address, srcInfo, &meta, notFoundWU, notFoundWU.Payload()).accept, "ingress must accept NotFound")
	require.False(t, r.runIngressPolicyChainMeta(impPeer, srcInfo.Address, srcInfo, &meta, invalidWU, invalidWU.Payload()).accept, "ingress must reject Invalid")

	// 2. Route/Redistribution match compiler (identical config via policy.CompileRouteMatch)
	redistMatch, err := policy.CompileRouteMatch(dropInvalidCfg, false, nil)
	require.NoError(t, err)
	validRoute := ribpolicy.Route{Family: family.IPv4Unicast, Prefix: netip.MustParsePrefix("198.51.100.0/24"), OriginAS: 65001, HasOriginAS: true}
	invalidRoute := ribpolicy.Route{Family: family.IPv4Unicast, Prefix: netip.MustParsePrefix("198.51.100.0/24"), OriginAS: 65099, HasOriginAS: true}
	notFoundRoute := ribpolicy.Route{Family: family.IPv4Unicast, Prefix: netip.MustParsePrefix("203.0.113.0/24"), OriginAS: 65099, HasOriginAS: true}
	rejValid, err := redistMatch.Rejects(&validRoute)
	require.NoError(t, err)
	require.False(t, rejValid, "redistribution must accept Valid")
	rejNotFound, err := redistMatch.Rejects(&notFoundRoute)
	require.NoError(t, err)
	require.False(t, rejNotFound, "redistribution must accept NotFound")
	rejInvalid, err := redistMatch.Rejects(&invalidRoute)
	require.NoError(t, err)
	require.True(t, rejInvalid, "redistribution must reject Invalid")

	// 3. BGP Egress (real "bgp export" attachment)
	expPol := compileRFC8893ExportChain(t, reg, []policychain.Member{{Name: "drop-invalid"}}, policy.FinalAccept)
	dest := (&PeerSettings{Address: netip.MustParseAddr("10.0.0.2"), LocalAS: 65000, PeerAS: 65002}).SessionFacts(65002)
	expSession := policyview.ExportSession(dest, filterapi.PeerFilterInfo{}, false, filterapi.ExportExplicit)
	require.True(t, r.runEgressPolicyChainASN4(expPol, "10.0.0.2", 65002, 65000, false, expSession, validWU, true).accept, "egress must accept Valid")
	require.True(t, r.runEgressPolicyChainASN4(expPol, "10.0.0.2", 65002, 65000, false, expSession, notFoundWU, true).accept, "egress must accept NotFound")
	require.False(t, r.runEgressPolicyChainASN4(expPol, "10.0.0.2", 65002, 65000, false, expSession, invalidWU, true).accept, "egress must reject Invalid")
}

// RFC requirement: RFC8893-3-1 negative -- the shared rpki-validation-state
// primitive refuses unknown or empty state values on route, "bgp import", and
// "bgp export" compilers, and when no RPKI validator is active does not falsely
// match Valid across BGP ingress, route/redistribution, or BGP egress.
func TestRFC8893ValidationStatePrimitiveRefusesInvalidConfigAndInactiveValidator(t *testing.T) {
	// Unknown state value refused identically by route and real BGP compilers.
	badCfg := map[string]any{"rpki-validation-state": []any{"unknown-state"}, "action": "reject"}
	_, err := policy.CompileRouteMatch(badCfg, false, nil)
	require.Error(t, err, "route compiler must refuse unknown rpki-validation-state value")

	regBad, err := policy.NewRegistry(policy.Instance{Name: "bad", Type: policy.MatchFilterType, Config: badCfg})
	require.NoError(t, err)
	_, err = policychain.Compile(rfc8893ExportAttachment, regBad, []policychain.Member{{Name: "bad"}}, policy.FinalAccept, policychain.Bases{})
	require.Error(t, err, "BGP export compiler must refuse unknown rpki-validation-state value")
	_, err = policychain.Compile(rfc8893ImportAttachment, regBad, []policychain.Member{{Name: "bad"}}, policy.FinalAccept, policychain.Bases{})
	require.Error(t, err, "BGP import compiler must refuse unknown rpki-validation-state value")

	// Empty state list refused.
	_, err = policy.ParseRPKIValidationStates([]any{})
	require.Error(t, err)

	// No active RPKI validator: match rpki-validation-state [ valid ] action accept
	// MUST NOT falsely match a route on BGP ingress, redistribution, or BGP egress.
	ribpolicy.SetOriginValidator(nil)
	onlyValidCfg := map[string]any{"rpki-validation-state": []any{"valid"}, "action": "accept"}
	regValid, err := policy.NewRegistry(policy.Instance{
		Name:   "only-valid",
		Type:   policy.MatchFilterType,
		Config: onlyValidCfg,
	})
	require.NoError(t, err)

	wu, _ := rfc8893IPv4Update(t, asSequence4(65001), []byte{24, 198, 51, 100})
	r := &Reactor{attrModHandlers: attrModHandlersWithDefaults()}

	// BGP Ingress
	impChain, err := policychain.Compile(rfc8893ImportAttachment, regValid, []policychain.Member{{Name: "only-valid"}}, policy.FinalAccept, policychain.Bases{})
	require.NoError(t, err)
	impPeer := NewPeer(&PeerSettings{
		Address:      netip.MustParseAddr("10.0.0.1"),
		LocalAS:      65000,
		PeerAS:       65001,
		ImportPolicy: mustPolicy(t, []policy.Ref{{Name: "only-valid"}}, policy.FinalAccept).WithLibrary(impChain),
	})
	srcInfo := &filterapi.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.1"), LocalAS: 65000, PeerAS: 65001}
	var meta map[string]any
	require.False(t, r.runIngressPolicyChainMeta(impPeer, srcInfo.Address, srcInfo, &meta, wu, wu.Payload()).accept,
		"without an active validator, ingress match requiring valid must reject")

	// Redistribution / route match
	routeMatch, err := policy.CompileRouteMatch(onlyValidCfg, false, nil)
	require.NoError(t, err)
	route := ribpolicy.Route{Family: family.IPv4Unicast, Prefix: netip.MustParsePrefix("198.51.100.0/24"), OriginAS: 65001, HasOriginAS: true}
	rej, err := routeMatch.Rejects(&route)
	require.NoError(t, err)
	require.True(t, rej, "without an active validator, route/redistribution match requiring valid must reject")

	// BGP Egress
	expPol := compileRFC8893ExportChain(t, regValid, []policychain.Member{{Name: "only-valid"}}, policy.FinalAccept)
	dest := (&PeerSettings{Address: netip.MustParseAddr("10.0.0.2"), LocalAS: 65000, PeerAS: 65002}).SessionFacts(65002)
	session := policyview.ExportSession(dest, filterapi.PeerFilterInfo{}, false, filterapi.ExportExplicit)
	require.False(t, r.runEgressPolicyChainASN4(expPol, "10.0.0.2", 65002, 65000, false, session, wu, true).accept,
		"without an active validator, egress match requiring valid must reject")
}

// RFC requirement: RFC8893-3-2 positive -- "When applied to egress policy,
// validation state MUST be determined using the effective origin AS of the
// route as it will (or would) be announced to the peer."
// Drives a real forwarding fixture with live Session writer and asserts that
// the origin AS passed to RPKI validation equals the EXACT rightmost AS in the
// wire UPDATE transmitted to the peer (covering ordinary eBGP, as-override,
// and 2-octet AS_PATH + AS4_PATH reconstruction).
func TestRFC8893EgressValidationUsesEffectiveAnnouncedOriginAS(t *testing.T) {
	queries := installRFC8893Validator(t, []rfc8893VRP{
		{prefix: netip.MustParsePrefix("198.51.100.0/24"), maxLen: 24, asn: 65000},
		{prefix: netip.MustParsePrefix("203.0.113.0/24"), maxLen: 24, asn: 4200000100},
	})

	reg, err := policy.NewRegistry(policy.Instance{
		Name:   "only-valid",
		Type:   policy.MatchFilterType,
		Config: map[string]any{"rpki-validation-state": []any{"valid"}, "action": "accept"},
	})
	require.NoError(t, err)

	t.Run("live forward with as-override validates exact announced rightmost AS", func(t *testing.T) {
		f := newAIGPReplayFixture(t, nil)
		f.r.orderedEgressSteps = buildOrderedEgressSteps()
		peer := f.destination
		peer.settings.LocalAS = 65000
		peer.settings.PeerAS = 65002
		peer.settings.ASOverride = true
		peer.settings.NextHopMode = NextHopUnchanged
		peer.settings.ExportPolicy = compileRFC8893ExportChain(t, reg, []policychain.Member{{Name: "only-valid"}}, policy.FinalAccept)
		peer.refreshForwardFacts()

		// Received AS_PATH=[65001, 65002]; as-override + eBGP prepend announces
		// [65000, 65001, 65000] -> effective announced origin AS is 65000 (Valid).
		attrs := makeAttr(0x40, byte(attribute.AttrOrigin), []byte{0})
		attrs = append(attrs, makeAttr(0x40, byte(attribute.AttrASPath), asSequence4(65001, 65002))...)
		attrs = append(attrs, makeAttr(0x40, byte(attribute.AttrNextHop), []byte{192, 0, 2, 254})...)
		body := buildUpdatePayload(attrs, []byte{24, 198, 51, 100})

		id := f.receive(t, body)
		f.forward(t, id)
		forwardSocketBarrier(t, f.r)

		bodies := aigpSocketBodies(t, f.conn)
		require.Len(t, bodies, 1, "Valid post-as-override route must be advertised to peer")
		u, err := message.UnpackUpdate(bodies[0])
		require.NoError(t, err)
		_, _, rawAS, found := attribute.AttrFind(u.PathAttributes, attribute.AttrASPath)
		require.True(t, found)
		parsed, err := attribute.ParseASPath(rawAS, true)
		require.NoError(t, err)
		wireASNs := flatASNs(parsed)
		require.Equal(t, []uint32{65000, 65001, 65000}, wireASNs)

		// The exact origin AS queried during egress validation matches the wire rightmost AS.
		require.NotEmpty(t, *queries)
		lastQ := (*queries)[len(*queries)-1]
		require.Equal(t, netip.MustParsePrefix("198.51.100.0/24"), lastQ.prefix)
		require.Equal(t, wireASNs[len(wireASNs)-1], lastQ.originAS)
	})

	t.Run("2-octet destination body reconstructs 4-octet effective origin from AS4_PATH", func(t *testing.T) {
		r := &Reactor{attrModHandlers: attrModHandlersWithDefaults()}
		expPol := compileRFC8893ExportChain(t, reg, []policychain.Member{{Name: "only-valid"}}, policy.FinalAccept)

		ctx2 := bgpctx.EncodingContextForASN4(false)
		ctx2ID, err := bgpctx.Registry.Register(ctx2)
		require.NoError(t, err)
		// 2-byte AS_PATH: [65001, AS_TRANS(23456)] + AS4_PATH: [65001, 4200000100]
		as2 := []byte{byte(attribute.ASSequence), 2, 0xFD, 0xE9, 0x5B, 0xA0}
		as4 := asSequence4(65001, 4200000100)
		attrs := makeAttr(0x40, byte(attribute.AttrOrigin), []byte{0})
		attrs = append(attrs, makeAttr(0x40, byte(attribute.AttrASPath), as2)...)
		attrs = append(attrs, makeAttr(0x40, byte(attribute.AttrNextHop), []byte{192, 0, 2, 254})...)
		attrs = append(attrs, makeAttr(0xC0, byte(attribute.AttrAS4Path), as4)...)
		body := buildUpdatePayload(attrs, []byte{24, 203, 0, 113})
		wu := wireu.NewWireUpdate(body, ctx2ID)

		dest := filterapi.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.2"), LocalAS: 65000, PeerAS: 65002}
		session := policyview.ExportSession(dest, filterapi.PeerFilterInfo{}, false, filterapi.ExportExplicit)

		res := r.runEgressPolicyChainASN4(expPol, "10.0.0.2", 65002, 65000, false, session, wu, false)
		require.True(t, res.accept, "4-octet origin 4200000100 reconstructed from AS4_PATH must validate as Valid")
		lastQ := (*queries)[len(*queries)-1]
		require.Equal(t, uint32(4200000100), lastQ.originAS)
	})
}

// RFC requirement: RFC8893-3-2 negative -- egress validation MUST NOT use the
// stale received origin AS when egress configuration changes what will be
// announced, and MUST NOT block withdrawals.
func TestRFC8893EgressValidationDoesNotUseStaleReceivedOriginAS(t *testing.T) {
	queries := installRFC8893Validator(t, []rfc8893VRP{
		// Only the stale pre-override PeerAS 65002 is authorized; LocalAS 65000 is NOT.
		{prefix: netip.MustParsePrefix("198.51.100.0/24"), maxLen: 24, asn: 65002},
	})

	reg, err := policy.NewRegistry(policy.Instance{
		Name:   "only-valid",
		Type:   policy.MatchFilterType,
		Config: map[string]any{"rpki-validation-state": []any{"valid"}, "action": "accept"},
	})
	require.NoError(t, err)

	f := newAIGPReplayFixture(t, nil)
	f.r.orderedEgressSteps = buildOrderedEgressSteps()
	peer := f.destination
	peer.settings.LocalAS = 65000
	peer.settings.PeerAS = 65002
	peer.settings.ASOverride = true
	peer.settings.NextHopMode = NextHopUnchanged
	peer.settings.ExportPolicy = compileRFC8893ExportChain(t, reg, []policychain.Member{{Name: "only-valid"}}, policy.FinalAccept)
	peer.refreshForwardFacts()

	attrs := makeAttr(0x40, byte(attribute.AttrOrigin), []byte{0})
	attrs = append(attrs, makeAttr(0x40, byte(attribute.AttrASPath), asSequence4(65001, 65002))...)
	attrs = append(attrs, makeAttr(0x40, byte(attribute.AttrNextHop), []byte{192, 0, 2, 254})...)
	body := buildUpdatePayload(attrs, []byte{24, 198, 51, 100})

	id := f.receive(t, body)
	f.forward(t, id)
	forwardSocketBarrier(t, f.r)

	// Because as-override rewrites 65002 -> 65000, the route would be announced
	// with origin 65000 (Invalid), so nothing may be advertised to the peer.
	bodies := aigpSocketBodies(t, f.conn)
	require.Empty(t, bodies, "route whose effective announced origin 65000 is Invalid must not be sent even though received origin 65002 was Valid")
	require.NotEmpty(t, *queries)
	for _, q := range *queries {
		require.NotEqual(t, uint32(65002), q.originAS, "egress validation must not query stale received origin AS 65002")
		require.Equal(t, uint32(65000), q.originAS)
	}

	// Withdrawal-only UPDATE must still pass through egress RPKI policy unblocked.
	r := &Reactor{attrModHandlers: attrModHandlersWithDefaults()}
	expPol := compileRFC8893ExportChain(t, reg, []policychain.Member{{Name: "only-valid"}}, policy.FinalAccept)
	ctx := bgpctx.NewEncodingContext(
		&capability.PeerIdentity{LocalASN: 65000, PeerASN: 65002},
		&capability.EncodingCaps{ASN4: true},
		bgpctx.DirectionRecv,
	)
	ctxID, err := bgpctx.Registry.Register(ctx)
	require.NoError(t, err)
	withdrawBody := []byte{0x00, 0x04, 24, 198, 51, 100, 0x00, 0x00}
	withdrawWU := wireu.NewWireUpdate(withdrawBody, ctxID)
	dest := filterapi.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.2"), LocalAS: 65000, PeerAS: 65002}
	session := policyview.ExportSession(dest, filterapi.PeerFilterInfo{}, false, filterapi.ExportExplicit)
	res := r.runEgressPolicyChainASN4(expPol, "10.0.0.2", 65002, 65000, false, session, withdrawWU, true)
	require.True(t, res.accept, "route withdrawals must not be blocked by egress RPKI validation")
}
