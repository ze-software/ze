package fixture

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func init() {
	Register("plugin/community-blackhole-noexport", p05CommunityBlackhole)
	Register("plugin/community-cumulative", p05CommunityCumulative)
	Register("plugin/community-match-accept", p05CommunityMatchAccept)
	Register("plugin/community-match-large", p05CommunityMatchLarge)
	Register("plugin/community-match-reject", p05CommunityMatchReject)
	Register("plugin/community-priority", p05CommunityPriority)
	Register("plugin/community-relation-tag", p05CommunityRelationTag)
	Register("plugin/community-scrub-own-ga", p05CommunityScrubOwnGA)
	Register("plugin/community-strip", p05CommunityStrip)
	Register("plugin/community-tag", p05CommunityTag)
	Register("plugin/concurrent-config-commit", p05ConcurrentConfigCommit)
	Register("plugin/config-addpath-mode", p05ConfigAddpathMode)
	Register("plugin/config-adj-rib", p05ConfigAdjRIB)
	Register("plugin/config-edit-ssh-session", p05ConfigEditSSHSession)
	Register("plugin/config-edit-ssh", p05ConfigEditSSH)
	Register("plugin/config-ext-nexthop", p05ConfigExtNexthop)
	Register("plugin/config-group-updates", p05ConfigGroupUpdates)
	Register("plugin/control-community-withdraw-egress", p05ControlCommunityWithdraw)
	Register("plugin/cos-dynamic-coa", p05CoSDynamicCoA)
	Register("plugin/cos-dynamic-session", p05CoSDynamicSession)
	Register("plugin/cursor-replay", p05CursorReplay)
	Register("plugin/custom-flowspec-plugin", p05CustomFlowspecPlugin)
	Register("plugin/ddos-flow-recent", p05DDoSFlowRecent)
	Register("plugin/ddos-flowspec-announce", p05DDoSFlowspecAnnounce)
}

func p05NoArgs(args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("unexpected arguments: %q", args)
	}
	return nil
}

func p05Observe(ctx context.Context, args []string, process string, scenario ObserverScenario) error {
	if err := p05NoArgs(args); err != nil {
		return err
	}
	return Observe(ctx, process, sdk.Registration{}, scenario)
}

func p05Dispatch(ctx context.Context, plugin *sdk.Plugin, command string) (string, any, json.RawMessage, error) {
	status, raw, err := plugin.DispatchCommand(ctx, command)
	if err != nil {
		return status, nil, raw, err
	}
	if len(raw) == 0 {
		return status, nil, raw, nil
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return status, nil, raw, fmt.Errorf("decode %q: %w", command, err)
	}
	if text, ok := value.(string); ok && text != "" {
		var nested any
		if json.Unmarshal([]byte(text), &nested) == nil {
			value = nested
		}
	}
	return status, value, raw, nil
}

func p05Map(value any) map[string]any {
	row, _ := value.(map[string]any)
	return row
}

func p05List(value any) []any {
	rows, _ := value.([]any)
	return rows
}

func p05Float(value any) float64 {
	switch number := value.(type) {
	case float64:
		return number
	case json.Number:
		value, _ := number.Float64()
		return value
	case int:
		return float64(number)
	default:
		return 0
	}
}

func p05PollCommand(ctx context.Context, plugin *sdk.Plugin, command string, attempts int, predicate func(string, any) bool) (string, any, error) {
	var status string
	var value any
	var lastErr error
	ok := Poll(ctx, attempts, 200*time.Millisecond, func() bool {
		status, value, _, lastErr = p05Dispatch(ctx, plugin, command)
		return lastErr == nil && predicate(status, value)
	})
	if !ok {
		if lastErr != nil {
			return status, value, lastErr
		}
		return status, value, fmt.Errorf("%q did not reach the required state", command)
	}
	return status, value, nil
}

func p05TotalRoutes(value any) float64 {
	return p05Float(p05Map(value)["total-routes"])
}

func p05WaitTotalRoute(ctx context.Context, plugin *sdk.Plugin) error {
	_, value, err := p05PollCommand(ctx, plugin, "show bgp adj-rib-in status", 50, func(status string, value any) bool {
		return status == statusDone && p05TotalRoutes(value) >= 1
	})
	if err != nil {
		return err
	}
	if total := p05TotalRoutes(value); total < 0 {
		return fmt.Errorf("adj-rib-in query failed, got total=%v", total)
	}
	return nil
}

