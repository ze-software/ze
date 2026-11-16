// Design: docs/architecture/testing/interop.md -- each declared lab binary carries its own build inputs.
// Related: zebuild.go -- stageBinary sends those inputs to the compiler process.
package interoplab

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// TestLabBinaryPackageAndOverlay checks the compiler request, using the existing
// process seam, so a standalone relay can carry both its package and a recorded
// source break without changing another binary's build inputs.
func TestLabBinaryPackageAndOverlay(t *testing.T) {
	const parentFlags = "-trimpath -mod=readonly"
	t.Setenv("GOFLAGS", parentFlags)
	root, err := lepath.Root()
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	ambientTarget := filepath.Join(root, "internal", "le", "interoplab", "l2tp", "avprelay", "pppinject.go")
	scaleTarget := filepath.Join(root, "internal", "component", "l2tp", "session.go")
	ambientBacking := filepath.Join(directory, "recorded-break.go")
	scaleBacking := filepath.Join(directory, "scale-candidate.go")
	ambient := writeLabTestOverlay(t, directory, "ambient.json", map[string]string{ambientTarget: ambientBacking})
	declared := writeLabTestOverlay(t, directory, "declared.json", map[string]string{scaleTarget: scaleBacking})
	environment := []string{"GOFLAGS=-trimpath -mod=vendor -overlay=" + ambient, "CGO_ENABLED=0"}
	before := append([]string(nil), environment...)
	cleanups := 0
	binary := LabBinary{Name: "avp-relay", Base: "ze_core", Output: "test/interop-l2tp/avp-relay-linux"}
	binary.Package = "./internal/le/interoplab/l2tp/avprelay"
	binary.PrepareOverlay = func(string) (string, func() error, error) {
		return declared, func() error { cleanups++; return nil }, nil
	}
	runner := newBuildRunner("arm64")
	var effective map[string]string
	runner.run = func(command ProcessCommand) (ProcessResult, error) {
		path := labTestOverlayPath(t, command)
		effective = readLabTestOverlay(t, path)
		return ProcessResult{}, nil
	}
	if err := stageBinary(context.Background(), runner, root, environment, binary); err != nil {
		t.Fatal(err)
	}
	builds := runner.builds()
	if len(builds) != 1 {
		t.Fatalf("build count = %d, want one relay build", len(builds))
	}
	command := builds[0]
	if got := command.Arguments[len(command.Arguments)-1]; got != "./internal/le/interoplab/l2tp/avprelay" {
		t.Fatalf("build package = %q, want the declared relay package", got)
	}
	for target, backing := range map[string]string{ambientTarget: ambientBacking, scaleTarget: scaleBacking} {
		if effective[target] != backing {
			t.Errorf("effective overlay[%s] = %q, want %q", target, effective[target], backing)
		}
	}
	if cleanups != 1 {
		t.Errorf("overlay cleanup count = %d, want one", cleanups)
	}
	if !reflect.DeepEqual(environment, before) {
		t.Errorf("staging changed the caller's environment: %v", environment)
	}
	if got := os.Getenv("GOFLAGS"); got != parentFlags {
		t.Errorf("staging changed process-global GOFLAGS to %q", got)
	}
	flags, held := effectiveEnvironment(command.Environment, "GOFLAGS")
	if !held {
		t.Fatal("build lost its inherited GOFLAGS")
	}
	for _, flag := range []string{"-trimpath", "-mod=vendor"} {
		if !strings.Contains(flags, flag) {
			t.Errorf("compiler GOFLAGS %q lost %s", flags, flag)
		}
	}
}

// TestLabBinaryOverlayCleanup checks a failed compiler still releases only the
// overlay prepared for that binary, through the existing process-error path.
func TestLabBinaryOverlayCleanup(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	declared := writeLabTestOverlay(t, directory, "declared.json", map[string]string{})
	cleanups := 0
	binary := LabBinary{Name: "scale", Base: "ze_core", Output: "test/interop-l2tp/ze-scale-linux"}
	binary.PrepareOverlay = func(string) (string, func() error, error) {
		return declared, func() error { cleanups++; return nil }, nil
	}
	runner := newBuildRunner("arm64")
	runner.run = func(ProcessCommand) (ProcessResult, error) {
		return ProcessResult{}, errors.New("compiler failed")
	}
	if err := stageBinary(context.Background(), runner, root, nil, binary); err == nil {
		t.Fatal("staging accepted the failed compiler")
	}
	if cleanups != 1 {
		t.Fatalf("failed-build cleanup count = %d, want one", cleanups)
	}
}

// TestLabBinaryOverlayConflict prevents a scale overlay from replacing a
// recorder's different edit of the same source file before compilation.
func TestLabBinaryOverlayConflict(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	target := filepath.Join(root, "internal", "component", "l2tp", "session.go")
	ambient := writeLabTestOverlay(t, directory, "ambient.json", map[string]string{target: filepath.Join(directory, "recorded.go")})
	// Go resolves this relative source against the compiler directory. It is
	// the same file the inherited overlay names with an absolute path.
	declared := writeLabTestOverlay(t, directory, "declared.json", map[string]string{"internal/component/l2tp/session.go": filepath.Join(directory, "scale.go")})
	cleanups := 0
	binary := LabBinary{Name: "scale", Base: "ze_core", Output: "test/interop-l2tp/ze-scale-linux"}
	binary.PrepareOverlay = func(string) (string, func() error, error) {
		return declared, func() error { cleanups++; return nil }, nil
	}
	runner := newBuildRunner("arm64")
	err = stageBinary(context.Background(), runner, root, []string{"GOFLAGS=-overlay=" + ambient}, binary)
	if err == nil {
		t.Fatal("staging silently replaced the recorder's source break")
	}
	if !strings.Contains(err.Error(), target) {
		t.Errorf("overlay refusal does not identify the conflicting source: %v", err)
	}
	if len(runner.builds()) != 0 {
		t.Error("compiler ran despite an unresolved overlay conflict")
	}
	if cleanups != 1 {
		t.Errorf("conflict cleanup count = %d, want one", cleanups)
	}
}

