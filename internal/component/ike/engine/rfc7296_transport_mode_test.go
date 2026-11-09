package engine

import (
	"net"
	"testing"
)

// VALIDATES: AC-17, RFC 7296 Section 2.23.1: a transport-mode peer whose remote-address
// is a DNS name proposes exactly the host pair the connect cycle resolved. TSi is the
// one address the IKE SA sends from and TSr the one address it dialed, and no second
// DNS query is made to build the proposal.
//
// RED before the fix: observedRemoteAddress fell back to net.ParseIP(remote-address),
// nil for a name, so transportSelectorPairs answered nil and the proposal fell back to
// the wildcard, a range in each payload, which the MUST forbids for transport mode.
//
// RFC requirement: RFC7296-2.23.1-2 positive -- a transport-mode proposal toward a peer
// named by DNS carries one TSi entry of exactly one address, the IKE SA's source.
// RFC requirement: RFC7296-2.23.1-3 positive -- the same proposal carries one TSr entry
// of exactly one address, the address the remote-address name resolved to.
func TestTransportModeDNSRemoteProposesResolvedPair(t *testing.T) {
	previous := resolveRemoteFn
	resolveRemoteFn = func(host string) (*net.UDPAddr, error) {
		t.Errorf("the proposal resolved %q again; it MUST use the address the cycle resolved", host)
		return nil, net.UnknownNetworkError("resolution refused by the test")
	}
	t.Cleanup(func() { resolveRemoteFn = previous })

	peer := transportPeer()
	peer.RemoteAddress = "vpn.example.net"
	resolved := net.ParseIP("198.51.100.7").To4()
	sa := &SA{
		PeerCfg:     peer,
		IsInitiator: true,
		cycleRemote: resolved,
		cycleLocal:  net.ParseIP(peer.LocalAddress),
	}

	tsi, tsr := proposeChildTSPayloads(sa)
	if tsi == nil || tsr == nil {
		t.Fatal("transport-mode proposal produced no TS payloads")
	}
	if len(tsi.TrafficSelectors) != 1 || len(tsr.TrafficSelectors) != 1 {
		t.Fatalf("proposal carries %d TSi and %d TSr selectors, want one each",
			len(tsi.TrafficSelectors), len(tsr.TrafficSelectors))
	}

	// RFC 7296 Section 2.23.1: "The TSi entries MUST have exactly one IP address, and
	// that MUST match the source address of the IKE SA."
	wantLocal := net.ParseIP("10.0.0.1").To4()
	i := tsi.TrafficSelectors[0]
	if !bytesEqual(i.StartAddress, wantLocal) || !bytesEqual(i.EndAddress, wantLocal) {
		t.Errorf("TSi spans %v-%v, want exactly %v", i.StartAddress, i.EndAddress, wantLocal)
	}
	// RFC 7296 Section 2.23.1: "The TSr entries MUST have exactly one IP address, and
	// that MUST match the destination address of the IKE SA."
	r := tsr.TrafficSelectors[0]
	if !bytesEqual(r.StartAddress, resolved) || !bytesEqual(r.EndAddress, resolved) {
		t.Errorf("TSr spans %v-%v, want exactly the resolved %v", r.StartAddress, r.EndAddress, resolved)
	}
	if sa.ProposedChildPairs == nil {
		t.Error("the pinned pair was not recorded as the proposal, so the answer is judged against the wildcard")
	}

	host := configuredHostPair(sa)
	if host == nil || !host.remote.IP.Equal(resolved) {
		t.Errorf("host pair %+v, want remote %v: the installed transport pair follows the resolved address", host, resolved)
	}
}
