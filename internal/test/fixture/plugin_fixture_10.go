package fixture

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const fixture10PollDelay = 100 * time.Millisecond

type fixture10Driver struct {
	pluginName   string
	registration sdk.Registration
	setup        func(*sdk.Plugin)
	scenario     ObserverScenario
}

func init() {
	observers := map[string]fixture10Driver{
		"plugin/modify-community-add":        {pluginName: "test-modify-comm", scenario: fixture10WaitAdjRIBIn},
		"plugin/modify-increment-localpref":  {pluginName: "test-modify-inc", scenario: fixture10WaitAdjRIBIn},
		"plugin/modify-oversize-suppress":    {pluginName: "test-oversize", scenario: fixture10OversizeSuppress},
		"plugin/monitor-basic":               {pluginName: "monitor-basic-test", scenario: fixture10MonitorBasic},
		"plugin/monitor-events":              {pluginName: "monitor-events-test", scenario: fixture10MonitorEvents},
		"plugin/monitor-peer":                {pluginName: "monitor-peer-test", scenario: fixture10MonitorPeer},
		"plugin/monitor-system-netlink":      {pluginName: "monitor-system-netlink-test", scenario: fixture10MonitorSystemNetlink},
		"plugin/monitor-traceroute":          {pluginName: "monitor-traceroute-test", scenario: fixture10MonitorTraceroute},
		"plugin/mpls-forwarding-show":        {pluginName: "mpls-forwarding-show-test", scenario: fixture10MPLSForwardingShow},
		"plugin/mpls-push":                   {pluginName: "route-driver", scenario: fixture10MPLSPush},
		"plugin/mpls-withdraw":               {pluginName: "route-driver", scenario: fixture10MPLSWithdraw},
		"plugin/mrt-dump-all":                {pluginName: "mrt-observer", scenario: fixture10MRTAll},
		"plugin/mrt-dump-updates":            {pluginName: "mrt-observer", scenario: fixture10MRTUpdates},
		"plugin/multipath-basic":             {pluginName: "multipath-test", scenario: fixture10Multipath},
		"plugin/mup-ipv4-announce":           {pluginName: "announce-routes", scenario: fixture10MUPIPv4},
		"plugin/open-in-established":         {pluginName: "watch-down", scenario: fixture10OpenInEstablished},
		"plugin/mup-ipv6-announce":           {pluginName: "announce-routes", scenario: fixture10MUPIPv6},
		"plugin/nexthop-self-ipv6-forward":   {pluginName: "rr-ipv6-test", scenario: fixture10NextHopSelfIPv6},
		"plugin/nexthop-self":                {pluginName: "nh-test", scenario: fixture10NextHopSelf},
		"plugin/nexthop-unchanged":           {pluginName: "nh-test", scenario: fixture10NextHopUnchanged},
		"plugin/originated-nexthop-peer-own": {pluginName: "originated-nh", scenario: fixture10OriginatedNextHop},
		"plugin/peer-selector-asn":           {pluginName: "peer-selector-asn", scenario: fixture10PeerSelectorASN},
		"plugin/peer-selector-name-and-ip":   {pluginName: "peer-selector", scenario: fixture10PeerSelectorNameIP},
		"plugin/ping-show":                   {pluginName: "ping-show-test", scenario: fixture10PingShow},
		"plugin/pki-certificate-export-show": {pluginName: "pki-export-show-test", scenario: fixture10PKIExport},
		"plugin/plugin-announce":             {pluginName: pluginNameAddRemove, scenario: fixture10PluginAnnounce},
		"plugin/plugin-attributes":           {pluginName: pluginNameAddRemove, scenario: fixture10PluginAttributes},
		"plugin/plugin-command-completion": {
			pluginName: "completion-test",
			registration: sdk.Registration{Commands: []sdk.CommandDecl{
				{Name: cmdShowTestCompletionVisible, Description: "Visible command"},
				{Name: cmdShowTestCompletionHidden, Description: "Hidden command", Hidden: true},
			}},
			setup: fixture10CompletionSetup, scenario: fixture10CommandCompletion,
		},
	}
	// Each closure keeps its own copy of the driver, so the value is copied
	// deliberately rather than indexed.
	for name, observer := range observers { //nolint:gocritic // the registered closure outlives the iteration
		Register(name, func(ctx context.Context, _ []string) error {
			if observer.setup == nil {
				return Observe(ctx, observer.pluginName, observer.registration, observer.scenario)
			}
			return fixture10Observe(ctx, &observer)
		})
	}
	Register("plugin/plugin-check", fixture10PluginCheck)
}

