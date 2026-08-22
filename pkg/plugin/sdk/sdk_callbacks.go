// Design: docs/architecture/api/process-protocol.md — callback registration methods
// Overview: sdk.go — plugin SDK core

package sdk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// ConfigureHandler handles Stage 2 config delivery. Return nil to accept, error to reject.
type ConfigureHandler func([]ConfigSection) error

// ShareRegistryHandler handles Stage 4 registry delivery.
type ShareRegistryHandler func([]RegistryCommand)

// EventHandler handles runtime text event delivery (one JSON-encoded event string per call).
type EventHandler func(event string) error

// StructuredEventHandler handles structured event delivery via DirectBridge.
// Each element is a *rpc.StructuredEvent.
type StructuredEventHandler func(events []any) error

// ByeHandler handles shutdown notification with the shutdown reason.
type ByeHandler func(reason string)

// EncodeNLRIHandler handles NLRI encoding requests. Returns hex-encoded NLRI.
type EncodeNLRIHandler func(family string, args []string) (string, error)

// DecodeNLRIHandler handles NLRI decoding requests. Returns a Go value (JSON-marshaled by the SDK).
type DecodeNLRIHandler func(family string, hex string) (any, error)

// DecodeCapabilityHandler handles capability decoding requests. Returns a Go value (JSON-marshaled by the SDK).
type DecodeCapabilityHandler func(code uint8, hex string) (any, error)

// ExecuteCommandHandler handles command execution requests.
// Returns status, data (JSON-marshaled by the SDK), and error.
//
// A command that walks a large collection returns a Records as its data: the
// walk itself rather than the collection it would build. The SDK then writes
// the answer one row at a time and holds no more than one row, and the handler
// MUST keep whatever those rows read alive until it returns, because the walk
// runs before the call it was returned from is answered
// (Records, pkg/plugin/records.go).
//
// Every other data value keeps the answer it has always had: one marshaled
// value in ExecuteCommandOutput.Data, byte for byte.
type ExecuteCommandHandler func(serial, command string, args []string, peer string) (string, any, error)

// ConfigVerifyHandler handles config verification in the reload pipeline.
// Return nil to accept the candidate config, error to reject.
type ConfigVerifyHandler func([]ConfigSection) error

// ConfigApplyHandler handles config apply in the reload pipeline.
type ConfigApplyHandler func([]ConfigDiffSection) error

// ConfigRollbackHandler handles config rollback in the transaction protocol.
type ConfigRollbackHandler func(txID string) error

// ConfigOperationDecomposeHandler handles operation decomposition.
type ConfigOperationDecomposeHandler func(ConfigOperationDecomposeInput) (*ConfigOperationDecomposeOutput, error)

// ConfigOperationVerifyHandler handles operation verification.
type ConfigOperationVerifyHandler func(ConfigOperationVerifyInput) error

// ConfigOperationApplyHandler handles applying one config operation.
type ConfigOperationApplyHandler func(ConfigOperationApplyInput) (*ConfigOperationApplyOutput, error)

// ConfigOperationRollbackHandler handles rolling back config operations.
type ConfigOperationRollbackHandler func(ConfigOperationRollbackInput) error

// ConfigOperationCommitHandler handles committing operation journals.
type ConfigOperationCommitHandler func(ConfigOperationCommitInput) error

// ValidateOpenHandler handles OPEN validation requests. Returns accept/reject decision.
type ValidateOpenHandler func(*ValidateOpenInput) *ValidateOpenOutput

// FilterUpdateHandler handles route filter requests (accept/reject/modify with optional delta).
type FilterUpdateHandler func(*FilterUpdateInput) (*FilterUpdateOutput, error)

// DoctorCheckHandler handles doctor check requests. Receives the check name, returns diagnostics.
type DoctorCheckHandler func(name string) ([]rpc.DoctorCheckDiagnostic, error)

// EnrichShowHandler handles show enrichment requests. Receives the command, key, mode
// ("detail" or "brief"), and base data map. Returns enrichment data to merge into the base map.
type EnrichShowHandler func(command, key, mode string, base map[string]any) (map[string]any, error)

