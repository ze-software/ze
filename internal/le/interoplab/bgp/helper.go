// Design: docs/architecture/testing/interop.md -- compiled BGP interop personalities.
package bgp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const helperUsage = "usage: ze-test interop-bgp <process|speaker|bmp-collector|exabgp-api|exabgp-server> ..."

// Helper runs the compiled personalities used by test/interop scenarios.
func Helper(args []string) int {
	if len(args) == 0 {
		_, _ = fmt.Fprintln(os.Stderr, helperUsage)
		return 2
	}
	var err error
	switch args[0] {
	case "process":
		err = runProcessHelper(args[1:])
	case zeTestCommandSpeaker:
		err = runSpeakerHelper(args[1:], os.Stdout)
	case "bmp-collector":
		err = runBMPCollector(context.Background(), ":11019", "/tmp/bmp-collector.json")
	case "exabgp-api":
		err = runExaBGPHelper(args[1:], os.Stdin, os.Stdout, os.Stderr)
	case "exabgp-server":
		err = runExaBGPServer(args[1:], os.Stdout)
	default:
		err = fmt.Errorf("unknown interop-bgp personality %q", args[0])
	}
	if err != nil {
		slog.Error("interop-bgp helper failed", fieldError, err)
		return 1
	}
	return 0
}

type announcement struct {
	selector string
	command  string
	delay    time.Duration
	quiesce  bool
}

type processPlan struct {
	startup time.Duration
	life    time.Duration
	updates []announcement
	stop    bool
}

