// Design: docs/architecture/api/process-protocol.md — plugin-to-engine RPC methods
// Overview: sdk.go — plugin SDK core
// Related: union.go — event stream correlation using EmitEvent

package sdk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ze-software/ze/internal/core/selector"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// UpdateRoute injects a route update to matching peers via the engine.
// Returns the number of NLRIs announced and withdrawn.
func (p *Plugin) UpdateRoute(ctx context.Context, peerSelector, command string) (announced, withdrawn uint32, err error) {
	return p.UpdateRouteWithMeta(ctx, peerSelector, command, nil)
}

// UpdateRouteSel is the typed-selector variant of UpdateRoute.
// In-process plugins use this to pass a *selector.Selector through DirectBridge
// without stringifying. Falls back to the string path for external plugins.
func (p *Plugin) UpdateRouteSel(ctx context.Context, sel *selector.Selector, command string) (announced, withdrawn uint32, err error) {
	return p.UpdateRouteSelWithMeta(ctx, sel, command, nil)
}

// UpdateRouteSelWithMeta is the typed-selector variant of UpdateRouteWithMeta.
func (p *Plugin) UpdateRouteSelWithMeta(ctx context.Context, sel *selector.Selector, command string, meta map[string]any) (announced, withdrawn uint32, err error) {
	if p.bridge != nil && p.bridge.HasUpdateRouteSel() {
		return p.bridge.UpdateRouteSel(sel, command, meta)
	}
	return p.UpdateRouteWithMeta(ctx, sel.String(), command, meta)
}

// UpdateRouteWithMeta injects a route update with metadata to matching peers.
// Meta is plumbed to CommandContext.Meta for the command dispatch path.
// For forwarded cached routes (peer-to-peer), ingress filters set ReceivedUpdate.Meta
// which egress filters read. Plugin-originated routes currently go through AnnounceNLRIBatch
// (direct send) where CommandContext.Meta is not yet consumed by egress filters.
// Pass nil meta for routes without metadata (equivalent to UpdateRoute).
func (p *Plugin) UpdateRouteWithMeta(ctx context.Context, peerSelector, command string, meta map[string]any) (announced, withdrawn uint32, err error) {
	input := &rpc.UpdateRouteInput{PeerSelector: peerSelector, Command: command, Meta: meta}
	result, err := p.callEngineWithResult(ctx, rpc.MethodUpdateRoute, input)
	if err != nil {
		return 0, 0, err
	}
	var out rpc.UpdateRouteOutput
	if err := json.Unmarshal(result, &out); err != nil {
		return 0, 0, fmt.Errorf("unmarshal update-route result: %w", err)
	}
	return out.Announced, out.Withdrawn, nil
}

// ForwardCached forwards cached UPDATEs identified by updateIDs to the listed
// destination peers. Bypasses the update-route tokenise path (rs-fastpath-3).
//
// destinations are peer IP address strings; the engine parses them once at the
// reactor boundary and maps them to resolved peers. updateIDs must have been
// delivered to this plugin via its event subscription while CacheConsumer is
// true -- the engine acks each id for this plugin after forwarding (FIFO on
// FIFO consumers, per-entry on CacheConsumerUnordered consumers).
//
// Returns an error when the reactor cannot look up any of the updateIDs or
// the request cannot be dispatched. Individual per-destination failures are
// logged and do not fail the call.
func (p *Plugin) ForwardCached(ctx context.Context, updateIDs []uint64, destinations []string) error {
	// Fast path: typed DirectBridge dispatch (no JSON serialization).
	if p.bridge != nil && p.bridge.HasForwardCached() {
		return p.bridge.ForwardCached(ctx, updateIDs, destinations)
	}
	// Slow path: JSON-based RPC (external plugins or pre-startup).
	input := &rpc.ForwardCachedInput{IDs: updateIDs, Destinations: destinations}
	_, err := p.callEngineWithResult(ctx, rpc.MethodForwardCached, input)
	return err
}

// ReleaseCached acks the listed cached updateIDs for this plugin without
// forwarding to peers. Symmetric with ForwardCached for the "decided not to
// forward" path. rs-fastpath-3.
func (p *Plugin) ReleaseCached(ctx context.Context, updateIDs []uint64) error {
	if p.bridge != nil && p.bridge.HasReleaseCached() {
		return p.bridge.ReleaseCached(ctx, updateIDs)
	}
	input := &rpc.ReleaseCachedInput{IDs: updateIDs}
	_, err := p.callEngineWithResult(ctx, rpc.MethodReleaseCached, input)
	return err
}

