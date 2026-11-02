// Design: docs/guide/route-policy.md -- the community members an operator names in a peer's filter chain
// RFC: rfc/short/rfc7999.md -- Section 3.1, the BLACKHOLE agreement this plugin inserts on export
// RFC: rfc/short/rfc8195.md -- Section 3.2, the relation tag
// Overview: filter_community.go -- the plugin's config and its per-peer config lookup
// Related: filter.go -- applyIngressFilter and applyRelationTag, the decisions the import members answer
// Related: egress.go -- applyEgressFilter, the decision the export member answers
// Related: blackhole.go -- blackholeReadvertiseRefused, the decision the inserted export member answers

package filter_community

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
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

const (
	// communityPluginName is the plugin's registered name, the first half of
	// every member reference it serves.
	communityPluginName = "bgp-filter-community"

	// The four filters. The operator names the first three in a peer's
	// chain (spec-filter-wire-6-stage-filters D-3): ingress and relation on
	// import, egress on export. The config loader inserts the fourth.
	ingressFilterName   = "ingress"
	relationFilterName  = "relation"
	egressFilterName    = "egress"
	blackholeFilterName = "blackhole-agreement"

	// srcPeerRoleName is bgp-role's route metadata: what the source peer IS
	// to us. The relation member reads it. Every inserted member, bgp-role's
	// OTC procedure that writes it included, runs ahead of the operator's
	// members, so the relation member always reads it written.
	srcPeerRoleName = "src-peer-role"

	// blackholePriority places the agreement check among the inserted
	// export members. It only withholds, so its position against the other
	// procedures changes no route they produce.
	blackholePriority = 10

	// blackholeRequiredReason is the text verify prints when an operator
	// names, moves or deactivates the agreement member.
	blackholeRequiredReason = `RFC 7999 Section 3.1 says "In a bilateral peering relationship, use of the BLACKHOLE community MUST be ` +
		`agreed upon by the two networks before advertising it.", so a BLACKHOLE route is withheld from every ` +
		"eBGP destination that is not a route-server client unless its blackhole agreement names the community"
)

// communityFilterRef answers the chain reference of one of this plugin's
// filters, the form an operator writes in a peer's filter list.
func communityFilterRef(name string) string { return communityPluginName + ":" + name }

// communitySets are the list fields the rewriting members write.
var communitySets = []string{
	policy.FieldCommunity.String(), policy.FieldLargeCommunity.String(), policy.FieldExtendedCommunity.String(),
}

// communityFilterDecls declares the plugin's filters. It has two readers and
// MUST stay one function: register.go puts it on registry.Registration.Filters
// and runFilterCommunity sends it at Stage 1. A failure rejects: a route
// this plugin cannot judge is never passed on unjudged.
func communityFilterDecls() []sdk.FilterDecl {
	return []sdk.FilterDecl{
		{
			Name:      ingressFilterName,
			Direction: sdk.FilterImport,
			Reads: []string{
				rpc.FilterFieldRaw, policyview.FieldPeerName.String(), policyview.FieldPeerGroup.String(),
				policyview.FieldLocalAS.String(), policyview.FieldPeerAS.String(),
				policyview.FieldRouteServerAS.String(),
			},
			Sets:    communitySets,
			OnError: sdk.OnErrorReject,
		},
		{
			Name:      relationFilterName,
			Direction: sdk.FilterImport,
			Reads: []string{
				rpc.FilterFieldRaw, policyview.FieldPeerName.String(), policyview.FieldPeerGroup.String(),
				policyview.FieldLocalAS.String(), policyview.FieldIBGP.String(), srcPeerRoleName,
			},
			Sets:    communitySets,
			OnError: sdk.OnErrorReject,
		},
		{
			Name:      egressFilterName,
			Direction: sdk.FilterExport,
			Reads: []string{
				rpc.FilterFieldRaw, policyview.FieldPeerName.String(), policyview.FieldPeerGroup.String(),
				policyview.FieldPeerAddress.String(), policyview.FieldIBGP.String(), policyview.FieldRSClient.String(),
			},
			Sets:    communitySets,
			OnError: sdk.OnErrorReject,
		},
		{
			Name:      blackholeFilterName,
			Direction: sdk.FilterExport,
			Reads: []string{
				rpc.FilterFieldRaw, policyview.FieldPeerName.String(), policyview.FieldPeerGroup.String(),
				policyview.FieldPeerAddress.String(), policyview.FieldIBGP.String(), policyview.FieldRSClient.String(),
			},
			OnError: sdk.OnErrorReject,
		},
	}
}