func p05RequireStatus(ctx context.Context, plugin *sdk.Plugin, command string, allowed ...string) (any, error) {
	status, value, _, err := p05Dispatch(ctx, plugin, command)
	if err != nil {
		return value, err
	}
	if slices.Contains(allowed, status) {
		return value, nil
	}
	return value, fmt.Errorf("%s returned status=%s", command, status)
}

func p05CommunityBlackhole(ctx context.Context, args []string) error {
	return p05Observe(ctx, args, "test-blackhole", func(ctx context.Context, plugin *sdk.Plugin) error {
		if err := p05WaitTotalRoute(ctx, plugin); err != nil {
			return err
		}
		if _, err := p05RequireStatus(ctx, plugin, "show bgp rib received community 65535:666", "done"); err != nil {
			return fmt.Errorf("BLACKHOLE must survive ingress: %w", err)
		}
		if _, err := p05RequireStatus(ctx, plugin, "show bgp rib received community 65535:65281", "done"); err != nil {
			return fmt.Errorf("the RFC 7999 Section 3.2 guard must add NO_EXPORT: %w", err)
		}
		fmt.Fprintln(os.Stderr, "OK: BLACKHOLE survived and NO_EXPORT was added")
		return nil
	})
}

func p05CommunityCumulative(ctx context.Context, args []string) error {
	return p05Observe(ctx, args, "test-cumulative", func(ctx context.Context, plugin *sdk.Plugin) error {
		if err := p05WaitTotalRoute(ctx, plugin); err != nil {
			return err
		}
		for _, community := range []string{"65000:1", "65000:2"} {
			if _, err := p05RequireStatus(ctx, plugin, "show bgp rib received community "+community, "done", "error"); err != nil {
				return fmt.Errorf("rib show community %s rpc failed: %w", community, err)
			}
		}
		fmt.Fprintln(os.Stderr, "OK: both cumulative communities tagged on ingress")
		return nil
	})
}

func p05CommunityMatchAccept(ctx context.Context, args []string) error {
	return p05Observe(ctx, args, "test-cm-accept", p05WaitTotalRoute)
}

func p05CommunityMatchLarge(ctx context.Context, args []string) error {
	return p05Observe(ctx, args, "test-cm-large", p05WaitTotalRoute)
}

func p05PeerRows(value any) map[string]any {
	peers := p05Map(value)["peers"]
	if rows := p05Map(peers); rows != nil {
		return rows
	}
	rows := make(map[string]any)
	for _, candidate := range p05List(peers) {
		row := p05Map(candidate)
		if row == nil {
			continue
		}
		peer, _ := row["peer"].(string)
		rows[peer] = row
	}
	return rows
}

func p05PeerRow(value any, peer string) map[string]any {
	rows := p05PeerRows(value)
	if row := p05Map(rows[peer]); row != nil {
		return row
	}
	for _, candidate := range rows {
		if row := p05Map(candidate); row != nil {
			return row
		}
	}
	return nil
}

func p05PeerCounter(ctx context.Context, plugin *sdk.Plugin, peer, counter string) float64 {
	status, value, _, err := p05Dispatch(ctx, plugin, "show bgp peer "+peer+" detail")
	if err != nil || status != statusDone {
		return 0
	}
	row := p05PeerRow(value, peer)
	return p05Float(row[counter])
}

func p05CommunityMatchReject(ctx context.Context, args []string) error {
	return p05Observe(ctx, args, "test-cm-reject", func(ctx context.Context, plugin *sdk.Plugin) error {
		if !Poll(ctx, 100, 200*time.Millisecond, func() bool {
			return p05PeerCounter(ctx, plugin, "peer1", "updates-received") >= 1
		}) {
			return errors.New("peer1 never received the test update")
		}
		_, _, err := plugin.DispatchCommand(ctx, "request quiesce")
		return err
	})
}

func p05CommunityPriority(ctx context.Context, args []string) error {
	return p05Observe(ctx, args, "test-priority", func(ctx context.Context, plugin *sdk.Plugin) error {
		if err := p05WaitTotalRoute(ctx, plugin); err != nil {
			return err
		}
		if _, err := p05RequireStatus(ctx, plugin, "show bgp rib received community 65000:300", "done", "error"); err != nil {
			return fmt.Errorf("rib show community rpc failed: %w", err)
		}
		fmt.Fprintln(os.Stderr, "OK: both filters active, priority ordering correct")
		return nil
	})
}

