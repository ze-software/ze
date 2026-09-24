// Design: docs/guide/rpki.md -- policy revalidation of retained received routes.
// Related: prepare.go -- ze-reload.conf selects the daemon's writable SIGHUP input.
package bgp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/netip"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

const (
	rpkiReloadScenario = "rtr-stayrtr"
	rpkiReloadInvalid  = "9.58.0.0/24"
	rpkiReloadControl  = "10.58.0.0/16"
)

type rpkiReloadRoute struct {
	Key        string `json:"key"`
	Family     string `json:"family"`
	Attributes string `json:"attr-hex"`
	NextHop    string `json:"nhop-hex"`
	NLRI       string `json:"nlri-hex"`
	State      *uint8 `json:"validation-state"`
	Ineligible *bool  `json:"ineligible"`
}

type rpkiReloadSource struct {
	Generation uint64
	Counters   map[string]int64
}

type rpkiReloadSink struct {
	Since        string
	NeighborID   string
	LocalAddress string
	LocalPort    int64
	RemotePort   int64
	Counters     map[string]int64
}

// checkRPKIPolicyReload leaves FRR's configuration untouched for the entire run.
// Source UPDATE/refresh counters and FRR's session generation fence the policy
// transitions. BIRD's session identity and Ze's sink connection/OPEN counters
// fence downstream export and withdrawal. RTR may reconnect during replaceConfig;
// each phase requires a synchronized, usable copy of the same StayRTR payload set.
func checkRPKIPolicyReload(ctx context.Context, check *interoplab.CheckContext) error {
	// Preserve the existing independent RTR and per-prefix validation checks.
	initial := []operation{
		{kind: opWaitContains, peer: peerStayRTR, command: []string{"wget", "-q", "-O", "-", "http://127.0.0.1:9847/rpki.json"}, contains: []string{"prefix"}},
		{kind: opWaitJSONFields, peer: "ze", command: zeCommand("show bgp rpki status"), minimum: map[string]int{"vrp-count-ipv4": 2, "vrp-count-ipv6": 2}, timeout: 90 * time.Second},
		{kind: opRequireContains, peer: "ze", command: zeCommand("show bgp rpki roa"), contains: []string{"9.58.0.0/16", "10.58.0.0/16", "2001:db8:58::/48", "2001:db8:59::/48", "4200000001", "65001"}},
		{kind: opRequireJSONFields, peer: "ze", command: zeCommand("request bgp rpki validate 9.58.0.0/16 4200000001"), fields: map[string]string{fieldState: rpkiStateValid}},
		{kind: opRequireJSONFields, peer: "ze", command: zeCommand("request bgp rpki validate 9.58.0.0/24 4200000001"), fields: map[string]string{fieldState: rpkiStateValid}},
		{kind: opRequireJSONFields, peer: "ze", command: zeCommand("request bgp rpki validate 9.58.0.0/25 4200000001"), fields: map[string]string{fieldState: rpkiStateInvalid}},
		{kind: opRequireJSONFields, peer: "ze", command: zeCommand("request bgp rpki validate 9.58.0.0/16 65001"), fields: map[string]string{fieldState: rpkiStateInvalid}},
		{kind: opRequireJSONFields, peer: "ze", command: zeCommand("request bgp rpki validate 10.58.0.0/16 65001"), fields: map[string]string{fieldState: rpkiStateValid}},
		{kind: opRequireJSONFields, peer: "ze", command: zeCommand("request bgp rpki validate 10.58.0.0/24 65001"), fields: map[string]string{fieldState: rpkiStateInvalid}},
		{kind: opRequireJSONFields, peer: "ze", command: zeCommand("request bgp rpki validate 2001:db8:58::/48 4200000001"), fields: map[string]string{fieldState: rpkiStateValid}},
		{kind: opRequireJSONFields, peer: "ze", command: zeCommand("request bgp rpki validate 2001:db8:58::/64 4200000001"), fields: map[string]string{fieldState: rpkiStateInvalid}},
		{kind: opRequireJSONFields, peer: "ze", command: zeCommand("request bgp rpki validate 11.58.0.0/16 65001"), fields: map[string]string{fieldState: rpkiStateNotFound}},
		{kind: opRequireJSONFields, peer: "ze", command: zeCommand("request bgp rpki validate 2001:db8:5a::/48 65001"), fields: map[string]string{fieldState: rpkiStateNotFound}},
		{kind: opFRRSession, argument: zeLabAddress},
		{kind: opBIRDSession, argument: birdZeProtocol},
		{kind: opFRRRoute, argument: rpkiReloadInvalid},
		{kind: opFRRRoute, argument: rpkiReloadControl},
	}
	for index := range initial {
		if err := runOperation(ctx, check.Network, check.Lab, &initial[index]); err != nil {
			return checkerFailure(ctx, check.Lab, rpkiReloadScenario, index+1, err)
		}
	}
	source, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: 60 * time.Second, Interval: time.Second, Description: "FRR initial UPDATE and End-of-RIB",
	}, func(probeCtx context.Context) (rpkiReloadSource, error) {
		return readRPKIReloadSource(probeCtx, check)
	}, func(state rpkiReloadSource) bool {
		return state.Counters["updates-received"] > 0 && state.Counters["eor-received"] > 0
	})
	if err != nil {
		return err
	}
	sink, err := readRPKIReloadSink(ctx, check)
	if err != nil {
		return err
	}
	original, err := waitRPKIReloadPhase(ctx, check, "initial reject", false, nil)
	if err != nil {
		return err
	}
	if err := requireRPKIReloadContinuity(ctx, check, source, sink); err != nil {
		return err
	}
	for _, phase := range []struct {
		name     string
		config   string
		accepted bool
	}{
		{"accept retained Invalid", zeMountedReloadConfig, true},
		{"restore rejection", zeMountedConfig, false},
	} {
		if err := applyRPKIReload(ctx, check.Lab, phase.config); err != nil {
			return fmt.Errorf("%s: %w", phase.name, err)
		}
		if _, err := waitRPKIReloadPhase(ctx, check, phase.name, phase.accepted, original); err != nil {
			return err
		}
		if err := requireRPKIReloadContinuity(ctx, check, source, sink); err != nil {
			return fmt.Errorf("%s: %w", phase.name, err)
		}
	}
	return nil
}

