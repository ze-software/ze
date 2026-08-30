package fixture

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const plugin01PollDelay = 250 * time.Millisecond

type plugin01Scenario func(context.Context, *sdk.Plugin) error

type plugin01Observer struct {
	authName string
	scenario plugin01Scenario
}

func init() {
	observers := map[string]plugin01Observer{
		"plugin/add-remove":                         {pluginNameAddRemove, plugin01AddRemove},
		"plugin/adj-rib-in-query":                   {"adj-rib-query-test", plugin01AdjRIBInQuery},
		"plugin/adj-rib-in-replay-addpath-source":   {"replay-addpath", plugin01ReplayAddPath},
		"plugin/adj-rib-in-replay-on-peerup":        {"replay-relay", plugin01ReplayOnPeerUp},
		"plugin/adj-rib-in-replay-rfc2545-next-hop": {"replay-nh2545", plugin01ReplayRFC2545},
		"plugin/adj-rib-in-replay-unowned-peer":     {"unowned-peer", plugin01ReplayUnownedPeer},
		"plugin/anomaly-observe-show":               {"anomaly-observe-show-test", plugin01AnomalyObserve},
		"plugin/anomaly-shape-shadow":               {"anomaly-shape-test", plugin01AnomalyShape},
		"plugin/anomaly-show":                       {"anomaly-show-test", plugin01AnomalyShow},
		"plugin/answer-many-records":                {"answer-records", plugin01AnswerManyRecords},
		"plugin/answer-unconditional-second":        {"answer-second", plugin01AnswerUnconditionalSecond},
		"plugin/api-bgp-summary":                    {"summary-test", plugin01APIBGPSummary},
		"plugin/api-cache-forward":                  {"cache-forward-test", plugin01APICacheForward},
		"plugin/api-cache-ops":                      {"cache-ops-test", plugin01APICacheOps},
		"plugin/api-commit-lifecycle":               {"commit-lifecycle-test", plugin01APICommitLifecycle},
		"plugin/api-commit-workflow":                {"commit-workflow-test", plugin01APICommitWorkflow},
		"plugin/api-config-commit-reject":           {"api-commit-reject", plugin01APIConfigCommitReject},
		"plugin/api-doctor-check-checker":           {"doctor-checker", plugin01APIDoctorChecker},
		"plugin/api-peer-capabilities":              {"peer-caps-test", plugin01APIPeerCapabilities},
		"plugin/api-peer-list":                      {"peer-list-test", plugin01APIPeerList},
		"plugin/api-peer-pause-resume":              {"peer-pause-test", plugin01APIPeerPauseResume},
		"plugin/api-peer-prefix-update":             {"prefix-update-test", plugin01APIPeerPrefixUpdate},
		"plugin/api-peer-remove":                    {"peer-remove-test", plugin01APIPeerRemove},
		"plugin/api-peer-show":                      {"peer-show-test", plugin01APIPeerShow},
		"plugin/api-raw":                            {"raw-test", plugin01APIRaw},
		"plugin/api-rib-in-clear":                   {"rib-in-clear-test", plugin01APIRIBInClear},
		"plugin/api-rib-in-show":                    {"rib-in-show-test", plugin01APIRIBInShow},
	}
	for fixtureName, observer := range observers {
		Register(fixtureName, func(ctx context.Context, _ []string) error {
			return Observe(ctx, observer.authName, sdk.Registration{}, ObserverScenario(observer.scenario))
		})
	}

	Register("plugin/answer-first-bounds-long-walk", plugin01AnswerFirstBoundsLongWalk)
	Register("plugin/answer-payload-unchanged", plugin01AnswerPayloadUnchanged)
	Register("plugin/answer-single-record", plugin01AnswerSingleRecord)
	Register("plugin/answer-truncation-detected", plugin01AnswerTruncationDetected)
	Register("plugin/answer-unconditional-first", plugin01AnswerUnconditionalFirst)
	Register("plugin/api-doctor-check-provider", plugin01APIDoctorProvider)
}

func plugin01DispatchMap(ctx context.Context, plugin *sdk.Plugin, command string) (map[string]any, string, error) {
	status, raw, err := plugin.DispatchCommand(ctx, command)
	if err != nil {
		if status == rpc.StatusError || strings.HasPrefix(err.Error(), "rpc error:") {
			return map[string]any{fieldError: err.Error()}, rpc.StatusError, nil
		}
		return nil, status, err
	}
	if len(raw) == 0 {
		return nil, status, nil
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, status, err
	}
	if encoded, ok := value.(string); ok && encoded != "" {
		var decoded any
		if err := json.Unmarshal([]byte(encoded), &decoded); err == nil {
			value = decoded
		}
	}
	if data, ok := value.(map[string]any); ok {
		return data, status, nil
	}
	return map[string]any{fieldValue: value}, status, nil
}

func plugin01RequireDone(ctx context.Context, plugin *sdk.Plugin, command string) (map[string]any, error) {
	data, status, err := plugin01DispatchMap(ctx, plugin, command)
	if err != nil {
		return data, fmt.Errorf("%s: status=%s: %w", command, status, err)
	}
	if status != rpc.StatusDone {
		return data, fmt.Errorf("%s: status=%s data=%v", command, status, data)
	}
	return data, nil
}

func plugin01Number(value any) float64 {
	switch number := value.(type) {
	case float64:
		return number
	case json.Number:
		value, _ := number.Float64()
		return value
	case int:
		return float64(number)
	case uint64:
		return float64(number)
	default:
		return 0
	}
}