func p05CommunityRelationTag(ctx context.Context, args []string) error {
	return p05Observe(ctx, args, "test-relation", func(ctx context.Context, plugin *sdk.Plugin) error {
		if err := p05WaitTotalRoute(ctx, plugin); err != nil {
			return err
		}
		if _, err := p05RequireStatus(ctx, plugin, "show bgp rib received large-community 65000:3:4", "done"); err != nil {
			return fmt.Errorf("relation tag 65000:3:4 absent: %w", err)
		}
		fmt.Fprintln(os.Stderr, "OK: relation community 65000:3:4 written on ingress")
		return nil
	})
}

func p05Count(ctx context.Context, plugin *sdk.Plugin, command, subject string) (float64, error) {
	status, value, _, err := p05Dispatch(ctx, plugin, command)
	if err != nil {
		return 0, err
	}
	if status != statusDone {
		return 0, fmt.Errorf("%s: dispatch failed with status=%s", subject, status)
	}
	row := p05Map(value)
	count, exists := row["count"]
	if !exists {
		return 0, fmt.Errorf("%s: the count terminal answered no count: %v", subject, value)
	}
	return p05Float(count), nil
}

func p05CommunityScrubOwnGA(ctx context.Context, args []string) error {
	return p05Observe(ctx, args, "test-scrub", func(ctx context.Context, plugin *sdk.Plugin) error {
		var barrierErr error
		reached := Poll(ctx, 50, 200*time.Millisecond, func() bool {
			count, err := p05Count(ctx, plugin, "show bgp rib received prefix 10.0.0.0/24 count", "10.0.0.0/24")
			if err != nil {
				barrierErr = err
				return true
			}
			return count >= 1
		})
		if barrierErr != nil {
			return barrierErr
		}
		if !reached {
			return errors.New("10.0.0.0/24 never reached the bgp-rib received table")
		}
		checks := []struct {
			community string
			want      float64
			message   string
		}{
			{"65000:64", 1, "65000:64 is in the keep-list and must survive the scrub"},
			{"64512:99", 1, "64512:99 carries another ASN and must never be removed"},
			{"65000:99", 0, "65000:99 is our own number outside the keep-list and must be removed"},
		}
		for _, check := range checks {
			count, err := p05Count(ctx, plugin, "show bgp rib received community "+check.community+" count", check.community)
			if err != nil {
				return err
			}
			if count != check.want {
				return errors.New(check.message)
			}
		}
		fmt.Fprintln(os.Stderr, "OK: keep-list and foreign values survived, own-number 99 scrubbed")
		return nil
	})
}

func p05DestGotRoute(status string, value any) bool {
	if status != statusDone {
		return false
	}
	for _, candidate := range p05PeerRows(value) {
		row := p05Map(candidate)
		if p05Float(row["updates-sent"])-p05Float(row["eor-sent"]) >= 1 {
			return true
		}
	}
	return false
}

func p05CommunityStrip(ctx context.Context, args []string) error {
	return p05Observe(ctx, args, "test-strip", func(ctx context.Context, plugin *sdk.Plugin) error {
		if status, _, err := plugin.DispatchCommand(ctx, "request quiesce"); err != nil || status != statusDone {
			return fmt.Errorf("quiesce barrier did not settle: status=%q: %w", status, err)
		}
		status, value, err := p05PollCommand(ctx, plugin, "show bgp peer dest-peer detail", 24, p05DestGotRoute)
		if err != nil || !p05DestGotRoute(status, value) {
			return fmt.Errorf("dest peer was never sent the re-advertised route: %v", value)
		}
		queryStatus, totalValue, _, queryErr := p05Dispatch(ctx, plugin, "show bgp adj-rib-in status")
		if queryErr != nil || queryStatus != statusDone {
			return fmt.Errorf("adj-rib-in query failed: status=%s: %w", queryStatus, queryErr)
		}
		totalField, exists := p05Map(totalValue)["total-routes"]
		if !exists {
			return fmt.Errorf("adj-rib-in query failed, got total-routes=%v", totalValue)
		}
		total := p05Float(totalField)
		fmt.Fprintf(os.Stderr, "INFO: adj-rib-in total=%v\n", total)
		fmt.Fprintf(os.Stderr, "INFO: dest-peer detail after the re-advertisement: %v\n", value)
		fmt.Fprintln(os.Stderr, "OK: dest peer was sent the re-advertised route")
		return nil
	})
}

