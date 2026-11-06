// Design: docs/architecture/plugin/feature-gates.md -- coupling shares committed metadata and source.
package coupling

import (
	"os/exec"
	"slices"
	"testing"
)

func couplingGit(t *testing.T, root string, args ...string) {
	t.Helper()
	command := exec.CommandContext(t.Context(), "git", append([]string{"-C", root}, args...)...)
	if raw, err := command.CombinedOutput(); err != nil {
		t.Fatalf("fixture git %v: %v: %s", args, err, raw)
	}
}

// TestCouplingUsesOneCommittedMembershipManifestAndSource drives the real Check
// over a committed cross-gate dependency, then dirties each producer input in a
// way that would hide the coupling if any consumer reopened working-tree bytes.
func TestCouplingUsesOneCommittedMembershipManifestAndSource(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"go.mod": "module github.com/ze-software/ze\n\ngo 1.26\n",
		"feature-gates.txt": "ze_a internal/plugins/a\nze_b internal/plugins/b\n",
		"cmd/ze/main.go": "package main\nimport (_ \"github.com/ze-software/ze/internal/plugins/a\"; _ \"github.com/ze-software/ze/internal/plugins/b\")\nfunc main() {}\n",
		"internal/component/plugin/registry/registry.go": "package registry\ntype Registration struct { Name, Description string; RunEngine func(); StartAfter []string }\nfunc Register(Registration) error { return nil }\n",
		"internal/core/events/events.go": "package events\ntype Handle struct{}\nfunc Register[T any](string,string) Handle { return Handle{} }\nfunc (Handle) Emit(any,any) {}\nfunc (Handle) Subscribe(any,any) {}\n",
		"internal/core/ev/ev.go": "package ev\nimport \"github.com/ze-software/ze/internal/core/events\"\nvar Thing = events.Register[*int](\"ns\", \"thing\")\n",
		"internal/plugins/a/register.go": "package a\nimport (r \"github.com/ze-software/ze/internal/component/plugin/registry\"; renamed \"github.com/ze-software/ze/internal/core/ev\")\nfunc init() { _ = r.Register(r.Registration{Name: \"a\", Description: \"consumer\", RunEngine: func(){}}) }\nfunc listen() { renamed.Thing.Subscribe(nil,nil) }\n",
		"internal/plugins/b/register.go": "package b\nimport (r \"github.com/ze-software/ze/internal/component/plugin/registry\"; \"github.com/ze-software/ze/internal/core/ev\")\nfunc init() { _ = r.Register(r.Registration{Name: \"b\", Description: \"provider\", RunEngine: func(){}}) }\nfunc emit() { ev.Thing.Emit(nil,nil) }\n",
	}
	for name, source := range files {
		writeFile(t, root, name, source)
	}
	couplingGit(t, root, "init", "--initial-branch=main")
	couplingGit(t, root, "add", ".")
	couplingGit(t, root, "-c", "user.name=Coupling Fixture", "-c", "user.email=coupling@example.invalid", "commit", "-m", "committed coupling")
	writeFile(t, root, "feature-gates.txt", "ze_same internal/plugins/a\nze_same internal/plugins/b\n")
	writeFile(t, root, "internal/plugins/b/register.go", "package b\n")
	report, err := Check(root)
	if err != nil {
		t.Fatal(err)
	}
	if report.Plugins != 2 || len(report.Couplings) != 1 {
		t.Fatalf("committed coupling population changed with dirty inputs: %+v", report)
	}
	coupling := report.Couplings[0]
	if coupling.Consumer != "a" || coupling.Kind != KindEvent || !slices.Equal(coupling.Providers, []string{"b"}) {
		t.Fatalf("committed coupling identity changed: %+v", coupling)
	}
	if !slices.Equal(coupling.ConsumerTags, []string{"ze_a"}) || !slices.Equal(coupling.ProviderTags, []string{"ze_b"}) {
		t.Fatalf("committed coupling gates changed: %+v", coupling)
	}
}
