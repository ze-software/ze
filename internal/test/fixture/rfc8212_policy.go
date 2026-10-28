// Design: docs/functional-tests.md -- real-process default-policy wiring probes.
// Related: test/draft/plugin/rfc8212-*.ci -- destination echoes and checked socket receipts.
package fixture

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// rfc8212PolicyWire is safe for concurrent event delivery and scenario polling.
// Forbidden announcements remain failures even if a later withdrawal hides them.
type rfc8212PolicyWire struct {
	mu          sync.Mutex
	routes      map[string]map[string]bool
	withdrawals map[string]map[string]bool
	forbidden   map[string]map[string]bool
	failure     error
}

func rfc8212PolicyDriver(mode string) Driver {
	return func(ctx context.Context, args []string) error {
		if len(args) != 0 {
			return errors.New("RFC8212 route observer takes no arguments")
		}
		state := &rfc8212PolicyWire{
			routes:      make(map[string]map[string]bool),
			withdrawals: make(map[string]map[string]bool),
			forbidden:   make(map[string]map[string]bool),
		}
		if mode == "import" {
			state.forbidden["127.0.0.2"] = map[string]bool{"10.82.1.0/24": true, "10.82.4.0/24": true, "2001:db8:82::/48": true}
		}
		if mode == "export" {
			state.forbidden["127.0.0.2"] = map[string]bool{"10.82.1.0/24": true, "10.82.10.0/24": true, "10.82.11.0/24": true, "10.82.12.0/24": true, "10.82.13.0/24": true, "10.82.14.0/24": true, "10.82.15.0/24": true, "10.82.16.0/24": true}
		}
		return observeConfigured(ctx, "rfc8212-policy", &sdk.Registration{}, func(plugin *sdk.Plugin) error {
			plugin.OnEvent(state.receive)
			plugin.SetStartupSubscriptions([]string{"update"}, []string{"127.0.0.2", "127.0.0.3"}, "full")
			return nil
		}, func(ctx context.Context, plugin *sdk.Plugin) error {
			if mode == "export" {
				return rfc8212PolicyExport(ctx, plugin, state)
			}
			return rfc8212PolicyImport(ctx, plugin, state, mode == "ibgp")
		})
	}
}

