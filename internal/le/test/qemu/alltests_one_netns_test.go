// Design: docs/architecture/testing/qemu-integration.md -- one-CI namespace preparation.

package testqemu

import (
	"errors"
	"slices"
	"testing"
)

// TestOnePluginSelectionUsesCanonicalNamespaceMetadata reaches the one-CI
// consumer with real resource declarations and opaque embedded text. A link
// requires namespace preparation, while BPF alone keeps the guest namespace.
func TestOnePluginSelectionUsesCanonicalNamespaceMetadata(t *testing.T) {
	for _, test := range []struct {
		name       string
		body       string
		namespace  string
		commands   int
		skipRouted bool
	}{
		{
			name:      "declared link",
			body:      "option=needs-linux:caps=net-admin\noption=netns-link:name=eth1:address=10.0.0.254/24\n",
			namespace: "per-test", commands: 2,
		},
		{
			name:      "embedded literal is not a declaration",
			body:      "tmpfs=example.txt:terminator=END_EXAMPLE\noption=netns-link:name=eth1\nEND_EXAMPLE\n",
			namespace: "guest-root", commands: 1,
		},
		{
			name:      "BPF without a link",
			body:      "option=needs-linux:caps=net-admin,bpf\n",
			namespace: "guest-root", commands: 1,
		},
		{
			name:      "unrelated routed suites skipped",
			body:      "option=needs-linux:caps=net-admin\noption=netns-link:name=eth1\n",
			namespace: "per-test", commands: 2, skipRouted: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			run := vmFixture(t)
			run.Test = "test/plugin/selected-resource.ci"
			writeFile(t, run.Workspace, run.Test, test.body+"cmd=foreground:seq=1:exec=ze doctor\n")
			if test.skipRouted {
				run.Skip = append(run.Skip, perTestSuites...)
			}
			rec := &recorder{}
			run.Run = rec.run
			report, code := run.Execute()
			if code != 0 {
				t.Fatalf("selected CI failed before its runner: code=%d phases=%v", code, report.Phases)
			}
			if len(rec.calls) != test.commands {
				t.Fatalf("got %d commands, want %d including namespace preparation", len(rec.calls), test.commands)
			}
			if len(report.Phases) != 1 {
				t.Fatalf("got %d phases, want the selected suite alone", len(report.Phases))
			}
			if report.Phases[0].Namespace != test.namespace {
				t.Fatalf("selected CI namespace = %q, want %q", report.Phases[0].Namespace, test.namespace)
			}
			child := rec.envs[len(rec.envs)-1]
			mode := slices.Contains(child, netnsModeKey+"="+netnsModeValue)
			if mode != (test.namespace == "per-test") {
				t.Fatalf("namespace mode in child = %v for %q", mode, test.namespace)
			}
		})
	}
}

// TestOnePluginSelectionRefusesUnreadableResourceMetadata rejects a missing or
// malformed selected CI before any child can run with guessed resource needs.
func TestOnePluginSelectionRefusesUnreadableResourceMetadata(t *testing.T) {
	for _, test := range []struct {
		name string
		body string
	}{
		{"missing file", ""},
		{"malformed declaration", "option=netns-link:name=\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			run := vmFixture(t)
			run.Test = "test/plugin/selected-resource.ci"
			if test.body != "" {
				writeFile(t, run.Workspace, run.Test, test.body)
			}
			rec := &recorder{}
			run.Run = rec.run
			if report, code := run.Execute(); code == 0 {
				t.Fatalf("unreadable resource metadata was accepted: %+v", report)
			}
			if len(rec.calls) != 0 {
				t.Fatalf("%d children ran before the resource metadata refusal", len(rec.calls))
			}
		})
	}
}

// TestOnePluginSelectionVerifiesItsOwnNamespaceTools refuses a link selection
// without setcap even when every unrelated routed suite has been skipped.
func TestOnePluginSelectionVerifiesItsOwnNamespaceTools(t *testing.T) {
	run := vmFixture(t)
	run.Test = "test/plugin/selected-resource.ci"
	writeFile(t, run.Workspace, run.Test, "option=netns-link:name=eth1\ncmd=foreground:seq=1:exec=ze doctor\n")
	run.Skip = append(run.Skip, perTestSuites...)
	look := run.Look
	queried := false
	run.Look = func(name string) (string, error) {
		if name == setcapCommand {
			queried = true
			return "", errors.New("setcap is absent")
		}
		return look(name)
	}
	rec := &recorder{}
	run.Run = rec.run
	if report, code := run.Execute(); code == 0 {
		t.Fatalf("selected namespace ran without setcap: %+v", report)
	}
	if !queried {
		t.Fatal("the selected namespace's setcap prerequisite was never checked")
	}
	if len(rec.calls) != 0 {
		t.Fatalf("%d children ran before the selected namespace tool refusal", len(rec.calls))
	}
}
