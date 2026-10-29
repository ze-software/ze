// Design: docs/contributing/terminal-demos.md -- live policy topics on one running router.
// Related: register_routing.go -- shared standalone and showcase checks.

package siteterminaldemo

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"time"
)

func checkTopicRPKI() error {
	before, err := showcaseCLI("show bgp rpki status | json | no-more")
	if err != nil {
		return err
	}
	var status struct {
		Running  bool `json:"running"`
		Sessions int  `json:"sessions"`
		VRPs     int  `json:"vrp-count-ipv4"`
	}
	if err := json.Unmarshal([]byte(before), &status); err != nil {
		return err
	}
	if status.Running {
		return fmt.Errorf("RPKI already running before its live commit: %s", before)
	}
	if status.Sessions != 0 {
		return fmt.Errorf("RTR already configured before its live commit: %s", before)
	}
	if status.VRPs != 0 {
		return fmt.Errorf("VRPs already present before the live commit: %s", before)
	}
	stored, err := runZe([]string{commandConfig, commandCat, zeConfigFile}, scenarioEnv(showcaseLabID, demoPassword), nil)
	if err != nil {
		return err
	}
	if err := notContains(stored, "bgp-rpki"); err != nil {
		return err
	}
	if err := awaitPolicyTopic(func() error { return requireRPKITransit(false) }); err != nil {
		return err
	}
	connections, err := policyTransitConnections()
	if err != nil {
		return err
	}
	if _, err := showcaseCLI("request log level bgp.rpki info"); err != nil {
		return err
	}
	if err := loadShowcaseSections("topics/rpki/1-cache.conf", "topics/rpki/2-policy.conf", "topics/rpki/3-attach.conf"); err != nil {
		return err
	}
	if err := awaitTopicRPKI(stageHealthy); err != nil {
		return err
	}
	if err := awaitTopicRPKI("outage-boundary"); err != nil {
		return err
	}
	if err := runShowcaseLab("control", []string{showcaseRTRCache, controlStop}, io.Discard); err != nil {
		return err
	}
	if err := awaitTopicRPKI(stageDown); err != nil {
		return err
	}
	if err := runShowcaseLab("control", []string{showcaseRTRCache, controlStart}, io.Discard); err != nil {
		return err
	}
	if err := awaitTopicRPKI("recovered"); err != nil {
		return err
	}
	after, err := policyTransitConnections()
	if err != nil {
		return err
	}
	if after != connections {
		return fmt.Errorf("RPKI live attach or recovery reset transit: connections %d -> %d", connections, after)
	}
	return nil
}

func awaitTopicRPKI(stage string) error {
	switch stage {
	case "outage-boundary":
		return beginRPKIOutage()
	case stageHealthy:
		return awaitPolicyTopic(requireRPKIHealthy)
	case stageDown:
		return awaitPolicyTopic(func() error {
			if err := requireRPKICycleLog(false); err != nil {
				return err
			}
			// A failed poll does not invalidate the last complete set before its lease expires.
			return requireRPKIHealthy()
		})
	case "recovered":
		return awaitPolicyTopic(func() error {
			if err := requireRPKICycleLog(true); err != nil {
				return err
			}
			return requireRPKIHealthy()
		})
	default:
		return fmt.Errorf("unknown RPKI readiness stage %q", stage)
	}
}

func requireRPKIHealthy() error {
	status, err := showcaseCLI("show bgp rpki status | yaml | no-more")
	if err != nil {
		return err
	}
	if err := requireAll(status, "running: true", "sessions-synced: 1", "vrp-count-ipv4: 171"); err != nil {
		return err
	}
	cache, err := showcaseCLI("show bgp rpki cache | yaml | no-more")
	if err != nil {
		return err
	}
	if err := requireAll(cache, "127.0.0.3", "synced: true", "refresh-interval: 2", "retry-interval: 1"); err != nil {
		return err
	}
	roa, err := showcaseCLI("show bgp rpki roa 10.43.0.0/24 | yaml | no-more")
	if err != nil {
		return err
	}
	if err := requireAll(roa, "10.0.0.0/8", "65099"); err != nil {
		return err
	}
	if err := requireRPKITransit(true); err != nil {
		return err
	}
	_, err = policyTransitConnections()
	return err
}

