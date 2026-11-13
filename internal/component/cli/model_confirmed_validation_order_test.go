// Design: docs/guide/config-editor.md -- validation precedes the backend refusal.
// Related: model_commands_commit.go -- cmdCommitRequest, commitValidationRefusal.

package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

// TestConfirmedSessionValidatesBeforeDaemonRequirement proves an invalid
// candidate receives the shared refusal even without a daemon, while a valid
// candidate retains the no-daemon guard and neither attempt writes anything.
func TestConfirmedSessionValidatesBeforeDaemonRequirement(t *testing.T) {
	for _, at := range []string{"root", "bgp"} {
		for _, suffix := range []string{"", " force"} {
			t.Run(at+suffix, func(t *testing.T) {
				model, store, configPath := newCommitGrammarModel(t, "alice")
				prepareConfirmedPeer(t, model)
				before, err := store.ReadFile(configPath)
				if err != nil {
					t.Fatal(err)
				}
				path := []string{"bgp", "peer", "peer1", "session", "capability"}
				if err := model.editor.SetValue(path, "route-refresh", "requre"); err != nil {
					t.Fatal(err)
				}
				fix := "fix: set bgp peer peer1 session capability route-refresh require"
				if at == "bgp" {
					model.contextPath = []string{"bgp"}
					fix = "fix: top set bgp peer peer1 session capability route-refresh require"
				}
				command := "commit confirmed 60" + suffix
				result, err := model.dispatchCommand(command)
				if err != nil {
					t.Fatalf("validation must report issues before the daemon guard: %v", err)
				}
				if !strings.HasPrefix(result.statusMessage, "commit blocked: 1 error(s)") {
					t.Fatalf("status = %q", result.statusMessage)
				}
				if strings.Contains(result.statusMessage, "\n") {
					t.Fatalf("the status must hold only the header: %q", result.statusMessage)
				}
				if !strings.Contains(result.output, "route-refresh accepts only these values:") {
					t.Fatalf("the shared refusal is missing: %q", result.output)
				}
				if !strings.Contains(result.output, fix) {
					t.Fatalf("the framed fix is missing: %q", result.output)
				}
				if !model.editor.Dirty() {
					t.Fatal("refusal cleared the candidate's dirty flag")
				}
				if !strings.Contains(model.editor.WorkingContent(), "requre") {
					t.Fatal("refusal discarded the invalid candidate")
				}
				if model.confirmTimerActive {
					t.Fatal("refusal opened a confirmation timer")
				}
				if result.setConfirmTimer {
					t.Fatal("refusal requested a confirmation timer")
				}
				if err := model.editor.SetValue(path, "route-refresh", "disable"); err != nil {
					t.Fatal(err)
				}
				_, err = model.dispatchCommand(command)
				if !errors.Is(err, errCommitConfirmedNeedsDaemon) {
					t.Fatalf("valid offline session lost the daemon guard: %v", err)
				}
				after, err := store.ReadFile(configPath)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(after, before) {
					t.Fatal("refused confirmed commit changed the active configuration")
				}
				if len(model.editor.PendingChanges(model.editor.session.ID)) == 0 {
					t.Fatal("refused confirmed commit removed the pending change")
				}
			})
		}
	}
}

// TestConfirmedDaemonSessionFramesEnumRefusal exercises the actual window
// route: invalid candidates show the shared correction and open no window;
// a valid candidate then reaches the same backend and opens its real window.
func TestConfirmedDaemonSessionFramesEnumRefusal(t *testing.T) {
	for _, at := range []string{"root", "bgp peer peer1", "ospf"} {
		t.Run(at, func(t *testing.T) {
			fixture := newWindowFixture(t)
			model := fixture.model(t, "alice")
			prepareConfirmedPeer(t, model)
			if at == "ospf" {
				if err := model.editor.SetValue([]string{"ospf"}, "router-id", "1.2.3.4"); err != nil {
					t.Fatal(err)
				}
			}
			fix := "fix: set bgp peer peer1 session capability route-refresh require"
			if at != "root" {
				model.contextPath = strings.Fields(at)
				fix = "fix: top set bgp peer peer1 session capability route-refresh require"
			}
			path := []string{"bgp", "peer", "peer1", "session", "capability"}
			if err := model.editor.SetValue(path, "route-refresh", "requre"); err != nil {
				t.Fatal(err)
			}
			before := fixture.active(t)
			result, err := model.dispatchCommand("commit confirmed 60")
			if err != nil {
				t.Fatal(err)
			}
			if !strings.HasPrefix(result.statusMessage, "commit blocked: 1 error(s)") {
				t.Fatalf("status = %q", result.statusMessage)
			}
			if strings.Contains(result.statusMessage, "\n") {
				t.Fatalf("status contains more than its header: %q", result.statusMessage)
			}
			if !strings.Contains(result.output, "route-refresh accepts only these values:") {
				t.Fatalf("the shared refusal is missing: %q", result.output)
			}
			if !strings.HasSuffix(strings.TrimSpace(result.output), fix) {
				t.Fatalf("the refusal does not end with its framed fix: %q", result.output)
			}
			if _, open := fixture.window.Status(); open {
				t.Fatal("a refused candidate opened the daemon window")
			}
			if fixture.active(t) != before {
				t.Fatal("refusal changed the active configuration")
			}
			if !strings.Contains(model.editor.WorkingContent(), "requre") {
				t.Fatal("refusal discarded the pending candidate")
			}
			if err := model.editor.SetValue(path, "route-refresh", "disable"); err != nil {
				t.Fatal(err)
			}
			result, err = model.dispatchCommand("commit confirmed 60")
			if err != nil {
				t.Fatal(err)
			}
			if _, open := fixture.window.Status(); !open {
				t.Fatalf("valid candidate did not reach the daemon window: %s\n%s", result.statusMessage, result.output)
			}
		})
	}
}

// prepareConfirmedPeer supplies the endpoint and AS fields the peer resolver
// requires, so a valid capability can reach the backend after the refusal.
func prepareConfirmedPeer(t *testing.T, model *Model) {
	t.Helper()
	for _, field := range []struct {
		path  []string
		key   string
		value string
	}{
		{[]string{"bgp", "peer", "peer1", "connection", "local"}, "ip", "auto"},
		{[]string{"bgp", "peer", "peer1", "connection", "remote"}, "ip", "1.1.1.1"},
		{[]string{"bgp", "peer", "peer1", "session", "asn"}, "remote", "65001"},
	} {
		if err := model.editor.SetValue(field.path, field.key, field.value); err != nil {
			t.Fatal(err)
		}
	}
}
