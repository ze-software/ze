// Design: docs/contributing/terminal-demos.md -- evidence must belong to the demonstrated failure.
// Related: scenarios_routing.go -- live RPKI and IRR checks.

package siteterminaldemo

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/component/config"
)

const (
	rpkiFailedPoll    = "rtr: cache did not answer, trying the next one in preference order"
	rpkiCompletedSync = "rtr: sync complete"
	irrHealthyControl = "ipv4/unicast:10.0.1.0/24"
	irrWrongControl   = "ipv4/unicast:192.0.2.0/24"
)

type policyLogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
}

// rpkiOutage persists across the separate ze-demo invocations in the authored tape.
// A new boundary clears Failure, so a previous chapter/run cannot supply recovery.
type rpkiOutage struct {
	Boundary time.Time `json:"boundary"`
	Failure  time.Time `json:"failure"`
}

func rpkiLogEntries() ([]policyLogEntry, error) {
	document, err := showcaseCLI("show log recent component bgp.rpki count 100 | json | no-more")
	if err != nil {
		return nil, err
	}
	var answer struct {
		Entries []policyLogEntry `json:"entries"`
	}
	if err := json.Unmarshal([]byte(document), &answer); err != nil {
		return nil, err
	}
	return answer.Entries, nil
}

func beginRPKIOutage() error {
	entries, err := rpkiLogEntries()
	if err != nil {
		return err
	}
	// The command producer exports daemon timestamps to whole seconds. Exclude
	// the entire observation second as well as every already-visible timestamp:
	// equal-precision events cannot prove they happened after this boundary.
	boundary := time.Now().UTC().Truncate(time.Second)
	for _, entry := range entries {
		if entry.Timestamp.After(boundary) {
			boundary = entry.Timestamp
		}
	}
	return saveRPKIOutage(rpkiOutage{Boundary: boundary})
}

func saveRPKIOutage(cycle rpkiOutage) error {
	data, err := json.Marshal(cycle)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(demoState(showcaseLabID), "rpki-outage.json"), data, 0o600)
}

func requireRPKICycleLog(recovered bool) error {
	data, err := os.ReadFile(filepath.Join(demoState(showcaseLabID), "rpki-outage.json"))
	if err != nil {
		return fmt.Errorf("RPKI outage has no captured pre-stop boundary: %w", err)
	}
	var cycle rpkiOutage
	if err := json.Unmarshal(data, &cycle); err != nil {
		return err
	}
	entries, err := rpkiLogEntries()
	if err != nil {
		return err
	}
	observed, err := rpkiCycleEvent(entries, cycle, recovered)
	if err != nil {
		return err
	}
	if recovered {
		return nil
	}
	cycle.Failure = observed
	return saveRPKIOutage(cycle)
}

func rpkiCycleEvent(entries []policyLogEntry, cycle rpkiOutage, recovered bool) (time.Time, error) {
	if cycle.Boundary.IsZero() {
		return time.Time{}, fmt.Errorf("RPKI outage boundary is missing")
	}
	after, level, message := cycle.Boundary, "warn", rpkiFailedPoll
	if recovered {
		if !cycle.Failure.After(cycle.Boundary) {
			return time.Time{}, fmt.Errorf("RPKI recovery has no failed poll from this outage")
		}
		after, level, message = cycle.Failure, "info", rpkiCompletedSync
	}
	for _, entry := range entries {
		if entry.Timestamp.After(after) && entry.Level == level && entry.Message == message {
			return entry.Timestamp, nil
		}
	}
	return time.Time{}, fmt.Errorf("RPKI log lacks %s %q strictly after %s", level, message, after.Format(time.RFC3339))
}

func requireTransitConnectionLog() error {
	data, err := os.ReadFile(filepath.Join(demoState(showcaseLabID), "transit.log"))
	if err != nil {
		return err
	}
	return checkTransitConnectionLog(string(data))
}

