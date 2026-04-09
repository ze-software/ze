package filter

import (
	"encoding/binary"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
)

// VALIDATES: RFC 4271 Section 9 (AS loop), RFC 4456 Section 8 (originator-ID and cluster-list loops).
// PREVENTS: Routes looping through the local AS or reflected back to originators.

func ibgpPeer() registry.PeerFilterInfo {
	return registry.PeerFilterInfo{
		Address:  netip.MustParseAddr("192.0.2.1"),
		PeerAS:   65001,
		LocalAS:  65001,
		RouterID: 0x01020301,
	}
}

func ebgpPeer() registry.PeerFilterInfo {
	return registry.PeerFilterInfo{
		Address:  netip.MustParseAddr("192.0.2.1"),
		PeerAS:   65002,
		LocalAS:  65001,
		RouterID: 0x01020301,
	}
}

func makeUpdateBody(pathAttrs []byte) []byte {
	aLen := len(pathAttrs)
	body := make([]byte, 2+2+aLen)
	// withdrawn_len = 0
	binary.BigEndian.PutUint16(body[2:4], uint16(aLen))
	copy(body[4:], pathAttrs)
	return body
}

func buildASPathAttr(asns []uint32, asn4 bool) []byte {
	asnSize := 2
	if asn4 {
		asnSize = 4
	}
	segLen := 2 + len(asns)*asnSize
	attr := make([]byte, 3+segLen)
	attr[0] = 0x40 // Transitive
	attr[1] = 0x02 // AS_PATH
	attr[2] = byte(segLen)
	attr[3] = 2 // AS_SEQUENCE
	attr[4] = byte(len(asns))
	off := 5
	for _, asn := range asns {
		if asn4 {
			binary.BigEndian.PutUint32(attr[off:], asn)
			off += 4
		} else {
			binary.BigEndian.PutUint16(attr[off:], uint16(asn))
			off += 2
		}
	}
	return attr
}

func buildASSetAttr(asns []uint32) []byte {
	segLen := 2 + len(asns)*2
	attr := make([]byte, 3+segLen)
	attr[0] = 0x40 // Transitive
	attr[1] = 0x02 // AS_PATH
	attr[2] = byte(segLen)
	attr[3] = 1 // AS_SET
	attr[4] = byte(len(asns))
	off := 5
	for _, asn := range asns {
		binary.BigEndian.PutUint16(attr[off:], uint16(asn))
		off += 2
	}
	return attr
}

func buildOriginatorIDAttr(id uint32) []byte {
	attr := make([]byte, 7)
	attr[0] = 0x80 // Optional
	attr[1] = 0x09 // ORIGINATOR_ID
	attr[2] = 4
	binary.BigEndian.PutUint32(attr[3:], id)
	return attr
}

func buildClusterListAttr(ids []uint32) []byte {
	dataLen := len(ids) * 4
	attr := make([]byte, 3+dataLen)
	attr[0] = 0x80 // Optional
	attr[1] = 0x0A // CLUSTER_LIST
	attr[2] = byte(dataLen)
	for i, id := range ids {
		binary.BigEndian.PutUint32(attr[3+i*4:], id)
	}
	return attr
}

func accept(src registry.PeerFilterInfo, body []byte) bool {
	ok, _ := LoopIngress(src, body, nil)
	return ok
}

// --- AS Loop Detection ---

// TestDetectASLoop verifies local ASN in AS_SEQUENCE is detected.
// VALIDATES: AC-1, AC-2 -- route with local ASN in AS_PATH treated as withdrawn.
func TestDetectASLoop(t *testing.T) {
	body := makeUpdateBody(buildASPathAttr([]uint32{65002, 65001, 65003}, false))
	assert.False(t, accept(ibgpPeer(), body), "should detect local ASN 65001 in AS_PATH")
}

// TestDetectASLoop_ASSet verifies local ASN in AS_SET is detected.
// VALIDATES: AC-10 -- AS_SET members count for loop detection.
func TestDetectASLoop_ASSet(t *testing.T) {
	body := makeUpdateBody(buildASSetAttr([]uint32{65002, 65001}))
	assert.False(t, accept(ibgpPeer(), body), "should detect local ASN 65001 in AS_SET")
}

// TestDetectASLoop_NotPresent verifies no false positive when local ASN absent.
// VALIDATES: AC-3 -- route without local ASN accepted normally.
func TestDetectASLoop_NotPresent(t *testing.T) {
	body := makeUpdateBody(buildASPathAttr([]uint32{65002, 65003, 65004}, false))
	assert.True(t, accept(ibgpPeer(), body), "should not detect loop when local ASN absent")
}

