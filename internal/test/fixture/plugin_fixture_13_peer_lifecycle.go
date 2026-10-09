// Design: docs/architecture/api/commands.md — `create bgp peer`, `delete bgp peer`
// Overview: plugin_fixture_13.go — the REST and lifecycle fixtures of group 13
// Related: register_peer_create_delete_rib.go — the registration of this driver

package fixture

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// The addresses and the prefix test/plugin/api-peer-create-delete-rib.ci wires:
// the configured peer the route is forwarded to, the peer the scenario creates
// at runtime, and the prefix the created peer announces.
const (
	lifecycleConfiguredPeer = "127.0.0.1"
	lifecycleCreatedPeer    = "127.0.0.2"
	lifecyclePrefix         = "192.0.2.0/24"
)

// peerCreateDeleteRIB13 drives the runtime peer lifecycle through the command
// dispatcher a plugin and an operator share, and reads every effect back from
// the daemon rather than trusting an answer: the peer table, the RIB, the
// configuration file, a configuration commit and a reload.
//
// The wire half lives in the .ci: the configured peer must receive the created
// peer's route and then its WITHDRAW, and the created peer's listener takes one
// TCP connection only, so a commit that reset its session fails the run.
func peerCreateDeleteRIB13(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("peer lifecycle fixture requires the REST port")
	}
	driver := restLifecycle13{
		base:    "http://127.0.0.1:" + args[0] + "/api/v1",
		headers: map[string]string{"Authorization": "Bearer secret", headerContentType: mediaTypeJSON},
	}
	return observe13(ctx, "lifecycle-driver", func(ctx context.Context, plugin *sdk.Plugin) error {
		// Bounded: an event that finds the channel full is dropped, and the
		// channel is drained before the one wait that reads it.
		events := make(chan string, 256)
		plugin.OnEvent(func(event string) error {
			select {
			case events <- event:
			default:
			}
			return nil
		})
		if !lifecycleWaitState(ctx, plugin, lifecycleConfiguredPeer, "established") {
			return errors.New("the configured peer never reached established")
		}
		if _, ok := waitHTTP13(ctx, driver.base+"/commands", driver.headers, 50, 300*time.Millisecond); !ok {
			return errors.New("REST API did not respond")
		}
		before, err := os.ReadFile(fileBGPConf)
		if err != nil {
			return fmt.Errorf("read %s: %w", fileBGPConf, err)
		}
		if err := lifecycleCreate(ctx, plugin); err != nil {
			return err
		}
		if err := lifecycleRoutesReachTheRIB(ctx, plugin); err != nil {
			return err
		}
		if err := lifecycleFileUntouched(ctx, driver, before); err != nil {
			return err
		}
		if err := lifecycleSurvivesUnrelatedCommit(ctx, plugin, driver); err != nil {
			return err
		}
		if err := lifecycleDeleteWithdraws(ctx, plugin, events); err != nil {
			return err
		}
		return lifecycleDeleteConfiguredPeer(ctx, plugin)
	})
}

// lifecycleCreate holds AC-1 and AC-4: the answer names the peer, its AS and
// the outcome, and the peer table then holds the peer, established, which
// proves it dialed the address.
func lifecycleCreate(ctx context.Context, plugin *sdk.Plugin) error {
	data, err := plugin01RequireDone(ctx, plugin,
		"create bgp peer "+lifecycleCreatedPeer+" asn 65002 local-as 65000 local-address "+lifecycleCreatedPeer+
			" router-id 1.2.3.4 accept false attach bgp-rib,rs,lifecycle-driver")
	if err != nil {
		return err
	}
	if data["peer"] != lifecycleCreatedPeer {
		return fmt.Errorf("create bgp peer answered peer %v, want %s: %v", data["peer"], lifecycleCreatedPeer, data)
	}
	if fmt.Sprint(data["remote-as"]) != "65002" {
		return fmt.Errorf("create bgp peer answered remote-as %v, want 65002: %v", data["remote-as"], data)
	}
	if data["message"] != "peer created" {
		return fmt.Errorf("create bgp peer answered outcome %v, want peer created: %v", data["message"], data)
	}
	if !lifecycleWaitState(ctx, plugin, lifecycleCreatedPeer, "established") {
		return fmt.Errorf("the created peer %s never reached established", lifecycleCreatedPeer)
	}
	fmt.Fprintln(os.Stderr, "OK: the created peer is listed and established")
	return nil
}