// TestLabBinaryDefaultInputs checks the extension leaves a normal daemon build
// on its original package and passes its inherited recorder overlay unchanged.
func TestLabBinaryDefaultInputs(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatal(err)
	}
	binary := LabBinary{Name: "ze", Base: "ze_core", Output: "test/interop-l2tp/ze-linux"}
	runner := newBuildRunner("arm64")
	environment := []string{"GOFLAGS=-trimpath -overlay=/recorded/overlay.json"}
	if err := stageBinary(context.Background(), runner, root, environment, binary); err != nil {
		t.Fatal(err)
	}
	builds := runner.builds()
	if len(builds) != 1 {
		t.Fatalf("build count = %d, want one", len(builds))
	}
	command := builds[0]
	if got := command.Arguments[len(command.Arguments)-1]; got != labBuildPackage {
		t.Errorf("default build package = %q, want %q", got, labBuildPackage)
	}
	if !reflect.DeepEqual(command.Environment, environment) {
		t.Errorf("default build changed GOFLAGS: %v", command.Environment)
	}
}

// TestLabBinaryOverlayPreparationFailure checks a callback that reports a
// partially prepared overlay can release it before staging returns its error.
func TestLabBinaryOverlayPreparationFailure(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatal(err)
	}
	cleanups := 0
	binary := LabBinary{Name: "scale", Base: "ze_core", Output: "test/interop-l2tp/ze-scale-linux"}
	binary.PrepareOverlay = func(string) (string, func() error, error) {
		return "", func() error { cleanups++; return nil }, errors.New("source shape changed")
	}
	runner := newBuildRunner("arm64")
	err = stageBinary(context.Background(), runner, root, nil, binary)
	if err == nil {
		t.Fatal("staging accepted failed overlay preparation")
	}
	if !strings.Contains(err.Error(), "source shape changed") {
		t.Errorf("preparation error lost its cause: %v", err)
	}
	if cleanups != 1 {
		t.Errorf("preparation-failure cleanup count = %d, want one", cleanups)
	}
	if len(runner.builds()) != 0 {
		t.Error("compiler ran after overlay preparation failed")
	}
}

// TestLabBinaryOverlayCleanupFailure prevents a successful compiler from hiding
// an overlay artifact whose owner could not remove it.
func TestLabBinaryOverlayCleanupFailure(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	declared := writeLabTestOverlay(t, directory, "declared.json", map[string]string{})
	binary := LabBinary{Name: "scale", Base: "ze_core", Output: "test/interop-l2tp/ze-scale-linux"}
	binary.PrepareOverlay = func(string) (string, func() error, error) {
		return declared, func() error { return errors.New("overlay remains") }, nil
	}
	err = stageBinary(context.Background(), newBuildRunner("arm64"), root, nil, binary)
	if err == nil {
		t.Fatal("staging hid the overlay cleanup failure")
	}
	if !strings.Contains(err.Error(), "overlay remains") {
		t.Errorf("cleanup failure lost its cause: %v", err)
	}
}

// TestLabBinaryOverlayDeclarationRefused checks the staging boundary rejects a
// missing overlay or owner cleanup before it starts a compiler process.
func TestLabBinaryOverlayDeclarationRefused(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	declared := writeLabTestOverlay(t, directory, "declared.json", map[string]string{})
	for _, test := range []struct {
		name    string
		path    string
		cleanup func() error
	}{
		{name: "no path", cleanup: func() error { return nil }},
		{name: "no cleanup", path: declared},
	} {
		t.Run(test.name, func(t *testing.T) {
			binary := LabBinary{Name: "scale", Base: "ze_core", Output: "test/interop-l2tp/ze-scale-linux"}
			binary.PrepareOverlay = func(string) (string, func() error, error) {
				return test.path, test.cleanup, nil
			}
			runner := newBuildRunner("arm64")
			if err := stageBinary(context.Background(), runner, root, nil, binary); err == nil {
				t.Fatal("staging accepted an incomplete overlay declaration")
			}
			if len(runner.builds()) != 0 {
				t.Error("compiler ran despite an incomplete overlay declaration")
			}
		})
	}
}

func writeLabTestOverlay(t *testing.T, directory, name string, replacements map[string]string) string {
	t.Helper()
	body, err := json.Marshal(struct {
		Replace map[string]string
	}{Replace: replacements})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, name)
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func readLabTestOverlay(t *testing.T, path string) map[string]string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var overlay struct {
		Replace map[string]string
	}
	if err := json.Unmarshal(body, &overlay); err != nil {
		t.Fatal(err)
	}
	return overlay.Replace
}

func labTestOverlayPath(t *testing.T, command ProcessCommand) string {
	t.Helper()
	for index, argument := range command.Arguments {
		if path, held := strings.CutPrefix(argument, "-overlay="); held {
			return path
		}
		if argument == "-overlay" {
			if index+1 < len(command.Arguments) {
				return command.Arguments[index+1]
			}
		}
	}
	flags, held := effectiveEnvironment(command.Environment, "GOFLAGS")
	if held {
		for argument := range strings.FieldsSeq(flags) {
			if path, found := strings.CutPrefix(argument, "-overlay="); found {
				return path
			}
		}
	}
	t.Fatal("compiler command carries no effective overlay")
	return ""
}
