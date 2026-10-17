package fixture

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	coreevents "github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

type eventScenario02 func(context.Context, *sdk.Plugin, <-chan string, <-chan struct{}) error

func init() {
	Register("plugin/attach-process-dynamic-group", dynamicGroupDriver02)
	Register("plugin/attach-process-dynamic-group-wait", dynamicGroupWait02)
	Register("plugin/wait-file", waitFileDriver02)
	Register("plugin/attach-process-receive-filter-state", eventObserver02("receive-filter-state", []string{eventUpdate, eventState}, receiveFilterState02))
	Register("plugin/attach-process-receive-filter-update", eventObserver02("receive-filter-update", []string{eventUpdate, eventState}, receiveFilterUpdate02))
	Register("plugin/labeled-withdraw-event", eventObserver02("labeled-withdraw-event", []string{eventUpdate}, labeledWithdrawEvent02))
	Register("plugin/attach-process-reload-kept", eventObserver02("reload-kept", []string{eventState, bgpevents.EventKeepalive}, reloadKept02))
	Register("plugin/attach-process-reload-added", eventObserver02("reload-added", []string{eventState}, reloadAdded02))
	Register("plugin/attach-receive-only-live", eventObserver02("attach-receive-only-live", []string{eventUpdate, eventState}, attachReceiveOnlyLive02))
	Register("plugin/attach-process-runtime-subscribe", eventObserver02("runtime-subscribe", []string{eventState}, runtimeSubscribe02))
	Register("plugin/attach-process-runtime-subscribe-trigger", runtimeSubscribeTrigger02)
	Register("plugin/attach-process-send-permission", eventObserver02("send-permission-injector", []string{eventUpdate, eventState}, sendPermission02))
	Register("plugin/attach-process-unattached-served", eventObserver02("unattached-served", []string{eventUpdate, eventState}, unattachedServed02))
	Register("plugin/attach-process-unattached-silent", eventObserver02("unattached-silent", []string{eventUpdate, eventState}, unattachedSilent02))
	Register("plugin/audit-config-commit", observer02("audit-commit-test", auditConfigCommit02))
}

func eventObserver02(name string, events []string, scenario eventScenario02) Driver {
	return func(ctx context.Context, args []string) error {
		if len(args) != 0 {
			return fmt.Errorf("%s takes no arguments", name)
		}
		plugin, err := newObserver(name)
		if err != nil {
			return fmt.Errorf("connect observer %s: %w", name, err)
		}
		defer plugin.Close() //nolint:errcheck // fixture teardown, so a close failure changes no assertion

		runCtx, cancel := context.WithCancel(ctx)
		defer cancel()
		eventCh := make(chan string, 256)
		shutdown := make(chan struct{})
		var shutdownOnce sync.Once
		plugin.SetStartupSubscriptions(events, nil, "")
		plugin.OnEvent(func(event string) error {
			select {
			case eventCh <- event:
				return nil
			case <-runCtx.Done():
				return runCtx.Err()
			}
		})
		plugin.OnBye(func(string) error { shutdownOnce.Do(func() { close(shutdown) }); return nil })
		result := make(chan error, 1)
		plugin.OnAllPluginsReady(func() error {
			go func() {
				scenarioErr := scenario(runCtx, plugin, eventCh, shutdown)
				if scenarioErr != nil {
					// Reported before the shutdown it requests, as observeConfigured
					// does: the error this driver returns is printed only after Run
					// ends, which is after the daemon was asked to stop, so it
					// reaches the runner only when it wins that race. CI lost the
					// verdict of attach-process-unattached-is-silent that way on
					// 2026-10-16 and 2026-10-17.
					ReportFailure(scenarioErr)
					requestShutdownAsync02(runCtx, plugin)
				}
				result <- scenarioErr
			}()
			return nil
		})

		runErr := plugin.Run(runCtx, &sdk.Registration{})
		shutdownOnce.Do(func() { close(shutdown) })
		cancel()
		select {
		case scenarioErr := <-result:
			return errors.Join(scenarioErr, runErr)
		case <-time.After(2 * time.Second):
			return runErr
		}
	}
}