// StartedHandler runs after the 5-stage startup completes but before the event loop.
// Do NOT call DispatchCommand from here targeting other plugins; use AllPluginsReadyHandler.
type StartedHandler func(ctx context.Context) error

// AllPluginsReadyHandler runs via the event loop after all plugins are loaded and registries frozen.
// This is the only safe place to DispatchCommand targeting another plugin at startup.
type AllPluginsReadyHandler func() error

// initCallbackDefaults registers default handlers for callbacks that have
// graceful no-handler behavior (accept/no-op). Called from constructors.
func (p *Plugin) initCallbackDefaults() {
	p.callbacks = map[string]callbackHandler{
		// Events: no-op when no handler registered.
		callbackDeliverEvent: func(json.RawMessage) (json.RawMessage, error) { return nil, nil },
		callbackDeliverBatch: func(json.RawMessage) (json.RawMessage, error) { return nil, nil },
		// Config: accept when no handler registered.
		callbackConfigVerify:   marshalStatusOK,
		callbackConfigApply:    marshalStatusOK,
		callbackConfigRollback: func(json.RawMessage) (json.RawMessage, error) { return nil, nil },
		// Validate-open: accept when no handler registered.
		callbackValidateOpen: func(json.RawMessage) (json.RawMessage, error) {
			return json.Marshal(&rpc.ValidateOpenOutput{Accept: true})
		},
		// Doctor check: empty diagnostics when no handler registered.
		callbackDoctorCheck: func(json.RawMessage) (json.RawMessage, error) {
			return json.Marshal(&rpc.DoctorCheckOutput{Diagnostics: []rpc.DoctorCheckDiagnostic{}})
		},
		// Enrich-show: empty data when no handler registered.
		callbackEnrichShow: func(json.RawMessage) (json.RawMessage, error) {
			return json.Marshal(&rpc.EnrichShowOutput{})
		},
		// Bye: no-op when no handler registered.
		callbackBye: func(json.RawMessage) (json.RawMessage, error) { return nil, nil },
		// Post-startup: no-op when no handler registered. Plugins that need
		// to dispatch to other plugins at startup register via OnAllPluginsReady.
		callbackPostStartup: func(json.RawMessage) (json.RawMessage, error) { return nil, nil },
	}
}

// marshalStatusOK returns a JSON status OK response. Shared default for config callbacks.
func marshalStatusOK(json.RawMessage) (json.RawMessage, error) {
	return json.Marshal(struct {
		Status string `json:"status"`
	}{Status: rpc.StatusOK})
}

// OnConfigure sets the handler for Stage 2 config delivery.
func (p *Plugin) OnConfigure(fn ConfigureHandler) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onConfigure = fn
}

// OnShareRegistry sets the handler for Stage 4 registry delivery.
func (p *Plugin) OnShareRegistry(fn ShareRegistryHandler) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onShareRegistry = fn
}

// OnEvent sets the handler for runtime event delivery.
func (p *Plugin) OnEvent(fn EventHandler) {
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
func (p *Plugin) OnStructuredEvent(fn StructuredEventHandler) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onStructuredEvent = fn
}

// OnBye sets the handler for shutdown notification.
func (p *Plugin) OnBye(fn ByeHandler) {
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
func (p *Plugin) OnEncodeNLRI(fn EncodeNLRIHandler) {
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
// The handler receives the address family and hex-encoded NLRI, and returns
// a Go data structure. The SDK marshals it once into the response.
func (p *Plugin) OnDecodeNLRI(fn DecodeNLRIHandler) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.callbacks[callbackDecodeNLRI] = func(params json.RawMessage) (json.RawMessage, error) {
		var input rpc.DecodeNLRIInput
		if err := json.Unmarshal(params, &input); err != nil {
			return nil, fmt.Errorf("unmarshal decode-nlri: %w", err)
		}
		data, err := fn(input.Family, input.Hex)
		if err != nil {
			return nil, err
		}
		raw, err := json.Marshal(data)
		if err != nil {
			return nil, fmt.Errorf("marshal decode-nlri result: %w", err)
		}
		return json.Marshal(struct {
			JSON json.RawMessage `json:"json"`
		}{JSON: raw})
	}
}

// OnDecodeCapability sets the handler for capability decoding requests.
// The handler receives the capability code and hex-encoded bytes, and returns
// a Go data structure. The SDK marshals it once into the response.
func (p *Plugin) OnDecodeCapability(fn DecodeCapabilityHandler) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.callbacks[callbackDecodeCapability] = func(params json.RawMessage) (json.RawMessage, error) {
		var input rpc.DecodeCapabilityInput
		if err := json.Unmarshal(params, &input); err != nil {
			return nil, fmt.Errorf("unmarshal decode-capability: %w", err)
		}
		data, err := fn(input.Code, input.Hex)
		if err != nil {
			return nil, err
		}
		raw, err := json.Marshal(data)
		if err != nil {
			return nil, fmt.Errorf("marshal decode-capability result: %w", err)
		}
		return json.Marshal(struct {
			JSON json.RawMessage `json:"json"`
		}{JSON: raw})
	}
}

