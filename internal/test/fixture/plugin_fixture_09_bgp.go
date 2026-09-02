package fixture

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func peerRows09(r commandResult09) []map[string]any {
	if !done09(r) {
		return nil
	}
	root := map09(r.data)
	peers := root["peers"]
	rows := make([]map[string]any, 0)
	switch value := peers.(type) {
	case map[string]any:
		for _, item := range value {
			if row := map09(item); row != nil {
				rows = append(rows, row)
			}
		}
	case []any:
		for _, item := range value {
			if row := map09(item); row != nil {
				rows = append(rows, row)
			}
		}
	default:
		if root["eor-sent"] != nil || root["updates-sent"] != nil {
			rows = append(rows, root)
		}
	}
	return rows
}

func bgpEORSent09(r commandResult09, expected int) bool {
	count := 0
	for _, row := range peerRows09(r) {
		if number09(row["eor-sent"]) >= 1 {
			count++
		}
	}
	return count >= expected
}

func peerCounter09(ctx context.Context, p *sdk.Plugin, address, field string) int {
	r := dispatch09(ctx, p, "show bgp peer * detail")
	root := map09(r.data)
	if peers, ok := root["peers"].(map[string]any); ok {
		if row := map09(peers[address]); row != nil {
			return number09(row[field])
		}
	}
	for _, row := range peerRows09(r) {
		if row["address"] == address || row["peer"] == address {
			return number09(row[field])
		}
	}
	return 0
}

func llgrReadvertise09(_ string) Driver {
	return func(ctx context.Context, _ []string) error {
		return observe09(ctx, "observer", func(ctx context.Context, p *sdk.Plugin) error {
			var best commandResult09
			Poll(ctx, 60, 250*time.Millisecond, func() bool {
				best = dispatch09(ctx, p, "show bgp rib best")
				return done09(best) && strings.Contains(best.text, "10.0.0.0/24")
			})
			if !strings.Contains(best.text, "10.0.0.0/24") {
				return fmt.Errorf("source route never became best: %s", best.text)
			}
			if !Poll(ctx, 60, 250*time.Millisecond, func() bool { return bgpEORSent09(dispatch09(ctx, p, "show bgp peer * detail"), 2) }) {
				return fmt.Errorf("ze never sent both peers their End-of-Rib")
			}
			routesSent := func() int {
				return peerCounter09(ctx, p, "127.0.0.2", "updates-sent") - peerCounter09(ctx, p, "127.0.0.2", "eor-sent")
			}
			if !Poll(ctx, 60, 250*time.Millisecond, func() bool { return routesSent() >= 1 }) {
				return fmt.Errorf("obsn was never sent the reflected route (routes sent=%d)", routesSent())
			}
			base := routesSent()
			mark := dispatch09(ctx, p, "request bgp rib mark-stale 127.0.0.1 60 2")
			if !done09(mark) {
				return fmt.Errorf("mark-stale failed: %s", mark.text)
			}
			clear := dispatch09(ctx, p, "clear bgp rib out * ipv4/unicast")
			if !done09(clear) {
				return fmt.Errorf("clear-rib-out failed: %s", clear.text)
			}
			fmt.Fprintln(os.Stderr, "OK: mark-stale + clear-rib-out dispatched")
			if !Poll(ctx, 60, 250*time.Millisecond, func() bool { return routesSent() > base }) {
				return fmt.Errorf("readvertise never reached obsn (routes sent=%d, want %d)", routesSent(), base+1)
			}
			return nil
		})
	}
}

func localPrefStrip09(ctx context.Context, _ []string) error {
	return observe09(ctx, "test-localpref", func(ctx context.Context, p *sdk.Plugin) error {
		got := func() bool {
			r := dispatch09(ctx, p, "show bgp peer dest-ebgp-peer detail")
			for _, row := range peerRows09(r) {
				if number09(row["updates-sent"])-number09(row["eor-sent"]) >= 1 {
					return true
				}
			}
			return false
		}
		if !Poll(ctx, 24, 250*time.Millisecond, got) {
			return fmt.Errorf("dest-ebgp-peer was never sent the forwarded route")
		}
		fmt.Fprintln(os.Stderr, "OK: dest-ebgp-peer was sent the forwarded route")
		return nil
	})
}

