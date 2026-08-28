package fixture

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func init() {
	Register("plugin/bfd-config-load", bfdConfigLoad03)
	Register("plugin/bfd-echo-config", observe03("bfd-echo-config-test", bfdEchoConfig03))
	Register("plugin/bfd-echo-handshake-peer", bfdEchoPeer03)
	Register("plugin/bfd-echo-handshake", observePort03("bfd-echo-observe", bfdEchoHandshake03))
	Register("plugin/bfd-echo-multi-hop-reject", bfdEchoMultiHopReject03)
	Register("plugin/bfd-ipv6-dual-bind", bfdIPv6DualBind03)
	Register("plugin/bfd-json-show", observe03("bfd-json-show-test", bfdJSONShow03))
	Register("plugin/bfd-metrics", observe03("bfd-metrics-test", bfdMetrics03))
	Register("plugin/bfd-profile-show", observe03("bfd-profile-show-test", bfdProfileShow03))
	Register("plugin/bfd-session-show", observe03("bfd-session-show-test", bfdSessionShow03))
	Register("plugin/bfd-sessions-show", observe03("bfd-sessions-show-test", bfdSessionsShow03))
	Register("plugin/bfd-transport-stage2", bfdTransportStage203)
	Register("plugin/bgp-bfd-opt-in", bgpBFDOptIn03)
	Register("plugin/bgp-gtsm", observe03("gtsm-show-test", bgpGTSM03))
	Register("plugin/bgp-health-show", observe03("bgp-health-show-test", bgpHealthShow03))
	Register("plugin/bgp-monitor-dashboard", observe03("dashboard-test", bgpMonitorDashboard03))
	Register("plugin/bgp-peer-detail-show", observe03("detail-test", bgpPeerDetailShow03))
	Register("plugin/bgp-redistribute-announce", observe03("announce-test", bgpRedistributeAnnounce03))
	Register("plugin/bgp-redistribute-burst", observe03("burst-test", bgpRedistributeBurst03))
	Register("plugin/bgp-redistribute-explicit-nhop", observe03("nhop-test", bgpRedistributeExplicitNhop03))
	Register("plugin/bgp-redistribute-filtered-out", observe03("filtered-test", bgpRedistributeFilteredOut03))
	Register("plugin/bgp-redistribute-metrics", observe03("metrics-test", bgpRedistributeMetrics03))
	Register("plugin/bgp-redistribute-nexthop-self", observe03("nhself-test", bgpRedistributeNexthopSelf03))
	Register("plugin/bgp-redistribute-withdraw", observe03("withdraw-test", bgpRedistributeWithdraw03))
	Register("plugin/bgp-rs-asn4-transcode", observe03("shutdown-after-up", routeServerObserver03(2, true)))
	Register("plugin/bgp-rs-community-strip-multi-fastpath", observe03("shutdown-after-up", routeServerObserver03(2, true)))
	Register("plugin/bgp-rs-community-strip-multi", observe03("shutdown-after-up", routeServerObserver03(2, true)))
	Register("plugin/bgp-rs-control-community-withdraw-egress", observe03("rs-community-withdraw", bgpRSControlWithdraw03))
	Register("plugin/bgp-rs-fastpath-ebgp-shared", observe03("shutdown-after-up", routeServerObserver03(2, true)))
	Register("plugin/bgp-rs-fastpath-ibgp-identity", observe03("shutdown-after-up", routeServerObserver03(1, false)))
	Register("plugin/bgp-rs-fastpath", observe03("bgp-rs-fastpath", bgpRSFastpath03))
	Register("plugin/bgp-rs-mod-copy", observe03("shutdown-after-up", routeServerObserver03(1, false)))
	Register("plugin/bgp-rs-perf-pprof", observePort03("pprof-probe", bgpRSPprof03))
}

func observe03(name string, scenario ObserverScenario) Driver {
	return func(ctx context.Context, args []string) error {
		if len(args) != 0 {
			return fmt.Errorf("%s: unexpected arguments %q", name, args)
		}
		return Observe(ctx, name, sdk.Registration{}, scenario)
	}
}

func observePort03(name string, scenario func(context.Context, *sdk.Plugin, string) error) Driver {
	return func(ctx context.Context, args []string) error {
		if len(args) != 1 {
			return fmt.Errorf("%s: expected port argument, got %q", name, args)
		}
		return Observe(ctx, name, sdk.Registration{}, func(ctx context.Context, plugin *sdk.Plugin) error {
			return scenario(ctx, plugin, args[0])
		})
	}
}

