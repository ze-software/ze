// Design: docs/architecture/plugin/plugin-system.md -- compiled-in plugin bus send coverage (AC-29)
// Related: all_test.go -- composition-root invariants over registry.All
// Related: ../../../le/cli/dispatch/plugin_sends.go -- shared AST scanner backing both this test and `le cli dispatch check`

package all_test

import (
	"os"
	"path/filepath"
	"testing"

	_ "github.com/ze-software/ze/internal/component/plugin/all"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	clidispatch "github.com/ze-software/ze/internal/le/cli/dispatch"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// TestCompiledInDeclarationsCoverWhatTheySend enforces AC-29 over every
// compiled-in plugin registered in registry.All().
//
// VALIDATES: Every compiled-in plugin's Registration.Sends covers every static
// engine operation (bgp:update, bgp:refresh, bgp:raw, system-rib:route-install,
// system-rib:route-metrics, resolve:dns, cli:request bgp adj-rib-in
// batch-validate) and every static CLI command prefix (cli:<prefix>) its
// package directory (and owned subpackages) invokes, and the scanner catches
// undeclared calls on a negative synthetic fixture.
// PREVENTS: A compiled-in plugin losing a command or engine operation at
// runtime because its Registration.Sends declaration is incomplete.
func TestCompiledInDeclarationsCoverWhatTheySend(t *testing.T) {
	t.Run("compiled-in-plugins", func(t *testing.T) {
		root, err := lepath.Root()
		if err != nil {
			t.Fatalf("find checkout root: %v", err)
		}
		regs := registry.All()
		if len(regs) == 0 {
			t.Fatal("registry.All() returned no plugins, so the walk proves nothing")
		}

		findings, checked, err := clidispatch.CheckPluginSends(root, regs)
		if err != nil {
			t.Fatalf("CheckPluginSends failed: %v", err)
		}
		if checked != len(regs) {
			t.Errorf("checked %d plugins, want %d", checked, len(regs))
		}
		for _, finding := range findings {
			t.Errorf("%s:%d: %s: %s(%q) -- %s",
				finding.File, finding.Line, finding.Kind, finding.Emitter, finding.Command, finding.Detail)
		}
	})

	t.Run("negative-synthetic-undeclared-sends", func(t *testing.T) {
		tree := t.TempDir()
		writeSyntheticFile(t, tree, "go.mod", "module github.com/ze-software/ze\n")
		writeSyntheticFile(t, tree, "internal/plugins/synthetic/register.go", `package synthetic

import (
	"context"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func init() {
	_ = registry.Register(registry.Registration{Name: "synthetic"})
}

func run(ctx context.Context, p *sdk.Plugin) {
	_, _ = p.RouteInstall(ctx, nil)
	_, _, _ = p.DispatchCommand(ctx, "clear bgp peer 192.0.2.1")
}
`)

		regs := []*registry.Registration{{Name: "synthetic"}}
		findings, checked, err := clidispatch.CheckPluginSends(tree, regs)
		if err != nil {
			t.Fatalf("CheckPluginSends on synthetic tree failed: %v", err)
		}
		if checked != 1 {
			t.Errorf("checked = %d, want 1", checked)
		}

		foundInstall := false
		foundCLI := false
		for _, finding := range findings {
			if finding.Kind != clidispatch.KindUndeclaredSend {
				t.Errorf("unexpected finding kind %q in %+v", finding.Kind, finding)
			}
			switch finding.Command {
			case "system-rib:route-install":
				foundInstall = true
			case "cli:clear bgp peer":
				foundCLI = true
			}
		}
		if !foundInstall {
			t.Errorf("expected undeclared-send finding for system-rib:route-install; got %+v", findings)
		}
		if !foundCLI {
			t.Errorf("expected undeclared-send finding for cli:clear bgp peer; got %+v", findings)
		}
	})
}

func writeSyntheticFile(t *testing.T, root, rel, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatalf("mkdir %s: %v", full, err)
	}
	if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", full, err)
	}
}