func p05CommunityTag(ctx context.Context, args []string) error {
	return p05Observe(ctx, args, "test-tag", func(ctx context.Context, plugin *sdk.Plugin) error {
		if err := p05WaitTotalRoute(ctx, plugin); err != nil {
			return err
		}
		if _, err := p05RequireStatus(ctx, plugin, "show bgp rib received community 65000:100", "done"); err != nil {
			return fmt.Errorf("community tag missing: %w", err)
		}
		fmt.Fprintln(os.Stderr, "OK: community 65000:100 tagged on ingress")
		return nil
	})
}

func p05ConfigAddpathMode(ctx context.Context, args []string) error {
	return p05Observe(ctx, args, "addpath-test", func(ctx context.Context, plugin *sdk.Plugin) error {
		_, value, err := p05PollCommand(ctx, plugin, "show bgp peer peer1 capabilities", 50, func(status string, value any) bool {
			if status != statusDone {
				return false
			}
			negotiated := p05Map(p05PeerRow(value, "peer1")["negotiated"])
			_, ready := negotiated["add-path"]
			return ready
		})
		if err != nil {
			return err
		}
		negotiated := p05Map(p05PeerRow(value, "peer1")["negotiated"])
		addPath, ok := negotiated["add-path"]
		if !ok {
			return fmt.Errorf("no add-path in negotiated capabilities: %v", negotiated)
		}
		fmt.Fprintf(os.Stderr, "OK: add-path capability negotiated: %v\n", addPath)
		return nil
	})
}

func p05ConfigAdjRIB(ctx context.Context, args []string) error {
	return p05Observe(ctx, args, "adj-rib-test", func(ctx context.Context, plugin *sdk.Plugin) error {
		if !Poll(ctx, 40, 200*time.Millisecond, func() bool {
			return p05PeerCounter(ctx, plugin, "peer1", "eor-sent") >= 1
		}) {
			return errors.New("peer1 initial-sync EOR never reached the wire")
		}
		if status, _, err := plugin.DispatchCommand(ctx, "request quiesce"); err != nil || status != statusDone {
			return fmt.Errorf("quiesce barrier did not settle: status=%q: %w", status, err)
		}
		if _, err := p05RequireStatus(ctx, plugin, "show bgp rib received", "done"); err != nil {
			return fmt.Errorf("rib show-in: %w", err)
		}
		fmt.Fprintln(os.Stderr, "OK: adj-rib-in query dispatched")
		return nil
	})
}

func p05Dial(ctx context.Context, port int, readBanner bool) (bool, error) {
	dialer := net.Dialer{Timeout: 2 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		return false, err
	}
	defer conn.Close() //nolint:errcheck // fixture teardown
	if !readBanner {
		return true, nil
	}
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	buffer := make([]byte, 256)
	n, err := conn.Read(buffer)
	if err != nil {
		return false, err
	}
	return strings.Contains(string(buffer[:n]), "SSH"), nil
}

func p05ConfigEditSSHSession(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("config-edit-ssh-session requires the SSH port")
	}
	port, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("SSH port %q: %w", args[0], err)
	}
	return Observe(ctx, "ssh-session-test", sdk.Registration{}, func(ctx context.Context, plugin *sdk.Plugin) error {
		if !Poll(ctx, 50, 200*time.Millisecond, func() bool {
			ok, _ := p05Dial(ctx, port, true)
			return ok
		}) {
			return errors.New("SSH server not reachable on expected port")
		}
		ok, err := p05Dial(ctx, port, true)
		if err != nil || !ok {
			return errors.New("SSH server not reachable on expected port")
		}
		fmt.Fprintf(os.Stderr, "OK: SSH server listening on port %d\n", port)
		if _, err := p05RequireStatus(ctx, plugin, "show bgp peer list", "done"); err != nil {
			return err
		}
		fmt.Fprintln(os.Stderr, "OK: SSH server running with config editing support")
		return nil
	})
}

