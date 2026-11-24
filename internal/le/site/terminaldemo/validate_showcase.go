// Design: docs/contributing/terminal-demos.md -- the lab a user would build
// Related: scenarios_showcase.go -- runShowcaseLab, the lab this checks

package siteterminaldemo

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"time"

	"github.com/ze-software/ze/internal/component/config"
)

// validateShowcaseLab proves the fixtures are ready on a topic-free base and
// selective controls do not replace the shown daemon. Topic validators then
// prove their live commits and traffic; this check does not enable any topic.
func validateShowcaseLab() (err error) {
	defer func() {
		if stopErr := runShowcaseLab(commandStop, nil, io.Discard); err == nil {
			err = stopErr
		}
	}()
	if err := runShowcaseLab(actionPrepare, nil, io.Discard); err != nil {
		return err
	}
	if err := runShowcaseLab(commandStart, nil, io.Discard); err != nil {
		return err
	}
	pid, err := showcasePID()
	if err != nil {
		return err
	}
	if err := requireShowcaseBase(); err != nil {
		return err
	}
	// These are real lifecycle operations, not declarations inspected as proof.
	for _, control := range [][]string{
		{showcaseRTRCache, controlStop}, {showcaseRTRCache, controlStart},
		{showcaseCustomerPeer, controlReannounce}, {showcaseCustomerPeer, controlReannounce},
		{showcaseFRR, controlDown}, {showcaseFRR, "up"},
		{showcaseKeepalived, controlStop}, {showcaseKeepalived, controlStart},
		{showcaseTrackedUplink, controlDown}, {showcaseTrackedUplink, "up"},
		{showcaseTraffic, "burst"}, {showcaseTraffic, controlStop},
	} {
		if err := runShowcaseLab("control", control, io.Discard); err != nil {
			return err
		}
		if err := requireShowcasePID(pid); err != nil {
			return err
		}
	}
	if err := awaitShowcasePeers(scenarioEnv(showcaseLabID, demoPassword), showcaseTransit, showcaseFRR, showcaseCustomerPeer); err != nil {
		return err
	}
	for _, fixture := range showcaseLabFixtures() {
		for _, ready := range fixture.ready {
			if err := awaitReadiness(fixture.name, ready); err != nil {
				return err
			}
		}
	}
	if err := requireShowcaseBase(); err != nil {
		return err
	}
	return requireShowcasePID(pid)
}

// awaitShowcasePeers matches each named peer's own state. An unrelated
// established session cannot stand in for a missing or failed lab peer.
func awaitShowcasePeers(environ []string, names ...string) error {
	var last error
	for range 300 {
		answer, err := cli(environ, "show bgp peer list | json | no-more")
		if err == nil {
			err = requireShowcasePeers(answer, names)
		}
		if err == nil {
			return nil
		}
		// A command stopped at its deadline hangs again on the next attempt.
		if errors.Is(err, errCommandDeadline) {
			return err
		}
		last = err
		// sleep(poll-interval): each requested peer must report Established.
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("showcase BGP readiness: %w", last)
}

func requireShowcasePeers(document string, names []string) error {
	var answer struct {
		Peers map[string]struct {
			Name  string `json:"name"`
			State string `json:"state"`
		} `json:"peers"`
	}
	if err := json.Unmarshal([]byte(document), &answer); err != nil {
		return fmt.Errorf("decode showcase peers: %w; answer: %s", err, document)
	}
	for _, name := range names {
		found := false
		for _, peer := range answer.Peers {
			if peer.Name == name {
				if peer.State != peerEstablished {
					return fmt.Errorf("showcase peer %s is %s, not established", name, peer.State)
				}
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("showcase peer %s is absent: %s", name, document)
		}
	}
	return nil
}

// requireShowcaseBase reads the stored configuration the daemon actually
// started, not the source fixture or a hash. No topic may be preconfigured.
func requireShowcaseBase() error {
	text, err := runZe([]string{commandConfig, commandCat, zeConfigFile}, scenarioEnv(showcaseLabID, demoPassword), nil)
	if err != nil {
		return err
	}
	tree, err := config.ParseTreeForValidation(text)
	if err != nil {
		return fmt.Errorf("parse running showcase base: %w", err)
	}
	if err := requireShowcaseBaseTree(tree); err != nil {
		return err
	}
	// The probe must have run and gone unanswered. A ping that could not run
	// fails too, and says nothing about whether the hosts can communicate.
	if err := requireTopicIPsecPingFailure(); err != nil {
		return fmt.Errorf("protected hosts before the shown VPN is configured: %w", err)
	}
	return nil
}

// requireShowcaseBaseTree permits baseline route storage, not topic bindings.
// What a topic adds is not listed here: each live topic declares it at its
// registration, and this runs every declaration, so a new topic is guarded
// without an edit to this function. Transit alone needs the RIB's Route
// Refresh catch-up subscriptions.
func requireShowcaseBaseTree(tree *config.Tree) error {
	if err := requireLiveTopicsAbsent(scenarios, tree); err != nil {
		return err
	}
	bgp := tree.GetContainer("bgp")
	if bgp == nil {
		return fmt.Errorf("showcase base has no BGP configuration")
	}
	for name, peer := range bgp.GetList("peer") {
		if attach := peer.GetContainer("attach"); attach != nil {
			for process, binding := range attach.GetList("process") {
				if process != "bgp-adj-rib-in" && (process != pluginBGPRIB || name != showcaseTransit) {
					return fmt.Errorf("showcase base peer %s pre-binds %s", name, process)
				}
				if process == pluginBGPRIB {
					receive, send := binding.GetSlice("receive"), binding.GetSlice("send")
					if len(receive) != 3 || !slices.Contains(receive, "update") || !slices.Contains(receive, "state") || !slices.Contains(receive, "refresh") || len(send) != 1 || send[0] != "update" {
						return fmt.Errorf("showcase base peer %s has non-baseline bgp-rib subscriptions: receive %v, send %v", name, receive, send)
					}
				}
			}
		}
	}
	if bus := tree.GetContainer("bus"); bus != nil {
		for name, ep := range bus.GetList("endpoint") {
			if name != "bgp-adj-rib-in" && name != pluginBGPRIB {
				return fmt.Errorf("showcase base preconfigures endpoint %s", name)
			}
			if name == pluginBGPRIB {
				if internal, _ := ep.Get("internal"); internal != pluginBGPRIB {
					return fmt.Errorf("showcase base endpoint bgp-rib uses %q, not bgp-rib", internal)
				}
			}
		}
	}
	return nil
}
