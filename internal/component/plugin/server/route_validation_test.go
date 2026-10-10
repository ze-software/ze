package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/commandtest"
	plugin "github.com/ze-software/ze/internal/component/plugin"
)

// modelPath is a command the registered model declares one argument for:
// leaf `name`, `length "1..128"`, mandatory (ze-telemetry-cmd). The routes that
// read the model's definitions (R4, R8, R9) are judged against it, so these
// tests use the real model rather than a substitute source.
const modelPath = "show metrics name"

var (
	nameAtBound  = strings.Repeat("m", 128)
	nameOverLong = strings.Repeat("m", 129)
)

// VALIDATES: AC-22 for the daemon dispatcher (R1). A token that breaks a
// declared length is refused before the handler runs, a token at the bound
// reaches it, and a command whose model declares no definition still passes
// its tokens through ValidateArgs unchanged (AC-23).
// PREVENTS: Dispatch skipping the validator for any matched command.
// METHOD: register commands with explicit definitions and call Dispatch.
func TestDispatchInvokesWithValidatedArguments(t *testing.T) {
	d := NewDispatcher()
	var got []string
	handler := func(_ *CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
		args := validated.Tokens()
		got = args
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}
	defs := []command.ArgDef{commandtest.Must(command.NewStringArg("label", []command.UintRange{{Min: 1, Max: 4}}, nil, command.ArgOptions{Mandatory: true}))}
	if err := d.RegisterWithOptions("show route test probe", handler, "", RegisterOptions{ReadOnly: true, ArgDefs: defs}); err != nil {
		t.Fatal(err)
	}
	if err := d.RegisterWithOptions("show route test free", handler, "", RegisterOptions{ReadOnly: true}); err != nil {
		t.Fatal(err)
	}

	if _, err := d.Dispatch(nil, "show route test probe abcde"); err == nil {
		t.Fatal("an over-long label was accepted")
	}
	if got != nil {
		t.Fatalf("an over-long label reached the handler as %q", got)
	}
	if _, err := d.Dispatch(nil, "show route test probe abcd"); err != nil {
		t.Fatalf("a label at the bound was refused: %v", err)
	}
	if len(got) != 1 || got[0] != "abcd" {
		t.Errorf("handler args = %q, want [abcd]", got)
	}
	if _, err := d.Dispatch(nil, "show route test free any thing"); err != nil {
		t.Fatalf("a command with no definitions refused its tokens: %v", err)
	}
	if len(got) != 2 || got[0] != "any" || got[1] != "thing" {
		t.Errorf("handler args = %q, want [any thing]", got)
	}
}

// VALIDATES: AC-22 for the RPC wrapper (R4): the params' arguments are judged
// against the model's leaves for the command before the handler runs.
// PREVENTS: the dormant RPC route running a handler on unjudged tokens once
// somebody wires it.
func TestWrapHandlerInvokesWithValidatedArguments(t *testing.T) {
	s := &Server{dispatcher: NewDispatcher()}
	var got []string
	ran := false
	rpcHandler := s.wrapHandler(func(_ *CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
		args := validated.Tokens()
		ran, got = true, args
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}, modelPath, true)

	params := func(name string) json.RawMessage {
		encoded, err := json.Marshal(rpcParams{Args: []string{name}})
		if err != nil {
			t.Fatal(err)
		}
		return encoded
	}
	if _, err := rpcHandler("m", params(nameOverLong)); err == nil {
		t.Fatal("an over-long name was accepted")
	}
	if ran {
		t.Fatalf("an over-long name reached the handler as %q", got)
	}
	if _, err := rpcHandler("m", params(nameAtBound)); err != nil {
		t.Fatalf("a name at the bound was refused: %v", err)
	}
	if len(got) != 1 || got[0] != nameAtBound {
		t.Errorf("handler args = %q, want the name at the bound", got)
	}
}

// VALIDATES: AC-22 for the ensure chain (R5): an ancestor's leaves, bound as
// selectors, are judged against the ancestor's definitions before its creation
// handler runs, and a refusal runs neither the step nor the leaf.
// PREVENTS: an ensure step creating a resource from a value its leaf forbids.
func TestEnsureChainInvokesWithValidatedArguments(t *testing.T) {
	stepRan, leafRan := false, false
	step := EnsureStep{
		Handler: func(_ *CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
			args := validated.Tokens()
			stepRan = true
			if len(args) != 0 {
				t.Errorf("ensure step got tokens %q, want none", args)
			}
			return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map{"created": false}}, nil
		},
		RollbackHandler: func(*CommandContext, command.ValidatedArgs) (*plugin.Response, error) {
			return &plugin.Response{Status: plugin.StatusDone}, nil
		},
		WireMethod: "test:create",
		ArgDefs:    []command.ArgDef{commandtest.Must(command.NewStringArg("name", []command.UintRange{{Min: 1, Max: 4}}, nil, command.ArgOptions{Mandatory: true}))},
	}
	leaf := func(*CommandContext, command.ValidatedArgs) (*plugin.Response, error) {
		leafRan = true
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}
	wrapped := wrapWithEnsureChain(leaf, []EnsureStep{step})

	if _, err := wrapped(&CommandContext{Selectors: map[string]string{"name": "abcde"}}, commandtest.Args()); err == nil {
		t.Fatal("an over-long ancestor name was accepted")
	}
	if stepRan || leafRan {
		t.Fatalf("a refused ancestor ran: step %v leaf %v", stepRan, leafRan)
	}
	if _, err := wrapped(&CommandContext{Selectors: map[string]string{"name": "abcd"}}, commandtest.Args()); err != nil {
		t.Fatalf("an ancestor name at the bound was refused: %v", err)
	}
	if !stepRan || !leafRan {
		t.Errorf("an accepted ancestor did not run: step %v leaf %v", stepRan, leafRan)
	}
}