// OnExecuteCommand sets the handler for command execution requests.
// The handler receives serial, command, args, peer and returns (status, data, error).
// The SDK marshals the data value into ExecuteCommandOutput.Data as json.RawMessage,
// producing a single marshal instead of double-encoding.
//
// A handler MAY answer with a Records instead, which is a walk rather than a
// built value. The callback registered here is the transport whose result is
// one marshaled value: the direct bridge, where a walk collapses to the
// document the record path would have carried (Records.MarshalJSON). The socket
// event loop reaches the same code with an answer writer, and every answer it
// writes there is a head, its records and a terminator
// (Plugin.answerExecuteCommand, sdk_dispatch.go).
func (p *Plugin) OnExecuteCommand(fn ExecuteCommandHandler) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onExecuteCommand = fn
	p.callbacks[callbackExecuteCommand] = func(params json.RawMessage) (json.RawMessage, error) {
		return executeCommandAnswer(fn, params, nil, 0)
	}
}

// executeCommandAnswer answers one execute-command callback and reports the
// result the caller must send.
//
// answer is non-nil for a transport that carries a line for each record, which
// is every wire connection. On that transport EVERY answer is the SEQUENCE: a
// walk writes its rows, and a value the handler built writes as the one document
// a bounded walk collapses to. The returned result is then nil, because the
// answer is already on the wire and nothing may follow it, and that is the only
// case where both the result and the error are nil.
//
// The frame never follows the payload, because the engine must know which frame
// is arriving before it reads the first line. A frame chosen from the payload
// would leave the reader guessing, and a reader that guesses wrong takes a head
// line's tail for its result (R-1 of spec-record-answers-1-sdk-path).
//
// answer is nil for a transport whose result is one marshaled value, which is
// the direct bridge, and id is then unread. That result is the answer this
// callback has always carried, byte for byte, and so is the document the
// sequence's one record line carries (AC-5). A walk reaching the value transport
// collapses through the one collapse rather than through a second rendering
// (Records.MarshalJSON).
//
// The walk runs to its end before this returns, which is what lets a handler
// hold the state its rows read for exactly the length of its own call (Records,
// pkg/plugin/records.go).
func executeCommandAnswer(
	fn ExecuteCommandHandler,
	params json.RawMessage,
	answer io.Writer,
	id uint64,
) (json.RawMessage, error) {
	if fn == nil {
		return nil, errors.New("execute-command handler not set")
	}
	var input rpc.ExecuteCommandInput
	if err := json.Unmarshal(params, &input); err != nil {
		return nil, fmt.Errorf("unmarshal execute-command: %w", err)
	}
	status, data, err := fn(input.Serial, input.Command, input.Args, input.Peer)
	if err != nil {
		return nil, err
	}
	if answer == nil {
		out, outErr := executeCommandOutput(status, data)
		if outErr != nil {
			return nil, outErr
		}
		return json.Marshal(out)
	}

	if records, walk := data.(Records); walk {
		// A handler that reported a failure beside a walk gave no text with it,
		// so the terminator states the fixed one. A failure that said nothing
		// would otherwise reach the caller as a completed answer (rpc.Verdict).
		return nil, records.WriteAnswer(answer, id, answerFailureMessage(status, nil))
	}
	// The value the handler built is the document a bounded walk collapses to,
	// so it takes the same three lines: the head states how the record is read,
	// the one record line carries the value's bytes unchanged, and the
	// terminator counts it and states the failure, if any (AC-5).
	out, err := executeCommandOutput(status, data)
	if err != nil {
		return nil, err
	}
	head := rpc.AnswerTail{Message: answerFailureMessage(status, out.Data)}
	if head.Message != "" {
		// A failing answer carries its reason and no payload, which is what the
		// caller rebuilds the failure from.
		return nil, rpc.WriteDocumentAnswer(answer, id, head, nil)
	}
	return nil, rpc.WriteDocumentAnswer(answer, id, head, out.Data)
}