// TestDetectASLoop_EmptyPath verifies empty AS_PATH does not trigger.
func TestDetectASLoop_EmptyPath(t *testing.T) {
	body := makeUpdateBody([]byte{0x40, 0x02, 0x00})
	assert.True(t, accept(ibgpPeer(), body), "empty AS_PATH should not trigger loop")
}

// --- Originator-ID Loop Detection ---

// TestDetectOriginatorIDLoop verifies matching ORIGINATOR_ID is detected.
// VALIDATES: AC-4 -- iBGP UPDATE with ORIGINATOR_ID matching local Router ID.
func TestDetectOriginatorIDLoop(t *testing.T) {
	body := makeUpdateBody(buildOriginatorIDAttr(0x01020301))
	assert.False(t, accept(ibgpPeer(), body), "should detect ORIGINATOR_ID matching Router ID")
}

// TestDetectOriginatorIDLoop_Different verifies non-matching ORIGINATOR_ID passes.
// VALIDATES: AC-5 -- different ORIGINATOR_ID accepted normally.
func TestDetectOriginatorIDLoop_Different(t *testing.T) {
	body := makeUpdateBody(buildOriginatorIDAttr(0x0A000001))
	assert.True(t, accept(ibgpPeer(), body), "different ORIGINATOR_ID should pass")
}

// TestDetectOriginatorIDLoop_Absent verifies missing ORIGINATOR_ID passes.
func TestDetectOriginatorIDLoop_Absent(t *testing.T) {
	body := makeUpdateBody(buildASPathAttr([]uint32{65002}, true))
	assert.True(t, accept(ibgpPeer(), body), "missing ORIGINATOR_ID should pass")
}

// TestDetectOriginatorIDLoop_eBGP verifies eBGP session skips originator-ID check.
// VALIDATES: AC-6 -- eBGP with ORIGINATOR_ID accepted.
func TestDetectOriginatorIDLoop_eBGP(t *testing.T) {
	body := makeUpdateBody(buildOriginatorIDAttr(0x01020301))
	assert.True(t, accept(ebgpPeer(), body), "eBGP should skip ORIGINATOR_ID check")
}

// --- Cluster-List Loop Detection ---

// TestDetectClusterListLoop verifies local Router ID in CLUSTER_LIST is detected.
// VALIDATES: AC-7 -- iBGP UPDATE with local Router ID in CLUSTER_LIST.
func TestDetectClusterListLoop(t *testing.T) {
	body := makeUpdateBody(buildClusterListAttr([]uint32{0x0A000001, 0x01020301, 0x0A000002}))
	assert.False(t, accept(ibgpPeer(), body), "should detect local Router ID in CLUSTER_LIST")
}

// TestDetectClusterListLoop_NotPresent verifies CLUSTER_LIST without local ID passes.
// VALIDATES: AC-8 -- CLUSTER_LIST not containing local Router ID accepted.
func TestDetectClusterListLoop_NotPresent(t *testing.T) {
	body := makeUpdateBody(buildClusterListAttr([]uint32{0x0A000001, 0x0A000002}))
	assert.True(t, accept(ibgpPeer(), body), "CLUSTER_LIST without local ID should pass")
}

// TestDetectClusterListLoop_Absent verifies missing CLUSTER_LIST passes.
func TestDetectClusterListLoop_Absent(t *testing.T) {
	body := makeUpdateBody(buildASPathAttr([]uint32{65002}, true))
	assert.True(t, accept(ibgpPeer(), body), "missing CLUSTER_LIST should pass")
}

// TestDetectClusterListLoop_eBGP verifies eBGP session skips cluster-list check.
// VALIDATES: AC-9 -- eBGP with CLUSTER_LIST accepted.
func TestDetectClusterListLoop_eBGP(t *testing.T) {
	body := makeUpdateBody(buildClusterListAttr([]uint32{0x01020301}))
	assert.True(t, accept(ebgpPeer(), body), "eBGP should skip CLUSTER_LIST check")
}

// --- allow-own-as Tests ---

// TestLoopIngressAllowOwnAS1 verifies allow-own-as=1 accepts one occurrence.
// VALIDATES: AC-7 -- allow-own-as N permits up to N occurrences of local ASN.
func TestLoopIngressAllowOwnAS1(t *testing.T) {
	// AS_PATH: 65002, 65001, 65003 -- local ASN 65001 appears once.
	body := makeUpdateBody(buildASPathAttr([]uint32{65002, 65001, 65003}, false))
	src := ebgpPeer()
	src.LocalAS = 65001
	src.AllowOwnAS = 1
	assert.True(t, accept(src, body), "allow-own-as=1 should accept 1 occurrence of local ASN")
}