func requestShutdownAsync02(ctx context.Context, plugin *sdk.Plugin) {
	go func() {
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_, _, _ = plugin.DispatchCommand(shutdownCtx, "request shutdown")
	}()
}

func nextEvent02(ctx context.Context, events <-chan string, shutdown <-chan struct{}, timeout time.Duration) (string, bool) {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case event := <-events:
		return event, true
	case <-shutdown:
		select {
		case event := <-events:
			return event, true
		default:
			return "", false
		}
	case <-ctx.Done():
		return "", false
	case <-timer.C:
		return "", false
	}
}

func eventBody02(raw string) map[string]any {
	var decoded map[string]any
	if json.Unmarshal([]byte(raw), &decoded) != nil {
		return nil
	}
	if bgp, ok := decoded["bgp"].(map[string]any); ok {
		return bgp
	}
	return decoded
}

func eventFacts02(raw string) (name, kind, direction string) {
	body := eventBody02(raw)
	peer, _ := body["peer"].(map[string]any)
	message, _ := body["message"].(map[string]any)
	name, _ = peer["name"].(string)
	kind, _ = message["type"].(string)
	direction, _ = message["direction"].(string)
	return name, kind, direction
}

func waitForShutdown02(ctx context.Context, shutdown <-chan struct{}, timeout time.Duration) error {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-shutdown:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return fmt.Errorf("shutdown not observed within %s", timeout)
	}
}

func dynamicRefusal02(who, kind, direction string) string {
	switch who {
	case groupDyn, "inherits":
		if kind != eventState {
			return fmt.Sprintf("%s is granted state alone by its group and was fed %s (%s)", who, kind, direction)
		}
	case "restates":
		if kind != eventUpdate {
			return fmt.Sprintf("restates states its own list, update-received, and was fed %s", kind)
		}
		if direction != directionReceived {
			return "restates is granted update-received and was fed an update ze SENT"
		}
	default:
		return fmt.Sprintf("an event arrived for a peer no attach block covers: %q", who)
	}
	return ""
}

// dynamicGroupMarker02 is the file the observer writes once it is subscribed,
// and the file dynamicGroupWait02 holds the dynamic member's dial behind. It is
// relative on purpose. The observer inherits the daemon's working directory, the
// wait step runs in the same directory, and the runner creates that directory
// fresh for each run (Record.WorkDir). The shared absolute path this replaced
// outlived the run that wrote it and was visible to every other run on the host.
// The wait could then pass on a marker no observer of THIS run had written, and
// the member dialed before anything was subscribed to its session-up event.
const dynamicGroupMarker02 = "dynamic-group.ready"

func dynamicGroupDriver02(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("unexpected arguments: %v", args)
	}
	scenario := func(ctx context.Context, plugin *sdk.Plugin, events <-chan string, shutdown <-chan struct{}) error {
		return dynamicGroup02(ctx, plugin, events, shutdown, dynamicGroupMarker02)
	}
	return eventObserver02("dyn-group-observer", []string{eventUpdate, eventState}, scenario)(ctx, nil)
}

func dynamicGroup02(ctx context.Context, plugin *sdk.Plugin, events <-chan string, shutdown <-chan struct{}, marker string) error {
	if err := os.WriteFile(marker, []byte("ready"), 0o600); err != nil {
		return err
	}
	defer os.Remove(marker) //nolint:errcheck // scratch cleanup on exit, so a removal failure changes no assertion
	seen := make(map[string]bool)
	for len(seen) != 3 {
		event, ok := nextEvent02(ctx, events, shutdown, 25*time.Second)
		if !ok {
			missing := make([]string, 0, 3)
			for _, wanted := range []string{groupDyn, "inherits", "restates"} {
				if !seen[wanted] {
					missing = append(missing, wanted)
				}
			}
			return fmt.Errorf("DYN-GROUP: never fed %v", missing)
		}
		name, kind, direction := eventFacts02(event)
		who := name
		if strings.HasPrefix(name, "dyn-") {
			who = groupDyn
		}
		if reason := dynamicRefusal02(who, kind, direction); reason != "" {
			return fmt.Errorf("DYN-GROUP: %s", reason)
		}
		seen[who] = true
	}
	fmt.Fprintln(os.Stderr, "DYN-GROUP: the dynamic member was fed its group's list")
	fmt.Fprintln(os.Stderr, "DYN-GROUP: the member that states no block kept its group's list")
	fmt.Fprintln(os.Stderr, "DYN-GROUP: the member that restates it replaced its group's list")
	if err := quiesce02(ctx, plugin); err != nil {
		return err
	}
	for {
		event, ok := nextEvent02(ctx, events, shutdown, 2*time.Second)
		if !ok {
			break
		}
		name, kind, direction := eventFacts02(event)
		who := name
		if strings.HasPrefix(name, "dyn-") {
			who = groupDyn
		}
		if reason := dynamicRefusal02(who, kind, direction); reason != "" {
			return fmt.Errorf("DYN-GROUP: %s", reason)
		}
	}
	fmt.Fprintln(os.Stderr, "DYN-GROUP: no peer was fed outside its list")
	requestShutdownAsync02(ctx, plugin)
	return waitForShutdown02(ctx, shutdown, 15*time.Second)
}