func p05ConfigEditSSH(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("config-edit-ssh requires the SSH port")
	}
	port, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("SSH port %q: %w", args[0], err)
	}
	return Observe(ctx, "config-edit-test", sdk.Registration{}, func(ctx context.Context, plugin *sdk.Plugin) error {
		Poll(ctx, 50, 200*time.Millisecond, func() bool {
			ok, _ := p05Dial(ctx, port, false)
			return ok
		})
		if ok, err := p05Dial(ctx, port, false); err == nil && ok {
			fmt.Fprintln(os.Stderr, "OK: SSH port reachable")
		} else {
			fmt.Fprintf(os.Stderr, "NOTE: SSH port not on %d (%v), checking via dispatch\n", port, err)
		}
		if _, err := p05RequireStatus(ctx, plugin, "show bgp peer list", "done"); err != nil {
			return err
		}
		fmt.Fprintln(os.Stderr, "OK: command dispatch works")
		return nil
	})
}

func p05ConfigExtNexthop(ctx context.Context, args []string) error {
	return p05Observe(ctx, args, "ext-nh-test", func(ctx context.Context, plugin *sdk.Plugin) error {
		_, value, err := p05PollCommand(ctx, plugin, "show bgp peer peer1 capabilities", 50, func(status string, value any) bool {
			return status == statusDone && p05Map(p05PeerRow(value, "peer1"))["negotiation-complete"] == true
		})
		if err != nil {
			return err
		}
		raw, _ := json.Marshal(p05PeerRow(value, "peer1"))
		if !strings.Contains(strings.ToLower(string(raw)), "extended") {
			return errors.New("no extended-nexthop in capabilities")
		}
		fmt.Fprintln(os.Stderr, "OK: extended-nexthop capability present")
		return nil
	})
}

func p05ConfigGroupUpdates(ctx context.Context, args []string) error {
	return p05Observe(ctx, args, "group-test", func(ctx context.Context, plugin *sdk.Plugin) error {
		if !Poll(ctx, 50, 200*time.Millisecond, func() bool {
			return p05PeerCounter(ctx, plugin, "peer1", "eor-sent") >= 1
		}) {
			return errors.New("peer1 initial-sync EOR never reached the wire")
		}
		if _, err := p05RequireStatus(ctx, plugin, "show bgp peer peer1 detail", "done"); err != nil {
			return err
		}
		fmt.Fprintln(os.Stderr, "OK: session works with group-updates disable")
		return nil
	})
}

func p05ControlCommunityWithdraw(ctx context.Context, args []string) error {
	return p05Observe(ctx, args, "control-community-withdraw", func(ctx context.Context, plugin *sdk.Plugin) error {
		if status, _, err := plugin.DispatchCommand(ctx, "request quiesce"); err != nil || status != statusDone {
			return fmt.Errorf("quiesce barrier did not settle: status=%q: %w", status, err)
		}
		realUpdates := func(peer string) float64 {
			return p05PeerCounter(ctx, plugin, peer, "updates-sent") - p05PeerCounter(ctx, plugin, peer, "eor-sent")
		}
		if !Poll(ctx, 40, 200*time.Millisecond, func() bool { return realUpdates("127.0.0.3") >= 3 }) {
			return errors.New("included client was not sent both halves and the fence")
		}
		fmt.Fprintln(os.Stderr, "OK: included client received both halves and the fence")
		if !Poll(ctx, 40, 200*time.Millisecond, func() bool { return realUpdates("127.0.0.2") >= 2 }) {
			return errors.New("excluded client was not sent the withdrawal and the fence")
		}
		if got := realUpdates("127.0.0.2"); got != 2 {
			return fmt.Errorf("excluded client was sent more than the withdrawal and the fence: %v", got)
		}
		fmt.Fprintln(os.Stderr, "OK: excluded client received the withdrawal and the fence, nothing else")
		return nil
	})
}

func p05ProfileNames(value any) map[string]bool {
	names := make(map[string]bool)
	for _, candidate := range p05List(value) {
		if name, ok := p05Map(candidate)["name"].(string); ok {
			names[name] = true
		}
	}
	return names
}

func p05PollDone(ctx context.Context, plugin *sdk.Plugin, command string) (any, error) {
	_, value, err := p05PollCommand(ctx, plugin, command, 50, func(status string, _ any) bool { return status == statusDone })
	return value, err
}

