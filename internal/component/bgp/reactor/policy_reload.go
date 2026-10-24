// Design: docs/architecture/core-design.md -- final-only export policy reconciliation.
// Related: reactor_api_advertised.go -- serialized withdrawal of advertised keys.
// Related: reactor_api_relay.go -- retained routes use the ordinary export writer.
package reactor

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// reconcileExportFinal runs after the new forwarding facts are visible, with no
// peer or reactor lock held. Before snapshotting a rejecting destination it
// joins the writer: every old accepting decision has then reached adjOut, and
// every later managed write reads the rejecting final.
func (p *Peer) reconcileExportFinal() error {
	if p.State() != PeerStateEstablished {
		return nil
	}
	r := p.reactor
	if r == nil {
		return nil
	}
	session := p.currentSession()
	if session == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	adapter := &reactorAPIAdapter{r: r}
	if !p.ExportPolicy().AcceptsAtEnd() {
		if r.fwdPool != nil {
			if err := r.fwdPool.barrierPeer(ctx, p.Settings().PeerKey()); err != nil {
				return fmt.Errorf("export final withdrawal: %w", err)
			}
		}
		// The pool barrier does not cover direct API writers. They hold writeMu
		// from their final decision through ownership admission, so joining it
		// after publication closes that window without taking p.mu under it.
		// Nothing runs under the lock: acquiring it is the whole barrier.
		session.writeMu.Lock()
		session.writeMu.Unlock() //nolint:gocritic,staticcheck // badLock, SA2001: the empty critical section is the barrier
		_, err := adapter.withdrawPeerAdvertised(ctx, p, family.Family{}, plugin.OperatorSender())
		return err
	}

	// Only a denied-to-accepted transition reaches restoration. An initially
	// denied static set may still have been recorded as delivered by its sender.
	p.staticMu.Lock()
	if p.staticWire.session == session {
		p.staticWire.routes = nil
	}
	p.staticMu.Unlock()
	p.deliverStaticRouteDelta(p.staticRoutes())
	if negotiated := p.negotiated.Load(); negotiated != nil {
		previous := p.sendingConfigStatic.Swap(true)
		p.sendDefaultOriginateRoutes(negotiated)
		p.sendPluginRoutesVia(session, p.sendUpdateDirect)
		p.sendingConfigStatic.Store(previous)
	}
	if r.api == nil {
		return nil
	}
	owner := p.policyReplayOwner()
	if !owner.IsSet() {
		return nil
	}
	negotiated := p.negotiated.Load()
	ordinary := negotiated == nil
	if negotiated != nil {
		for fam, supported := range negotiated.families {
			ordinary = ordinary || supported && !ribevents.IsFlowSpec(fam)
		}
	}
	hasReceiveStore := r.api.Dispatcher().Registry().Lookup(ribevents.ReplayRoutesCommand) != nil
	if ordinary && !hasReceiveStore {
		return errors.New("export final restoration of negotiated ordinary families needs the receive store: load bgp-adj-rib-in and attach it to route sources")
	}
	// This is a live session, unlike peer-up replay. Join every admitted writer
	// while raising the existing live-order hold. Writers recheck it under the
	// same lock, so even work queued before this transition rejoins overflow.
	p.beginPolicyReplay(session)
	defer p.endPolicyReplay()
	dispatch := func(ctx context.Context, command string, args ...string) (string, json.RawMessage, error) {
		request := &pluginserver.CommandContext{Server: r.api, RequestContext: ctx, Sender: owner}
		response, err := r.api.Dispatcher().ForwardToPlugin(request, command, args, "*")
		if err != nil {
			return "", nil, err
		}
		if response == nil {
			return "", nil, errors.New("export final replay returned no answer")
		}
		if response.Error != "" {
			return "", nil, errors.New(response.Error)
		}
		data, err := json.Marshal(response.Data)
		return response.Status, data, err
	}
	relay := func(_ context.Context, _ string, routes []rpc.StoredRoute) error {
		if p.currentSession() != session {
			return errRecoveryDestinationClosed
		}
		return adapter.RelayStoredRoute(p.Settings().Address, routes, owner)
	}
	var err error
	if ordinary {
		_, err = ribevents.RelayReplay(ctx, dispatch, relay, p.addrString, 0, "")
	}
	if err == nil {
		// FlowSpec is deliberately absent from receive-store replay. Its
		// selecting RIB is authoritative, including when Adj-RIB-In is absent.
		// Relay through the actual owner's grant and the ordinary live gate,
		// which rechecks the current path's authorization and generation.
		for _, key := range ribevents.FlowSpecRoutes() {
			if key.Peer == p.Settings().Address {
				continue
			}
			if negotiated == nil {
				continue
			}
			if !negotiated.families[key.Family] {
				continue
			}
			path, present := ribevents.LookupFlowSpecPath(key)
			if !present {
				continue
			}
			route := rpc.StoredRoute{SourcePeer: key.Peer.String(), Family: key.Family.String(),
				PathID: key.PathID, MsgID: path.MsgID, NLRIFraming: rpc.NLRIFramingPrefixOnly,
				NLRIHex: hex.EncodeToString([]byte(key.NLRI)), AttrHex: hex.EncodeToString(path.Attributes),
				InitialUpdate: true}
			err = ctx.Err()
			if err == nil {
				err = relay(ctx, p.addrString, []rpc.StoredRoute{route})
			}
			if err != nil {
				break
			}
		}
	}
	return errors.Join(err, p.flushPolicyReplay(ctx, session))
}