func queryRPKIReloadJSON(ctx context.Context, lab interoplab.CheckerLab, command string, target any) error {
	argv := zeCommand(command)
	output, err := lab.Query(ctx, "ze", argv, queryEnvironment("ze", argv))
	if err != nil {
		return err
	}
	if err := json.Unmarshal([]byte(output), target); err != nil {
		return fmt.Errorf("%s: decode JSON: %w; output: %s", command, err, output)
	}
	return nil
}

func applyRPKIReload(ctx context.Context, lab interoplab.CheckerLab, config string) error {
	type reloadStatus struct {
		Generation *uint64 `json:"generation"`
		Outcome    string  `json:"last-outcome"`
	}
	var before reloadStatus
	if err := queryRPKIReloadJSON(ctx, lab, "show reload-status", &before); err != nil {
		return err
	}
	if before.Generation == nil {
		return errors.New("reload status has no generation")
	}
	if _, err := lab.Exec(ctx, "ze", []string{"cp", config, zeRunningConfig}, nil); err != nil {
		return err
	}
	if err := lab.Signal(ctx, "ze", signalHUP); err != nil {
		return err
	}
	after, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: 60 * time.Second, Interval: time.Second, Description: "daemon SIGHUP completion",
	}, func(probeCtx context.Context) (reloadStatus, error) {
		var current reloadStatus
		err := queryRPKIReloadJSON(probeCtx, lab, "show reload-status", &current)
		return current, err
	}, func(current reloadStatus) bool {
		return current.Generation != nil && *current.Generation > *before.Generation
	})
	if err != nil {
		return err
	}
	if *after.Generation != *before.Generation+1 || after.Outcome != "applied" {
		return fmt.Errorf("reload generation/outcome = %d/%s, want %d/applied", *after.Generation, after.Outcome, *before.Generation+1)
	}
	return nil
}

