// Design: docs/architecture/testing/interop.md -- the virtual-IP scenarios, Ze leasing inner addresses to strongSwan clients.
// Related: checkers.go -- the scenario registry these two checkers join.
// Related: helpers.go -- ESP direction counters, policy pairs and the Ze CLI query.
// RFC: rfc/short/rfc3948.md -- Section 5.1, locally unique inner addresses for clients behind one NAT.
// RFC: rfc/short/rfc7296.md -- Section 2.19, the IRAS answering a CFG_REQUEST.
package ipsec

import (
	"context"
	"fmt"
	"maps"
	"net/netip"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/interoplab"
)

const (
	// vipZeInner is Ze's inner address in both scenarios: the local selector of every
	// pool-bound peer, and the remote_ts of every strongSwan child.
	vipZeInner = "10.20.0.1"

	// vipAssignPool and vipAssignDNS are pool "clients" of virtual-ip-assignment.
	vipAssignPool = "10.10.0.0/24"
	vipAssignDNS  = "10.10.0.53"

	// vipUniquePool is pool "clients" of virtual-ip-uniqueness, and vipRequested is
	// the address both of its clients ask for (swanctl `vips = 10.1.2.3`).
	vipUniquePool = "10.1.2.0/24"
	vipRequested  = "10.1.2.3"

	// vipSecondClient is the second client address strongSwan holds in
	// virtual-ip-uniqueness, and vipSecondPublic is where the NAT box shows it to Ze.
	vipSecondClient = "172.28.0.8"
	vipSecondPublic = "172.28.0.9"

	vipWaitTimeout = 90 * time.Second

	// zeStateEstablished is the state `show vpn ipsec sa` names an established IKE SA by.
	zeStateEstablished = "established"
)

// swanLocalVIP reads the virtual IP charon reports on the local line of one IKE SA of
// `swanctl --list-sas`: "  local  '<id>' @ <host>[<port>] [<vip>]". The bracketed
// address is charon's record of what the CFG_REPLY gave it.
var swanLocalVIP = regexp.MustCompile(`(?m)^ {2}local {2}'[^']*' @ \S+\[\d+\] \[([0-9a-fA-F.:]+)\]`)

// swanVirtualIP answers the one virtual IP one IKE SA of `swanctl --list-sas --ike`
// carries. An answer with none, or with one that does not parse, is refused: an SA
// that holds no lease is the failure these scenarios exist to catch.
func swanVirtualIP(answer string) (netip.Addr, error) {
	matches := swanLocalVIP.FindAllStringSubmatch(answer, -1)
	if len(matches) == 0 {
		return netip.Addr{}, fmt.Errorf("strongSwan reports no virtual IP on its IKE SA; answer:\n%s", interoplab.OutputExcerpt(answer))
	}
	if len(matches) > 1 {
		return netip.Addr{}, fmt.Errorf("strongSwan reports %d IKE SAs where one was asked for; answer:\n%s", len(matches), interoplab.OutputExcerpt(answer))
	}
	addr, err := netip.ParseAddr(matches[0][1])
	if err != nil {
		return netip.Addr{}, fmt.Errorf("strongSwan reports virtual IP %q, which is not an address: %w", matches[0][1], err)
	}
	return addr, nil
}

// requireLeaseInPool refuses a lease outside the pool, and the pool's network
// address, which Ze never hands out.
func requireLeaseInPool(lease netip.Addr, pool string) error {
	prefix, err := netip.ParsePrefix(pool)
	if err != nil {
		return fmt.Errorf("pool %q: %w", pool, err)
	}
	if !prefix.Contains(lease) {
		return fmt.Errorf("strongSwan was given %s, which is outside Ze's pool %s", lease, pool)
	}
	if lease == prefix.Masked().Addr() {
		return fmt.Errorf("strongSwan was given %s, the network address of Ze's pool %s", lease, pool)
	}
	return nil
}

// zeLease is what `show vpn ipsec sa` reports for one pool-bound peer.
type zeLease struct {
	virtualIP string
	tsLocal   string
	tsRemote  string
}

