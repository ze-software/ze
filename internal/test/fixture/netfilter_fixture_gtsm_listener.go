// Design: rfc/short/rfc5082.md -- the listen-port floor a GTSM peer owes before the handshake
// Related: netfilter_fixture_gtsm.go -- the related-ICMP half of the same table
//
// The driver for test/firewall/gtsm-listener-floor.ci. The daemon under test
// carries one IPv4 and one IPv6 BGP peer with `connection ttl max 1`. The
// reactor publishes both to internal/component/gtsm, which puts one rule per
// peer in the `ze_gtsm` table: TCP from the peer to Ze's listen port below the
// floor is dropped (spec-bcp194-3-session AC-1, AC-2). This driver reads both
// rules off the kernel, reloads a configuration without the IPv4 peer's `ttl`
// block and reads that its rule alone left, then reloads one without either
// block and reads the table withdrawn (AC-6).

package fixture

import (
	"context"
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"
)

// The rules the .ci configures, each as the words one rendered nft rule
// carries. nft prints a rule on one line, so a rule is present when one line
// holds every word.
var (
	gtsmListenRuleIPv4 = []string{"saddr 192.0.2.2", "dport 179", "ttl < 255", "drop"}
	gtsmListenRuleIPv6 = []string{"saddr 2001:db8::2", "dport 179", "hoplimit < 255", "drop"}
)

// gtsmListenerFloor reads both listen-port rules, then withdraws them one
// reload at a time. Every read is a poll over kernel state: the daemon
// publishes once its peer reconcile runs, and no moment is fixed.
func gtsmListenerFloor(ctx context.Context, _ []string) error {
	pid, err := waitDaemon(ctx, 200)
	if err != nil {
		return err
	}

	var table string
	if !Poll(ctx, 100, 50*time.Millisecond, func() bool {
		table, err = netfilterCommandOutput(ctx, "nft", "list", "table", gtsmTableFamily, gtsmTableName)
		return err == nil && rulePresent(table, gtsmListenRuleIPv4) && rulePresent(table, gtsmListenRuleIPv6)
	}) {
		return fmt.Errorf("CONFIGURED: %s table never carried the listen-port rule of both peers:\n%s", gtsmTableName, table)
	}
	fmt.Print(table)
	fmt.Fprintln(os.Stderr, "CONFIGURED: IPv4 listen rule present")
	fmt.Fprintln(os.Stderr, "CONFIGURED: IPv6 listen rule present")

	if err := reloadWith(pid, "config2.conf"); err != nil {
		return err
	}
	if !Poll(ctx, 200, 50*time.Millisecond, func() bool {
		table, err = netfilterCommandOutput(ctx, "nft", "list", "table", gtsmTableFamily, gtsmTableName)
		return err == nil && !rulePresent(table, gtsmListenRuleIPv4) && rulePresent(table, gtsmListenRuleIPv6)
	}) {
		return fmt.Errorf("RELOADED: the IPv4 peer's listen rule did not leave alone after its ttl block was removed:\n%s", table)
	}
	fmt.Fprintln(os.Stderr, "RELOADED: IPv4 listen rule withdrawn, IPv6 kept")

	if err := reloadWith(pid, "config3.conf"); err != nil {
		return err
	}
	if !Poll(ctx, 200, 50*time.Millisecond, func() bool {
		table, err = netfilterCommandOutput(ctx, "nft", "list", "table", gtsmTableFamily, gtsmTableName)
		// refuse-by-default: guards netfilterCommandOutput -- nft fails to list a table the kernel does not hold, which is the withdrawal this poll waits for
		return err != nil
	}) {
		return fmt.Errorf("RELOADED: %s table still present after the last ttl block was removed", gtsmTableName)
	}
	fmt.Fprintln(os.Stderr, "RELOADED: table withdrawn with the last GTSM peer")

	return signalProcess(pid, syscall.SIGTERM)
}

// rulePresent reports whether one line of the rendered table holds every word
// of the rule.
func rulePresent(table string, words []string) bool {
	for line := range strings.SplitSeq(table, "\n") {
		if containsAll(line, words) {
			return true
		}
	}
	return false
}

// containsAll reports whether line holds every word.
func containsAll(line string, words []string) bool {
	for _, word := range words {
		if !strings.Contains(line, word) {
			return false
		}
	}
	return true
}

// reloadWith stages the named configuration as the daemon's file and signals
// the daemon to reload it.
func reloadWith(pid int, name string) error {
	data, err := os.ReadFile(name) //nolint:gosec // name is one of this fixture's own staged config files
	if err != nil {
		return err
	}
	if err := os.WriteFile("ze-bgp.conf", data, 0o600); err != nil {
		return err
	}
	return signalProcess(pid, syscall.SIGHUP)
}
