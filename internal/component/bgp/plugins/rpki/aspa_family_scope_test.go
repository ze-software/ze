// RFC: rfc/short/draft-ietf-sidrops-aspa-verification.md -- Section 6.2, the families ASPA verifies
// Related: rpki.go -- handleStructuredUpdate, the producer under test
// Related: aspa_verify.go -- aspaAppliesTo, the family gate it reads

package rpki

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// aspaInvalidPlugin returns an active plugin with ASPA enabled and a cache under
// which the path [100, 200, 300] is Invalid: 200 authorizes 100, and 300 holds
// an ASPA record that does not name 200.
func aspaInvalidPlugin(t *testing.T) *rPKIPlugin {
	t.Helper()
	rp, _ := aspaRolePlugin(t, "provider")
	rp.aspaCache.Set(200, []uint32{100})
	rp.aspaCache.Set(300, []uint32{999})
	return rp
}

// aspaRolePlugin applies a real dynamic-group role and the global ASPA policy.
func aspaRolePlugin(t *testing.T, role string) (*rPKIPlugin, *rpc.DirectBridge) {
	t.Helper()
	cfg, err := parseRPKIConfig(fmt.Sprintf(`{
		"rpki": {"aspa": {"validation": "true"}, "action": {"invalid": "reject"}, "cache-server": {"192.0.2.1": {"trusted-network": "true"}}},
		"group": {"ix": {"connection": {"remote": {"ip": "dynamic"}}, "role": {"import": %q}}}
	}`, role))
	require.NoError(t, err)
	rp, bridge := groupMemberPlugin(t)
	rp.aspaEnabled.Store(cfg.ASPAValidation)
	rp.aspaInvalidAction.Store(uint32(cfg.ASPAInvalidAction))
	rp.aspaUnknownAction.Store(uint32(cfg.ASPAUnknownAction))
	rp.originInvalidAction.Store(uint32(cfg.OriginInvalidAction))
	rp.originNotFoundAction.Store(uint32(cfg.OriginNotFoundAction))
	rp.perPeerActions.Store(&cfg.PeerActions)
	return rp, bridge
}

// aspaMPReachBody builds an UPDATE body with ORIGIN, AS_PATH [100 200 300] and
// one MP_REACH_NLRI for afi/safi carrying nextHop and nlri.
func aspaMPReachBody(afi uint16, safi byte, nextHop, nlri []byte) []byte {
	mpReach := []byte{byte(afi >> 8), byte(afi), safi, byte(len(nextHop))} //nolint:gosec // test next hop is short
	mpReach = append(mpReach, nextHop...)
	mpReach = append(mpReach, 0x00) // Reserved
	mpReach = append(mpReach, nlri...)

	attrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x0e, 0x02, 0x03, // AS_PATH: one AS_SEQUENCE of three 4-octet ASNs
		0x00, 0x00, 0x00, 0x64, // 100
		0x00, 0x00, 0x00, 0xc8, // 200
		0x00, 0x00, 0x01, 0x2c, // 300
	}
	attrs = append(attrs, 0x80, 0x0e, byte(len(mpReach))) //nolint:gosec // test NLRI is short
	attrs = append(attrs, mpReach...)

	body := []byte{0x00, 0x00, byte(len(attrs) >> 8), byte(len(attrs))} //nolint:gosec // test attrs are short
	return append(body, attrs...)
}

// feedASPAUpdate drives body through handleStructuredUpdate from peer AS 100 and
// returns the validation requests it produced.
func feedASPAUpdate(t *testing.T, rp *rPKIPlugin, body []byte) []validationRequest {
	t.Helper()
	ctxID, _ := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))
	wu := wireu.NewWireUpdate(body, ctxID)
	attrs, err := wu.Attrs()
	require.NoError(t, err, "the test UPDATE body must parse")

	rp.handleStructuredUpdate(&rpc.StructuredEvent{
		EventType:   rpc.EventKindUpdate,
		PeerAddress: "192.0.2.50",
		PeerName:    "ix-192.0.2.50",
		PeerGroup:   "ix",
		PeerAS:      100,
		LocalAS:     65000,
		MessageID:   71,
		RawMessage: &bgptypes.RawMessage{
			Type:       msgtype.TypeUPDATE,
			RawBytes:   body,
			WireUpdate: wu,
			AttrsWire:  attrs,
		},
	})
	return drainRequests(rp.validateCh)
}

