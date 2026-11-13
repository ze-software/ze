//go:build linux

// Design: docs/functional-tests.md -- an LDP neighbor on a veth pair
// Related: ldp_neighbor_linux.go -- the neighbor these scenarios drive
// Related: register_ldp_neighbor_linux.go -- the fixture names
// Related: routing_fixture.go -- routingRows

package fixture

// ldp_session_end_linux.go holds the two scenarios that judge how Ze treats the
// end of an LDP session, from outside the daemon: what the neighbor sees on the
// wire, what `show ldp neighbor` reports, and which `ldp` session events an
// observer subscribed to the namespace receives.
//
// The observer plugin and the neighbor are one process: the plugin is forked by
// the daemon inside Ze's namespace, and it opens the neighbor's sockets inside
// the neighbor's namespace (inNamedNetns), so one scenario can order a wire
// action, a daemon read and an event check without a marker file between two
// processes.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const (
	// ldpObserverName is the fallback plugin name; the daemon assigns the real one.
	ldpObserverName = "ldp-neighbor"
	// ldpNeighborStateOperational and ldpNeighborStateDiscovered are the two
	// `show ldp neighbor` states the scenarios wait on: a session that reached
	// OPERATIONAL, and an adjacency with no session at all (showNeighbors).
	ldpNeighborStateOperational = "operational"
	ldpNeighborStateDiscovered  = "discovered"
	// ldpFirstDialWait bounds Ze's first dial: daemon start, discovery on the
	// link, and the neighbor's first Hellos.
	ldpFirstDialWait = 30 * time.Second
	// ldpRedialWait bounds the redial after one Hello once an operational
	// session ended. It sits below the 15 seconds a failed setup waits
	// (setupRetryDelayMin), so a session end recorded as a failed setup cannot
	// pass it, and far above the milliseconds a dial takes.
	ldpRedialWait = 10 * time.Second
	// ldpSetupBackoffFloor is the least time Ze may wait after a NAK'd
	// Initialization. RFC 5036 Section 2.5.3: "The session establishment setup
	// attempt following a NAK'd Initialization message MUST be delayed no less
	// than 15 seconds, and subsequent delays MUST grow to a maximum delay of no
	// less than 2 minutes."
	ldpSetupBackoffFloor = 15 * time.Second
	// ldpSetupBackoffWait bounds the dial after a rejection: the floor plus a
	// margin for the Hello cadence and a loaded host.
	ldpSetupBackoffWait = 40 * time.Second
	// ldpPollAttempts and ldpPollDelay bound each wait on daemon state or on an
	// event: 10 seconds.
	ldpPollAttempts = 100
	ldpPollDelay    = 100 * time.Millisecond
)

// ldpSessionEventKind is which `ldp` session event the observer received.
type ldpSessionEventKind uint8

const (
	ldpSessionEventUnspecified ldpSessionEventKind = iota
	ldpSessionEventUp
	ldpSessionEventDown
)

func (k ldpSessionEventKind) String() string {
	switch k {
	case ldpSessionEventUp:
		return "session-up"
	case ldpSessionEventDown:
		return "session-down"
	case ldpSessionEventUnspecified:
		return "unspecified"
	}
	panic("BUG: invalid ldpSessionEventKind")
}

// ldpSessionEventRecord is one session event, as the observer decoded it.
type ldpSessionEventRecord struct {
	kind        ldpSessionEventKind
	peerAddress string
	identifier  string
	iface       string
}

// ldpSessionEventLog records the `ldp` session events the observer receives,
// in delivery order. Safe for concurrent use: the SDK delivers on its own
// goroutine and the scenario reads on another.
type ldpSessionEventLog struct {
	mu          sync.Mutex
	records     []ldpSessionEventRecord
	undecodable []string
}

// setup subscribes the observer to session-up and session-down, enveloped so
// each delivery names its event, before the plugin's first stage.
func (l *ldpSessionEventLog) setup(plugin *sdk.Plugin) error {
	plugin.SetStartupSubscriptionsIn("ldp", []string{"session-up", "session-down"}, nil, "")
	plugin.SetEnvelope(true)
	plugin.OnEvent(l.observe)
	return nil
}

// observe records one delivery. One the log cannot read is kept, and fails
// the scenario's verdict, rather than being skipped.
func (l *ldpSessionEventLog) observe(event string) error {
	record, err := decodeLDPSessionEvent(event)
	l.mu.Lock()
	defer l.mu.Unlock()
	if err != nil {
		l.undecodable = append(l.undecodable, err.Error())
		return nil
	}
	l.records = append(l.records, record)
	return nil
}

