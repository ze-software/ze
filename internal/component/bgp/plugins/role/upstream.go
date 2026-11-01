// Design: docs/guide/bgp-role.md -- the upstream check a role inserts
// Related: config_tx.go -- parseRoleConfig derives the set this check compares
// Related: register.go -- the filter, its insertions and the asn-set reference source

package role

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"sync/atomic"

	"github.com/ze-software/ze/internal/component/bgp/configjson"
	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

const (
	// upstreamFilterName is the filter a role peer's list carries as
	// bgp-role:upstream-asn, inserted by upstreamInsertions.
	upstreamFilterName = "upstream-asn"
	// upstreamReference is that member's chain reference.
	upstreamReference = "bgp-role:" + upstreamFilterName
	// upstreamPriority places the member among inserted members.
	upstreamPriority = 200
	// asnSetList is the policy list role transit-asns names (ze-role.yang).
	asnSetList = "asn-set"
	// asnSetReferenceSource is the ze:reference source of transit-asns.
	asnSetReferenceSource = "policy-asn-set"
	// fieldASPath is the route field the check reads.
	fieldASPath = "as-path"
)

// upstreamImportRoles and upstreamExportRoles are the local roles (role
// import) whose sessions carry the check, in each direction.
//
// RFC 7454 Section 9: "This loose policy could be combined with filters for
// specific 2-byte or 4-byte AS paths that must not be accepted if advertised
// by the customer, such as upstream transit providers or peer ASNs." The
// local role provider is the session to a customer. RFC 9234 Section 3.1:
// "Peer:  MAY propagate any route learned from a Customer, or that is
// locally originated, to a Peer.  All other routes MUST NOT be propagated."
// and the same of an RS-Client toward an RS: a route a peer or an RS-client
// sends through Ze's own upstream was learned from neither, so the local
// roles peer and rs carry the import check too.
//
// RFC 7454 Section 9: "Network administrators SHOULD NOT advertise prefixes
// with upstream AS numbers in the AS path to their peering AS unless they
// intend to provide transit for these prefixes." The peering sessions are
// the local roles peer and rs-client (toward a route server).
//
// The local role customer is the session to a provider, which carries no
// import check: every route it sends holds its own ASN.
var (
	upstreamImportRoles = []string{roleProvider, rolePeer, roleRS}
	upstreamExportRoles = []string{rolePeer, roleRSClient}
)

// upstreamFilterDecls is the plugin's one filter declaration. It has two
// readers and MUST stay one function: register.go puts it on
// registry.Registration.Filters, which config load reads before the plugin
// runs, and RunRolePlugin sends it at Stage 1. A malformed AS path fails
// closed.
func upstreamFilterDecls() []sdk.FilterDecl {
	return []sdk.FilterDecl{{
		Name:      upstreamFilterName,
		Direction: sdk.FilterBoth,
		Reads:     []string{fieldASPath},
		OnError:   sdk.OnErrorReject,
	}}
}

// upstreamInsertions places the check on every peer whose role import is a
// role upstreamImportRoles or upstreamExportRoles names, inherited from the
// group like the rest of role. It is not required: RFC 7454 is a Best
// Current Practice whose Section 9 "lists the RECOMMENDED practices", so an
// operator removes the member with `inactive:bgp-role:upstream-asn`. Like
// upstreamFilterDecls it has two readers and MUST stay one function.
func upstreamInsertions() []rpc.FilterInsertion {
	return []rpc.FilterInsertion{
		{
			Filter: upstreamReference, Direction: rpc.FilterImport, Priority: upstreamPriority,
			Config: []string{"role", "import"}, Values: upstreamImportRoles,
		},
		{
			Filter: upstreamReference, Direction: rpc.FilterExport, Priority: upstreamPriority,
			Config: []string{"role", "import"}, Values: upstreamExportRoles,
		},
	}
}

// upstreamSet is the router's upstream ASNs, sorted, for binary search.
// Immutable once published.
type upstreamSet []uint32

// upstreamASNs is the set the check compares, swapped whole by
// installRoleConfig. nil compares nothing. Safe for concurrent use: the
// evaluator reads one snapshot per route.
var upstreamASNs atomic.Pointer[upstreamSet]

func installUpstreamSet(set upstreamSet) { upstreamASNs.Store(&set) }

