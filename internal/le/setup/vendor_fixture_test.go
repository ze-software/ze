// Design: docs/contributing/patching-the-standard-library.md -- legacy command contracts retain staged-target safety.
package setup

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// vendorFixtureShell simulates only validated staged outputs; the independent
// real-Go preservation test carries the actual filesystem safety proof.
func vendorFixtureShell(t *testing.T, root string, rec *recorder) *Shell {
	t.Helper()
	recording := &vendorFixtureRecording{root: root}
	shell := rec.shell()
	shell.Exec = func(_ context.Context, cmd Cmd) Result {
		raw := slices.Clone(cmd.Argv)
		rec.calls = append(rec.calls, raw)
		normalized, err := recording.command(cmd.Argv)
		if err != nil {
			return Result{Argv: raw, Code: 1, Err: err.Error()}
		}
		// Normalize unique routing tokens only after validating the complete
		// command shape and the exact producer-selected output identity.
		rec.calls[len(rec.calls)-1] = normalized
		answer := rec.answers[strings.Join(normalized, " ")]
		if !answer.OK() {
			return answer
		}
		if slices.Equal(normalized, []string{"go", "mod", "vendor"}) {
			target := cmd.Argv[4]
			if err := os.MkdirAll(target, 0o750); err != nil {
				t.Fatal(err)
			}
			modules, err := os.ReadFile(filepath.Join(root, "vendor", "modules.txt"))
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(target, "modules.txt"), modules, 0o600); err != nil {
				t.Fatal(err)
			}
			recording.stage = filepath.Base(filepath.Dir(target))
		}
		return Result{Argv: raw}
	}
	return shell
}

type vendorFixtureRecording struct {
	root  string
	stage string
}

// command rejects unsafe routes before producing the existing canonical argv.
func (r *vendorFixtureRecording) command(argv []string) ([]string, error) {
	normalized := slices.Clone(argv)
	if len(argv) >= 3 && slices.Equal(argv[:3], []string{"go", "mod", "vendor"}) {
		if len(argv) != 5 {
			return nil, fmt.Errorf("unsafe vendor invocation: want explicit owned -o target: %v", argv)
		}
		if argv[3] != "-o" {
			return nil, fmt.Errorf("unsafe vendor output flag: %v", argv)
		}
		target := argv[4]
		stage := filepath.Dir(target)
		if filepath.Clean(target) != target {
			return nil, fmt.Errorf("unclean vendor output: %s", target)
		}
		if filepath.Dir(stage) != r.root {
			return nil, fmt.Errorf("vendor output outside owned root: %s", target)
		}
		if !strings.HasPrefix(filepath.Base(stage), ".le-vendor-") {
			return nil, fmt.Errorf("vendor output outside owned staging directory: %s", target)
		}
		if filepath.Base(target) != "vendor" {
			return nil, fmt.Errorf("wrong staged vendor directory: %s", target)
		}
		if r.stage != "" {
			return nil, fmt.Errorf("duplicate staged vendor operation: %v", argv)
		}
		return normalized[:3], nil
	}
	if len(argv) >= 2 && slices.Equal(argv[:2], []string{"git", "apply"}) {
		if len(argv) < 5 {
			return nil, fmt.Errorf("unsafe apply invocation: want explicit staged --directory: %v", argv)
		}
		if argv[2] != "--directory" {
			return nil, fmt.Errorf("unsafe apply directory flag: %v", argv)
		}
		if r.stage == "" {
			return nil, fmt.Errorf("apply before staged vendor output: %v", argv)
		}
		if argv[3] != r.stage {
			return nil, fmt.Errorf("apply targets another staging directory: got %s want %s", argv[3], r.stage)
		}
		return append(normalized[:2], normalized[4:]...), nil
	}
	return normalized, nil
}

// TestVendorFixtureRecordingRejectsUnsafeTargets independently discriminates
// the adapter: old bare commands and a lookalike-but-different owned stage fail.
func TestVendorFixtureRecordingRejectsUnsafeTargets(t *testing.T) {
	for _, name := range []string{"bare-vendor", "malformed-vendor", "bare-apply", "wrong-stage"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			writeVendorPatchSet(t, root, nil)
			rec := rootShell("go", "git")
			shell := vendorFixtureShell(t, root, rec)
			stage := filepath.Join(root, ".le-vendor-good", "vendor")
			if name == "bare-apply" || name == "wrong-stage" {
				result := shell.Run(Cmd{Argv: []string{"go", "mod", "vendor", "-o", stage}, Dir: root})
				if !result.OK() {
					t.Fatalf("valid staged vendor refused: %s", result.Err)
				}
			}
			argv := []string{"go", "mod", "vendor"}
			if name == "malformed-vendor" {
				argv = []string{"go", "mod", "vendor", "-o", stage, "extra"}
			}
			if name == "bare-apply" {
				argv = []string{"git", "apply", "--check", "owned.patch"}
			}
			if name == "wrong-stage" {
				argv = []string{"git", "apply", "--directory", ".le-vendor-other", "--check", "owned.patch"}
			}
			result := shell.Run(Cmd{Argv: argv, Dir: root})
			if result.OK() {
				t.Fatalf("unsafe native routing accepted: %v", argv)
			}
			if result.Err == "" {
				t.Fatalf("unsafe native routing has no diagnostic: %v", argv)
			}
		})
	}
}
