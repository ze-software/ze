// Design: docs/architecture/wire/attributes.md -- RFC 9774 Section 3, received AS_SET
// Related: config_as_set.go -- applyASSetPolicy, the leaf this test drives
// Related: rfc9774_receive_test.go -- the treat-as-withdraw half, without the leaf

package reactor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/capability"
)

// asSetPeerTree is a minimal eBGP peer config carrying `session as-set <policy>`,
// or no leaf at all when policy is empty.
func asSetPeerTree(policy string) map[string]any {
	session := map[string]any{
		"asn":       map[string]any{"remote": "65002", "local": "65001"},
		"router-id": "1.2.3.1",
	}
	if policy != "" {
		session["as-set"] = policy
	}
	return map[string]any{
		"connection": map[string]any{
			"remote": map[string]any{"ip": "192.0.2.1", "connect": "false"},
			"local":  map[string]any{"ip": "192.0.2.2"},
		},
		"session": session,
	}
}

// TestRFC9774ReceiveASSetAcceptedByOperator proves the operator exception of RFC 9774
// Section 3 ("Unless explicitly configured by a network operator to do otherwise"):
// a peer whose config says `session as-set accept` keeps an UPDATE whose AS_PATH or
// AS4_PATH holds an AS_SET or AS_CONFED_SET, and a peer with `withdraw` or no leaf
// treats the same bytes as withdrawn. Method: parse the peer from a config tree with
// parsePeerFromTree, then drive the receive rail (enforceRFC7606) with each UPDATE.
// The leaf keeps every other check: AS 0 in the same path is still treat-as-withdraw.
//
// RFC requirement: RFC9774-3-2 positive -- a peer parsed from config with session as-set
// withdraw or no as-set leaf draws treat-as-withdraw from Session.enforceRFC7606 for an
// AS_SET or AS_CONFED_SET in AS_PATH and an AS_SET in AS4_PATH.
func TestRFC9774ReceiveASSetAcceptedByOperator(t *testing.T) {
	nlri := []byte{24, 192, 0, 2}
	setPath := append(rfc9774Segment(rfc9774SegSequence, 4, 65002), rfc9774Segment(rfc9774SegSet, 4, 65020, 65021)...)
	confedPath := append(rfc9774Segment(rfc9774SegSequence, 4, 65002), rfc9774Segment(rfc9774SegConfedSet, 4, 65020)...)
	zeroPath := append(rfc9774Segment(rfc9774SegSequence, 4, 65002), rfc9774Segment(rfc9774SegSet, 4, 0)...)
	as4Set := collapseAttr(0xc0, 17, append(rfc9774Segment(rfc9774SegSequence, 4, 65002),
		rfc9774Segment(rfc9774SegSet, 4, 4200000001)...))

	for _, tc := range []struct {
		policy string
		accept bool
	}{
		{"", false},
		{asSetPolicyWithdraw, false},
		{asSetPolicyAccept, true},
	} {
		ps, err := parsePeerFromTree("peer1", asSetPeerTree(tc.policy), 65001, 0x01020301)
		require.NoError(t, err, "policy %q", tc.policy)
		require.Equal(t, tc.accept, ps.AcceptASSet, "policy %q", tc.policy)
		s, _ := firstASSession(t, ps, []capability.Capability{&capability.ASN4{ASN: 65002}}, nil)

		want := message.RFC7606ActionTreatAsWithdraw
		if tc.accept {
			want = message.RFC7606ActionNone
		}
		for name, attrs := range map[string][]byte{
			"AS_SET in AS_PATH":        rfc9774Attrs(setPath),
			"AS_CONFED_SET in AS_PATH": rfc9774Attrs(confedPath),
		} {
			_, action, err := s.enforceRFC7606(wireu.NewWireUpdate(makeUpdateBody(nil, attrs, nlri), 0))
			require.NoError(t, err)
			assert.Equal(t, want, action, "policy %q: %s", tc.policy, name)
		}

		// AS4_PATH reaches its validator on every session; this row drives a 2-octet one,
		// where the AS4_PATH is consumed rather than ignored.
		s2 := rfc7607Session()
		s2.settings.AcceptASSet = ps.AcceptASSet
		asPath := rfc9774Segment(rfc9774SegSequence, 2, 65002, 23456)
		_, action, err := s2.enforceRFC7606(wireu.NewWireUpdate(makeUpdateBody(nil, rfc9774Attrs(asPath, as4Set...), nlri), 0))
		require.NoError(t, err)
		assert.Equal(t, want, action, "policy %q: AS_SET in AS4_PATH", tc.policy)

		_, action, err = s.enforceRFC7606(wireu.NewWireUpdate(makeUpdateBody(nil, rfc9774Attrs(zeroPath), nlri), 0))
		require.NoError(t, err)
		assert.Equal(t, message.RFC7606ActionTreatAsWithdraw, action,
			"policy %q: AS 0 inside a set stays treat-as-withdraw (RFC 7607 Section 2)", tc.policy)
	}
}

// TestRFC9774ASSetPolicyRefusesUnknownValue proves a value outside withdraw and accept
// stops the peer's config, naming both.
func TestRFC9774ASSetPolicyRefusesUnknownValue(t *testing.T) {
	_, err := parsePeerFromTree("peer1", asSetPeerTree("keep"), 65001, 0x01020301)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "use withdraw or accept")
}
