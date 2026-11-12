// VALIDATES: a prepared preview binds its exact inputs and both Linux targets.
// PREVENTS: mislabeled previews, partial publication and undetected corruption.
package buildpreview

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func put(t *testing.T, root, name, body string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestSelectedInputIdentityIncludesNewRuntimeSource(t *testing.T) {
	root := t.TempDir()
	put(t, root, "cmd/ze/main.go", "original")
	put(t, root, "cmd/ze/new_runtime.go", "untracked runtime")
	put(t, root, "private.env", "secret")
	paths := []string{"cmd/ze/new_runtime.go", "cmd/ze/main.go"}
	_, first, err := fingerprintInputs(root, paths)
	if err != nil {
		t.Fatal(err)
	}
	put(t, root, "private.env", "new secret")
	_, unchanged, err := fingerprintInputs(root, paths)
	if err != nil {
		t.Fatal(err)
	}
	if first != unchanged {
		t.Fatal("an unrelated secret entered source identity")
	}
	put(t, root, "cmd/ze/new_runtime.go", "changed runtime")
	inputs, changed, err := fingerprintInputs(root, paths)
	if err != nil {
		t.Fatal(err)
	}
	if first == changed {
		t.Fatal("new runtime input was omitted")
	}
	if len(inputs) != 2 || inputs[0].Path != "cmd/ze/main.go" {
		t.Fatalf("inputs = %#v", inputs)
	}
	if _, _, err := fingerprintInputs(root, []string{"../escape"}); err == nil {
		t.Fatal("escaping input admitted")
	}
	if err := os.Symlink(filepath.Join(root, "private.env"), filepath.Join(root, "linked.go")); err != nil {
		t.Fatal(err)
	}
	if _, _, err := fingerprintInputs(root, []string{"linked.go"}); err == nil {
		t.Fatal("symlink input admitted")
	}
}

func fixture(t *testing.T) (string, dependencies) {
	t.Helper()
	root := t.TempDir()
	put(t, root, "LICENSE", "GPL source license")
	source := Source{Commit: strings.Repeat("a", 40), Dirty: true, BuildDate: "2026-11-11T00:00:00Z", Toolchain: "go1.27.2", Tags: []string{"ze_core", "ze_distro", "ze_bgp"}, Inputs: []Input{{Path: "cmd/ze/main.go", SHA256: strings.Repeat("b", 64)}}}
	source.Digest = sourceDigest(inputDigest(source.Inputs), source.Toolchain, source.Tags)
	deps := dependencies{
		source: func(context.Context, string) (Source, error) { return source, nil },
		build: func(_ context.Context, _, arch, output, version, date string) error {
			if !strings.HasPrefix(version, "preview-") || !strings.HasSuffix(version, "-dirty") || date != source.BuildDate {
				t.Fatalf("wrong identity %s %s", version, date)
			}
			return os.WriteFile(output, []byte("binary "+arch), 0o755)
		},
		inspect: func(string, string, Source, string) error { return nil },
	}
	return root, deps
}

func TestPrepareAndCheckBindBothTargetsAndRejectTampering(t *testing.T) {
	root, deps := fixture(t)
	out := filepath.Join(t.TempDir(), "preview")
	manifest, err := prepare(context.Background(), root, out, false, deps)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Channel != "preview" || manifest.Published || len(manifest.Artifacts) != 4 {
		t.Fatalf("manifest = %#v", manifest)
	}
	if _, err := check(out, deps.inspect); err != nil {
		t.Fatal(err)
	}
	var binary string
	for _, artifact := range manifest.Artifacts {
		if artifact.Kind == "binary" && artifact.Arch == "arm64" {
			binary = artifact.File
		}
	}
	if binary == "" {
		t.Fatal("arm64 binary missing")
	}
	put(t, out, binary, "tampered")
	if _, err := check(out, deps.inspect); err == nil {
		t.Fatal("tampered binary accepted")
	}
	if _, err := prepare(context.Background(), root, out, false, deps); err == nil {
		t.Fatal("existing output overwritten")
	}
}

func TestSourceMovementRefusesACompletedBundle(t *testing.T) {
	root, deps := fixture(t)
	read := deps.source
	calls := 0
	deps.source = func(ctx context.Context, root string) (Source, error) {
		source, err := read(ctx, root)
		calls++
		if calls > 1 {
			source.Digest = strings.Repeat("c", 64)
		}
		return source, err
	}
	out := filepath.Join(t.TempDir(), "preview")
	if _, err := prepare(context.Background(), root, out, false, deps); err == nil {
		t.Fatal("moving source accepted")
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatalf("failed bundle became available: %v", err)
	}
}

func TestGeneratedOutputDoesNotInvalidateSelectedInputs(t *testing.T) {
	root, deps := fixture(t)
	deps.build = func(_ context.Context, _, arch, output, version, _ string) error {
		if strings.HasSuffix(version, "-dirty") {
			t.Fatal("clean starting source was labeled dirty")
		}
		return os.WriteFile(output, []byte("binary "+arch), 0o755)
	}
	read := deps.source
	calls := 0
	deps.source = func(ctx context.Context, root string) (Source, error) {
		source, err := read(ctx, root)
		calls++
		// An untracked output directory can change checkout dirtiness while
		// selected compiler inputs remain exactly the captured population.
		source.Dirty = calls > 1
		return source, err
	}
	manifest, err := prepare(t.Context(), root, filepath.Join(t.TempDir(), "preview"), false, deps)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Source.Dirty || strings.HasSuffix(manifest.Version, "-dirty") {
		t.Fatal("preview changed its captured starting identity")
	}
}

func TestArchivesHaveFixedHeadersAndOnlyPublicFiles(t *testing.T) {
	root, deps := fixture(t)
	first := filepath.Join(t.TempDir(), "one")
	second := filepath.Join(t.TempDir(), "two")
	a, err := prepare(context.Background(), root, first, false, deps)
	if err != nil {
		t.Fatal(err)
	}
	b, err := prepare(context.Background(), root, second, false, deps)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a, b) {
		t.Fatal("same source produced different manifests")
	}
	for _, artifact := range a.Artifacts {
		if artifact.Kind != "archive" {
			continue
		}
		x, err := os.ReadFile(filepath.Join(first, artifact.File))
		if err != nil {
			t.Fatal(err)
		}
		y, err := os.ReadFile(filepath.Join(second, artifact.File))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(x, y) {
			t.Fatal("archive contains wall-clock or destination identity")
		}
	}
}

func TestPreviewGrammarRefusesMissingOutput(t *testing.T) {
	for _, args := range [][]string{{"run"}, {"check"}, {"run", "output", "x", "publish"}} {
		if _, code := Answer(args); code == 0 {
			t.Fatalf("accepted %v", args)
		}
	}
}
