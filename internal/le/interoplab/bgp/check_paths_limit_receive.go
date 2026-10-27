// Design: docs/architecture/testing/interop.md -- the receive ceiling against a sender that ignores PATHS-LIMIT.
// Related: check_paths_limit.go -- the FRR scenario, whose sender honors the limit.
// Related: register_paths_limit.go -- registry entry that dispatches this checker.
package bgp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"

	"github.com/ze-software/ze/internal/le/interoplab"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

const (
	pathsLimitReceiveScenario = "bgp-paths-limit-receive-bird"
	// pathsLimitReceivePrefix is the prefix BIRD offers four paths of, one per
	// static protocol in bird.conf.
	pathsLimitReceivePrefix = "10.70.0.0/24"
	// pathsLimitReceiveOtherPrefix has one path. It MUST install whatever the
	// ceiling does to the prefix it may share an UPDATE with.
	pathsLimitReceiveOtherPrefix = "10.71.0.0/24"
	// pathsLimitReceiveOffered is the number of paths bird.conf gives
	// pathsLimitReceivePrefix, and pathsLimitReceiveCeiling the limit ze.conf
	// advertises for ipv4/unicast.
	pathsLimitReceiveOffered = 4
	pathsLimitReceiveCeiling = 2
	// pathsLimitRefusedMetric counts every path the receive ceiling refused
	// (reportPathsLimitRefusals, internal/component/bgp/reactor).
	pathsLimitRefusedMetric = "ze_bgp_paths_limit_refused_total"
)

// pathsLimitReceiveObservation is one reading of what Ze did with the paths
// BIRD offered: the paths its Adj-RIB-In holds for each prefix, and the
// refusals it counted for the session.
type pathsLimitReceiveObservation struct {
	held     int
	other    int
	refused  int
	accounts bool
	// rows is Ze's received-routes answer, cut to rowsShownMax octets, so a
	// wait that times out shows what Ze held rather than only the counts.
	rows string
}

// rowsShownMax bounds the received-routes answer a failure message carries.
const rowsShownMax = 2048

// String renders the observation a timed-out wait prints as its last output.
func (o pathsLimitReceiveObservation) String() string {
	return fmt.Sprintf("held %d of %s, held %d of %s, refused %d; received rows: %s",
		o.held, pathsLimitReceivePrefix, o.other, pathsLimitReceiveOtherPrefix, o.refused, o.rows)
}

// checkPathsLimitReceive proves, against BIRD 3, that Ze's receive ceiling holds
// exactly the advertised number of paths of an over-offered prefix, counts each
// path it refused, installs the other prefix and keeps the session up.
//
// The wait ends once every offered path is accounted for, held or refused, so
// the held count is read after the last offer, never before it. That makes the
// red of a removed ceiling a reading of four held paths, which is also the
// evidence that BIRD offered four (assumption A-8 of the spec).
func checkPathsLimitReceive(ctx context.Context, check *interoplab.CheckContext) error {
	fail := func(assertion int, cause error) error {
		return checkerFailure(ctx, check.Lab, pathsLimitReceiveScenario, assertion, cause)
	}
	if !check.Network.IPv4.IsValid() {
		return fail(1, errors.New("receive ceiling scenario has no selected IPv4 network"))
	}
	birdAddress := networkHostAddress(check.Network, 4)

	// Assertion 1. The session is up before anything is counted.
	session := operation{kind: opBIRDSession, argument: birdZeProtocol}
	if err := runOperation(ctx, check.Network, check.Lab, &session); err != nil {
		return fail(1, err)
	}

	// Assertion 2. Every offered path reached a verdict.
	observation, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: 60 * time.Second, Interval: time.Second, Description: "every offered path held or refused",
	}, func(probe context.Context) (pathsLimitReceiveObservation, error) {
		return observePathsLimitReceive(probe, check.Lab, birdAddress)
	}, func(observed pathsLimitReceiveObservation) bool {
		return observed.accounts
	})
	if err != nil {
		return fail(2, err)
	}

	// Assertion 3. The ceiling held, counted, and spared the other prefix.
	if err := requirePathsLimitReceive(observation); err != nil {
		return fail(3, err)
	}

	// Assertion 4. A refusal is not an error: the session survived it.
	if err := runOperation(ctx, check.Network, check.Lab, &session); err != nil {
		return fail(4, err)
	}
	return nil
}

// observePathsLimitReceive reads Ze's Adj-RIB-In and its refusal counter once.
func observePathsLimitReceive(ctx context.Context, lab interoplab.CheckerLab, birdAddress string) (pathsLimitReceiveObservation, error) {
	command := zeCommand("show bgp rib received")
	routes, err := lab.Query(ctx, "ze", command, queryEnvironment("ze", command))
	if err != nil {
		return pathsLimitReceiveObservation{}, fmt.Errorf("query received routes: %w", err)
	}
	command = zeCommand("show metrics values")
	metrics, err := lab.Query(ctx, "ze", command, queryEnvironment("ze", command))
	if err != nil {
		return pathsLimitReceiveObservation{}, fmt.Errorf("query metrics values: %w", err)
	}
	return decodePathsLimitReceive(routes, metrics, birdAddress)
}