func requireRPKITransit(validated bool) error {
	document, err := showcaseCLI("show bgp adj-rib-in | json | no-more")
	if err != nil {
		return err
	}
	var answer struct {
		Routes map[string][]rpkiDemoRoute `json:"adj-rib-in"`
	}
	if err := json.Unmarshal([]byte(document), &answer); err != nil {
		return err
	}
	routes := answer.Routes[loopbackPeerAddress]
	if len(routes) != len(rpkiDemoRoutesExpected) {
		return fmt.Errorf("transit must retain all three routes: %s", document)
	}
	for _, expected := range rpkiDemoRoutesExpected {
		found := false
		for _, route := range routes {
			if route.Key != expected.Key {
				continue
			}
			found = true
			if validated {
				if route != expected {
					return fmt.Errorf("wrong RPKI state or eligibility for %s: %s", expected.Key, document)
				}
			} else if route.Ineligible {
				return fmt.Errorf("transit route already ineligible before RPKI: %s", document)
			}
		}
		if !found {
			return fmt.Errorf("transit lacks %s: %s", expected.Key, document)
		}
	}
	return nil
}

func policyTransitConnections() (uint32, error) {
	if err := requireTransitConnectionLog(); err != nil {
		return 0, err
	}
	document, err := showcaseCLI("show bgp peer detail transit | json | no-more")
	if err != nil {
		return 0, err
	}
	var answer struct {
		Peers map[string]struct {
			Name        string `json:"name"`
			State       string `json:"state"`
			Connections uint32 `json:"connections-established"`
		} `json:"peers"`
	}
	if err := json.Unmarshal([]byte(document), &answer); err != nil {
		return 0, err
	}
	peer, found := answer.Peers[loopbackPeerAddress]
	if !found {
		return 0, fmt.Errorf("transit peer missing: %s", document)
	}
	if peer.Name != showcaseTransit {
		return 0, fmt.Errorf("wrong peer at the transit address: %s", document)
	}
	if peer.State != peerEstablished {
		return 0, fmt.Errorf("transit is not established: %s", document)
	}
	if peer.Connections == 0 {
		return 0, fmt.Errorf("transit has no established connection counter: %s", document)
	}
	return peer.Connections, nil
}

func checkTopicIRR() error {
	before, err := showcaseCLI("show bgp irr | json | no-more")
	if err != nil {
		return err
	}
	var answer struct {
		Entries []json.RawMessage `json:"entries"`
	}
	if err := json.Unmarshal([]byte(before), &answer); err != nil {
		return err
	}
	if len(answer.Entries) != 0 {
		return fmt.Errorf("IRR already configured before its live commit: %s", before)
	}
	stored, err := runZe([]string{commandConfig, commandCat, zeConfigFile}, scenarioEnv(showcaseLabID, demoPassword), nil)
	if err != nil {
		return err
	}
	if err := requireNone(stored, "AS-TEST", "bgp-filter-irr"); err != nil {
		return err
	}
	if err := loadShowcaseSections("topics/irr/1-server.conf", "topics/irr/2-as-set.conf", "topics/irr/3-import.conf"); err != nil {
		return err
	}
	if err := runShowcaseLab("control", []string{showcaseCustomerPeer, controlReannounce}, io.Discard); err != nil {
		return err
	}
	if err := awaitTopicIRR(stageHealthy); err != nil {
		return err
	}
	if err := loadShowcaseSections("topics/irr/4-wrong-as-set.conf"); err != nil {
		return err
	}
	if err := runShowcaseLab("control", []string{showcaseCustomerPeer, controlReannounce}, io.Discard); err != nil {
		return err
	}
	if err := awaitTopicIRR("wrong"); err != nil {
		return err
	}
	if err := loadShowcaseSections("topics/irr/2-as-set.conf"); err != nil {
		return err
	}
	if _, err := showcaseCLI("update bgp irr as-set AS-TEST"); err != nil {
		return err
	}
	if err := runShowcaseLab("control", []string{showcaseCustomerPeer, controlReannounce}, io.Discard); err != nil {
		return err
	}
	return awaitTopicIRR(stageHealthy)
}

