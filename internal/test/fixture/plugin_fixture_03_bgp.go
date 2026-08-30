package fixture

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func bgpGTSM03(ctx context.Context, p *sdk.Plugin) error {
	if !waitPeersEOR03(ctx, p, 1) {
		return fmt.Errorf("initial-sync EOR never reached the wire")
	}
	rows, err := peerRows03(ctx, p, "peer1")
	if err != nil {
		return err
	}
	peer := rows["127.0.0.1"]
	if peer == nil {
		return fmt.Errorf("127.0.0.1 not in detail response")
	}
	out, min := number03(peer["gtsm-ttl-out"]), number03(peer["gtsm-ttl-min"])
	if out != 255 || min != 255 {
		return fmt.Errorf("GTSM out=%d min=%d, want 255/255", out, min)
	}
	fmt.Fprintf(os.Stderr, "OK: GTSM active out=%d min=%d state=%v\n", out, min, peer["state"])
	return nil
}

func bgpHealthShow03(ctx context.Context, p *sdk.Plugin) error {
	if !waitPeersEOR03(ctx, p, 1) {
		return fmt.Errorf("initial-sync EOR never reached the wire")
	}
	value, err := done03(ctx, p, "show bgp health")
	if err != nil {
		return err
	}
	data, ok := value.(map[string]any)
	if !ok {
		return fmt.Errorf("bgp-health: expected object, got %T", value)
	}
	for _, key := range []string{fieldPeers, fieldCount, "not-established"} {
		if _, exists := data[key]; !exists {
			return fmt.Errorf("bgp-health: missing key %q", key)
		}
	}
	if _, ok := data["peers"].([]any); !ok {
		return fmt.Errorf("bgp-health: peers is not a list: %T", data["peers"])
	}
	return nil
}

func bgpMonitorDashboard03(ctx context.Context, p *sdk.Plugin) error {
	if !waitPeersEOR03(ctx, p, 1) {
		return fmt.Errorf("initial-sync EOR never reached the wire")
	}
	value, err := done03(ctx, p, "show bgp")
	if err != nil {
		return err
	}
	data, ok := value.(map[string]any)
	if !ok {
		return fmt.Errorf("show bgp: expected object, got %T", value)
	}
	for _, key := range []string{fieldRouterID, fieldPeers, fieldLocalAS} {
		if _, exists := data[key]; !exists {
			return fmt.Errorf("show bgp: missing %s", key)
		}
	}
	if _, ok := data["peers"].([]any); !ok {
		return fmt.Errorf("show bgp: peers is not a list: %T", data["peers"])
	}
	return nil
}

func bgpPeerDetailShow03(ctx context.Context, p *sdk.Plugin) error {
	if !waitPeersEOR03(ctx, p, 1) {
		return fmt.Errorf("initial-sync EOR never reached the wire")
	}
	rows, err := peerRows03(ctx, p, "127.0.0.1")
	if err != nil {
		return err
	}
	peer := rows["127.0.0.1"]
	if peer == nil {
		return fmt.Errorf("127.0.0.1 not in peer detail")
	}
	if kind, ok := peer["peer-type"].(string); !ok || (kind != "internal" && kind != "external") {
		return fmt.Errorf("bad peer-type: %v", peer["peer-type"])
	}
	messages, ok := peer["messages"].(map[string]any)
	if !ok {
		return fmt.Errorf("missing messages block")
	}
	for _, direction := range []string{directionReceived, directionSent} {
		row, ok := messages[direction].(map[string]any)
		if !ok {
			return fmt.Errorf("missing messages.%s", direction)
		}
		for _, field := range []string{"opens", "updates", "notifications", "keepalives", "route-refresh", "total"} {
			if _, exists := row[field]; !exists {
				return fmt.Errorf("missing messages.%s.%s", direction, field)
			}
		}
	}
	caps, ok := peer["capabilities"].(map[string]any)
	if !ok {
		return fmt.Errorf("missing capabilities block")
	}
	for _, field := range []string{"negotiation-complete", "asn4", "extended-message", "route-refresh", "enhanced-route-refresh"} {
		if _, exists := caps[field]; !exists {
			return fmt.Errorf("missing capabilities.%s", field)
		}
	}
	for _, field := range []string{"connections-established", columnConnectionsDropped, "flap-count", "connect-retry-counter", "updates-received", "updates-sent", "keepalives-received", "keepalives-sent", "eor-received", "eor-sent"} {
		if _, exists := peer[field]; !exists {
			return fmt.Errorf("missing peer detail field %s", field)
		}
	}
	return nil
}

func bgpRedistributeAnnounce03(ctx context.Context, p *sdk.Plugin) error {
	if !waitPeersEOR03(ctx, p, 1) {
		return fmt.Errorf("initial-sync EOR never reached the wire")
	}
	base, err := peerCounter03(ctx, p, "127.0.0.1", "updates-sent")
	if err != nil {
		return err
	}
	if _, err := done03(ctx, p, "request fakeredist emit add ipv4/unicast 10.0.0.1/32"); err != nil {
		return err
	}
	if !waitPeerCounter03(ctx, p, "127.0.0.1", "updates-sent", base+1, 40) {
		return fmt.Errorf("redistribute UPDATE never counted as sent")
	}
	return nil
}