// communityInsertions places the BLACKHOLE agreement check on the export
// list of every bilateral eBGP destination: an eBGP session that is not a
// route-server client (blackholeReadvertiseRefused says why those two are
// exempt). It is required, and it exists only while this plugin is loaded,
// because only a loaded plugin's insertions are placed. Like
// communityFilterDecls it has two readers and MUST stay one function.
func communityInsertions() []rpc.FilterInsertion {
	return []rpc.FilterInsertion{{
		Filter: communityFilterRef(blackholeFilterName), Direction: rpc.FilterExport, Priority: blackholePriority,
		Session: []rpc.FilterSessionFact{
			{Fact: policyview.FieldIBGP.String(), Values: []string{sessionFactFalse}},
			{Fact: policyview.FieldRSClient.String(), Values: []string{sessionFactFalse}},
		},
		Required: true, Reason: blackholeRequiredReason,
	}}
}

// errNoSuchCommunityFilter fails a call for a filter this plugin does not serve.
var errNoSuchCommunityFilter = errors.New("bgp-filter-community: the engine called a filter this plugin does not declare")

// evaluateCommunityFilter is the plugin's one filter evaluator
// (sdk.Plugin.OnFilter), on both transports: it answers each declared
// filter by its name, through the decision functions the plugin's tests
// prove (ingressFilter, relationIngressFilter, egressFilter,
// blackholeReadvertiseRefused). On import the session's peer is the route's
// source, on export its destination.
func evaluateCommunityFilter(route *sdk.FilterRoute) (*sdk.FilterUpdateOutput, error) {
	session, err := readCommunitySession(route)
	if err != nil {
		return nil, err
	}
	switch route.Filter() {
	case ingressFilterName:
		accept, modified := ingressFilter(session.peer, route.Raw(), nil)
		return rewriteVerdict(accept, route.Raw(), modified)
	case relationFilterName:
		meta := map[string]any{srcPeerRoleName: session.srcPeerRole}
		accept, modified := relationIngressFilter(session.peer, route.Raw(), meta)
		return rewriteVerdict(accept, route.Raw(), modified)
	case egressFilterName:
		var mods filterapi.ModAccumulator
		if !egressFilter(filterapi.PeerFilterInfo{}, session.peer, route.Raw(), nil, &mods) {
			return &sdk.FilterUpdateOutput{Action: sdk.FilterReject}, nil
		}
		return opsVerdict(mods.Ops())
	case blackholeFilterName:
		mu.RLock()
		agreed := agreements
		mu.RUnlock()
		// RFC 7999 Section 3.1
		if blackholeReadvertiseRefused(agreed, session.peer, route.Raw()) {
			logger().Debug("blackhole route withheld: destination did not agree to the BLACKHOLE community",
				"dest", session.peer.Address, "dest-name", session.peer.Name)
			return &sdk.FilterUpdateOutput{Action: sdk.FilterReject}, nil
		}
		return acceptRoute(), nil
	}
	return nil, fmt.Errorf("%w: %q", errNoSuchCommunityFilter, route.Filter())
}

func acceptRoute() *sdk.FilterUpdateOutput { return &sdk.FilterUpdateOutput{Action: sdk.FilterAccept} }

// sessionFactFalse is the value a boolean session fact (ibgp, rs-client)
// carries when the condition is that the fact does not hold.
const sessionFactFalse = "false"