func decodeLDPSessionEvent(event string) (ldpSessionEventRecord, error) {
	var record ldpSessionEventRecord
	envelope, err := rpc.ParseEventEnvelope(event)
	if err != nil {
		return record, fmt.Errorf("decode the envelope of %q: %w", event, err)
	}
	if envelope.Namespace != "ldp" {
		return record, fmt.Errorf("event %q from namespace %q, subscribed to ldp only", envelope.Event, envelope.Namespace)
	}
	switch envelope.Event {
	case "session-up":
		record.kind = ldpSessionEventUp
	case "session-down":
		record.kind = ldpSessionEventDown
	default:
		return record, fmt.Errorf("event ldp/%s, subscribed to session-up and session-down only", envelope.Event)
	}
	var payload struct {
		PeerAddress   string `json:"peer-address"`
		LDPIdentifier string `json:"ldp-identifier"`
		Interface     string `json:"interface"`
	}
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		return record, fmt.Errorf("decode the ldp/%s payload %s: %w", envelope.Event, envelope.Payload, err)
	}
	record.peerAddress = payload.PeerAddress
	record.identifier = payload.LDPIdentifier
	record.iface = payload.Interface
	return record, nil
}

// snapshot answers the neighbor's events so far, and an error for any
// delivery the log could not read.
func (l *ldpSessionEventLog) snapshot() ([]ldpSessionEventRecord, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.undecodable) != 0 {
		return nil, fmt.Errorf("the observer received %d event(s) it could not read: %v", len(l.undecodable), l.undecodable)
	}
	var mine []ldpSessionEventRecord
	for _, record := range l.records {
		if record.identifier == ldpNeighborIdentifier {
			mine = append(mine, record)
		}
	}
	return mine, nil
}

// await waits until the observer holds at least count events for the
// neighbor, and answers them.
func (l *ldpSessionEventLog) await(ctx context.Context, count int) ([]ldpSessionEventRecord, error) {
	var records []ldpSessionEventRecord
	var readErr error
	reached := Poll(ctx, ldpPollAttempts, ldpPollDelay, func() bool {
		records, readErr = l.snapshot()
		return readErr == nil && len(records) >= count
	})
	if readErr != nil {
		return nil, readErr
	}
	if !reached {
		return nil, fmt.Errorf("the observer holds %d ldp session event(s) for %s, want %d: %s",
			len(records), ldpNeighborIdentifier, count, describeLDPSessionEvents(records))
	}
	return records, nil
}

// requireLDPSessionEvents compares the neighbor's whole event history with
// want, and checks that every event names the neighbor and Ze's link.
func requireLDPSessionEvents(records []ldpSessionEventRecord, want ...ldpSessionEventKind) error {
	kinds := make([]ldpSessionEventKind, 0, len(records))
	for _, record := range records {
		kinds = append(kinds, record.kind)
	}
	if !slices.Equal(kinds, want) {
		return fmt.Errorf("the ldp session events for %s are %s, want %v",
			ldpNeighborIdentifier, describeLDPSessionEvents(records), want)
	}
	for _, record := range records {
		if record.peerAddress != ldpNeighborPeerAddress {
			return fmt.Errorf("%s names peer-address %q, want 10.99.0.1", record.kind, record.peerAddress)
		}
		if record.iface != ldpNeighborZeLink {
			return fmt.Errorf("%s names interface %q, want %s", record.kind, record.iface, ldpNeighborZeLink)
		}
	}
	return nil
}

func describeLDPSessionEvents(records []ldpSessionEventRecord) string {
	var line textbuf.Buffer
	line.Byte('[')
	for index, record := range records {
		if index > 0 {
			line.Byte(' ')
		}
		line.Str(record.kind.String())
	}
	return line.Byte(']').String()
}

// ldpNeighborState answers the state `show ldp neighbor` reports for the
// neighbor, and an error when the daemon reports no such neighbor.
func ldpNeighborState(ctx context.Context, plugin *sdk.Plugin) (string, error) {
	rows, err := routingRows(ctx, plugin, "show ldp neighbor", true)
	if err != nil {
		return "", err
	}
	for _, row := range rows {
		if row["lsr-id"] != ldpNeighborPeerAddress {
			continue
		}
		state, ok := row["state"].(string)
		if !ok {
			return "", fmt.Errorf("show ldp neighbor: the row for 10.99.0.1 has no state: %#v", row)
		}
		return state, nil
	}
	return "", fmt.Errorf("show ldp neighbor holds no row for 10.99.0.1: %#v", rows)
}