// lifecycleRoutesReachTheRIB holds AC-2: the route the created peer announces
// is in its own RIB view, in the RIB, and its best path names that peer.
func lifecycleRoutesReachTheRIB(ctx context.Context, plugin *sdk.Plugin) error {
	for _, command := range []string{
		"show bgp peer " + lifecycleCreatedPeer + " rib received",
		"show bgp rib",
	} {
		if !lifecycleWaitText(ctx, plugin, command, func(text string) bool { return strings.Contains(text, lifecyclePrefix) }) {
			return fmt.Errorf("%s never answered %s", command, lifecyclePrefix)
		}
	}
	best := ""
	if !lifecycleWaitText(ctx, plugin, "show bgp rib best", func(text string) bool {
		best = text
		return strings.Contains(text, lifecyclePrefix) && strings.Contains(text, lifecycleCreatedPeer)
	}) {
		return fmt.Errorf("show bgp rib best names no path for %s from %s: %.400s", lifecyclePrefix, lifecycleCreatedPeer, best)
	}
	fmt.Fprintln(os.Stderr, "OK: the created peer's route is in the RIB and is the best path")
	return nil
}

// lifecycleFileUntouched holds AC-3: creating a peer writes nothing, so the
// file is byte-identical and the resolved configuration names no such peer.
func lifecycleFileUntouched(ctx context.Context, driver restLifecycle13, before []byte) error {
	after, err := os.ReadFile(fileBGPConf)
	if err != nil {
		return fmt.Errorf("read %s: %w", fileBGPConf, err)
	}
	if !bytes.Equal(before, after) {
		return fmt.Errorf("creating a peer changed the configuration file:\n%s", after)
	}
	// The running configuration is what `show config` prints. `show config
	// dump` is a local CLI command no plugin can dispatch, so the read goes
	// through the REST view of the same tree.
	running, err := driver.request(ctx, http.MethodGet, "/config/running", nil)
	if err != nil {
		return err
	}
	if text := fmt.Sprint(running); strings.Contains(text, lifecycleCreatedPeer) {
		return fmt.Errorf("the running configuration names the created peer: %.400s", text)
	}
	fmt.Fprintln(os.Stderr, "OK: the configuration file and the running configuration name no created peer")
	return nil
}

// lifecycleSurvivesUnrelatedCommit holds AC-9: a commit that names no peer
// leaves the created peer up, on the same session, with its route.
func lifecycleSurvivesUnrelatedCommit(ctx context.Context, plugin *sdk.Plugin, driver restLifecycle13) error {
	sid, err := driver.createSession(ctx)
	if err != nil {
		return err
	}
	if err := driver.set(ctx, sid, "system.peeringdb.margin", "20"); err != nil {
		return err
	}
	if err := driver.commit(ctx, sid, "unrelated leaf"); err != nil {
		return err
	}
	if err := lifecycleCreatedPeerKept(ctx, plugin, "an unrelated commit"); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "OK: an unrelated commit keeps the created peer and its routes")
	// A SIGHUP reload reads the file, which does not declare the created peer
	// either, so it is the same question asked through the other entry point.
	if err := plugin01SignalDaemonReload(); err != nil {
		return err
	}
	if err := lifecycleCreatedPeerKept(ctx, plugin, "a reload"); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "OK: a reload keeps the created peer and its routes")
	return nil
}

// lifecycleCreatedPeerKept answers an error unless the created peer stays
// established, on the same session, with its route in its Adj-RIB-In. after
// names the step that ran, for the error.
//
// The created peer's listener takes one connection, so a step that reset the
// session leaves it down for good: three seconds of established is the same
// session, not a new one.
func lifecycleCreatedPeerKept(ctx context.Context, plugin *sdk.Plugin, after string) error {
	for range 12 {
		state, present := peerState13(ctx, plugin, lifecycleCreatedPeer)
		if !present {
			return fmt.Errorf("after %s the created peer is gone", after)
		}
		if state != "established" {
			return fmt.Errorf("after %s the created peer is %q", after, state)
		}
		time.Sleep(plugin01PollDelay)
	}
	data, err := plugin01RequireDone(ctx, plugin, "show bgp peer "+lifecycleCreatedPeer+" rib received")
	if err != nil {
		return err
	}
	if !strings.Contains(fmt.Sprint(data), lifecyclePrefix) {
		return fmt.Errorf("after %s the created peer's Adj-RIB-In lost %s", after, lifecyclePrefix)
	}
	return nil
}