// zeLeaseOf reads the lease and the installed selectors of the one ESTABLISHED IKE SA
// of peer. It refuses no record, two records, a virtual-ip list that is not exactly
// one string, and an SA with no Child SA: each is an answer that cannot say which
// address the peer holds.
func zeLeaseOf(records []map[string]any, peer string) (zeLease, error) {
	var found []map[string]any
	for _, record := range records {
		if record["peer-name"] != peer {
			continue
		}
		if record["state"] != zeStateEstablished {
			continue
		}
		found = append(found, record)
	}
	if len(found) != 1 {
		return zeLease{}, fmt.Errorf("show vpn ipsec sa reports %d established IKE SAs for peer %s, want 1", len(found), peer)
	}
	record := found[0]
	list, ok := record["virtual-ip"].([]any)
	if !ok {
		return zeLease{}, fmt.Errorf("show vpn ipsec sa peer %s carries no virtual-ip list: %v", peer, record["virtual-ip"])
	}
	if len(list) != 1 {
		return zeLease{}, fmt.Errorf("show vpn ipsec sa peer %s virtual-ip holds %d addresses %v, want exactly the one lease", peer, len(list), list)
	}
	address, ok := list[0].(string)
	if !ok {
		return zeLease{}, fmt.Errorf("show vpn ipsec sa peer %s virtual-ip holds %v, not an address string", peer, list[0])
	}
	child, ok := record["child-sa"].(map[string]any)
	if !ok {
		return zeLease{}, fmt.Errorf("show vpn ipsec sa peer %s reports no child-sa", peer)
	}
	local, ok := child["ts-local"].(string)
	if !ok {
		return zeLease{}, fmt.Errorf("show vpn ipsec sa peer %s child-sa carries no ts-local string", peer)
	}
	remote, ok := child["ts-remote"].(string)
	if !ok {
		return zeLease{}, fmt.Errorf("show vpn ipsec sa peer %s child-sa carries no ts-remote string", peer)
	}
	return zeLease{virtualIP: address, tsLocal: local, tsRemote: remote}, nil
}

// requireZeLease refuses unless Ze reports exactly the lease strongSwan holds, and the
// Child SA it installed names that lease alone as the remote inner source (RFC 3948
// Section 5.1). A Child SA whose remote selector is the configured pool prefix, or
// the peer's proposed wildcard, is the substitution missing.
func requireZeLease(lease zeLease, peer string, want netip.Addr) error {
	if lease.virtualIP != want.String() {
		return fmt.Errorf("ze reports virtual-ip %s for peer %s, strongSwan holds %s", lease.virtualIP, peer, want)
	}
	if lease.tsLocal != vipZeInner+"/32" {
		return fmt.Errorf("ze's Child SA of peer %s has ts-local %s, want %s/32", peer, lease.tsLocal, vipZeInner)
	}
	if lease.tsRemote != want.String()+"/32" {
		return fmt.Errorf("ze's Child SA of peer %s has ts-remote %s, want the lease %s/32", peer, lease.tsRemote, want)
	}
	return nil
}

// requireLeasePolicies refuses unless Ze's kernel holds an ESP policy between its
// inner address and each lease, and none between its inner address and the pool
// prefix. The pool policy is the substitution missing, seen in the dataplane.
func requireLeasePolicies(pairs map[string]struct{}, leases []netip.Addr, pool string) error {
	for _, lease := range leases {
		if _, ok := pairs[policyPair(vipZeInner+"/32", lease.String()+"/32")]; !ok {
			return fmt.Errorf("ze's kernel holds no ESP policy between %s/32 and the lease %s/32; policies: %v",
				vipZeInner, lease, slices.Sorted(maps.Keys(pairs)))
		}
	}
	if _, ok := pairs[policyPair(vipZeInner+"/32", pool)]; ok {
		return fmt.Errorf("ze's kernel holds an ESP policy for the whole pool %s, so the lease did not replace the selector; policies: %v",
			pool, slices.Sorted(maps.Keys(pairs)))
	}
	return nil
}

// waitSwanIKE waits for one strongSwan IKE SA to be ESTABLISHED with an INSTALLED
// Child SA, and answers its `swanctl --list-sas --ike` text.
func (l *scenarioLab) waitSwanIKE(ctx context.Context, ike string) (string, error) {
	var tb textbuf.Buffer
	description := tb.Str("strongSwan IKE SA ").Str(ike).Str(" with an installed Child SA").String()
	answer, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: vipWaitTimeout, Interval: 2 * time.Second, Description: description,
	}, func(probe context.Context) (string, error) {
		return l.exec(probe, swanPeer, cmdSwanctl, "--list-sas", "--ike", ike)
	}, func(output string) bool {
		if !strings.Contains(output, "ESTABLISHED") {
			return false
		}
		return strings.Contains(output, "INSTALLED")
	})
	return answer, err
}

