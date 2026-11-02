// Design: docs/guide/graceful-restart.md -- the LLGR procedures this plugin inserts in the peers' filter lists
// RFC: rfc/short/rfc9494.md -- Section 4.3, Section 4.6
// Overview: gr.go -- GR plugin entry point
// Related: gr_ingress.go -- LLGRIngressFilter, the ingress decision llgr-ingress answers
// Related: gr_egress.go -- LLGREgressFilter, the egress decision llgr-egress answers

package gr

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"strconv"
	"sync"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

const (
	// llgrIngressFilterName and llgrEgressFilterName are the RFC 9494
	// Section 4.3 procedures, one filter per direction: the ingress step
	// marks a received LLGR_STALE route, the egress step decides where a
	// long-lived stale route goes.
	llgrIngressFilterName = "llgr-ingress"
	llgrEgressFilterName  = "llgr-egress"
	// llgrIngressPriority places the ingress step ahead of bgp-role's
	// procedures (priority 100), as the protocol stage put it before them:
	// the step runs before every operator import policy, so a policy that
	// removes LLGR_STALE cannot hide the route from it.
	llgrIngressPriority = 50
	// llgrEgressPriority places the egress step after bgp-role's export
	// procedures (priority 100).
	llgrEgressPriority = 150
	// llgrRequiredReason is the text verify prints when an operator names,
	// moves or deactivates an LLGR member.
	llgrRequiredReason = "a peer's long-lived-stale-time makes Ze advertise the Long-Lived Graceful Restart Capability, and " +
		`RFC 9494 Section 4.3 says a speaker "that has advertised the Long-Lived Graceful Restart Capability to a neighbor MUST perform" ` +
		"its procedures for the routes of that neighbor"
	// communityNoExportText and localPrefZeroText are the RFC 9494 Section
	// 4.6 depreference, in the set values the engine reads.
	communityNoExportText = `"65535:65281"`
	localPrefZeroText     = "0"
)

// llgrConfig is the config condition both insertions share: the leaf whose
// presence makes Ze advertise the LLGR Capability (code 71) to the peer.
var llgrConfig = []string{"session", "capability", "graceful-restart", "long-lived-stale-time"}

// grFilterDecls declares the two procedures. It has two readers and MUST
// stay one function: register.go puts it on registry.Registration.Filters
// and runGRPlugin sends it at Stage 1.
//
// The ingress step reads the body for COMMUNITIES and llgr-advertised for
// whether Ze sent the capability on this session; it sets the stale level.
// The egress step reads the stale level, the destination's address and AS
// facts, and, for a route rebuilt from storage, the source's
// llgr-advertised and the body; it opts into the stale readvertise rail
// (RFC 9494 Section 4.3 governs the routes that rail re-advertises). A
// failure rejects: a route this plugin cannot judge is never sent on.
func grFilterDecls() []sdk.FilterDecl {
	return []sdk.FilterDecl{
		{
			Name:      llgrIngressFilterName,
			Direction: sdk.FilterImport,
			Reads:     []string{rpc.FilterFieldRaw, policyview.FieldLLGRAdvertised.String()},
			Sets:      []string{fieldStale.String()},
			OnError:   sdk.OnErrorReject,
		},
		{
			Name:      llgrEgressFilterName,
			Direction: sdk.FilterExport,
			Reads: []string{
				rpc.FilterFieldRaw, fieldStale.String(), policyview.FieldSourceLLGRAdvertised.String(),
				policyview.FieldPeerAddress.String(), policyview.FieldPeerAS.String(), policyview.FieldLocalAS.String(),
			},
			Sets:        []string{policy.FieldCommunity.String(), policy.FieldLocalPreference.String()},
			OnError:     sdk.OnErrorReject,
			Readvertise: true,
		},
	}
}

