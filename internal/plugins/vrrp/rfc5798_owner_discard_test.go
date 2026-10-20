// Design: docs/architecture/vrrp/vrrp-first-hop-redundancy.md -- receive-path validation
// Related: instance.go -- onPacket, the owner discard of a checksum rfc5798-compatible group
// Related: rfc9568_owner_conflict_test.go -- the RFC 9568 owner that logs and processes
package vrrp

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/plugins/vrrp/fsm"
	"github.com/ze-software/ze/internal/plugins/vrrp/packet"
	"github.com/ze-software/ze/internal/plugins/vrrp/transport"
)

// v3AdvertItemRFC5798 encodes a VRRPv3 IPv4 advertisement for VRID 10 from
// source with the RFC 5798 pseudo-header checksum, the only form a group
// configured `checksum rfc5798-compatible` accepts.
func v3AdvertItemRFC5798(t *testing.T, priority uint8, source netip.Addr, vips ...netip.Addr) transport.RxItem {
	t.Helper()
	adv := packet.Advertisement{
		Version:         packet.VersionV3,
		Family:          packet.V4,
		VRID:            10,
		Priority:        priority,
		AdverIntervalMS: 1000,
		VIPs:            vips,
	}
	var buf [packet.MaxLenV3v4]byte
	n := adv.WriteTo(buf[:], 0)
	packet.FillChecksum(buf[:], 0, n, source, packet.MulticastV4, packet.ChecksumRFC5798Compatible)
	return transport.RxItem{
		Meta:    packet.RxMeta{TTL: 255, Family: packet.V4, Src: source, Dst: packet.MulticastV4},
		Payload: append([]byte(nil), buf[:n]...),
	}
}

// TestRFC5798OwnerDiscardsAdvertisement proves that the VRRPv3 address owner
// of a group configured `checksum rfc5798-compatible` discards an
// advertisement it receives for its VRID.
//
// Method: a v3 owner (Priority 255) in an rfc5798-compatible group receives a
// valid advertisement for VRID 10. It counts the owner reason and the FSM sees
// no AdvertReceived.
func TestRFC5798OwnerDiscardsAdvertisement(t *testing.T) {
	// RFC requirement: RFC5798-7.1-3 positive -- a VRRPv3 address owner (Priority 255) in a group configured checksum rfc5798-compatible discards a received advertisement for its VRID: it counts the owner reason and the FSM receives no AdvertReceived (onPacket instance.go).
	spec := testSpec()
	spec.Checksum = packet.ChecksumRFC5798Compatible
	spec.IsOwner = true
	spec.Priority = ownerPriority
	owner, deps, _ := newTestInstance(t, spec)
	owner.dispatch(fsm.Startup{Config: owner.fsmConfig()})

	owner.onPacket(v3AdvertItemRFC5798(t, 200, netip.MustParseAddr("192.0.2.200"), netip.MustParseAddr("192.0.2.1")))

	got := deps.snapshot()
	if len(got.rxErrors) != 1 || got.rxErrors[0] != packet.ReasonOwner {
		t.Fatalf("the rfc5798-compatible owner must count %q once, got %+v", packet.ReasonOwner, got.rxErrors)
	}
	if drainAdvertReceived(owner) {
		t.Fatal("the rfc5798-compatible owner processed the advertisement; RFC 5798 Section 7.1 makes it discard it")
	}
}

// TestRFC5798NonOwnerProcessesAdvertisement proves the discard is the owner
// check alone: a router in the same rfc5798-compatible group that is not the
// address owner processes the same advertisement.
//
// Method: a v3 non-owner (Priority 100) in an rfc5798-compatible group
// receives the advertisement the owner test discards. It counts nothing and the
// FSM sees AdvertReceived.
func TestRFC5798NonOwnerProcessesAdvertisement(t *testing.T) {
	// RFC requirement: RFC5798-7.1-3 negative -- a VRRPv3 router that is not the address owner, in a group configured checksum rfc5798-compatible, counts no receive error for the same advertisement and delivers it to the FSM (onPacket instance.go).
	spec := testSpec()
	spec.Checksum = packet.ChecksumRFC5798Compatible
	spec.IsOwner = false
	spec.Priority = 100
	backup, deps, _ := newTestInstance(t, spec)
	backup.dispatch(fsm.Startup{Config: backup.fsmConfig()})

	backup.onPacket(v3AdvertItemRFC5798(t, 200, netip.MustParseAddr("192.0.2.200"), netip.MustParseAddr("192.0.2.1")))

	if got := deps.snapshot(); len(got.rxErrors) != 0 {
		t.Fatalf("the rfc5798-compatible non-owner counted %+v, want nothing", got.rxErrors)
	}
	if !drainAdvertReceived(backup) {
		t.Fatal("the rfc5798-compatible non-owner did not deliver the advertisement to the FSM")
	}
}