// TestASPAAppliesToIPv6Unicast proves an IPv6 unicast route carried in
// MP_REACH_NLRI (AFI 2 / SAFI 1) is run through ASPA verification: the Invalid
// verdict reaches the decision path and the route is tracked for
// re-validation.
//
// VALIDATES: handleStructuredUpdate hands the MP_REACH family's routes to
// validateNLRIs with the verified ASPA state, and to trackNLRIs, when the
// family is IPv6 unicast.
// PREVENTS: a family gate that only lets the plain-NLRI IPv4 unicast branch
// through, leaving IPv6 unicast unverified.
//
// RFC requirement: DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-6.2-1 positive -- an IPv6 unicast route (AFI 2, SAFI 1) in MP_REACH_NLRI whose AS_PATH fails the ASPA check reaches the decision path with ASPA state Invalid and is tracked for re-validation (Section 6.2).
// RFC requirement: DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-6.2-2 positive -- the family gate keeps IPv6 unicast (AFI 2, SAFI 1) inside the verified set: its route carries the Invalid ASPA state rather than none (Section 6.2).
func TestASPAAppliesToIPv6Unicast(t *testing.T) {
	rp := aspaInvalidPlugin(t)

	nextHop := []byte{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}
	nlri := []byte{0x20, 0x20, 0x01, 0x0d, 0xb8} // 2001:db8::/32
	reqs := feedASPAUpdate(t, rp, aspaMPReachBody(2, 1, nextHop, nlri))

	require.Len(t, reqs, 1, "the IPv6 unicast prefix must reach the decision path")
	assert.Equal(t, "ipv6/unicast", reqs[0].family)
	assert.Equal(t, "2001:db8::/32", reqs[0].prefix)
	assert.Equal(t, ASPAInvalid, reqs[0].aspaState,
		"ASPA verification was not applied to the IPv6 unicast route")
	assert.Equal(t, 1, rp.aspaTracker.count(),
		"an ASPA-verified IPv6 unicast route is tracked for re-validation")
}

// TestASPAAppliesToIPv4Unicast is the same proof over the plain-NLRI branch:
// an IPv4 unicast route in the UPDATE's own NLRI field carries the Invalid
// ASPA state.
//
// RFC requirement: DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-6.2-1 negative -- an IPv4 unicast route whose AS_PATH fails the ASPA check is not let through unverified: it reaches the decision path with ASPA state Invalid, never with none (Section 6.2).
func TestASPAAppliesToIPv4Unicast(t *testing.T) {
	rp := aspaInvalidPlugin(t)

	body := []byte{
		0x00, 0x00, // Withdrawn Routes length 0
		0x00, 0x1c, // Total Path Attribute Length 28
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x0e, 0x02, 0x03, // AS_PATH [100 200 300]
		0x00, 0x00, 0x00, 0x64,
		0x00, 0x00, 0x00, 0xc8,
		0x00, 0x00, 0x01, 0x2c,
		0x40, 0x03, 0x04, 0x0a, 0x00, 0x00, 0x01, // NEXT_HOP = 10.0.0.1
		0x18, 0x0a, 0x00, 0x00, // NLRI 10.0.0.0/24
	}
	reqs := feedASPAUpdate(t, rp, body)

	require.Len(t, reqs, 1, "the IPv4 unicast prefix must reach the decision path")
	assert.Equal(t, "ipv4/unicast", reqs[0].family)
	assert.Equal(t, ASPAInvalid, reqs[0].aspaState,
		"ASPA verification was not applied to the IPv4 unicast route")
}

// TestASPANotAppliedToOtherFamilies proves a route of another address family
// carries no ASPA state even when its AS_PATH would be Invalid: the decision
// path sees aspaStateNone and nothing is tracked for re-validation, so no ASPA
// action can exclude it.
//
// VALIDATES: aspaAppliesTo gates the MP_REACH branch of handleStructuredUpdate
// for IPv4 flowspec (SAFI 133) and VPN-IPv4 (SAFI 128).
// PREVENTS: the ASPA verdict computed once per UPDATE leaking onto every
// family the UPDATE carries.
//
// RFC requirement: DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-6.2-2 negative -- an IPv4 flowspec route (AFI 1, SAFI 133) and a VPN-IPv4 route (AFI 1, SAFI 128) whose AS_PATH fails the ASPA check reach the decision path with no ASPA state and are not tracked for re-validation (Section 6.2).
func TestASPANotAppliedToOtherFamilies(t *testing.T) {
	cases := []struct {
		name    string
		safi    byte
		nextHop []byte
		nlri    []byte
		family  string
	}{
		{
			name:   "ipv4 flowspec",
			safi:   133,
			nlri:   []byte{0x05, 0x01, 0x18, 0x0a, 0x00, 0x00}, // destination 10.0.0.0/24
			family: "ipv4/flow",
		},
		{
			name:    "vpn-ipv4",
			safi:    128,
			nextHop: []byte{0, 0, 0, 0, 0, 0, 0, 0, 10, 0, 0, 1}, // RD 0 + 10.0.0.1
			// Label 100 (bottom of stack), RD 0:0, 10.0.0.0/8: length 24+64+8.
			nlri:   []byte{96, 0x00, 0x06, 0x41, 0, 0, 0, 0, 0, 0, 0, 0, 0x0a},
			family: "ipv4/mpls-vpn",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rp := aspaInvalidPlugin(t)
			reqs := feedASPAUpdate(t, rp, aspaMPReachBody(1, tc.safi, tc.nextHop, tc.nlri))

			require.NotEmpty(t, reqs, "the %s route must reach the decision path", tc.family)
			for i := range reqs {
				assert.Equal(t, aspaStateNone, reqs[i].aspaState,
					"ASPA verification was applied to a %s route", tc.family)
			}
			assert.Equal(t, 0, rp.aspaTracker.count(),
				"a %s route must not be tracked for ASPA re-validation", tc.family)
		})
	}
}

