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
//
// Ze's listen port is the port the runner assigns to the test, never 179, so
// the .ci passes it as the driver's one argument (`$PORT`), and a rule on any
// other port is not the listen rule.

package fixture

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// gtsmListenRuleIPv4 and gtsmListenRuleIPv6 are the rules the .ci configures,
// each as the words one rendered nft rule carries for the listen port. nft
// prints a rule on one line, so a rule is present when one line holds every
// word as whole tokens.
func gtsmListenRuleIPv4(port string) []string {
	return []string{"saddr 192.0.2.2", "tcp dport " + port, "ttl < 255", "drop"}
}

func gtsmListenRuleIPv6(port string) []string {
	return []string{"saddr 2001:db8::2", "tcp dport " + port, "hoplimit < 255", "drop"}
}

// gtsmListenPort answers the listen port a GTSM driver was given: exactly one
// argument, a TCP port number. The runner assigns the port, so a driver that
// guessed one would judge a rule the daemon never rendered.
func gtsmListenPort(name string, args []string) (string, error) {
	if len(args) != 1 {
		return "", fmt.Errorf("%s takes Ze's listen port, got %d arguments", name, len(args))
	}
	port, err := strconv.ParseUint(args[0], 10, 16)
	if err != nil {
		return "", fmt.Errorf("%s: listen port %q is not a TCP port: %w", name, args[0], err)
	}
	if port == 0 {
		return "", fmt.Errorf("%s: listen port 0 is not a TCP port a daemon listens on", name)
	}
	return args[0], nil
}

// gtsmListenerFloor reads both listen-port rules, then withdraws them one
// reload at a time. Every read is a poll over kernel state: the daemon
// publishes once its peer reconcile runs, and no moment is fixed.
func gtsmListenerFloor(ctx context.Context, args []string) error {
	port, err := gtsmListenPort("firewall/gtsm-listener-floor", args)
	if err != nil {
		return err
	}
	ruleIPv4 := gtsmListenRuleIPv4(port)
	ruleIPv6 := gtsmListenRuleIPv6(port)

	pid, err := waitDaemon(ctx, 200)
	if err != nil {
		return err
	}

	var table string
	if !Poll(ctx, 100, 50*time.Millisecond, func() bool {
		table, err = netfilterCommandOutput(ctx, "nft", "list", "table", gtsmTableFamily, gtsmTableName)
		return err == nil && rulePresent(table, ruleIPv4) && rulePresent(table, ruleIPv6)
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
		return err == nil && !rulePresent(table, ruleIPv4) && rulePresent(table, ruleIPv6)
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
		if containsAll(strings.Fields(line), words) {
			return true
		}
	}
	return false
}

// containsAll reports whether the tokens of one rule line hold every word as
// a run of whole tokens. A substring match would read `dport 1790` as
// `dport 179`, so a rule on another port would pass for the listen rule.
func containsAll(tokens, words []string) bool {
	for _, word := range words {
		if !hasTokenRun(tokens, strings.Fields(word)) {
			return false
		}
	}
	return true
}

// hasTokenRun answers whether run appears in tokens as consecutive tokens.
func hasTokenRun(tokens, run []string) bool {
	for start := range len(tokens) - len(run) + 1 {
		if slices.Equal(tokens[start:start+len(run)], run) {
			return true
		}
	}
	return false
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