func fixture10Observe(ctx context.Context, driver *fixture10Driver) error {
	plugin, err := newObserver(driver.pluginName)
	if err != nil {
		return fmt.Errorf("connect observer %s: %w", driver.pluginName, err)
	}
	defer plugin.Close() //nolint:errcheck // fixture teardown, so a close failure changes no assertion
	if driver.setup != nil {
		driver.setup(plugin)
	}
	result := make(chan error, 1)
	plugin.OnAllPluginsReady(func() error {
		go func() {
			scenarioErr := invokeScenario(ctx, plugin, driver.scenario)
			result <- scenarioErr
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_, _, _ = plugin.DispatchCommand(shutdownCtx, "request shutdown")
		}()
		return nil
	})
	runErr := plugin.Run(ctx, driver.registration)
	select {
	case scenarioErr := <-result:
		return errors.Join(scenarioErr, runErr)
	default:
		return runErr
	}
}

type fixture10Reply struct {
	status string
	raw    json.RawMessage
	err    error
}

func fixture10Call(ctx context.Context, plugin *sdk.Plugin, command string) fixture10Reply {
	var raw json.RawMessage
	status, err := Dispatch(ctx, plugin, command, &raw)
	return fixture10Reply{status: status, raw: raw, err: err}
}

func (reply fixture10Reply) requireDone(label string) error {
	if reply.err != nil || reply.status != statusDone {
		return fmt.Errorf("%s: status=%q data=%s error=%w", label, reply.status, reply.raw, reply.err)
	}
	return nil
}

func fixture10Decode(raw json.RawMessage, value any) error {
	if len(raw) == 0 {
		return fmt.Errorf("empty command data")
	}
	if raw[0] == '"' {
		var text string
		if err := json.Unmarshal(raw, &text); err != nil {
			return err
		}
		raw = json.RawMessage(text)
	}
	if err := json.Unmarshal(raw, value); err != nil {
		return err
	}
	return nil
}

func fixture10Object(reply fixture10Reply, label string) (map[string]any, error) {
	if err := reply.requireDone(label); err != nil {
		return nil, err
	}
	var data map[string]any
	if err := fixture10Decode(reply.raw, &data); err != nil {
		return nil, fmt.Errorf("%s: decode data: %w", label, err)
	}
	return data, nil
}

func fixture10Number(value any) int {
	switch number := value.(type) {
	case float64:
		return int(number)
	case int:
		return number
	case json.Number:
		value, _ := number.Int64()
		return int(value)
	default:
		return 0
	}
}

func fixture10Quiesce(ctx context.Context, plugin *sdk.Plugin) error {
	reply := fixture10Call(ctx, plugin, "request quiesce")
	return reply.requireDone("request quiesce")
}

func fixture10Update(ctx context.Context, plugin *sdk.Plugin, selector, command string) error {
	_, _, err := plugin.UpdateRoute(ctx, selector, command)
	if err != nil {
		return fmt.Errorf("update route for %s: %w", selector, err)
	}
	return nil
}

func fixture10PeerRows(ctx context.Context, plugin *sdk.Plugin, selector string) (map[string]any, bool) {
	reply := fixture10Call(ctx, plugin, "show bgp peer "+selector+" detail")
	data, err := fixture10Object(reply, "peer detail")
	if err != nil {
		return nil, false
	}
	rows, ok := data["peers"].(map[string]any)
	return rows, ok
}

func fixture10PeerCounter(ctx context.Context, plugin *sdk.Plugin, selector, field string) (int, bool) {
	rows, ok := fixture10PeerRows(ctx, plugin, selector)
	if !ok {
		return 0, false
	}
	for _, raw := range rows {
		row, ok := raw.(map[string]any)
		if ok {
			return fixture10Number(row[field]), true
		}
	}
	return 0, false
}

func fixture10WaitEOR(ctx context.Context, plugin *sdk.Plugin, selector string, attempts int) error {
	if Poll(ctx, attempts, fixture10PollDelay, func() bool {
		rows, ok := fixture10PeerRows(ctx, plugin, selector)
		if !ok {
			return false
		}
		for _, raw := range rows {
			if row, ok := raw.(map[string]any); ok && fixture10Number(row["eor-sent"]) >= 1 {
				return true
			}
		}
		return false
	}) {
		return nil
	}
	return fmt.Errorf("ze never sent its End-of-RIB to %s", selector)
}

func fixture10WaitAdjRIBIn(ctx context.Context, plugin *sdk.Plugin) error {
	if !Poll(ctx, 60, fixture10PollDelay, func() bool {
		data, err := fixture10Object(fixture10Call(ctx, plugin, "show bgp adj-rib-in status"), "adj-rib-in status")
		return err == nil && fixture10Number(data["total-routes"]) >= 1
	}) {
		return fmt.Errorf("adj-rib-in never reported the received route")
	}
	return nil
}