// decodePathsLimitReceive counts the received rows BIRD's session holds for each
// prefix, and reads the refusal counter of that session from the metrics reply.
func decodePathsLimitReceive(routesJSON, metricsJSON, birdAddress string) (pathsLimitReceiveObservation, error) {
	var routes []struct {
		Peer      string `json:"peer"`
		Direction string `json:"direction"`
		Prefix    string `json:"prefix"`
		Family    string `json:"family"`
	}
	if err := json.Unmarshal([]byte(routesJSON), &routes); err != nil {
		return pathsLimitReceiveObservation{}, fmt.Errorf("decode received routes: %w", err)
	}
	if routes == nil {
		return pathsLimitReceiveObservation{}, errors.New("ze received RIB query omitted routes")
	}
	var observation pathsLimitReceiveObservation
	observation.rows = routesJSON[:min(len(routesJSON), rowsShownMax)]
	for _, route := range routes {
		if route.Peer != birdAddress {
			continue
		}
		if route.Direction != rpc.DirectionReceived.String() {
			continue
		}
		if route.Family != zeIPv4Unicast {
			continue
		}
		// bgp-rib names an ADD-PATH row "10.70.0.0/24 [pathID=4]": the
		// identifier is what keeps the paths of one prefix apart.
		prefix, _, _ := strings.Cut(route.Prefix, " ")
		switch prefix {
		case pathsLimitReceivePrefix:
			observation.held++
		case pathsLimitReceiveOtherPrefix:
			observation.other++
		}
	}
	refused, err := pathsLimitRefusedCount(metricsJSON, birdAddress)
	if err != nil {
		return pathsLimitReceiveObservation{}, err
	}
	observation.refused = refused
	observation.accounts = observation.held+observation.refused >= pathsLimitReceiveOffered && observation.other > 0
	return observation, nil
}

// pathsLimitRefusedCount reads ze_bgp_paths_limit_refused_total for the session
// and ipv4/unicast. The counter vector emits a series only once it was first
// stepped, so an absent series is a count of zero refusals, never a failure to
// read: an absent or empty metrics reply is the failure, and it is refused.
func pathsLimitRefusedCount(metricsJSON, peer string) (int, error) {
	var envelope struct {
		Metrics string `json:"metrics"`
	}
	if err := json.Unmarshal([]byte(metricsJSON), &envelope); err != nil {
		return 0, fmt.Errorf("decode metrics reply: %w", err)
	}
	if envelope.Metrics == "" {
		return 0, fmt.Errorf("metrics reply contains no Prometheus text: %s", metricsJSON)
	}
	parser := expfmt.NewTextParser(model.UTF8Validation)
	families, err := parser.TextToMetricFamilies(strings.NewReader(envelope.Metrics))
	if err != nil {
		return 0, fmt.Errorf("parse metrics: %w", err)
	}
	family, present := families[pathsLimitRefusedMetric]
	if !present {
		// refuse-by-default: guards requirePathsLimitReceive -- no session has stepped the counter yet, so nothing was refused.
		return 0, nil
	}
	refused := -1
	for _, metric := range family.Metric {
		labels := make(map[string]string, len(metric.Label))
		for _, label := range metric.Label {
			labels[label.GetName()] = label.GetValue()
		}
		if labels["peer"] != peer {
			continue
		}
		if labels["family"] != zeIPv4Unicast {
			continue
		}
		if refused >= 0 {
			return 0, fmt.Errorf("%s has two series for peer %s %s", pathsLimitRefusedMetric, peer, zeIPv4Unicast)
		}
		if metric.Counter == nil {
			return 0, fmt.Errorf("%s for peer %s is not a counter", pathsLimitRefusedMetric, peer)
		}
		refused = int(metric.Counter.GetValue())
	}
	if refused < 0 {
		// refuse-by-default: guards requirePathsLimitReceive -- the vector holds another session's series only, so this session refused nothing yet.
		return 0, nil
	}
	return refused, nil
}

// requirePathsLimitReceive judges one complete observation: exactly the
// ceiling held, every other offered path counted as refused, and the other
// prefix installed once.
func requirePathsLimitReceive(observed pathsLimitReceiveObservation) error {
	if observed.held != pathsLimitReceiveCeiling {
		return fmt.Errorf("ze holds %d paths of %s, want the advertised ceiling %d (refused %d)",
			observed.held, pathsLimitReceivePrefix, pathsLimitReceiveCeiling, observed.refused)
	}
	if observed.refused != pathsLimitReceiveOffered-pathsLimitReceiveCeiling {
		return fmt.Errorf("%s counts %d refusals, want %d: one for each path offered over the ceiling",
			pathsLimitRefusedMetric, observed.refused, pathsLimitReceiveOffered-pathsLimitReceiveCeiling)
	}
	if observed.other != 1 {
		return fmt.Errorf("ze holds %d paths of %s, want 1", observed.other, pathsLimitReceiveOtherPrefix)
	}
	return nil
}