func receiveFilterState02(ctx context.Context, _ *sdk.Plugin, events <-chan string, shutdown <-chan struct{}) error {
	seenState := false
	for {
		event, ok := nextEvent02(ctx, events, shutdown, 20*time.Second)
		if !ok {
			break
		}
		_, kind, direction := eventFacts02(event)
		if kind == eventUpdate {
			return fmt.Errorf("FILTER-STATE: update leaked to a process granted state only (%s)", direction)
		}
		if kind == eventState && !seenState {
			seenState = true
			fmt.Fprintln(os.Stderr, "FILTER-STATE: state event delivered")
		}
	}
	if !seenState {
		return fmt.Errorf("FILTER-STATE: the peer grants state and delivered none")
	}
	return nil
}

func receiveFilterUpdate02(ctx context.Context, plugin *sdk.Plugin, events <-chan string, shutdown <-chan struct{}) error {
	for {
		event, ok := nextEvent02(ctx, events, shutdown, 20*time.Second)
		if !ok {
			return fmt.Errorf("FILTER-UPDATE: no received update arrived")
		}
		_, kind, direction := eventFacts02(event)
		if kind == eventState {
			return fmt.Errorf("FILTER-UPDATE: state leaked to a process granted update-received")
		}
		if kind == eventUpdate && direction == directionSent {
			return fmt.Errorf("FILTER-UPDATE: a SENT update reached an update-received grant")
		}
		if kind == eventUpdate {
			break
		}
	}
	fmt.Fprintln(os.Stderr, "FILTER-UPDATE: received update delivered")
	if err := quiesce02(ctx, plugin); err != nil {
		return err
	}
	requestShutdownAsync02(ctx, plugin)
	return waitForShutdown02(ctx, shutdown, 15*time.Second)
}

func stateEvent02(raw string) bool {
	_, kind, _ := eventFacts02(raw)
	return kind == eventState
}

// stopped02 reports whether the run ended: the daemon said bye or the context
// was canceled. nextEvent02 returns at once after either, so a loop that keeps
// polling it on a miss MUST check this, or it spins until its own deadline.
func stopped02(ctx context.Context, shutdown <-chan struct{}) bool {
	if ctx.Err() != nil {
		return true
	}
	select {
	case <-shutdown:
		return true
	default:
		return false
	}
}

// reloadKeptKeepalives02 is how many KEEPALIVEs Ze SENT the kept edge must
// receive after the attached marker. Peer.bindProcesses tells the added process
// up BEFORE applyHotSwappableSettings calls Reactor.publishDeliveryGraphLocked,
// so the first sent KEEPALIVE after the marker may still have been dispatched
// through the old graph. Only the sent direction is counted: the test peer
// answers every frame with a KEEPALIVE, so a received one can arrive a few
// milliseconds after a sent one, while two sent KEEPALIVEs are two fires of
// Ze's keepalive timer, which fsm.Timers schedules at least one second apart.
// The second therefore went through the rebuilt graph as long as the up and the
// publish in that one apply are less than a second apart.
const reloadKeptKeepalives02 = 2

// The markers test/plugin/attach-process-reload.ci's two processes order
// themselves by: the attached process writes the first once it was told the
// session is up, the kept process writes the second once it was fed after that.
const (
	reloadAttachedMarker02 = "attached.marker"
	reloadKeptMarker02     = "kept.marker"
)