func fixture10OversizeSuppress(ctx context.Context, plugin *sdk.Plugin) error {
	if !Poll(ctx, 100, fixture10PollDelay, func() bool {
		rows, ok := fixture10PeerRows(ctx, plugin, "*")
		if !ok || len(rows) != 2 {
			return false
		}
		source, sourceOK := rows["127.0.0.1"].(map[string]any)
		dest, destOK := rows["127.0.0.2"].(map[string]any)
		return sourceOK && destOK && fixture10Number(source["updates-received"])-fixture10Number(source["eor-received"]) >= 1 && fixture10Number(dest["eor-sent"]) >= 1
	}) {
		return fmt.Errorf("route or two-peer End-of-RIB state never became observable")
	}
	if err := fixture10Quiesce(ctx, plugin); err != nil {
		return err
	}
	rows, ok := fixture10PeerRows(ctx, plugin, "*")
	if !ok || len(rows) != 2 {
		return fmt.Errorf("expected two peer rows after quiesce")
	}
	dest, ok := rows["127.0.0.2"].(map[string]any)
	if !ok {
		return fmt.Errorf("destination peer 127.0.0.2 missing")
	}
	if beyond := fixture10Number(dest["updates-sent"]) - fixture10Number(dest["eor-sent"]); beyond != 0 {
		return fmt.Errorf("destination received %d UPDATEs beyond its End-of-RIB", beyond)
	}
	fmt.Fprintln(os.Stderr, "OK: route accepted from the source, nothing beyond EOR sent to the dest")
	return nil
}

func fixture10MonitorBasic(ctx context.Context, plugin *sdk.Plugin) error {
	if err := fixture10WaitEOR(ctx, plugin, "peer1", 60); err != nil {
		return err
	}
	data, err := fixture10Object(fixture10Call(ctx, plugin, "monitor bgp"), "monitor bgp")
	if err != nil {
		return err
	}
	if data["status"] != statusMonitorConfigured {
		return fmt.Errorf("monitor bgp: expected monitor-configured, got %v", data["status"])
	}
	fmt.Fprintln(os.Stderr, "OK: monitor returned status=monitor-configured")
	return nil
}

func fixture10MonitorEvents(ctx context.Context, plugin *sdk.Plugin) error {
	if err := fixture10WaitEOR(ctx, plugin, "peer1", 60); err != nil {
		return err
	}
	data, err := fixture10Object(fixture10Call(ctx, plugin, "monitor bgp event update,state"), "monitor bgp event")
	if err != nil {
		return err
	}
	events, ok := data["event-types"].([]any)
	if !ok {
		return fmt.Errorf("monitor bgp event: event-types is %T", data["event-types"])
	}
	foundUpdate, foundState := false, false
	for _, event := range events {
		foundUpdate = foundUpdate || event == eventUpdate
		foundState = foundState || event == eventState
	}
	if !foundUpdate || !foundState {
		return fmt.Errorf("monitor bgp event: expected update and state, got %v", events)
	}
	fmt.Fprintf(os.Stderr, "OK: monitor event-types=%v\n", events)
	return nil
}

func fixture10MonitorPeer(ctx context.Context, plugin *sdk.Plugin) error {
	if err := fixture10WaitEOR(ctx, plugin, "peer1", 60); err != nil {
		return err
	}
	data, err := fixture10Object(fixture10Call(ctx, plugin, "monitor bgp peer 10.0.0.1"), "monitor bgp peer")
	if err != nil {
		return err
	}
	if data["peer"] != addrPeerOne {
		return fmt.Errorf("monitor bgp peer: expected 10.0.0.1, got %v", data["peer"])
	}
	fmt.Fprintln(os.Stderr, "OK: monitor peer=10.0.0.1")
	return nil
}

func fixture10MonitorSystemNetlink(ctx context.Context, plugin *sdk.Plugin) error {
	first := fixture10Call(ctx, plugin, "monitor system netlink")
	if first.err != nil && strings.Contains(first.err.Error(), "not available on this platform") {
		fmt.Fprintln(os.Stderr, "OK: netlink monitor not available on this platform")
		return nil
	}
	data, err := fixture10Object(first, "netlink monitor")
	if err != nil {
		return err
	}
	if data["status"] != statusMonitorConfigured || data["group"] != "all" {
		return fmt.Errorf("netlink monitor: expected status=monitor-configured group=all, got %v", data)
	}
	route, err := fixture10Object(fixture10Call(ctx, plugin, "monitor system netlink route"), "netlink route monitor")
	if err != nil {
		return err
	}
	if route["group"] != groupRoute {
		return fmt.Errorf("netlink route monitor: expected group=route, got %v", route["group"])
	}
	bogus := fixture10Call(ctx, plugin, "monitor system netlink bogus")
	if bogus.status != statusError && bogus.err == nil {
		return fmt.Errorf("netlink bogus group: expected error, got status=%q data=%s", bogus.status, bogus.raw)
	}
	fmt.Fprintln(os.Stderr, "OK: monitor system netlink all/route/bogus contracts held")
	return nil
}