// grInsertions places the ingress step on every peer whose config sets
// long-lived-stale-time, and the egress step on every peer's export list
// while any peer sets it: the egress procedure is keyed on the route's
// source, the neighbor Ze advertised the capability to, and acts toward
// every destination. Both are required. Like grFilterDecls it has two
// readers and MUST stay one function.
func grInsertions() []rpc.FilterInsertion {
	return []rpc.FilterInsertion{
		{
			Filter: grPluginName + ":" + llgrIngressFilterName, Direction: rpc.FilterImport,
			Priority: llgrIngressPriority, Config: llgrConfig, Required: true, Reason: llgrRequiredReason,
		},
		{
			Filter: grPluginName + ":" + llgrEgressFilterName, Direction: rpc.FilterExport,
			Priority: llgrEgressPriority, Config: llgrConfig, Scope: rpc.FilterScopeAnyPeer,
			Required: true, Reason: llgrRequiredReason,
		},
	}
}

// errNoSuchGRFilter fails a call for a filter this plugin does not serve.
var errNoSuchGRFilter = errors.New("bgp-gr: the engine called a filter this plugin does not declare")

// evaluateGRFilter is the plugin's one filter evaluator (sdk.Plugin.OnFilter),
// on both transports: it answers each declared filter by its name. It runs
// in the plugin's own process, beside the LLGR capability state the egress
// decision reads (egressState), so a forked engine judges with the same
// state as an internal one.
func evaluateGRFilter(route *sdk.FilterRoute) (*sdk.FilterUpdateOutput, error) {
	switch route.Filter() {
	case llgrIngressFilterName:
		return evaluateLLGRIngress(route)
	case llgrEgressFilterName:
		return evaluateLLGREgress(route)
	}
	return nil, fmt.Errorf("%w: %q", errNoSuchGRFilter, route.Filter())
}

// evaluateLLGRIngress answers the RFC 9494 Section 4.3 ingress step for one
// received UPDATE: a route from a session Ze sent the capability on, whose
// COMMUNITIES holds LLGR_STALE, gets stale level 2.
func evaluateLLGRIngress(route *sdk.FilterRoute) (*sdk.FilterUpdateOutput, error) {
	session, err := readLLGRSession(route)
	if err != nil {
		return nil, err
	}
	// RFC 9494 Section 4.3
	if !llgrStaleFromSource(filterapi.PeerFilterInfo{LLGRAdvertised: session.llgrAdvertised}, route.Raw()) {
		return &sdk.FilterUpdateOutput{Action: sdk.FilterAccept}, nil
	}
	return &sdk.FilterUpdateOutput{
		Action: sdk.FilterModify,
		Sets: []rpc.FilterSet{{
			Field: fieldStale.String(), Action: rpc.FilterSetActionSet,
			Value: strconv.AppendUint(nil, uint64(llgrStaleLevel), 10),
		}},
	}, nil
}

// llgrEgressMeta holds the two-key metadata maps LLGREgressFilter reads, so
// the export path allocates none per governed route.
var llgrEgressMeta = sync.Pool{New: func() any { return make(map[string]any, 2) }}

// evaluateLLGREgress answers the RFC 9494 Section 4.3 and 4.6 egress step for
// one UPDATE toward one destination. A route the procedures do not govern
// (no LLGR stale level, and not a rebuilt copy of a route from an LLGR
// session) is accepted before any destination fact is read: that is the
// forward path's fast path.
func evaluateLLGREgress(route *sdk.FilterRoute) (*sdk.FilterUpdateOutput, error) {
	session, err := readLLGRSession(route)
	if err != nil {
		return nil, err
	}
	if !session.governed() {
		return &sdk.FilterUpdateOutput{Action: sdk.FilterAccept}, nil
	}
	meta, _ := llgrEgressMeta.Get().(map[string]any) //nolint:forcetypeassert // the pool's New makes this type
	if session.stale > 0 {
		meta["stale"] = session.stale
	}
	if session.sourceKnown {
		// source-llgr-advertised answers only for a route rebuilt from
		// storage (policyview answerSourceLLGRAdvertised), so its presence
		// is the rebuilt mark advertisedStaleLevel recovers the level by.
		meta[filterapi.MetaKeyRebuilt] = true
	}
	var mods filterapi.ModAccumulator
	src := filterapi.PeerFilterInfo{LLGRAdvertised: session.sourceLLGR}
	// RFC 9494 Section 4.3
	keep := LLGREgressFilter(src, session.dest, route.Raw(), meta, &mods)
	clear(meta)
	llgrEgressMeta.Put(meta)
	return llgrEgressVerdict(keep, &mods), nil
}

