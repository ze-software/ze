// Design: docs/architecture/api/process-protocol.md — callback registration methods
// Overview: sdk.go — plugin SDK core

package sdk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
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

// ByeHandler completes removal before acknowledging it. An error keeps the
// plugin running so the engine can retain its dependencies and retry removal.
type ByeHandler func(reason string) error

// EncodeNLRIHandler handles NLRI encoding requests. Returns hex-encoded NLRI.
type EncodeNLRIHandler func(family string, args []string) (string, error)

// DecodeNLRIHandler handles NLRI decoding requests. Returns a Go value (JSON-marshaled by the SDK).
// addPath states whether each NLRI in hex carries a 4-octet Path Identifier
// ahead of it (RFC 7911 Section 3).
// withdraw selects MP_UNREACH withdrawal semantics rather than announcement semantics.
type DecodeNLRIHandler func(family string, hex string, addPath, withdraw bool) (any, error)

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

// ConfigCommittedHandler handles the commit of a transaction the plugin
// applied in: no rollback can undo that apply any more.
type ConfigCommittedHandler func(txID string) error

// ConfigOperationDecomposeHandler handles operation decomposition.
type ConfigOperationDecomposeHandler func(ConfigOperationDecomposeInput) (*ConfigOperationDecomposeOutput, error)

// ConfigOperationVerifyHandler handles operation verification.
type ConfigOperationVerifyHandler func(ConfigOperationVerifyInput) error

// ConfigOperationApplyHandler handles applying one config operation.
type ConfigOperationApplyHandler func(ConfigOperationApplyInput) (*ConfigOperationApplyOutput, error)

// ConfigOperationRollbackHandler handles rolling back config operations.
type ConfigOperationRollbackHandler func(ConfigOperationRollbackInput) error

// ConfigOperationCommitHandler acknowledges reversible operation commit.
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
		// A refused removal can redeliver the committed configuration without
		// killing the live plugin or losing its retryable cleanup state.
		callbackConfigure: func(params json.RawMessage) (json.RawMessage, error) {
			return nil, p.handleConfigure(params)
		},
		// Config verify: a plugin that can apply a change accepts it when it
		// registered no verify handler, and one that cannot is refused here,
		// before any participant applies.
		callbackConfigVerify: p.configVerifyCallback(acceptConfigSections),
		// Config apply: refused until OnConfigApply or DeclareAppliesNothing
		// replaces it. Verify already refused such a plugin, so this answers
		// only an engine that skipped verify, and it never answers OK.
		callbackConfigApply: p.refuseConfigApply,
		callbackConfigRollback: func(json.RawMessage) (json.RawMessage, error) {
			p.configTx.rollbackOwed()
			return nil, nil
		},
		// Commit closes the open apply, so a later transaction's rollback
		// does not replay an undo that belongs to this committed one.
		callbackConfigCommitted: func(json.RawMessage) (json.RawMessage, error) {
			p.configTx.committed()
			return nil, nil
		},
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