// TestLoopIngressAllowOwnAS0Rejects verifies default allow-own-as=0 rejects first occurrence.
// VALIDATES: AC-6 -- allow-own-as 0 (default) rejects on first local ASN.
func TestLoopIngressAllowOwnAS0Rejects(t *testing.T) {
	// AS_PATH: 65002, 65001, 65003 -- local ASN 65001 appears once.
	body := makeUpdateBody(buildASPathAttr([]uint32{65002, 65001, 65003}, false))
	src := ebgpPeer()
	src.LocalAS = 65001
	src.AllowOwnAS = 0
	assert.False(t, accept(src, body), "allow-own-as=0 should reject on first occurrence of local ASN")
}

// TestLoopIngressAllowOwnASExceeded verifies rejection when count exceeds allow-own-as.
// VALIDATES: AC-7 -- allow-own-as 1 rejects when local ASN appears twice.
func TestLoopIngressAllowOwnASExceeded(t *testing.T) {
	// AS_PATH: 65001, 65002, 65001 -- local ASN 65001 appears twice.
	body := makeUpdateBody(buildASPathAttr([]uint32{65001, 65002, 65001}, false))
	src := ebgpPeer()
	src.LocalAS = 65001
	src.AllowOwnAS = 1
	assert.False(t, accept(src, body), "allow-own-as=1 should reject when local ASN appears twice")
}

// TestLoopIngressAllowOwnASExactBoundary verifies allow-own-as=2 accepts exactly 2 occurrences.
// VALIDATES: AC-7 -- boundary test: count == allow-own-as is accepted.
func TestLoopIngressAllowOwnASExactBoundary(t *testing.T) {
	// AS_PATH: 65001, 65002, 65001 -- local ASN 65001 appears twice.
	body := makeUpdateBody(buildASPathAttr([]uint32{65001, 65002, 65001}, false))
	src := ebgpPeer()
	src.LocalAS = 65001
	src.AllowOwnAS = 2
	assert.True(t, accept(src, body), "allow-own-as=2 should accept exactly 2 occurrences")
}

// --- cluster-id override Tests ---

// TestLoopIngressClusterIDOverride verifies explicit cluster-id is used for CLUSTER_LIST check.
// VALIDATES: AC-9 -- cluster-id override replaces RouterID for CLUSTER_LIST loop detection.
func TestLoopIngressClusterIDOverride(t *testing.T) {
	// CLUSTER_LIST contains 0x0A000001 (10.0.0.1). RouterID is 0x01020301.
	// With ClusterID set to 0x0A000001, the check should match and reject.
	body := makeUpdateBody(buildClusterListAttr([]uint32{0x0A000001}))
	src := ibgpPeer()
	src.ClusterID = 0x0A000001
	assert.False(t, accept(src, body), "explicit cluster-id should be used for CLUSTER_LIST check")
}

// TestLoopIngressClusterIDZeroUsesRouterID verifies ClusterID=0 falls back to RouterID.
// VALIDATES: AC-8 -- no cluster-id configured uses router-id (existing behavior).
func TestLoopIngressClusterIDZeroUsesRouterID(t *testing.T) {
	// CLUSTER_LIST contains RouterID (0x01020301). ClusterID is 0 (not set).
	body := makeUpdateBody(buildClusterListAttr([]uint32{0x01020301}))
	src := ibgpPeer()
	src.ClusterID = 0
	assert.False(t, accept(src, body), "ClusterID=0 should use RouterID for CLUSTER_LIST check")
}

// TestLoopIngressClusterIDDoesNotAffectOriginatorID verifies ORIGINATOR_ID still uses RouterID.
// VALIDATES: ORIGINATOR_ID check uses RouterID per RFC 4456, not ClusterID.
func TestLoopIngressClusterIDDoesNotAffectOriginatorID(t *testing.T) {
	// ORIGINATOR_ID matches RouterID (0x01020301). ClusterID is different.
	body := makeUpdateBody(buildOriginatorIDAttr(0x01020301))
	src := ibgpPeer()
	src.ClusterID = 0x0A000001 // Different from RouterID
	// Should still reject because ORIGINATOR_ID == RouterID.
	assert.False(t, accept(src, body), "ORIGINATOR_ID check should use RouterID, not ClusterID")
}

// TestLoopIngressClusterIDMismatchAccepts verifies non-matching cluster-id passes.
// VALIDATES: AC-9 -- CLUSTER_LIST not containing explicit cluster-id is accepted.
func TestLoopIngressClusterIDMismatchAccepts(t *testing.T) {
	// CLUSTER_LIST contains 0x0A000002. ClusterID is 0x0A000001. RouterID is 0x01020301.
	// Neither ClusterID nor RouterID matches, so it should accept.
	body := makeUpdateBody(buildClusterListAttr([]uint32{0x0A000002}))
	src := ibgpPeer()
	src.ClusterID = 0x0A000001
	assert.True(t, accept(src, body), "CLUSTER_LIST without matching cluster-id should pass")
}