// reloadKept02 is the process attached in both configs of
// test/plugin/attach-process-reload.ci.
//
// Goal: a reload that attaches another receive-only process leaves this edge in
// the delivery graph and keeps feeding it, on the same session. Method: wait for
// the session's own up, then, once the attached process was told up, require
// reloadKeptKeepalives02 sent KEEPALIVEs queued after that point, so at least
// one went through the rebuilt delivery graph. Any state event after the first up
// fails: a down or a second up is a restart, or this process being detached.
func reloadKept02(ctx context.Context, _ *sdk.Plugin, events <-chan string, shutdown <-chan struct{}) error {
	for {
		event, ok := nextEvent02(ctx, events, shutdown, 25*time.Second)
		if !ok {
			return fmt.Errorf("RELOAD-KEPT: never told the session is up")
		}
		if eventIsUp02(event) {
			break
		}
	}
	deadline := time.Now().Add(30 * time.Second)
	attached := false
	keepalives := 0
	for time.Now().Before(deadline) {
		if !attached && regularFileExists02(reloadAttachedMarker02) {
			attached = true
			// What was queued before the marker may predate the reload, so it
			// proves nothing about the edge after it: drain it, still refusing
			// a state event.
			if err := drainKeptQueue02(events); err != nil {
				return err
			}
		}
		event, ok := nextEvent02(ctx, events, shutdown, 500*time.Millisecond)
		if !ok {
			if stopped02(ctx, shutdown) {
				return fmt.Errorf("RELOAD-KEPT: the run ended first: attached marker seen %t, %d of %d sent KEEPALIVEs after it", attached, keepalives, reloadKeptKeepalives02)
			}
			continue
		}
		if stateEvent02(event) {
			return fmt.Errorf("RELOAD-KEPT: state event after the first up, the session restarted or the edge was detached: %.300s", event)
		}
		_, kind, direction := eventFacts02(event)
		if !attached {
			continue
		}
		if kind != bgpevents.EventKeepalive {
			continue
		}
		if direction != coreevents.DirectionSent {
			continue
		}
		keepalives++
		if keepalives < reloadKeptKeepalives02 {
			continue
		}
		fmt.Fprintln(os.Stderr, "RELOAD-KEPT: fed before and after the reload, on one session")
		if err := os.WriteFile(reloadKeptMarker02, []byte("done"), 0o600); err != nil {
			return err
		}
		return waitForShutdown02(ctx, shutdown, 30*time.Second)
	}
	return fmt.Errorf("RELOAD-KEPT: attached marker seen %t, and %d of %d sent KEEPALIVEs reached the kept edge after it", attached, keepalives, reloadKeptKeepalives02)
}

// drainKeptQueue02 empties the kept process's queue without waiting, and fails
// on a state event, which after the first up means a restart or a detach.
func drainKeptQueue02(events <-chan string) error {
	for {
		select {
		case event := <-events:
			if stateEvent02(event) {
				return fmt.Errorf("RELOAD-KEPT: state event after the first up, the session restarted or the edge was detached: %.300s", event)
			}
		default:
			return nil
		}
	}
}

