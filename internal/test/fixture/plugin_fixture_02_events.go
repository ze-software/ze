package fixture

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

type eventScenario02 func(context.Context, *sdk.Plugin, <-chan string, <-chan struct{}) error

func init() {
	Register("plugin/attach-process-dynamic-group", dynamicGroupDriver02)
	Register("plugin/attach-process-dynamic-group-wait", dynamicGroupWait02)
	Register("plugin/wait-file", dynamicGroupWait02)
	Register("plugin/attach-process-receive-filter-state", eventObserver02("receive-filter-state", []string{"update", "state"}, receiveFilterState02))
	Register("plugin/attach-process-receive-filter-update", eventObserver02("receive-filter-update", []string{"update", "state"}, receiveFilterUpdate02))
	Register("plugin/attach-process-reload-kept", eventObserver02("reload-kept", []string{"state"}, reloadKept02))
	Register("plugin/attach-process-reload-added", eventObserver02("reload-added", []string{"state"}, reloadAdded02))
	Register("plugin/attach-process-reload-trigger", reloadTrigger02)
	Register("plugin/attach-process-runtime-subscribe", eventObserver02("runtime-subscribe", []string{"state"}, runtimeSubscribe02))
	Register("plugin/attach-process-runtime-subscribe-trigger", runtimeSubscribeTrigger02)
	Register("plugin/attach-process-send-permission", eventObserver02("send-permission-injector", []string{"update", "state"}, sendPermission02))
	Register("plugin/attach-process-unattached-served", eventObserver02("unattached-served", []string{"update", "state"}, unattachedServed02))
	Register("plugin/attach-process-unattached-silent", eventObserver02("unattached-silent", []string{"update", "state"}, unattachedSilent02))
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
		defer plugin.Close() //nolint:errcheck

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
		plugin.OnBye(func(string) { shutdownOnce.Do(func() { close(shutdown) }) })
		result := make(chan error, 1)
		plugin.OnAllPluginsReady(func() error {
			go func() {
				scenarioErr := scenario(runCtx, plugin, eventCh, shutdown)
				if scenarioErr != nil {
					requestShutdownAsync02(runCtx, plugin)
				}
				result <- scenarioErr
			}()
			return nil
		})

		runErr := plugin.Run(runCtx, sdk.Registration{})
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
	case "dyn", "inherits":
		if kind != "state" {
			return fmt.Sprintf("%s is granted state alone by its group and was fed %s (%s)", who, kind, direction)
		}
	case "restates":
		if kind != "update" {
			return fmt.Sprintf("restates states its own list, update-received, and was fed %s", kind)
		}
		if direction != "received" {
			return "restates is granted update-received and was fed an update ze SENT"
		}
	default:
		return fmt.Sprintf("an event arrived for a peer no attach block covers: %q", who)
	}
	return ""
}

func dynamicGroupDriver02(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("dynamic group observer requires an absolute marker path")
	}
	marker := args[0]
	_ = os.Remove(marker)
	scenario := func(ctx context.Context, plugin *sdk.Plugin, events <-chan string, shutdown <-chan struct{}) error {
		return dynamicGroup02(ctx, plugin, events, shutdown, marker)
	}
	return eventObserver02("dyn-group-observer", []string{"update", "state"}, scenario)(ctx, nil)
}

