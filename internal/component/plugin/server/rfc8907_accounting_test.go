// Design: docs/architecture/aaa-tacacs.md -- accounting at command admission.
package server

import (
	"context"
	"reflect"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	_ "github.com/ze-software/ze/internal/component/tacacs/yang"
	"github.com/ze-software/ze/internal/core/redact"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

type typedAccountingCapture struct {
	fakeAccountant
	startTokens [][]string
	stopTokens  [][]string
}

func (a *typedAccountingCapture) CommandStartArgs(_, _ string, tokens []string) string {
	a.startTokens = append(a.startTokens, slices.Clone(tokens))
	return "task-1"
}

func (a *typedAccountingCapture) CommandStopArgs(_, _, _ string, tokens []string) {
	a.stopTokens = append(a.stopTokens, slices.Clone(tokens))
}

// RFC requirement: RFC8907-8.3-4 positive -- typed inter-plugin dispatch records START and STOP even when authorization refuses the command, without flattening its arguments.
func TestRFC8907TypedDispatchAccountsDeniedCommand(t *testing.T) {
	d := NewDispatcher()
	accountant := &typedAccountingCapture{}
	d.SetAccountingHook(accountant)
	authorizer := &captureCommandArgsAuthorizer{allow: false}
	d.SetAuthorizer(authorizer)
	s := &Server{dispatcher: d}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	defer s.cancel()
	caller := process.NewProcess(plugin.PluginConfig{Name: "caller-plugin"})
	args := []string{"two words", `quote"inside`, `slash\inside`}
	_, err := s.dispatchCommandArgs(t.Context(), caller, "request target echo", args, "*")
	if err != ErrUnauthorized {
		t.Fatalf("dispatch error = %v, want denial", err)
	}
	want := [][]string{{"request", "target", "echo", "two words", `quote"inside`, `slash\inside`}}
	if !reflect.DeepEqual(accountant.startTokens, want) || !reflect.DeepEqual(accountant.stopTokens, want) {
		t.Fatalf("typed accounting changed the command: START=%q STOP=%q", accountant.startTokens, accountant.stopTokens)
	}
	if !reflect.DeepEqual(authorizer.args, args) {
		t.Fatalf("accounting changed the policy input: %q", authorizer.args)
	}
}

// The string command path gives policy, accounting and the handler the same
// quoted argument value.
func TestDispatcherQuotedArgumentsReachPolicyAndAccounting(t *testing.T) {
	d := NewDispatcher()
	accountant := &typedAccountingCapture{}
	d.SetAccountingHook(accountant)
	authorizer := &captureCommandArgsAuthorizer{allow: true}
	d.SetAuthorizer(authorizer)
	var handled []string
	d.Register("request target echo", func(_ *CommandContext, args []string) (*plugin.Response, error) {
		handled = slices.Clone(args)
		return plugin.NewResponse(plugin.StatusDone, nil), nil
	}, "")
	_, err := d.Dispatch(&CommandContext{Username: "alice"}, `request target echo "two words"`)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(handled, []string{"two words"}) || !reflect.DeepEqual(authorizer.args, []string{"target", "echo", "two words"}) {
		t.Fatalf("argument boundaries diverged: handler=%q policy=%q", handled, authorizer.args)
	}
	want := [][]string{{"request", "target", "echo", "two words"}}
	if !reflect.DeepEqual(accountant.startTokens, want) || !reflect.DeepEqual(accountant.stopTokens, want) {
		t.Fatalf("quoted argument accounting = %q / %q", accountant.startTokens, accountant.stopTokens)
	}
}

// RFC requirement: RFC8907-10.5.1-1 negative -- dispatch accounts a TACACS+ key update without sending its quoted secret to the accountant, while the handler receives the original value.
func TestRFC8907AccountingRedactsConfigSecretWithoutChangingExecution(t *testing.T) {
	d := NewDispatcher()
	accountant := &typedAccountingCapture{}
	d.SetAccountingHook(accountant)
	var handled []string
	d.Register("set", func(_ *CommandContext, args []string) (*plugin.Response, error) {
		handled = slices.Clone(args)
		return plugin.NewResponse(plugin.StatusDone, nil), nil
	}, "")
	_, err := d.Dispatch(&CommandContext{Username: "alice"}, `set system authentication tacacs server 192.0.2.1 key "private value"`)
	if err != nil {
		t.Fatal(err)
	}
	if len(handled) != 7 || handled[6] != "private value" {
		t.Fatalf("execution lost the secret value: %q", handled)
	}
	want := [][]string{{"set", "system", "authentication", "tacacs", "server", "192.0.2.1", "key", redact.Placeholder}}
	if !reflect.DeepEqual(accountant.startTokens, want) || !reflect.DeepEqual(accountant.stopTokens, want) {
		t.Fatalf("secret command accounting = %q / %q", accountant.startTokens, accountant.stopTokens)
	}
}

// RFC requirement: RFC8907-8.3-4 positive -- a command routed to a registered plugin emits one accounting pair around its successful IPC execution.
func TestRFC8907PluginDispatchAccountsCommand(t *testing.T) {
	d := NewDispatcher()
	accountant := &fakeAccountant{}
	d.SetAccountingHook(accountant)
	done := registerExecuteCommandTarget(t, d, "request target echo", 1,
		func(_ int, input *rpc.ExecuteCommandInput) (*rpc.ExecuteCommandOutput, error) {
			if !reflect.DeepEqual(input.Args, []string{"alpha"}) {
				t.Errorf("plugin received wrong args: %q", input.Args)
			}
			return &rpc.ExecuteCommandOutput{Status: plugin.StatusDone}, nil
		})
	input := "request target echo alpha"
	response, err := d.Dispatch(&CommandContext{Username: "alice"}, input)
	if err != nil || response == nil || response.Status != plugin.StatusDone {
		t.Fatalf("plugin dispatch failed: response=%+v error=%v", response, err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(accountant.starts, []string{input}) || !reflect.DeepEqual(accountant.stops, []string{input}) {
		t.Fatalf("plugin command was not accounted exactly once: START=%q STOP=%q", accountant.starts, accountant.stops)
	}
}
