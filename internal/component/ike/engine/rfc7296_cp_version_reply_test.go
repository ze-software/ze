// Design: docs/architecture/ike/ipsec-14-responder.md -- the Configuration payload in IKE_AUTH
// Related: cp.go -- buildCFGReply, the CFG_REPLY these tests read
// Related: rfc7296_cp_version_test.go -- the same request to a peer bound to no pool
// VALIDATES: a pool-bound peer's CFG_REQUEST that asks APPLICATION_VERSION is answered by
// a CFG_REPLY whose APPLICATION_VERSION is the empty string (RFC 7296 Section 2.20).
// PREVENTS: a CFG_REPLY that gives out a version string, echoes the requester's own
// version, or drops the attribute a request asked for.

package engine

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/wire"
)

// versionReplyOf drives a pool-bound peer's IKE_AUTH with one CFG_REQUEST of attrs and
// returns the response's CFG_REPLY. cpAuth fails the test unless the responder's IKE SA
// is established.
func versionReplyOf(t *testing.T, attrs ...wire.ConfigAttr) *wire.PayloadCP {
	t.Helper()
	ini, resp, ps := cpResponder(t, cpVIP(t, cpPool()), "ze", true)
	cp, _, _ := replyOf(cpAuth(t, ini, resp, ps, cfgRequest(attrs...)))
	if cp == nil {
		t.Fatal("the IKE_AUTH response to a pool-bound peer's CFG_REQUEST carries no CP payload")
	}
	if cp.CFGType != wire.CFGTypeReply {
		t.Fatalf("CP CFG type = %d, want CFG_REPLY (%d)", cp.CFGType, wire.CFGTypeReply)
	}
	return cp
}

// TestRFC7296GrantedVersionRequestIsAnsweredEmpty drives a pool-bound peer's IKE_AUTH
// with a CFG_REQUEST that asks an IPv4 address and APPLICATION_VERSION.
//
// Goal: Ze supports CP for a pool-bound peer, so the decline RFC 7296 Section 2.20 allows
// takes its first form, the empty string. Method: the real responder answers the request
// through handleAuthRequest; the CFG_REPLY MUST carry exactly one APPLICATION_VERSION
// attribute, of length zero.
//
// RFC 7296 Section 2.20: "In that case, it MUST either return an empty string or no CP
// payload if CP is not supported."
//
// MUTATION: the guard `grant.req.appVersion` in buildCFGReply (cp.go) set to false drops
// the attribute from the reply, and this test goes red.
//
// RFC requirement: RFC7296-2.20-1 positive -- a pool-bound peer's IKE_AUTH CFG_REQUEST asking an IPv4 address and an empty APPLICATION_VERSION draws, with the responder's IKE SA established, a CFG_REPLY carrying exactly one APPLICATION_VERSION attribute whose value is zero octets long.
func TestRFC7296GrantedVersionRequestIsAnsweredEmpty(t *testing.T) {
	cp := versionReplyOf(t, ip4Ask(netip.Addr{}), wire.ConfigAttr{Type: wire.CPAttrApplicationVersion})
	versions := attrsOf(cp, wire.CPAttrApplicationVersion)
	if len(versions) != 1 {
		t.Fatalf("CFG_REPLY carries %d APPLICATION_VERSION attributes, want exactly one", len(versions))
	}
	if len(versions[0]) != 0 {
		t.Fatalf("APPLICATION_VERSION = %q, want the empty string: Ze declines to give out its version", versions[0])
	}
}

// TestRFC7296GrantedVersionRequestReflectsNoVersionString drives a pool-bound peer's
// IKE_AUTH with a CFG_REQUEST whose APPLICATION_VERSION carries the requester's own
// version string.
//
// Goal: the decline holds whatever the request carried, so no version string reaches the
// reply: neither Ze's own nor the requester's echoed back. Method: the request carries an
// IPv4 address ask and APPLICATION_VERSION "strongSwan 6.0.1"; the CFG_REPLY MUST carry
// exactly one APPLICATION_VERSION attribute, of length zero, and the IPv4 address the
// request asked.
//
// RFC 7296 Section 2.20: "In that case, it MUST either return an empty string or no CP
// payload if CP is not supported."
//
// MUTATION: the APPLICATION_VERSION reply attribute in buildCFGReply (cp.go) given the
// value []byte("ze") puts a version string in the reply, and this test goes red.
//
// RFC requirement: RFC7296-2.20-1 negative -- a pool-bound peer's IKE_AUTH CFG_REQUEST whose APPLICATION_VERSION carries the requester's version string "strongSwan 6.0.1" draws, with the responder's IKE SA established, a CFG_REPLY carrying one INTERNAL_IP4_ADDRESS and exactly one APPLICATION_VERSION attribute whose value is zero octets long, so the reply carries no version string.
func TestRFC7296GrantedVersionRequestReflectsNoVersionString(t *testing.T) {
	cp := versionReplyOf(t, ip4Ask(netip.Addr{}),
		wire.ConfigAttr{Type: wire.CPAttrApplicationVersion, Value: []byte("strongSwan 6.0.1")})
	if addrs := attrsOf(cp, wire.CPAttrInternalIP4Address); len(addrs) != 1 {
		t.Fatalf("CFG_REPLY carries %d INTERNAL_IP4_ADDRESS attributes, want one: the request was not granted", len(addrs))
	}
	versions := attrsOf(cp, wire.CPAttrApplicationVersion)
	if len(versions) != 1 {
		t.Fatalf("CFG_REPLY carries %d APPLICATION_VERSION attributes, want exactly one", len(versions))
	}
	if len(versions[0]) != 0 {
		t.Fatalf("APPLICATION_VERSION = %q, want the empty string: the reply must carry no version string", versions[0])
	}
}
