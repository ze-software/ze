//go:build integration && linux

// Design: docs/architecture/testing/interop.md -- SRv6 evidence predicates.
// Related: vpp_srv6_probe_integration_linux_test.go -- real VPP dump consumer.
// Related: vpp_srv6_wire_integration_linux_test.go -- real packet consumer.
// These synthetic inputs test the oracle, never stand in for dataplane evidence.
package testdeployment

import (
	"encoding/binary"
	"net/netip"
	"testing"

	"go.fd.io/govpp/binapi/ip_types"
	"go.fd.io/govpp/binapi/sr"
	"go.fd.io/govpp/binapi/sr_types"
)

// TestVPPSRv6StateOracle proves that policy existence alone cannot certify a
// route: table, encapsulation, ownership, segment and steering must all agree.
func TestVPPSRv6StateOracle(t *testing.T) {
	first := netip.MustParsePrefix("10.94.1.0/24")
	second := netip.MustParsePrefix("10.94.2.0/24")
	sid := netip.MustParseAddr(vppSRv6SIDOne)
	bsid := netip.MustParseAddr("fd00:94::1")
	want := map[netip.Prefix]netip.Addr{first: sid, second: sid}
	for _, test := range []struct {
		name   string
		mutate func(*sr.SrPoliciesDetails, []*sr.SrSteeringPolDetails)
	}{
		{name: "insertion-not-encapsulation", mutate: func(policy *sr.SrPoliciesDetails, _ []*sr.SrSteeringPolDetails) { policy.IsEncap = false }},
		{name: "wrong-policy-table", mutate: func(policy *sr.SrPoliciesDetails, _ []*sr.SrSteeringPolDetails) { policy.FibTable = 9 }},
		{name: "missing-segment-list", mutate: func(policy *sr.SrPoliciesDetails, _ []*sr.SrSteeringPolDetails) { policy.SidLists = nil }},
		{name: "wrong-segment-count", mutate: func(policy *sr.SrPoliciesDetails, _ []*sr.SrSteeringPolDetails) { policy.SidLists[0].NumSids = 2 }},
		{name: "wrong-remote-sid", mutate: func(policy *sr.SrPoliciesDetails, _ []*sr.SrSteeringPolDetails) {
			policy.SidLists[0].Sids[0] = netip.MustParseAddr(vppSRv6SIDTwo).As16()
		}},
		{name: "remote-sid-used-as-bsid", mutate: func(policy *sr.SrPoliciesDetails, entries []*sr.SrSteeringPolDetails) {
			policy.Bsid = sid.As16()
			for _, entry := range entries {
				entry.Bsid = policy.Bsid
			}
		}},
		{name: "wrong-steering-table", mutate: func(_ *sr.SrPoliciesDetails, entries []*sr.SrSteeringPolDetails) { entries[0].FibTable = 9 }},
		{name: "wrong-traffic-type", mutate: func(_ *sr.SrPoliciesDetails, entries []*sr.SrSteeringPolDetails) {
			entries[0].TrafficType = sr_types.SR_STEER_API_IPV6
		}},
		{name: "orphan-steering", mutate: func(_ *sr.SrPoliciesDetails, entries []*sr.SrSteeringPolDetails) { entries[0].Bsid = sid.As16() }},
		{name: "duplicate-prefix", mutate: func(_ *sr.SrPoliciesDetails, entries []*sr.SrSteeringPolDetails) {
			entries[0].Prefix = entries[1].Prefix
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			policy := &sr.SrPoliciesDetails{Bsid: bsid.As16(), IsEncap: true, NumSidLists: 1,
				SidLists: []sr.Srv6SidList{{NumSids: 1, Sids: [16]ip_types.IP6Address{sid.As16()}}}}
			entries := make([]*sr.SrSteeringPolDetails, 0, 2)
			for _, prefix := range []netip.Prefix{first, second} {
				wirePrefix, err := ip_types.ParsePrefix(prefix.String())
				if err != nil {
					t.Fatal(err)
				}
				entries = append(entries, &sr.SrSteeringPolDetails{TrafficType: sr_types.SR_STEER_API_IPV4,
					Prefix: wirePrefix, Bsid: bsid.As16()})
			}
			policies := []*sr.SrPoliciesDetails{policy}
			if err := vppSRv6StateMatches(policies, entries, want); err != nil {
				t.Fatal("positive control: ", err)
			}
			if err := vppSRv6StateMatches(nil, entries, want); err == nil {
				t.Fatal("steering without a policy passed")
			}
			if err := vppSRv6StateMatches(policies, entries[:1], want); err == nil {
				t.Fatal("missing shared prefix passed")
			}
			if err := vppSRv6StateMatches(policies, entries, nil); err == nil {
				t.Fatal("stale state after withdrawal passed")
			}
			test.mutate(policy, entries)
			if err := vppSRv6StateMatches(policies, entries, want); err == nil {
				t.Fatal("corrupt installed state passed")
			}
		})
	}
	if err := vppSRv6StateMatches(nil, nil, nil); err != nil {
		t.Fatal("empty withdrawal state: ", err)
	}
	t.Run("same-prefix-distinct-tenant-tables", func(t *testing.T) {
		wirePrefix, err := ip_types.ParsePrefix(first.String())
		if err != nil {
			t.Fatal(err)
		}
		policies := []*sr.SrPoliciesDetails{{Bsid: bsid.As16(), IsEncap: true, NumSidLists: 1,
			SidLists: []sr.Srv6SidList{{NumSids: 1, Sids: [16]ip_types.IP6Address{sid.As16()}}}}}
		entries := []*sr.SrSteeringPolDetails{
			{TrafficType: sr_types.SR_STEER_API_IPV4, Prefix: wirePrefix, Bsid: bsid.As16(), FibTable: 10},
			{TrafficType: sr_types.SR_STEER_API_IPV4, Prefix: wirePrefix, Bsid: bsid.As16(), FibTable: 20},
		}
		want := map[vppSRv6RouteKey]netip.Addr{
			{prefix: first, table: 10}: sid,
			{prefix: first, table: 20}: sid,
		}
		if err := vppSRv6TablesMatch(policies, entries, want); err != nil {
			t.Fatal("tenant-table positive control: ", err)
		}
		entries[1].FibTable = 10
		if err := vppSRv6TablesMatch(policies, entries, want); err == nil {
			t.Fatal("collapsing identical prefixes into one tenant table passed")
		}
		entries[1].FibTable = 20
		policies[0].FibTable = 10
		if err := vppSRv6TablesMatch(policies, entries, want); err == nil {
			t.Fatal("moving the underlay lookup into a tenant table passed")
		}
	})
}

