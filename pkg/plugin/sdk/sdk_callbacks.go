// Design: docs/architecture/api/process-protocol.md — callback registration methods
// Overview: sdk.go — plugin SDK core

package sdk

import (
	"context"
	"encoding/json"
	"fmt"

	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

// initCallbackDefaults registers default handlers for callbacks that have
// graceful no-handler behavior (accept/no-op). Called from constructors.
func (p *Plugin) initCallbackDefaults() {
	p.callbacks = map[string]callbackHandler{
		// Events: no-op when no handler registered.
		callbackDeliverEvent: func(json.RawMessage) (json.RawMessage, error) { return nil, nil },
		callbackDeliverBatch: func(json.RawMessage) (json.RawMessage, error) { return nil, nil },
		// Config: accept when no handler registered.
		callbackConfigVerify: marshalStatusOK,
		callbackConfigApply:  marshalStatusOK,
		// Validate-open: accept when no handler registered.
		callbackValidateOpen: func(json.RawMessage) (json.RawMessage, error) {
			return json.Marshal(&rpc.ValidateOpenOutput{Accept: true})
		},
		// Bye: no-op when no handler registered.
		callbackBye: func(json.RawMessage) (json.RawMessage, error) { return nil, nil },
	}
}

// marshalStatusOK returns a JSON status OK response. Shared default for config callbacks.
func marshalStatusOK(json.RawMessage) (json.RawMessage, error) {
	return json.Marshal(struct {
		Status string `json:"status"`
	}{Status: rpc.StatusOK})
}

// OnConfigure sets the handler for Stage 2 config delivery.
func (p *Plugin) OnConfigure(fn func([]ConfigSection) error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onConfigure = fn
}

// OnShareRegistry sets the handler for Stage 4 registry delivery.
func (p *Plugin) OnShareRegistry(fn func([]RegistryCommand)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onShareRegistry = fn
}

// OnEvent sets the handler for runtime event delivery.
func (p *Plugin) OnEvent(fn func(string) error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onEvent = fn // Keep field for bridge direct delivery hot path.
	p.callbacks[callbackDeliverEvent] = func(params json.RawMessage) (json.RawMessage, error) {
		var input struct {
			Event string `json:"event"`
		}
		if err := json.Unmarshal(params, &input); err != nil {
			return nil, fmt.Errorf("unmarshal deliver-event: %w", err)
		}
		return nil, fn(input.Event)
	}
	p.callbacks[callbackDeliverBatch] = func(params json.RawMessage) (json.RawMessage, error) {
		events, err := rpc.ParseBatchEvents(params)
		if err != nil {
			return nil, err
		}
		for _, raw := range events {
			var eventStr string
			if err := json.Unmarshal(raw, &eventStr); err != nil {
				return nil, fmt.Errorf("unmarshal batch event: %w", err)
			}
			if err := fn(eventStr); err != nil {
				return nil, err
			}
		}
		return nil, nil
	}
}

// OnStructuredEvent sets the handler for structured event delivery via DirectBridge.
// When registered, the bridge delivers structured events directly (no text formatting).
// The handler receives []any where each element is a *rpc.StructuredEvent.
func (p *Plugin) OnStructuredEvent(fn func([]any) error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onStructuredEvent = fn
}

// OnBye sets the handler for shutdown notification.
func (p *Plugin) OnBye(fn func(string)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.callbacks[callbackBye] = func(params json.RawMessage) (json.RawMessage, error) {
		var input struct {
			Reason string `json:"reason,omitempty"`
		}
		if params != nil {
			_ = json.Unmarshal(params, &input) //nolint:errcheck // best-effort
		}
		fn(input.Reason)
		return nil, nil
	}
}

// OnEncodeNLRI sets the handler for NLRI encoding requests.
// The handler receives the address family and arguments, and returns hex-encoded NLRI.
func (p *Plugin) OnEncodeNLRI(fn func(family string, args []string) (string, error)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.callbacks[callbackEncodeNLRI] = func(params json.RawMessage) (json.RawMessage, error) {
		var input rpc.EncodeNLRIInput
		if err := json.Unmarshal(params, &input); err != nil {
			return nil, fmt.Errorf("unmarshal encode-nlri: %w", err)
		}
		hex, err := fn(input.Family, input.Args)
		if err != nil {
			return nil, err
		}
		return json.Marshal(struct {
			Hex string `json:"hex"`
		}{Hex: hex})
	}
}

// OnDecodeNLRI sets the handler for NLRI decoding requests.
// The handler receives the address family and hex-encoded NLRI, and returns JSON.
func (p *Plugin) OnDecodeNLRI(fn func(family string, hex string) (string, error)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.callbacks[callbackDecodeNLRI] = func(params json.RawMessage) (json.RawMessage, error) {
		var input rpc.DecodeNLRIInput
		if err := json.Unmarshal(params, &input); err != nil {
			return nil, fmt.Errorf("unmarshal decode-nlri: %w", err)
		}
		jsonResult, err := fn(input.Family, input.Hex)
		if err != nil {
			return nil, err
		}
		return json.Marshal(struct {
			JSON string `json:"json"`
		}{JSON: jsonResult})
	}
}

// OnDecodeCapability sets the handler for capability decoding requests.
// The handler receives the capability code and hex-encoded bytes, and returns JSON.
func (p *Plugin) OnDecodeCapability(fn func(code uint8, hex string) (string, error)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.callbacks[callbackDecodeCapability] = func(params json.RawMessage) (json.RawMessage, error) {
		var input rpc.DecodeCapabilityInput
		if err := json.Unmarshal(params, &input); err != nil {
			return nil, fmt.Errorf("unmarshal decode-capability: %w", err)
		}
		jsonResult, err := fn(input.Code, input.Hex)
		if err != nil {
			return nil, err
		}
		return json.Marshal(struct {
			JSON string `json:"json"`
		}{JSON: jsonResult})
	}
}

// OnExecuteCommand sets the handler for command execution requests.
// The handler receives serial, command, args, peer and returns (status, data, error).
func (p *Plugin) OnExecuteCommand(fn func(serial, command string, args []string, peer string) (string, string, error)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.callbacks[callbackExecuteCommand] = func(params json.RawMessage) (json.RawMessage, error) {
		var input rpc.ExecuteCommandInput
		if err := json.Unmarshal(params, &input); err != nil {
			return nil, fmt.Errorf("unmarshal execute-command: %w", err)
		}
		status, data, err := fn(input.Serial, input.Command, input.Args, input.Peer)
		if err != nil {
			return nil, err
		}
		return json.Marshal(&rpc.ExecuteCommandOutput{Status: status, Data: data})
	}
}

// OnConfigVerify sets the handler for config verification requests (reload pipeline).
// The handler receives the full candidate config sections and returns nil to accept
// or an error to reject. If no handler is registered, config-verify returns OK (no-op).
func (p *Plugin) OnConfigVerify(fn func([]ConfigSection) error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.callbacks[callbackConfigVerify] = func(params json.RawMessage) (json.RawMessage, error) {
		var input rpc.ConfigVerifyInput
		if err := json.Unmarshal(params, &input); err != nil {
			return marshalStatusError(fmt.Sprintf("unmarshal config-verify: %v", err))
		}
		if err := fn(input.Sections); err != nil {
			return marshalStatusError(err.Error())
		}
		return marshalStatusOK(nil)
	}
}

// OnConfigApply sets the handler for config apply requests (reload pipeline).
// The handler receives diff sections describing what changed and returns nil to accept
// or an error to reject. If no handler is registered, config-apply returns OK (no-op).
func (p *Plugin) OnConfigApply(fn func([]ConfigDiffSection) error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.callbacks[callbackConfigApply] = func(params json.RawMessage) (json.RawMessage, error) {
		var input rpc.ConfigApplyInput
		if err := json.Unmarshal(params, &input); err != nil {
			return marshalStatusError(fmt.Sprintf("unmarshal config-apply: %v", err))
		}
		if err := fn(input.Sections); err != nil {
			return marshalStatusError(err.Error())
		}
		return marshalStatusOK(nil)
	}
}

// OnValidateOpen sets the handler for OPEN validation requests.
// The handler receives both local and remote OPEN messages and returns accept/reject.
// When registered, WantsValidateOpen is automatically set in Stage 1 registration.
// If no handler is registered, validate-open returns accept (no-op).
func (p *Plugin) OnValidateOpen(fn func(*ValidateOpenInput) *ValidateOpenOutput) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.callbacks[callbackValidateOpen] = func(params json.RawMessage) (json.RawMessage, error) {
		var input rpc.ValidateOpenInput
		if err := json.Unmarshal(params, &input); err != nil {
			return json.Marshal(&rpc.ValidateOpenOutput{
				Accept: false, Reason: fmt.Sprintf("unmarshal validate-open: %v", err),
			})
		}
		return json.Marshal(fn(&input))
	}
}