// communityKinds are the three list attributes and the size of one value.
var communityKinds = [...]struct {
	code attribute.AttributeCode
	size int
}{
	{attribute.AttrCommunity, 4},
	{attribute.AttrLargeCommunity, 12},
	{attribute.AttrExtCommunity, 8},
}

// rewriteVerdict preserves the ingress decision before deriving any edits.
// Rejection wins over a rewritten body. For an accepted route, each value
// the body lost is removed, then each value it gained is added, per list
// attribute. A nil body is no change, not permission to accept a rejected route.
// The sender sizes the lists, so each side is indexed once in a map rather
// than scanned per value (handler.go containsValue says what a scan cost).
func rewriteVerdict(accept bool, original, modified []byte) (*sdk.FilterUpdateOutput, error) {
	if !accept {
		return &sdk.FilterUpdateOutput{Action: sdk.FilterReject}, nil
	}
	if modified == nil {
		return acceptRoute(), nil
	}
	var sets []rpc.FilterSet
	for _, kind := range communityKinds {
		before := communityData(original, kind.code)
		after := communityData(modified, kind.code)
		var err error
		if sets, err = appendMissing(sets, kind.code, kind.size, rpc.FilterSetActionRemove, before, after); err != nil {
			return nil, err
		}
		if sets, err = appendMissing(sets, kind.code, kind.size, rpc.FilterSetActionAdd, after, before); err != nil {
			return nil, err
		}
	}
	if len(sets) == 0 {
		return acceptRoute(), nil
	}
	return &sdk.FilterUpdateOutput{Action: sdk.FilterModify, Sets: sets}, nil
}

// appendMissing appends one set of action for each size-octet value of from
// that against does not hold. Both loops are bounded by the attribute
// lengths, which findAttribute checked against the body.
func appendMissing(sets []rpc.FilterSet, code attribute.AttributeCode, size int, action rpc.FilterSetAction,
	from, against []byte,
) ([]rpc.FilterSet, error) {
	if len(from) < size {
		return sets, nil
	}
	held := make(map[string]struct{}, len(against)/size)
	for off := 0; off+size <= len(against); off += size {
		held[string(against[off:off+size])] = struct{}{}
	}
	for off := 0; off+size <= len(from); off += size {
		value := from[off : off+size]
		if _, ok := held[string(value)]; ok {
			continue
		}
		set, err := communitySet(code, action, value)
		if err != nil {
			return nil, err
		}
		sets = append(sets, set)
	}
	return sets, nil
}

// opsVerdict turns the egress decision's accumulated operations into sets,
// in the order the decision wrote them (strip, then tag).
func opsVerdict(ops []filterapi.AttrOp) (*sdk.FilterUpdateOutput, error) {
	if len(ops) == 0 {
		return acceptRoute(), nil
	}
	sets := make([]rpc.FilterSet, 0, len(ops))
	for i := range ops {
		action := rpc.FilterSetActionAdd
		if ops[i].Action == filterapi.AttrModRemove {
			action = rpc.FilterSetActionRemove
		}
		set, err := communitySet(attribute.AttributeCode(ops[i].Code), action, ops[i].Buf)
		if err != nil {
			return nil, err
		}
		sets = append(sets, set)
	}
	return &sdk.FilterUpdateOutput{Action: sdk.FilterModify, Sets: sets}, nil
}

// communityData answers the value octets of one list attribute of an
// UPDATE body, borrowed from it; an absent attribute answers none.
func communityData(payload []byte, code attribute.AttributeCode) []byte {
	_, _, start, end, found := findAttribute(payload, code)
	if !found {
		return nil
	}
	return payload[start:end]
}

// errCommunityValue fails a value whose attribute or length the sets cannot
// carry.
var errCommunityValue = errors.New("bgp-filter-community: a community value has no set form")

