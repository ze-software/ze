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

package fixture

import (
	"context"
	"fmt"
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

	var table string
	if !Poll(ctx, 100, 50*time.Millisecond, func() bool {
		table, err = netfilterCommandOutput(ctx, "nft", "list", "table", gtsmTableFamily, gtsmTableName)
		return err == nil && strings.Contains(table, gtsmPeerAddress)
	}) {
		return fmt.Errorf("CONFIGURED: %s table never carried a term for %s:\n%s", gtsmTableName, gtsmPeerAddress, table)
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

// gtsmHostRoute answers the main-table routes to the peer's host prefix. An
// empty answer with no error is the route's absence; a failed command is an
// error, so the withdrawal poll cannot read a query that did not run as an
// absence.
func gtsmHostRoute(ctx context.Context) (string, error) {
	return netfilterCommandOutput(ctx, "ip", "route", "show", "table", "main", gtsmPeerAddress+"/32")
}