func fixture10Probe(ctx context.Context, plugin *sdk.Plugin, command, label string) error {
	reply := fixture10Call(ctx, plugin, command)
	if reply.status == statusError || reply.err != nil {
		if reply.err == nil || !strings.Contains(reply.err.Error(), "CAP_NET_RAW") {
			return fmt.Errorf("%s: unexpected error (want CAP_NET_RAW): %w", label, reply.err)
		}
		fmt.Fprintf(os.Stderr, "OK: %s wired; raw ICMP needs CAP_NET_RAW: %v\n", label, reply.err)
		return nil
	}
	data, err := fixture10Object(reply, label)
	if err != nil {
		return err
	}
	hops, ok := data["hops"].([]any)
	if !ok || len(hops) == 0 {
		return fmt.Errorf("%s: expected non-empty hops, got %v", label, data["hops"])
	}
	hop, ok := hops[0].(map[string]any)
	if !ok || fixture10Number(hop["ttl"]) != 1 || hop["addr"] != addrLoopback {
		return fmt.Errorf("%s: invalid first hop: %v", label, hops[0])
	}
	for _, field := range []string{"ttl", "addr", "rtt-ms"} {
		if _, found := hop[field]; !found {
			return fmt.Errorf("%s: missing %q in first hop: %v", label, field, hop)
		}
	}
	return nil
}

func fixture10MonitorTraceroute(ctx context.Context, plugin *sdk.Plugin) error {
	if err := fixture10WaitEOR(ctx, plugin, "peer1", 60); err != nil {
		return err
	}
	if err := fixture10Probe(ctx, plugin, "monitor traceroute 127.0.0.1 max-hops 1", "monitor-traceroute"); err != nil {
		return err
	}
	return fixture10Probe(ctx, plugin, "show probe-round 127.0.0.1 max-hops 1", "probe-round")
}

func fixture10MPLSForwardingShow(ctx context.Context, plugin *sdk.Plugin) error {
	// The .ci expects ze's initial-sync End-of-RIB on the peer. Returning before
	// it is on the wire lets ze shut down first, and the peer then reads the
	// Cease NOTIFICATION where the marker should be.
	if err := fixture10WaitEOR(ctx, plugin, "peer1", 60); err != nil {
		return err
	}
	data, err := fixture10Object(fixture10Call(ctx, plugin, "show mpls forwarding"), "mpls-forwarding")
	if err != nil {
		return err
	}
	entries, ok := data["entries"].([]any)
	if !ok || len(entries) != 0 {
		return fmt.Errorf("mpls-forwarding: expected empty entries list, got %v", data["entries"])
	}
	limited, err := fixture10Object(fixture10Call(ctx, plugin, "show mpls forwarding limit 5"), "mpls-forwarding-limit")
	if err != nil {
		return err
	}
	if _, ok := limited["entries"].([]any); !ok {
		return fmt.Errorf("mpls-forwarding-limit: entries is %T", limited["entries"])
	}
	fmt.Fprintln(os.Stderr, "OK: show mpls forwarding returned an empty table")
	return nil
}

const fixture10MPLSAnnounce = "update text origin igp local-preference 100 nhop 10.0.0.2 nlri ipv4/mpls-label label 1000 add 10.1.0.0/24"

func fixture10MPLSPush(ctx context.Context, plugin *sdk.Plugin) error {
	if err := fixture10WaitEOR(ctx, plugin, "peer1", 60); err != nil {
		return err
	}
	if err := fixture10Update(ctx, plugin, "*", fixture10MPLSAnnounce); err != nil {
		return err
	}
	return fixture10Quiesce(ctx, plugin)
}

func fixture10MPLSWithdraw(ctx context.Context, plugin *sdk.Plugin) error {
	if err := fixture10MPLSPush(ctx, plugin); err != nil {
		return err
	}
	if !Poll(ctx, 60, fixture10PollDelay, func() bool {
		count, ok := fixture10PeerCounter(ctx, plugin, "peer1", "updates-sent")
		return ok && count >= 2
	}) {
		return fmt.Errorf("labeled announce never reached the wire")
	}
	if err := fixture10Update(ctx, plugin, "*", "update text nlri ipv4/mpls-label label 1000 del 10.1.0.0/24"); err != nil {
		return err
	}
	return fixture10Quiesce(ctx, plugin)
}

func fixture10MRT(ctx context.Context, plugin *sdk.Plugin, label string) error {
	if !Poll(ctx, 100, 200*time.Millisecond, func() bool {
		count, ok := fixture10PeerCounter(ctx, plugin, "peer1", "updates-sent")
		return ok && count >= 1
	}) {
		return fmt.Errorf("timeout waiting for %s BGP UPDATE exchange", label)
	}
	return nil
}

func fixture10MRTAll(ctx context.Context, plugin *sdk.Plugin) error {
	return fixture10MRT(ctx, plugin, "all-message")
}

func fixture10MRTUpdates(ctx context.Context, plugin *sdk.Plugin) error {
	return fixture10MRT(ctx, plugin, "updates")
}

func fixture10BestPaths(reply fixture10Reply) ([]any, error) {
	data, err := fixture10Object(reply, "show bgp rib best")
	if err != nil {
		return nil, err
	}
	rows, ok := data["best-path"].([]any)
	if !ok {
		return nil, fmt.Errorf("best-path is %T", data["best-path"])
	}
	return rows, nil
}

