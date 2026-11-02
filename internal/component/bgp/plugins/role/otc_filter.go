// Design: docs/guide/bgp-role.md -- the OTC procedures a role inserts in the peer's filter lists
// Related: otc.go -- decideOTCIngress and OTCEgressFilter, the decisions these filters answer
// Related: upstream.go -- the plugin's other filter, served by the same evaluator
// Related: register.go -- the filters and their insertions

package role

import (
	"encoding/binary"
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
	// otcIngressFilterName and otcEgressFilterName are the RFC 9234 Section 5
	// procedures, one filter per direction because each sets different fields.
	otcIngressFilterName = "otc-ingress"
	otcEgressFilterName  = "otc-egress"
	// otcIngressReference and otcEgressReference are their chain references.
	otcIngressReference = rolePluginName + ":" + otcIngressFilterName
	otcEgressReference  = rolePluginName + ":" + otcEgressFilterName
	// otcPriority places the procedures ahead of the upstream check
	// (upstreamPriority): the upstream check reads a route the procedures
	// already judged eligible.
	otcPriority = 100
	// otcRequiredReason is the text verify prints when an operator names,
	// moves or deactivates an OTC member.
	otcRequiredReason = "the peer's role makes Ze run the RFC 9234 Section 5 OTC procedures on it, and " +
		`RFC 9234 Section 5 says "The operator MUST NOT have the ability to modify the procedures defined in this section."`
	// fieldRaw is the UPDATE body read: the OTC length judgement, the
	// unicast-only scope and the "is to be advertised" clause read it as
	// received, on both transports.
	fieldRaw = rpc.FilterFieldRaw
)

// otcFilterDecls declares the two procedures. It has two readers and MUST
// stay one function with roleFilterDecls: register.go puts it on
// registry.Registration.Filters and RunRolePlugin sends it at Stage 1.
//
// The ingress filter reads the body because RFC 9234 Section 5 says "An
// UPDATE message with a malformed OTC Attribute SHALL be handled using the
// approach of "treat-as-withdraw" [RFC7606]." A read of the otc field that
// finds a malformed value fails the member before it runs, which would
// answer the filter's on-error instead of the withdrawal. A failure rejects
// (fail closed): a route this plugin cannot judge is never sent on.
func otcFilterDecls() []sdk.FilterDecl {
	return []sdk.FilterDecl{
		{
			Name:      otcIngressFilterName,
			Direction: sdk.FilterImport,
			Reads: []string{
				fieldRaw,
				policyview.FieldPeerName.String(), policyview.FieldPeerGroup.String(),
				policyview.FieldPeerAddress.String(), policyview.FieldPeerAS.String(),
			},
			Sets:    []string{fieldOTC.String(), fieldSrcRole.String(), fieldSrcPeerRole.String()},
			OnError: sdk.OnErrorReject,
		},
		{
			Name:      otcEgressFilterName,
			Direction: sdk.FilterExport,
			Reads: []string{
				fieldRaw, fieldSrcRole.String(),
				policyview.FieldPeerName.String(), policyview.FieldPeerGroup.String(),
				policyview.FieldPeerAddress.String(), policyview.FieldLocalAS.String(),
				policyview.FieldSourcePeerName.String(), policyview.FieldSourcePeerGroup.String(),
				policyview.FieldSourcePeerAddress.String(),
			},
			Sets:    []string{fieldOTC.String()},
			OnError: sdk.OnErrorReject,
		},
	}
}

// otcInsertions places both procedures on every peer whose config sets role
// import, inherited from the group like the rest of role, and makes them
// required. role import is the leaf that declares a role: the container is
// present on every peer, because strict has a default. RFC 9234 Section 5: "The operator MUST NOT have the ability
// to modify the procedures defined in this section." Like otcFilterDecls it
// has two readers and MUST stay one function.
func otcInsertions() []rpc.FilterInsertion {
	return []rpc.FilterInsertion{
		{
			Filter: otcIngressReference, Direction: rpc.FilterImport, Priority: otcPriority,
			Config: roleImportPath(), Required: true, Reason: otcRequiredReason,
		},
		{
			Filter: otcEgressReference, Direction: rpc.FilterExport, Priority: otcPriority,
			Config: roleImportPath(), Required: true, Reason: otcRequiredReason,
		},
	}
}