// reloadAdded02 is the process only the SECOND config of
// test/plugin/attach-process-reload.ci attaches, `receive [ state ]`.
//
// Goal: the reload delivers the binding to the running session, and the process
// is told the session is up by the targeted up (reason "attached", no
// initial-replay token). Method: the process is attached to nothing before the
// reload, so its first event must be that up. An event before it was fed before
// the config attached it, and an up with another reason is a restarted session's.
func reloadAdded02(ctx context.Context, plugin *sdk.Plugin, events <-chan string, shutdown <-chan struct{}) error {
	event, ok := nextEvent02(ctx, events, shutdown, 40*time.Second)
	if !ok {
		return fmt.Errorf("RELOAD-ADDED: the reload attached it and it was told nothing")
	}
	body := eventBody02(event)
	state, _ := body["state"].(string)
	reason, _ := body["reason"].(string)
	if !eventIsUp02(event) {
		return fmt.Errorf("RELOAD-ADDED: first event is not an up (state %q): %.300s", state, event)
	}
	if reason != "attached" {
		return fmt.Errorf("RELOAD-ADDED: up with reason %q, want \"attached\" (a restarted session's up): %.300s", reason, event)
	}
	if _, replay := body["initial-replay"]; replay {
		return fmt.Errorf("RELOAD-ADDED: attached up carries an initial-replay token: %.300s", event)
	}
	fmt.Fprintln(os.Stderr, "RELOAD-ADDED: told up, reason attached, by the reload")
	if err := os.WriteFile(reloadAttachedMarker02, []byte("done"), 0o600); err != nil {
		return err
	}
	deadline := time.Now().Add(30 * time.Second)
	for !regularFileExists02(reloadKeptMarker02) {
		if time.Now().After(deadline) {
			return fmt.Errorf("RELOAD-ADDED: the kept edge never reported")
		}
		later, ok := nextEvent02(ctx, events, shutdown, 500*time.Millisecond)
		if !ok && stopped02(ctx, shutdown) {
			return fmt.Errorf("RELOAD-ADDED: the run ended before the kept edge reported")
		}
		if ok && stateEvent02(later) {
			return fmt.Errorf("RELOAD-ADDED: state event after the attached up, the session did not stay up: %.300s", later)
		}
	}
	requestShutdownAsync02(ctx, plugin)
	return waitForShutdown02(ctx, shutdown, 15*time.Second)
}

// attachReceiveOnlyLive02 is the process a reload attaches, receive-only, to a
// peer whose session is already up (test/reload/bgp-attach-receive-only-live.ci).
//
// Goal: the process is told the session is up, by a state up carrying reason
// "attached" and no initial-replay token, before any UPDATE of that session
// reaches it, and it then receives the route the peer announced BEFORE the
// reload, which the peer re-sends on Ze's ROUTE-REFRESH. Method: refuse an
// UPDATE that beats the attached up, and refuse an up with any other reason,
// because an ordinary up is what a restarted session delivers.
func attachReceiveOnlyLive02(ctx context.Context, _ *sdk.Plugin, events <-chan string, shutdown <-chan struct{}) error {
	attached := false
	for {
		event, ok := nextEvent02(ctx, events, shutdown, 40*time.Second)
		if !ok {
			return fmt.Errorf("ATTACH-LIVE: attached up seen %t, and the route announced before the reload never arrived", attached)
		}
		body := eventBody02(event)
		_, kind, _ := eventFacts02(event)
		if kind == eventState {
			state, _ := body["state"].(string)
			if state != "up" {
				return fmt.Errorf("ATTACH-LIVE: session went %q while the reload should keep it up: %.300s", state, event)
			}
			reason, _ := body["reason"].(string)
			if reason != "attached" {
				return fmt.Errorf("ATTACH-LIVE: up with reason %q, want \"attached\" (a restarted session's up): %.300s", reason, event)
			}
			if _, replay := body["initial-replay"]; replay {
				return fmt.Errorf("ATTACH-LIVE: attached up carries an initial-replay token: %.300s", event)
			}
			attached = true
			continue
		}
		if kind != eventUpdate {
			continue
		}
		if !attached {
			return fmt.Errorf("ATTACH-LIVE: UPDATE delivered before the attached up: %.300s", event)
		}
		if strings.Contains(event, "192.168.1.0/24") {
			break
		}
	}
	fmt.Fprintln(os.Stderr, "ATTACH-LIVE: attached up before the route the peer re-sent")
	return waitForShutdown02(ctx, shutdown, 30*time.Second)
}

func eventIsUp02(raw string) bool {
	body := eventBody02(raw)
	_, kind, _ := eventFacts02(raw)
	state, _ := body["state"].(string)
	return kind == eventState && state == "up"
}

func waitUp02(ctx context.Context, events <-chan string, shutdown <-chan struct{}, what string) error {
	deadline := time.Now().Add(25 * time.Second)
	for time.Now().Before(deadline) {
		event, ok := nextEvent02(ctx, events, shutdown, time.Second)
		if ok && eventIsUp02(event) {
			return nil
		}
	}
	return fmt.Errorf("RUNTIME-SUB: the session never came up %s", what)
}