// beginPolicyReplay MUST precede accepting-facts publication and every retained
// page read. Nested holds cover settings publication and direct reconcile
// callers; each hold MUST be paired with endPolicyReplay.
func (p *Peer) beginPolicyReplay(session *Session) {
	session.writeMu.Lock()
	if p.policyReplayHolds.Add(1) == 1 {
		p.policyReplayEpoch.Add(1)
	}
	session.writeMu.Unlock()
}

// endPolicyReplay MUST pair with beginPolicyReplay. Retained-replay callers
// MUST resolve flushPolicyReplay before releasing their hold.
func (p *Peer) endPolicyReplay() {
	p.policyReplayHolds.Add(-1)
	p.wakeForwardOverflow()
}

// flushPolicyReplay MUST finish before reconciliation lowers policyReplayHolds.
// Its marker is replay work, so it passes held live changes and follows every
// replayed page. A normal barrier would wait behind those held live changes.
func (p *Peer) flushPolicyReplay(ctx context.Context, session *Session) error {
	pool := p.reactor.fwdPool
	if pool == nil {
		return nil
	}
	done := make(chan struct{})
	marker := fwdItem{peer: p, session: session, initialUpdate: true,
		done: func() { close(done) }}
	if !pool.dispatchOverflow(fwdKey{peerAddr: p.Settings().PeerKey()}, &marker) {
		return errRecoveryDestinationClosed
	}
	select {
	case <-ctx.Done():
		return fmt.Errorf("export final replay drain: %w", ctx.Err())
	case <-done:
	}
	if p.currentSession() != session {
		return errRecoveryDestinationClosed
	}
	session.writeMu.Lock()
	err := session.writeFailed
	session.writeMu.Unlock()
	return err
}

// policyReplayOwner reads the actual configured claimant, not an implementation
// name. External and aliased plugins use the same runtime declaration and grant.
func (p *Peer) policyReplayOwner() plugin.Sender {
	manager := p.reactor.api.ProcessManager()
	if manager == nil {
		return plugin.Sender{}
	}
	p.mu.RLock()
	bindings := p.settings.ProcessBindings
	p.mu.RUnlock()
	replayCommand := p.reactor.api.Dispatcher().Registry().Lookup(ribevents.ReplayRoutesCommand)
	var selfReplay plugin.Sender
	for i := range bindings {
		binding := &bindings[i]
		if !binding.MaySend(bgpevents.SendUpdate) {
			continue
		}
		if !binding.ReceivesPeerState() {
			continue
		}
		process := manager.GetProcess(binding.PluginName)
		if process == nil {
			continue
		}
		// The store self-replays when no claimant receives this peer's state.
		// Its command registration identifies the real process, including aliases.
		if replayCommand != nil && replayCommand.Process == process {
			selfReplay = plugin.ProcessSender(binding.PluginName)
		}
		registration := process.Registration()
		if registration != nil && len(registration.Claims) > 0 {
			if slices.Contains(registration.Claims, "bgp-peer-up-replay") {
				return plugin.ProcessSender(binding.PluginName)
			}
			continue
		}
		// Built-in plugins can declare the role only in their compiled
		// registration. Resolve aliases exactly as server.claimsForPlugin does.
		for _, name := range plugin.RegistryNames(process.Config()) {
			if slices.Contains(registry.ClaimsFor(name), "bgp-peer-up-replay") {
				return plugin.ProcessSender(binding.PluginName)
			}
		}
	}
	return selfReplay
}