// OnBye sets the handler for the engine's `bye`. The engine sends it when it
// removes this plugin while the daemon keeps running (reason `removed`) and
// when it stops a subsystem process. It never sends it on daemon stop, which
// closes the connection instead, so cleanup owed then runs after Run returns.
func (p *Plugin) OnBye(fn ByeHandler) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.callbacks[callbackBye] = func(params json.RawMessage) (json.RawMessage, error) {
		var input struct {
			Reason string `json:"reason,omitempty"`
		}
		if params != nil {
			if err := json.Unmarshal(params, &input); err != nil {
				return nil, fmt.Errorf("unmarshal bye: %w", err)
			}
		}
		return nil, fn(input.Reason)
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
//
// The handler receives the address family, the hex-encoded NLRI section,
// whether the session negotiated ADD-PATH for that family, and whether the
// enclosing message withdraws the NLRI. It returns a Go data structure, which
// the SDK marshals once into the response.
//
// The third argument is not advisory. RFC 7911 Section 3 puts a 4-octet Path
// Identifier ahead of EACH NLRI in the section when ADD-PATH is negotiated,
// and nothing in the octets says so, so a handler that ignores it reads the
// identifier as prefix bytes and answers with routes the peer never sent.
// The fourth argument selects MP_UNREACH withdrawal semantics; false or absent
// means announcement. Callers MUST supply the enclosing message's action, and
// handlers MUST NOT infer that action from the NLRI bytes.
// DecodeNLRIHandler carries the same statement over the type.
func (p *Plugin) OnDecodeNLRI(fn DecodeNLRIHandler) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.callbacks[callbackDecodeNLRI] = func(params json.RawMessage) (json.RawMessage, error) {
		var input rpc.DecodeNLRIInput
		if err := json.Unmarshal(params, &input); err != nil {
			return nil, fmt.Errorf("unmarshal decode-nlri: %w", err)
		}
		data, err := fn(input.Family, input.Hex, input.AddPath, input.Withdraw)
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
		//
		// A handler that reported a warning has its terminator spelled
		// rpc.AnswerKindWarning, so the engine reads the status the handler
		// returned rather than done.
		head := rpc.AnswerTail{
			Key:     records.Key,
			Fields:  records.Fields,
			Message: answerFailureMessage(status, nil),
			Warning: status == rpc.StatusWarning,
		}
		return nil, rpc.WriteRecordAnswer(answer, id, head, records.Rows)
	}
	// The value the handler built is the document a bounded walk collapses to,
	// so it takes the same three lines: the head states how the record is read,
	// the one record line carries the value's bytes unchanged, and the
	// terminator counts it and states the failure, if any (AC-5).
	out, err := executeCommandOutput(status, data)
	if err != nil {
		return nil, err
	}
	head := rpc.AnswerTail{Message: answerFailureMessage(status, out.Data), Warning: status == rpc.StatusWarning}
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
// or an error to reject. If no handler is registered, config-verify accepts.
// Either way a plugin that registered neither OnConfigApply nor
// DeclareAppliesNothing is refused before the handler runs.
func (p *Plugin) OnConfigVerify(fn ConfigVerifyHandler) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.callbacks[callbackConfigVerify] = p.configVerifyCallback(fn)
}

// acceptConfigSections is the verify of a plugin that registered none: every
// candidate is accepted.
func acceptConfigSections([]ConfigSection) error { return nil }

// configVerifyCallback wraps a verify handler. It refuses first when the
// plugin has no way to apply a change (refuseWithoutApply).
func (p *Plugin) configVerifyCallback(fn ConfigVerifyHandler) callbackHandler {
	return func(params json.RawMessage) (json.RawMessage, error) {
		var input rpc.ConfigVerifyInput
		if err := json.Unmarshal(params, &input); err != nil {
			return marshalStatusError("unmarshal config-verify: " + err.Error())
		}
		p.mu.Lock()
		applies := p.configApplyRegistered
		p.mu.Unlock()
		if !applies {
			roots := make([]string, 0, len(input.Sections))
			for i := range input.Sections {
				roots = append(roots, input.Sections[i].Root)
			}
			return marshalStatusError(p.refuseWithoutApply(roots))
		}
		if err := fn(input.Sections); err != nil {
			return marshalStatusError(err.Error())
		}
		return marshalStatusOK(nil)
	}
}

// refuseConfigApply is the config-apply of a plugin that registered neither
// OnConfigApply nor DeclareAppliesNothing. It refuses, so no code path answers
// an apply OK without a handler the plugin registered, and it opens no
// rollback.
func (p *Plugin) refuseConfigApply(params json.RawMessage) (json.RawMessage, error) {
	var input rpc.ConfigApplyInput
	if err := json.Unmarshal(params, &input); err != nil {
		return marshalStatusError("unmarshal config-apply: " + err.Error())
	}
	return marshalStatusError(p.refuseWithoutApply(diffSectionRoots(input.Sections)))
}

// refuseWithoutApply is the refusal of a change to roots for a plugin that
// registered neither OnConfigApply nor DeclareAppliesNothing: what is refused,
// why, and the restart that delivers the change at startup instead.
func (p *Plugin) refuseWithoutApply(roots []string) string {
	var message textbuf.Buffer
	message.Str("plugin ").Str(p.name).Str(" cannot apply a change to ")
	for i, root := range roots {
		if i > 0 {
			message.Str(", ")
		}
		message.Str(root)
	}
	message.Str(" while ze runs: it registered no config apply handler and does not declare").
		Str(" that it applies nothing, so it would accept the change and keep running the old config\n").
		Str(RestartSteps())
	return message.String()
}

// diffSectionRoots answers the root of each section of a config-apply.
func diffSectionRoots(sections []ConfigDiffSection) []string {
	roots := make([]string, 0, len(sections))
	for i := range sections {
		roots = append(roots, sections[i].Root)
	}
	return roots
}

