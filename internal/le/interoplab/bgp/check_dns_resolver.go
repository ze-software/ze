// Design: docs/architecture/testing/interop.md -- ze's LAN DNS resolver against Unbound, Knot DNS and dhcpcd.
// Related: check_dns_resolver_predicate.go -- the pure readings of kdig, dhcpcd and Unbound output.
// Related: register_dns_resolver.go -- the registration of both checkers.
package bgp

import (
	"context"
	"encoding/json"
	"errors"
	"net/netip"
	"strconv"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/interoplab"
)

const (
	scenarioDNSForwardUnbound = "dns-resolver-forward-unbound"
	scenarioDNSDHCPLAN        = "dns-resolver-dhcp-lan"

	// The DNS scenarios' sidecars: the file that selects each one and the
	// last byte of its lab address. dnsclient starts with every DNS
	// scenario; it is the LAN host that queries ze.
	knotConfigFile       = "knot.conf"
	knotZoneFile         = "lab.example.zone"
	unboundConfigFile    = "unbound.conf"
	peerKnot             = "knot"
	peerUnbound          = "unbound"
	peerDNSClient        = "dnsclient"
	knotHostNumber       = uint8(40)
	unboundHostNumber    = uint8(41)
	dnsClientHostNumber  = uint8(42)
	dnsLeasedHostNumber  = uint8(60)
	dnsUnboundLogLines   = 4000
	dnsDHCPTimeoutSecond = 40
	dnsReadyWait         = 30 * time.Second
	dnsPoll              = time.Second

	dnsNameWWW     = "www.lab.example."
	dnsNameAlias   = "alias.lab.example."
	dnsNameMissing = "missing.lab.example."
	dnsNameBig     = "big.lab.example."
	dnsBigRecords  = 30
)

// dnsWWWRecords is the RRset the lab zone holds for www
// (test/interop/scenarios/*/lab.example.zone).
var dnsWWWRecords = []string{"A 192.0.2.10", "A 192.0.2.11"}

// labIPv4 is host number host of the lab network, the address the
// scenario's 172.30.0.<host> renders to.
func labIPv4(network interoplab.Network, host uint8) string {
	octets := network.IPv4.Addr().As4()
	octets[3] = host
	return netip.AddrFrom4(octets).String()
}

// kdig runs one query from the dnsclient container and reads the reply.
// options are kdig's own (`+tcp`, `-b <source>`, ...).
func kdig(ctx context.Context, lab interoplab.CheckerLab, server, name, qtype string, options ...string) (kdigAnswer, error) {
	argv := append([]string{"kdig", "@" + server, name, qtype, "+retry=1", "+time=3"}, options...)
	output, err := lab.Query(ctx, peerDNSClient, argv, nil)
	if err != nil {
		return kdigAnswer{}, err
	}
	return parseKdig(output)
}

// waitDNSAnswer polls until server answers name with any reply, so a
// checker starts once Knot has loaded its zone and every hop is up.
func waitDNSAnswer(ctx context.Context, lab interoplab.CheckerLab, server, name string) error {
	deadline := time.Now().Add(dnsReadyWait)
	for {
		answer, err := kdig(ctx, lab, server, name, "A")
		if err == nil {
			if answer.status == "NOERROR" {
				return nil
			}
			var tb textbuf.Buffer
			err = errors.New(tb.Str(server).Str(" answered ").Str(answer.status).String())
		}
		if time.Now().After(deadline) {
			return err
		}
		if sleepErr := sleepContext(ctx, dnsPoll); sleepErr != nil {
			return sleepErr
		}
	}
}