// The external speaker outlives Ze Peer replacement. Counting its accepts cannot
// be reset by replacing a Ze Peer and zeroing that object's connection counter.
func checkTransitConnectionLog(document string) error {
	connections := 0
	for line := range strings.SplitSeq(document, "\n") {
		if strings.HasPrefix(line, "new connection from ") {
			connections++
		}
	}
	if connections != 1 {
		return fmt.Errorf("transit fixture accepted %d connections; the whole chapter requires exactly one", connections)
	}
	return nil
}

// Peer.runMessageLoop sends SendRoutes in order. Session.processMessage calls
// the receive callback synchronously, and server.onMessageReceived collects all
// delivery results before returning. The final accepted control is therefore a
// receive-side fence for the earlier exclusions, not merely a send-side marker.
func requireIRRRoutes(document string, accepted bool) error {
	var answer struct {
		Routes map[string][]rpkiDemoRoute `json:"adj-rib-in"`
	}
	if err := json.Unmarshal([]byte(document), &answer); err != nil {
		return err
	}
	control, excludedControl := irrWrongControl, irrHealthyControl
	if accepted {
		control, excludedControl = irrHealthyControl, irrWrongControl
	}
	found, completed := false, false
	for _, route := range answer.Routes["127.0.0.4"] {
		switch route.Key {
		case "ipv4/unicast:192.168.0.0/24":
			return fmt.Errorf("IRR admitted the outside prefix: %s", document)
		case excludedControl:
			return fmt.Errorf("IRR retained the other AS-set's control route: %s", document)
		case control:
			completed = true
		case "ipv4/unicast:10.0.0.0/24":
			found = true
		}
	}
	if !completed {
		return fmt.Errorf("IRR announcement cycle has not processed its final accepted control %s: %s", control, document)
	}
	if found != accepted {
		return fmt.Errorf("IRR customer route presence is %t, expected %t: %s", found, accepted, document)
	}
	return nil
}

// The control calls this while the previous fixture is stopped, before starting
// its successor. A cleared RIB and departed session fence retained control routes.
func requireIRRCustomerCleared() error {
	document, err := showcaseCLI("show bgp peer detail customer | json | no-more")
	if err != nil {
		return err
	}
	var peers struct {
		Peers map[string]struct {
			State string `json:"state"`
		} `json:"peers"`
	}
	if err := json.Unmarshal([]byte(document), &peers); err != nil {
		return err
	}
	peer, found := peers.Peers["127.0.0.4"]
	if !found {
		return fmt.Errorf("customer peer missing while waiting for session departure: %s", document)
	}
	if peer.State == "established" {
		return fmt.Errorf("previous customer session remains established")
	}
	document, err = showcaseCLI("show bgp adj-rib-in | json | no-more")
	if err != nil {
		return err
	}
	var answer struct {
		Routes map[string][]rpkiDemoRoute `json:"adj-rib-in"`
	}
	if err := json.Unmarshal([]byte(document), &answer); err != nil {
		return err
	}
	if answer.Routes == nil {
		return fmt.Errorf("customer departure answer lacks Adj-RIB-In: %s", document)
	}
	if len(answer.Routes["127.0.0.4"]) != 0 {
		return fmt.Errorf("previous customer routes have not cleared: %s", document)
	}
	return nil
}

func awaitIRRCustomer() error {
	// Peer.run can already be in its real 60-second maximum reconnect backoff
	// when this deliberately withheld fixture starts. Allow 15s for handshake;
	// policy convergence below still has its original, separate 10-second bound.
	deadline := time.Now().Add(75 * time.Second)
	var last error
	for {
		last = requireIRRCustomerStarted()
		if last == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("customer did not establish within reconnect backoff plus handshake: %w", last)
		}
		// sleep(poll-interval): wait for this fixture's final send and Ze's Established state.
		time.Sleep(100 * time.Millisecond)
	}
}