func dispatchAny03(ctx context.Context, p *sdk.Plugin, command string) (string, any, error) {
	status, raw, err := p.DispatchCommand(ctx, command)
	if err != nil {
		if status == "error" || strings.HasPrefix(err.Error(), "rpc error:") {
			return "error", err.Error(), nil
		}
		return status, nil, err
	}
	if len(raw) == 0 {
		return status, nil, nil
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return status, nil, fmt.Errorf("decode %q: %w", command, err)
	}
	if text, ok := value.(string); ok && text != "" {
		var decoded any
		if json.Unmarshal([]byte(text), &decoded) == nil {
			value = decoded
		}
	}
	return status, value, nil
}

func done03(ctx context.Context, p *sdk.Plugin, command string) (any, error) {
	status, value, err := dispatchAny03(ctx, p, command)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", command, err)
	}
	if status != "done" {
		return value, fmt.Errorf("%s: status=%s data=%v", command, status, value)
	}
	return value, nil
}

func donePoll03(ctx context.Context, p *sdk.Plugin, command string) (any, error) {
	var status string
	var value any
	var lastErr error
	if !Poll(ctx, 40, 250*time.Millisecond, func() bool {
		status, value, lastErr = dispatchAny03(ctx, p, command)
		return lastErr == nil && status == "done"
	}) {
		if lastErr != nil {
			return nil, fmt.Errorf("%s: %w", command, lastErr)
		}
		return value, fmt.Errorf("%s: status=%s data=%v", command, status, value)
	}
	return value, nil
}

func number03(value any) int64 {
	switch n := value.(type) {
	case float64:
		return int64(n)
	case json.Number:
		v, _ := n.Int64()
		return v
	case int:
		return int64(n)
	case int64:
		return n
	default:
		return 0
	}
}

func peerRows03(ctx context.Context, p *sdk.Plugin, selector string) (map[string]map[string]any, error) {
	if selector == "" {
		selector = "*"
	}
	command := "show bgp peer " + selector + " detail"
	value, err := done03(ctx, p, command)
	if err != nil {
		return nil, err
	}
	root, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s: expected object, got %T", command, value)
	}
	rawPeers, ok := root["peers"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s: missing peers object", command)
	}
	rows := make(map[string]map[string]any, len(rawPeers))
	for address, raw := range rawPeers {
		if row, ok := raw.(map[string]any); ok {
			rows[address] = row
		}
	}
	return rows, nil
}

func peerCounter03(ctx context.Context, p *sdk.Plugin, peer, counter string) (int64, error) {
	rows, err := peerRows03(ctx, p, peer)
	if err != nil {
		return 0, err
	}
	if row := rows[peer]; row != nil {
		return number03(row[counter]), nil
	}
	if len(rows) == 1 {
		for _, row := range rows {
			return number03(row[counter]), nil
		}
	}
	return 0, fmt.Errorf("peer %s not present", peer)
}

func waitPeerCounter03(ctx context.Context, p *sdk.Plugin, peer, counter string, want int64, attempts int) bool {
	return Poll(ctx, attempts, 250*time.Millisecond, func() bool {
		got, err := peerCounter03(ctx, p, peer, counter)
		return err == nil && got >= want
	})
}

func waitPeersEOR03(ctx context.Context, p *sdk.Plugin, count int) bool {
	return Poll(ctx, 40, 250*time.Millisecond, func() bool {
		rows, err := peerRows03(ctx, p, "")
		if err != nil {
			return false
		}
		ready := 0
		for _, row := range rows {
			if number03(row["eor-sent"]) >= 1 {
				ready++
			}
		}
		return ready >= count
	})
}

func quiesce03(ctx context.Context, p *sdk.Plugin) error {
	_, err := done03(ctx, p, "request quiesce")
	return err
}

func realUpdates03(ctx context.Context, p *sdk.Plugin, peer string) (int64, error) {
	updates, err := peerCounter03(ctx, p, peer, "updates-sent")
	if err != nil {
		return 0, err
	}
	eor, err := peerCounter03(ctx, p, peer, "eor-sent")
	if err != nil {
		return 0, err
	}
	return updates - eor, nil
}