// roleImportPath is the insertion condition of every member this plugin
// inserts: the role import leaf under a peer's resolved entry, which this
// plugin's own YANG module adds.
func roleImportPath() []string {
	return []string{"role", "import"}
}

// roleFilterDecls is every filter this plugin serves, and roleInsertions
// every member it inserts: the two readers of each are register.go and
// RunRolePlugin, so the Stage 1 declaration and the compiled registration
// cannot disagree.
func roleFilterDecls() []sdk.FilterDecl {
	return append(otcFilterDecls(), upstreamFilterDecls()...)
}

func roleInsertions() []rpc.FilterInsertion {
	return append(otcInsertions(), upstreamInsertions()...)
}

// errNoSuchRoleFilter fails a call for a filter this plugin does not serve.
var errNoSuchRoleFilter = errors.New("bgp-role: the engine called a filter this plugin does not declare")

// evaluateRoleFilter is the plugin's one filter evaluator (sdk.Plugin.OnFilter),
// on both transports: it answers each declared filter by its name.
func evaluateRoleFilter(route *sdk.FilterRoute) (*sdk.FilterUpdateOutput, error) {
	switch route.Filter() {
	case otcIngressFilterName:
		return evaluateOTCIngress(route)
	case otcEgressFilterName:
		return evaluateOTCEgress(route)
	case upstreamFilterName:
		return evaluateUpstream(route)
	}
	return nil, fmt.Errorf("%w: %q", errNoSuchRoleFilter, route.Filter())
}

// evaluateOTCIngress answers the RFC 9234 Section 5 ingress procedures for
// one received UPDATE.
func evaluateOTCIngress(route *sdk.FilterRoute) (*sdk.FilterUpdateOutput, error) {
	session, err := readOTCSession(route)
	if err != nil {
		return nil, err
	}
	decision := decideOTCIngress(session.peer, route.Raw())
	switch decision.result {
	case otcRejectLeak:
		// RFC 9234 Section 5: "If a route with the OTC Attribute is received
		// from a Customer or an RS-Client, then it is a route leak and MUST be
		// considered ineligible (see Section 3)." An ineligible route replaces what the peer
		// sent before for its prefixes, so the route is withdrawn, not only
		// stopped.
		return &sdk.FilterUpdateOutput{Action: sdk.FilterWithdraw}, nil
	case otcTreatWithdraw:
		// RFC 9234 Section 5: "An UPDATE message with a malformed OTC
		// Attribute SHALL be handled using the approach of
		// "treat-as-withdraw" [RFC7606]."
		return &sdk.FilterUpdateOutput{Action: sdk.FilterWithdraw}, nil
	}
	sets := make([]rpc.FilterSet, 0, 3)
	if decision.srcRole != "" {
		sets = append(sets, textSet(fieldSrcRole, decision.srcRole))
	}
	if decision.peerRole != "" {
		sets = append(sets, textSet(fieldSrcPeerRole, decision.peerRole))
	}
	if decision.stampASN > 0 {
		// RFC 9234 Section 5: "If a route is received from a Provider, a
		// Peer, or an RS and the OTC Attribute is not present, then it MUST
		// be added with a value equal to the AS number of the remote AS."
		sets = append(sets, otcSet(decision.stampASN))
	}
	if len(sets) == 0 {
		return &sdk.FilterUpdateOutput{Action: sdk.FilterAccept}, nil
	}
	return &sdk.FilterUpdateOutput{Action: sdk.FilterModify, Sets: sets}, nil
}

// otcEgressMeta holds the one-key metadata maps OTCEgressFilter reads, so the
// export path allocates none per route.
var otcEgressMeta = sync.Pool{New: func() any { return make(map[string]any, 1) }}

