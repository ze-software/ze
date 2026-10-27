// Design: docs/contributing/terminal-demos.md -- observations of the one running showcase lab
// Related: validate_topics.go -- shared SSH candidate and scenario lifetime

package siteterminaldemo

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/component/config"
)

func networkJSON[T any](command string) (T, error) {
	var result T
	answer, err := showcaseCLI(command + " | json | no-more")
	if err != nil {
		return result, err
	}
	if err := json.Unmarshal([]byte(answer), &result); err != nil {
		return result, fmt.Errorf("decode %s: %w; answer: %s", command, err, answer)
	}
	return result, nil
}

func awaitNetworkObservation(label string, timeout time.Duration, observe func() error) error {
	deadline := time.Now().Add(timeout)
	for {
		err := observe()
		if err == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("%s: %w", label, err)
		}
		// sleep(poll-interval): wait for the real protocol or kernel observation.
		time.Sleep(100 * time.Millisecond)
	}
}

func controlNetworkFixture(fixture, verb string) error {
	return runShowcaseLab("control", []string{fixture, verb}, io.Discard)
}

// requireNetworkTopicAbsent checks the active store before the live commit,
// including peer bindings that could otherwise conceal a pre-enabled feature.
func requireNetworkTopicAbsent(topic string) error {
	text, err := runZe([]string{commandConfig, commandCat, zeConfigFile}, scenarioEnv(showcaseLabID, demoPassword), nil)
	if err != nil {
		return err
	}
	tree, err := config.ParseTreeForValidation(text)
	if err != nil {
		return fmt.Errorf("parse running configuration: %w", err)
	}
	if topic != "vrrp" && tree.GetContainerPath(topic) != nil {
		return fmt.Errorf("%s is configured before its live commit", topic)
	}
	if topic == "bfd" {
		if bgp := tree.GetContainer("bgp"); bgp != nil {
			for name, peer := range bgp.GetList("peer") {
				if connection := peer.GetContainer("connection"); connection != nil {
					if _, present := connection.Get("bfd"); present {
						return fmt.Errorf("peer %s already binds BFD", name)
					}
				}
			}
		}
	}
	if topic == "vrrp" {
		if interfaces := tree.GetContainer("interface"); interfaces != nil {
			for _, kind := range interfaces.ListNames() {
				for name, iface := range interfaces.GetList(kind) {
					for _, unit := range iface.GetList("unit") {
						for _, family := range []string{"ipv4", "ipv6"} {
							if addresses := unit.GetContainer(family); addresses != nil && addresses.GetContainer("vrrp") != nil {
								return fmt.Errorf("interface %s already configures VRRP", name)
							}
						}
					}
				}
			}
		}
	}
	return nil
}

type networkMetricAnswer struct {
	Series []struct {
		Line string `json:"line"`
	} `json:"series"`
}

// networkMetric reads the selected producer's samples, never treating a command
// or parse error as a zero counter. An empty series is a legitimate zero.
func networkMetric(command string) (float64, error) {
	answer, err := networkJSON[networkMetricAnswer](command)
	if err != nil {
		return 0, err
	}
	return sumNetworkMetric(answer)
}

func sumNetworkMetric(answer networkMetricAnswer) (float64, error) {
	var sum float64
	for _, series := range answer.Series {
		fields := strings.Fields(series.Line)
		if len(fields) != 2 {
			return 0, fmt.Errorf("unexpected metric series %q", series.Line)
		}
		value, err := strconv.ParseFloat(fields[1], 64)
		if err != nil {
			return 0, fmt.Errorf("parse metric %q: %w", series.Line, err)
		}
		sum += value
	}
	return sum, nil
}