// llgrEgressVerdict turns LLGREgressFilter's decision into the filter's
// answer: withdraw toward an eBGP destination that has not sent the
// capability, NO_EXPORT and LOCAL_PREF 0 toward an iBGP one, unchanged
// otherwise.
func llgrEgressVerdict(keep bool, mods *filterapi.ModAccumulator) *sdk.FilterUpdateOutput {
	if !keep {
		return &sdk.FilterUpdateOutput{Action: sdk.FilterReject}
	}
	if mods.IsWithdraw() {
		// RFC 9494 Section 4.3: "Note that this requirement implies that such
		// routes should be withdrawn from any such neighbor."
		return &sdk.FilterUpdateOutput{Action: sdk.FilterWithdraw}
	}
	var sets []rpc.FilterSet
	for _, op := range mods.Ops() {
		switch op.Code {
		case attrCodeCommunity:
			sets = append(sets, rpc.FilterSet{Field: policy.FieldCommunity.String(),
				Action: rpc.FilterSetActionAdd, Value: []byte(communityNoExportText)})
		case attrCodeLocalPref:
			sets = append(sets, rpc.FilterSet{Field: policy.FieldLocalPreference.String(),
				Action: rpc.FilterSetActionSet, Value: []byte(localPrefZeroText)})
		}
	}
	if len(sets) == 0 {
		return &sdk.FilterUpdateOutput{Action: sdk.FilterAccept}
	}
	// RFC 9494 Section 4.6
	return &sdk.FilterUpdateOutput{Action: sdk.FilterModify, Sets: sets}
}

// llgrSession is what the LLGR filters read beside the UPDATE body: on
// import whether Ze sent the capability on the session, on export the
// route's stale level, the source's capability fact when the route was
// rebuilt from storage, and the destination's address and AS facts.
type llgrSession struct {
	llgrAdvertised bool
	stale          uint8
	sourceKnown    bool // source-llgr-advertised is present: the route was rebuilt
	sourceLLGR     bool
	dest           filterapi.PeerFilterInfo
}

// governed reports whether LLGREgressFilter can decide anything but accept:
// the route carries a stale level at or above the LLGR level, or carries
// none and is a rebuilt copy whose level advertisedStaleLevel recovers.
func (s llgrSession) governed() bool {
	if s.stale >= llgrStaleLevel {
		return true
	}
	return s.stale == 0 && s.sourceKnown
}

// llgrReaders are the view readers of the fields llgrSession holds, resolved
// once by resolveLLGRReaders from init, after the stale field is registered.
var llgrReaders struct {
	llgrAdvertised, stale, sourceLLGR, peerAddress, peerAS, localAS policyview.Reader
}

func resolveLLGRReaders() {
	llgrReaders.llgrAdvertised = mustLLGRReader(policyview.FieldLLGRAdvertised)
	llgrReaders.stale = mustLLGRReader(fieldStale)
	llgrReaders.sourceLLGR = mustLLGRReader(policyview.FieldSourceLLGRAdvertised)
	llgrReaders.peerAddress = mustLLGRReader(policyview.FieldPeerAddress)
	llgrReaders.peerAS = mustLLGRReader(policyview.FieldPeerAS)
	llgrReaders.localAS = mustLLGRReader(policyview.FieldLocalAS)
}

func mustLLGRReader(field policy.Field) policyview.Reader {
	reader, err := policyview.ReaderFor(field)
	if err != nil {
		panic("BUG: bgp-gr: no reader for " + field.String() + ": " + err.Error())
	}
	return reader
}