// lifecycleDeleteWithdraws holds AC-5, AC-6 and AC-8: the peer leaves the peer
// table, its route leaves the RIB, and a plugin subscribed to updates is told
// the route is withdrawn. The WITHDRAW on the wire is asserted in the .ci.
func lifecycleDeleteWithdraws(ctx context.Context, plugin *sdk.Plugin, events chan string) error {
	// A config apply discards every runtime subscription
	// (Server.DiscardRuntimeSubscriptions), and AC-9 has just committed and
	// reloaded, so the subscription is taken here, where its events are read.
	if _, err := plugin01RequireDone(ctx, plugin, "request subscribe bgp event update"); err != nil {
		return err
	}
	for drained := false; !drained; {
		select {
		case <-events:
		default:
			drained = true
		}
	}
	if _, err := plugin01RequireDone(ctx, plugin, "delete bgp peer "+lifecycleCreatedPeer); err != nil {
		return err
	}
	if !lifecycleWaitGone(ctx, plugin, lifecycleCreatedPeer) {
		return fmt.Errorf("%s is still a peer after delete bgp peer", lifecycleCreatedPeer)
	}
	if !lifecycleWaitText(ctx, plugin, "show bgp rib", func(text string) bool { return !strings.Contains(text, lifecyclePrefix) }) {
		return fmt.Errorf("show bgp rib still answers %s after the peer was deleted", lifecyclePrefix)
	}
	timer := time.NewTimer(15 * time.Second)
	defer timer.Stop()
	// The events seen while waiting go into the failure, so a red names what
	// the plugin was told instead of only what it was not.
	seen, last := 0, ""
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			return fmt.Errorf("no update event withdrew %s within 15s (%d update events seen, last %q)", lifecyclePrefix, seen, last)
		case event := <-events:
			seen, last = seen+1, event
			if strings.Contains(event, lifecyclePrefix) && strings.Contains(event, `"action":"del"`) {
				fmt.Fprintln(os.Stderr, "OK: the deleted peer's route left the RIB and a plugin was told")
				return nil
			}
		}
	}
}

// lifecycleDeleteConfiguredPeer holds AC-7: deleting a peer the file declares
// writes nothing, and a reload of that file brings the peer back.
func lifecycleDeleteConfiguredPeer(ctx context.Context, plugin *sdk.Plugin) error {
	before, err := os.ReadFile(fileBGPConf)
	if err != nil {
		return fmt.Errorf("read %s: %w", fileBGPConf, err)
	}
	if _, err := plugin01RequireDone(ctx, plugin, "delete bgp peer "+lifecycleConfiguredPeer); err != nil {
		return err
	}
	if !lifecycleWaitGone(ctx, plugin, lifecycleConfiguredPeer) {
		return fmt.Errorf("%s is still a peer after delete bgp peer", lifecycleConfiguredPeer)
	}
	after, err := os.ReadFile(fileBGPConf)
	if err != nil {
		return fmt.Errorf("read %s: %w", fileBGPConf, err)
	}
	if !bytes.Equal(before, after) {
		return fmt.Errorf("deleting a configured peer changed the configuration file:\n%s", after)
	}
	if err := plugin01SignalDaemonReload(); err != nil {
		return err
	}
	if _, present := waitPeerPresent13(ctx, plugin, lifecycleConfiguredPeer); !present {
		return fmt.Errorf("a reload of the unchanged file did not bring %s back", lifecycleConfiguredPeer)
	}
	fmt.Fprintln(os.Stderr, "OK: deleting a configured peer leaves the file, and a reload brings it back")
	return nil
}

// lifecycleWaitState polls the peer table until address reports state.
func lifecycleWaitState(ctx context.Context, plugin *sdk.Plugin, address, want string) bool {
	return Poll(ctx, 80, plugin01PollDelay, func() bool {
		state, present := peerState13(ctx, plugin, address)
		if !present {
			return false
		}
		return state == want
	})
}

// lifecycleWaitGone polls the peer table until address is no longer in it.
func lifecycleWaitGone(ctx context.Context, plugin *sdk.Plugin, address string) bool {
	return Poll(ctx, 40, plugin01PollDelay, func() bool {
		current, status, err := plugin01DispatchMap(ctx, plugin, "show bgp peer list")
		if err != nil {
			return false
		}
		if status != rpc.StatusDone {
			return false
		}
		_, exists := plugin01PeerRows(current)[address]
		return !exists
	})
}

// lifecycleWaitText polls command until its answer, as text, satisfies match.
func lifecycleWaitText(ctx context.Context, plugin *sdk.Plugin, command string, match func(string) bool) bool {
	return Poll(ctx, 40, plugin01PollDelay, func() bool {
		data, status, err := plugin01DispatchMap(ctx, plugin, command)
		if err != nil {
			return false
		}
		if status != rpc.StatusDone {
			return false
		}
		return match(fmt.Sprint(data))
	})
}