// evaluateOTCEgress answers the RFC 9234 Section 5 egress procedures, and the
// operator's role export set, for one UPDATE toward one destination.
func evaluateOTCEgress(route *sdk.FilterRoute) (*sdk.FilterUpdateOutput, error) {
	session, err := readOTCSession(route)
	if err != nil {
		return nil, err
	}
	meta, _ := otcEgressMeta.Get().(map[string]any) //nolint:forcetypeassert // the pool's New makes this type
	if session.srcRoleSet {
		meta[fieldSrcRole.String()] = session.srcRole
	}
	var mods filterapi.ModAccumulator
	keep := OTCEgressFilter(session.source, session.peer, route.Raw(), meta, &mods)
	clear(meta)
	otcEgressMeta.Put(meta)
	if !keep {
		return &sdk.FilterUpdateOutput{Action: sdk.FilterReject}, nil
	}
	for _, op := range mods.Ops() {
		if op.Code != otcAttrCode || op.Action != filterapi.AttrModSet || len(op.Buf) != otcAttrLen {
			continue
		}
		// RFC 9234 Section 5: "If a route is to be advertised to a Customer,
		// a Peer, or an RS-Client (when the sender is an RS), and the OTC
		// Attribute is not present, then when advertising the route, an OTC
		// Attribute MUST be added with a value equal to the AS number of the
		// local AS."
		return &sdk.FilterUpdateOutput{
			Action: sdk.FilterModify,
			Sets:   []rpc.FilterSet{otcSet(binary.BigEndian.Uint32(op.Buf))},
		}, nil
	}
	return &sdk.FilterUpdateOutput{Action: sdk.FilterAccept}, nil
}

// textSet sets a text metadata field to value.
func textSet(field policy.Field, value string) rpc.FilterSet {
	return rpc.FilterSet{Field: field.String(), Action: rpc.FilterSetActionSet, Value: strconv.AppendQuote(nil, value)}
}

// otcSet sets the otc field to asn.
func otcSet(asn uint32) rpc.FilterSet {
	return rpc.FilterSet{Field: fieldOTC.String(), Action: rpc.FilterSetActionSet, Value: strconv.AppendUint(nil, uint64(asn), 10)}
}

// otcSession is what the OTC filters read beside the UPDATE body: the
// session's peer (the sender on import, the destination on export), the
// exported route's source peer, and the src-role metadata ingress wrote.
type otcSession struct {
	peer       filterapi.PeerFilterInfo
	source     filterapi.PeerFilterInfo
	srcRole    string
	srcRoleSet bool
}

// otcReaders are the view readers of the fields otcSession holds, resolved
// once by resolveOTCReaders from init, after the src-role metadata field is
// registered.
var otcReaders struct {
	peerName, peerGroup, peerAddress, peerAS, localAS policyview.Reader
	sourceName, sourceGroup, sourceAddress, srcRole   policyview.Reader
}

func resolveOTCReaders() {
	otcReaders.peerName = mustOTCReader(policyview.FieldPeerName)
	otcReaders.peerGroup = mustOTCReader(policyview.FieldPeerGroup)
	otcReaders.peerAddress = mustOTCReader(policyview.FieldPeerAddress)
	otcReaders.peerAS = mustOTCReader(policyview.FieldPeerAS)
	otcReaders.localAS = mustOTCReader(policyview.FieldLocalAS)
	otcReaders.sourceName = mustOTCReader(policyview.FieldSourcePeerName)
	otcReaders.sourceGroup = mustOTCReader(policyview.FieldSourcePeerGroup)
	otcReaders.sourceAddress = mustOTCReader(policyview.FieldSourcePeerAddress)
	otcReaders.srcRole = mustOTCReader(fieldSrcRole)
}

func mustOTCReader(field policy.Field) policyview.Reader {
	reader, err := policyview.ReaderFor(field)
	if err != nil {
		panic("BUG: bgp-role: no reader for " + field.String() + ": " + err.Error())
	}
	return reader
}