func mcpHiddenCommand09(ctx context.Context, _ []string) error {
	registration := sdk.Registration{Commands: []sdk.CommandDecl{
		{Name: "show mcpgate visible", Description: "visible probe command"},
		{Name: "show mcpgate concealed", Description: "concealed probe command", Hidden: true},
	}}
	return passiveObserve09(ctx, "mcpgate-test", registration, func(context.Context, *sdk.Plugin) error {
		fmt.Fprintln(os.Stderr, "OK: declared one visible and one hidden command")
		return nil
	})
}

func medLocallySet09(ctx context.Context, _ []string) error {
	return passiveObserve09(ctx, "med-local", sdk.Registration{}, func(ctx context.Context, p *sdk.Plugin) error {
		announced, withdrawn, err := p.UpdateRoute(ctx, "*", "update text origin igp med 100 nhop 101.1.101.1 nlri ipv4/unicast add 1.0.0.1/32")
		if err != nil {
			return err
		}
		if announced != 1 || withdrawn != 0 {
			return fmt.Errorf("MED route acknowledgement announced=%d withdrawn=%d", announced, withdrawn)
		}
		return nil
	})
}

func removeMED09(body []byte) ([]byte, bool, error) {
	if len(body) < 4 {
		return nil, false, fmt.Errorf("raw UPDATE body too short")
	}
	withdrawnLength := int(binary.BigEndian.Uint16(body))
	attributeLengthOffset := 2 + withdrawnLength
	if attributeLengthOffset+2 > len(body) {
		return nil, false, fmt.Errorf("withdrawn section overruns body")
	}
	attributeLength := int(binary.BigEndian.Uint16(body[attributeLengthOffset:]))
	start := attributeLengthOffset + 2
	end := start + attributeLength
	if end > len(body) {
		return nil, false, fmt.Errorf("attribute section overruns body")
	}
	attrs := body[start:end]
	out := make([]byte, 0, len(attrs))
	removed := false
	for position := 0; position < len(attrs); {
		if position+3 > len(attrs) {
			return nil, false, fmt.Errorf("truncated attribute header")
		}
		headerLength := 3
		length := int(attrs[position+2])
		if attrs[position]&0x10 != 0 {
			if position+4 > len(attrs) {
				return nil, false, fmt.Errorf("truncated extended attribute header")
			}
			headerLength = 4
			length = int(binary.BigEndian.Uint16(attrs[position+2:]))
		}
		spanEnd := position + headerLength + length
		if spanEnd > len(attrs) {
			return nil, false, fmt.Errorf("attribute length overruns section")
		}
		if attrs[position+1] == 4 {
			removed = true
		} else {
			out = append(out, attrs[position:spanEnd]...)
		}
		position = spanEnd
	}
	if !removed {
		return body, false, nil
	}
	result := make([]byte, 0, len(body)-attributeLength+len(out))
	result = append(result, body[:attributeLengthOffset]...)
	result = append(result, byte(len(out)>>8), byte(len(out)))
	result = append(result, out...)
	result = append(result, body[end:]...)
	return result, true, nil
}

func medRawDrop09(ctx context.Context, _ []string) error {
	plugin, err := newObserver("raw-med-drop")
	if err != nil {
		return err
	}
	defer plugin.Close() //nolint:errcheck // fixture teardown
	var mu sync.Mutex
	removed := 0
	plugin.OnFilterUpdate(func(input *sdk.FilterUpdateInput) (*sdk.FilterUpdateOutput, error) {
		if input.Direction != directionExport {
			return &sdk.FilterUpdateOutput{Action: sdk.FilterAccept}, nil
		}
		if len(input.Raw) == 0 {
			return nil, fmt.Errorf("RAW-MED-DROP: export callback had no raw body")
		}
		stripped, changed, err := removeMED09(input.Raw)
		if err != nil {
			return nil, fmt.Errorf("RAW-MED-DROP: %w", err)
		}
		if !changed {
			return &sdk.FilterUpdateOutput{Action: sdk.FilterAccept}, nil
		}
		mu.Lock()
		removed++
		mu.Unlock()
		fmt.Fprintf(os.Stderr, "RAW-MED-DROP: removed MULTI_EXIT_DISC for %s\n", input.Peer)
		return &sdk.FilterUpdateOutput{Action: sdk.FilterModify, Raw: stripped}, nil
	})
	result := make(chan error, 1)
	plugin.OnAllPluginsReady(func() error {
		go func() {
			var scenarioErr error
			if !Poll(ctx, 100, 100*time.Millisecond, func() bool { mu.Lock(); defer mu.Unlock(); return removed > 0 }) {
				scenarioErr = fmt.Errorf("RAW-MED-DROP: export filter never saw the forwarded route")
			} else if !Poll(ctx, 40, 250*time.Millisecond, func() bool { return peerCounter09(ctx, plugin, "127.0.0.2", "updates-sent") >= 2 }) {
				scenarioErr = fmt.Errorf("RAW-MED-DROP: ze never wrote the restored UPDATE to dest")
			}
			result <- scenarioErr
			_, _, _ = plugin.DispatchCommand(context.Background(), "request shutdown")
		}()
		return nil
	})
	runErr := plugin.Run(ctx, sdk.Registration{Filters: []sdk.FilterDecl{{Name: "drop-med-raw", Direction: sdk.FilterExport, Raw: true, OnError: sdk.OnErrorReject}}})
	select {
	case scenarioErr := <-result:
		if scenarioErr != nil {
			return scenarioErr
		}
	default:
	}
	return runErr
}