func bgpRedistributeBurst03(ctx context.Context, p *sdk.Plugin) error {
	if !waitPeersEOR03(ctx, p, 1) {
		return fmt.Errorf("initial-sync EOR never reached the wire")
	}
	if _, err := done03(ctx, p, "request fakeredist emit-burst 500 add ipv4/unicast 10.0.0.0/32"); err != nil {
		return err
	}
	return quiesce03(ctx, p)
}

func bgpRedistributeExplicitNhop03(ctx context.Context, p *sdk.Plugin) error {
	if !waitPeersEOR03(ctx, p, 1) {
		return fmt.Errorf("initial-sync EOR never reached the wire")
	}
	if _, err := done03(ctx, p, "request fakeredist emit add ipv4/unicast 10.0.0.1/32 192.0.2.1"); err != nil {
		return err
	}
	return quiesce03(ctx, p)
}

func bgpRedistributeFilteredOut03(ctx context.Context, p *sdk.Plugin) error {
	if !waitPeersEOR03(ctx, p, 1) {
		return fmt.Errorf("initial-sync EOR never reached the wire")
	}
	if _, err := done03(ctx, p, "request fakeredist emit add ipv6/unicast 2001:db8::1/128"); err != nil {
		return err
	}
	if err := quiesce03(ctx, p); err != nil {
		return err
	}
	delivered, err := realUpdates03(ctx, p, "127.0.0.1")
	if err != nil {
		return err
	}
	if delivered != 0 {
		return fmt.Errorf("peer received %d UPDATEs beyond its End-of-RIB, want zero", delivered)
	}
	return nil
}

func bgpRedistributeMetrics03(ctx context.Context, p *sdk.Plugin) error {
	if !waitPeersEOR03(ctx, p, 1) {
		return fmt.Errorf("initial-sync EOR never reached the wire")
	}
	sequence := []struct {
		command string
		sends   bool
	}{
		{"request fakeredist emit add ipv4/unicast 10.0.0.1/32", true},
		{"request fakeredist emit add ipv4/unicast 10.0.0.2/32", true},
		{"request fakeredist emit add ipv4/unicast 10.0.0.3/32", true},
		{"request fakeredist emit remove ipv4/unicast 10.0.0.1/32", true},
		{"request fakeredist emit add ipv6/unicast 2001:db8::1/128", false},
	}
	for _, step := range sequence {
		base, err := peerCounter03(ctx, p, "127.0.0.1", "updates-sent")
		if err != nil {
			return err
		}
		if _, err := done03(ctx, p, step.command); err != nil {
			return err
		}
		if step.sends && !waitPeerCounter03(ctx, p, "127.0.0.1", "updates-sent", base+1, 16) {
			return fmt.Errorf("%s: its UPDATE was never counted as sent", step.command)
		}
	}
	if err := quiesce03(ctx, p); err != nil {
		return err
	}
	delivered, err := realUpdates03(ctx, p, "127.0.0.1")
	if err != nil {
		return err
	}
	if delivered != 4 {
		return fmt.Errorf("%d UPDATEs beyond the End-of-RIB, want exactly 4", delivered)
	}
	return nil
}

func bgpRedistributeNexthopSelf03(ctx context.Context, p *sdk.Plugin) error {
	if !waitPeersEOR03(ctx, p, 2) {
		return fmt.Errorf("initial-sync EOR never reached both peers")
	}
	if _, err := done03(ctx, p, "request fakeredist emit add ipv4/unicast 10.0.0.1/32"); err != nil {
		return err
	}
	return quiesce03(ctx, p)
}

func bgpRedistributeWithdraw03(ctx context.Context, p *sdk.Plugin) error {
	if !waitPeersEOR03(ctx, p, 1) {
		return fmt.Errorf("initial-sync EOR never reached the wire")
	}
	for _, command := range []string{
		"request fakeredist emit add ipv4/unicast 10.0.0.1/32",
		"request fakeredist emit remove ipv4/unicast 10.0.0.1/32",
	} {
		base, err := peerCounter03(ctx, p, "127.0.0.1", "updates-sent")
		if err != nil {
			return err
		}
		if _, err := done03(ctx, p, command); err != nil {
			return err
		}
		if !waitPeerCounter03(ctx, p, "127.0.0.1", "updates-sent", base+1, 24) {
			return fmt.Errorf("%s: its UPDATE was never counted as sent", command)
		}
	}
	return quiesce03(ctx, p)
}

const bgpBFDOptInConfig03 = `environment {
}

bfd {
	enabled true;
	profile fast {
		detect-multiplier 3
		desired-min-tx-us 50000
		required-min-rx-us 50000
	}
}

bgp {
	peer peer1 {
		connection {
			local {
				ip 127.0.0.1
				accept false
			}
			remote {
				ip 127.0.0.254
			}
			bfd {
				enabled true
				mode single-hop
				profile fast
			}
		}
		session {
			asn {
				local 65001
				remote 65002
			}
			router-id 10.0.0.1
			family {
				ipv4/unicast { prefix { maximum 10000; } }
			}
		}
	}
}
`

func bgpBFDOptIn03(ctx context.Context, _ []string) error {
	return runZeUntilLogsRejecting03(ctx, bgpBFDOptInConfig03,
		[]string{logBFDStarting, logBFDConfigured, logBFDRunning},
		[]string{"unknown top-level keyword", "unknown key bfd", "invalid bfd"},
		12*time.Second, map[string]string{envLogBFD: logLevelDebug, envLogBGP: logLevelInfo})
}