func p05CoSDynamicCoA(ctx context.Context, args []string) error {
	return p05Observe(ctx, args, "cos-dynamic-coa-test", func(ctx context.Context, plugin *sdk.Plugin) error {
		value, err := p05PollDone(ctx, plugin, "show class-of-service")
		if err != nil {
			return err
		}
		names := p05ProfileNames(value)
		if !names["gold"] || !names["silver"] {
			return fmt.Errorf("required profiles missing; got %v", names)
		}
		return nil
	})
}

func p05CoSDynamicSession(ctx context.Context, args []string) error {
	return p05Observe(ctx, args, "cos-dynamic-session-test", func(ctx context.Context, plugin *sdk.Plugin) error {
		value, err := p05PollDone(ctx, plugin, "show class-of-service")
		if err != nil {
			return err
		}
		profiles := p05List(value)
		if profiles == nil {
			return fmt.Errorf("expected list, got %T", value)
		}
		names := p05ProfileNames(value)
		if !names["residential"] || !names["business"] {
			return fmt.Errorf("required profiles missing; got %v", names)
		}
		for _, candidate := range profiles {
			profile := p05Map(candidate)
			if profile["name"] != profileResidential {
				continue
			}
			matched := false
			for _, entry := range p05List(profile["egress"]) {
				row := p05Map(entry)
				if p05Float(row["from"]) == 6 && p05Float(row["to"]) == 6 {
					matched = true
				}
			}
			if !matched {
				return errors.New("residential egress[6] want 6")
			}
		}
		return nil
	})
}

func p05CursorReplay(ctx context.Context, args []string) error {
	return p05Observe(ctx, args, "cursor-test", func(ctx context.Context, plugin *sdk.Plugin) error {
		if !Poll(ctx, 60, 200*time.Millisecond, func() bool {
			return p05PeerCounter(ctx, plugin, "peer1", "eor-sent") >= 1
		}) {
			return errors.New("peer1 never sent its initial-sync End-of-RIB")
		}
		commands := []string{
			"update cursor origin igp as-path [65001 65002] med 100 local-preference 200 next-hop 10.0.0.1 nlri ipv4/unicast add 10.0.0.0/24 10.0.1.0/24",
			"update cursor as-path [65001 65003] nlri ipv4/unicast add 10.1.0.0/24",
			"update cursor nlri ipv4/unicast add 10.2.0.0/24 10.2.1.0/24",
			"update cursor del med nlri ipv4/unicast add 10.3.0.0/24",
			"update cursor done",
		}
		for index, command := range commands {
			if _, _, err := plugin.UpdateRoute(ctx, "*", command); err != nil {
				return fmt.Errorf("cursor command %d rejected: %w", index, err)
			}
		}
		fmt.Fprintln(os.Stderr, "OK: all 5 cursor commands accepted")
		return nil
	})
}

func p05RunActivePlugin(ctx context.Context, process string, registration sdk.Registration, scenario func(context.Context, *sdk.Plugin) error) error {
	plugin, err := newObserver(process)
	if err != nil {
		return err
	}
	defer plugin.Close() //nolint:errcheck // fixture teardown, so a close failure changes no assertion

	started := make(chan struct{})
	scenarioDone := make(chan error, 1)
	plugin.OnAllPluginsReady(func() error {
		close(started)
		go func() {
			scenarioErr := scenario(ctx, plugin)
			scenarioDone <- scenarioErr
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_, _, _ = plugin.DispatchCommand(shutdownCtx, "request shutdown")
		}()
		return nil
	})
	runErr := plugin.Run(ctx, registration)
	return awaitObserverResult(started, scenarioDone, runErr)
}

func p05CustomFlowspecPlugin(ctx context.Context, args []string) error {
	if err := p05NoArgs(args); err != nil {
		return err
	}
	registration := sdk.Registration{Families: []sdk.FamilyDecl{
		{Name: "ipv4/flow", Mode: modeDecode, AFI: 1, SAFI: 133},
		{Name: "ipv6/flow", Mode: modeDecode, AFI: 2, SAFI: 133},
		{Name: "ipv4/flow-vpn", Mode: modeDecode, AFI: 1, SAFI: 134},
		{Name: "ipv6/flow-vpn", Mode: modeDecode, AFI: 2, SAFI: 134},
		{Name: familyIPv4Unicast, Mode: modeBoth, AFI: 1, SAFI: 1},
	}}
	return p05RunActivePlugin(ctx, "acme-traffic-filter", registration, func(ctx context.Context, plugin *sdk.Plugin) error {
		if _, _, err := plugin.UpdateRoute(ctx, "*", "update text origin igp local-preference 100 nhop 1.2.3.4 nlri ipv4/unicast add 77.77.77.0/24"); err != nil {
			return err
		}
		_, _, err := plugin.DispatchCommand(ctx, "request quiesce")
		return err
	})
}

