// Design: docs/architecture/api/process-protocol.md — callback registration methods
// Overview: sdk.go — plugin SDK core

package sdk

import (
	"context"

	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

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
	p.onEvent = fn
}

// OnStructuredEvent sets the handler for structured event delivery via DirectBridge.
// When registered, the bridge delivers structured events directly (no text formatting).
// The handler receives []any where each element is a *rpc.StructuredUpdate.
func (p *Plugin) OnStructuredEvent(fn func([]any) error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onStructuredEvent = fn
}

// OnBye sets the handler for shutdown notification.
func (p *Plugin) OnBye(fn func(string)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onBye = fn
}

// OnEncodeNLRI sets the handler for NLRI encoding requests.
// The handler receives the address family and arguments, and returns hex-encoded NLRI.
func (p *Plugin) OnEncodeNLRI(fn func(family string, args []string) (string, error)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onEncodeNLRI = fn
}

// OnDecodeNLRI sets the handler for NLRI decoding requests.
// The handler receives the address family and hex-encoded NLRI, and returns JSON.
func (p *Plugin) OnDecodeNLRI(fn func(family string, hex string) (string, error)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onDecodeNLRI = fn
}

// OnDecodeCapability sets the handler for capability decoding requests.
// The handler receives the capability code and hex-encoded bytes, and returns JSON.
func (p *Plugin) OnDecodeCapability(fn func(code uint8, hex string) (string, error)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onDecodeCapability = fn
}

// OnExecuteCommand sets the handler for command execution requests.
// The handler receives serial, command, args, peer and returns (status, data, error).
func (p *Plugin) OnExecuteCommand(fn func(serial, command string, args []string, peer string) (string, string, error)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onExecuteCommand = fn
}

// OnConfigVerify sets the handler for config verification requests (reload pipeline).
// The handler receives the full candidate config sections and returns nil to accept
// or an error to reject. If no handler is registered, config-verify returns OK (no-op).
func (p *Plugin) OnConfigVerify(fn func([]ConfigSection) error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onConfigVerify = fn
}

// OnConfigApply sets the handler for config apply requests (reload pipeline).
// The handler receives diff sections describing what changed and returns nil to accept
// or an error to reject. If no handler is registered, config-apply returns OK (no-op).
func (p *Plugin) OnConfigApply(fn func([]ConfigDiffSection) error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onConfigApply = fn
}

// OnValidateOpen sets the handler for OPEN validation requests.
// The handler receives both local and remote OPEN messages and returns accept/reject.
// When registered, WantsValidateOpen is automatically set in Stage 1 registration.
// If no handler is registered, validate-open returns accept (no-op).
func (p *Plugin) OnValidateOpen(fn func(*ValidateOpenInput) *ValidateOpenOutput) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onValidateOpen = fn
}

// OnStarted sets a callback that runs after the 5-stage startup completes
// but before the event loop begins. This is the safe place to make engine
// calls (e.g., SubscribeEvents) because Socket A is no longer blocked by
// the startup coordinator. Do NOT make engine calls inside OnShareRegistry
// or OnConfigure — those run while the engine is waiting on Socket B,
// causing a cross-socket deadlock.
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
