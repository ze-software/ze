// Design: docs/architecture/testing/interop.md -- causal watchdog completion fence before an absence assertion
// FRR: https://github.com/FRRouting/frr/blob/frr-10.7.1/bgpd/bgp_vty.c --
// bgp_show_peer_gr_extra_info publishes endOfRibRecv only after receiving EOR
// on an Established session whose peer advertised Graceful Restart.
// FRR: https://github.com/FRRouting/frr/blob/frr-10.7.1/bgpd/bgp_route.c --
// bgp_show_route and bgp_show_table supply the JSON consumed by the route checks.
// Related: check_rapid_reconnect_initial_sync.go -- strict FRR neighbor decoder

package bgp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/le/interoplab"
)

const (
	watchdogFencePrefix = "10.77.4.0/24"
	watchdogHeldPrefix  = "10.77.3.0/24"
)

// checkWatchdogWithdrawValueFRR waits for the watchdog's credited completion,
// drains the earlier writes, and injects a fresh unconfigured fence. FRR must
// receive that fence before its held-route absence can be judged. Initial EOR
// is a protocol control: SignalsSessionReady deliberately does not hold EOR.
func checkWatchdogWithdrawValueFRR(ctx context.Context, check *interoplab.CheckContext) error {
	if !check.Network.IPv4.IsValid() {
		return errors.New("the watchdog value scenario has no selected IPv4 network")
	}
	address := networkHostAddress(check.Network, zeHostNumber)
	initial, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     60 * time.Second,
		Interval:    100 * time.Millisecond,
		Description: "FRR received Ze's initial IPv4 End-of-RIB",
	}, func(probe context.Context) (string, error) {
		return check.Lab.Query(probe, peerFRR, []string{cmdVtysh, "-c", "show bgp neighbor " + address + " json"}, nil)
	}, func(output string) bool {
		return watchdogInitialEOR(output, address) == nil
	})
	if err != nil {
		return fmt.Errorf("FRR did not receive the initial IPv4 End-of-RIB: %w", err)
	}
	if err := watchdogFirstSession(initial, address); err != nil {
		return err
	}
	frrAddress := networkHostAddress(check.Network, 3)
	if err := waitWatchdogReadyReceipt(ctx, check, frrAddress); err != nil {
		return err
	}
	// handleStateUp reports only after submitting every pool's sends.
	// creditAPIReady accepts only the captured current-session receipt.
	// Quiesce drains both forward workers and the initial-sync queue before
	// the new fence enters the same TCP stream, so it cannot overtake a leak.
	fenceOperations := []operation{
		{kind: opExec, peer: "ze", command: zeCommand("request quiesce")},
		{kind: opExec, peer: "ze", command: zeCommand("send bgp " + frrAddress + " update text origin igp nhop " + address + " nlri add " + watchdogFencePrefix)},
		{kind: opFRRRoute, argument: watchdogFencePrefix},
	}
	for index := range fenceOperations {
		if err := runOperation(ctx, check.Network, check.Lab, &fenceOperations[index]); err != nil {
			return err
		}
	}
	if err := requireWatchdogFirstSession(ctx, check, address); err != nil {
		return err
	}
	if err := checkWatchdogWithdrawValueRoutes(ctx, check); err != nil {
		return err
	}
	return requireWatchdogFirstSession(ctx, check, address)
}

// checkWatchdogWithdrawValueRoutes belongs only to the bespoke checker. A
// second generic registration would create a fallback without its causal fence.
func checkWatchdogWithdrawValueRoutes(ctx context.Context, check *interoplab.CheckContext) error {
	operations := []operation{
		{kind: opFRRSession, argument: zeLabAddress},
		{kind: opFRRRoute, argument: "10.77.0.0/24"},
		{kind: opFRRRoute, argument: "10.77.1.0/24"},
		{kind: opFRRRoute, argument: "10.77.2.0/24"},
		{kind: opFRRRouteAbsent, argument: watchdogHeldPrefix},
		{kind: opExec, peer: "ze", command: zeCommand("request bgp watchdog announce held")},
		{kind: opFRRRoute, argument: watchdogHeldPrefix},
		{kind: opExec, peer: "ze", command: zeCommand("request bgp watchdog withdraw held")},
		{kind: opFRRRouteAbsent, argument: watchdogHeldPrefix},
		{kind: opFRRRoute, argument: "10.77.0.0/24"},
		{kind: opFRRRoute, argument: "10.77.1.0/24"},
		{kind: opFRRRoute, argument: "10.77.2.0/24"},
		{kind: opFRRSession, argument: zeLabAddress},
		{kind: opFRRRoute, argument: watchdogFencePrefix},
	}
	for index := range operations {
		if err := runOperation(ctx, check.Network, check.Lab, &operations[index]); err != nil {
			return checkerFailure(ctx, check.Lab, scenarioWatchdogWithdrawValueFRR, index+1,
				fmt.Errorf("selected networks IPv4=%s IPv6=%s: %w", check.Network.IPv4, check.Network.IPv6, err))
		}
	}
	return nil
}