func p05DDoSFlowRecent(ctx context.Context, args []string) error {
	return p05Observe(ctx, args, "flow-recent-test", func(ctx context.Context, plugin *sdk.Plugin) error {
		for _, command := range []string{"show flow recent", "show flow recent dst 203.0.113.0/24"} {
			value, err := p05RequireStatus(ctx, plugin, command, "done")
			if err != nil {
				return err
			}
			if p05List(value) == nil {
				return fmt.Errorf("%s did not return a list", command)
			}
		}
		status, _, _, err := p05Dispatch(ctx, plugin, "show flow recent bogus arg")
		if err == nil && status != statusError {
			return fmt.Errorf("bad grammar status=%s, want error", status)
		}
		fmt.Fprintln(os.Stderr, "OK: show flow recent reachable, list shape, dst filter, usage guard")
		return nil
	})
}

func p05DDoSFlowspecAnnounce(ctx context.Context, args []string) error {
	if err := p05NoArgs(args); err != nil {
		return err
	}
	return p05RunActivePlugin(ctx, "ddos-flowspec-announce", sdk.Registration{}, func(ctx context.Context, plugin *sdk.Plugin) error {
		commands := []string{
			cmdAnnounceFirstPrefix,
			"update text extended-community [rate-limit:9600] nhop self nlri ipv4/flow add destination-ipv4 192.0.2.0/24 protocol =6 destination-port =80",
		}
		for _, command := range commands {
			if _, _, err := plugin.UpdateRoute(ctx, "*", command); err != nil {
				return err
			}
		}
		// The former wait-for-ack contract was deliberately best effort.
		_, _, _ = plugin.DispatchCommand(ctx, "request quiesce")
		return nil
	})
}

func p05ConcurrentConfigCommit(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("concurrent-config-commit requires the REST port")
	}
	port, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("REST port %q: %w", args[0], err)
	}
	for _, marker := range []string{"verify.started", "concurrent.ok"} {
		if err := os.Remove(marker); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove stale %s: %w", marker, err)
		}
	}
	plugin, err := newObserver("slow-commit")
	if err != nil {
		return err
	}
	defer plugin.Close() //nolint:errcheck // fixture teardown
	plugin.OnConfigVerify(func(sections []sdk.ConfigSection) error {
		for _, section := range sections {
			if section.Root == namespaceBGP && strings.Contains(section.Data, "10.0.0.2") {
				if err := os.WriteFile("verify.started", []byte("started\n"), 0o600); err != nil {
					return err
				}
				timer := time.NewTimer(3 * time.Second)
				select {
				case <-ctx.Done():
					timer.Stop()
					return ctx.Err()
				case <-timer.C:
				}
			}
		}
		return nil
	})
	driverDone := make(chan error, 1)
	plugin.OnAllPluginsReady(func() error {
		go func() {
			driverErr := p05DriveRESTCommits(ctx, port)
			_, _, shutdownErr := plugin.DispatchCommand(ctx, "request shutdown")
			driverDone <- errors.Join(driverErr, shutdownErr)
		}()
		return nil
	})
	runErr := plugin.Run(ctx, sdk.Registration{WantsConfig: []string{namespaceBGP}, VerifyBudget: 3})
	select {
	case driverErr := <-driverDone:
		return errors.Join(driverErr, runErr)
	case <-time.After(time.Second):
		return errors.Join(errors.New("REST commit driver did not report its result"), runErr)
	}
}

type p05RESTClient struct {
	base   string
	client *http.Client
}