// executeCommandOutput is the value form of a handler's answer: the status it
// reported and its payload marshaled whole. It is the one place that marshal
// happens, so the bridge and the socket carry the same bytes for the same
// value.
func executeCommandOutput(status string, data any) (*rpc.ExecuteCommandOutput, error) {
	var raw json.RawMessage
	if data != nil {
		var err error
		raw, err = json.Marshal(data)
		if err != nil {
			return nil, fmt.Errorf("marshal execute-command data: %w", err)
		}
	}
	return &rpc.ExecuteCommandOutput{Status: status, Data: raw}, nil
}

// answerFailureMessage is the operational text an answer's TERMINATOR carries
// for the status a handler reported, and the empty string when the handler
// reported no failure.
//
// The terminator is the one line an answer states its outcome on, so a failure
// that named nothing must still say something: rpc.Verdict reads an empty
// message with zero counts as a completed answer, and a silent failure would
// reach the caller as a success (ai/rules/evidence.md). reason is the payload
// the handler built, which is the text its caller has always reported, and a
// handler that built none earns the fixed text instead.
//
// It is the same repair the engine makes for its own answers (answerMessage,
// internal/component/plugin/dispatch.go).
func answerFailureMessage(status string, reason []byte) string {
	if status != rpc.StatusError {
		return ""
	}
	if len(reason) > 0 {
		return string(reason)
	}
	return rpc.AnswerFailureUnstated
}

// OnConfigVerify sets the handler for config verification requests (reload pipeline).
// The handler receives the full candidate config sections and returns nil to accept
// or an error to reject. If no handler is registered, config-verify returns OK (no-op).
func (p *Plugin) OnConfigVerify(fn ConfigVerifyHandler) {
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
func (p *Plugin) OnConfigApply(fn ConfigApplyHandler) {
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

// OnConfigRollback sets the handler for config rollback requests (transaction protocol).
// The handler receives the transaction ID and should undo changes applied during this
// transaction (typically by calling journal.Rollback()). If no handler is registered,
// rollback is a no-op.
func (p *Plugin) OnConfigRollback(fn ConfigRollbackHandler) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.callbacks[callbackConfigRollback] = func(params json.RawMessage) (json.RawMessage, error) {
		var input struct {
			TransactionID string `json:"transaction-id"`
		}
		if params != nil {
			if err := json.Unmarshal(params, &input); err != nil {
				return nil, fmt.Errorf("unmarshal config-rollback: %w", err)
			}
		}
		return nil, fn(input.TransactionID)
	}
}

// OnConfigOperationDecompose sets the handler for operation decomposition.
func (p *Plugin) OnConfigOperationDecompose(fn ConfigOperationDecomposeHandler) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.callbacks[callbackOpDecompose] = func(params json.RawMessage) (json.RawMessage, error) {
		var input rpc.ConfigOperationDecomposeInput
		if err := json.Unmarshal(params, &input); err != nil {
			return marshalStatusError(fmt.Sprintf("unmarshal config-operation-decompose: %v", err))
		}
		out, err := fn(input)
		if err != nil {
			return marshalStatusError(err.Error())
		}
		if out == nil {
			out = &rpc.ConfigOperationDecomposeOutput{Status: rpc.StatusOK}
		}
		if out.Status == "" {
			out.Status = rpc.StatusOK
		}
		return json.Marshal(out)
	}
}