func plugin01Object(value any) map[string]any {
	object, _ := value.(map[string]any)
	return object
}

func plugin01Array(value any) []any {
	array, _ := value.([]any)
	return array
}

func plugin01PeerRows(data map[string]any) map[string]any {
	return plugin01Object(data["peers"])
}

func plugin01PeerCounter(ctx context.Context, plugin *sdk.Plugin, peer, counter string) float64 {
	command := "show bgp peer * detail"
	if peer != "*" && peer != "" {
		command = "show bgp peer " + peer + " detail"
	}
	data, _, err := plugin01DispatchMap(ctx, plugin, command)
	if err != nil {
		return -1
	}
	rows := plugin01PeerRows(data)
	if peer != "*" && peer != "" {
		if row := plugin01Object(rows[peer]); row != nil {
			return plugin01Number(row[counter])
		}
		for _, value := range rows {
			row := plugin01Object(value)
			if row["name"] == peer || row["address"] == peer || row["peer"] == peer {
				return plugin01Number(row[counter])
			}
		}
		if len(rows) == 1 {
			for _, value := range rows {
				return plugin01Number(plugin01Object(value)[counter])
			}
		}
		return 0
	}
	var total float64
	for _, value := range rows {
		total += plugin01Number(plugin01Object(value)[counter])
	}
	return total
}

func plugin01WaitCounter(ctx context.Context, plugin *sdk.Plugin, peer, counter string, minimum float64, attempts int) bool {
	return Poll(ctx, attempts, plugin01PollDelay, func() bool {
		return plugin01PeerCounter(ctx, plugin, peer, counter) >= minimum
	})
}

func plugin01WaitPeers(ctx context.Context, plugin *sdk.Plugin, count, attempts int) bool {
	return plugin01WaitCounter(ctx, plugin, "*", "connections-established", float64(count), attempts)
}

func plugin01Quiesce(ctx context.Context, plugin *sdk.Plugin) error {
	_, err := plugin01RequireDone(ctx, plugin, "request quiesce")
	return err
}

func plugin01TotalRoutes(ctx context.Context, plugin *sdk.Plugin, attempts int) bool {
	return Poll(ctx, attempts, plugin01PollDelay, func() bool {
		data, status, err := plugin01DispatchMap(ctx, plugin, "show bgp adj-rib-in status")
		if err != nil || status != rpc.StatusDone {
			return false
		}
		return plugin01Number(data["total-routes"]) > 0
	})
}

func plugin01RouteUpdates(ctx context.Context, plugin *sdk.Plugin, peer string) float64 {
	return plugin01PeerCounter(ctx, plugin, peer, "updates-sent") - plugin01PeerCounter(ctx, plugin, peer, "eor-sent")
}

func plugin01AddRemove(ctx context.Context, plugin *sdk.Plugin) error {
	commands := []string{
		cmdAnnounceFirstPrefix,
		"update text nhop 101.1.101.1 nlri ipv4/unicast add 1.1.0.0/25",
	}
	for _, command := range commands {
		if _, _, err := plugin.UpdateRoute(ctx, "*", command); err != nil {
			return err
		}
	}
	if err := plugin01Quiesce(ctx, plugin); err != nil {
		return err
	}
	for _, command := range []string{
		"update text nlri ipv4/unicast del 1.1.0.0/24",
		"update text nlri ipv4/unicast del 1.1.0.0/25",
	} {
		if _, _, err := plugin.UpdateRoute(ctx, "*", command); err != nil {
			return err
		}
	}
	return plugin01Quiesce(ctx, plugin)
}

func plugin01AnomalyObserve(ctx context.Context, plugin *sdk.Plugin) error {
	data, err := plugin01RequireDone(ctx, plugin, "show anomaly observe")
	if err != nil {
		return err
	}
	incidents, exists := data["incidents"]
	if data["enabled"] != true || !exists || plugin01Array(incidents) == nil || len(plugin01Array(incidents)) != 0 || plugin01Number(data["active-count"]) != 0 {
		return fmt.Errorf("unexpected anomaly observe response: %v", data)
	}
	if err := plugin01WaitEORAndQuiesce(ctx, plugin); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "OK: show anomaly observe enabled=true active-count=0 incidents=0")
	return nil
}

func plugin01AnomalyShape(ctx context.Context, plugin *sdk.Plugin) error {
	data, err := plugin01RequireDone(ctx, plugin, "show anomaly shape")
	if err != nil {
		return err
	}
	if data["enabled"] != true || data["mode"] != "shadow" || plugin01Number(data["armed-count"]) != 0 {
		return fmt.Errorf("unexpected anomaly shape response: %v", data)
	}
	if err := plugin01WaitEORAndQuiesce(ctx, plugin); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "OK: show anomaly shape mode=shadow armed=0")
	return nil
}

func plugin01AnomalyShow(ctx context.Context, plugin *sdk.Plugin) error {
	data, err := plugin01RequireDone(ctx, plugin, "show anomaly detect")
	if err != nil {
		return err
	}
	incidents, exists := data["incidents"]
	if !exists || plugin01Array(incidents) == nil || data["enabled"] != true {
		return fmt.Errorf("unexpected anomaly detect response: %v", data)
	}
	if err := plugin01WaitEORAndQuiesce(ctx, plugin); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "OK: show anomaly detect enabled=true incidents=%d\n", len(plugin01Array(incidents)))
	return nil
}