// TestVPPSRv6PacketOracle checks both polarities of the independent packet
// predicate. Synthetic frames here are not claimed as an interoperability run.
func TestVPPSRv6PacketOracle(t *testing.T) {
	var inner [60]byte
	inner[0], inner[8], inner[9] = 0x45, 64, 17
	binary.BigEndian.PutUint16(inner[2:4], uint16(len(inner)))
	copy(inner[12:20], []byte{192, 0, 2, 2, 10, 94, 1, 1})
	copy(inner[28:], "independent-payload")
	binary.BigEndian.PutUint16(inner[10:12], vppSRv6Checksum(inner[:20]))
	sid := netip.MustParseAddr(vppSRv6SIDOne)
	for _, test := range []struct {
		name   string
		mutate func([]byte) []byte
	}{
		{name: "short", mutate: func(frame []byte) []byte { return frame[:53] }},
		{name: "plain-ipv4", mutate: func(frame []byte) []byte { frame[12] = 8; return frame }},
		{name: "wrong-version", mutate: func(frame []byte) []byte { frame[14] = 0x40; return frame }},
		{name: "unexpected-srh", mutate: func(frame []byte) []byte { frame[20] = 43; return frame }},
		{name: "wrong-payload-length", mutate: func(frame []byte) []byte { frame[19]++; return frame }},
		{name: "wrong-outer-source", mutate: func(frame []byte) []byte { frame[22]++; return frame }},
		{name: "wrong-remote-sid", mutate: func(frame []byte) []byte { frame[53]++; return frame }},
		{name: "corrupt-inner-header", mutate: func(frame []byte) []byte { frame[54]++; return frame }},
		{name: "wrong-hop-count", mutate: func(frame []byte) []byte { frame[62]--; return frame }},
		{name: "wrong-inner-protocol", mutate: func(frame []byte) []byte { frame[63]++; return frame }},
		{name: "corrupt-checksum", mutate: func(frame []byte) []byte { frame[64]++; return frame }},
		{name: "wrong-inner-payload", mutate: func(frame []byte) []byte { frame[100]++; return frame }},
	} {
		t.Run(test.name, func(t *testing.T) {
			var frame [114]byte
			binary.BigEndian.PutUint16(frame[12:14], 0x86dd)
			frame[14], frame[20] = 0x60, 4
			binary.BigEndian.PutUint16(frame[18:20], uint16(len(inner)))
			source := netip.MustParseAddr(vppSRv6Source).As16()
			copy(frame[22:38], source[:])
			segment := sid.As16()
			copy(frame[38:54], segment[:])
			copy(frame[54:], inner[:])
			frame[62]--
			frame[64], frame[65] = 0, 0
			binary.BigEndian.PutUint16(frame[64:66], vppSRv6Checksum(frame[54:74]))
			if err := vppSRv6PacketMatches(frame[:], inner[:], sid); err != nil {
				t.Fatal("positive control: ", err)
			}
			if err := vppSRv6PacketMatches(test.mutate(frame[:]), inner[:], sid); err == nil {
				t.Fatal("corrupt packet passed")
			}
		})
	}
}