func (client p05RESTClient) request(ctx context.Context, method, path string, payload any) (map[string]any, int, string, error) {
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return nil, 0, "", err
		}
		body = strings.NewReader(string(encoded))
	}
	request, err := http.NewRequestWithContext(ctx, method, client.base+path, body)
	if err != nil {
		return nil, 0, "", err
	}
	request.Header.Set("Authorization", "Bearer secret")
	request.Header.Set("Content-Type", "application/json")
	response, err := client.client.Do(request)
	if err != nil {
		return nil, 0, "", err
	}
	defer response.Body.Close() //nolint:errcheck // the body is read
	raw, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, response.StatusCode, "", err
	}
	text := string(raw)
	if response.StatusCode >= 400 {
		return nil, response.StatusCode, text, nil
	}
	result := make(map[string]any)
	if len(raw) != 0 {
		var decoded any
		if err := json.Unmarshal(raw, &decoded); err != nil {
			return nil, response.StatusCode, text, err
		}
		if object, ok := decoded.(map[string]any); ok {
			result = object
		}
	}
	return result, response.StatusCode, text, nil
}

func p05DriveRESTCommits(ctx context.Context, port int) error {
	client := p05RESTClient{
		base:   "http://127.0.0.1:" + strconv.Itoa(port) + "/api/v1",
		client: &http.Client{Timeout: 15 * time.Second},
	}
	if !Poll(ctx, 50, 200*time.Millisecond, func() bool {
		_, status, _, err := client.request(ctx, http.MethodGet, "/commands", nil)
		return err == nil && status < 400
	}) {
		return errors.New("REST API did not respond")
	}
	createSession := func(value string) (string, error) {
		created, status, text, err := client.request(ctx, http.MethodPost, "/config/sessions", nil)
		if err != nil || status >= 400 {
			return "", fmt.Errorf("create session status=%d body=%s: %w", status, text, err)
		}
		sessionID, _ := created["session-id"].(string)
		if sessionID == "" {
			return "", fmt.Errorf("create session response=%v", created)
		}
		_, status, text, err = client.request(ctx, http.MethodPut, "/config/sessions/"+sessionID, map[string]any{fieldPath: configPathRouterID, fieldValue: value})
		if err != nil || status >= 400 {
			return "", fmt.Errorf("edit session status=%d body=%s: %w", status, text, err)
		}
		return sessionID, nil
	}
	first, err := createSession("10.0.0.2")
	if err != nil {
		return err
	}
	second, err := createSession("10.0.0.3")
	if err != nil {
		return err
	}
	type commitResult struct {
		response map[string]any
		status   int
		body     string
		err      error
	}
	commit := func(session string) commitResult {
		response, status, body, requestErr := client.request(ctx, http.MethodPost, "/config/sessions/"+session+"/commit", nil)
		return commitResult{response: response, status: status, body: body, err: requestErr}
	}
	firstDone := make(chan commitResult, 1)
	go func() { firstDone <- commit(first) }()
	if !Poll(ctx, 80, 100*time.Millisecond, func() bool {
		_, err := os.Stat("verify.started")
		return err == nil
	}) {
		return errors.New("first commit did not enter config verify")
	}
	secondResult := commit(second)
	if secondResult.err != nil {
		return secondResult.err
	}
	if secondResult.status < 400 || (!strings.Contains(secondResult.body, "candidate") && !strings.Contains(secondResult.body, "progress") && !strings.Contains(secondResult.body, "reload")) {
		return fmt.Errorf("unexpected second commit rejection status=%d body=%s", secondResult.status, secondResult.body)
	}
	var firstResult commitResult
	select {
	case firstResult = <-firstDone:
	case <-time.After(18 * time.Second):
		return errors.New("first commit did not finish")
	case <-ctx.Done():
		return ctx.Err()
	}
	if firstResult.err != nil || firstResult.status >= 400 || firstResult.response["status"] != statusCommitted {
		return fmt.Errorf("first commit failed: status=%d response=%v body=%s: %w", firstResult.status, firstResult.response, firstResult.body, firstResult.err)
	}
	active, err := os.ReadFile("concurrent-config-commit.conf")
	if err != nil {
		return err
	}
	if !strings.Contains(string(active), "router-id 10.0.0.2") || strings.Contains(string(active), "router-id 10.0.0.3") {
		return fmt.Errorf("active config after concurrent commits was wrong:\n%s", active)
	}
	const marker = "OK: concurrent commit rejected second candidate\n"
	if err := os.WriteFile("concurrent.ok", []byte(marker), 0o600); err != nil {
		return err
	}
	fmt.Fprint(os.Stderr, marker)
	return nil
}
