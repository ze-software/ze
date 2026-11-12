// Design: docs/features/dns-resolver.md -- show system dns drops a consumer the engine removed
// Overview: register_dns_source_withdraw_consumers.go -- the observers registered from these functions
// Related: dns_source_withdraw.go -- pollSystemDNS, the markers, the dns-resolver case
// Related: dns_source_operator.go -- showSystemDNSAnswer, showDNSSelection, judgeSelection

package fixture

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"os"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// The DHCP server's subnet and configured option 6 server that
// test/plugin/show-system-dns-withdraw-dhcp-server.ci writes.
var (
	withdrawDHCPSubnet = netip.MustParsePrefix("198.18.21.0/24")
	withdrawDHCPServer = netip.MustParseAddr("198.51.100.1")
)

const (
	// The consumer names the DHCP server and the l2tp pool publish under
	// (dhcpserver dhcpConsumer, pool poolConsumer).
	withdrawDHCPConsumer = "dhcpserver"
	withdrawPoolConsumer = "l2tp-pool"
	// The path every DHCP subnet's selection carries (dhcpserver
	// subnetDNSSourcePath); the subnet itself is the selection's instance.
	withdrawDHCPPath = "service dhcp-server shared-network subnet dns-source"
)

// withdrawCase is one consumer whose removal the observer proves: its
// consumer name, the judge of its pre-reload selection, and the two lines
// the .ci expects.
type withdrawCase struct {
	consumer string
	listed   func(*showSystemDNSAnswer) error
	before   string
	after    string
}

// showSystemDNSWithdrawDHCP proves, through the daemon dispatcher, that a
// DHCP server the reload removes leaves show system dns: its OnBye handler
// sends the withdrawal while the engine removes it.
func showSystemDNSWithdrawDHCP(ctx context.Context, p *sdk.Plugin) error {
	return showSystemDNSWithdrawCase(ctx, p, &withdrawCase{
		consumer: withdrawDHCPConsumer,
		listed:   judgeDHCPListed,
		before:   "OK: show system dns lists the DHCP server's subnet selection before the reload",
		after:    "OK: show system dns drops the removed DHCP server and keeps the router selection",
	})
}

// showSystemDNSWithdrawPool proves, through the daemon dispatcher, that an
// l2tp pool plugin the reload removes leaves show system dns: its OnBye
// handler sends the withdrawal while the engine removes it.
func showSystemDNSWithdrawPool(ctx context.Context, p *sdk.Plugin) error {
	return showSystemDNSWithdrawCase(ctx, p, &withdrawCase{
		consumer: withdrawPoolConsumer,
		listed:   judgePoolListed,
		before:   "OK: show system dns lists the l2tp pool's default list before the reload",
		after:    "OK: show system dns drops the removed l2tp pool and keeps the router selection",
	})
}

// showSystemDNSWithdrawCase waits until show system dns lists c's
// selection, hands the reload to the trigger through the markers, then waits
// until the answer still carries the router's selection, so it came from a
// live registry, and carries no c.consumer at all.
func showSystemDNSWithdrawCase(ctx context.Context, p *sdk.Plugin, c *withdrawCase) error {
	if err := pollSystemDNS(ctx, p, "before the reload", c.listed); err != nil {
		return err
	}
	// Stderr: a plugin's stdout carries the plugin protocol.
	if _, err := fmt.Fprintln(os.Stderr, c.before); err != nil {
		return err
	}
	if err := os.WriteFile(withdrawInitialMarker, []byte("ok"), 0o600); err != nil {
		return fmt.Errorf("write %s: %w", withdrawInitialMarker, err)
	}
	if !Poll(ctx, withdrawMarkerPolls, withdrawMarkerDelay, func() bool {
		_, err := os.Stat(withdrawReloadMarker)
		return err == nil
	}) {
		return errors.New("the trigger wrote no " + withdrawReloadMarker + ": no reload was sent")
	}
	withdrawn := func(answer *showSystemDNSAnswer) error {
		return judgeConsumerWithdrawn(answer, c.consumer)
	}
	if err := pollSystemDNS(ctx, p, "after the reload", withdrawn); err != nil {
		return err
	}
	_, err := fmt.Fprintln(os.Stderr, c.after)
	return err
}

// judgeDHCPListed names the first expectation the pre-reload answer misses:
// the router's selection, and the DHCP subnet's configured server.
func judgeDHCPListed(answer *showSystemDNSAnswer) error {
	if showDNSSelection(answer, "system", "") == nil {
		return errors.New("no router selection (consumer system)")
	}
	for i := range answer.Consumers {
		if answer.Consumers[i].Consumer != withdrawDHCPConsumer {
			continue
		}
		for j := range answer.Consumers[i].Selections {
			selection := &answer.Consumers[i].Selections[j]
			if selection.Path != withdrawDHCPPath {
				continue
			}
			if selection.Instance != withdrawDHCPSubnet.String() {
				continue
			}
			return judgeSelection(selection, withdrawDHCPServer)
		}
	}
	return fmt.Errorf("no %s selection at %q for subnet %s", withdrawDHCPConsumer, withdrawDHCPPath, withdrawDHCPSubnet)
}

// judgePoolListed names the first expectation the pre-reload answer misses:
// the router's selection, and the pool's built-in list naming its
// dns-primary.
func judgePoolListed(answer *showSystemDNSAnswer) error {
	if showDNSSelection(answer, "system", "") == nil {
		return errors.New("no router selection (consumer system)")
	}
	pool := showDNSSelection(answer, withdrawPoolConsumer, showDNSPoolPath)
	if pool == nil {
		return fmt.Errorf("no %s selection at %q", withdrawPoolConsumer, showDNSPoolPath)
	}
	return judgeSelection(pool, showDNSPoolPrimary)
}

// judgeConsumerWithdrawn names the first expectation the post-reload answer
// misses: the router's selection is still there, and consumer is not, not
// even with no selection left.
func judgeConsumerWithdrawn(answer *showSystemDNSAnswer, consumer string) error {
	if showDNSSelection(answer, "system", "") == nil {
		return errors.New("no router selection (consumer system)")
	}
	for i := range answer.Consumers {
		if answer.Consumers[i].Consumer == consumer {
			return fmt.Errorf("the removed %s is still listed with %d selection(s)",
				consumer, len(answer.Consumers[i].Selections))
		}
	}
	return nil
}
