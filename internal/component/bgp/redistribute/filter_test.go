package redistribute

import (
	"encoding/binary"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/config/redistribute"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"

	"github.com/stretchr/testify/assert"
)

// buildIPv4Payload builds a minimal UPDATE body with only IPv4 unicast NLRI.
func buildIPv4Payload() []byte {
	// withdrawn len=0, attr len=0, NLRI: 24 10.0.0.0
	return []byte{
		0, 0, // withdrawn routes length
		0, 0, // total path attribute length
		24, 10, 0, 0, // NLRI: /24 10.0.0.0
	}
}

// buildMPReachPayload builds an UPDATE body with MP_REACH_NLRI for the given AFI/SAFI.
func buildMPReachPayload(afi uint16, safi uint8) []byte {
	// withdrawn=0, attrs with MP_REACH_NLRI (code 14)
	// attr value: AFI(2) + SAFI(1) + NH len(1) + NH(4) + reserved(1) + NLRI
	nhLen := byte(4) // IPv4 next-hop
	attrValue := []byte{
		byte(afi >> 8), byte(afi), safi,
		nhLen, 10, 0, 0, 1, // next-hop 10.0.0.1
		0,               // reserved
		24, 192, 168, 1, // NLRI: /24 192.168.1.0
	}
	// Attribute header: flags=0xC0 (optional, transitive), code=14, extended-length
	var attr []byte
	attr = append(attr, 0xC0|0x10, 14) // flags + code
	attr = binary.BigEndian.AppendUint16(attr, uint16(len(attrValue)))
	attr = append(attr, attrValue...)

	attrTotalLen := make([]byte, 2)
	binary.BigEndian.PutUint16(attrTotalLen, uint16(len(attr)))

	var payload []byte
	payload = append(payload, 0, 0)            // withdrawn routes length
	payload = append(payload, attrTotalLen...) // total path attribute length
	payload = append(payload, attr...)         // attributes
	return payload
}

// TestIngressFilterNoRedistribution verifies all routes pass when no redistribution is configured.
//
// VALIDATES: IngressFilter returns true when Global() is nil.
// PREVENTS: Redistribution blocking routes when not configured.
func TestIngressFilterNoRedistribution(t *testing.T) {
	// Ensure no global evaluator is set (default state).
	redistribute.SetGlobal(nil)

	src := registry.PeerFilterInfo{LocalAS: 65000, PeerAS: 65001}
	accept, modified := IngressFilter(src, buildIPv4Payload(), make(map[string]any))
	assert.True(t, accept)
	assert.Nil(t, modified)
}

// TestIngressFilterEBGPAccepted verifies ebgp routes pass when redistribution allows them.
//
// VALIDATES: IngressFilter accepts ebgp ipv4/unicast when import rule matches.
// PREVENTS: Valid routes rejected.
func TestIngressFilterEBGPAccepted(t *testing.T) {
	redistribute.SetGlobal(redistribute.NewEvaluator([]redistribute.ImportRule{
		{Source: "ebgp", Families: []string{"ipv4/unicast"}},
	}))
	defer redistribute.SetGlobal(nil)

	src := registry.PeerFilterInfo{LocalAS: 65000, PeerAS: 65001} // eBGP
	accept, _ := IngressFilter(src, buildIPv4Payload(), make(map[string]any))
	assert.True(t, accept)
}

// TestIngressFilterFamilyRejected verifies routes are rejected when family doesn't match.
//
// VALIDATES: IngressFilter rejects ipv6/unicast when only ipv4/unicast is allowed.
// PREVENTS: Wrong families leaking through.
func TestIngressFilterFamilyRejected(t *testing.T) {
	redistribute.SetGlobal(redistribute.NewEvaluator([]redistribute.ImportRule{
		{Source: "ebgp", Families: []string{"ipv4/unicast"}},
	}))
	defer redistribute.SetGlobal(nil)

	src := registry.PeerFilterInfo{LocalAS: 65000, PeerAS: 65001} // eBGP
	// IPv6 unicast (AFI=2, SAFI=1)
	accept, _ := IngressFilter(src, buildMPReachPayload(2, 1), make(map[string]any))
	assert.False(t, accept)
}

// TestIngressFilterIBGPvsEBGP verifies source detection from peer AS numbers.
//
// VALIDATES: IngressFilter correctly identifies ibgp (same AS) vs ebgp (different AS).
// PREVENTS: ibgp/ebgp source mismatch.
func TestIngressFilterIBGPvsEBGP(t *testing.T) {
	redistribute.SetGlobal(redistribute.NewEvaluator([]redistribute.ImportRule{
		{Source: "ibgp"},
	}))
	defer redistribute.SetGlobal(nil)

	// iBGP peer (same AS) should match "ibgp" rule
	ibgpSrc := registry.PeerFilterInfo{LocalAS: 65000, PeerAS: 65000}
	accept, _ := IngressFilter(ibgpSrc, buildIPv4Payload(), make(map[string]any))
	assert.True(t, accept)

	// eBGP peer (different AS) should not match "ibgp" rule
	ebgpSrc := registry.PeerFilterInfo{LocalAS: 65000, PeerAS: 65001}
	accept, _ = IngressFilter(ebgpSrc, buildIPv4Payload(), make(map[string]any))
	assert.False(t, accept)
}

// TestFamilyFromPayloadIPv4 verifies family extraction for IPv4 unicast UPDATEs.
//
// VALIDATES: familyFromPayload returns ipv4/unicast for standard UPDATEs.
// PREVENTS: IPv4 unicast misidentified.
func TestFamilyFromPayloadIPv4(t *testing.T) {
	assert.Equal(t, "ipv4/unicast", familyFromPayload(buildIPv4Payload()))
}

// TestFamilyFromPayloadMPReach verifies family extraction from MP_REACH_NLRI.
//
// VALIDATES: familyFromPayload returns correct family from MP_REACH_NLRI attribute.
// PREVENTS: Multi-protocol families misidentified.
func TestFamilyFromPayloadMPReach(t *testing.T) {
	tests := []struct {
		name string
		afi  uint16
		safi uint8
		want string
	}{
		{"ipv6/unicast", 2, 1, "ipv6/unicast"},
		{"ipv4/multicast", 1, 2, "ipv4/multicast"},
		{"ipv6/multicast", 2, 2, "ipv6/multicast"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, familyFromPayload(buildMPReachPayload(tt.afi, tt.safi)))
		})
	}
}

// TestFamilyFromPayloadMalformed verifies graceful handling of malformed payloads.
//
// VALIDATES: familyFromPayload defaults to ipv4/unicast on malformed input.
// PREVENTS: Panic or wrong family on garbage input.
func TestFamilyFromPayloadMalformed(t *testing.T) {
	assert.Equal(t, "ipv4/unicast", familyFromPayload(nil))
	assert.Equal(t, "ipv4/unicast", familyFromPayload([]byte{}))
	assert.Equal(t, "ipv4/unicast", familyFromPayload([]byte{0}))
	assert.Equal(t, "ipv4/unicast", familyFromPayload([]byte{0, 0, 0, 0}))
}
