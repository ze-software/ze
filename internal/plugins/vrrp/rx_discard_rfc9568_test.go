// Design: docs/architecture/vrrp/vrrp-first-hop-redundancy.md -- receive validation
// Related: rx_discard_test.go -- rxCheckItem and the mutations reused here
// Related: instance.go -- onPacket, the discard point
//
// VALIDATES: a VRRPv3 advertisement that arrives with a TTL or Hop Limit
// other than 255, a Type other than ADVERTISEMENT, or an address Count of 0
// is dropped before the state machine, and the same advertisement with that
// field valid reaches it.
// PREVENTS: a check that returns an error which onPacket records and then
// dispatches anyway.
package vrrp

import (
	"testing"

	"github.com/ze-software/ze/internal/plugins/vrrp/fsm"
	"github.com/ze-software/ze/internal/plugins/vrrp/packet"
)

// rxTTL returns a mutation that delivers the advertisement at the given TTL
// (IPv4) or Hop Limit (IPv6).
func rxTTL(ttl uint8) rxCheckMutation {
	return func(_ []byte, n int, meta *packet.RxMeta) int {
		meta.TTL = ttl
		return n
	}
}

// rxCountZero rewrites the advertisement as an exact-length packet whose
// IPvX Addr Count is 0: the fixed header alone, checksum recomputed.
func rxCountZero(buf []byte, _ int, meta *packet.RxMeta) int {
	buf[3] = 0
	packet.FillChecksum(buf, 0, packet.HeaderLen, meta.Src, meta.Dst)
	return packet.HeaderLen
}

// rxReachesFSM feeds item to a fresh non-owner instance for spec and reports
// the recorded rx errors and whether an AdvertReceived reached the state
// machine. It fails the test when the state moves without an event.
func rxReachesFSM(t *testing.T, spec GroupSpec, mutate rxCheckMutation) (rxErrors []string, reached bool) {
	t.Helper()
	spec.IsOwner = false
	in, f, _ := newTestInstance(t, spec)
	in.dispatch(fsm.Startup{Config: in.fsmConfig()})
	before := in.machine.State()
	in.onPacket(rxCheckItem(spec, mutate))
	select {
	case ev := <-in.events:
		_, reached = ev.(fsm.AdvertReceived)
	default:
		if after := in.machine.State(); after != before {
			t.Fatalf("state moved %v -> %v with no event", before, after)
		}
	}
	return f.snapshot().rxErrors, reached
}

// TestInstanceRxDiscardsBadTTLTypeAndCountZero proves the RFC 9568 discard
// rules for the TTL, the Hop Limit, the Type and the Count reach the state
// machine boundary, not only the decoder's return value.
//
// Method: for each rule, fresh instances receive a well-formed VRRPv3
// advertisement with the field valid (TTL/Hop Limit 255, Type 1, Count 1):
// nothing is recorded and an AdvertReceived reaches the FSM. The same
// advertisement with the field broken (TTL/Hop Limit 0, 64, 254; Type 0, 2,
// 15; Count 0) records exactly that reason and posts no FSM event.
//
// RFC requirement: RFC9568-5.1.1.3-2 positive -- a VRRPv3 IPv4 advert received with TTL 255 records nothing and reaches the state machine as AdvertReceived (onPacket instance.go).
// RFC requirement: RFC9568-5.1.1.3-2 negative -- the same IPv4 advert received with TTL 0, 64 or 254 records reason ttl and never reaches the state machine (onPacket instance.go, Decode packet/validate.go).
// RFC requirement: RFC9568-5.1.2.3-2 positive -- a VRRPv3 IPv6 advert received with Hop Limit 255 records nothing and reaches the state machine as AdvertReceived (onPacket instance.go).
// RFC requirement: RFC9568-5.1.2.3-2 negative -- the same IPv6 advert received with Hop Limit 0, 64 or 254 records reason ttl and never reaches the state machine (onPacket instance.go, Decode packet/validate.go).
// RFC requirement: RFC9568-5.2.2-1 positive -- a VRRPv3 packet of Type 1 records nothing and reaches the state machine as AdvertReceived (onPacket instance.go).
// RFC requirement: RFC9568-5.2.2-1 negative -- the same packet with Type 0, 2 or 15 and a valid checksum records reason type and never reaches the state machine (onPacket instance.go, Decode packet/validate.go).
// RFC requirement: RFC9568-5.2.5-1 positive -- a VRRPv3 advert with a non-zero Count records nothing and reaches the state machine as AdvertReceived (onPacket instance.go).
// RFC requirement: RFC9568-5.2.5-1 negative -- an exact-length VRRPv3 advert whose Count is 0 records reason count-zero and never reaches the state machine (onPacket instance.go, Decode packet/validate.go).
func TestInstanceRxDiscardsBadTTLTypeAndCountZero(t *testing.T) {
	v4, v6 := testSpec(), testSpecV6()
	type rxCase struct {
		name   string
		spec   GroupSpec
		mutate rxCheckMutation
		reason string
	}
	var cases []rxCase
	for _, ttl := range []uint8{0, 64, 254} {
		cases = append(cases,
			rxCase{"ipv4-ttl", v4, rxTTL(ttl), "ttl"},
			rxCase{"ipv6-hop-limit", v6, rxTTL(ttl), "ttl"})
	}
	for _, typ := range []byte{0, 2, 15} {
		cases = append(cases, rxCase{"type", v4, refilled(func(b []byte) { b[0] = packet.VersionV3<<4 | typ }), "type"})
	}
	cases = append(cases, rxCase{"count-zero", v4, rxCountZero, "count-zero"})

	for _, spec := range []GroupSpec{v4, v6} {
		if errs, reached := rxReachesFSM(t, spec, rxTTL(255)); len(errs) != 0 || !reached {
			t.Fatalf("valid advert (family %v): rx errors %v, reached FSM %v; want none, true", spec.Family, errs, reached)
		}
	}
	for _, tc := range cases {
		errs, reached := rxReachesFSM(t, tc.spec, tc.mutate)
		if len(errs) != 1 || errs[0] != tc.reason {
			t.Errorf("%s: rx errors = %v, want exactly [%s]", tc.name, errs, tc.reason)
		}
		if reached {
			t.Errorf("%s: a packet failing the %s check reached the FSM", tc.name, tc.reason)
		}
	}
}
