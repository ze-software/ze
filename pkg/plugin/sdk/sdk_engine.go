// Design: docs/architecture/api/process-protocol.md — plugin-to-engine RPC methods
// Overview: sdk.go — plugin SDK core
// Related: union.go — event stream correlation using EmitEvent

package sdk

import (
	"context"
	"encoding/json"
	"fmt"

	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

// UpdateRoute injects a route update to matching peers via the engine.
// Returns the number of peers affected and routes sent.
func (p *Plugin) UpdateRoute(ctx context.Context, peerSelector, command string) (peersAffected, routesSent uint32, err error) {
	input := &rpc.UpdateRouteInput{PeerSelector: peerSelector, Command: command}
	result, err := p.callEngineWithResult(ctx, "ze-plugin-engine:update-route", input)
	if err != nil {
		return 0, 0, err
	}
	var out rpc.UpdateRouteOutput
	if err := json.Unmarshal(result, &out); err != nil {
		return 0, 0, fmt.Errorf("unmarshal update-route result: %w", err)
	}
	return out.PeersAffected, out.RoutesSent, nil
}

// DispatchCommand dispatches a command through the engine's command dispatcher.
// Returns the status and data from the target handler's response. This enables
// inter-plugin communication: the engine routes the command to the target plugin
// via longest-match registry lookup and returns the full structured response.
func (p *Plugin) DispatchCommand(ctx context.Context, command string) (status, data string, err error) {
	input := &rpc.DispatchCommandInput{Command: command}
	result, err := p.callEngineWithResult(ctx, "ze-plugin-engine:dispatch-command", input)
	if err != nil {
		return "", "", err
	}
	var out rpc.DispatchCommandOutput
	if err := json.Unmarshal(result, &out); err != nil {
		return "", "", fmt.Errorf("unmarshal dispatch-command result: %w", err)
	}
	return out.Status, out.Data, nil
}

// EmitEvent pushes an event into the engine's delivery pipeline.
// The engine finds subscribers matching the namespace, event type, direction, and peer,
// then delivers the event string to each. Returns the number of subscribers reached.
func (p *Plugin) EmitEvent(ctx context.Context, namespace, eventType, direction, peerAddress, event string) (int, error) {
	input := &rpc.EmitEventInput{
		Namespace:   namespace,
		EventType:   eventType,
		Direction:   direction,
		PeerAddress: peerAddress,
		Event:       event,
	}
	result, err := p.callEngineWithResult(ctx, "ze-plugin-engine:emit-event", input)
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
	return p.callEngine(ctx, "ze-plugin-engine:subscribe-events", input)
}

// UnsubscribeEvents stops event delivery from the engine.
func (p *Plugin) UnsubscribeEvents(ctx context.Context) error {
	return p.callEngine(ctx, "ze-plugin-engine:unsubscribe-events", nil)
}

// DecodeNLRI requests NLRI decoding from the engine via the plugin registry.
// The engine routes the request to the in-process decoder for the given family.
// Returns the JSON representation of the decoded NLRI.
func (p *Plugin) DecodeNLRI(ctx context.Context, family, hex string) (string, error) {
	input := &rpc.DecodeNLRIInput{Family: family, Hex: hex}
	result, err := p.callEngineWithResult(ctx, "ze-plugin-engine:decode-nlri", input)
	if err != nil {
		return "", err
	}
	var out rpc.DecodeNLRIOutput
	if err := json.Unmarshal(result, &out); err != nil {
		return "", fmt.Errorf("unmarshal decode-nlri result: %w", err)
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