// deriveUpstreamSet builds the set from the bgp and policy roots: the remote
// ASN of every peer whose role import is customer, and every ASN of each
// asn-set a transit-asns leaf names on any peer or group.
//
// RFC 9234 Section 3.1: "Customer:  the local AS is a transit customer of the
// remote AS;" so the remote AS of such a session is a transit provider.
//
// A transit-asns naming no asn-set is refused, so the commit fails rather
// than run a check missing the providers the operator listed.
func deriveUpstreamSet(bgpSubtree, policyRoot map[string]any) (upstreamSet, error) {
	var set upstreamSet
	var setErr error
	configjson.ForEachPeer(bgpSubtree, func(peerAddr string, peerMap, groupMap map[string]any, _ configjson.PeerOrigin) {
		if setErr != nil {
			return
		}
		roleMap := effectiveRoleMap(peerMap, groupMap)
		if roleMap == nil {
			return
		}
		if roleName, _ := roleMap["import"].(string); roleName == roleCustomer {
			asn, configured, err := remoteASN(peerMap, groupMap)
			if err != nil {
				setErr = fmt.Errorf("bgp-role: peer %s has role import customer, so its remote ASN is an upstream, "+
					"and it cannot be read: %w; fix: set session asn remote to the provider's ASN", peerAddr, err)
				return
			}
			// A peer whose AS arrives in the OPEN (a dynamic group's members,
			// RFC 4271 Section 4.2) names no ASN the config can hold; its
			// provider goes in an asn-set through transit-asns.
			if configured {
				set = append(set, asn)
			}
		}
		for _, m := range []map[string]any{peerMap, groupMap} {
			names, err := transitSetASNs(m, policyRoot)
			if err != nil {
				setErr = fmt.Errorf("bgp-role: peer %s: %w", peerAddr, err)
				return
			}
			set = append(set, names...)
		}
	})
	if setErr != nil {
		return nil, setErr
	}
	slices.Sort(set)
	return slices.Compact(set), nil
}

// effectiveRoleMap answers the role container that applies to a peer: its
// own, else its group's.
func effectiveRoleMap(peerMap, groupMap map[string]any) map[string]any {
	if roleMap, ok := peerMap["role"].(map[string]any); ok {
		return roleMap
	}
	roleMap, _ := groupMap["role"].(map[string]any) // no group role: the peer declares none
	return roleMap
}

// transitSetASNs answers the ASNs of the asn-set the transit-asns leaf of one
// role container names; none when the container sets no leaf.
func transitSetASNs(m, policyRoot map[string]any) ([]uint32, error) {
	roleMap, _ := m["role"].(map[string]any) // no role container: no leaf
	name, _ := roleMap["transit-asns"].(string)
	if name == "" {
		return nil, nil
	}
	sets, _ := policyRoot[asnSetList].(map[string]any) // no asn-set list: the lookup below refuses
	entry, found := sets[name].(map[string]any)
	if !found {
		return nil, fmt.Errorf("role transit-asns names asn-set %q, which policy does not define; "+
			"fix: add `policy { asn-set %s { asn [ <ASN> ]; } }` or name an existing set", name, name)
	}
	return parseASNList(entry["asn"])
}

// remoteASN reads session asn remote from the peer, else its group;
// configured is false when neither sets it.
func remoteASN(peerMap, groupMap map[string]any) (asn uint32, configured bool, err error) {
	for _, m := range []map[string]any{peerMap, groupMap} {
		session, _ := m["session"].(map[string]any)
		asnMap, _ := session["asn"].(map[string]any)
		if v, present := asnMap["remote"]; present {
			asn, err = parseASN(v)
			return asn, err == nil, err
		}
	}
	return 0, false, nil
}

// parseASNList reads a leaf-list of ASNs, one value or a list.
func parseASNList(v any) ([]uint32, error) {
	values, isList := v.([]any)
	if !isList {
		values = []any{v}
	}
	asns := make([]uint32, 0, len(values))
	for _, value := range values {
		asn, err := parseASN(value)
		if err != nil {
			return nil, err
		}
		asns = append(asns, asn)
	}
	return asns, nil
}