func fixture10FindPrefix(rows []any, prefix string) map[string]any {
	for _, raw := range rows {
		if row, ok := raw.(map[string]any); ok && row["prefix"] == prefix {
			return row
		}
	}
	return nil
}

func fixture10Multipath(ctx context.Context, plugin *sdk.Plugin) error {
	if err := fixture10WaitEOR(ctx, plugin, "peer1", 40); err != nil {
		return err
	}
	inject := fixture10Call(ctx, plugin, "request bgp rib inject 10.0.0.99 ipv4/unicast 10.1.0.0/24 origin igp aspath 65001 localpref 100")
	if err := inject.requireDone("rib inject"); err != nil {
		return err
	}
	var found map[string]any
	if !Poll(ctx, 60, fixture10PollDelay, func() bool {
		rows, err := fixture10BestPaths(fixture10Call(ctx, plugin, "show bgp rib best"))
		if err != nil {
			return false
		}
		found = fixture10FindPrefix(rows, "10.1.0.0/24")
		peers, ok := found["multipath-peers"].([]any)
		return ok && len(peers) == 1
	}) {
		return fmt.Errorf("10.1.0.0/24 never formed a two-path multipath entry")
	}
	peers, _ := found["multipath-peers"].([]any)
	fmt.Fprintf(os.Stderr, "OK: multipath: best=%v, siblings=%v\n", found["best-peer"], peers)
	return fixture10WaitEOR(ctx, plugin, "peer1", 40)
}

func fixture10MUPIPv4(ctx context.Context, plugin *sdk.Plugin) error {
	if err := fixture10WaitEOR(ctx, plugin, "peer1", 40); err != nil {
		return err
	}
	for _, command := range []string{
		"update text origin igp local-preference 200 nhop 10.0.1.254 nlri ipv4/unicast add 10.0.1.0/24",
		"update text nlri ipv4/unicast del 10.0.1.0/24",
	} {
		if err := fixture10Update(ctx, plugin, "*", command); err != nil {
			return err
		}
		if err := fixture10Quiesce(ctx, plugin); err != nil {
			return err
		}
	}
	return nil
}

func fixture10MUPIPv6(ctx context.Context, plugin *sdk.Plugin) error {
	if err := fixture10WaitEOR(ctx, plugin, "peer1", 40); err != nil {
		return err
	}
	for _, command := range []string{
		"update text origin igp local-preference 200 nhop 2001::11 nlri ipv6/unicast add fc00:1::/64",
		"update text nlri ipv6/unicast del fc00:1::/64",
	} {
		if err := fixture10Update(ctx, plugin, "*", command); err != nil {
			return err
		}
		if err := fixture10Quiesce(ctx, plugin); err != nil {
			return err
		}
	}
	return nil
}

func fixture10NextHopSelfIPv6(ctx context.Context, plugin *sdk.Plugin) error {
	if err := fixture10WaitEOR(ctx, plugin, "peer-src", 40); err != nil {
		return err
	}
	reply := fixture10Call(ctx, plugin, "show bgp peer peer-src detail")
	if err := reply.requireDone("peer detail"); err != nil {
		return err
	}
	if !strings.Contains(string(reply.raw), "route-reflector-client") {
		return fmt.Errorf("peer detail missing route-reflector-client: %.300s", reply.raw)
	}
	fmt.Fprintln(os.Stderr, "OK: IPv6 RR client config accepted, peer detail shows RR fields")
	return nil
}

func fixture10NextHopMode(ctx context.Context, plugin *sdk.Plugin, mode string) error {
	if err := fixture10WaitEOR(ctx, plugin, "peer1", 40); err != nil {
		return err
	}
	reply := fixture10Call(ctx, plugin, "show bgp peer peer1 detail")
	if err := reply.requireDone("peer detail"); err != nil {
		return err
	}
	text := string(reply.raw)
	if !strings.Contains(text, `"next-hop"`) || !strings.Contains(text, `"`+mode+`"`) {
		return fmt.Errorf("peer detail does not show next-hop %s: %.300s", mode, text)
	}
	if !Poll(ctx, 60, fixture10PollDelay, func() bool {
		rows, err := fixture10BestPaths(fixture10Call(ctx, plugin, "show bgp rib best"))
		return err == nil && fixture10FindPrefix(rows, "10.1.0.0/24") != nil
	}) {
		return fmt.Errorf("route 10.1.0.0/24 not in RIB")
	}
	fmt.Fprintf(os.Stderr, "OK: next-hop %s configured and route in RIB\n", mode)
	return nil
}

func fixture10NextHopSelf(ctx context.Context, plugin *sdk.Plugin) error {
	return fixture10NextHopMode(ctx, plugin, "self")
}

func fixture10NextHopUnchanged(ctx context.Context, plugin *sdk.Plugin) error {
	return fixture10NextHopMode(ctx, plugin, "unchanged")
}