// awaitLDPNeighborState waits until the daemon reports the neighbor in want.
func awaitLDPNeighborState(ctx context.Context, plugin *sdk.Plugin, want string) error {
	var state string
	var readErr error
	reached := Poll(ctx, ldpPollAttempts, ldpPollDelay, func() bool {
		state, readErr = ldpNeighborState(ctx, plugin)
		return readErr == nil && state == want
	})
	if reached {
		return nil
	}
	if readErr != nil {
		return fmt.Errorf("waiting for show ldp neighbor to report %s: %w", want, readErr)
	}
	return fmt.Errorf("show ldp neighbor reports 10.99.0.1 %s, want %s", state, want)
}

// ldpNeighborNamespace reads the scenario arguments, `netns <name>`: the
// namespace that holds the neighbor's link.
func ldpNeighborNamespace(args []string) (string, error) {
	if len(args) != 2 {
		return "", fmt.Errorf("want netns <name>, got %d argument(s) %q", len(args), args)
	}
	if args[0] != ldpNamespaceKeyword {
		return "", fmt.Errorf("want netns <name>, got %q", args[0])
	}
	return args[1], nil
}

// runLDPNeighborScenario opens the neighbor, runs scenario as the observer
// plugin, and closes the neighbor only after the observer stopped, so a
// session the scenario leaves operational ends with the daemon.
func runLDPNeighborScenario(ctx context.Context, args []string, scenario func(context.Context, *sdk.Plugin, *ldpNeighbor, *ldpSessionEventLog) error) error {
	namespace, err := ldpNeighborNamespace(args)
	if err != nil {
		return err
	}
	neighbor, err := openLDPNeighbor(ctx, namespace)
	if err != nil {
		return err
	}
	events := &ldpSessionEventLog{}
	runErr := observeConfigured(ctx, ldpObserverName, &sdk.Registration{}, events.setup,
		func(ctx context.Context, plugin *sdk.Plugin) error {
			return scenario(ctx, plugin, neighbor, events)
		})
	return errors.Join(runErr, neighbor.close())
}

// bringUpLDPSession brings an accepted session to operational on both sides,
// and waits for the daemon and the observer to say so: Ze's state reads
// operational and the observer holds eventCount events.
func bringUpLDPSession(ctx context.Context, plugin *sdk.Plugin, session *ldpNeighborSession, events *ldpSessionEventLog, eventCount int) ([]ldpSessionEventRecord, error) {
	if err := session.establish(); err != nil {
		return nil, err
	}
	if err := awaitLDPNeighborState(ctx, plugin, ldpNeighborStateOperational); err != nil {
		return nil, err
	}
	return events.await(ctx, eventCount)
}

// endLDPSession stops the Hellos and ends an operational session from the
// neighbor's side, then waits until the daemon reports the session gone and
// the observer holds eventCount events. One daemon read passes between the
// last Hello and the close, so Ze reads that Hello while the session still
// exists and nothing dials on it after the end.
func endLDPSession(ctx context.Context, plugin *sdk.Plugin, neighbor *ldpNeighbor, session *ldpNeighborSession, events *ldpSessionEventLog, eventCount int) ([]ldpSessionEventRecord, error) {
	neighbor.hellos.setEnabled(false)
	if err := awaitLDPNeighborState(ctx, plugin, ldpNeighborStateOperational); err != nil {
		return nil, err
	}
	if err := session.close(); err != nil {
		return nil, err
	}
	if err := awaitLDPNeighborState(ctx, plugin, ldpNeighborStateDiscovered); err != nil {
		return nil, fmt.Errorf("after the neighbor closed the operational session: %w", err)
	}
	records, err := events.await(ctx, eventCount)
	if err != nil {
		return nil, fmt.Errorf("after the neighbor closed the operational session: %w", err)
	}
	return records, nil
}

// ldpSessionEndClearsBackoffDriver is `ldp/session-end-clears-backoff netns
// <name>`, the observer of test/ldp/ldp-session-end-clears-backoff.ci.
func ldpSessionEndClearsBackoffDriver(ctx context.Context, args []string) error {
	return runLDPNeighborScenario(ctx, args, ldpSessionEndClearsBackoff)
}

