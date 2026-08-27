package application

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/terminaldemo"
)

const terminalDemoParityRelease = "26.08.27"

// VALIDATES: spec-le-is-a-ze-binary AC-11 -- the Python producer and Go port publish byte-identical terminal artifacts and check output from one fixture tree.
// PREVENTS: a structurally valid Go manifest whose key order, cast timing, digest input, output, or exit code differs from the producer.
func TestTerminalDemoRendererAndPortAgreeWithoutLiveArtifacts(t *testing.T) {
	fixture := terminalDemoParityFixture(t)
	pythonArtifacts := filepath.Join(fixture, "python-artifacts")
	goArtifacts := filepath.Join(fixture, "go-artifacts")
	pythonOutput, pythonCode := terminalDemoRunPython(t, fixture, pythonArtifacts, "--all", "--release", terminalDemoParityRelease)
	if pythonCode != 0 {
		t.Fatalf("Python render code = %d, output:\n%s", pythonCode, pythonOutput)
	}

	var goOutput bytes.Buffer
	goEngine := terminaldemo.New(terminaldemo.Options{
		Root:         filepath.Join(fixture, "main"),
		ArtifactRoot: goArtifacts,
		Executor:     terminalDemoGoExecutor(goArtifacts),
		Output:       &goOutput,
	})
	if _, err := goEngine.RenderAll(terminalDemoParityRelease); err != nil {
		t.Fatalf("Go render: %v", err)
	}
	if pythonOutput != goOutput.String() {
		t.Errorf("render output differs\nPython: %q\nGo:     %q", pythonOutput, goOutput.String())
	}
	for _, name := range []string{"manifest.json", "term.cast", "term.txt"} {
		pythonBytes := terminalDemoRead(t, filepath.Join(pythonArtifacts, name))
		goBytes := terminalDemoRead(t, filepath.Join(goArtifacts, name))
		if !bytes.Equal(pythonBytes, goBytes) {
			t.Errorf("%s differs\nPython:\n%s\nGo:\n%s", name, pythonBytes, goBytes)
		}
	}

	pythonCheck, pythonCheckCode := terminalDemoRunPython(t, fixture, pythonArtifacts, "--all", "--release", terminalDemoParityRelease, "--check")
	var goCheck bytes.Buffer
	checkEngine := terminaldemo.New(terminaldemo.Options{
		Root:         filepath.Join(fixture, "main"),
		ArtifactRoot: pythonArtifacts,
		Output:       &goCheck,
	})
	_, goErr := checkEngine.CheckAll(terminalDemoParityRelease)
	goCode := 0
	if goErr != nil {
		goCode = 1
	}
	if pythonCheckCode != goCode {
		t.Errorf("check code: Python=%d Go=%d Go error=%v", pythonCheckCode, goCode, goErr)
	}
	if pythonCheck != goCheck.String() {
		t.Errorf("check output differs: Python=%q Go=%q", pythonCheck, goCheck.String())
	}
}

// VALIDATES: the two implementations reject release drift with the same code and leave artifact bytes unchanged.
// PREVENTS: parity obtained by rewriting a fixture manifest during a read-only check.
func TestTerminalDemoReleaseDriftParityIsReadOnly(t *testing.T) {
	fixture := terminalDemoParityFixture(t)
	artifacts := filepath.Join(fixture, "artifacts")
	if output, code := terminalDemoRunPython(t, fixture, artifacts, "--all", "--release", terminalDemoParityRelease); code != 0 {
		t.Fatalf("fixture render code = %d, output:\n%s", code, output)
	}
	before := terminalDemoTreeBytes(t, artifacts)
	pythonOutput, pythonCode := terminalDemoRunPython(t, fixture, artifacts, "--all", "--release", "different", "--check")
	engine := terminaldemo.New(terminaldemo.Options{Root: filepath.Join(fixture, "main"), ArtifactRoot: artifacts})
	_, goErr := engine.CheckAll("different")
	goCode := 0
	if goErr != nil {
		goCode = 1
	}
	if pythonCode != goCode || goCode != 1 {
		t.Errorf("release drift codes: Python=%d Go=%d error=%v", pythonCode, goCode, goErr)
	}
	goOutput := ""
	if goErr != nil {
		goOutput = "error: " + goErr.Error() + "\n"
	}
	if pythonOutput != goOutput {
		t.Errorf("release drift output: Python=%q Go=%q", pythonOutput, goOutput)
	}
	after := terminalDemoTreeBytes(t, artifacts)
	if !reflect.DeepEqual(before, after) {
		t.Error("a release check modified fixture artifacts")
	}
}

