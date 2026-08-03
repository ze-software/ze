// Design: docs/architecture/api/process-protocol.md — 5-stage plugin startup protocol
// Overview: startup.go — engineStartupSink (full engine registration + barrier)
// Overview: subsystem.go — hubStartupSink (harvest commands/schema, nil payloads)
//
// This file holds the single shared implementation of the 5-stage plugin
// startup wire choreography. Both the engine (Server.handleProcessStartupRPC)
// and the hub (SubsystemHandler.completeProtocol) drive startup through
// runStartupHandshake; the caller-specific effects between stages live behind
// the startupSink interface, so a protocol change (a new method string, a
// reordered stage, an added validation) touches exactly one place.

package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	plugin "github.com/ze-software/ze/internal/component/plugin"
	pluginipc "github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// The three plugin-initiated stage methods of the 5-stage startup handshake.
// These are the wire contract between the engine/hub and every plugin. They
// live here, in the single shared stage-driver, so the choreography is declared
// once rather than duplicated per orchestrator. The two engine-initiated
// callback methods (ze-plugin-callback:configure and :share-registry) live in
// the ipc layer, inside PluginConn.SendConfigure / SendShareRegistry.
const (
	methodDeclareRegistration = "ze-plugin-engine:declare-registration"
	methodDeclareCapabilities = "ze-plugin-engine:declare-capabilities"
	methodReady               = "ze-plugin-engine:ready"
)

var (
	// errStartupConnClosed is returned when the sink has no connection to drive
	// the handshake over.
	errStartupConnClosed = errors.New("startup connection closed before protocol")

	// errStartupBarrierAborted is returned when a barrier transition fails
	// (engine tier abort). The barrier already recorded the tier-wide failure,
	// so no error is sent to the plugin; the engine caller ignores this value
	// and relies on the process stage set through the transition.
	errStartupBarrierAborted = errors.New("startup barrier aborted")
)

// startupSink injects the caller-specific effects of the 5-stage plugin startup
// handshake. runStartupHandshake owns the wire choreography — reading each
// plugin-initiated request, validating its method string, responding, and
// (via the deliver hooks) sending the two engine-initiated callbacks — while
// the sink owns what happens to the decoded input at each stage and how the
// optional barrier advances.
//
// Two sinks implement it. engineStartupSink (startup.go) performs the full set
// of engine-side registrations between stages (registry, families, capabilities,
// commands, subscriptions, bridge dispatch, reactor signaling) and synchronizes
// concurrent plugins in a tier through the Server's StartupCoordinator barrier.
// hubStartupSink (subsystem.go) is minimal: it harvests the plugin's declared
// commands and schema, delivers nil config and nil registry, and runs a single
// connection with no barrier. New callers implement this interface rather than
// the driver growing a per-caller branch (ai/rules/plugins.md).
type startupSink interface {
	// conn returns the connection the handshake runs over. Called once, first.
	conn() *pluginipc.PluginConn

	// onRegistration processes the decoded declare-registration input after the
	// driver validated the method string and before it responds OK. A non-nil
	// error aborts startup: the driver sends err.Error() to the plugin via
	// SendError, then returns the error. Sinks that mutate shared state roll
	// their own changes back before returning the error.
	onRegistration(input *rpc.DeclareRegistrationInput) error

	// deliverConfig sends the Stage-2 configure callback. Each sink builds its
	// own payload (engine: real config sections; hub: nil) and owns its own
	// delivery-error policy. A non-nil error aborts the handshake.
	deliverConfig(ctx context.Context) error

	// onCapabilities processes the decoded declare-capabilities input after the
	// driver validated the method string and before it responds OK. Same error
	// contract as onRegistration.
	onCapabilities(input *rpc.DeclareCapabilitiesInput) error

	// deliverRegistry sends the Stage-4 share-registry callback. Same payload
	// and error contract as deliverConfig.
	deliverRegistry(ctx context.Context) error

	// onReady processes the decoded ready input before the Ready->Running
	// transition (engine registers subscriptions, wires bridge dispatch, and
	// registers dispatcher commands). Same error contract as onRegistration.
	onReady(input *rpc.ReadyInput) error

	// onRunning runs after the Ready->Running transition succeeds and before
	// the final OK response (engine signals the reactor API-ready).
	onRunning()

	// postReady runs after the final OK response is sent (engine switches to
	// bridge transport when the plugin requested it).
	postReady(input *rpc.ReadyInput)

	// transition advances the barrier from one stage to the next and records
	// the new process stage. It returns false to abort the handshake without an
	// error sent to the plugin (the barrier already recorded the tier-wide
	// failure). Sinks with no barrier return true unconditionally.
	transition(from, to plugin.PluginStage) bool
}