func runProcessHelper(args []string) error {
	if len(args) != 2 {
		return errors.New("process wants SCENARIO and PLUGIN-NAME")
	}
	scenario, name := args[0], args[1]
	if scenario == scenarioMEDIBGPPostSelectionRemovalGoBGP {
		return runRawMEDFilter(name)
	}
	if scenario == scenarioRPKIFRR {
		return runRPKIObserver(name)
	}
	if scenario == "lg-graph-lab" {
		return runLGLab(name)
	}
	plan, err := announcementPlan(scenario)
	if err != nil {
		return err
	}
	plugin, err := sdk.NewFromEnv(name)
	if err != nil {
		return fmt.Errorf("connect %s to engine: %w", name, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	plugin.OnAllPluginsReady(func() error {
		go func() {
			if sequenceErr := runAnnouncements(ctx, plugin, scenario, plan); sequenceErr != nil &&
				!errors.Is(sequenceErr, context.Canceled) {
				runtimeFailure(ctx, plugin, sequenceErr)
			}
			cancel()
		}()
		return nil
	})
	runErr := plugin.Run(ctx, sdk.Registration{})
	if errors.Is(runErr, context.Canceled) {
		return nil
	}
	return runErr
}

func runAnnouncements(ctx context.Context, plugin *sdk.Plugin, scenario string, plan processPlan) error {
	if err := sleepContext(ctx, plan.startup); err != nil {
		return err
	}
	if scenario == scenarioWireEditAPIOriginBIRD {
		if err := runWireEditAnnouncements(ctx, plugin); err != nil {
			return err
		}
		return sleepContext(ctx, plan.life)
	}
	for _, update := range plan.updates {
		if err := sleepContext(ctx, update.delay); err != nil {
			return err
		}
		if _, _, err := plugin.UpdateRoute(ctx, update.selector, update.command); err != nil {
			return fmt.Errorf("announce %q: %w", update.command, err)
		}
		if update.quiesce {
			// The former wait_for_ack contract was deliberately best effort.
			_, _, _ = plugin.DispatchCommand(ctx, "request quiesce")
		}
	}
	if plan.stop {
		if err := sleepContext(ctx, 5500*time.Millisecond); err != nil {
			return err
		}
		_, _, err := plugin.DispatchCommand(ctx, "request shutdown")
		return err
	}
	return sleepContext(ctx, plan.life)
}

func announcementPlan(scenario string) (processPlan, error) {
	const (
		v4 = "update text origin igp path 65001 nhop 172.30.0.2 nlri ipv4/unicast add "
		v6 = "update text origin igp path 65001 nhop 2001:db8::2 nlri ipv6/unicast add "
	)
	plan := processPlan{startup: time.Second, life: 120 * time.Second}
	add := func(command string) {
		plan.updates = append(plan.updates, announcement{selector: "*", command: command})
	}
	addAfter := func(delay time.Duration, command string) {
		plan.updates = append(plan.updates, announcement{selector: "*", command: command, delay: delay})
	}
	switch scenario {
	case "bgp-routes-to-frr", "bgp-route-refresh-frr":
		add(v4 + injectPrefixFirst)
		addAfter(100*time.Millisecond, v4+injectPrefixSecond)
		addAfter(100*time.Millisecond, v4+injectPrefixThird)
	case "bgp-route-withdrawal-frr":
		add(v4 + injectPrefixFirst)
		addAfter(100*time.Millisecond, v4+injectPrefixSecond)
		addAfter(100*time.Millisecond, v4+injectPrefixThird)
		addAfter(5*time.Second, "update text nlri ipv4/unicast del 10.10.1.0/24")
	case "bgp-routes-gobgp", "bgp-multihop-ebgp-bird", "bgp-multihop-ebgp-frr", "bgp-multihop-ebgp-gobgp", scenarioGracefulRestartFRR:
		add(v4 + injectPrefixFirst)
		addAfter(100*time.Millisecond, v4+injectPrefixSecond)
	case "bgp-ipv6-ebgp-bird":
		plan.updates = []announcement{
			{selector: "*", command: v6 + injectPrefixV6First, quiesce: true},
			{selector: "*", command: v6 + injectPrefixV6Second, quiesce: true},
			{selector: "*", command: v6 + injectPrefixV6Third, quiesce: true},
		}
	case "bgp-ipv6-ebgp-frr", "bgp-ipv6-ebgp-gobgp":
		add(v6 + injectPrefixV6First)
		addAfter(100*time.Millisecond, v6+injectPrefixV6Second)
		addAfter(100*time.Millisecond, v6+injectPrefixV6Third)
	case scenarioAddPathFRR:
		add("update text path-information 0.0.0.1 origin igp path 65001 65010 nhop 172.30.0.2 nlri ipv4/unicast add 10.10.0.0/24")
		addAfter(100*time.Millisecond, "update text path-information 0.0.0.2 origin igp path 65001 65020 nhop 172.30.0.2 nlri ipv4/unicast add 10.10.0.0/24")
	case "bgp-paths-limit-frr":
		plan.startup = 2 * time.Second
		plan.stop = true
		for offset := range 3 {
			index := offset + 1
			delay := time.Duration(0)
			if offset > 0 {
				delay = 500 * time.Millisecond
			}
			addAfter(delay, fmt.Sprintf("update text path-information 0.0.0.%d origin igp next-hop 10.10.0.%d nlri ipv4/unicast add 10.10.0.0/24", index, index))
		}
	case "bgp-community-frr":
		add("update text origin igp path 65001 nhop 172.30.0.2 community [65001:100 65001:200] nlri ipv4/unicast add 10.10.0.0/24")
		addAfter(100*time.Millisecond, "update text origin igp path 65001 nhop 172.30.0.2 large-community [65001:0:1] nlri ipv4/unicast add 10.10.1.0/24")
	case "bgp-extended-community-frr":
		add("update text origin igp path 65001 nhop 172.30.0.2 extended-community [target:65001:100] nlri ipv4/unicast add 10.10.0.0/24")
	case scenarioECMPFRR:
		add("update text origin igp path 65001 nhop 172.30.0.2 nlri ipv4/unicast add 10.100.0.0/24")
	case scenarioFlowspecFRR:
		plan.updates = []announcement{
			{selector: "*", command: "update text extended-community [rate-limit:0] nhop 172.30.0.2 nlri ipv4/flow add destination 10.99.0.0/24", quiesce: true},
			{selector: "*", command: "update text extended-community [rate-limit:9600] nhop 172.30.0.2 nlri ipv4/flow add destination 10.99.1.0/24 source 10.0.0.0/8 protocol tcp", quiesce: true},
		}
	case scenarioFlowspecGoBGP:
		// The third rule is an OR of two AND groups on one component type. RFC 8955
		// Section 4.2 allows a component type exactly once, so it MUST reach GoBGP as
		// one Type 4 component holding both groups. GoBGP renders that as
		// flowspecOrOfAndRule; two Type 4 components render as two [port: ...] terms
		// and do not match.
		plan.updates = []announcement{
			{selector: "*", command: "update text extended-community [rate-limit:0] nhop 172.30.0.2 nlri ipv4/flow add destination 10.99.0.0/24"},
			{selector: "*", command: "update text extended-community [rate-limit:9600] nhop 172.30.0.2 nlri ipv4/flow add destination 10.99.1.0/24 source 10.0.0.0/8 protocol tcp", delay: 100 * time.Millisecond, quiesce: true},
			{selector: "*", command: "update text extended-community [rate-limit:0] nhop 172.30.0.2 nlri ipv4/flow add destination 10.99.2.0/24 port >80 <100 port >443 <500", delay: 100 * time.Millisecond, quiesce: true},
		}
	case scenarioEVPNFRR, scenarioEVPNGoBGP:
		plan.updates = []announcement{
			{selector: "*", command: "update text origin igp nhop 172.30.0.2 nlri l2vpn/evpn add mac-ip rd 1:1 mac 00:11:22:33:44:55 etag 0 label 100", quiesce: true},
			{selector: "*", command: "update text origin igp nhop 172.30.0.2 nlri l2vpn/evpn add mac-ip rd 1:1 mac 00:11:22:33:44:66 ip 192.168.1.1 etag 0 label 100", quiesce: true},
		}
	case scenarioVPNFRR, scenarioVPNGoBGP:
		plan.updates = []announcement{
			{selector: "*", command: "update text origin igp nhop 172.30.0.2 nlri ipv4/mpls-vpn rd 65001:100 label 1000 add 10.99.0.0/24", quiesce: true},
			{selector: "*", command: "update text origin igp nhop 172.30.0.2 nlri ipv4/mpls-vpn rd 65001:100 label 1001 add 10.99.1.0/24", quiesce: true},
		}
	case "as112-community-frr":
		add("update text origin igp nhop 172.30.0.2 community [no-export] nlri ipv4/unicast add 192.175.48.0/24")
		addAfter(100*time.Millisecond, "update text origin igp nhop 172.30.0.2 community [nopeer] nlri ipv4/unicast add 192.31.196.0/24")
	case scenarioAS112OriginASFRR:
		add("update text origin igp nhop 172.30.0.2 nlri ipv4/unicast add 192.175.48.0/24")
		addAfter(100*time.Millisecond, "update text origin igp nhop 172.30.0.2 nlri ipv4/unicast add 192.31.196.0/24")
	case "bgp-med-across-as-gobgp":
		plan.startup = 25 * time.Second
		plan.life = 180 * time.Second
		plan.updates = []announcement{{selector: "!172.30.0.9", command: "update text origin igp med 42 nhop 172.30.0.2 nlri ipv4/unicast add 10.60.9.0/24"}}
	case scenarioShutdownCeaseFRR:
		plan.life = 180 * time.Second
		add(v4 + injectPrefixFirst)
	case scenarioWireEditAPIOriginBIRD:
		plan.startup = 0
	default:
		return processPlan{}, fmt.Errorf("scenario %q has no compiled process helper", scenario)
	}
	return plan, nil
}

func runWireEditAnnouncements(ctx context.Context, plugin *sdk.Plugin) error {
	const (
		birdAddress = "172.30.0.4"
		attributes  = "nhop 172.30.0.2 community [65001:100 65001:200] large-community [65001:0:1]"
	)
	row, err := peerDetail(ctx, plugin, birdAddress)
	if err != nil {
		return fmt.Errorf("queue-rail guard: %w", err)
	}
	state, ok := row[fieldState].(string)
	if !ok || (state != "stopped" && state != "connecting" && state != "active" && state != peerStateEstablished && state != "idle-hold") {
		return fmt.Errorf("peer %s reports state=%v, which is not a peer state", birdAddress, row[fieldState])
	}
	if state == peerStateEstablished {
		sent, valid := number(row["eor-sent"])
		if !valid || sent != 0 {
			return fmt.Errorf("peer %s already established with eor-sent=%v; queue rail was not exercised", birdAddress, row["eor-sent"])
		}
	}
	if _, _, err := plugin.UpdateRoute(ctx, "*", "update text "+attributes+" nlri ipv4/unicast add 10.55.0.0/24"); err != nil {
		return err
	}
	budget := 90
	if value, conversionErr := strconv.Atoi(os.Getenv("SESSION_TIMEOUT")); conversionErr == nil {
		budget = value
	}
	deadline := time.Now().Add(time.Duration(budget+90) * time.Second)
	for time.Now().Before(deadline) {
		row, err = peerDetail(ctx, plugin, "peer1")
		if err == nil {
			if sent, valid := number(row["eor-sent"]); valid && sent >= 1 {
				break
			}
		}
		if err := sleepContext(ctx, 250*time.Millisecond); err != nil {
			return err
		}
	}
	if sent, valid := number(row["eor-sent"]); !valid || sent < 1 {
		return errors.New("peer1 initial-sync EOR never reached the wire")
	}
	status, _, err := plugin.DispatchCommand(ctx, "request quiesce")
	if err != nil || status != rpc.StatusDone {
		return fmt.Errorf("quiesce barrier did not settle: status=%q: %w", status, err)
	}
	_, _, err = plugin.UpdateRoute(ctx, "*", "update text "+attributes+" nlri ipv4/unicast add 10.55.1.0/24")
	return err
}

func peerDetail(ctx context.Context, plugin *sdk.Plugin, selector string) (map[string]any, error) {
	status, data, err := plugin.DispatchCommand(ctx, "show bgp peer "+selector+" detail")
	if err != nil {
		return nil, err
	}
	if status != rpc.StatusDone {
		return nil, fmt.Errorf("peer detail status %q", status)
	}
	document, err := decodeDispatchDocument(data)
	if err != nil {
		return nil, err
	}
	if row := findPeerRow(document, selector); row != nil {
		return row, nil
	}
	return nil, fmt.Errorf("peer %q is absent from detail response", selector)
}

func findPeerRow(value any, selector string) map[string]any {
	switch current := value.(type) {
	case map[string]any:
		if row, ok := current[selector].(map[string]any); ok {
			return row
		}
		if _, ok := current[fieldState]; ok {
			return current
		}
		for _, child := range current {
			if row := findPeerRow(child, selector); row != nil {
				return row
			}
		}
	case []any:
		for _, child := range current {
			if row := findPeerRow(child, selector); row != nil {
				return row
			}
		}
	}
	return nil
}

func number(value any) (int64, bool) {
	switch current := value.(type) {
	case float64:
		return int64(current), true
	case json.Number:
		value, err := current.Int64()
		return value, err == nil
	case string:
		value, err := strconv.ParseInt(current, 10, 64)
		return value, err == nil
	default:
		return 0, false
	}
}

func runRawMEDFilter(name string) error {
	plugin, err := sdk.NewFromEnv(name)
	if err != nil {
		return err
	}
	plugin.OnFilterUpdate(func(input *sdk.FilterUpdateInput) (*sdk.FilterUpdateOutput, error) {
		if input.Direction != "export" {
			return &sdk.FilterUpdateOutput{Action: sdk.FilterAccept}, nil
		}
		if len(input.Raw) == 0 {
			return &sdk.FilterUpdateOutput{Action: sdk.FilterReject}, errors.New("RAW-MED-DROP: export callback had no raw body")
		}
		stripped, changed, err := dropMED(input.Raw)
		if err != nil {
			return &sdk.FilterUpdateOutput{Action: sdk.FilterReject}, err
		}
		if !changed {
			return &sdk.FilterUpdateOutput{Action: sdk.FilterAccept}, nil
		}
		_, _ = fmt.Fprintf(os.Stderr, "RAW-MED-DROP: removed MULTI_EXIT_DISC for %s\n", input.Peer)
		return &sdk.FilterUpdateOutput{Action: sdk.FilterModify, Raw: stripped}, nil
	})
	return plugin.Run(context.Background(), sdk.Registration{Filters: []sdk.FilterDecl{{
		Name: "drop-med-raw", Direction: sdk.FilterExport, Raw: true, OnError: sdk.OnErrorReject,
	}}})
}

func dropMED(body []byte) ([]byte, bool, error) {
	if len(body) < 4 {
		return nil, false, errors.New("RAW-MED-DROP: raw UPDATE body too short")
	}
	withdrawnLength := int(body[0])<<8 | int(body[1])
	attributeLengthOffset := 2 + withdrawnLength
	if attributeLengthOffset+2 > len(body) {
		return nil, false, errors.New("RAW-MED-DROP: withdrawn section overruns body")
	}
	attributeLength := int(body[attributeLengthOffset])<<8 | int(body[attributeLengthOffset+1])
	attributeStart := attributeLengthOffset + 2
	attributeEnd := attributeStart + attributeLength
	if attributeEnd > len(body) {
		return nil, false, errors.New("RAW-MED-DROP: attribute section overruns body")
	}
	result := make([]byte, 0, len(body))
	result = append(result, body[:attributeStart]...)
	lengthPosition := attributeLengthOffset
	result[lengthPosition], result[lengthPosition+1] = 0, 0
	removed := false
	for position := attributeStart; position < attributeEnd; {
		if position+3 > attributeEnd {
			return nil, false, errors.New("RAW-MED-DROP: truncated attribute header")
		}
		headerLength := 3
		length := int(body[position+2])
		if body[position]&0x10 != 0 {
			if position+4 > attributeEnd {
				return nil, false, errors.New("RAW-MED-DROP: truncated extended attribute header")
			}
			headerLength = 4
			length = int(body[position+2])<<8 | int(body[position+3])
		}
		end := position + headerLength + length
		if end > attributeEnd {
			return nil, false, errors.New("RAW-MED-DROP: attribute length overruns section")
		}
		if body[position+1] == 4 {
			removed = true
		} else {
			result = append(result, body[position:end]...)
		}
		position = end
	}
	if !removed {
		return body, false, nil
	}
	newLength := len(result) - attributeStart
	result[lengthPosition], result[lengthPosition+1] = byte(newLength>>8), byte(newLength)
	result = append(result, body[attributeEnd:]...)
	return result, true, nil
}

func runRPKIObserver(name string) error {
	plugin, err := sdk.NewFromEnv(name)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	plugin.OnAllPluginsReady(func() error {
		go func() {
			if err := observeRPKI(ctx, plugin, "/tmp/rpki-check.json"); err != nil {
				slog.Error("RPKI interop validation failed", fieldError, err)
			}
			_ = sleepContext(ctx, 120*time.Second)
			cancel()
		}()
		return nil
	})
	err = plugin.Run(ctx, sdk.Registration{})
	if errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}

func observeRPKI(ctx context.Context, plugin *sdk.Plugin, statusPath string) error {
	deadline := time.Now().Add(45 * time.Second)
	var last map[string]any
	for time.Now().Before(deadline) {
		_, statusData, statusErr := plugin.DispatchCommand(ctx, "show bgp rpki status")
		_, routesData, routesErr := plugin.DispatchCommand(ctx, "show bgp adj-rib-in")
		last = map[string]any{fieldRPKI: statusData, "adj-rib-in": routesData}
		if statusErr == nil && routesErr == nil {
			status, statusDecodeErr := decodeDispatchDocument(statusData)
			routes, routesDecodeErr := decodeDispatchDocument(routesData)
			if statusDecodeErr == nil && routesDecodeErr == nil {
				states := make(map[string]int64)
				collectValidationStates(routes, states)
				sessions := recursiveNumber(status, "sessions")
				vrps := recursiveNumber(status, "vrp-count-ipv4")
				_, invalidPresent := states["10.43.0.0/24"]
				if sessions >= 1 && vrps >= 1 && states["9.43.0.0/24"] == 1 && !invalidPresent && states["11.43.0.0/24"] == 2 {
					return writeJSON(statusPath, map[string]any{fieldStatus: "ok", "detail": map[string]any{fieldRPKI: status, "routes": states}})
				}
			}
		}
		if err := sleepContext(ctx, time.Second); err != nil {
			return err
		}
	}
	if err := writeJSON(statusPath, map[string]any{fieldStatus: "fail", "detail": last}); err != nil {
		return err
	}
	return errors.New("RPKI decisions did not reach the expected states")
}

func decodeDispatchDocument(data []byte) (any, error) {
	var document any
	if err := json.Unmarshal(data, &document); err != nil {
		return nil, err
	}
	if encoded, ok := document.(string); ok {
		if err := json.Unmarshal([]byte(encoded), &document); err != nil {
			return nil, err
		}
	}
	return document, nil
}

func collectValidationStates(value any, states map[string]int64) {
	switch current := value.(type) {
	case map[string]any:
		if key, ok := current["key"].(string); ok {
			parts := strings.Split(key, ":")
			if len(parts) >= 2 {
				if state, valid := number(current["validation-state"]); valid {
					states[parts[1]] = state
				}
			}
		}
		for _, child := range current {
			collectValidationStates(child, states)
		}
	case []any:
		for _, child := range current {
			collectValidationStates(child, states)
		}
	}
}

func recursiveNumber(value any, key string) int64 {
	switch current := value.(type) {
	case map[string]any:
		if found, ok := current[key]; ok {
			if result, valid := number(found); valid {
				return result
			}
		}
		for _, child := range current {
			if result := recursiveNumber(child, key); result != 0 {
				return result
			}
		}
	case []any:
		for _, child := range current {
			if result := recursiveNumber(child, key); result != 0 {
				return result
			}
		}
	}
	return 0
}

func runBMPCollector(ctx context.Context, address, statusPath string) error {
	deadline := time.Now().Add(180 * time.Second)
	var config net.ListenConfig
	listener, err := config.Listen(ctx, "tcp", address)
	if err != nil {
		return err
	}
	defer func() { _ = listener.Close() }()
	if tcp, ok := listener.(*net.TCPListener); ok {
		_ = tcp.SetDeadline(time.Now().Add(60 * time.Second))
	}
	connection, err := listener.Accept()
	if err != nil {
		if writeErr := writeJSON(statusPath, map[string]any{fieldTypes: []uint8{}, fieldError: err.Error()}); writeErr != nil {
			return writeErr
		}
		return waitBMPDeadline(ctx, deadline)
	}
	defer func() { _ = connection.Close() }()
	_ = connection.SetDeadline(time.Now().Add(120 * time.Second))
	types := make([]uint8, 0, 8)
	header := make([]byte, 6)
	for {
		if _, err := io.ReadFull(connection, header); err != nil {
			if writeErr := writeJSON(statusPath, map[string]any{fieldTypes: types, fieldError: err.Error()}); writeErr != nil {
				return writeErr
			}
			return waitBMPDeadline(ctx, deadline)
		}
		length := int(header[1])<<24 | int(header[2])<<16 | int(header[3])<<8 | int(header[4])
		if length < 6 {
			return waitBMPDeadline(ctx, deadline)
		}
		if _, err := io.CopyN(io.Discard, connection, int64(length-6)); err != nil {
			if writeErr := writeJSON(statusPath, map[string]any{fieldTypes: types, fieldError: err.Error()}); writeErr != nil {
				return writeErr
			}
			return waitBMPDeadline(ctx, deadline)
		}
		types = append(types, header[5])
		if err := writeJSON(statusPath, map[string]any{fieldTypes: types}); err != nil {
			return err
		}
		if containsType(types, 0) && containsType(types, 3) && containsType(types, 4) {
			return waitBMPDeadline(ctx, deadline)
		}
	}
}

func waitBMPDeadline(ctx context.Context, deadline time.Time) error {
	wait := time.NewTimer(max(time.Until(deadline), 0))
	defer wait.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-wait.C:
		return nil
	}
}

func containsType(types []uint8, wanted uint8) bool {
	return slices.Contains(types, wanted)
}

func writeJSON(path string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func sleepContext(ctx context.Context, duration time.Duration) error {
	if duration <= 0 {
		return nil
	}
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func runtimeFailure(ctx context.Context, plugin *sdk.Plugin, err error) {
	_, _ = fmt.Fprintf(os.Stderr, "ZE-OBSERVER-FAIL: %v\n", err)
	_, _, _ = plugin.DispatchCommand(ctx, "request shutdown")
}
