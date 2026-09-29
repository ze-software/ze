// Design: docs/architecture/vrrp/vrrp-first-hop-redundancy.md -- receive validation
// Related: instance.go -- onPacket, the discard point
// Related: packet/validate.go -- Decode, the receive checks
//
// VALIDATES: a received advertisement that fails any one of the Section 7.1
// receive checks is discarded before the state machine sees it, for VRRPv2 and
// VRRPv3 and for both address families, while the same advertisement with the
// check passing reaches the state machine.
// PREVENTS: an onPacket that records the failure and still dispatches the
// advertisement.
package vrrp

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/plugins/vrrp/fsm"
	"github.com/ze-software/ze/internal/plugins/vrrp/packet"
	"github.com/ze-software/ze/internal/plugins/vrrp/transport"
)

// rxCheckMutation breaks one receive check in an encoded advertisement. It
// receives the buffer, the encoded length, the receive metadata, and returns
// the length to deliver.
type rxCheckMutation func(buf []byte, n int, meta *packet.RxMeta) int

// rxCheckItem encodes a well-formed advertisement for spec's VRID and address
// list, at Priority 250 from a peer, then applies mutate when it is non-nil.
// Every call builds its own buffer, so no case sees another case's bytes.
func rxCheckItem(spec GroupSpec, mutate rxCheckMutation) transport.RxItem {
	family := packet.V4
	source := netip.MustParseAddr("192.0.2.9")
	target := packet.MulticastV4
	if spec.Family == familyIPv6 {
		family = packet.V6
		source = netip.MustParseAddr("fe80::9")
		target = packet.MulticastV6
	}
	version := packet.VersionV3
	if spec.Version == versionV2 {
		version = packet.VersionV2
	}
	adv := packet.Advertisement{
		Version:         version,
		Family:          family,
		VRID:            spec.VRID,
		Priority:        250,
		AdverIntervalMS: 1000,
		VIPs:            spec.VIPs,
	}
	buf := make([]byte, packet.MaxLenV3v6)
	n := adv.WriteTo(buf, 0)
	packet.FillChecksum(buf, 0, n, source, target)
	meta := packet.RxMeta{TTL: 255, Family: family, Src: source, Dst: target}
	if mutate != nil {
		n = mutate(buf, n, &meta)
	}
	return transport.RxItem{Meta: meta, Payload: buf[:n]}
}

// refilled applies edit to the encoded bytes and recomputes the checksum, so
// the only check the result fails is the one edit breaks.
func refilled(edit func(buf []byte)) rxCheckMutation {
	return func(buf []byte, n int, meta *packet.RxMeta) int {
		edit(buf)
		packet.FillChecksum(buf, 0, n, meta.Src, meta.Dst)
		return n
	}
}

func rxHopLimit254(_ []byte, n int, meta *packet.RxMeta) int {
	meta.TTL = 254
	return n
}

func rxFixedFieldsShort(_ []byte, _ int, _ *packet.RxMeta) int { return packet.HeaderLen - 1 }

// rxTailShort drops the last byte (an address byte for VRRPv3, an
// Authentication Data byte for VRRPv2) and recomputes the checksum over what
// remains, so only the completeness check can refuse the packet.
func rxTailShort(buf []byte, n int, meta *packet.RxMeta) int {
	packet.FillChecksum(buf, 0, n-1, meta.Src, meta.Dst)
	return n - 1
}

func rxChecksumWrong(buf []byte, n int, _ *packet.RxMeta) int {
	buf[6] ^= 0xff
	return n
}