func waitRPKIReloadPhase(ctx context.Context, check *interoplab.CheckContext, phase string, accepted bool, original map[string]rpkiReloadRoute) (map[string]rpkiReloadRoute, error) {
	var lastErr error
	routes, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: 60 * time.Second, Interval: time.Second, Description: "RPKI reload " + phase,
	}, func(probeCtx context.Context) (map[string]rpkiReloadRoute, error) {
		routes, err := readRPKIReloadPhase(probeCtx, check, accepted, original)
		lastErr = err
		return routes, err
	}, func(routes map[string]rpkiReloadRoute) bool { return routes != nil })
	if err != nil {
		return nil, fmt.Errorf("%s: %w", phase, errors.Join(err, lastErr))
	}
	return routes, nil
}

func readRPKIReloadPhase(ctx context.Context, check *interoplab.CheckContext, accepted bool, original map[string]rpkiReloadRoute) (map[string]rpkiReloadRoute, error) {
	if err := requireRPKIReloadCache(ctx, check.Lab); err != nil {
		return nil, err
	}
	var received struct {
		Peers map[string][]rpkiReloadRoute `json:"adj-rib-in"`
	}
	if err := queryRPKIReloadJSON(ctx, check.Lab, "show bgp adj-rib-in", &received); err != nil {
		return nil, err
	}
	source := networkHostAddress(check.Network, 3)
	routes, err := requireRPKIReloadRoutes(received.Peers[source], accepted, original)
	if err != nil {
		return nil, err
	}
	var best []struct {
		Prefix string `json:"prefix"`
		Peer   string `json:"best-peer"`
	}
	if err := queryRPKIReloadJSON(ctx, check.Lab, "show bgp rib best first 10", &best); err != nil {
		return nil, err
	}
	selected := make(map[string]string, len(best))
	for _, route := range best {
		selected[route.Prefix] = route.Peer
	}
	if selected[rpkiReloadControl] != source || (selected[rpkiReloadInvalid] == source) != accepted || (!accepted && selected[rpkiReloadInvalid] != "") {
		return nil, fmt.Errorf("best paths %v do not match Invalid accepted=%t with Valid control from %s", selected, accepted, source)
	}
	output, err := check.Lab.Query(ctx, peerBIRD, []string{cmdBirdc, "show route protocol ze_peer all"}, nil)
	if err != nil {
		return nil, err
	}
	if err := requireRPKIReloadExport(output, accepted); err != nil {
		return nil, err
	}
	// Recheck usable data after the route/export observation, not just before it.
	if err := requireRPKIReloadCache(ctx, check.Lab); err != nil {
		return nil, err
	}
	return routes, nil
}

func requireRPKIReloadRoutes(received []rpkiReloadRoute, accepted bool, original map[string]rpkiReloadRoute) (map[string]rpkiReloadRoute, error) {
	routes := make(map[string]rpkiReloadRoute, 2)
	for _, route := range received {
		for _, prefix := range []string{rpkiReloadInvalid, rpkiReloadControl} {
			if route.Key != "ipv4/unicast:"+prefix {
				continue
			}
			if _, duplicate := routes[prefix]; duplicate {
				return nil, fmt.Errorf("received duplicate path for %s", prefix)
			}
			routes[prefix] = route
		}
	}
	for _, prefix := range []string{rpkiReloadInvalid, rpkiReloadControl} {
		route, exists := routes[prefix]
		state, ineligible := uint8(1), false
		if prefix == rpkiReloadInvalid {
			state, ineligible = 3, !accepted
		}
		if !exists || route.Family != "ipv4/unicast" || route.State == nil || route.Ineligible == nil || *route.State != state || *route.Ineligible != ineligible || route.Attributes == "" || route.NextHop == "" || route.NLRI == "" {
			return nil, fmt.Errorf("retained %s does not have validation-state=%d ineligible=%t and received wire attributes: %+v", prefix, state, ineligible, route)
		}
		if before, ok := original[prefix]; ok && (route.Attributes != before.Attributes || route.NextHop != before.NextHop || route.NLRI != before.NLRI) {
			return nil, fmt.Errorf("policy reload changed retained received bytes for %s", prefix)
		}
	}
	return routes, nil
}