// OnConfigApply sets the handler for config apply requests (reload pipeline).
// The handler receives diff sections describing what changed and returns nil to accept
// or an error to reject. fn MUST NOT be nil. A plugin that declares WantsConfig
// MUST register this or DeclareAppliesNothing: without either, every reload
// that reaches it is refused at config-verify.
//
// A handler that returns an error MUST first undo whatever it partly applied:
// the transaction's rollback does not reach a plugin whose own apply failed
// (config_tx_gate.go). The one exception is an error wrapping
// ErrRunningConfigLost, the handler's statement that it could not: the
// rollback then reaches the handler, whose own failure restarts the plugin.
// A handler that accepts an apply without changing
// anything MUST drop the undo set it kept from an earlier apply: the rollback
// handler runs after every accepted apply, and that set belongs to a
// transaction that already committed.
func (p *Plugin) OnConfigApply(fn ConfigApplyHandler) {
	if fn == nil {
		panic("BUG: OnConfigApply needs a handler; a plugin that applies nothing calls DeclareAppliesNothing")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.callbacks[callbackConfigApply] = p.configApplyCallback(fn)
	p.configApplyRegistered = true
}

// DeclareAppliesNothing states that this plugin consumes its config roots and
// holds nothing a reload can apply: it reads no leaf, or only capability
// leaves, whose change DeclareCapabilitiesFrom refuses at config-verify. The
// apply is accepted and a debug line names the plugin, the changed roots and
// reason, so an accepted reload that changed nothing in the plugin says why.
// It is the explicit form of an apply handler, never a default: a plugin that
// reads a leaf it could apply MUST register OnConfigApply instead. reason
// MUST NOT be empty.
func (p *Plugin) DeclareAppliesNothing(reason string) {
	if reason == "" {
		panic("BUG: DeclareAppliesNothing needs the reason the plugin applies nothing")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.callbacks[callbackConfigApply] = p.configApplyCallback(func(sections []ConfigDiffSection) error {
		slog.Debug("config apply: plugin applies nothing", "plugin", p.name, "roots", diffSectionRoots(sections), "reason", reason)
		return nil
	})
	p.configApplyRegistered = true
}

// configApplyCallback wraps a section apply handler and opens the rollback
// gate when the apply is accepted, or when it failed with ErrRunningConfigLost
// and so left something for the rollback to repair. fn MUST NOT be nil: a
// plugin without one keeps refuseConfigApply.
func (p *Plugin) configApplyCallback(fn ConfigApplyHandler) callbackHandler {
	return func(params json.RawMessage) (json.RawMessage, error) {
		var input rpc.ConfigApplyInput
		if err := json.Unmarshal(params, &input); err != nil {
			return marshalStatusError("unmarshal config-apply: " + err.Error())
		}
		if err := fn(input.Sections); err != nil {
			if errors.Is(err, ErrRunningConfigLost) {
				p.configTx.applied()
			}
			return marshalStatusError(err.Error())
		}
		p.configTx.applied()
		return marshalStatusOK(nil)
	}
}

// OnConfigCommitted sets the handler for the commit of a transaction
// (transaction protocol). It runs only when this plugin accepted a
// config-apply that is neither committed nor rolled back, as the rollback
// handler does, so it is where a plugin performs a step of its apply that a
// rollback could not undo, such as tearing down state other components hold
// handles into. Without a handler, a commit only closes the rollback gate.
func (p *Plugin) OnConfigCommitted(fn ConfigCommittedHandler) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.callbacks[callbackConfigCommitted] = func(params json.RawMessage) (json.RawMessage, error) {
		var input struct {
			TransactionID string `json:"transaction-id"`
		}
		if params != nil {
			if err := json.Unmarshal(params, &input); err != nil {
				return nil, fmt.Errorf("unmarshal config-committed: %w", err)
			}
		}
		if !p.configTx.committed() {
			return nil, nil
		}
		return nil, fn(input.TransactionID)
	}
}

// OnConfigComplete registers acknowledged whole-reload cleanup. The handler
// MUST retain transaction-owned undo through operation commit, and MUST finish
// or release its resources before ctx expires. The engine MUST call it only for
// the captured process lifetime, after publication (accepted) or restoration.
// Handlers MUST match TransactionID and never act on a newer transaction.
func (p *Plugin) OnConfigComplete(fn func(context.Context, rpc.ConfigCompleteInput) error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.callbacks[callbackConfigComplete] = func(params json.RawMessage) (json.RawMessage, error) {
		var decoded struct {
			rpc.ConfigCompleteInput
			Accepted *bool `json:"accepted"`
		}
		if err := json.Unmarshal(params, &decoded); err != nil {
			return nil, fmt.Errorf("unmarshal config-complete: %w", err)
		}
		if decoded.Accepted == nil {
			return nil, fmt.Errorf("config-complete requires an explicit boolean accepted decision")
		}
		input := decoded.ConfigCompleteInput
		input.Accepted = *decoded.Accepted
		if input.TransactionID == "" {
			return nil, fmt.Errorf("config-complete requires a transaction")
		}
		if len(input.TransactionID) > 256 {
			return nil, fmt.Errorf("config-complete transaction exceeds 256 bytes")
		}
		if input.DeadlineMS <= 0 {
			return nil, fmt.Errorf("config-complete requires an absolute deadline")
		}
		ctx, cancel := context.WithDeadline(context.Background(), time.UnixMilli(input.DeadlineMS))
		defer cancel()
		out := rpc.ConfigCompleteOutput{TransactionID: input.TransactionID, Accepted: input.Accepted, Status: rpc.StatusOK}
		err := ctx.Err()
		if err == nil {
			err = fn(ctx, input)
			if err == nil {
				err = ctx.Err()
			}
		}
		if err != nil {
			out.Status, out.Error = rpc.StatusError, err.Error()
		}
		return json.Marshal(out)
	}
}

// OnSourcePolicy declares retained source ownership and its lifecycle handler.
// The coordinator MUST drain prior event application before prepare. The owner
// MUST fence its asynchronous workers and downstream settlement before returning,
// honor ctx cancellation, and retain prepare pins until abort or finalize.
// Registration MUST finish before Run; no SDK lock survives a handler call.
func (p *Plugin) OnSourcePolicy(fn func(context.Context, rpc.SourcePolicyInput) error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if fn == nil {
		delete(p.callbacks, callbackSourcePolicy)
		return
	}
	p.callbacks[callbackSourcePolicy] = func(params json.RawMessage) (json.RawMessage, error) {
		var input rpc.SourcePolicyInput
		if err := json.Unmarshal(params, &input); err != nil {
			return nil, fmt.Errorf("unmarshal source-policy: %w", err)
		}
		if err := input.Validate(time.Now()); err != nil {
			return nil, err
		}
		ctx, cancel := context.WithDeadline(context.Background(), time.UnixMilli(input.DeadlineMS))
		defer cancel()
		out := rpc.SourcePolicyOutput{
			TransactionID: input.TransactionID, Operation: input.Operation,
			Source: input.Source, NextPolicyEpoch: input.NextPolicyEpoch, Status: rpc.StatusOK,
		}
		err := ctx.Err()
		if err == nil {
			err = fn(ctx, input)
			if err == nil {
				err = ctx.Err()
			}
		}
		if err != nil {
			out.Status, out.Error = rpc.StatusError, err.Error()
			if len(out.Error) > 4096 {
				return nil, fmt.Errorf("source-policy handler error exceeds 4096 bytes")
			}
		}
		return json.Marshal(out)
	}
}

// OnConfigRollback sets the handler for config rollback requests (transaction protocol).
// The handler receives the transaction ID and should undo changes applied during this
// transaction (typically by calling journal.Rollback()). If no handler is registered,
// rollback is a no-op.
//
// The handler runs only when this plugin accepted a config-apply that is neither
// committed nor rolled back. The engine broadcasts a rollback to every
// participant, and for one that never applied in the failed transaction the
// rollback is a no-op, so a journal kept from an earlier, committed apply is
// never replayed (config_tx_gate.go).
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
		if !p.configTx.rollbackOwed() {
			return nil, nil
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

// OnConfigOperationCommit registers reversible commit. The owner MUST retain
// operation undo until OnConfigComplete receives the matching accepted decision.
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
	p.wantsValidateOpen = true
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

// marshalStatusError returns a JSON status error response with the given message.
func marshalStatusError(msg string) (json.RawMessage, error) {
	return json.Marshal(struct {
		Status string `json:"status"`
		Error  string `json:"error,omitempty"`
	}{Status: rpc.StatusError, Error: msg})
}