func fixture10OriginatedNextHop(ctx context.Context, plugin *sdk.Plugin) error {
	peerUpdates := func() (int, bool) {
		rows, ok := fixture10PeerRows(ctx, plugin, "127.0.0.1")
		if !ok {
			return 0, false
		}
		for _, raw := range rows {
			if row, ok := raw.(map[string]any); ok {
				return fixture10Number(row["updates-sent"]) - fixture10Number(row["eor-sent"]), true
			}
		}

		return 0, false
	}
	before, _ := peerUpdates()
	announce := func(nextHop, prefix string) error {
		if err := fixture10Update(ctx, plugin, "127.0.0.1", "update text origin igp nhop "+nextHop+" nlri ipv4/unicast add "+prefix); err != nil {
			return err
		}
		return fixture10Quiesce(ctx, plugin)
	}
	if err := announce("203.0.113.1", "10.0.0.0/24"); err != nil {
		return err
	}
	if !Poll(ctx, 40, fixture10PollDelay, func() bool { count, ok := peerUpdates(); return ok && count > before }) {
		return fmt.Errorf("peer was never sent the route before the withheld one")
	}
	middle, _ := peerUpdates()
	if err := announce("127.0.0.1", "10.1.0.0/24"); err != nil {
		return err
	}
	if err := announce("203.0.113.1", "10.2.0.0/24"); err != nil {
		return err
	}
	if !Poll(ctx, 40, fixture10PollDelay, func() bool { count, ok := peerUpdates(); return ok && count > middle }) {
		return fmt.Errorf("peer was never sent the route after the withheld one")
	}
	fmt.Fprintln(os.Stderr, "OK: peer received the routes before and after the withheld route")
	return nil
}

func fixture10OpenInEstablished(ctx context.Context, plugin *sdk.Plugin) error {
	const metric = `ze_bgp_open_in_established_total{peer="127.0.0.1"}`
	var observed string
	if !Poll(ctx, 60, 250*time.Millisecond, func() bool {
		data, err := fixture10Object(fixture10Call(ctx, plugin, "show metrics values"), "show metrics values")
		if err != nil {
			observed = err.Error()
			return false
		}
		text, _ := data["metrics"].(string)
		for line := range strings.SplitSeq(text, "\n") {
			if strings.HasPrefix(line, metric) {
				observed = line
				return strings.HasSuffix(line, " 1")
			}
		}
		observed = "metric absent"
		return false
	}) {
		return fmt.Errorf("the refused OPEN did not increment %s: %s", metric, observed)
	}
	fmt.Fprintln(os.Stderr, "OK: session dropped after the second OPEN was refused")
	fmt.Fprintln(os.Stderr, "OK: peer went down after the second OPEN was refused")
	return nil
}
func fixture10PeerSelectorASN(ctx context.Context, plugin *sdk.Plugin) error {
	if err := fixture10Update(ctx, plugin, "as1", "update text nhop 101.1.101.1 nlri ipv4/unicast add 1.1.0.0/24"); err != nil {
		return err
	}
	return fixture10Quiesce(ctx, plugin)
}

func fixture10PeerSelectorNameIP(ctx context.Context, plugin *sdk.Plugin) error {
	if err := fixture10Update(ctx, plugin, "peer1", "update text nhop 101.1.101.1 nlri ipv4/unicast add 1.1.0.0/24"); err != nil {
		return err
	}
	if err := fixture10Quiesce(ctx, plugin); err != nil {
		return err
	}
	if err := fixture10Update(ctx, plugin, "127.0.0.1", "update text nhop 101.1.101.1 nlri ipv4/unicast add 2.2.0.0/24"); err != nil {
		return err
	}
	return fixture10Quiesce(ctx, plugin)
}

func fixture10PingCAPError(reply fixture10Reply, label string) error {
	if reply.status != statusError && reply.err == nil {
		return fmt.Errorf("%s: expected error, got status=%q data=%s", label, reply.status, reply.raw)
	}
	if reply.err == nil || !strings.Contains(reply.err.Error(), "CAP_NET_RAW") {
		return fmt.Errorf("%s: unexpected error (want CAP_NET_RAW): %w", label, reply.err)
	}
	return nil
}