// RelayStoredRoute asks the engine to relay routes the plugin holds as raw wire
// bytes to one newly-established peer, through the SAME egress pipeline a live
// forward uses. spec-fixit-bgp-egress-rail-divergence.
//
// This exists because the older replay path -- emitting "update hex ... add"
// text commands -- reached the peer by a different rail than a live forward:
// it prepended the local AS BEFORE the write gate and ran only the session's
// export filters, skipping the in-process role/OTC steps. Same route, two
// transforms, so a peer establishing while an UPDATE was in flight could see
// a rewritten AS_PATH, a duplicate announce, or an unsuppressed OTC route.
// Relaying through this call gives a replayed route one egress transform.
//
// Each route carries the peer it was learned from, so the engine can apply the
// source-dependent egress decisions (prepend, role/OTC, export policy) it would
// have applied had the route been forwarded live.
//
// Returns an error when the destination cannot be resolved to an established
// peer or the request cannot be dispatched. Per-route failures are logged and
// do not fail the call.
func (p *Plugin) RelayStoredRoute(ctx context.Context, destination string, routes []rpc.StoredRoute) error {
	// Fast path: typed DirectBridge dispatch (no JSON serialization).
	if p.bridge != nil && p.bridge.HasRelayStoredRoute() {
		return p.bridge.RelayStoredRoute(ctx, destination, routes)
	}
	// Slow path: JSON-based RPC (external plugins or pre-startup).
	input := &rpc.RelayStoredRouteInput{Destination: destination, Routes: routes}
	_, err := p.callEngineWithResult(ctx, rpc.MethodRelayStoredRoute, input)
	return err
}

// RouteInstall inserts a batch of computed routes into the engine's process-wide
// Loc-RIB. It is the cross-process bridge for a FORKED route-installing plugin
// (OSPF, IS-IS): in-process the plugin writes locrib.Default() directly, but a
// forked subprocess gets a nil Loc-RIB (default.go returns nil under
// ze.plugin.hub.token), so it ships the ops here and the engine applies them to
// the real singleton, where sysrib's OnChange programs the kernel. Returns the
// number of routes applied. Each entry carries the protocol NAME; the engine
// re-resolves it to its own numeric ProtocolID.
func (p *Plugin) RouteInstall(ctx context.Context, routes []rpc.RouteInstallEntry) (installed uint32, err error) {
	input := &rpc.RouteInstallInput{Routes: routes}
	result, err := p.callEngineWithResult(ctx, rpc.MethodRouteInstall, input)
	if err != nil {
		return 0, err
	}
	var out rpc.RouteInstallOutput
	if err := json.Unmarshal(result, &out); err != nil {
		return 0, fmt.Errorf("unmarshal route-install result: %w", err)
	}
	return out.Installed, nil
}

// RouteRemove withdraws a batch of routes from the engine's Loc-RIB by their
// (protocol, family, prefix, instance) identity. Symmetric with RouteInstall for
// the forked route-installing path. Returns the number of routes withdrawn.
func (p *Plugin) RouteRemove(ctx context.Context, routes []rpc.RouteRemoveEntry) (removed uint32, err error) {
	input := &rpc.RouteRemoveInput{Routes: routes}
	result, err := p.callEngineWithResult(ctx, rpc.MethodRouteRemove, input)
	if err != nil {
		return 0, err
	}
	var out rpc.RouteRemoveOutput
	if err := json.Unmarshal(result, &out); err != nil {
		return 0, fmt.Errorf("unmarshal route-remove result: %w", err)
	}
	return out.Removed, nil
}

// InjectWireRoute sends raw BGP UPDATE body bytes to the RIB under a named
// protocol. Zero-copy via DirectBridge typed handler (no hex encoding) for
// in-process plugins; a forked/external plugin with no typed slot falls back to
// the JSON codec over the socket (rpc.MethodInjectWireRoute). The method has no
// ctx parameter, so the fallback uses a background context.
func (p *Plugin) InjectWireRoute(protocol, peerKey string, updateBody []byte) error {
	if p.bridge != nil && p.bridge.HasInjectWireRoute() {
		return p.bridge.InjectWireRoute(protocol, peerKey, updateBody)
	}
	input := &rpc.InjectWireRouteInput{Protocol: protocol, PeerKey: peerKey, UpdateBody: updateBody}
	_, err := p.callEngineWithResult(context.Background(), rpc.MethodInjectWireRoute, input)
	return err
}