// readOTCSession reads the session from the engine's view on the fast path,
// else from the fields object a forked call carries. A field the filter did
// not declare reads as absent on both.
func readOTCSession(route *sdk.FilterRoute) (otcSession, error) {
	if view, isView := route.Engine().(*policyview.View); isView {
		return viewOTCSession(view)
	}
	raw, err := route.Fields()
	if err != nil {
		return otcSession{}, err
	}
	return fieldsOTCSession(raw)
}

func viewOTCSession(view *policyview.View) (otcSession, error) {
	var s otcSession
	var errs [9]error
	var value policyview.Value
	value, errs[0] = view.Read(otcReaders.peerName)
	s.peer.Name = value.Text()
	value, errs[1] = view.Read(otcReaders.peerGroup)
	s.peer.GroupName = value.Text()
	value, errs[2] = view.Read(otcReaders.peerAddress)
	s.peer.Address = value.Address()
	value, errs[3] = view.Read(otcReaders.peerAS)
	s.peer.PeerAS = viewASN(value)
	value, errs[4] = view.Read(otcReaders.localAS)
	s.peer.LocalAS = viewASN(value)
	value, errs[5] = view.Read(otcReaders.sourceName)
	s.source.Name = value.Text()
	value, errs[6] = view.Read(otcReaders.sourceGroup)
	s.source.GroupName = value.Text()
	value, errs[7] = view.Read(otcReaders.sourceAddress)
	s.source.Address = value.Address()
	value, errs[8] = view.Read(otcReaders.srcRole)
	s.srcRole, s.srcRoleSet = value.Text(), value.Present()
	if err := errors.Join(errs[:]...); err != nil {
		return otcSession{}, fmt.Errorf("bgp-role: OTC: the session cannot be read: %w", err)
	}
	return s, nil
}

// viewASN answers an AS fact; the facts are 32-bit numbers by construction.
func viewASN(value policyview.Value) uint32 {
	asn, _ := value.Number() // an absent fact reads as no AS, which every caller compares as unknown
	return uint32(asn)       //nolint:gosec // G115: an AS fact holds a 32-bit AS number
}

// otcFieldsJSON is the fields object of a forked call, keyed by the field
// names otcFilterDecls declares.
type otcFieldsJSON struct {
	PeerName      string  `json:"peer-name"`
	PeerGroup     string  `json:"peer-group"`
	PeerAddress   string  `json:"peer-address"`
	PeerAS        uint32  `json:"peer-as"`
	LocalAS       uint32  `json:"local-as"`
	SourceName    string  `json:"source-peer-name"`
	SourceGroup   string  `json:"source-peer-group"`
	SourceAddress string  `json:"source-peer-address"`
	SrcRole       *string `json:"src-role"`
}

func fieldsOTCSession(raw []byte) (otcSession, error) {
	var fields otcFieldsJSON
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &fields); err != nil {
			return otcSession{}, fmt.Errorf("bgp-role: OTC: the fields object cannot be read: %w", err)
		}
	}
	peerAddress, err := parseFieldAddress(fields.PeerAddress)
	if err != nil {
		return otcSession{}, err
	}
	sourceAddress, err := parseFieldAddress(fields.SourceAddress)
	if err != nil {
		return otcSession{}, err
	}
	s := otcSession{
		peer: filterapi.PeerFilterInfo{
			Name: fields.PeerName, GroupName: fields.PeerGroup, Address: peerAddress,
			PeerAS: fields.PeerAS, LocalAS: fields.LocalAS,
		},
		source: filterapi.PeerFilterInfo{
			Name: fields.SourceName, GroupName: fields.SourceGroup, Address: sourceAddress,
		},
	}
	if fields.SrcRole != nil {
		s.srcRole, s.srcRoleSet = *fields.SrcRole, true
	}
	return s, nil
}

// parseFieldAddress reads an address fact; absent is the zero address, as
// the view answers it.
func parseFieldAddress(text string) (netip.Addr, error) {
	if text == "" {
		return netip.Addr{}, nil
	}
	addr, err := netip.ParseAddr(text)
	if err != nil {
		return netip.Addr{}, fmt.Errorf("bgp-role: OTC: address fact %q cannot be read: %w", text, err)
	}
	return addr, nil
}
