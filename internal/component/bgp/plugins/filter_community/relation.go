// Design: docs/architecture/core-design.md — community filter ingress path
// RFC: rfc/short/rfc8195.md — Section 3.2, the informational "relation to origin" function
// RFC: rfc/short/rfc9234.md — Section 4.1 Table 1, the role names this maps from
// Overview: filter_community.go — plugin entry point
// Related: filter.go — the ingress pass this tag is written by
// Related: docs/architecture/meta/filter-community.md — the meta key this reads

package filter_community

import "encoding/binary"

// Role name tokens, spelled here rather than imported from the role plugin.
//
// This is the agreed-spelling pattern ai/rules/plugins.md states for
// cross-plugin coupling: the producer and the consumer agree on a token,
// and the consumer spells it itself. Deleting
// internal/component/bgp/plugins/role therefore leaves this package
// building. The meta key never appears, relationParameterFor is
// never reached with a role, and no tag is written. Importing the role
// package would make one plugin a compile-time dependency of another, which
// is the coupling "delete the folder and the feature vanishes" forbids.
//
// The tokens themselves are RFC 9234 Section 4.1 Table 1 role names, not a
// private vocabulary. So the two spellings cannot drift without the RFC
// changing.
const (
	roleProvider = "provider"
	roleCustomer = "customer"
	rolePeer     = "peer"
	roleRS       = "rs"
	roleRSClient = "rs-client"
)

// RFC 8195 Section 3.2 parameters for the "relation to origin" function.
//
// RFC 8195 Section 3.2 states that an AS "could assign" a function number
// to denote the relation of a route to its origin and use the parameter to
// carry the relation itself. The parameter values below are the ones that
// section tabulates. The FUNCTION number is not fixed here: RFC 8195 makes
// it a local convention, so it is configuration (relationFunction),
// defaulting to 3.
const (
	// relationInternal marks a route the local AS originated. It is not
	// written here: it is a property of route origination rather than of any
	// peer. So no per-peer ingress filter can produce it (see the spec's
	// Known Limitations).
	relationInternal uint32 = 1
	relationCustomer uint32 = 2 // learned from a customer
	relationPeer     uint32 = 3 // learned from a peer
	relationProvider uint32 = 4 // learned from a provider (transit upstream)
)

// relationParameterFor maps what the source peer IS to us onto RFC 8195
// Section 3.2 parameter, or 0 when no tag may be written.
//
// The input is the resolved peer role (meta key "src-peer-role"), never the
// locally configured role. RFC 9234 Section 4.2 Table 2 pairs the two. The
// role plugin's resolver already holds that table AND prefers a Role
// capability the peer announced over the configuration. Complementing the
// configured role here would duplicate the table and would disagree with
// the resolver for any peer that advertises a role.
//
// It FAILS CLOSED, which is why it is a switch over known values rather
// than a map lookup with a default. Zero means "write nothing":
//
//   - "" — the role could not be resolved. A guess would put a wrong relation on
//     the route, and every downstream policy keyed on it would act on that guess.
//   - "rs", "rs-client" — RFC 7947 requires route-server transparency. Writing an
//     attribute at a route server is the behavior that document forbids, and
//     there is in any case no relation to state: a route server is not in the
//     customer/peer/provider lattice RFC 8195 Section 3.2 describes.
//   - anything else — an unrecognized token is not a license to invent a value.
func relationParameterFor(peerRole string) uint32 {
	switch peerRole {
	case roleCustomer:
		return relationCustomer
	case rolePeer:
		return relationPeer
	case roleProvider:
		return relationProvider
	case roleRS, roleRSClient:
		return 0
	}
	return 0
}

// relationWireValue builds the twelve wire bytes of the large community
// <globalAdmin>:<function>:<parameter> (RFC 8092 Section 3: three
// four-octet fields, Global Administrator first, in network byte order).
func relationWireValue(globalAdmin, function, parameter uint32) []byte {
	buf := make([]byte, 12)
	binary.BigEndian.PutUint32(buf[0:4], globalAdmin)
	binary.BigEndian.PutUint32(buf[4:8], function)
	binary.BigEndian.PutUint32(buf[8:12], parameter)
	return buf
}

// relationPeerRoleFromMeta reads the resolved source-peer role the role
// plugin publishes. It returns "" for a missing key and for a key of the
// wrong type: docs/architecture/meta/README.md makes the declared type the
// contract and requires a reader to treat a wrong type as absent. Absent
// here means no tag, which is the closed branch.
func relationPeerRoleFromMeta(meta map[string]any) string {
	raw, ok := meta["src-peer-role"]
	if !ok {
		return ""
	}
	role, ok := raw.(string)
	if !ok {
		return ""
	}
	return role
}