func (state *rfc8212PolicyWire) receive(event string) error {
	var envelope struct {
		BGP struct {
			Message struct {
				Type      string `json:"type"`
				Direction string `json:"direction"`
			} `json:"message"`
			Peer struct {
				Remote struct {
					Address string `json:"address"`
				} `json:"remote"`
			} `json:"peer"`
			Update struct {
				NLRI map[string][]struct {
					Action string   `json:"action"`
					NLRI   []string `json:"nlri"`
				} `json:"nlri"`
			} `json:"update"`
		} `json:"bgp"`
	}
	if err := json.Unmarshal([]byte(event), &envelope); err != nil {
		return err
	}
	bgp := &envelope.BGP
	if bgp.Message.Type != "update" {
		return nil
	}
	if bgp.Message.Direction != "received" {
		return nil
	}
	peer := bgp.Peer.Remote.Address
	if peer != "127.0.0.2" && peer != "127.0.0.3" {
		return nil
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.routes[peer] == nil {
		state.routes[peer] = make(map[string]bool)
		state.withdrawals[peer] = make(map[string]bool)
	}
	for _, operations := range bgp.Update.NLRI {
		for _, operation := range operations {
			for _, prefix := range operation.NLRI {
				switch operation.Action {
				case "add":
					if state.forbidden[peer][prefix] {
						state.failure = fmt.Errorf("forbidden announcement reached %s: %s", peer, prefix)
					}
					state.routes[peer][prefix] = true
				case "del":
					delete(state.routes[peer], prefix)
					state.withdrawals[peer][prefix] = true
				default:
					state.failure = fmt.Errorf("unexpected wire action %q", operation.Action)
				}
			}
		}
	}
	return nil
}

func (state *rfc8212PolicyWire) wait(ctx context.Context, peer string, withdrawn string, prefixes ...string) error {
	matched := Poll(ctx, 80, liveStagesPollDelay, func() bool {
		state.mu.Lock()
		defer state.mu.Unlock()
		if state.failure != nil {
			return true
		}
		if withdrawn != "" && !state.withdrawals[peer][withdrawn] {
			return false
		}
		if len(state.routes[peer]) != len(prefixes) {
			return false
		}
		for _, prefix := range prefixes {
			if !state.routes[peer][prefix] {
				return false
			}
		}
		return true
	})
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.failure != nil {
		return state.failure
	}
	if !matched {
		return fmt.Errorf("%s wire routes=%v withdrawals=%v; want routes=%v withdrawal=%s", peer, state.routes[peer], state.withdrawals[peer], prefixes, withdrawn)
	}
	return nil
}

func rfc8212PolicyImport(ctx context.Context, plugin *sdk.Plugin, state *rfc8212PolicyWire, ibgp bool) error {
	count := 4
	if ibgp {
		count = 2
	}
	if !waitPeersEOR03(ctx, plugin, count) {
		return errors.New("policy peers did not finish initial synchronization")
	}
	if !ibgp {
		if err := rfc8212PolicyDualStack(ctx, plugin); err != nil {
			return err
		}
	}
	// Let the rejected source finish, then establish the accepted control.
	// The absent source consumes that control before sending its subject.
	peers := []string{"127.0.0.4", "127.0.0.3", "127.0.0.1"}
	if ibgp {
		peers = []string{"127.0.0.1"}
	}
	for _, peer := range peers {
		before, err := peerCounter03(ctx, plugin, peer, "updates-received")
		if err != nil {
			return err
		}
		if err := liveStagesRoute(ctx, plugin, peer+" update text nhop 192.0.2.1 nlri add 192.168.1.0/24"); err != nil {
			return err
		}
		if !Poll(ctx, 80, liveStagesPollDelay, func() bool {
			got, err := peerCounter03(ctx, plugin, peer, "updates-received")
			return err == nil && got >= before+1
		}) {
			return fmt.Errorf("%s did not send the import subject", peer)
		}
	}
	if err := quiesce03(ctx, plugin); err != nil {
		return err
	}
	if err := liveStagesRoute(ctx, plugin, "127.0.0.2 update text nhop 192.0.2.1 nlri add 192.168.250.0/24"); err != nil {
		return err
	}
	wanted := "10.82.3.0/24"
	if ibgp {
		wanted = "10.82.1.0/24"
	}
	wantedRoutes := []string{wanted, "192.168.250.0/24"}
	if !ibgp {
		wantedRoutes = append(wantedRoutes, "2001:db8:83::/48")
	}
	if err := state.wait(ctx, "127.0.0.2", "", wantedRoutes...); err != nil {
		return err
	}
	if !ibgp {
		if err := rfc8212PolicyBest(ctx, plugin, "2001:db8:83::/48", true); err != nil {
			return err
		}
	}
	for _, prefix := range []string{"10.82.1.0/24", "10.82.3.0/24", "10.82.4.0/24", "2001:db8:82::/48"} {
		if err := rfc8212PolicyBest(ctx, plugin, prefix, prefix == wanted); err != nil {
			return err
		}
	}
	// The accepted source's explicit withdrawal must remove its selected route
	// and its advertisement, rather than merely empty a pre-policy store.
	accepted := "127.0.0.3"
	if ibgp {
		accepted = "127.0.0.1"
	}
	if err := liveStagesRoute(ctx, plugin, accepted+" update text nhop 192.0.2.1 nlri add 192.168.2.0/24"); err != nil {
		return err
	}
	if err := state.wait(ctx, "127.0.0.2", wanted, "192.168.250.0/24"); err != nil {
		return err
	}
	if err := rfc8212PolicyBest(ctx, plugin, wanted, false); err != nil {
		return err
	}
	if !ibgp {
		if err := state.wait(ctx, "127.0.0.2", "2001:db8:83::/48", "192.168.250.0/24"); err != nil {
			return err
		}
		if err := rfc8212PolicyBest(ctx, plugin, "2001:db8:83::/48", false); err != nil {
			return err
		}
	}
	if ibgp {
		if err := quiesce03(ctx, plugin); err != nil {
			return err
		}
		// The native sink MUST keep checking through this same-socket fence;
		// the observer MUST require its receipt and the exact surviving map
		// before shutdown, not trust receipts sent before a late withdrawal.
		if err := liveStagesRoute(ctx, plugin, "127.0.0.2 update text nlri del 192.168.251.0/24"); err != nil {
			return err
		}
		if err := state.wait(ctx, "127.0.0.2", "192.168.251.0/24", "192.168.250.0/24"); err != nil {
			return err
		}
	}
	return policyFinalSessions(ctx, plugin, count)
}

// rfc8212PolicyDualStack checks negotiated capabilities, not configured intent.
// Both EOR writes must finish before any test route is released to the sources.
func rfc8212PolicyDualStack(ctx context.Context, plugin *sdk.Plugin) error {
	var result struct {
		Peers map[string]struct {
			EORSent      int `json:"eor-sent"`
			Capabilities struct {
				Complete bool     `json:"negotiation-complete"`
				Families []string `json:"families"`
			} `json:"capabilities"`
		} `json:"peers"`
	}
	var queryErr error
	matched := Poll(ctx, 60, liveStagesPollDelay, func() bool {
		var status string
		status, queryErr = Dispatch(ctx, plugin, "show bgp peer * detail", &result)
		if queryErr != nil {
			return false
		}
		if status != statusDone {
			queryErr = fmt.Errorf("dual-stack detail status=%s", status)
			return false
		}
		for _, address := range []string{"127.0.0.1", "127.0.0.2", "127.0.0.3"} {
			row, found := result.Peers[address]
			if !found {
				return false
			}
			if !row.Capabilities.Complete {
				return false
			}
			if row.EORSent < 2 {
				return false
			}
			if len(row.Capabilities.Families) != 2 {
				return false
			}
			ipv4, ipv6 := false, false
			for _, family := range row.Capabilities.Families {
				if family == "ipv4/unicast" {
					ipv4 = true
				}
				if family == "ipv6/unicast" {
					ipv6 = true
				}
			}
			if !ipv4 {
				return false
			}
			if !ipv6 {
				return false
			}
		}
		return true
	})
	if queryErr != nil {
		return queryErr
	}
	if !matched {
		return fmt.Errorf("sources and sink did not negotiate both families and send both EORs: %+v", result.Peers)
	}
	return nil
}

func rfc8212PolicyBest(ctx context.Context, plugin *sdk.Plugin, prefix string, present bool) error {
	var result struct {
		Paths []struct {
			Prefix string `json:"prefix"`
		} `json:"best-path"`
	}
	status, err := Dispatch(ctx, plugin, "show bgp rib best prefix "+prefix, &result)
	if err != nil {
		return err
	}
	if status != statusDone {
		return fmt.Errorf("best-path %s status=%s", prefix, status)
	}
	want := 0
	if present {
		want = 1
	}
	if len(result.Paths) != want {
		return fmt.Errorf("eligible %s paths=%v, want count=%d", prefix, result.Paths, want)
	}
	if present && result.Paths[0].Prefix != prefix {
		return fmt.Errorf("wrong selected prefix: %v", result.Paths)
	}
	return nil
}

func rfc8212PolicyExport(ctx context.Context, plugin *sdk.Plugin, state *rfc8212PolicyWire) error {
	if !waitPeersEOR03(ctx, plugin, 3) {
		return errors.New("export peers did not finish initial synchronization")
	}
	if err := state.wait(ctx, "127.0.0.3", "", "10.82.10.0/24"); err != nil {
		return err
	}
	if err := liveStagesRoute(ctx, plugin, "127.0.0.1 update text nhop 192.0.2.1 nlri add 192.168.1.0/24"); err != nil {
		return err
	}
	if err := state.wait(ctx, "127.0.0.3", "", "10.82.10.0/24", "10.82.1.0/24"); err != nil {
		return err
	}
	for _, peer := range []string{"127.0.0.2", "127.0.0.3"} {
		if err := liveStagesRoute(ctx, plugin, peer+" update text nhop 192.0.2.1 nlri add 10.82.11.0/24"); err != nil {
			return err
		}
	}
	if err := state.wait(ctx, "127.0.0.3", "", "10.82.10.0/24", "10.82.1.0/24", "10.82.11.0/24"); err != nil {
		return err
	}
	if err := os.WriteFile("rfc8212-bridge-ready", []byte("ready\n"), 0o600); err != nil {
		return err
	}
	if err := state.wait(ctx, "127.0.0.3", "", "10.82.10.0/24", "10.82.1.0/24", "10.82.11.0/24", "10.82.12.0/24"); err != nil {
		return err
	}
	if _, err := done03(ctx, plugin, "request fakeredist emit add ipv4/unicast 10.82.13.0/24"); err != nil {
		return err
	}
	if err := state.wait(ctx, "127.0.0.3", "", "10.82.10.0/24", "10.82.1.0/24", "10.82.11.0/24", "10.82.12.0/24", "10.82.13.0/24"); err != nil {
		return err
	}
	for _, peer := range []string{"127.0.0.2", "127.0.0.3"} {
		if err := liveStagesRoute(ctx, plugin, peer+" raw hex 000000144001010040020602010000FDE8400304C0000201180A520E type update"); err != nil {
			return err
		}
	}
	if err := state.wait(ctx, "127.0.0.3", "", "10.82.10.0/24", "10.82.1.0/24", "10.82.11.0/24", "10.82.12.0/24", "10.82.13.0/24", "10.82.14.0/24"); err != nil {
		return err
	}
	for _, peer := range []string{"127.0.0.2", "127.0.0.3"} {
		if err := liveStagesRoute(ctx, plugin, peer+" raw hex FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF002F02000000144001010040020602010000FDE8400304C0000201180A520F"); err != nil {
			return err
		}
	}
	if err := state.wait(ctx, "127.0.0.3", "", "10.82.10.0/24", "10.82.1.0/24", "10.82.11.0/24", "10.82.12.0/24", "10.82.13.0/24", "10.82.14.0/24", "10.82.15.0/24"); err != nil {
		return err
	}
	// A KEEPALIVE prefix cannot hide an UPDATE from export admission.
	if err := liveStagesRoute(ctx, plugin, "127.0.0.2 raw hex FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF002F02000000144001010040020602010000FDE8400304C0000201180A5210"); err != nil {
		return err
	}
	if err := quiesce03(ctx, plugin); err != nil {
		return err
	}
	// Keep the accepted socket checked until this receipt. Completing after
	// the last announcement would miss a later withdrawal before observation.
	if err := liveStagesRoute(ctx, plugin, "127.0.0.3 update text nlri del 192.168.251.0/24"); err != nil {
		return err
	}
	if err := state.wait(ctx, "127.0.0.3", "192.168.251.0/24", "10.82.10.0/24", "10.82.1.0/24", "10.82.11.0/24", "10.82.12.0/24", "10.82.13.0/24", "10.82.14.0/24", "10.82.15.0/24"); err != nil {
		return err
	}
	// This never-advertised session cannot carry a managed withdrawal fence.
	// The native peer acknowledges EOR after checking every preceding frame;
	// persistent reject rules also cover forbidden announcements in linger.
	if err := liveStagesRoute(ctx, plugin, "127.0.0.2 update text nlri eor"); err != nil {
		return err
	}
	if err := state.wait(ctx, "127.0.0.2", "192.168.250.0/24"); err != nil {
		return err
	}
	return policyFinalSessions(ctx, plugin, 3)
}

func rfc8212PolicyBridge(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("RFC8212 bridge helper takes no arguments")
	}
	if !waitForFile(ctx, "rfc8212-bridge-ready", 200, liveStagesPollDelay) {
		return errors.New("bridge route was never released")
	}
	return fixture06ExaBGPWrite(ctx, "announce route 10.82.12.0/24 next-hop 192.0.2.1 origin igp")
}