// runStartupHandshake drives the 5-stage plugin startup handshake over the
// sink's connection. It is the single implementation of the wire choreography
// shared by the engine and hub paths; every caller-specific effect lives behind
// the startupSink.
//
// Returns nil when the plugin reaches StageRunning. On a wire error, a method
// mismatch, a sink stage error, or a barrier abort it returns a non-nil error
// (the hub propagates it up its call stack; the engine ignores the value and
// relies on the process stage the sink recorded through transition).
func runStartupHandshake(ctx context.Context, sink startupSink) error {
	conn := sink.conn()
	if conn == nil {
		return errStartupConnClosed
	}

	// Stage 1: declare-registration (plugin-initiated).
	req, err := conn.ReadRequest(ctx)
	if err != nil {
		return fmt.Errorf("stage 1 read: %w", err)
	}
	if req.Method != methodDeclareRegistration {
		var tb textbuf.Buffer
		_ = conn.SendError(ctx, req.ID, tb.Str("expected declare-registration, got ").Str(req.Method).String())
		return fmt.Errorf("stage 1: expected declare-registration, got %s", req.Method)
	}
	var regInput rpc.DeclareRegistrationInput
	if err := json.Unmarshal(req.Params, &regInput); err != nil {
		var tb textbuf.Buffer
		_ = conn.SendError(ctx, req.ID, tb.Str("invalid registration: ").Err(err).String())
		return fmt.Errorf("stage 1 parse: %w", err)
	}
	if err := sink.onRegistration(&regInput); err != nil {
		_ = conn.SendError(ctx, req.ID, err.Error())
		return err
	}
	if err := conn.SendResult(ctx, req.ID, nil); err != nil {
		return fmt.Errorf("stage 1 respond: %w", err)
	}

	// Barrier + Stage 2: configure (engine-initiated).
	if !sink.transition(plugin.StageRegistration, plugin.StageConfig) {
		return errStartupBarrierAborted
	}
	if err := sink.deliverConfig(ctx); err != nil {
		return err
	}
	if !sink.transition(plugin.StageConfig, plugin.StageCapability) {
		return errStartupBarrierAborted
	}

	// Stage 3: declare-capabilities (plugin-initiated).
	req, err = conn.ReadRequest(ctx)
	if err != nil {
		return fmt.Errorf("stage 3 read: %w", err)
	}
	if req.Method != methodDeclareCapabilities {
		var tb textbuf.Buffer
		_ = conn.SendError(ctx, req.ID, tb.Str("expected declare-capabilities, got ").Str(req.Method).String())
		return fmt.Errorf("stage 3: expected declare-capabilities, got %s", req.Method)
	}
	var capsInput rpc.DeclareCapabilitiesInput
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &capsInput); err != nil {
			var tb textbuf.Buffer
			_ = conn.SendError(ctx, req.ID, tb.Str("invalid capabilities: ").Err(err).String())
			return fmt.Errorf("stage 3 parse: %w", err)
		}
	}
	if err := sink.onCapabilities(&capsInput); err != nil {
		_ = conn.SendError(ctx, req.ID, err.Error())
		return err
	}
	if err := conn.SendResult(ctx, req.ID, nil); err != nil {
		return fmt.Errorf("stage 3 respond: %w", err)
	}

	// Barrier + Stage 4: share-registry (engine-initiated).
	if !sink.transition(plugin.StageCapability, plugin.StageRegistry) {
		return errStartupBarrierAborted
	}
	if err := sink.deliverRegistry(ctx); err != nil {
		return err
	}
	if !sink.transition(plugin.StageRegistry, plugin.StageReady) {
		return errStartupBarrierAborted
	}

	// Stage 5: ready (plugin-initiated).
	req, err = conn.ReadRequest(ctx)
	if err != nil {
		return fmt.Errorf("stage 5 read: %w", err)
	}
	if req.Method != methodReady {
		var tb textbuf.Buffer
		_ = conn.SendError(ctx, req.ID, tb.Str("expected ready, got ").Str(req.Method).String())
		return fmt.Errorf("stage 5: expected ready, got %s", req.Method)
	}
	// Startup subscriptions ride in the ready params. A parse failure is
	// non-fatal: startup proceeds with a zero-value ReadyInput (no subscription,
	// no bridge transport), matching the pre-refactor engine behavior.
	var readyInput rpc.ReadyInput
	if req.Params != nil {
		if perr := json.Unmarshal(req.Params, &readyInput); perr != nil {
			logger().Warn("rpc startup: invalid ready params", "error", perr)
		}
	}
	if err := sink.onReady(&readyInput); err != nil {
		_ = conn.SendError(ctx, req.ID, err.Error())
		return err
	}

	// Final transition: Ready -> Running. Move the barrier BEFORE the OK below
	// so every plugin in the tier has registered its commands and reached
	// StageReady before any receives OK and starts its runtime event loop.
	if !sink.transition(plugin.StageReady, plugin.StageRunning) {
		return errStartupBarrierAborted
	}
	sink.onRunning()
	if err := conn.SendResult(ctx, req.ID, nil); err != nil {
		return fmt.Errorf("stage 5 respond: %w", err)
	}
	sink.postReady(&readyInput)
	return nil
}