// readLLGRSession reads the session from the engine's view on the fast path,
// else from the fields object a forked call carries. A field the filter did
// not declare reads as absent on both.
func readLLGRSession(route *sdk.FilterRoute) (llgrSession, error) {
	if view, isView := route.Engine().(*policyview.View); isView {
		return viewLLGRSession(view, route.Filter() == llgrEgressFilterName)
	}
	raw, err := route.Fields()
	if err != nil {
		return llgrSession{}, err
	}
	return fieldsLLGRSession(raw)
}

// viewLLGRSession reads the facts of one direction: the ingress step's one
// flag, or the egress step's stale level and source fact, then the
// destination facts only for a route the procedures govern.
func viewLLGRSession(view *policyview.View, egress bool) (llgrSession, error) {
	var s llgrSession
	if !egress {
		value, err := view.Read(llgrReaders.llgrAdvertised)
		if err != nil {
			return llgrSession{}, fmt.Errorf("bgp-gr: llgr-ingress: the session cannot be read: %w", err)
		}
		s.llgrAdvertised = value.Flag()
		return s, nil
	}
	var errs [5]error
	var value policyview.Value
	value, errs[0] = view.Read(llgrReaders.stale)
	s.stale = viewOctet(value)
	value, errs[1] = view.Read(llgrReaders.sourceLLGR)
	s.sourceKnown, s.sourceLLGR = value.Present(), value.Flag()
	if errs[0] == nil && errs[1] == nil && !s.governed() {
		return s, nil
	}
	value, errs[2] = view.Read(llgrReaders.peerAddress)
	s.dest.Address = value.Address()
	value, errs[3] = view.Read(llgrReaders.peerAS)
	s.dest.PeerAS = viewASN(value)
	value, errs[4] = view.Read(llgrReaders.localAS)
	s.dest.LocalAS = viewASN(value)
	if err := errors.Join(errs[:]...); err != nil {
		return llgrSession{}, fmt.Errorf("bgp-gr: llgr-egress: the session cannot be read: %w", err)
	}
	return s, nil
}

// viewOctet answers the stale level; an absent level reads as 0, the level
// of a route no stale procedure marked.
func viewOctet(value policyview.Value) uint8 {
	level, _ := value.Number() // absent is no level: governed() reads 0 as unmarked
	return uint8(level)        //nolint:gosec // G115: the stale field is a metadata octet
}

// viewASN answers an AS fact; the facts are 32-bit numbers by construction.
func viewASN(value policyview.Value) uint32 {
	asn, _ := value.Number() // an absent fact reads as no AS, which LLGREgressFilter compares as unknown
	return uint32(asn)       //nolint:gosec // G115: an AS fact holds a 32-bit AS number
}

// llgrFieldsJSON is the fields object of a forked call, keyed by the field
// names grFilterDecls declares.
type llgrFieldsJSON struct {
	LLGRAdvertised bool   `json:"llgr-advertised"`
	Stale          uint8  `json:"stale"`
	SourceLLGR     *bool  `json:"source-llgr-advertised"`
	PeerAddress    string `json:"peer-address"`
	PeerAS         uint32 `json:"peer-as"`
	LocalAS        uint32 `json:"local-as"`
}

func fieldsLLGRSession(raw []byte) (llgrSession, error) {
	var fields llgrFieldsJSON
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &fields); err != nil {
			return llgrSession{}, fmt.Errorf("bgp-gr: LLGR: the fields object cannot be read: %w", err)
		}
	}
	s := llgrSession{llgrAdvertised: fields.LLGRAdvertised, stale: fields.Stale}
	if fields.SourceLLGR != nil {
		s.sourceKnown, s.sourceLLGR = true, *fields.SourceLLGR
	}
	s.dest.PeerAS, s.dest.LocalAS = fields.PeerAS, fields.LocalAS
	if fields.PeerAddress == "" {
		return s, nil
	}
	addr, err := netip.ParseAddr(fields.PeerAddress)
	if err != nil {
		return llgrSession{}, fmt.Errorf("bgp-gr: LLGR: address fact %q cannot be read: %w", fields.PeerAddress, err)
	}
	s.dest.Address = addr
	return s, nil
}