// communitySet writes one add or remove of one wire value, in the text form
// the engine's set of that field reads (policychain applySet): "high:low"
// for a standard community, "global:local1:local2" for a large one, and the
// named vocabulary attribute.ExtendedCommunity.AppendDecoded writes for an
// extended one.
func communitySet(code attribute.AttributeCode, action rpc.FilterSetAction, wire []byte) (rpc.FilterSet, error) {
	var text []byte
	var field policy.Field
	switch {
	case code == attribute.AttrCommunity && len(wire) == 4:
		field = policy.FieldCommunity
		text = strconv.AppendUint(text, uint64(binary.BigEndian.Uint16(wire[0:2])), 10)
		text = append(text, ':')
		text = strconv.AppendUint(text, uint64(binary.BigEndian.Uint16(wire[2:4])), 10)
	case code == attribute.AttrLargeCommunity && len(wire) == 12:
		field = policy.FieldLargeCommunity
		for i := 0; i < 12; i += 4 {
			if i > 0 {
				text = append(text, ':')
			}
			text = strconv.AppendUint(text, uint64(binary.BigEndian.Uint32(wire[i:i+4])), 10)
		}
	case code == attribute.AttrExtCommunity && len(wire) == 8:
		field = policy.FieldExtendedCommunity
		text = attribute.ExtendedCommunity(wire).AppendDecoded(text)
	default:
		return rpc.FilterSet{}, fmt.Errorf("%w: attribute %d, %d octets", errCommunityValue, code, len(wire))
	}
	return rpc.FilterSet{Field: field.String(), Action: action, Value: strconv.AppendQuote(nil, string(text))}, nil
}

// communitySession is what the members read beside the UPDATE body: the
// session's peer, held in the form the existing decisions take, and
// bgp-role's src-peer-role.
type communitySession struct {
	peer        filterapi.PeerFilterInfo
	srcPeerRole string
}

// communityReaders are the view readers of the facts communitySession holds,
// resolved once by resolveCommunityReaders from init.
var communityReaders struct {
	peerName, peerGroup, peerAddress, localAS, peerAS, routeServerAS, ibgp, rsClient policyview.Reader
}

func resolveCommunityReaders() {
	communityReaders.peerName = mustCommunityReader(policyview.FieldPeerName)
	communityReaders.peerGroup = mustCommunityReader(policyview.FieldPeerGroup)
	communityReaders.peerAddress = mustCommunityReader(policyview.FieldPeerAddress)
	communityReaders.localAS = mustCommunityReader(policyview.FieldLocalAS)
	communityReaders.peerAS = mustCommunityReader(policyview.FieldPeerAS)
	communityReaders.routeServerAS = mustCommunityReader(policyview.FieldRouteServerAS)
	communityReaders.ibgp = mustCommunityReader(policyview.FieldIBGP)
	communityReaders.rsClient = mustCommunityReader(policyview.FieldRSClient)
}

func mustCommunityReader(field policy.Field) policyview.Reader {
	reader, err := policyview.ReaderFor(field)
	if err != nil {
		panic("BUG: bgp-filter-community: no reader for " + field.String() + ": " + err.Error())
	}
	return reader
}

// srcPeerRoleReader resolves the reader of bgp-role's metadata at the first
// relation call: bgp-role registers the field from its own init, which may
// run after this package's. A daemon without bgp-role answers the error,
// and the relation member, declared to read the field, then fails closed.
var srcPeerRoleReader = sync.OnceValues(func() (policyview.Reader, error) {
	meta, found := policyview.MetadataNamed(srcPeerRoleName)
	if !found {
		return policyview.Reader{}, errors.New("bgp-filter-community: no plugin registered the " + srcPeerRoleName + " metadata")
	}
	return policyview.ReaderFor(meta.Field())
})

// readCommunitySession reads the session from the engine's view on the fast
// path, else from the fields object a forked call carries. A field the
// filter did not declare reads as absent on both.
func readCommunitySession(route *sdk.FilterRoute) (communitySession, error) {
	if view, isView := route.Engine().(*policyview.View); isView {
		return viewCommunitySession(view, route.Filter())
	}
	raw, err := route.Fields()
	if err != nil {
		return communitySession{}, err
	}
	return fieldsCommunitySession(raw)
}

