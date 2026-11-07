// Design: docs/architecture/ike/ipsec-7-ikev2-engine.md -- the addresses a connect cycle measured
// Related: fsm.go -- runInitiator, which resolves the pair at the start of every cycle
// Related: ts_nat_substitute.go -- observedLocalAddress and observedRemoteAddress, built on these
// Related: dynamic_wan.go -- the address worker that restarts a session whose source left the host
// RFC: rfc/short/rfc7296.md -- NAT detection (Section 2.23)

package engine

import (
	"fmt"
	"net"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
)

// resolveRemoteFn resolves the remote-address leaf, an IPv4 literal or a DNS name, to
// the address and port one connect cycle dials. IKE over IPv6 is not supported, so the
// lookup is udp4. Swapped in unit tests, which cannot depend on a resolver.
var resolveRemoteFn = func(host string) (*net.UDPAddr, error) {
	return net.ResolveUDPAddr("udp4", ikeAddr(host))
}

// routeSourceFn is routeSourceTo, swapped in unit tests that cannot change the host's
// routing table. Production never replaces it.
var routeSourceFn = routeSourceTo

// cyclePair is what one connect cycle measured: the address it dials and the address it
// sends from. Both are set on success.
type cyclePair struct {
	remote *net.UDPAddr
	local  net.IP
}

// resolveCyclePair measures the pair a connect cycle of an initiating peer runs on.
//
// The remote is resolved here, once per cycle, so a name that answers with a new address
// is dialed at that address on the next cycle, and a name that does not resolve fails
// the cycle: PeerSession.run counts the failure and backs off (reconnectDelay), so a CPE
// booted while DNS is down keeps trying and establishes once the name answers.
//
// The local is the peer's own literal when it has one. That literal is either what the
// operator wrote or the interface address the apply filled in (applyConfig). A peer with
// neither, while `vpn ipsec interface` is unset, takes the source address the routing
// table picks toward the resolved remote: the route-chosen mode. routeSourceTo asks the
// routing table and connects no socket, so it answers under the SPD catch-all DISCARD.
//
// A peer whose interface has no address is never started (planPeers, dynamic_wan.go), so
// an empty LocalAddress reaching this function always means the route-chosen mode.
func resolveCyclePair(name string, peer ipsec.SiteToSitePeer) (cyclePair, error) {
	remote, err := resolveRemoteFn(peer.RemoteAddress)
	if err != nil {
		return cyclePair{}, fmt.Errorf("ike: peer %s remote-address %q does not resolve (%w), retrying with backoff; "+
			"check the name and the system resolvers, or give the address with: set vpn ipsec site-to-site peer %s remote-address <ipv4>",
			name, peer.RemoteAddress, err, name)
	}
	if peer.LocalAddress != "" {
		local := net.ParseIP(peer.LocalAddress)
		if local == nil {
			return cyclePair{}, fmt.Errorf("ike: peer %s local-address %q is not an IP address; fix it with: set vpn ipsec site-to-site peer %s local-address <ipv4>",
				name, peer.LocalAddress, name)
		}
		return cyclePair{remote: remote, local: local}, nil
	}
	source, err := routeSourceFn(remote.IP)
	if err != nil {
		return cyclePair{}, fmt.Errorf("ike: peer %s has no local-address and the routing table names no source toward %s (%w); "+
			"add a route to the peer, or set one with: set vpn ipsec site-to-site peer %s local-address <ipv4>",
			name, remote.IP, err, name)
	}
	return cyclePair{remote: remote, local: source}, nil
}

// cycleRemoteAddress returns the remote address of the IKE SA as the cycle resolved it,
// or the configured literal when no cycle measured one (an SA a test built directly).
// A remote-address written as a DNS name therefore reaches every reader as the address
// it resolved to, where net.ParseIP of the name answered nil and each reader skipped its
// work: the NAT_DETECTION payloads, their check, the Child SA endpoints, the wildcard
// selector family and the transport host pair.
//
// The returned slice is shared; callers MUST NOT modify it.
func cycleRemoteAddress(sa *SA) net.IP {
	if sa.cycleRemote != nil {
		return sa.cycleRemote
	}
	return net.ParseIP(sa.PeerCfg.RemoteAddress)
}

// cycleLocalAddress returns the local address of the IKE SA as the cycle measured it, or
// the configured literal when no cycle measured one. A route-chosen peer has no literal,
// so this is the only place its address is found.
//
// The returned slice is shared; callers MUST NOT modify it.
func cycleLocalAddress(sa *SA) net.IP {
	if sa.cycleLocal != nil {
		return sa.cycleLocal
	}
	return net.ParseIP(sa.PeerCfg.LocalAddress)
}

// ipText renders ip for the string-typed readers (newChildSA, createFirstChildSA). A nil
// address renders empty, which those readers refuse as they refused an empty leaf.
func ipText(ip net.IP) string {
	if ip == nil {
		return ""
	}
	return ip.String()
}