func keepaliveWithin02(ctx context.Context, events <-chan string, shutdown <-chan struct{}, duration time.Duration) bool {
	deadline := time.Now().Add(duration)
	for time.Now().Before(deadline) {
		event, ok := nextEvent02(ctx, events, shutdown, 500*time.Millisecond)
		if !ok {
			continue
		}
		_, kind, _ := eventFacts02(event)
		if kind == "keepalive" {
			return true
		}
	}
	return false
}

func runtimeGrantedReceive02(ctx context.Context, plugin *sdk.Plugin) ([]string, error) {
	raw, err := requireDone02(ctx, plugin, "show event delivery")
	if err != nil {
		return nil, err
	}
	var data struct {
		Peers []struct {
			Peer      string `json:"peer"`
			Processes []struct {
				Process string   `json:"process"`
				Receive []string `json:"receive"`
			} `json:"processes"`
		} `json:"peers"`
	}
	if err := decode02(raw, &data); err != nil {
		return nil, err
	}
	for _, peer := range data.Peers {
		if peer.Peer != addrLoopback {
			continue
		}
		for _, process := range peer.Processes {
			if process.Process == "runtime-subscribe" {
				return process.Receive, nil
			}
		}
	}
	return nil, nil
}

func runtimeSubscribe02(ctx context.Context, plugin *sdk.Plugin, events <-chan string, shutdown <-chan struct{}) error {
	if err := waitUp02(ctx, events, shutdown, "for the first session"); err != nil {
		return err
	}
	if keepaliveWithin02(ctx, events, shutdown, 4*time.Second) {
		return fmt.Errorf("RUNTIME-SUB: fed a keepalive the program never declared")
	}
	fmt.Fprintln(os.Stderr, "RUNTIME-SUB: the program declares state only")
	if _, err := requireDone02(ctx, plugin, "request subscribe peer 127.0.0.1 event keepalive"); err != nil {
		return fmt.Errorf("RUNTIME-SUB: subscribe refused: %w", err)
	}
	if !keepaliveWithin02(ctx, events, shutdown, 8*time.Second) {
		return fmt.Errorf("RUNTIME-SUB: the live subscribe did not take effect")
	}
	fmt.Fprintln(os.Stderr, "RUNTIME-SUB: the live subscribe takes effect at once")
	granted, err := runtimeGrantedReceive02(ctx, plugin)
	if err != nil {
		return err
	}
	slices.Sort(granted)
	if len(granted) != 2 || granted[0] != "keepalive" || granted[1] != eventState {
		return fmt.Errorf("RUNTIME-SUB: configured receive grant changed: %v", granted)
	}
	fmt.Fprintln(os.Stderr, "RUNTIME-SUB: the config index is unchanged by the override")
	if err := os.WriteFile("subscribed.marker", []byte("done"), 0o600); err != nil {
		return err
	}
	cameUp := false
	for {
		if _, err := os.Stat("reloaded.marker"); err == nil {
			break
		}
		event, ok := nextEvent02(ctx, events, shutdown, 500*time.Millisecond)
		cameUp = cameUp || (ok && eventIsUp02(event))
	}
	if !cameUp {
		if err := waitUp02(ctx, events, shutdown, "after the reload"); err != nil {
			return err
		}
	}
	if keepaliveWithin02(ctx, events, shutdown, 4*time.Second) {
		return fmt.Errorf("RUNTIME-SUB: the override outlived the config apply")
	}
	fmt.Fprintln(os.Stderr, "RUNTIME-SUB: the config apply discards the override")
	requestShutdownAsync02(ctx, plugin)
	return waitForShutdown02(ctx, shutdown, 15*time.Second)
}

func peerCounter02(ctx context.Context, plugin *sdk.Plugin, peer, field string) (int, error) {
	peers, err := peerFields02(ctx, plugin, peer)
	if err != nil {
		return 0, err
	}
	row, ok := peers[peer].(map[string]any)
	if !ok {
		return 0, fmt.Errorf("SEND-PERMISSION: no detail row for %s", peer)
	}
	return int02(row[field]), nil
}