// parseASN reads one ASN delivered as a string (ai/rules/config.md) or a
// JSON number. 0 is reserved (RFC 7607) and refused.
func parseASN(v any) (uint32, error) {
	var text string
	switch value := v.(type) {
	case string:
		text = value
	case float64:
		text = strconv.FormatFloat(value, 'f', -1, 64)
	default:
		return 0, fmt.Errorf("ASN %v is not a number", v)
	}
	asn, err := strconv.ParseUint(text, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("ASN %q is not a 32-bit number: %w", text, err)
	}
	if asn == 0 {
		return 0, errors.New("ASN 0 is reserved (RFC 7607)")
	}
	return uint32(asn), nil
}

// asnSetReference answers the asn-set names the candidate's policy root
// defines, for the transit-asns leaf's completion and commit check.
func asnSetReference(root map[string]any, _ []string) yang.ReferenceAnswer {
	policyCfg, _ := root[policy.RootName].(map[string]any) // no policy root: no set is defined
	sets, _ := policyCfg[asnSetList].(map[string]any)      // no asn-set list: none is defined
	names := make([]string, 0, len(sets))
	for name := range sets {
		names = append(names, name)
	}
	slices.Sort(names)
	return yang.ReferenceAnswer{Names: names}
}

// errUpstreamPathUnread fails the member when the route's AS path cannot be
// read; the declared on-error rejects the route.
var errUpstreamPathUnread = errors.New("bgp-role: upstream check: the route's AS path cannot be read")

// evaluateUpstream rejects a route whose AS path carries an upstream ASN
// other than the session's own remote ASN, which the check never compares
// (D-16: the session to a provider itself). It keeps nothing of the route.
func evaluateUpstream(route *sdk.FilterRoute) (*sdk.FilterUpdateOutput, error) {
	setPtr := upstreamASNs.Load()
	if setPtr == nil {
		return &sdk.FilterUpdateOutput{Action: sdk.FilterAccept}, nil
	}
	set := *setPtr
	if len(set) == 0 {
		return &sdk.FilterUpdateOutput{Action: sdk.FilterAccept}, nil
	}
	asns, err := routeASNs(route)
	if err != nil {
		return nil, errors.Join(errUpstreamPathUnread, err)
	}
	if set.carries(asns, route.PeerAS()) {
		return &sdk.FilterUpdateOutput{Action: sdk.FilterReject}, nil
	}
	return &sdk.FilterUpdateOutput{Action: sdk.FilterAccept}, nil
}

// carries answers whether any ASN of the path, at any position and in any
// segment (RFC 7454 Section 9 rejects a path "containing" the ASN), is in
// the set and is not the session's own remote ASN.
func (s upstreamSet) carries(path []uint32, sessionASN uint32) bool {
	for _, asn := range path {
		if asn == sessionASN {
			continue
		}
		if _, found := slices.BinarySearch(s, asn); found {
			return true
		}
	}
	return false
}

// routeASNs answers every ASN of the route's AS path: from the engine's view
// on the fast path, else from the declared fields a forked call carries.
func routeASNs(route *sdk.FilterRoute) ([]uint32, error) {
	if view, isView := route.Engine().(*policyview.View); isView {
		return viewASNs(view)
	}
	raw, err := route.Fields()
	if err != nil {
		return nil, err
	}
	return fieldASNs(raw)
}

func viewASNs(view *policyview.View) ([]uint32, error) {
	reader, err := policyview.ReaderFor(policy.FieldASPath)
	if err != nil {
		return nil, fmt.Errorf("BUG: bgp-role: %w", err)
	}
	value, err := view.Read(reader)
	if err != nil {
		return nil, err
	}
	return pathASNs(value.ASPath()), nil
}

func pathASNs(path *attribute.ASPath) []uint32 {
	if path == nil {
		return nil
	}
	var asns []uint32
	for _, segment := range path.Segments {
		asns = append(asns, segment.ASNs...)
	}
	return asns
}

// fieldASNs reads as-path from the fields object: a list whose elements are
// ASNs, or lists of ASNs for an AS_SET. Absent means no AS path.
func fieldASNs(raw []byte) ([]uint32, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var fields struct {
		ASPath []json.RawMessage `json:"as-path"`
	}
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, err
	}
	var asns []uint32
	for _, element := range fields.ASPath {
		var asn uint32
		if err := json.Unmarshal(element, &asn); err == nil {
			asns = append(asns, asn)
			continue
		}
		var set []uint32
		if err := json.Unmarshal(element, &set); err != nil {
			return nil, fmt.Errorf("as-path element %s is neither an ASN nor a set: %w", element, err)
		}
		asns = append(asns, set...)
	}
	return asns, nil
}
