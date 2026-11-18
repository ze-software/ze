// Design: docs/architecture/api/process-protocol.md -- cleanup declaration admission
// Related: startup.go -- registrationFromRPC; stage1_checks.go -- real sink checks
package server

import (
	"encoding/json"
	"fmt"
	"net"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// The actual declaration decoder and engine projection preserve both forms
// of the opt-in without changing another accepted metadata field.
func TestExitCleanupDeclarationCopiesBothForms(t *testing.T) {
	for _, declared := range []bool{false, true} {
		input := rpc.DeclareRegistrationInput{ProtocolVersion: rpc.ProtocolVersion,
			ExitConfirmsCleanup: declared, StartedHandler: true}
		encoded, err := json.Marshal(input)
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := rpc.DecodeDeclaration("terminal-copy", encoded)
		if err != nil {
			t.Fatal(err)
		}
		if decoded.ExitConfirmsCleanup != declared {
			t.Fatalf("decoded cleanup=%v", decoded.ExitConfirmsCleanup)
		}
		converted := registrationFromRPC(&decoded)
		if converted.ExitConfirmsCleanup != declared {
			t.Fatalf("converted cleanup=%v", converted.ExitConfirmsCleanup)
		}
		if !converted.StartedHandler {
			t.Fatal("cleanup metadata changed the existing readiness declaration")
		}
	}
}

// Compiled engines must agree before configuration or OnStarted. Neither a
// missing runtime promise nor an undeclared zero exit can supply that agreement.
func TestExitCleanupCompiledDeclarationRequiresAgreement(t *testing.T) {
	for _, compiled := range []bool{false, true} {
		for _, declared := range []bool{false, true} {
			t.Run(fmt.Sprintf("%t-%t", compiled, declared), func(t *testing.T) {
				name := "terminal-agreement"
				exitCleanupCompiled(t, name, compiled)
				s, _ := newLifecycleStartupServer(t)
				proc := process.NewProcess(plugin.PluginConfig{Name: name, Internal: true})
				sink := &engineStartupSink{s: s, proc: proc}
				input := validStage1Declaration()
				input.ExitConfirmsCleanup = declared
				err := sink.refuseSinkChecks(input)
				if compiled == declared {
					if err != nil {
						t.Fatalf("matching declaration refused: %v", err)
					}
				} else if err == nil {
					t.Fatal("compiled and live cleanup declarations disagreed before native startup")
				}
			})
		}
	}
}

// A forked program owns its declaration even if its configured name matches
// a compiled row. It must borrow neither that row's promise nor its refusal.
func TestExitCleanupExternalDeclarationNeverBorrowsCompiledName(t *testing.T) {
	for _, compiled := range []bool{false, true} {
		for _, declared := range []bool{false, true} {
			name := fmt.Sprintf("terminal-external-%t-%t", compiled, declared)
			exitCleanupCompiled(t, name, compiled)
			s, _ := newLifecycleStartupServer(t)
			proc := process.NewProcess(plugin.PluginConfig{Name: name, Run: "/external/program"})
			sink := &engineStartupSink{s: s, proc: proc}
			input := validStage1Declaration()
			input.ExitConfirmsCleanup = declared
			if err := sink.refuseSinkChecks(input); err != nil {
				t.Fatalf("external declaration borrowed compiled metadata: %v", err)
			}
		}
	}
}

func exitCleanupCompiled(t *testing.T, name string, declared bool) {
	t.Helper()
	snapshot := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snapshot) })
	if err := registry.Register(registry.Registration{Name: name, Description: "compiled cleanup agreement fixture",
		ExitConfirmsCleanup: declared, RunEngine: func(net.Conn) int { return 0 }, CLIHandler: func([]string) int { return 0 }}); err != nil {
		t.Fatal(err)
	}
	copy := registry.Lookup(name)
	if copy == nil {
		t.Fatal("fixture registration is absent")
	}
	copy.ExitConfirmsCleanup = !declared
	if got := registry.Lookup(name); got.ExitConfirmsCleanup != declared {
		t.Fatal("caller changed the compiled cleanup promise")
	}
}