// checkDNSForwardUnbound proves AC-1, AC-6 and the TC path against Unbound.
// ze answers each query with the RCODE and answer RRset Unbound itself
// gives; a repeated query is served from ze's cache, so Unbound logs ze
// asking once; and a 3 KB RRset reaches the client over TCP after Unbound
// truncated ze's UDP query, while ze's own UDP reply to the client sets TC.
func checkDNSForwardUnbound(ctx context.Context, check *interoplab.CheckContext) error {
	fail := func(assertion int, cause error) error {
		return checkerFailure(ctx, check.Lab, scenarioDNSForwardUnbound, assertion, cause)
	}
	ze := labIPv4(check.Network, zeHostNumber)
	unbound := labIPv4(check.Network, unboundHostNumber)
	// Assertion 1. Unbound resolves the lab zone from Knot.
	if err := waitDNSAnswer(ctx, check.Lab, unbound, dnsNameWWW); err != nil {
		return fail(1, err)
	}
	// Assertion 2. Per query, ze's answer equals Unbound's: an A RRset, a
	// CNAME chain and a name that does not exist.
	for _, name := range []string{dnsNameWWW, dnsNameAlias, dnsNameMissing} {
		fromZe, err := kdig(ctx, check.Lab, ze, name, "A")
		if err != nil {
			return fail(2, err)
		}
		fromUnbound, err := kdig(ctx, check.Lab, unbound, name, "A")
		if err != nil {
			return fail(2, err)
		}
		if err := sameAnswerVerdict(fromZe, fromUnbound); err != nil {
			return fail(2, errors.New(name+": "+err.Error()))
		}
	}
	first, err := kdig(ctx, check.Lab, ze, dnsNameWWW, "A")
	if err != nil {
		return fail(2, err)
	}
	if err := dnsAnswerVerdict(first, "NOERROR", dnsWWWRecords); err != nil {
		return fail(2, err)
	}
	// Assertion 3. The repeated www queries were served from ze's cache:
	// Unbound logs ze asking for www once.
	logs, err := check.Lab.Logs(ctx, peerUnbound, dnsUnboundLogLines)
	if err != nil {
		return fail(3, err)
	}
	if count := unboundQueryCount(logs.Text, ze, dnsNameWWW, "A"); count != 1 {
		return fail(3, errors.New("Unbound logged ze asking for "+dnsNameWWW+" "+strconv.Itoa(count)+" times, want 1 (two queries, the second from ze's cache)"))
	}
	// Assertion 4. The 3 KB RRset over TCP equals Unbound's, and ze asked
	// Unbound again over TCP after a truncated UDP reply.
	bigZe, err := kdig(ctx, check.Lab, ze, dnsNameBig, "TXT", "+tcp")
	if err != nil {
		return fail(4, err)
	}
	bigUnbound, err := kdig(ctx, check.Lab, unbound, dnsNameBig, "TXT", "+tcp")
	if err != nil {
		return fail(4, err)
	}
	if err := sameAnswerVerdict(bigZe, bigUnbound); err != nil {
		return fail(4, err)
	}
	if len(bigZe.rrs) != dnsBigRecords {
		return fail(4, errors.New("TXT RRset holds "+strconv.Itoa(len(bigZe.rrs))+" records, want "+strconv.Itoa(dnsBigRecords)))
	}
	truncated, err := zeUpstreamTruncated(ctx, check.Lab)
	if err != nil {
		return fail(4, err)
	}
	if truncated < 1 {
		return fail(4, errors.New("ze counted no truncated upstream reply, so it never retried over TCP"))
	}
	// Assertion 5. Over UDP with a 1232-octet buffer, ze's reply sets TC.
	bigUDP, err := kdig(ctx, check.Lab, ze, dnsNameBig, "TXT", "+ignore", "+bufsize=1232")
	if err != nil {
		return fail(5, err)
	}
	if err := truncatedVerdict(bigUDP); err != nil {
		return fail(5, err)
	}
	return nil
}

// zeUpstreamTruncated reads ze's count of upstream replies that carried TC
// from `show dns-resolver`.
func zeUpstreamTruncated(ctx context.Context, lab interoplab.CheckerLab) (uint64, error) {
	output, err := lab.Query(ctx, "ze", zeCommand("show dns-resolver"), nil)
	if err != nil {
		return 0, err
	}
	var decoded any
	if err := json.Unmarshal([]byte(output), &decoded); err != nil {
		return 0, errors.New("show dns-resolver is not JSON: " + err.Error())
	}
	return upstreamTruncatedOf(decoded, output)
}

// checkDNSDHCPLAN proves AC-32: the LAN host leases the pool address from
// ze's DHCP server, the lease's option 6 names ze's resolver, and the host
// resolves through it from the leased address, while its other address,
// outside allow-from, is refused.
func checkDNSDHCPLAN(ctx context.Context, check *interoplab.CheckContext) error {
	fail := func(assertion int, cause error) error {
		return checkerFailure(ctx, check.Lab, scenarioDNSDHCPLAN, assertion, cause)
	}
	ze := labIPv4(check.Network, zeHostNumber)
	leased := labIPv4(check.Network, dnsLeasedHostNumber)
	other := labIPv4(check.Network, dnsClientHostNumber)
	// Assertion 1. dhcpcd binds a lease from ze. -1 exits once bound and
	// leaves the address on eth0.
	bind := []string{"dhcpcd", "-4", "-1", "-w", "-t", strconv.Itoa(dnsDHCPTimeoutSecond), containerInterface}
	if _, err := check.Lab.Exec(ctx, peerDNSClient, bind, nil); err != nil {
		return fail(1, err)
	}
	// Assertion 2. The lease is the pool address and option 6 is ze.
	dump, err := check.Lab.Query(ctx, peerDNSClient, []string{"dhcpcd", "-4", "--dumplease", containerInterface}, nil)
	if err != nil {
		return fail(2, err)
	}
	lease, err := parseDhcpcdLease(dump)
	if err != nil {
		return fail(2, err)
	}
	if err := dhcpLeaseVerdict(lease, leased, ze); err != nil {
		return fail(2, err)
	}
	// Assertion 3. From the leased address, the host resolves through the
	// server the lease named.
	answer, err := kdig(ctx, check.Lab, lease.servers[0], dnsNameWWW, "A", "-b", leased)
	if err != nil {
		return fail(3, err)
	}
	if err := dnsAnswerVerdict(answer, "NOERROR", dnsWWWRecords); err != nil {
		return fail(3, err)
	}
	// Assertion 4. From the host's other address, outside allow-from, ze
	// refuses.
	refused, err := kdig(ctx, check.Lab, lease.servers[0], dnsNameWWW, "A", "-b", other)
	if err != nil {
		return fail(4, err)
	}
	if err := dnsAnswerVerdict(refused, "REFUSED", nil); err != nil {
		return fail(4, err)
	}
	return nil
}