// BatchValidate sends a batch of RPKI validation decisions to adj-rib-in.
// Fast path: typed DirectBridge dispatch (no string serialization). Slow path:
// the JSON codec over the socket (rpc.MethodBatchValidate) for forked/external
// plugins without a typed slot.
func (p *Plugin) BatchValidate(ctx context.Context, decisions []rpc.ValidationDecision) (*rpc.BatchValidateResult, error) {
	if p.bridge != nil && p.bridge.HasBatchValidate() {
		return p.bridge.BatchValidate(decisions)
	}
	input := &rpc.BatchValidateInput{Decisions: decisions}
	result, err := p.callEngineWithResult(ctx, rpc.MethodBatchValidate, input)
	if err != nil {
		return nil, err
	}
	var out rpc.BatchValidateResult
	if len(result) > 0 {
		if err := json.Unmarshal(result, &out); err != nil {
			return nil, fmt.Errorf("batch-validate: unmarshal result: %w", err)
		}
	}
	return &out, nil
}

// DispatchCommand dispatches a command through the engine's command dispatcher.
// Returns the status and raw JSON data from the target handler's response. This
// enables inter-plugin communication: the engine routes the command to the target
// plugin via longest-match registry lookup and returns the full structured response.
// Error text from the handler is returned as a Go error (not in data).
func (p *Plugin) DispatchCommand(ctx context.Context, command string) (status string, data json.RawMessage, err error) {
	var out *rpc.DispatchCommandOutput

	if p.bridge != nil && p.bridge.HasDispatchCommand() {
		out, err = p.bridge.DispatchCommand(command)
	} else {
		input := &rpc.DispatchCommandInput{Command: command}
		out, err = p.dispatchCommandRPC(ctx, rpc.MethodDispatchCommand, input, "dispatch-command")
	}

	return dispatchCommandResult(out, err)
}

// DispatchCommandArgs dispatches an exact registered command with pre-tokenized
// arguments through the engine's command dispatcher. It preserves the external
// dispatch-command API while avoiding command-string tokenization for internal data.
func (p *Plugin) DispatchCommandArgs(ctx context.Context, command string, args []string, peer string) (status string, data json.RawMessage, err error) {
	var out *rpc.DispatchCommandOutput

	if p.bridge != nil && p.bridge.HasDispatchCommandArgs() {
		out, err = p.bridge.DispatchCommandArgs(command, args, peer)
	} else {
		input := &rpc.DispatchCommandArgsInput{Command: command, Args: args, Peer: peer}
		out, err = p.dispatchCommandRPC(ctx, rpc.MethodDispatchCommandArgs, input, "dispatch-command-args")
	}

	return dispatchCommandResult(out, err)
}

func (p *Plugin) dispatchCommandRPC(ctx context.Context, method string, input any, label string) (*rpc.DispatchCommandOutput, error) {
	result, err := p.callEngineWithResult(ctx, method, input)
	if err != nil {
		return nil, err
	}
	out := new(rpc.DispatchCommandOutput)
	if err := json.Unmarshal(result, out); err != nil {
		return nil, fmt.Errorf("unmarshal %s result: %w", label, err)
	}
	return out, nil
}

func dispatchCommandResult(out *rpc.DispatchCommandOutput, err error) (string, json.RawMessage, error) {
	if err != nil {
		return "", nil, err
	}
	if out.Error != "" {
		return out.Status, nil, errors.New(out.Error)
	}
	return out.Status, out.Data, nil
}

// EmitEvent pushes an event into the engine's delivery pipeline.
// The engine finds subscribers matching the namespace, event type, direction, and peer,
// then delivers the event string to each. Returns the number of subscribers reached.
func (p *Plugin) EmitEvent(ctx context.Context, namespace, eventType, direction, peerAddress, event string) (int, error) {
	// Fast path: typed DirectBridge emit (no JSON serialization).
	if p.bridge != nil && p.bridge.HasEmitEvent() {
		return p.bridge.EmitEvent(namespace, eventType, direction, peerAddress, event)
	}

	// Slow path: JSON-based RPC (external plugins or pre-startup).
	input := &rpc.EmitEventInput{
		Namespace:   namespace,
		EventType:   eventType,
		Direction:   direction,
		PeerAddress: peerAddress,
		Event:       event,
	}
	result, err := p.callEngineWithResult(ctx, rpc.MethodEmitEvent, input)
	if err != nil {
		return 0, err
	}
	var out rpc.EmitEventOutput
	if err := json.Unmarshal(result, &out); err != nil {
		return 0, fmt.Errorf("unmarshal emit-event result: %w", err)
	}
	return out.Delivered, nil
}

