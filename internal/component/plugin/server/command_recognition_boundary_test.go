// Design: docs/architecture/cli/command-completion.md -- registered operational input.
// Related: subsystem.go -- pre-registration and actual Stage-1 declaration.
package server

import (
	"reflect"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestSubsystemPreRegistrationOwnsCommandNames observes the constructor's
// pre-registration snapshot after its caller mutates the supplied slice.
func TestSubsystemPreRegistrationOwnsCommandNames(t *testing.T) {
	commands := []string{"request recognition task"}
	handler := NewSubsystemHandler(SubsystemConfig{Name: "recognition-copy", Commands: commands})
	commands[0] = "request replacement task"
	if got := handler.Commands(); !reflect.DeepEqual(got, []string{"request recognition task"}) {
		t.Fatalf("pre-registered commands = %v, want the constructor's owned command", got)
	}
}

// TestSubsystemDeclarationReplacesPreRegistration observes the real Stage-1
// sink replacing the provisional list, including withdrawal of all commands.
// Start owns handler.mu while it calls this sink; the test follows that contract.
func TestSubsystemDeclarationReplacesPreRegistration(t *testing.T) {
	handler := NewSubsystemHandler(SubsystemConfig{Name: "recognition-declare", Commands: []string{"request old task"}})
	// Start creates the external Process before Stage 1. This ordinary
	// declaration reserves no cleanup capacity and needs no running child.
	handler.proc = process.NewProcess(plugin.PluginConfig{Name: "subsystem-" + handler.config.Name})
	sink := &hubStartupSink{h: handler}
	handler.mu.Lock()
	err := sink.onRegistration(&rpc.DeclareRegistrationInput{Commands: []rpc.CommandDecl{{Name: "request current task"}}})
	handler.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	if got := handler.Commands(); !reflect.DeepEqual(got, []string{"request current task"}) {
		t.Fatalf("declared commands = %v, want only the current Stage-1 declaration", got)
	}
	handler.mu.Lock()
	err = sink.onRegistration(&rpc.DeclareRegistrationInput{})
	handler.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	if got := handler.Commands(); len(got) != 0 {
		t.Fatalf("empty declaration retained commands = %v", got)
	}
}

// TestSubsystemCommandMatchUsesWordBoundary observes the same declared command
// before and after snapshot publication. A longer final word is another command;
// a space or tab introduces arguments to this one.
func TestSubsystemCommandMatchUsesWordBoundary(t *testing.T) {
	manager := NewSubsystemManager()
	handler := &SubsystemHandler{commands: []string{"request recognition task"}}
	manager.Replace("recognition-boundary", handler)
	check := func() {
		t.Helper()
		for _, input := range []string{"request recognition task", "REQUEST recognition task name alpha", "request recognition task\tname alpha"} {
			if got := manager.FindHandler(input); got != handler {
				t.Errorf("handler for %q = %p, want declared owner %p", input, got, handler)
			}
		}
		if got := manager.FindHandler("request recognition taskful"); got != nil {
			t.Errorf("handler for another word = %p, want no match", got)
		}
	}
	check()
	manager.Freeze()
	check()
}

// TestCommandRecognitionIgnoresRenderingEscapeGrammar probes the registered
// command before a real pipe boundary. Rendering escapes and an unfinished
// rendering quote belong to that surface, not the invocation scanner.
func TestCommandRecognitionIgnoresRenderingEscapeGrammar(t *testing.T) {
	dispatcher := NewDispatcher()
	if err := dispatcher.Register("show recognition probe", func(*CommandContext, command.ValidatedArgs) (*plugin.Response, error) {
		return plugin.NewResponse(plugin.StatusDone, plugin.Map{}), nil
	}, "Inspect recognition"); err != nil {
		t.Fatal(err)
	}
	for _, input := range []string{
		`show recognition probe | match "\d"`,
		`show recognition probe | match \d`,
		`show recognition probe | match "unfinished`,
	} {
		if !dispatcher.RecognizesCommand(input) {
			t.Errorf("rendering suffix changed recognition of %q", input)
		}
	}
}

// TestCommandRecognitionRefusesCommandEscapeGrammar probes the opposite side
// of the same boundary: an invalid invocation remains invalid, even when a
// pipe-like character or a rendering suffix follows it.
func TestCommandRecognitionRefusesCommandEscapeGrammar(t *testing.T) {
	dispatcher := NewDispatcher()
	if err := dispatcher.Register("show recognition probe", func(*CommandContext, command.ValidatedArgs) (*plugin.Response, error) {
		return plugin.NewResponse(plugin.StatusDone, plugin.Map{}), nil
	}, "Inspect recognition"); err != nil {
		t.Fatal(err)
	}
	for _, input := range []string{
		`show recognition probe \d | match "\d"`,
		`show recognition probe "\d" | match "\d"`,
		`show recognition probe "unfinished | match "\d"`,
	} {
		if dispatcher.RecognizesCommand(input) {
			t.Errorf("invalid command escaped recognition checks: %q", input)
		}
	}
}