func dynamicGroup02(ctx context.Context, plugin *sdk.Plugin, events <-chan string, shutdown <-chan struct{}, marker string) error {
	if err := os.WriteFile(marker, []byte("ready"), 0o644); err != nil {
		return err
	}
	defer os.Remove(marker) //nolint:errcheck
	seen := make(map[string]bool)
	for len(seen) != 3 {
		event, ok := nextEvent02(ctx, events, shutdown, 25*time.Second)
		if !ok {
			missing := make([]string, 0, 3)
			for _, wanted := range []string{"dyn", "inherits", "restates"} {
				if !seen[wanted] {
					missing = append(missing, wanted)
				}
			}
			return fmt.Errorf("DYN-GROUP: never fed %v", missing)
		}
		name, kind, direction := eventFacts02(event)
		who := name
		if strings.HasPrefix(name, "dyn-") {
			who = "dyn"
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
			who = "dyn"
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
		if kind == "update" {
			return fmt.Errorf("FILTER-STATE: update leaked to a process granted state only (%s)", direction)
		}
		if kind == "state" && !seenState {
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
		if kind == "state" {
			return fmt.Errorf("FILTER-UPDATE: state leaked to a process granted update-received")
		}
		if kind == "update" && direction == "sent" {
			return fmt.Errorf("FILTER-UPDATE: a SENT update reached an update-received grant")
		}
		if kind == "update" {
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
	return kind == "state"
}

func reloadKept02(ctx context.Context, _ *sdk.Plugin, events <-chan string, shutdown <-chan struct{}) error {
	for {
		event, ok := nextEvent02(ctx, events, shutdown, 25*time.Second)
		if !ok {
			return fmt.Errorf("RELOAD-KEPT: never fed before the reload")
		}
		if stateEvent02(event) {
			break
		}
	}
	for {
		event, ok := nextEvent02(ctx, events, shutdown, 25*time.Second)
		if !ok {
			return fmt.Errorf("RELOAD-KEPT: the surviving edge was not fed after the reload")
		}
		if stateEvent02(event) {
			if _, err := os.Stat("reloaded.marker"); err == nil {
				break
			}
		}
	}
	fmt.Fprintln(os.Stderr, "RELOAD-KEPT: fed before and after the reload")
	if err := os.WriteFile("kept.marker", []byte("done"), 0o644); err != nil {
		return err
	}
	return waitForShutdown02(ctx, shutdown, 30*time.Second)
}

func reloadAdded02(ctx context.Context, plugin *sdk.Plugin, events <-chan string, shutdown <-chan struct{}) error {
	deadline := time.Now().Add(25 * time.Second)
	fed := false
	for !fed && time.Now().Before(deadline) {
		event, ok := nextEvent02(ctx, events, shutdown, time.Second)
		if !ok {
			continue
		}
		if _, err := os.Stat("reloaded.marker"); err != nil {
			return fmt.Errorf("RELOAD-ADDED: fed before the config attached it: %.200s", event)
		}
		fed = stateEvent02(event)
	}
	if !fed {
		return fmt.Errorf("RELOAD-ADDED: the reload attached it and it was fed nothing")
	}
	fmt.Fprintln(os.Stderr, "RELOAD-ADDED: fed only after the reload attached it")
	for {
		if _, err := os.Stat("kept.marker"); err == nil {
			break
		}
		_, _ = nextEvent02(ctx, events, shutdown, 500*time.Millisecond)
		if time.Now().After(deadline.Add(20 * time.Second)) {
			return fmt.Errorf("RELOAD-ADDED: the kept edge never reported")
		}
	}
	requestShutdownAsync02(ctx, plugin)
	return waitForShutdown02(ctx, shutdown, 15*time.Second)
}

func eventIsUp02(raw string) bool {
	body := eventBody02(raw)
	_, kind, _ := eventFacts02(raw)
	state, _ := body["state"].(string)
	return kind == "state" && state == "up"
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
		if peer.Peer != "127.0.0.1" {
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
	sort.Strings(granted)
	if len(granted) != 2 || granted[0] != "keepalive" || granted[1] != "state" {
		return fmt.Errorf("RUNTIME-SUB: configured receive grant changed: %v", granted)
	}
	fmt.Fprintln(os.Stderr, "RUNTIME-SUB: the config index is unchanged by the override")
	if err := os.WriteFile("subscribed.marker", []byte("done"), 0o644); err != nil {
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
	for {
		event, ok := nextEvent02(ctx, events, shutdown, 20*time.Second)
		if !ok {
			break
		}
		return fmt.Errorf("UNATTACHED-SILENT: fed a late event no peer attached it for: %.200s", event)
	}
	return nil
}

func regularFileExists02(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

func dynamicGroupWait02(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("wait-file fixture requires an absolute marker path")
	}
	path := args[0]
	for attempt := 0; attempt <= 200; attempt++ {
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
	body, err := os.ReadFile(source)
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

func reloadTrigger02(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("reload trigger fixture takes no arguments")
	}
	if err := waitForFile02(ctx, "daemon.pid"); err != nil {
		return err
	}
	if err := waitForFile02(ctx, "daemon.ready"); err != nil {
		return err
	}
	if err := copyFile02("config2.conf", "ze-bgp.conf"); err != nil {
		return err
	}
	if err := signalDaemon02(); err != nil {
		return err
	}
	return os.WriteFile("reloaded.marker", nil, 0o644)
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
	return os.WriteFile("reloaded.marker", nil, 0o644)
}