func rsObserver09(_ string, expectedPeers int, prefix string) Driver {
	return func(ctx context.Context, _ []string) error {
		plugin, err := newObserver("shutdown-after-up")
		if err != nil {
			return err
		}
		defer plugin.Close() //nolint:errcheck // fixture teardown
		plugin.SetStartupSubscriptions([]string{eventUpdate}, nil, "parsed")
		plugin.SetEncoding("json")
		var mu sync.Mutex
		forwardSeen := prefix == ""
		plugin.OnEvent(func(event string) error {
			var decoded map[string]any
			if json.Unmarshal([]byte(event), &decoded) != nil {
				return nil //nolint:nilerr // a malformed event is skipped, and failing the handler would end the session
			}
			bgp := map09(decoded["bgp"])
			message := map09(bgp["message"])
			update := map09(bgp["update"])
			encoded, _ := json.Marshal(update)
			if message["direction"] == directionSent && update["nlri"] != nil && strings.Contains(string(encoded), prefix) {
				mu.Lock()
				forwardSeen = true
				mu.Unlock()
			}
			return nil
		})
		result := make(chan error, 1)
		plugin.OnAllPluginsReady(func() error {
			go func() {
				complete := Poll(ctx, 120, 250*time.Millisecond, func() bool {
					mu.Lock()
					seen := forwardSeen
					mu.Unlock()
					return seen && bgpEORSent09(dispatch09(ctx, plugin, "show bgp peer * detail"), expectedPeers)
				})
				if !complete {
					result <- fmt.Errorf("route-server replay did not complete for %d peers and prefix %s", expectedPeers, prefix)
				} else {
					result <- nil
				}
				_, _, _ = plugin.DispatchCommand(context.Background(), "request shutdown")
			}()
			return nil
		})
		runErr := plugin.Run(ctx, sdk.Registration{})
		select {
		case scenarioErr := <-result:
			if scenarioErr != nil {
				return scenarioErr
			}
		default:
		}
		return runErr
	}
}

func medRemovalBeforeDecision09(ctx context.Context, _ []string) error {
	return observe09(ctx, "med-store-check", func(ctx context.Context, p *sdk.Plugin) error {
		status := pollCommand09(ctx, p, "show bgp adj-rib-in status", 80, func(r commandResult09) bool { return done09(r) && strings.Contains(r.text, `"total-routes":1`) })
		if !strings.Contains(status.text, `"total-routes":1`) {
			return fmt.Errorf("adj-rib-in never stored the route: %s", status.text)
		}
		rib := dispatch09(ctx, p, "show bgp adj-rib-in")
		peerRoutes := list09(map09(map09(rib.data)["adj-rib-in"])["127.0.0.1"])
		if len(peerRoutes) != 1 {
			return fmt.Errorf("127.0.0.1 stored %d routes, expected 1: %s", len(peerRoutes), rib.text)
		}
		attrs := strings.ToUpper(fmt.Sprint(map09(peerRoutes[0])["attr-hex"]))
		if strings.Contains(attrs, "80040400000064") {
			return fmt.Errorf("configured MED removal did not happen before storage: %s", attrs)
		}
		for _, survivor := range []string{"40010100", "40020602010000FDE9", "C008040000FDE9"} {
			if !strings.Contains(attrs, survivor) {
				return fmt.Errorf("MED removal also removed %s from %s", survivor, attrs)
			}
		}
		fmt.Fprintf(os.Stderr, "OK: stored attributes without a metric: %s\n", attrs)
		return nil
	})
}

