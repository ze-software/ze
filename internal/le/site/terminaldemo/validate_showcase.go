// Design: docs/contributing/terminal-demos.md -- the lab a user would build
// Related: scenarios_showcase.go -- runShowcaseLab, the lab this checks

package siteterminaldemo

import (
	"encoding/json"
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
		{"rtr-cache", "stop"}, {"rtr-cache", "start"},
		{"customer", "reannounce"}, {"customer", "reannounce"},
		{"frr", "down"}, {"frr", "up"},
		{"keepalived", "stop"}, {"keepalived", "start"},
		{"tracked-uplink", "down"}, {"tracked-uplink", "up"},
		{"traffic", "burst"}, {"traffic", "stop"},
	} {
		if err := runShowcaseLab("control", control, io.Discard); err != nil {
			return err
		}
		if err := requireShowcasePID(pid); err != nil {
			return err
		}
	}
	if err := awaitShowcasePeers(scenarioEnv(showcaseLabID, demoPassword), "transit", "frr", "customer"); err != nil {
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
				if peer.State != "established" {
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
	if err := runShowcaseLab("control", []string{"vpn", "ping"}, io.Discard); err == nil {
		return fmt.Errorf("protected hosts can communicate before the shown VPN is configured")
	}
	return nil
}

// requireShowcaseBaseTree permits baseline route storage, not topic bindings.
// Transit alone needs the RIB's Route Refresh catch-up subscriptions.
func requireShowcaseBaseTree(tree *config.Tree) error {
	for _, path := range []string{"vpn", "bfd", "ospf", "traffic", "bgp/rpki", "bgp/irr"} {
		if tree.GetContainerPath(path) != nil {
			return fmt.Errorf("showcase base already configures topic %s", path)
		}
	}
	bgp := tree.GetContainer("bgp")
	if bgp == nil {
		return fmt.Errorf("showcase base has no BGP configuration")
	}
	for name, peer := range bgp.GetList("peer") {
		if connection := peer.GetContainer("connection"); connection != nil {
			if _, present := connection.Get("bfd"); present {
				return fmt.Errorf("showcase base peer %s already configures BFD", name)
			}
		}
		if attach := peer.GetContainer("attach"); attach != nil {
			for process, binding := range attach.GetList("process") {
				if process != "bgp-adj-rib-in" && (process != "bgp-rib" || name != "transit") {
					return fmt.Errorf("showcase base peer %s pre-binds %s", name, process)
				}
				if process == "bgp-rib" {
					receive, send := binding.GetSlice("receive"), binding.GetSlice("send")
					if len(receive) != 3 || !slices.Contains(receive, "update") || !slices.Contains(receive, "state") || !slices.Contains(receive, "refresh") || len(send) != 1 || send[0] != "update" {
						return fmt.Errorf("showcase base peer %s has non-baseline bgp-rib subscriptions: receive %v, send %v", name, receive, send)
					}
				}
			}
		}
	}
	if plugins := tree.GetContainer("plugin"); plugins != nil {
		for name, plugin := range plugins.GetList("internal") {
			if name != "bgp-adj-rib-in" && name != "bgp-rib" {
				return fmt.Errorf("showcase base preconfigures plugin %s", name)
			}
			if name == "bgp-rib" {
				if use, _ := plugin.Get("use"); use != "bgp-rib" {
					return fmt.Errorf("showcase base plugin bgp-rib uses %q, not bgp-rib", use)
				}
			}
		}
	}
	if interfaces := tree.GetContainer("interface"); interfaces != nil {
		if len(interfaces.GetList("xfrm")) != 0 {
			return fmt.Errorf("showcase base already configures an XFRM interface")
		}
		for _, kind := range interfaces.ListNames() {
			for name, iface := range interfaces.GetList(kind) {
				for _, unit := range iface.GetList("unit") {
					for _, family := range []string{"ipv4", "ipv6"} {
						if addresses := unit.GetContainer(family); addresses != nil {
							if addresses.GetContainer("vrrp") != nil {
								return fmt.Errorf("showcase base interface %s already configures VRRP", name)
							}
						}
					}
				}
			}
		}
	}
	return nil
}