func awaitTopicIRR(stage string) error {
	var asSet string
	var accepted bool
	switch stage {
	case stageHealthy:
		asSet, accepted = "AS-TEST", true
	case "wrong":
		asSet = "AS-V4ONLY"
	default:
		return fmt.Errorf("unknown IRR readiness stage %q", stage)
	}
	if err := awaitIRRCustomer(); err != nil {
		return err
	}
	return awaitPolicyTopic(func() error { return requireIRRState(asSet, accepted) })
}

func requireIRRState(asSet string, accepted bool) error {
	peers, err := showcaseCLI("show bgp peer list | json | no-more")
	if err != nil {
		return err
	}
	if err := requireShowcasePeers(peers, []string{showcaseCustomerPeer}); err != nil {
		return err
	}
	status, err := showcaseCLI("show bgp irr | yaml | no-more")
	if err != nil {
		return err
	}
	if err := requireAll(status, "127.0.0.1:4343", asSet, "status: ok"); err != nil {
		return err
	}
	prefixes, err := showcaseCLI("show bgp irr prefix customer | yaml | no-more")
	if err != nil {
		return err
	}
	check, err := showcaseCLI("show bgp irr check customer 10.0.0.0/24 | yaml | no-more")
	if err != nil {
		return err
	}
	outside, err := showcaseCLI("show bgp irr check customer 192.168.0.0/24 | yaml | no-more")
	if err != nil {
		return err
	}
	if err := contains(outside, "accepted: false"); err != nil {
		return err
	}
	document, err := showcaseCLI("show bgp adj-rib-in | json | no-more")
	if err != nil {
		return err
	}
	if err := requireIRRRoutes(document, accepted); err != nil {
		return err
	}
	if accepted {
		if err := requireAll(prefixes, "10.0.0.0/24", "2001:db8::/32"); err != nil {
			return err
		}
		return requireAll(check, "accepted: true", "matched-entry: 10.0.0.0/24")
	}
	if err := requireAll(prefixes, "AS-V4ONLY", "192.0.2.0/24"); err != nil {
		return err
	}
	if err := notContains(prefixes, "10.0.0.0/24"); err != nil {
		return err
	}
	return contains(check, "accepted: false")
}

// awaitPolicyTopic bounds asynchronous convergence without placing retries on screen.
func awaitPolicyTopic(check func() error) error {
	var last error
	for range 100 {
		last = check()
		if last == nil {
			return nil
		}
		// sleep(poll-interval): wait for the requested live policy observation.
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("policy topic did not converge: %w", last)
}

// showcaseMock invokes the registered mock namespace, distinct from test peer.
func showcaseMock(name string, args ...string) func(string, []string) ([]int, error) {
	return func(state string, environ []string) ([]int, error) {
		pid, err := startCommand(demoHarness, append([]string{"mock"}, args...), environ, filepath.Join(state, name+".log"))
		if err != nil {
			return nil, err
		}
		return []int{pid}, nil
	}
}

// Shared tokens used by the fixture runners and tape parser.
const (
	flagPort            = "--port"
	commandMigrate      = "migrate"
	nsEdge              = "edge"
	ipVeth              = "veth"
	loopbackPeerAddress = "127.0.0.2"
	tapeWaitCommand     = "Wait"
)