func metricText09(ctx context.Context, p *sdk.Plugin) string {
	r := dispatch09(ctx, p, "show metrics values")
	if !done09(r) {
		return ""
	}
	metrics, _ := map09(r.data)["metrics"].(string)
	return metrics
}

func metricValue09(text, selector string) (float64, bool) {
	metric, labels, hasLabels := strings.Cut(selector, "{")
	if hasLabels {
		labels = strings.TrimSuffix(labels, "}")
	}
	for line := range strings.SplitSeq(text, "\n") {
		if strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		sample := fields[0]
		sampleMetric, sampleLabels, sampleHasLabels := strings.Cut(sample, "{")
		if sampleMetric != metric || sampleHasLabels != hasLabels {
			continue
		}
		if hasLabels {
			sampleLabels = strings.TrimSuffix(sampleLabels, "}")
			actual := make(map[string]struct{})
			for label := range strings.SplitSeq(sampleLabels, ",") {
				actual[label] = struct{}{}
			}
			matches := true
			for label := range strings.SplitSeq(labels, ",") {
				if _, found := actual[label]; !found {
					matches = false
					break
				}
			}
			if !matches {
				continue
			}
		}
		value, err := strconv.ParseFloat(fields[len(fields)-1], 64)
		if err == nil {
			return value, true
		}
	}
	return 0, false
}

func metricsFlap09(ctx context.Context, _ []string) error {
	return observe09(ctx, "flap-notif-test", func(ctx context.Context, p *sdk.Plugin) error {
		const durationMetric = `ze_peer_session_duration_seconds{peer="127.0.0.1"}`
		Poll(ctx, 40, 250*time.Millisecond, func() bool { value, ok := metricValue09(metricText09(ctx, p), durationMetric); return ok && value > 0 })
		before := metricText09(ctx, p)
		value, ok := metricValue09(before, durationMetric)
		if before == "" || !ok || value <= 0 {
			return fmt.Errorf("duration metric missing or not positive")
		}
		teardown := dispatch09(ctx, p, "request peer 127.0.0.1 teardown 4")
		if !done09(teardown) {
			return fmt.Errorf("teardown: status=%s data=%s", teardown.status, teardown.text)
		}
		Poll(ctx, 40, 250*time.Millisecond, func() bool {
			value, ok := metricValue09(metricText09(ctx, p), `ze_peer_session_flaps_total{peer="127.0.0.1"}`)
			return ok && value >= 1
		})
		after := metricText09(ctx, p)
		checks := []string{
			`ze_peer_session_flaps_total{peer="127.0.0.1"}`,
			`ze_peer_notifications_sent_total{peer="127.0.0.1",code="cease",subcode="4"}`,
			`ze_peer_messages_sent_total{peer="127.0.0.1",type="notification"}`,
		}
		for i, needle := range checks {
			value, ok := metricValue09(after, needle)
			if !ok || (i < 2 && value < 1) {
				return fmt.Errorf("metric %s missing or invalid (%v, %v)", needle, value, ok)
			}
		}
		fmt.Fprintln(os.Stderr, "OK: flap, notification, and duration metrics verified")
		return nil
	})
}

func metricsNameShow09(ctx context.Context, _ []string) error {
	return observe09(ctx, "metrics-name-show-test", func(ctx context.Context, p *sdk.Plugin) error {
		if !Poll(ctx, 60, 250*time.Millisecond, func() bool {
			return bgpEORSent09(dispatch09(ctx, p, "show bgp peer * detail"), 1)
		}) {
			return fmt.Errorf("ze never sent its initial-sync End-of-Rib")
		}
		got := ""
		named := dispatch09(ctx, p, "show metrics name go_goroutines")
		switch {
		case done09(named):
			data := map09(named.data)
			if _, ok := data["series"]; !ok {
				return fmt.Errorf("metrics-name: named form bad shape %s", named.text)
			}
			if _, ok := data["metric"]; !ok {
				return fmt.Errorf("metrics-name: named form bad shape %s", named.text)
			}
			got = statusDone
		case named.status == statusError && strings.Contains(named.text, "metrics not available"):
			got = "no-registry"
		default:
			return fmt.Errorf("metrics-name: named form did not reach handler: %s", named.text)
		}
		fmt.Fprintf(os.Stderr, "OK: show metrics name reaches handler (%s)\n", got)

		// The metric name is `mandatory true` on the model
		// (internal/component/cmd/show/yang/ze-cli-show-cmd.yang), so the bare
		// form is refused by the dispatcher and never reaches the handler. The
		// handler's own "usage:" answer serves a caller that is not the command
		// tree, and asking for it here would assert the opposite of the model.
		bare := dispatch09(ctx, p, "show metrics name")
		if bare.status != statusError {
			return fmt.Errorf("metrics-name: the bare form answered %q, want the model's refusal", bare.status)
		}
		if !strings.Contains(bare.text, "required argument missing: name") {
			return fmt.Errorf("metrics-name: the bare form was refused for something else: %s", bare.text)
		}
		fmt.Fprintln(os.Stderr, "OK: show metrics name with no name is refused for the name it needs")
		return nil
	})
}