// OnConfigOperationVerify sets the handler for operation verification.
func (p *Plugin) OnConfigOperationVerify(fn ConfigOperationVerifyHandler) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.callbacks[callbackOpVerify] = func(params json.RawMessage) (json.RawMessage, error) {
		var input rpc.ConfigOperationVerifyInput
		if err := json.Unmarshal(params, &input); err != nil {
			return marshalStatusError(fmt.Sprintf("unmarshal config-operation-verify: %v", err))
		}
		if err := fn(input); err != nil {
			return marshalStatusError(err.Error())
		}
		return json.Marshal(&rpc.ConfigOperationVerifyOutput{Status: rpc.StatusOK})
	}
}

// OnConfigOperationApply sets the handler for applying one operation.
func (p *Plugin) OnConfigOperationApply(fn ConfigOperationApplyHandler) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.callbacks[callbackOpApply] = func(params json.RawMessage) (json.RawMessage, error) {
		var input rpc.ConfigOperationApplyInput
		if err := json.Unmarshal(params, &input); err != nil {
			return marshalStatusError(fmt.Sprintf("unmarshal config-operation-apply: %v", err))
		}
		out, err := fn(input)
		if err != nil {
			return marshalStatusError(err.Error())
		}
		if out == nil {
			out = &rpc.ConfigOperationApplyOutput{Status: rpc.StatusOK}
		}
		if out.Status == "" {
			out.Status = rpc.StatusOK
		}
		return json.Marshal(out)
	}
}

// OnConfigOperationRollback sets the handler for rolling back operations.
func (p *Plugin) OnConfigOperationRollback(fn ConfigOperationRollbackHandler) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.callbacks[callbackOpRollback] = func(params json.RawMessage) (json.RawMessage, error) {
		var input rpc.ConfigOperationRollbackInput
		if err := json.Unmarshal(params, &input); err != nil {
			return marshalStatusError(fmt.Sprintf("unmarshal config-operation-rollback: %v", err))
		}
		if err := fn(input); err != nil {
			return marshalStatusError(err.Error())
		}
		return json.Marshal(&rpc.ConfigOperationRollbackOutput{Status: rpc.StatusOK})
	}
}

// OnConfigOperationCommit sets the handler for committing operation journals.
func (p *Plugin) OnConfigOperationCommit(fn ConfigOperationCommitHandler) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.callbacks[callbackOpCommit] = func(params json.RawMessage) (json.RawMessage, error) {
		var input rpc.ConfigOperationCommitInput
		if err := json.Unmarshal(params, &input); err != nil {
			return marshalStatusError(fmt.Sprintf("unmarshal config-operation-commit: %v", err))
		}
		if err := fn(input); err != nil {
			return marshalStatusError(err.Error())
		}
		return json.Marshal(&rpc.ConfigOperationCommitOutput{Status: rpc.StatusOK})
	}
}

// OnValidateOpen sets the handler for OPEN validation requests.
// The handler receives both local and remote OPEN messages and returns accept/reject.
// When registered, WantsValidateOpen is automatically set in Stage 1 registration.
// If no handler is registered, validate-open returns accept (no-op).
func (p *Plugin) OnValidateOpen(fn ValidateOpenHandler) {
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
func (p *Plugin) OnFilterUpdate(fn FilterUpdateHandler) {
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

// OnDoctorCheck sets the handler for doctor check requests.
// The handler receives the check name and returns diagnostics.
// If no handler is registered, doctor-check returns empty diagnostics (no-op).
func (p *Plugin) OnDoctorCheck(fn DoctorCheckHandler) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.callbacks[callbackDoctorCheck] = func(params json.RawMessage) (json.RawMessage, error) {
		var input rpc.DoctorCheckInput
		if err := json.Unmarshal(params, &input); err != nil {
			return nil, fmt.Errorf("unmarshal doctor-check: %w", err)
		}
		diags, err := fn(input.Name)
		if err != nil {
			return nil, err
		}
		if diags == nil {
			diags = []rpc.DoctorCheckDiagnostic{}
		}
		return json.Marshal(&rpc.DoctorCheckOutput{Diagnostics: diags})
	}
}

// OnEnrichShow sets the handler for show enrichment requests.
// The handler receives the command, enricher key, and base data map, and returns
// enrichment data to merge into the base map. If no handler is registered,
// enrich-show returns empty data (no-op).
func (p *Plugin) OnEnrichShow(fn EnrichShowHandler) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.callbacks[callbackEnrichShow] = func(params json.RawMessage) (json.RawMessage, error) {
		var input rpc.EnrichShowInput
		if err := json.Unmarshal(params, &input); err != nil {
			return nil, fmt.Errorf("unmarshal enrich-show: %w", err)
		}
		data, err := fn(input.Command, input.Key, input.Mode, input.Base)
		if err != nil {
			return nil, err
		}
		return json.Marshal(&rpc.EnrichShowOutput{Data: data})
	}
}