// VALIDATES: AC-22 for SSH streaming (R8): the lookup judges the words after
// the streaming prefix against the model's leaves, answers no handler on a
// refusal, and the judged tokens at the bound.
// PREVENTS: a streaming handler running on a value its leaf forbids.
func TestStreamingLookupInvokesWithValidatedArguments(t *testing.T) {
	streamingHandlersMu.Lock()
	saved := streamingHandlers
	streamingHandlers = make(map[string]StreamingHandler)
	streamingHandlersMu.Unlock()
	t.Cleanup(func() {
		streamingHandlersMu.Lock()
		streamingHandlers = saved
		streamingHandlersMu.Unlock()
	})
	RegisterStreamingHandler(modelPath, func(context.Context, *Server, io.Writer, string, command.ValidatedArgs) error { return nil })

	handler, _, err := GetStreamingHandlerForCommand(modelPath + " " + nameOverLong)
	if err == nil {
		t.Fatal("an over-long name was accepted")
	}
	if handler != nil {
		t.Fatal("a refused streaming command answered a handler")
	}
	if !IsStreamingCommand(modelPath + " " + nameOverLong) {
		t.Error("a refused argument made the command stop being a streaming command")
	}
	handler, validated, err := GetStreamingHandlerForCommand(modelPath + " " + nameAtBound)
	if err != nil {
		t.Fatalf("a name at the bound was refused: %v", err)
	}
	if handler == nil {
		t.Fatal("no handler for an accepted streaming command")
	}
	if got := validated.Tokens(); len(got) != 1 || got[0] != nameAtBound {
		t.Errorf("judged tokens = %q, want the name at the bound", got)
	}
}

// VALIDATES: AC-22 for plugin forwarding (R9): routeToProcess judges the
// tokens against the model's leaves for the plugin's command before it asks any
// process, so a refusal never reaches the plugin and an accepted call proceeds
// to the process lookup.
// PREVENTS: a plugin command receiving a value its declared leaf forbids.
// METHOD: a registered command with no process: an accepted call fails with
// ErrPluginProcessNotRunning, which is reached only after validation.
func TestRouteToProcessInvokesWithValidatedArguments(t *testing.T) {
	d := NewDispatcher()
	cmd := &RegisteredCommand{Name: modelPath, LowerName: modelPath}

	resp, err := d.routeToProcess(nil, cmd, []string{nameOverLong}, "*")
	if err == nil || errors.Is(err, ErrPluginProcessNotRunning) {
		t.Fatalf("an over-long name was forwarded: err %v", err)
	}
	if resp == nil || resp.Status != plugin.StatusError {
		t.Errorf("refusal response = %+v, want an error status", resp)
	}
	if _, err := d.routeToProcess(nil, cmd, []string{nameAtBound}, "*"); !errors.Is(err, ErrPluginProcessNotRunning) {
		t.Fatalf("a name at the bound: err %v, want ErrPluginProcessNotRunning", err)
	}
}

// VALIDATES: AC-22 for a forked subsystem (R9): the words after the command
// the subsystem declared are judged before the subsystem is asked.
// PREVENTS: a subsystem receiving a value its declared leaf forbids.
// METHOD: a handler with no process: an accepted call fails with
// ErrSubsystemNotRunning, which is reached only after validation.
func TestDispatchSubsystemInvokesWithValidatedArguments(t *testing.T) {
	d := NewDispatcher()
	handler := NewSubsystemHandler(SubsystemConfig{Name: "test"})
	handler.commands = []string{modelPath}
	ctx := &CommandContext{}

	if _, err := d.dispatchSubsystem(ctx, handler, modelPath+" "+nameOverLong); err == nil || errors.Is(err, ErrSubsystemNotRunning) {
		t.Fatalf("an over-long name reached the subsystem: err %v", err)
	}
	if _, err := d.dispatchSubsystem(ctx, handler, modelPath+" "+nameAtBound); !errors.Is(err, ErrSubsystemNotRunning) {
		t.Fatalf("a name at the bound: err %v, want ErrSubsystemNotRunning", err)
	}
}