func requireRPKIReloadExport(output string, accepted bool) error {
	// Attribute checks stay inside the exact route's block. A control route's
	// AS_PATH cannot establish where the Invalid route came from.
	blocks := make(map[string]string, 2)
	current := ""
	for line := range strings.SplitSeq(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if _, err := netip.ParsePrefix(fields[0]); err == nil {
			current = fields[0]
		}
		switch current {
		case rpkiReloadControl, rpkiReloadInvalid:
			blocks[current] += line + "\n"
		}
	}
	if err := requireBIRDASPath(blocks[rpkiReloadControl], rpkiReloadControl, "65000 65001"); err != nil {
		return err
	}
	if accepted {
		return requireBIRDASPath(blocks[rpkiReloadInvalid], rpkiReloadInvalid, "65000 65001")
	}
	if _, present := blocks[rpkiReloadInvalid]; present {
		return fmt.Errorf("BIRD still holds the rejected Invalid route: %s", blocks[rpkiReloadInvalid])
	}
	return nil
}

func requireRPKIReloadCache(ctx context.Context, lab interoplab.CheckerLab) error {
	var summary struct {
		Enabled bool `json:"validation-enabled"`
		Synced  int  `json:"sessions-synced"`
		VRPs    int  `json:"vrp-count"`
	}
	if err := queryRPKIReloadJSON(ctx, lab, "show bgp rpki summary", &summary); err != nil {
		return err
	}
	// The RTR state returns to idle between polls. Only a completed,
	// unexpired synchronization proves that this cache can validate routes.
	if !summary.Enabled || summary.Synced != 1 || summary.VRPs != 4 {
		return fmt.Errorf("StayRTR validation is not ready: %+v", summary)
	}
	type vrp struct {
		Prefix    string      `json:"prefix"`
		MaxLength int         `json:"max-length"`
		ASN       json.Number `json:"asn"`
	}
	var payload struct {
		Entries []vrp `json:"entries"`
	}
	if err := queryRPKIReloadJSON(ctx, lab, "show bgp rpki roa", &payload); err != nil {
		return err
	}
	want := map[vrp]bool{
		{"9.58.0.0/16", 24, "4200000001"}:      true,
		{"10.58.0.0/16", 16, "65001"}:          true,
		{"2001:db8:58::/48", 56, "4200000001"}: true,
		{"2001:db8:59::/48", 48, "65001"}:      true,
	}
	if len(payload.Entries) != len(want) {
		return fmt.Errorf("StayRTR payload differs from fixture: %+v", payload.Entries)
	}
	for _, entry := range payload.Entries {
		if !want[entry] {
			return fmt.Errorf("StayRTR payload has unexpected or duplicate VRP: %+v", entry)
		}
		delete(want, entry)
	}
	return nil
}

func readRPKIReloadSource(ctx context.Context, check *interoplab.CheckContext) (rpkiReloadSource, error) {
	var result rpkiReloadSource
	var document struct {
		Peers map[string]map[string]any `json:"peers"`
	}
	source := networkHostAddress(check.Network, 3)
	if err := queryRPKIReloadJSON(ctx, check.Lab, "show bgp peer "+source+" detail", &document); err != nil {
		return result, err
	}
	peer := document.Peers[source]
	if peer["state"] != "established" {
		return result, fmt.Errorf("FRR source is not established: %v", peer)
	}
	result.Counters = make(map[string]int64, 8)
	for _, key := range []string{"updates-received", "eor-received", "connections-established", "connections-dropped"} {
		value, ok := number(peer[key])
		if !ok || value < 0 {
			return result, fmt.Errorf("FRR source has no %s counter", key)
		}
		result.Counters[key] = value
	}
	messages, _ := peer["messages"].(map[string]any)
	for _, direction := range []string{"received", "sent"} {
		counters, _ := messages[direction].(map[string]any)
		for _, key := range []string{"opens", "route-refresh"} {
			value, ok := number(counters[key])
			if !ok || value < 0 {
				return result, fmt.Errorf("FRR source has no %s %s counter", direction, key)
			}
			result.Counters[direction+"-"+key] = value
		}
	}
	if result.Counters["connections-established"] < 1 {
		return result, errors.New("FRR source has no established connection counter")
	}
	generation, err := queryFRRSessionGeneration(ctx, check.Lab, networkHostAddress(check.Network, 2))
	result.Generation = generation
	return result, err
}