// OnStarted sets a callback that runs after the 5-stage startup completes
// but before the event loop begins. This is the safe place to make engine
// calls (e.g., SubscribeEvents) because the connection is no longer blocked
// by the startup coordinator. Do NOT make engine calls inside OnShareRegistry
// or OnConfigure -- those run while the engine is waiting for the response,
// causing a deadlock.
//
// IMPORTANT: OnStarted fires after THIS plugin's 5-stage handshake completes
// but potentially BEFORE other plugins in later startup phases are loaded. Do
// NOT call DispatchCommand from OnStarted targeting a plugin that lives in a
// different startup phase: the dispatcher command registry may not yet contain
// the target command. Use OnAllPluginsReady for cross-plugin dispatches.
func (p *Plugin) OnStarted(fn StartedHandler) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onStarted = fn
}

// OnAllPluginsReady sets a callback that runs via the event loop after the
// engine has finished loading ALL plugins across ALL startup phases (config-
// path auto-load, explicit, family, event-type, send-type) and has frozen the
// dispatcher and plugin registries. This is the only place in startup where a
// plugin can safely DispatchCommand targeting another plugin, because only at
// this point is every other plugin's command guaranteed to be registered.
//
// The handler is dispatched via the normal event loop, so it runs AFTER
// OnStarted has returned. It is delivered best-effort by the engine -- a
// plugin that has died before this point does not receive the callback.
//
// The fn parameter takes no context: runtime calls should use
// context.Background() with an explicit timeout. Errors returned from fn are
// propagated as an RPC error response to the engine and logged there; the
// plugin continues running.
func (p *Plugin) OnAllPluginsReady(fn AllPluginsReadyHandler) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onAllPluginsReady = fn
	p.callbacks[callbackPostStartup] = func(json.RawMessage) (json.RawMessage, error) {
		p.mu.Lock()
		handler := p.onAllPluginsReady
		p.mu.Unlock()
		if handler == nil {
			return nil, nil
		}
		return nil, handler()
	}
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

// SetStartupSubscriptionsIn is SetStartupSubscriptions with an explicit event
// namespace, letting a plugin subscribe to a non-bgp namespace (e.g.
// "vpn-ipsec") at startup. An empty namespace behaves exactly like
// SetStartupSubscriptions (resolves to the engine's default namespace, "bgp"
// today). Must be called before Run(). Added rather than changing
// SetStartupSubscriptions so out-of-tree plugins keep compiling unchanged.
func (p *Plugin) SetStartupSubscriptionsIn(namespace string, events, peers []string, format string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.startupSubscription = &rpc.SubscribeEventsInput{
		Namespace: namespace,
		Events:    events,
		Peers:     peers,
		Format:    format,
	}
}

// SetEnvelope opts this plugin into enveloped event delivery: each delivered
// event string becomes an rpc.EventEnvelope ({namespace, event, payload}) so a
// plugin subscribed to several event types can discriminate which one arrived
// without parsing payload-specific fields (parse with rpc.ParseEventEnvelope).
// Like SetEncoding it applies process-wide. Default false preserves the
// bare-payload delivery every pre-existing consumer relies on. Must be called
// after SetStartupSubscriptions[In] and before Run().
func (p *Plugin) SetEnvelope(enabled bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.startupSubscription == nil {
		p.startupSubscription = &rpc.SubscribeEventsInput{}
	}
	p.startupSubscription.Envelope = enabled
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