func requireIRRCustomerStarted() error {
	// startCommand truncates the log for each owned fixture start. The last send
	// proves this new speaker passed its OPEN/KEEPALIVE handshake; the accepted
	// RIB control, not this send-side log, subsequently proves receive completion.
	data, err := os.ReadFile(filepath.Join(demoState(showcaseLabID), "customer.log"))
	if err != nil {
		return err
	}
	if err := requireAll(string(data), "sending route 10.0.1.0/24 origin-as=65001", "sending route 192.0.2.0/24 origin-as=65001"); err != nil {
		return err
	}
	peers, err := showcaseCLI("show bgp peer list | json | no-more")
	if err != nil {
		return err
	}
	return requireShowcasePeers(peers, []string{"customer"})
}

// prepareIRRCustomerAnnouncements keeps one authored fixture, but moves the
// current policy's accepted control behind EVERY excluded announcement. A fixed
// ordering of the two controls would let one policy pass before its excluded
// control reached the receive filter.
func prepareIRRCustomerAnnouncements(state string, environ []string) (string, error) {
	text, err := runZe([]string{commandConfig, commandCat, zeConfigFile}, environ, nil)
	if err != nil {
		return "", err
	}
	tree, err := config.ParseTreeForValidation(text)
	if err != nil {
		return "", err
	}
	bgp := tree.GetContainer("bgp")
	if bgp == nil {
		return "", fmt.Errorf("customer announcement requires committed BGP configuration")
	}
	customer := bgp.GetList("peer")["customer"]
	if customer == nil {
		return "", fmt.Errorf("customer announcement requires a committed customer peer")
	}
	session := customer.GetContainer("session")
	if session == nil {
		return "", fmt.Errorf("customer announcement requires committed session configuration")
	}
	irr := session.GetContainer("irr")
	sourcePath := filepath.Join(demoDir(showcaseLabID), "customer.msg")
	if irr == nil {
		// The base-lab validator also exercises this fixture before any IRR topic.
		return sourcePath, nil
	}
	asSet, _ := irr.Get("as-set")
	source, err := os.ReadFile(sourcePath)
	if err != nil {
		return "", err
	}
	ordered, err := orderIRRAnnouncements(string(source), asSet)
	if err != nil {
		return "", err
	}
	path := filepath.Join(state, "customer.msg")
	if err := os.WriteFile(path, []byte(ordered), 0o600); err != nil {
		return "", err
	}
	return path, nil
}

func orderIRRAnnouncements(source, asSet string) (string, error) {
	var control string
	switch asSet {
	case "AS-TEST":
		control = "10.0.1.0/24"
	case "AS-V4ONLY":
		control = "192.0.2.0/24"
	default:
		return "", fmt.Errorf("customer announcement has unsupported committed AS-set %q", asSet)
	}
	required := map[string]int{
		"10.0.0.0/24":    0,
		"192.168.0.0/24": 0,
		"10.0.1.0/24":    0,
		"192.0.2.0/24":   0,
	}
	var final string
	var ordered strings.Builder
	ordered.Grow(len(source) + 1)
	for line := range strings.SplitSeq(source, "\n") {
		last := false
		if strings.HasPrefix(line, "option=update:value=send-route:") {
			_, value, _ := strings.Cut(line, ":prefix=")
			prefix, _, _ := strings.Cut(value, ":")
			if _, needed := required[prefix]; needed {
				required[prefix]++
				if prefix == control {
					final, last = line, true
				}
			}
		}
		if !last && line != "" {
			ordered.WriteString(line)
			ordered.WriteByte('\n')
		}
	}
	for prefix, count := range required {
		if count != 1 {
			return "", fmt.Errorf("customer fixture must announce %s exactly once, got %d", prefix, count)
		}
	}
	ordered.WriteString(final)
	ordered.WriteByte('\n')
	return ordered.String(), nil
}