func fixture10PingShow(ctx context.Context, plugin *sdk.Plugin) error {
	plain := fixture10Call(ctx, plugin, "show ping 127.0.0.1 count 1 timeout 2s")
	if plain.status == statusDone && plain.err == nil {
		data, err := fixture10Object(plain, "ping")
		if err != nil {
			return err
		}
		replies, ok := data["replies"].([]any)
		if data["destination"] != addrLoopback || fixture10Number(data["sent"]) != 1 || fixture10Number(data["received"]) != 1 || !ok || len(replies) != 1 {
			return fmt.Errorf("ping: unexpected loopback result: %v", data)
		}
		reply, ok := replies[0].(map[string]any)
		if !ok || reply["status"] != "ok" || reply["rtt-ms"] == nil {
			return fmt.Errorf("ping: invalid reply: %v", replies[0])
		}
	} else if err := fixture10PingCAPError(plain, "ping"); err != nil {
		return err
	}

	sized := fixture10Call(ctx, plugin, "show ping 127.0.0.1 count 1 size 1400 timeout 2s")
	if sized.status != statusDone || sized.err != nil {
		if err := fixture10PingCAPError(sized, "ping size 1400"); err != nil {
			return err
		}
	}

	rejected := fixture10Call(ctx, plugin, "show ping 127.0.0.1 count 1 size 99999 timeout 2s")
	if rejected.status != statusError && rejected.err == nil {
		return fmt.Errorf("ping size: expected error for 99999")
	}
	if rejected.err == nil || !strings.Contains(rejected.err.Error(), "65507") {
		return fmt.Errorf("ping size: expected bound naming 65507, got %w", rejected.err)
	}

	batch := fixture10Call(ctx, plugin, "show ping 127.0.0.1 count 3 timeout 2s")
	if batch.status == statusDone && batch.err == nil {
		data, err := fixture10Object(batch, "ping batch")
		if err != nil {
			return err
		}
		replies, ok := data["replies"].([]any)
		if fixture10Number(data["sent"]) != 3 || fixture10Number(data["received"]) != 3 || !ok || len(replies) != 3 {
			return fmt.Errorf("ping batch: invalid result: %v", data)
		}
		for index, raw := range replies {
			reply, ok := raw.(map[string]any)
			if !ok || fixture10Number(reply["seq"]) != index || reply["status"] == nil {
				return fmt.Errorf("ping batch: invalid reply %d: %v", index, raw)
			}
		}
	} else if err := fixture10PingCAPError(batch, "ping batch"); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "OK: show ping plain, size, bound, and batch contracts held")
	return nil
}

func fixture10PKIData(ctx context.Context, plugin *sdk.Plugin, command, label string) (map[string]any, error) {
	return fixture10Object(fixture10Call(ctx, plugin, command), label)
}

func fixture10PKIExport(ctx context.Context, plugin *sdk.Plugin) error {
	pem, err := fixture10PKIData(ctx, plugin, "show pki certificate name dev-1 pem", "pem")
	if err != nil {
		return err
	}
	pemText, _ := pem["pem"].(string)
	if !strings.Contains(pemText, "BEGIN CERTIFICATE") || strings.Contains(pemText, "PRIVATE KEY") {
		return fmt.Errorf("pem: certificate-only PEM contract failed")
	}
	bundle, err := fixture10PKIData(ctx, plugin, "show pki certificate name dev-1 bundle pem", "bundle")
	if err != nil {
		return err
	}
	bundleText, _ := bundle["pem"].(string)
	certAt, keyAt := strings.Index(bundleText, "BEGIN CERTIFICATE"), strings.Index(bundleText, "BEGIN PRIVATE KEY")
	if certAt < 0 || keyAt < 0 || certAt > keyAt {
		return fmt.Errorf("bundle: certificate/key ordering contract failed")
	}
	fingerprint, err := fixture10PKIData(ctx, plugin, "show pki certificate name dev-1 fingerprint", "fingerprint")
	if err != nil {
		return err
	}
	fp, _ := fingerprint["fingerprint"].(string)
	if fingerprint["algorithm"] != "sha256" || len(fp) != 95 || !strings.Contains(fp, ":") {
		return fmt.Errorf("fingerprint: invalid sha256 fingerprint %q (%v)", fp, fingerprint["algorithm"])
	}
	ca, err := fixture10PKIData(ctx, plugin, "show pki certificate name test-ca pem", "ca-pem")
	if err != nil {
		return err
	}
	if !strings.Contains(fmt.Sprint(ca["pem"]), "BEGIN CERTIFICATE") {
		return fmt.Errorf("ca-pem: missing certificate header")
	}
	caBundle := fixture10Call(ctx, plugin, "show pki certificate name test-ca bundle pem")
	if caBundle.status != statusError && caBundle.err == nil {
		return fmt.Errorf("ca-bundle: expected error for CA certificate")
	}
	fmt.Fprintln(os.Stderr, "OK: PKI PEM, bundle, fingerprint, CA, and CA refusal contracts held")
	return nil
}

func fixture10PluginAnnounce(ctx context.Context, plugin *sdk.Plugin) error {
	for _, command := range []string{
		cmdAnnounceFirstPrefix,
		"update text nhop 101.1.101.1 nlri ipv4/unicast add 1.2.0.0/25",
	} {
		if err := fixture10Update(ctx, plugin, "*", command); err != nil {
			return err
		}
	}
	return fixture10Quiesce(ctx, plugin)
}

func fixture10PluginAttributes(ctx context.Context, plugin *sdk.Plugin) error {
	for _, command := range []string{
		"update text origin igp local-preference 100 med 100 nhop 101.1.101.1 nlri ipv4/unicast add 1.0.0.1/32",
		"update text origin igp local-preference 100 med 100 nhop 101.1.101.1 nlri ipv4/unicast add 1.0.0.2/32",
		"update text origin igp as-path [1 2 3 4] local-preference 200 nhop 202.2.202.2 nlri ipv4/unicast add 2.0.0.1/32",
		"update text origin igp as-path [1 2 3 4] local-preference 200 nhop 202.2.202.2 nlri ipv4/unicast add 2.0.0.2/32",
	} {
		if err := fixture10Update(ctx, plugin, "*", command); err != nil {
			return err
		}
	}
	return fixture10Quiesce(ctx, plugin)
}