func terminalDemoParityFixture(t *testing.T) string {
	t.Helper()
	fixture := t.TempDir()
	root := filepath.Join(fixture, "main")
	demoRoot := filepath.Join(root, "demos", "terminal")
	terminalDemoMkdir(t, filepath.Join(root, "docs", "guide"))
	terminalDemoWrite(t, filepath.Join(root, "docs", "guide", "gallery.md"), []byte("gallery\n"), 0o644)
	terminalDemoWrite(t, filepath.Join(root, "docs", "guide", "demo.md"), []byte("demo\n"), 0o644)
	for _, name := range []string{"render.py", "screen.py"} {
		source := filepath.Join("..", "..", "..", "demos", "terminal", name)
		terminalDemoWrite(t, filepath.Join(demoRoot, name), terminalDemoRead(t, source), 0o755)
	}
	for _, name := range []string{"common.tape", "cards.sh", "Dockerfile", "container-entrypoint.sh", "demo-lock.sh", "validate-common.sh", "pty-session.py"} {
		terminalDemoWrite(t, filepath.Join(demoRoot, name), []byte(name+"\n"), 0o755)
	}
	terminalDemoWrite(t, filepath.Join(demoRoot, "term", "demo.tape"), []byte("Source common.tape\nSleep 5s\nType show term\n"), 0o644)
	terminalDemoWrite(t, filepath.Join(demoRoot, "term", "transcript.txt"), []byte("Terminal session\n\n$ show term\n"), 0o644)
	terminalDemoWrite(t, filepath.Join(demoRoot, "term", "validate.sh"), []byte("validate\n"), 0o755)
	manifest := `{
  "schema": 2,
  "renderer": {
    "name": "fixture",
    "version": "3",
    "image": "fixture-renderer:3",
    "platform": "linux/native"
  },
  "gallery-page": "guide/gallery.md",
  "demos": [
    {
      "id": "term",
      "title": "Terminal",
      "description": "Terminal demo",
      "page": "guide/demo.md",
      "anchor": "terminal",
      "platform": "portable",
      "kind": "terminal",
      "engine": "Ze recorder",
      "source": "term/demo.tape",
      "validate": "term/validate.sh"
    }
  ]
}
`
	terminalDemoWrite(t, filepath.Join(demoRoot, "manifest.json"), []byte(manifest), 0o644)
	terminalDemoWrite(t, filepath.Join(root, "tmp", "terminal-demos", "bin", "ze"), []byte("fixture-binary\n"), 0o755)

	fakeBin := filepath.Join(fixture, "fake-bin")
	terminalDemoMkdir(t, fakeBin)
	fakeDocker := `#!/bin/sh
case "$*" in
  *validate.sh*) exit 0 ;;
esac
mkdir -p "$TERMINAL_DEMO_PARITY_ARTIFACTS"
cat > "$TERMINAL_DEMO_PARITY_ARTIFACTS/term.cast" <<'EOF'
{"version": 2, "width": 80, "height": 24}
[0.1, "o", "$ show term\r\n"]
EOF
`
	terminalDemoWrite(t, filepath.Join(fakeBin, "docker"), []byte(fakeDocker), 0o755)
	return fixture
}

func terminalDemoGoExecutor(artifacts string) terminaldemo.Executor {
	return func(command terminaldemo.Command) int {
		entry := command.Args[len(command.Args)-1]
		if strings.Contains(entry, "validate.sh") {
			return 0
		}
		content := "{\"version\": 2, \"width\": 80, \"height\": 24}\n[0.1, \"o\", \"$ show term\\r\\n\"]\n"
		if err := os.WriteFile(filepath.Join(artifacts, "term.cast"), []byte(content), 0o644); err != nil {
			return 1
		}
		return 0
	}
}

func terminalDemoRunPython(t *testing.T, fixture, artifacts string, args ...string) (string, int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	script := filepath.Join(fixture, "main", "demos", "terminal", "render.py")
	command := exec.CommandContext(ctx, "python3", append([]string{script}, args...)...)
	fakeBin := filepath.Join(fixture, "fake-bin")
	command.Env = append(os.Environ(),
		"PATH="+fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"),
		"ZE_TERMINAL_DEMO_OUTPUT="+artifacts,
		"TERMINAL_DEMO_PARITY_ARTIFACTS="+artifacts,
	)
	var output bytes.Buffer
	command.Stdout = &output
	command.Stderr = &output
	err := command.Run()
	if err == nil {
		return output.String(), 0
	}
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		return output.String(), exitErr.ExitCode()
	}
	t.Fatalf("run Python renderer: %v", err)
	return "", 1
}

func terminalDemoTreeBytes(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(relative)] = string(terminalDemoRead(t, path))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func terminalDemoRead(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func terminalDemoWrite(t *testing.T, path string, content []byte, mode os.FileMode) {
	t.Helper()
	terminalDemoMkdir(t, filepath.Dir(path))
	if err := os.WriteFile(path, content, mode); err != nil {
		t.Fatal(err)
	}
}

func terminalDemoMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}