func sendPermission02(ctx context.Context, plugin *sdk.Plugin, events <-chan string, shutdown <-chan struct{}) error {
	const permitted = "127.0.0.1"
	const unattached = "127.0.0.2"
	const route = "nhop 101.1.101.1 nlri ipv4/unicast add 203.0.113.0/24"
	if !eorSent02(ctx, plugin, "*", 2, 60) {
		return fmt.Errorf("SEND-PERMISSION: both peers must send their End-of-RIB first")
	}
	before, err := peerCounter02(ctx, plugin, unattached, "updates-sent")
	if err != nil {
		return err
	}
	if _, _, err := plugin.UpdateRoute(ctx, "*", "update text "+route); err != nil {
		return err
	}
	if err := quiesce02(ctx, plugin); err != nil {
		return err
	}
	permittedSent, err := peerCounter02(ctx, plugin, permitted, "updates-sent")
	if err != nil {
		return fmt.Errorf("SEND-PERMISSION: the peer that grants send [ update ] received nothing: %w", err)
	}
	if permittedSent < 1 {
		return errors.New("SEND-PERMISSION: the peer that grants send [ update ] received nothing: <nil>")
	}
	fmt.Fprintln(os.Stderr, "SEND-PERMISSION: announce reached the attached peer")
	after, err := peerCounter02(ctx, plugin, unattached, "updates-sent")
	if err != nil {
		return err
	}
	if after != before {
		return fmt.Errorf("SEND-PERMISSION: the unattached peer was sent %d update(s)", after-before)
	}
	fmt.Fprintln(os.Stderr, "SEND-PERMISSION: the unattached peer was sent nothing")
	_, _, refusalErr := plugin.UpdateRoute(ctx, unattached, "update text "+route)
	refusal := ""
	if refusalErr != nil {
		refusal = refusalErr.Error()
	}
	if !strings.Contains(refusal, "send refused") {
		return fmt.Errorf("SEND-PERMISSION: the unattached peer accepted an announce: %q", refusal)
	}
	if !strings.Contains(refusal, "send-permission-injector") || !strings.Contains(refusal, unattached) {
		return fmt.Errorf("SEND-PERMISSION: the refusal names neither the peer nor the process: %q", refusal)
	}
	fmt.Fprintln(os.Stderr, "SEND-PERMISSION: refused for the unattached peer")
	if event, ok := nextEvent02(ctx, events, shutdown, 2*time.Second); ok {
		return fmt.Errorf("SEND-PERMISSION: a send-only binding was fed an event: %q", event)
	}
	fmt.Fprintln(os.Stderr, "SEND-PERMISSION: a send-only binding is fed nothing")
	requestShutdownAsync02(ctx, plugin)
	return waitForShutdown02(ctx, shutdown, 15*time.Second)
}

func unattachedServed02(ctx context.Context, _ *sdk.Plugin, events <-chan string, shutdown <-chan struct{}) error {
	seen := make(map[string]bool)
	for !seen["state"] || !seen["update"] {
		event, ok := nextEvent02(ctx, events, shutdown, 20*time.Second)
		if !ok {
			return fmt.Errorf("UNATTACHED-SERVED: the peer attaches this program and fed it %v", seen)
		}
		_, kind, _ := eventFacts02(event)
		if kind != "" {
			seen[kind] = true
		}
	}
	fmt.Fprintln(os.Stderr, "UNATTACHED-SERVED: fed by the peer that attaches it")
	return waitForShutdown02(ctx, shutdown, 30*time.Second)
}

func unattachedSilent02(ctx context.Context, plugin *sdk.Plugin, events <-chan string, shutdown <-chan struct{}) error {
	if event, ok := nextEvent02(ctx, events, shutdown, 5*time.Second); ok {
		return fmt.Errorf("UNATTACHED-SILENT: fed an event no peer attached it for: %.200s", event)
	}
	fmt.Fprintln(os.Stderr, "UNATTACHED-SILENT: fed nothing, as no peer attaches it")
	requestShutdownAsync02(ctx, plugin)
	if event, ok := nextEvent02(ctx, events, shutdown, 20*time.Second); ok {
		return fmt.Errorf("UNATTACHED-SILENT: fed a late event no peer attached it for: %.200s", event)
	}
	return nil
}

