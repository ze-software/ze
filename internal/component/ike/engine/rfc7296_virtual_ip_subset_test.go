// Design: docs/architecture/ike/ipsec-14-responder.md -- a pool-bound peer's selector is narrowed to its lease only inside the proposal
// RFC: rfc/short/rfc7296.md -- RFC7296-2.9-1

package engine

import (
	"errors"
	"net"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/wire"
)

// vipTS builds one any-port, any-protocol selector of the inclusive range start-end.
func vipTS(start, end []byte) wire.TrafficSelector {
	tsType := wire.TSTypeIPv4AddrRange
	if len(start) == net.IPv6len {
		tsType = wire.TSTypeIPv6AddrRange
	}
	return wire.TrafficSelector{TSType: tsType, EndPort: 65535, StartAddress: start, EndAddress: end}
}

// vipPayloads wraps one TSi and one TSr selector in their payloads.
func vipPayloads(tsi, tsr wire.TrafficSelector) (*wire.PayloadTS, *wire.PayloadTS) {
	return &wire.PayloadTS{TSPayloadType: wire.PayloadTypeTSi, TrafficSelectors: []wire.TrafficSelector{tsi}},
		&wire.PayloadTS{TSPayloadType: wire.PayloadTypeTSr, TrafficSelectors: []wire.TrafficSelector{tsr}}
}

// leasedResponder returns a pool-bound responder SA that holds an IPv4 lease after
// IKE_AUTH, and that lease.
func leasedResponder(t *testing.T) (*SA, netip.Addr) {
	t.Helper()
	ini, resp, ps := cpResponder(t, cpVIP(t, cpPool()), "ze", true)
	cpAuth(t, ini, resp, ps, cfgRequest(ip4Ask(netip.Addr{})))
	leases := resp.VirtualIPs()
	if len(leases) != 1 {
		t.Fatalf("SA holds %v, want one IPv4 lease", leases)
	}
	return resp, leases[0]
}

// TestRFC7296VirtualIPAnsweredOnlyInsideTheProposal drives narrowChildSelectors, the
// entry a fresh CREATE_CHILD_SA and both Child rekey paths share, for a pool-bound peer
// whose IKE SA holds a lease from 10.10.0.0/29.
//
// VALIDATES: a TSi proposal that contains the lease (0.0.0.0/0, the pool's /29) is
// answered with the lease host, fresh and on a rekey over the scope in use; a TSi
// proposal that does not contain it (192.0.2.50/32, or an IPv4-mapped IPv6 range holding
// the IPv4 lease's mapped form) is refused with TS_UNACCEPTABLE and records no answer.
// PREVENTS: the lease replacing a proposed selector that does not contain it, which
// answered 10.10.0.1/32 to a proposal of 192.0.2.50/32 (traffic the peer never proposed)
// and, on a rekey, let the floor check judge the substituted list and skip the refusal.
func TestRFC7296VirtualIPAnsweredOnlyInsideTheProposal(t *testing.T) {
	// RFC requirement: RFC7296-2.9-1 positive -- a pool-bound peer whose lease is 10.10.0.x and whose TSi proposal is 0.0.0.0/0 or the pool's 10.10.0.0/29, with TSr 0.0.0.0/0, is answered TSi exactly the lease /32 and TSr 0.0.0.0/0, on a fresh Child SA and on a rekey whose scope in use is that pair.
	// RFC requirement: RFC7296-2.9-1 negative -- the same peer whose TSi proposal is 192.0.2.50/32, or the IPv4-mapped IPv6 range ::ffff:10.10.0.0-::ffff:10.10.0.255, neither of which contains its IPv4 lease, with TSr 0.0.0.0/0 and ::/0 respectively, draws TS_UNACCEPTABLE and records no answered pair, on a fresh Child SA and on a rekey whose scope in use is the lease /32 <-> 0.0.0.0/0.
	any4 := vipTS([]byte{0, 0, 0, 0}, []byte{255, 255, 255, 255})

	resp, lease := leasedResponder(t)
	host := net.IPNet{IP: net.IP(lease.AsSlice()), Mask: net.CIDRMask(32, 32)}

	inside := map[string]wire.TrafficSelector{
		"any":  any4,
		"pool": vipTS([]byte{10, 10, 0, 0}, []byte{10, 10, 0, 7}),
	}
	var scope []tsPair
	for name, sel := range inside {
		tsi, tsr := vipPayloads(sel, any4)
		resp.NegotiatedPairs = nil
		if err := narrowChildSelectors(resp, tsi, tsr, nil); err != nil {
			t.Fatalf("%s: a fresh proposal containing the lease was refused: %v", name, err)
		}
		if len(resp.NegotiatedPairs) != 1 {
			t.Fatalf("%s: answered %d pairs, want one", name, len(resp.NegotiatedPairs))
		}
		got := resp.NegotiatedPairs[0]
		if got.I.Net.String() != host.String() || got.R.Net.String() != "0.0.0.0/0" {
			t.Fatalf("%s: answered %v <-> %v, want the lease %v <-> 0.0.0.0/0", name, got.I.Net, got.R.Net, &host)
		}
		scope = resp.NegotiatedPairs
	}

	tsi, tsr := vipPayloads(any4, any4)
	resp.NegotiatedPairs = nil
	if err := narrowChildSelectors(resp, tsi, tsr, scope); err != nil {
		t.Fatalf("a rekey proposal of 0.0.0.0/0 over the scope in use %s was refused: %v", pairsText(scope), err)
	}
	if len(resp.NegotiatedPairs) != 1 || resp.NegotiatedPairs[0].I.Net.String() != host.String() {
		t.Fatalf("the rekey answered %s, want the scope in use %s", pairsText(resp.NegotiatedPairs), pairsText(scope))
	}

	mapped := func(last byte) []byte {
		return []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0xff, 0xff, 10, 10, 0, last}
	}
	// The mapped range is an IPv6 selector, paired with TSr ::/0. Classified by its
	// address it would read as IPv4 and be given the IPv4 lease under its own IPv6 mask,
	// which the IPv6 TSr would then accept.
	any6 := vipTS(make([]byte, net.IPv6len), []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff})
	outside := []struct {
		name     string
		tsi, tsr wire.TrafficSelector
	}{
		{"other host", vipTS([]byte{192, 0, 2, 50}, []byte{192, 0, 2, 50}), any4},
		{"ipv4-mapped", vipTS(mapped(0), mapped(255)), any6},
	}
	for _, tc := range outside {
		for _, floor := range [][]tsPair{nil, scope} {
			tsi, tsr := vipPayloads(tc.tsi, tc.tsr)
			resp.NegotiatedPairs = nil
			err := narrowChildSelectors(resp, tsi, tsr, floor)
			if !errors.Is(err, errTSUnacceptable) {
				t.Errorf("%s, floor %s: err=%v, want TS_UNACCEPTABLE", tc.name, pairsText(floor), err)
			}
			if len(resp.NegotiatedPairs) != 0 {
				t.Errorf("%s, floor %s: recorded %s for a proposal that does not contain the lease %v",
					tc.name, pairsText(floor), pairsText(resp.NegotiatedPairs), lease)
			}
		}
	}
}