func waitWatchdogReadyReceipt(ctx context.Context, check *interoplab.CheckContext, peer string) error {
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     60 * time.Second,
		Interval:    100 * time.Millisecond,
		Description: "current-session watchdog completion credited for peer " + peer,
	}, func(probe context.Context) (string, error) {
		logs, err := check.Lab.Logs(probe, "ze", 200)
		if err != nil {
			return "", err
		}
		if !logs.Available {
			return "", errors.New("watchdog completion fence: Ze logs are unavailable")
		}
		return logs.Text, nil
	}, func(output string) bool {
		return watchdogReadyReceipt(output, peer)
	})
	return err
}

// watchdogReadyReceipt requires one complete record from the owning logger,
// exact peer and exact process. Fields on different lines cannot combine.
func watchdogReadyReceipt(output, peer string) bool {
	for line := range strings.SplitSeq(output, "\n") {
		level, message, attrs := slogutil.ParseLogLine(line)
		if level != slog.LevelDebug {
			continue
		}
		if message != "plugin session ready credited to this peer's barrier" {
			continue
		}
		fields := make(map[string]string, len(attrs)/2)
		for index := 0; index+1 < len(attrs); index += 2 {
			key, keyOK := attrs[index].(string)
			if !keyOK {
				continue
			}
			value, valueOK := attrs[index+1].(string)
			if valueOK {
				fields[key] = value
			}
		}
		if fields["subsystem"] != "bgp.routes" {
			continue
		}
		if fields["peer"] != peer {
			continue
		}
		if fields["process"] == "bgp-watchdog" {
			return true
		}
	}
	return false
}

func requireWatchdogFirstSession(ctx context.Context, check *interoplab.CheckContext, address string) error {
	output, err := check.Lab.Query(ctx, peerFRR, []string{cmdVtysh, "-c", "show bgp neighbor " + address + " json"}, nil)
	if err != nil {
		return err
	}
	return watchdogFirstSession(output, address)
}

// watchdogFirstSession rejects a reconnect, so a credited log line from an
// earlier session cannot fence the session whose routes the checker reads.
func watchdogFirstSession(output, address string) error {
	neighbor, err := decodeRapidReconnectNeighbor(output, address)
	if err != nil {
		return err
	}
	if neighbor.State != stateEstablished {
		return fmt.Errorf("watchdog fence requires Established, got %q", neighbor.State)
	}
	if neighbor.Established == nil {
		return errors.New("FRR omitted the established-session count for the watchdog fence")
	}
	if *neighbor.Established != 1 {
		return fmt.Errorf("watchdog fence requires the first session, FRR counts %d", *neighbor.Established)
	}
	if neighbor.Dropped == nil {
		return errors.New("FRR omitted the dropped-session count for the watchdog fence")
	}
	if *neighbor.Dropped != 0 {
		return fmt.Errorf("watchdog fence has a stale-session risk: FRR counts %d drops", *neighbor.Dropped)
	}
	return nil
}

// watchdogInitialEOR reads the received flag from its typed JSON path. The
// sent flag, a missing flag and a value with the wrong JSON type cannot pass.
func watchdogInitialEOR(output, address string) error {
	neighbor, err := decodeRapidReconnectNeighbor(output, address)
	if err != nil {
		return err
	}
	return neighbor.syncPending(1, []rapidReconnectFamily{{key: "ipv4Unicast"}})
}