// viewCommunitySession reads the facts the named filter declared.
func viewCommunitySession(view *policyview.View, filter string) (communitySession, error) {
	var s communitySession
	var errs [6]error
	var value policyview.Value
	value, errs[0] = view.Read(communityReaders.peerName)
	s.peer.Name = value.Text()
	value, errs[1] = view.Read(communityReaders.peerGroup)
	s.peer.GroupName = value.Text()
	switch filter {
	case ingressFilterName:
		value, errs[2] = view.Read(communityReaders.localAS)
		s.peer.LocalAS = viewASN(value)
		value, errs[3] = view.Read(communityReaders.peerAS)
		s.peer.PeerAS = viewASN(value)
		value, errs[4] = view.Read(communityReaders.routeServerAS)
		s.peer.RouteServerAS = viewASN(value)
	case relationFilterName:
		value, errs[2] = view.Read(communityReaders.localAS)
		s.peer.LocalAS = viewASN(value)
		value, errs[3] = view.Read(communityReaders.ibgp)
		s.peer.IsIBGP = value.Flag()
		var reader policyview.Reader
		if reader, errs[4] = srcPeerRoleReader(); errs[4] == nil {
			value, errs[5] = view.Read(reader)
			s.srcPeerRole = value.Text()
		}
	case egressFilterName, blackholeFilterName:
		value, errs[2] = view.Read(communityReaders.peerAddress)
		s.peer.Address = value.Address()
		value, errs[3] = view.Read(communityReaders.ibgp)
		s.peer.IsIBGP = value.Flag()
		value, errs[4] = view.Read(communityReaders.rsClient)
		s.peer.RSClient = value.Flag()
	}
	if err := errors.Join(errs[:]...); err != nil {
		return communitySession{}, fmt.Errorf("bgp-filter-community: %s: the session cannot be read: %w", filter, err)
	}
	return s, nil
}

// viewASN answers an AS fact; the facts are 32-bit numbers by construction.
func viewASN(value policyview.Value) uint32 {
	asn, _ := value.Number() // an absent fact reads as no AS, which every decision reads as unknown
	return uint32(asn)       //nolint:gosec // G115: an AS fact holds a 32-bit AS number
}

// communityFieldsJSON is the fields object of a forked call, keyed by the
// field names communityFilterDecls declares.
type communityFieldsJSON struct {
	PeerName      string `json:"peer-name"`
	PeerGroup     string `json:"peer-group"`
	PeerAddress   string `json:"peer-address"`
	LocalAS       uint32 `json:"local-as"`
	PeerAS        uint32 `json:"peer-as"`
	RouteServerAS uint32 `json:"route-server-as"`
	IBGP          bool   `json:"ibgp"`
	RSClient      bool   `json:"rs-client"`
	SrcPeerRole   string `json:"src-peer-role"`
}

func fieldsCommunitySession(raw []byte) (communitySession, error) {
	var fields communityFieldsJSON
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &fields); err != nil {
			return communitySession{}, fmt.Errorf("bgp-filter-community: the fields object cannot be read: %w", err)
		}
	}
	s := communitySession{srcPeerRole: fields.SrcPeerRole, peer: filterapi.PeerFilterInfo{
		Name: fields.PeerName, GroupName: fields.PeerGroup, LocalAS: fields.LocalAS, PeerAS: fields.PeerAS,
		RouteServerAS: fields.RouteServerAS, IsIBGP: fields.IBGP, RSClient: fields.RSClient,
	}}
	if fields.PeerAddress == "" {
		return s, nil
	}
	addr, err := netip.ParseAddr(fields.PeerAddress)
	if err != nil {
		return communitySession{}, fmt.Errorf("bgp-filter-community: peer-address %q: %w", fields.PeerAddress, err)
	}
	s.peer.Address = addr
	return s, nil
}
