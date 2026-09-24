// Design: docs/architecture/testing/interop.md -- multihop virtual-link lifecycle.
// RFC: rfc/short/rfc2328.md Section 15; rfc/short/rfc5340.md Section 4.2.
package bgp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"

	"github.com/ze-software/ze/internal/le/interoplab"
)

type virtualLinkNeighbor struct {
	Area       string `json:"area"`
	RouterID   string `json:"router-id"`
	Interface  string `json:"interface"`
	Address    string `json:"address"`
	State      string `json:"state"`
	DDSequence uint32 `json:"dd-sequence"`
}

type virtualLinkObservation struct {
	Cost    float64
	Up      float64
	Down    float64
	Changes float64
	// Completed exchanges catch a flap that recovered between observations;
	// lower-layer, explicit-kill and dead-timer events catch one still underway.
	Events   [4]float64
	Neighbor virtualLinkNeighbor
}

func checkOSPFVirtualLink(ctx context.Context, check *interoplab.CheckContext) error {
	v6 := check.Source.Name == ospfv3VirtualLinkScenario
	fail := func(stage int, err error) error {
		err = fmt.Errorf("%w%s", err, virtualLinkFailureDiagnostics(ctx, check.Lab, v6))
		return checkerFailure(ctx, check.Lab, check.Source.Name, stage, err)
	}
	if !check.Network.IPv4.IsValid() {
		return fail(1, errors.New("virtual-link lab has no selected management network"))
	}
	remote := networkHostAddress(check.Network, 3)
	gateway, err := virtualLinkTransitGateway(ctx, check.Lab, v6)
	if err != nil {
		return fail(1, err)
	}
	if err := waitVirtualLinkKernelRoute(ctx, check.Lab, v6, gateway, true); err != nil {
		return fail(1, err)
	}
	before, err := waitVirtualLink(ctx, check.Lab, remote, v6, 20, true)
	if err != nil {
		return fail(1, err)
	}
	if err := waitVirtualLinkRoute(ctx, check.Lab, v6, true); err != nil {
		return fail(2, err)
	}
	costCommand := "ip ospf cost 30"
	if v6 {
		costCommand = "ipv6 ospf6 cost 30"
	}
	if _, err := check.Lab.Exec(ctx, peerFRRTransit, []string{cmdVtysh, "-c", frrConfigureTerminal, "-c", "interface eth2", "-c", costCommand}, nil); err != nil {
		return fail(3, err)
	}
	changed, err := waitVirtualLink(ctx, check.Lab, remote, v6, 40, true)
	if err != nil {
		return fail(4, err)
	}
	if err := requireVirtualLinkContinuity(before, changed); err != nil {
		return fail(4, fmt.Errorf("%w; before: %+v; after: %+v", err, before, changed))
	}
	if err := waitVirtualLinkKernelRoute(ctx, check.Lab, v6, gateway, true); err != nil {
		return fail(5, err)
	}
	if err := waitVirtualLinkRoute(ctx, check.Lab, v6, true); err != nil {
		return fail(5, err)
	}
	if _, err := check.Lab.Exec(ctx, peerFRRTransit, []string{"ip", "link", "set", "eth2", "down"}, nil); err != nil {
		return fail(6, err)
	}
	down, err := waitVirtualLink(ctx, check.Lab, remote, v6, 0, false)
	if err != nil {
		return fail(7, err)
	}
	if down.Changes <= changed.Changes {
		return fail(7, errors.New("transit loss did not increase virtual-link reachability changes"))
	}
	if err := waitVirtualLinkKernelRoute(ctx, check.Lab, v6, gateway, false); err != nil {
		return fail(8, err)
	}
	if err := waitVirtualLinkRoute(ctx, check.Lab, v6, false); err != nil {
		return fail(8, err)
	}
	if _, err := check.Lab.Exec(ctx, peerFRRTransit, []string{"ip", "link", "set", "eth2", "up"}, nil); err != nil {
		return fail(9, err)
	}
	if err := waitVirtualLinkKernelRoute(ctx, check.Lab, v6, gateway, true); err != nil {
		return fail(10, err)
	}
	restored, err := waitVirtualLink(ctx, check.Lab, remote, v6, 40, true)
	if err != nil {
		return fail(10, err)
	}
	if restored.Changes <= down.Changes {
		return fail(10, errors.New("transit restoration did not increase virtual-link reachability changes"))
	}
	if restored.Events[0] <= changed.Events[0] {
		return fail(10, errors.New("restored virtual adjacency completed no new database exchange"))
	}
	if err := waitVirtualLinkRoute(ctx, check.Lab, v6, true); err != nil {
		return fail(11, err)
	}
	return nil
}