// TestASPAUpstreamRoles verifies the configured local role selects upstream
// verification on the received route, including a transparent route server.
func TestASPAUpstreamRoles(t *testing.T) {
	// Section 5.5 applies upstream verification to these receiving roles:
	// authorized paths are Valid; a denied upstream hop remains Invalid.
	for _, role := range []string{"provider", "peer", "rs", "rs-client"} {
		t.Run(role, func(t *testing.T) {
			rp, _ := aspaRolePlugin(t, role)
			rp.aspaCache.Set(200, []uint32{100})
			rp.aspaCache.Set(300, []uint32{200})
			body := aspaMPReachBody(1, 1, []byte{192, 0, 2, 50}, []byte{24, 10, 0, 0})
			reqs := feedASPAUpdate(t, rp, body)
			require.Len(t, reqs, 1)
			assert.Equal(t, ASPAValid, reqs[0].aspaState)
			assert.True(t, rp.buildDecisions(reqs)[0].Accept)

			rp.aspaCache.Set(300, []uint32{999})
			reqs = feedASPAUpdate(t, rp, body)
			require.Len(t, reqs, 1)
			assert.Equal(t, ASPAInvalid, reqs[0].aspaState)
			assert.False(t, rp.buildDecisions(reqs)[0].Accept)
		})
	}
}

// TestASPADownstreamRole accepts a provider path whose ramps meet at AS200.
// Interpreting the local customer role as upstream verification rejects it.
func TestASPADownstreamRole(t *testing.T) {
	rp, _ := aspaRolePlugin(t, "customer")
	rp.aspaCache.Set(100, []uint32{200})
	rp.aspaCache.Set(200, []uint32{0})
	rp.aspaCache.Set(300, []uint32{200})
	reqs := feedASPAUpdate(t, rp,
		aspaMPReachBody(1, 1, []byte{192, 0, 2, 50}, []byte{24, 10, 0, 0}))
	require.Len(t, reqs, 1)
	assert.Equal(t, ASPAValid, reqs[0].aspaState)
	assert.True(t, rp.buildDecisions(reqs)[0].Accept)
}

// TestASPARecoveryKeepsCurrentOriginVerdict drives the real receive path and
// both cache callbacks; repairing one validation result cannot erase the other.
func TestASPARecoveryKeepsCurrentOriginVerdict(t *testing.T) {
	rp := aspaInvalidPlugin(t)
	rp.originInvalidAction.Store(uint32(ASPAPolicyReject))
	rp.cache.Replace([]VRP{makeVRP("10.0.0.0/24", 24, 300)})
	reqs := feedASPAUpdate(t, rp,
		aspaMPReachBody(1, 1, []byte{192, 0, 2, 50}, []byte{24, 10, 0, 0}))
	require.Len(t, reqs, 1)
	initial := rp.buildDecisions(reqs)[0]
	assert.False(t, initial.Accept)
	assert.True(t, initial.Ineligible)
	assert.Equal(t, ValidationValid, initial.ValState)

	rp.aspaCache.Set(300, []uint32{200})
	rp.handleASPAChange([]uint32{300})
	reqs = drainRequests(rp.validateCh)
	require.Len(t, reqs, 1)
	repaired := rp.buildDecisions(reqs)[0]
	assert.True(t, repaired.Accept)
	assert.False(t, repaired.Ineligible)
	assert.Equal(t, initial.MsgID, repaired.MsgID)

	rp.cache.Replace([]VRP{makeVRP("10.0.0.0/24", 24, 999)})
	rp.handleROAChange()
	reqs = drainRequests(rp.validateCh)
	require.Len(t, reqs, 1)
	assert.False(t, rp.buildDecisions(reqs)[0].Accept)
	rp.aspaCache.Set(300, []uint32{999})
	rp.handleASPAChange([]uint32{300})
	drainRequests(rp.validateCh)
	rp.aspaCache.Set(300, []uint32{200})
	rp.handleASPAChange([]uint32{300})
	reqs = drainRequests(rp.validateCh)
	require.Len(t, reqs, 1)
	originRejected := rp.buildDecisions(reqs)[0]
	assert.False(t, originRejected.Accept)
	assert.True(t, originRejected.Ineligible)
	assert.Equal(t, ValidationInvalid, originRejected.ValState)

	// A later origin repair sees the latest ASPA verdict, not the original Invalid.
	rp.cache.Replace([]VRP{makeVRP("10.0.0.0/24", 24, 300)})
	rp.handleROAChange()
	reqs = drainRequests(rp.validateCh)
	require.Len(t, reqs, 1)
	assert.True(t, rp.buildDecisions(reqs)[0].Accept)
}