// ldpSessionEndClearsBackoff brings a session to operational, ends it from
// the neighbor's side, and requires Ze to dial again on the first Hello it
// receives after the daemon reports the session gone.
//
// The neighbor's Hellos pause while the session ends, and exactly one Hello is
// sent once `show ldp neighbor` reads `discovered` and the observer holds the
// session-down. onDone deletes the session and settles the backoff under one
// lock, so a daemon that reports `discovered` has already decided the backoff.
// A session end recorded as a failed setup leaves a 15-second backoff, and
// that one Hello then opens nothing.
func ldpSessionEndClearsBackoff(ctx context.Context, plugin *sdk.Plugin, neighbor *ldpNeighbor, events *ldpSessionEventLog) error {
	neighbor.hellos.setEnabled(true)
	first, err := neighbor.acceptSession(time.Now().Add(ldpFirstDialWait))
	if err != nil {
		return fmt.Errorf("first session: %w", err)
	}
	if _, err := bringUpLDPSession(ctx, plugin, first, events, 1); err != nil {
		return fmt.Errorf("first session: %w", err)
	}
	if _, err := endLDPSession(ctx, plugin, neighbor, first, events, 2); err != nil {
		return err
	}

	if err := neighbor.hellos.sendOne(); err != nil {
		return err
	}
	helloAt := time.Now()
	second, err := neighbor.acceptSession(helloAt.Add(ldpRedialWait))
	if err != nil {
		return fmt.Errorf("no session from Ze within %s of the first Hello after the operational session ended, "+
			"so that end left a setup backoff on the adjacency: %w", ldpRedialWait, err)
	}
	redial := time.Since(helloAt)
	neighbor.hellos.setEnabled(true)
	records, err := bringUpLDPSession(ctx, plugin, second, events, 3)
	if err != nil {
		return fmt.Errorf("second session: %w", err)
	}
	// The second session stays operational: it ends with the daemon, after the
	// Hellos stop, so no later dial can follow it.
	neighbor.hellos.setEnabled(false)
	if err := requireLDPSessionEvents(records, ldpSessionEventUp, ldpSessionEventDown, ldpSessionEventUp); err != nil {
		return err
	}
	if err := neighbor.hellos.err(); err != nil {
		return err
	}
	netnsSay("OK: Ze dialed ", ldpNeighborIdentifier, " ", redial.Round(time.Millisecond).String(),
		" after the first Hello that followed the end of an operational session; events ", describeLDPSessionEvents(records))
	return nil
}

// ldpSetupFailurePublishesNoSessionDownDriver is
// `ldp/setup-failure-publishes-no-session-down netns <name>`, the observer of
// test/ldp/ldp-setup-failure-publishes-no-session-down.ci.
func ldpSetupFailurePublishesNoSessionDownDriver(ctx context.Context, args []string) error {
	return runLDPNeighborScenario(ctx, args, ldpSetupFailurePublishesNoSessionDown)
}

// ldpSetupFailurePublishesNoSessionDown rejects Ze's first Initialization, then
// accepts the next session and ends it, and requires the event history to be
// that one session's up and down alone.
//
// The accepted session is the positive control: the observer does receive a
// session-down when one is owed, so the missing one for the rejected attempt
// is Ze's silence and not a deaf observer. Delivery to one plugin keeps the
// order Ze emitted in, and the rejected attempt ended at least 15 seconds
// before the second session's session-up, so a session-down it published sits
// first in the history.
func ldpSetupFailurePublishesNoSessionDown(ctx context.Context, plugin *sdk.Plugin, neighbor *ldpNeighbor, events *ldpSessionEventLog) error {
	neighbor.hellos.setEnabled(true)
	first, err := neighbor.acceptSession(time.Now().Add(ldpFirstDialWait))
	if err != nil {
		return fmt.Errorf("first attempt: %w", err)
	}
	if err := first.reject(); err != nil {
		return err
	}
	rejectedAt := time.Now()
	if err := awaitLDPNeighborState(ctx, plugin, ldpNeighborStateDiscovered); err != nil {
		return fmt.Errorf("after the neighbor rejected the Initialization: %w", err)
	}

	second, err := neighbor.acceptSession(rejectedAt.Add(ldpSetupBackoffWait))
	if err != nil {
		return fmt.Errorf("the session after the rejected attempt: %w", err)
	}
	waited := time.Since(rejectedAt)
	if _, err := bringUpLDPSession(ctx, plugin, second, events, 1); err != nil {
		return fmt.Errorf("the session after the rejected attempt: %w", err)
	}
	if waited < ldpSetupBackoffFloor {
		return fmt.Errorf("a new session %s after the neighbor NAK'd Ze's Initialization, under the %s floor",
			waited.Round(time.Millisecond), ldpSetupBackoffFloor)
	}

	records, err := endLDPSession(ctx, plugin, neighbor, second, events, 2)
	if err != nil {
		return err
	}
	if err := requireLDPSessionEvents(records, ldpSessionEventUp, ldpSessionEventDown); err != nil {
		return fmt.Errorf("%w: the rejected attempt published a session event", err)
	}
	if err := neighbor.hellos.err(); err != nil {
		return err
	}
	netnsSay("OK: the rejected attempt published no session event, Ze dialed again ",
		waited.Round(time.Millisecond).String(), " later; events ", describeLDPSessionEvents(records))
	return nil
}