// OnFilterUpdate sets the handler for route filter requests (redistribution).
// The handler receives filter input (filter name, direction, peer, update text)
// and returns a PolicyResponse (accept/reject/modify with optional delta).
func (p *Plugin) OnFilterUpdate(fn func(*FilterUpdateInput) (*FilterUpdateOutput, error)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.callbacks[callbackFilterUpdate] = func(params json.RawMessage) (json.RawMessage, error) {
		var input rpc.FilterUpdateInput
		if err := json.Unmarshal(params, &input); err != nil {
			return nil, fmt.Errorf("unmarshal filter-update: %w", err)
		}
		out, err := fn(&input)
		if err != nil {
			return nil, err
		}
		return json.Marshal(out)
	}
}

// OnStarted sets a callback that runs after the 5-stage startup completes
// but before the event loop begins. This is the safe place to make engine
// calls (e.g., SubscribeEvents) because the connection is no longer blocked
// by the startup coordinator. Do NOT make engine calls inside OnShareRegistry
// or OnConfigure -- those run while the engine is waiting for the response,
// causing a deadlock.
func (p *Plugin) OnStarted(fn func(ctx context.Context) error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onStarted = fn
}

// SetStartupSubscriptions sets event subscriptions to include in the "ready" RPC.
// The engine registers these atomically before SignalAPIReady, ensuring the plugin
// receives events from the very first route send. Must be called before Run().
//
// This replaces the pattern of calling SubscribeEvents in OnStarted, which had a
// race condition: SignalAPIReady triggered route sends before the subscription RPC
// could be processed.
func (p *Plugin) SetStartupSubscriptions(events, peers []string, format string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.startupSubscription = &rpc.SubscribeEventsInput{
		Events: events,
		Peers:  peers,
		Format: format,
	}
}

// SetEncoding sets the event encoding preference ("json" or "text").
// Must be called after SetStartupSubscriptions and before Run().
// Text encoding uses space-delimited output parseable by strings.Fields
// instead of nested JSON requiring json.Unmarshal.
func (p *Plugin) SetEncoding(enc string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.startupSubscription == nil {
		p.startupSubscription = &rpc.SubscribeEventsInput{}
	}
	p.startupSubscription.Encoding = enc
}

// SetCapabilities sets the capabilities to declare during Stage 3.
// Must be called before Run().
func (p *Plugin) SetCapabilities(caps []CapabilityDecl) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.capabilities = caps
}

// marshalStatusError returns a JSON status error response with the given message.
func marshalStatusError(msg string) (json.RawMessage, error) {
	return json.Marshal(struct {
		Status string `json:"status"`
		Error  string `json:"error,omitempty"`
	}{Status: rpc.StatusError, Error: msg})
}
