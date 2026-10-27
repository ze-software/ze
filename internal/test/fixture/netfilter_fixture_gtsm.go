// Design: rfc/short/rfc5082.md -- the kernel state a GTSM peer owes for its related ICMP messages
// Related: netfilter_fixture.go -- the registration table these drivers hang off
//
// The driver for test/firewall/gtsm-related-icmp.ci. The daemon under test
// carries one BGP peer with `connection ttl max 1`, which the reactor
// publishes to internal/component/gtsm. Two pieces of kernel state follow: a
// host route to the peer carrying hop limit 255, and the `ze_gtsm` nftables
// table that drops a Dangerous related ICMP error. This driver reads both off
// the kernel, then removes the `ttl` block by a config reload and reads their
// absence, so the operator-visible path from a configuration document to the
// kernel and back is what the .ci asserts on (spec AC-1, AC-7, AC-8).
//
// The table's terms name the peer by the IPv4 header the error QUOTES, never
// by the error's own source (spec-gtsm-related-icmp-quoted-destination AC-1).
// So the only rule carrying the peer's address in dotted form is the listen
// rule, which drops a SYN from the peer below the floor and reads the outer
// source by design (checkGTSMRuleset). nft prints the quoted read as a raw transport payload compare
// (`@th,192,32 0xc0000202`). quotedDestinationRead renders the same form from
// the peer's address, for the poll and the .ci to assert on.

package fixture

import (
	"context"
	"encoding/binary"
	"fmt"
	"net/netip"
	"os"
	"strings"
	"syscall"
	"time"
)

const (
	// gtsmPeerAddress is the peer the .ci configures. Its `option=netns-link`
	// gives the test namespace an interface carrying the peer's subnet, so the
	// kernel resolves a route the daemon's host route copies its nexthop from.
	gtsmPeerAddress = "192.0.2.2"
	gtsmTableFamily = "inet"
	gtsmTableName   = "ze_gtsm"
)

// gtsmRelatedICMP reads the peer's kernel state while the `ttl` block is
// configured, reloads a configuration without it, and reads the withdrawal.
//
// Every read is a poll over kernel state, never a wait on time: the daemon
// publishes the state once the peer reconcile runs, and the reload withdraws
// it once the next reconcile runs, and neither moment is fixed.
func gtsmRelatedICMP(ctx context.Context, _ []string) error {
	pid, err := waitDaemon(ctx, 200)
	if err != nil {
		return err
	}

	quoted, err := quotedDestinationRead(gtsmPeerAddress)
	if err != nil {
		return err
	}
	var table string
	if !Poll(ctx, 100, 50*time.Millisecond, func() bool {
		table, err = netfilterCommandOutput(ctx, "nft", "list", "table", gtsmTableFamily, gtsmTableName)
		return err == nil && strings.Contains(table, quoted)
	}) {
		return fmt.Errorf("CONFIGURED: %s table never carried a term reading the quoted destination %s (%s):\n%s", gtsmTableName, gtsmPeerAddress, quoted, table)
	}
	if err := checkGTSMRuleset(table, gtsmPeerAddress, quoted); err != nil {
		return fmt.Errorf("CONFIGURED: %w:\n%s", err, table)
	}
	fmt.Print(table)
	fmt.Fprintln(os.Stderr, "CONFIGURED: table present")

	var route string
	if !Poll(ctx, 100, 50*time.Millisecond, func() bool {
		route, err = gtsmHostRoute(ctx)
		return err == nil && strings.Contains(route, "hoplimit 255")
	}) {
		return fmt.Errorf("CONFIGURED: no host route to %s carrying hoplimit 255:\n%s", gtsmPeerAddress, route)
	}
	fmt.Print(route)
	fmt.Fprintln(os.Stderr, "CONFIGURED: route present")

	if err := stageReloadConfig(); err != nil {
		return err
	}
	if err := signalProcess(pid, syscall.SIGHUP); err != nil {
		return err
	}

	if !Poll(ctx, 200, 50*time.Millisecond, func() bool {
		_, listErr := netfilterCommandOutput(ctx, "nft", "list", "table", gtsmTableFamily, gtsmTableName)
		return listErr != nil
	}) {
		return fmt.Errorf("RELOADED: %s table still present after the ttl block was removed", gtsmTableName)
	}
	fmt.Fprintln(os.Stderr, "RELOADED: table withdrawn")

	if !Poll(ctx, 200, 50*time.Millisecond, func() bool {
		route, err = gtsmHostRoute(ctx)
		return err == nil && !strings.Contains(route, "hoplimit")
	}) {
		return fmt.Errorf("RELOADED: host route to %s still carries a hop limit:\n%s", gtsmPeerAddress, route)
	}
	fmt.Fprintln(os.Stderr, "RELOADED: route withdrawn")

	return signalProcess(pid, syscall.SIGTERM)
}