func fixture10CompletionSetup(plugin *sdk.Plugin) {
	plugin.OnExecuteCommand(func(_ string, command string, _ []string, _ string) (string, any, error) {
		switch command {
		case cmdShowTestCompletionVisible:
			return statusDone, map[string]any{"result": "visible-ok"}, nil
		case cmdShowTestCompletionHidden:
			return statusDone, map[string]any{"result": "hidden-ok"}, nil
		default:
			return statusError, nil, fmt.Errorf("unknown command %q", command)
		}
	})
}

func fixture10CommandCompletion(ctx context.Context, plugin *sdk.Plugin) error {
	if err := fixture10WaitEOR(ctx, plugin, "peer1", 60); err != nil {
		return err
	}
	data, err := fixture10Object(fixture10Call(ctx, plugin, "system command list"), "system command list")
	if err != nil {
		return err
	}
	commands, ok := data["commands"].([]any)
	if !ok || len(commands) < 100 {
		return fmt.Errorf("system command list: too few commands or wrong shape: %T len=%d", data["commands"], len(commands))
	}
	visibleCount, hiddenCount := 0, 0
	for _, raw := range commands {
		command, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		switch command["value"] {
		case cmdShowTestCompletionVisible:
			visibleCount++
			if hidden, _ := command["hidden"].(bool); hidden {
				return fmt.Errorf("visible command marked hidden")
			}
		case cmdShowTestCompletionHidden:
			hiddenCount++
			if hidden, _ := command["hidden"].(bool); !hidden {
				return fmt.Errorf("hidden command not marked hidden")
			}
		}
	}
	if visibleCount != 1 || hiddenCount != 1 {
		return fmt.Errorf("completion entries: visible=%d hidden=%d, want one each", visibleCount, hiddenCount)
	}
	fmt.Fprintln(os.Stderr, "OK: plugin commands in system command list, hidden flag correct, no duplicates")
	return nil
}

func fixture10PluginCheck(ctx context.Context, _ []string) error {
	plugin, err := newObserver("check-and-announce")
	if err != nil {
		return err
	}
	defer plugin.Close() //nolint:errcheck // fixture teardown, so a close failure changes no assertion
	matched := make(chan struct{}, 1)
	result := make(chan error, 1)
	plugin.SetStartupSubscriptions([]string{eventUpdate}, nil, "parsed")
	plugin.SetEncoding("json")
	plugin.OnEvent(func(event string) error {
		var root map[string]any
		if json.Unmarshal([]byte(event), &root) != nil {
			return nil //nolint:nilerr // a malformed event is skipped, and failing the handler would end the session
		}
		bgp, _ := root["bgp"].(map[string]any)
		message, _ := bgp["message"].(map[string]any)
		peer, _ := bgp["peer"].(map[string]any)
		remote, _ := peer["remote"].(map[string]any)
		if message["type"] != eventUpdate || remote["address"] != addrLoopback {
			return nil
		}
		update, _ := bgp["update"].(map[string]any)
		nlri, _ := update["nlri"].(map[string]any)
		var families []any
		switch family := nlri["ipv4/unicast"].(type) {
		case []any:
			families = family
		case map[string]any:
			families = []any{family}
		}
		for _, raw := range families {
			op, _ := raw.(map[string]any)
			prefixes, _ := op["nlri"].([]any)
			for _, prefix := range prefixes {
				if prefix == "0.0.0.0/32" {
					select {
					case matched <- struct{}{}:
					default:
					}
					return nil
				}
			}
		}
		return nil
	})
	plugin.OnAllPluginsReady(func() error {
		go func() {
			scenarioErr := func() error {
				select {
				case <-matched:
					if updateErr := fixture10Update(ctx, plugin, "127.0.0.1", "update text origin igp local-preference 100 nhop 5.6.7.8 nlri ipv4/unicast add 1.2.3.4/32"); updateErr != nil {
						return updateErr
					}
				case <-time.After(8 * time.Second):
					if updateErr := fixture10Update(ctx, plugin, "127.0.0.1", "update text nhop 255.255.255.255 nlri ipv4/unicast add 0.0.0.0/0"); updateErr != nil {
						return updateErr
					}
				}
				return fixture10Quiesce(ctx, plugin)
			}()
			result <- scenarioErr
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_, _, _ = plugin.DispatchCommand(shutdownCtx, "request shutdown")
		}()
		return nil
	})
	runErr := plugin.Run(ctx, sdk.Registration{})
	select {
	case scenarioErr := <-result:
		return errors.Join(scenarioErr, runErr)
	default:
		return runErr
	}
}