// swanLease reads the lease of one strongSwan IKE SA, and requires charon to have
// installed it on an interface and to have narrowed its child to it.
func (l *scenarioLab) swanLease(ctx context.Context, ike string) (netip.Addr, error) {
	answer, err := l.waitSwanIKE(ctx, ike)
	if err != nil {
		return netip.Addr{}, err
	}
	lease, err := swanVirtualIP(answer)
	if err != nil {
		return netip.Addr{}, err
	}
	if err := requireContains(answer, "local  "+lease.String()+"/32",
		"strongSwan's Child SA does not name its lease as the local selector"); err != nil {
		return netip.Addr{}, err
	}
	if err := requireContains(answer, "remote "+vipZeInner+"/32",
		"strongSwan's Child SA does not name Ze's inner address as the remote selector"); err != nil {
		return netip.Addr{}, err
	}
	addresses, err := l.exec(ctx, swanPeer, "ip", "-o", ipObjectAddress, "show")
	if err != nil {
		return netip.Addr{}, err
	}
	if err := requireContains(addresses, "inet "+lease.String()+"/",
		"charon did not install its lease on an interface"); err != nil {
		return netip.Addr{}, err
	}
	return lease, nil
}

// requireZeLeases reads `show vpn ipsec sa | json` once and requires each named peer to hold
// the lease strongSwan holds for it, in the same order.
func (l *scenarioLab) requireZeLeases(ctx context.Context, peers []string, leases []netip.Addr) error {
	records, answer, err := l.zeIKESAs(ctx)
	if err != nil {
		return err
	}
	for index, peer := range peers {
		lease, err := zeLeaseOf(records, peer)
		if err == nil {
			err = requireZeLease(lease, peer, leases[index])
		}
		if err != nil {
			return fmt.Errorf("%w; answer: %s", err, answer)
		}
	}
	return nil
}

// checkVirtualIPAssignment proves Ze is an IRAS for a strongSwan road-warrior client
// over EAP-MSCHAPv2 (RFC 7296 Section 2.19), as the spec's G-3 states it.
//
// Each observation is a different event, so no one of them answers for another:
//  1. charon reports a virtual IP inside Ze's pool, installed it on an interface and
//     narrowed its child to it: the CFG_REPLY reached strongSwan and was used.
//  2. Ze reports the same address as the peer's virtual-ip, and its Child SA's remote
//     selector is that address alone: the lease replaced the proposed selector.
//  3. Ze's kernel holds the lease policy and no pool-wide one: the dataplane agrees.
//  4. charon installed the pool's DNS server: the CFG_REPLY carried the attribute.
//  5. A ping from each end, each from its inner address, moves ESP on all four
//     simplex SAs: the Child SA the lease selects carries traffic both ways.
func checkVirtualIPAssignment(ctx context.Context, lab *scenarioLab) error {
	lease, err := lab.swanLease(ctx, swanConnection)
	if err != nil {
		return err
	}
	if err := requireLeaseInPool(lease, vipAssignPool); err != nil {
		return err
	}
	if err := lab.requireZeLeases(ctx, []string{swanConfigPeer}, []netip.Addr{lease}); err != nil {
		return err
	}
	pairs, err := lab.espPolicyPairs(ctx, zePeer)
	if err != nil {
		return err
	}
	if err := requireLeasePolicies(pairs, []netip.Addr{lease}, vipAssignPool); err != nil {
		return err
	}
	resolver, err := lab.exec(ctx, swanPeer, "cat", "/etc/resolv.conf")
	if err != nil {
		return err
	}
	if err := requireContains(resolver, "nameserver "+vipAssignDNS,
		"charon did not install the DNS server Ze's CFG_REPLY carries"); err != nil {
		return err
	}
	if err := lab.natInnerAddress(ctx, zePeer, vipZeInner, lease.String()); err != nil {
		return err
	}
	if err := lab.verifyESPDirectionsToward(ctx, "Ze's traffic to the lease did not cross the Child SA",
		pingProbe{peer: zePeer, target: lease.String(), source: vipZeInner}, espBothDirections); err != nil {
		return err
	}
	return lab.verifyESPDirectionsToward(ctx, "traffic from the lease did not cross the Child SA",
		pingProbe{peer: swanPeer, target: vipZeInner, source: lease.String()}, espBothDirections)
}

// vipClient is one client of virtual-ip-uniqueness: its strongSwan connection and
// child, its Ze peer, and the four simplex SAs of its round trip through the NAT box.
type vipClient struct {
	ike        string
	child      string
	peer       string
	directions []espDirection
}

func vipClientDirections(client, public string) []espDirection {
	return []espDirection{
		{peer: zePeer, source: zeIP, target: public, summary: "Ze encrypted nothing toward client " + public},
		{peer: swanPeer, source: zePublicIP, target: client, summary: "client " + client + " accepted no ESP from Ze"},
		{peer: swanPeer, source: client, target: zePublicIP, summary: "client " + client + " encrypted nothing toward Ze"},
		{peer: zePeer, source: public, target: zeIP, summary: "Ze decrypted no ESP from client " + public},
	}
}