func metricsLifecycle09(ctx context.Context, _ []string) error {
	return observe09(ctx, "session-lifecycle-test", func(ctx context.Context, p *sdk.Plugin) error {
		var metrics string
		if !Poll(ctx, 20, 250*time.Millisecond, func() bool {
			metrics = metricText09(ctx, p)
			return strings.Contains(metrics, `ze_peer_sessions_established_total{peer="127.0.0.1"} 1`) && strings.Contains(metrics, `type="eor"`)
		}) {
			return fmt.Errorf("session-up and EOR metrics never became ready")
		}
		checks := []string{
			`ze_peer_sessions_established_total{peer="127.0.0.1"} 1`,
			metricPeerStateTransitions, `to="established"`,
			`ze_peer_messages_received_total`, `type="keepalive"`, `type="eor"`,
			`ze_wire_bytes_received_total{peer="127.0.0.1"}`,
			`ze_wire_bytes_sent_total{peer="127.0.0.1"}`,
		}
		for _, needle := range checks {
			if !strings.Contains(metrics, needle) {
				return fmt.Errorf("session lifecycle metrics missing %s", needle)
			}
		}
		fmt.Fprintln(os.Stderr, "OK: session lifecycle metrics verified")
		return nil
	})
}

func accumulatorIsolation09(ctx context.Context, _ []string) error {
	plugin, err := newObserver("shutdown-after-up")
	if err != nil {
		return err
	}
	defer plugin.Close() //nolint:errcheck // fixture teardown
	clients := map[string]bool{addrLoopbackSecond: false, addrLoopbackThird: false, "127.0.0.4": false}
	var mu sync.Mutex
	plugin.SetStartupSubscriptions([]string{eventUpdate}, nil, "parsed")
	plugin.SetEncoding("json")
	plugin.OnEvent(func(event string) error {
		var decoded map[string]any
		if json.Unmarshal([]byte(event), &decoded) != nil {
			return nil //nolint:nilerr // a malformed event is skipped, and failing the handler would end the session
		}
		bgp := map09(decoded["bgp"])
		message := map09(bgp["message"])
		update := map09(bgp["update"])
		peer := map09(map09(bgp["peer"])["remote"])
		encoded, _ := json.Marshal(update)
		address, _ := peer["address"].(string)
		if message["direction"] == directionSent && update["nlri"] != nil && strings.Contains(string(encoded), "10.0.0.0/24") {
			mu.Lock()
			if _, ok := clients[address]; ok {
				clients[address] = true
			}
			mu.Unlock()
		}
		return nil
	})
	result := make(chan error, 1)
	plugin.OnAllPluginsReady(func() error {
		go func() {
			served := func() bool {
				mu.Lock()
				defer mu.Unlock()
				for _, ok := range clients {
					if !ok {
						return false
					}
				}
				return true
			}
			if !Poll(ctx, 960, 250*time.Millisecond, served) {
				mu.Lock()
				missing := fmt.Sprint(clients)
				mu.Unlock()
				result <- fmt.Errorf("forward did not reach every client: %s", missing)
			} else {
				result <- nil
			}
			_, _, _ = plugin.DispatchCommand(context.Background(), "request shutdown")
		}()
		return nil
	})
	runErr := plugin.Run(ctx, sdk.Registration{})
	select {
	case scenarioErr := <-result:
		if scenarioErr != nil {
			return scenarioErr
		}
	default:
	}
	return runErr
}