// SubscribeEvents requests event delivery from the engine.
func (p *Plugin) SubscribeEvents(ctx context.Context, events, peers []string, format string) error {
	input := &rpc.SubscribeEventsInput{Events: events, Peers: peers, Format: format}
	return p.callEngine(ctx, rpc.MethodSubscribeEvents, input)
}

// UnsubscribeEvents stops event delivery from the engine.
func (p *Plugin) UnsubscribeEvents(ctx context.Context) error {
	return p.callEngine(ctx, rpc.MethodUnsubscribeEvents, nil)
}

// DecodeNLRI requests NLRI decoding from the engine via the plugin registry.
// The engine routes the request to the in-process decoder for the given family.
// Returns the JSON representation of the decoded NLRI.
func (p *Plugin) DecodeNLRI(ctx context.Context, family, hex string) (json.RawMessage, error) {
	input := &rpc.DecodeNLRIInput{Family: family, Hex: hex}
	result, err := p.callEngineWithResult(ctx, "ze-plugin-engine:decode-nlri", input)
	if err != nil {
		return nil, err
	}
	var out rpc.DecodeNLRIOutput
	if err := json.Unmarshal(result, &out); err != nil {
		return nil, fmt.Errorf("unmarshal decode-nlri result: %w", err)
	}
	return out.JSON, nil
}

// EncodeNLRI requests NLRI encoding from the engine via the plugin registry.
// The engine routes the request to the in-process encoder for the given family.
// Returns hex-encoded NLRI bytes.
func (p *Plugin) EncodeNLRI(ctx context.Context, family string, args []string) (string, error) {
	input := &rpc.EncodeNLRIInput{Family: family, Args: args}
	result, err := p.callEngineWithResult(ctx, "ze-plugin-engine:encode-nlri", input)
	if err != nil {
		return "", err
	}
	var out rpc.EncodeNLRIOutput
	if err := json.Unmarshal(result, &out); err != nil {
		return "", fmt.Errorf("unmarshal encode-nlri result: %w", err)
	}
	return out.Hex, nil
}

// DecodeMPReach requests MP_REACH_NLRI decoding from the engine.
// The engine parses the attribute value (AFI+SAFI+NH+NLRI) and returns the family,
// next-hop, and decoded NLRI. RFC 4760 Section 3.
func (p *Plugin) DecodeMPReach(ctx context.Context, hex string, addPath bool) (*rpc.DecodeMPReachOutput, error) {
	input := &rpc.DecodeMPReachInput{Hex: hex, AddPath: addPath}
	result, err := p.callEngineWithResult(ctx, "ze-plugin-engine:decode-mp-reach", input)
	if err != nil {
		return nil, err
	}
	var out rpc.DecodeMPReachOutput
	if err := json.Unmarshal(result, &out); err != nil {
		return nil, fmt.Errorf("unmarshal decode-mp-reach result: %w", err)
	}
	return &out, nil
}

// DecodeMPUnreach requests MP_UNREACH_NLRI decoding from the engine.
// The engine parses the attribute value (AFI+SAFI+Withdrawn) and returns the family
// and decoded withdrawn NLRI. RFC 4760 Section 4.
func (p *Plugin) DecodeMPUnreach(ctx context.Context, hex string, addPath bool) (*rpc.DecodeMPUnreachOutput, error) {
	input := &rpc.DecodeMPUnreachInput{Hex: hex, AddPath: addPath}
	result, err := p.callEngineWithResult(ctx, "ze-plugin-engine:decode-mp-unreach", input)
	if err != nil {
		return nil, err
	}
	var out rpc.DecodeMPUnreachOutput
	if err := json.Unmarshal(result, &out); err != nil {
		return nil, fmt.Errorf("unmarshal decode-mp-unreach result: %w", err)
	}
	return &out, nil
}

// DecodeUpdate requests full UPDATE message decoding from the engine.
// The engine parses the UPDATE body (after 19-byte BGP header) and returns
// the ze-bgp JSON representation. RFC 4271 Section 4.3.
func (p *Plugin) DecodeUpdate(ctx context.Context, hex string, addPath bool) (string, error) {
	input := &rpc.DecodeUpdateInput{Hex: hex, AddPath: addPath}
	result, err := p.callEngineWithResult(ctx, "ze-plugin-engine:decode-update", input)
	if err != nil {
		return "", err
	}
	var out rpc.DecodeUpdateOutput
	if err := json.Unmarshal(result, &out); err != nil {
		return "", fmt.Errorf("unmarshal decode-update result: %w", err)
	}
	return out.JSON, nil
}