func requireVirtualLinkContinuity(before, after virtualLinkObservation) error {
	if after.Changes != before.Changes {
		return errors.New("cost-only change altered virtual-link reachability")
	}
	if after.Events != before.Events {
		return fmt.Errorf("cost-only change reset an OSPF adjacency: events %v -> %v", before.Events, after.Events)
	}
	if after.Neighbor.Interface != before.Neighbor.Interface {
		return errors.New("cost-only change replaced the virtual interface")
	}
	if after.Neighbor.DDSequence != before.Neighbor.DDSequence {
		return errors.New("cost-only change restarted virtual-link database exchange")
	}
	return nil
}

func waitVirtualLink(ctx context.Context, lab interoplab.CheckerLab, remote string, v6 bool, cost float64, up bool) (virtualLinkObservation, error) {
	var lastProbeError error
	observation, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{Timeout: 90 * time.Second, Interval: time.Second, Description: "multihop virtual-link state"},
		func(ctx context.Context) (virtualLinkObservation, error) {
			value, probeErr := readVirtualLink(ctx, lab, remote, v6)
			if probeErr != nil && ctx.Err() == nil {
				lastProbeError = probeErr
			}
			return value, probeErr
		},
		func(observation virtualLinkObservation) bool {
			if !up {
				return observation.Up == 0 && observation.Down == 1 && observation.Neighbor.State != "full"
			}
			if observation.Up != 1 || observation.Down != 0 || observation.Cost != cost {
				return false
			}
			if observation.Neighbor.State != "full" || observation.Neighbor.Interface == "" || observation.Events[0] == 0 {
				return false
			}
			address := "10.200.0.6"
			if v6 {
				address = "2001:db8:200::2"
			}
			return observation.Neighbor.Address == address
		})
	if err != nil {
		return observation, fmt.Errorf("%w; last completed probe error: %v; last virtual-link observation: %+v", err, lastProbeError, observation)
	}
	return observation, nil
}

func readVirtualLink(ctx context.Context, lab interoplab.CheckerLab, remote string, v6 bool) (virtualLinkObservation, error) {
	neighborCommand := "show ospf neighbor detail"
	if v6 {
		neighborCommand = "show ospf ipv6 neighbor detail"
	}
	command := zeCommand(neighborCommand)
	answer, err := lab.Query(ctx, "ze", command, queryEnvironment("ze", command))
	if err != nil {
		return virtualLinkObservation{}, fmt.Errorf("query %s: %w", neighborCommand, err)
	}
	var neighbors []virtualLinkNeighbor
	if err := json.Unmarshal([]byte(answer), &neighbors); err != nil {
		return virtualLinkObservation{}, fmt.Errorf("decode %s reply %q: %w", neighborCommand, answer, err)
	}
	var selected virtualLinkNeighbor
	for _, neighbor := range neighbors {
		if neighbor.Area != "0.0.0.0" || neighbor.RouterID != remote {
			continue
		}
		if selected.RouterID != "" {
			return virtualLinkObservation{}, errors.New("multiple backbone neighbors claim the virtual-link endpoint")
		}
		selected = neighbor
	}
	// Exchange-done is counted before Full: read counters after the neighbor
	// snapshot so an observed Full adjacency includes its completed exchange.
	command = zeCommand("show metrics values")
	answer, err = lab.Query(ctx, "ze", command, queryEnvironment("ze", command))
	if err != nil {
		return virtualLinkObservation{Neighbor: selected}, fmt.Errorf("query metrics values: %w", err)
	}
	var envelope struct {
		Metrics string `json:"metrics"`
	}
	if err := json.Unmarshal([]byte(answer), &envelope); err != nil {
		return virtualLinkObservation{Neighbor: selected}, fmt.Errorf("decode metrics reply %q: %w", answer, err)
	}
	if envelope.Metrics == "" {
		return virtualLinkObservation{Neighbor: selected}, fmt.Errorf("metrics reply contains no Prometheus text: %s", answer)
	}
	observation, err := decodeVirtualLinkMetrics(envelope.Metrics, remote, v6)
	observation.Neighbor = selected
	return observation, err
}