func requireRPKIReloadContinuity(ctx context.Context, check *interoplab.CheckContext, before rpkiReloadSource, sinkBefore rpkiReloadSink) error {
	after, err := readRPKIReloadSource(ctx, check)
	if err != nil {
		return err
	}
	if before.Generation != after.Generation || !maps.Equal(before.Counters, after.Counters) {
		return fmt.Errorf("source UPDATE, refresh, or session changed during RPKI reload: before=%+v after=%+v", before, after)
	}
	sinkAfter, err := readRPKIReloadSink(ctx, check)
	if err != nil {
		return err
	}
	if sinkBefore.Since != sinkAfter.Since || sinkBefore.NeighborID != sinkAfter.NeighborID ||
		sinkBefore.LocalAddress != sinkAfter.LocalAddress || sinkBefore.LocalPort != sinkAfter.LocalPort || sinkBefore.RemotePort != sinkAfter.RemotePort ||
		!maps.Equal(sinkBefore.Counters, sinkAfter.Counters) {
		return fmt.Errorf("BIRD sink session changed during RPKI reload: before=%+v after=%+v", sinkBefore, sinkAfter)
	}
	return nil
}

func readRPKIReloadSink(ctx context.Context, check *interoplab.CheckContext) (rpkiReloadSink, error) {
	var result rpkiReloadSink
	output, err := check.Lab.Query(ctx, peerBIRD, []string{cmdBirdc, "show protocols all " + birdZeProtocol}, nil)
	if err != nil {
		return result, err
	}
	// BIRD's proto_show() prints last_state_change in Since; bgp_show_proto_info()
	// supplies the established session's neighbor ID and endpoint addresses.
	detail := make(map[string]string)
	for line := range strings.SplitSeq(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 6 && fields[0] == birdZeProtocol && fields[1] == "BGP" && fields[3] == "up" && fields[len(fields)-1] == stateEstablished {
			result.Since = strings.Join(fields[4:len(fields)-1], " ")
		}
		if key, value, ok := strings.Cut(strings.TrimSpace(line), ":"); ok {
			detail[key] = strings.TrimSpace(value)
		}
	}
	result.NeighborID = detail["Neighbor ID"]
	if result.Since == "" || result.NeighborID == "" || detail["BGP state"] != stateEstablished ||
		detail["Neighbor address"] != networkHostAddress(check.Network, 2) || detail["Source address"] != networkHostAddress(check.Network, 4) {
		return result, fmt.Errorf("BIRD sink has no established session identity: %s", output)
	}
	var document struct {
		Peers map[string]map[string]any `json:"peers"`
	}
	sink := networkHostAddress(check.Network, 4)
	if err := queryRPKIReloadJSON(ctx, check.Lab, "show bgp peer "+sink+" detail", &document); err != nil {
		return result, err
	}
	peer := document.Peers[sink]
	result.LocalAddress, _ = peer["local-ip"].(string)
	var localOK, remoteOK bool
	result.LocalPort, localOK = number(peer["local-port"])
	result.RemotePort, remoteOK = number(peer["remote-port"])
	if peer["state"] != "established" || result.LocalAddress != networkHostAddress(check.Network, 2) ||
		!localOK || !remoteOK || result.LocalPort <= 0 || result.RemotePort <= 0 {
		return result, fmt.Errorf("no established BIRD sink connection identity in Ze: %v", peer)
	}
	// UPDATEs must remain free to carry the acceptance and withdrawal. Only
	// lifetime connection/drop and OPEN counters fence a reconnect/replay here.
	result.Counters = make(map[string]int64, 4)
	for _, key := range []string{"connections-established", "connections-dropped"} {
		value, ok := number(peer[key])
		if !ok || value < 0 {
			return result, fmt.Errorf("BIRD sink has no %s counter", key)
		}
		result.Counters[key] = value
	}
	messages, _ := peer["messages"].(map[string]any)
	for _, direction := range []string{"received", "sent"} {
		counters, _ := messages[direction].(map[string]any)
		value, ok := number(counters["opens"])
		if !ok || value < 1 {
			return result, fmt.Errorf("BIRD sink has no %s OPEN counter", direction)
		}
		result.Counters[direction+"-opens"] = value
	}
	if result.Counters["connections-established"] < 1 {
		return result, errors.New("BIRD sink has no established connection counter")
	}
	return result, nil
}