// gtsmRelatedICMPRuleCount is the number of related-ICMP terms the GTSM
// component publishes for one IPv4 peer (peerTerms,
// internal/component/gtsm/gtsm.go): each carries the quoted-destination read.
const gtsmRelatedICMPRuleCount = 10

// checkGTSMRuleset judges the rendered ze_gtsm table of one IPv4 peer. The
// table holds two kinds of rule, and each is recognized by its own shape,
// never by the absence of the other's:
//
//   - the listen rule reads the outer source on purpose, because a SYN's source
//     is the peer: `saddr <peer>`, `tcp dport 179`, a TTL compare, and no
//     `@th,` read. Exactly one line has that shape.
//   - every related-ICMP rule names the peer by the quoted header alone. Exactly
//     gtsmRelatedICMPRuleCount lines carry the quoted read.
//
// A `saddr` (ip or ip6) on any line other than the listen rule is refused, so
// an ICMP rule keyed on the error's own source fails here whether or not it
// also carries the quoted read, and at whatever offset it reads.
func checkGTSMRuleset(table, peer, quoted string) error {
	listenRules := 0
	relatedRules := 0
	for line := range strings.SplitSeq(table, "\n") {
		if strings.Contains(line, quoted) {
			relatedRules++
		}
		if isGTSMListenRule(line, peer) {
			listenRules++
			continue
		}
		if strings.Contains(line, "saddr") {
			return fmt.Errorf("%s table has a rule other than the listen rule that reads the packet's own source, and a related ICMP error is associated by the quoted header alone: %q", gtsmTableName, strings.TrimSpace(line))
		}
	}
	if listenRules != 1 {
		return fmt.Errorf("%s table holds %d listen rules for %s (saddr, tcp dport 179, a ttl compare, no @th read), want exactly 1", gtsmTableName, listenRules, peer)
	}
	if relatedRules != gtsmRelatedICMPRuleCount {
		return fmt.Errorf("%s table holds %d rules reading the quoted destination %s, want exactly %d", gtsmTableName, relatedRules, quoted, gtsmRelatedICMPRuleCount)
	}
	return nil
}

// isGTSMListenRule answers whether one rendered rule line has the listen
// rule's shape for peer: it reads `saddr <peer>` and `dport 179` as whole
// tokens, compares the TTL (`ttl <`), and reads no raw transport payload.
func isGTSMListenRule(line, peer string) bool {
	if strings.Contains(line, "@th,") {
		return false
	}
	tokens := strings.Fields(line)
	return hasTokenPair(tokens, "saddr", peer) && hasTokenPair(tokens, "tcp", "dport") &&
		hasTokenPair(tokens, "dport", "179") && hasTokenPair(tokens, "ttl", "<")
}

// hasTokenPair answers whether first is immediately followed by second.
func hasTokenPair(tokens []string, first, second string) bool {
	for i := range len(tokens) - 1 {
		if tokens[i] == first && tokens[i+1] == second {
			return true
		}
	}
	return false
}

// quotedDestinationRead renders the term nft prints for the quoted-destination
// match of a GTSM peer. The match reads the 4 octets at transport offset 24 of
// an ICMPv4 error, which is the destination of the IPv4 header the error
// quotes (lowerICMPErrorQuotedDestinationMatch,
// internal/plugins/firewall/nft/lower_linux.go). nft has no name for a field
// inside a quoted header. So it prints the raw payload read as
// `@th,<bit offset>,<bit length>`, and the compared value as one big-endian
// hexadecimal number with no zero padding (`0x6` for one octet of 0x06), so
// the render here pads none either.
func quotedDestinationRead(peer string) (string, error) {
	addr, err := netip.ParseAddr(peer)
	if err != nil {
		return "", fmt.Errorf("quoted destination: %w", err)
	}
	if !addr.Is4() {
		return "", fmt.Errorf("quoted destination: %s is not IPv4, and the ze_gtsm receive table is IPv4-only", peer)
	}
	octets := addr.As4()
	return fmt.Sprintf("@th,192,32 0x%x", binary.BigEndian.Uint32(octets[:])), nil
}

// gtsmHostRoute answers the main-table routes to the peer's host prefix. An
// empty answer with no error is the route's absence; a failed command is an
// error, so the withdrawal poll cannot read a query that did not run as an
// absence.
func gtsmHostRoute(ctx context.Context) (string, error) {
	return netfilterCommandOutput(ctx, "ip", "route", "show", "table", "main", gtsmPeerAddress+"/32")
}