// TestInstanceRxFailedCheckIsDiscarded proves every Section 7.1 receive check
// discards: the packet records its reason and never reaches the FSM.
//
// Method: for each check, a fresh instance receives a well-formed
// advertisement with that one check broken. It must record exactly that
// check's reason, post no FSM event, and keep its state. A second fresh
// instance, never the owner, receives the same advertisement unbroken and must
// record nothing and post an AdvertReceived: that control shows the only thing
// separating the two runs is the broken check.
//
// RFC requirement: RFC9568-7.1-6 negative -- a VRRPv3 packet failing any one of the mandatory checks, IPv4 TTL or IPv6 Hop Limit 255, version 3, type ADVERTISEMENT, complete fixed fields and addresses, checksum, VRID configured, and Max Advertise Interval non zero (erratum 8301), records that check's reason and never reaches the state machine (onPacket instance.go, Decode packet/validate.go)
// RFC requirement: RFC9568-7.1-6 positive -- the same VRRPv3 packet with every check passing records nothing and reaches the state machine as AdvertReceived (onPacket instance.go).
// RFC requirement: RFC5798-7.1-4 negative -- a VRRPv3 packet failing any one of the TTL or Hop Limit, version, complete packet, checksum, or VRID configured checks records that check's reason and never reaches the state machine; the owner half follows RFC 9568 erratum 8298 and is not claimed here (onPacket instance.go, Decode packet/validate.go)
// RFC requirement: RFC5798-7.1-4 positive -- the same VRRPv3 packet with every check passing records nothing and reaches the state machine as AdvertReceived (onPacket instance.go).
// RFC requirement: RFC3768-7.1-6 negative -- a VRRPv2 packet failing any one of the checks, TTL 255, version 2, complete fixed fields and addresses, checksum, VRID configured, local router not the address owner, and Auth Type 0, records that check's reason and never reaches the FSM (onPacket instance.go, Decode packet/validate.go)
// RFC requirement: RFC3768-7.1-6 positive -- the same VRRPv2 packet with every check passing, received by a router that is not the address owner, records nothing and reaches the FSM as AdvertReceived (onPacket instance.go).
func TestInstanceRxFailedCheckIsDiscarded(t *testing.T) {
	v3 := testSpec()
	v6 := testSpecV6()
	v2 := testSpec()
	v2.Version = versionV2

	cases := []struct {
		name   string
		spec   GroupSpec
		owner  bool
		mutate rxCheckMutation
		reason string
	}{
		{"v3-ipv4-ttl", v3, false, rxHopLimit254, "ttl"},
		{"v3-ipv6-hop-limit", v6, false, rxHopLimit254, "ttl"},
		{"v3-version", v3, false, refilled(func(b []byte) { b[0] = packet.VersionV2<<4 | 1 }), "version"},
		{"v3-type", v3, false, refilled(func(b []byte) { b[0] = packet.VersionV3<<4 | 2 }), "type"},
		{"v3-fixed-fields-short", v3, false, rxFixedFieldsShort, "truncated"},
		{"v3-address-short", v3, false, rxTailShort, "length"},
		{"v3-checksum", v3, false, rxChecksumWrong, "checksum"},
		{"v3-vrid", v3, false, refilled(func(b []byte) { b[1] = 11 }), "vrid"},
		{"v3-interval-zero", v3, false, refilled(func(b []byte) { b[4], b[5] = 0, 0 }), "interval-zero"},
		{"v2-ttl", v2, false, rxHopLimit254, "ttl"},
		{"v2-version", v2, false, refilled(func(b []byte) { b[0] = packet.VersionV3<<4 | 1 }), "version"},
		{"v2-fixed-fields-short", v2, false, rxFixedFieldsShort, "truncated"},
		{"v2-auth-data-short", v2, false, rxTailShort, "length"},
		{"v2-checksum", v2, false, rxChecksumWrong, "checksum"},
		{"v2-vrid", v2, false, refilled(func(b []byte) { b[1] = 11 }), "vrid"},
		{"v2-auth-type", v2, false, refilled(func(b []byte) { b[4] = 1 }), "auth-type"},
		{"v2-owner", v2, true, nil, packet.ReasonOwner},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spec := tc.spec
			spec.IsOwner = tc.owner
			in, f, _ := newTestInstance(t, spec)
			in.dispatch(fsm.Startup{Config: in.fsmConfig()})
			before := in.machine.State()

			in.onPacket(rxCheckItem(spec, tc.mutate))
			if got := f.snapshot().rxErrors; len(got) != 1 || got[0] != tc.reason {
				t.Fatalf("rx errors = %v, want exactly [%s]", got, tc.reason)
			}
			select {
			case ev := <-in.events:
				t.Fatalf("a packet failing the %s check reached the FSM as %T", tc.reason, ev)
			default:
			}
			if after := in.machine.State(); after != before {
				t.Fatalf("state moved %v -> %v on a discarded packet", before, after)
			}

			control := tc.spec
			control.IsOwner = false
			inC, fC, _ := newTestInstance(t, control)
			inC.dispatch(fsm.Startup{Config: inC.fsmConfig()})
			inC.onPacket(rxCheckItem(control, nil))
			if got := fC.snapshot().rxErrors; len(got) != 0 {
				t.Fatalf("control: the unbroken packet recorded %v, want nothing", got)
			}
			select {
			case ev := <-inC.events:
				if _, ok := ev.(fsm.AdvertReceived); !ok {
					t.Fatalf("control: event = %T, want fsm.AdvertReceived", ev)
				}
			default:
				t.Fatal("control: the unbroken packet produced no FSM event")
			}
		})
	}
}