func regularFileExists02(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

func dynamicGroupWait02(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("unexpected arguments: %v", args)
	}
	return waitForMarker02(ctx, dynamicGroupMarker02)
}

// waitFileDriver02 waits for the marker its producer names, independently of
// the dynamic-group fixture's fixed marker.
func waitFileDriver02(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("wait-file requires one readiness marker path")
	}
	return waitForMarker02(ctx, args[0])
}

func waitForMarker02(ctx context.Context, path string) error {
	for range 201 {
		if regularFileExists02(path) {
			return nil
		}
		timer := time.NewTimer(100 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	return fmt.Errorf("marker %s did not appear before the wait deadline", path)
}

func waitForFile02(ctx context.Context, path string) error {
	for {
		if regularFileExists02(path) {
			return nil
		}
		timer := time.NewTimer(100 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func copyFile02(source, destination string) error {
	body, err := os.ReadFile(source) //nolint:gosec // the path is the fixture's own scratch file
	if err != nil {
		return err
	}
	mode := os.FileMode(0o644)
	if info, statErr := os.Stat(source); statErr == nil {
		mode = info.Mode().Perm()
	}
	return os.WriteFile(destination, body, mode)
}

func signalDaemon02() error {
	body, err := os.ReadFile("daemon.pid")
	if err != nil {
		return err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(body)))
	if err != nil {
		return fmt.Errorf("parse daemon.pid: %w", err)
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return process.Signal(syscall.SIGHUP)
}

func runtimeSubscribeTrigger02(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("runtime subscribe trigger fixture takes no arguments")
	}
	if err := waitForFile02(ctx, "daemon.pid"); err != nil {
		return err
	}
	if err := waitForFile02(ctx, "subscribed.marker"); err != nil {
		return err
	}
	if err := copyFile02("config2.conf", "ze-bgp.conf"); err != nil {
		return err
	}
	if err := signalDaemon02(); err != nil {
		return err
	}
	return os.WriteFile("reloaded.marker", nil, 0o600)
}

// labeledWithdrawEvent02 decodes every received UPDATE event and prints one
// line for each ipv4/mpls-label entry to stderr, "LABELED-EVENT: <action>"
// followed by each NLRI element, until an entry with action del arrives. A bare
// string element prints as the prefix itself; an object element prints as
// "object", so a withdrawal handed over in the announcement shape cannot match
// "LABELED-EVENT: del 10.0.0.0/8". The line carries no quote because ze relays
// plugin stderr through its logger, which escapes quotes.
func labeledWithdrawEvent02(ctx context.Context, plugin *sdk.Plugin, events <-chan string, shutdown <-chan struct{}) error {
	for {
		event, ok := nextEvent02(ctx, events, shutdown, 20*time.Second)
		if !ok {
			return fmt.Errorf("LABELED-EVENT: no ipv4/mpls-label withdrawal event arrived")
		}
		_, kind, direction := eventFacts02(event)
		if kind != eventUpdate {
			continue
		}
		if direction != directionReceived {
			continue
		}
		if labeledEntries02(eventBody02(event)) {
			break
		}
	}
	if err := quiesce02(ctx, plugin); err != nil {
		return err
	}
	requestShutdownAsync02(ctx, plugin)
	return waitForShutdown02(ctx, shutdown, 15*time.Second)
}

// labeledEntries02 prints one LABELED-EVENT line per ipv4/mpls-label entry of
// the update in body and reports whether any entry was a del.
func labeledEntries02(body map[string]any) bool {
	update, _ := body["update"].(map[string]any)
	nlri, _ := update["nlri"].(map[string]any)
	entries, _ := nlri["ipv4/mpls-label"].([]any)
	withdrawn := false
	for _, raw := range entries {
		entry, _ := raw.(map[string]any)
		action, _ := entry["action"].(string)
		var line strings.Builder
		line.WriteString("LABELED-EVENT: ")
		line.WriteString(action)
		elements, _ := entry["nlri"].([]any)
		for _, element := range elements {
			line.WriteByte(' ')
			prefix, isString := element.(string)
			if !isString {
				prefix = "object"
			}
			line.WriteString(prefix)
		}
		fmt.Fprintln(os.Stderr, line.String())
		if action == "del" {
			withdrawn = true
		}
	}
	return withdrawn
}