func decodeVirtualLinkMetrics(answer, remote string, v6 bool) (virtualLinkObservation, error) {
	parser := expfmt.NewTextParser(model.UTF8Validation)
	families, err := parser.TextToMetricFamilies(strings.NewReader(answer))
	if err != nil {
		return virtualLinkObservation{}, err
	}
	prefix, neighborLabel := "ze_ospf", "neighbor"
	changesName := "ze_ospf_virtual_link_adjacency_changes_total"
	if v6 {
		prefix, neighborLabel = "ze_ospfv3", "remote_router_id"
		changesName = "ze_ospfv3_virtual_link_reresolves_total"
	}
	area := map[string]string{"transit_area": "0.0.0.1"}
	endpoint := map[string]string{"transit_area": "0.0.0.1", neighborLabel: remote}
	changesLabels := endpoint
	if v6 {
		changesLabels = area
	}
	var observation virtualLinkObservation
	for _, wanted := range []struct {
		name   string
		labels map[string]string
		out    *float64
	}{
		{prefix + "_virtual_link_cost", endpoint, &observation.Cost},
		{prefix + "_virtual_links", map[string]string{"transit_area": "0.0.0.1", "state": "up"}, &observation.Up},
		{prefix + "_virtual_links", map[string]string{"transit_area": "0.0.0.1", "state": "down"}, &observation.Down},
		{changesName, changesLabels, &observation.Changes},
	} {
		value, err := virtualLinkMetric(families[wanted.name], wanted.labels)
		if err != nil {
			return observation, fmt.Errorf("%s: %w", wanted.name, err)
		}
		*wanted.out = value
	}
	events := families["ze_ospf_nsm_events_total"]
	if events == nil {
		return observation, errors.New("missing OSPF NSM event counters")
	}
	for i, event := range []string{"exchange-done", "ll-down", "kill-nbr", "inactivity-timer"} {
		for _, metric := range events.Metric {
			for _, label := range metric.Label {
				if label.GetName() == "event" && label.GetValue() == event {
					observation.Events[i] += metric.GetCounter().GetValue()
				}
			}
		}
	}
	return observation, nil
}

func virtualLinkMetric(family *dto.MetricFamily, labels map[string]string) (float64, error) {
	if family == nil {
		return 0, errors.New("metric family missing")
	}
	found := false
	value := float64(0)
	for _, metric := range family.Metric {
		matched := 0
		for _, label := range metric.Label {
			if expected, ok := labels[label.GetName()]; ok && label.GetValue() == expected {
				matched++
			}
		}
		if matched != len(labels) {
			continue
		}
		if found {
			return 0, errors.New("multiple samples match the virtual-link identity")
		}
		found = true
		switch {
		case metric.Gauge != nil:
			value = metric.Gauge.GetValue()
		case metric.Counter != nil:
			value = metric.Counter.GetValue()
		default:
			return 0, errors.New("virtual-link sample is neither gauge nor counter")
		}
	}
	if !found {
		return 0, errors.New("virtual-link sample missing")
	}
	return value, nil
}

func waitVirtualLinkRoute(ctx context.Context, lab interoplab.CheckerLab, v6, present bool) error {
	const prefix = "192.0.2.1/32"
	const command = "show ip route 192.0.2.1/32 json"
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{Timeout: 90 * time.Second, Interval: time.Second, Description: "independent endpoint backbone prefix through the virtual link"},
		func(ctx context.Context) (bool, error) {
			if v6 {
				answer, err := lab.Query(ctx, peerBIRD, []string{cmdBirdc, "show route protocol vlink_ospf all"}, nil)
				if err != nil {
					return false, err
				}
				return virtualLinkBIRDRoutePresent(answer), nil
			}
			answer, err := lab.Query(ctx, peerFRR, []string{cmdVtysh, "-c", command}, nil)
			if err != nil {
				return false, err
			}
			var routes map[string][]struct {
				Protocol string `json:"protocol"`
			}
			if err := json.Unmarshal([]byte(answer), &routes); err != nil {
				return false, err
			}
			if routes == nil {
				return false, errors.New("FRR route query returned no route table")
			}
			for _, route := range routes[prefix] {
				if route.Protocol == "ospf" {
					return true, nil
				}
			}
			return false, nil
		}, func(found bool) bool { return found == present })
	return err
}

func virtualLinkBIRDRoutePresent(answer string) bool {
	for line := range strings.SplitSeq(answer, "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 && fields[0] == "2001:db8:10::1/128" && strings.Contains(line, "[vlink_ospf ") {
			return true
		}
	}
	return false
}