var vipClients = []vipClient{
	{ike: "ze-a", child: "child-a", peer: "swan-a", directions: vipClientDirections(swanIP, swanPublicIP)},
	{ike: "ze-b", child: "child-b", peer: "swan-b", directions: vipClientDirections(vipSecondClient, vipSecondPublic)},
}

// requireDistinctLeases refuses two leases that are equal, or a pair of which neither
// is the address both clients asked for, or one outside the pool.
func requireDistinctLeases(leases []netip.Addr, pool string) error {
	if len(leases) != 2 {
		return fmt.Errorf("read %d leases, want 2", len(leases))
	}
	if leases[0] == leases[1] {
		return fmt.Errorf("both clients hold %s: two Child SAs lead to one inner address (RFC 3948 Section 5.1)", leases[0])
	}
	if !slices.Contains(leases, netip.MustParseAddr(vipRequested)) {
		return fmt.Errorf("neither client holds the address both asked for, %s: leases %v", vipRequested, leases)
	}
	for _, lease := range leases {
		if err := requireLeaseInPool(lease, pool); err != nil {
			return err
		}
	}
	return nil
}

// checkVirtualIPUniqueness proves RFC 3948 Section 5.1 against two strongSwan clients
// behind one NAT that both choose 10.1.2.3: Ze holds two distinct leases, one of them
// the requested address, each Child SA names its own lease, and return traffic to each
// lease leaves on that client's SA and no other.
func checkVirtualIPUniqueness(ctx context.Context, lab *scenarioLab) error {
	if _, err := lab.exec(ctx, swanPeer, "ip", ipObjectAddress, "add", vipSecondClient+"/32", ipArgDev, containerInterface); err != nil {
		return fmt.Errorf("add the second client address %s to strongSwan: %w", vipSecondClient, err)
	}
	leases := make([]netip.Addr, 0, len(vipClients))
	peers := make([]string, 0, len(vipClients))
	for _, client := range vipClients {
		if _, err := lab.exec(ctx, swanPeer, cmdSwanctl, "--initiate", "--child", client.child); err != nil {
			return fmt.Errorf("strongSwan could not bring up %s: %w", client.ike, err)
		}
		lease, err := lab.swanLease(ctx, client.ike)
		if err != nil {
			return fmt.Errorf("%s: %w", client.ike, err)
		}
		leases = append(leases, lease)
		peers = append(peers, client.peer)
	}
	if err := requireDistinctLeases(leases, vipUniquePool); err != nil {
		return err
	}
	if err := lab.requireZeLeases(ctx, peers, leases); err != nil {
		return err
	}
	pairs, err := lab.espPolicyPairs(ctx, zePeer)
	if err != nil {
		return err
	}
	if err := requireLeasePolicies(pairs, leases, vipUniquePool); err != nil {
		return err
	}
	for _, lease := range leases {
		if err := lab.natInnerAddress(ctx, zePeer, vipZeInner, lease.String()); err != nil {
			return err
		}
	}
	for index, client := range vipClients {
		other := vipClients[1-index]
		if err := lab.requireReturnTraffic(ctx, leases[index], client, other); err != nil {
			return err
		}
	}
	return nil
}

// requireReturnTraffic pings one lease from Ze's inner address and requires that
// client's four simplex SAs to move, and the other client's outbound SA at Ze to stay
// still: an echo request for this lease that left on the other client's SA is exactly
// the misdelivery RFC 3948 Section 5.1 describes.
func (l *scenarioLab) requireReturnTraffic(ctx context.Context, lease netip.Addr, client, other vipClient) error {
	watched := append(slices.Clone(client.directions), other.directions[0])
	before, err := l.espCounters(ctx, watched)
	if err != nil {
		return err
	}
	if err := l.requireLosslessPingFrom(ctx, pingProbe{peer: zePeer, target: lease.String(), source: vipZeInner}, 4); err != nil {
		return fmt.Errorf("return traffic to %s (%s): %w", lease, client.ike, err)
	}
	after, err := l.espCounters(ctx, watched)
	if err != nil {
		return err
	}
	for _, want := range client.directions {
		if err := assertESPAdvanced(before[want.peer], after[want.peer], want); err != nil {
			return fmt.Errorf("return traffic to %s (%s): %w", lease, client.ike, err)
		}
	}
	stray := other.directions[0]
	err = assertESPAdvanced(before[stray.peer], after[stray.peer], stray)
	if err == nil {
		return fmt.Errorf("return traffic to %s (%s) also moved Ze's SA toward %s, the other client", lease, client.ike, stray.target)
	}
	if strings.Contains(err.Error(), "reports no SA") {
		return fmt.Errorf("return traffic to %s (%s): the other client's SA is missing: %w", lease, client.ike, err)
	}
	return nil
}
