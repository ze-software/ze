// Design: docs/contributing/patching-the-standard-library.md -- fresh daemon bootstrap.
package gotoolchain

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestZeLauncherBootstrapsPatchedGoWithoutLE runs the real launcher from outside
// its checkout with no bin/le. A stand-in compiler rejects an unpatched build.
func TestZeLauncherBootstrapsPatchedGoWithoutLE(t *testing.T) {
	repository, err := filepath.Abs(filepath.Join("..", "..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	checkout := t.TempDir()
	for _, name := range []string{"ze", "feature-tags", "feature-gates.txt", "go.mod"} {
		body, err := os.ReadFile(filepath.Join(repository, name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(checkout, name), body, 0o700); err != nil { //nolint:gosec // executable launcher fixture
			t.Fatal(err)
		}
	}
	tools := t.TempDir()
	tree := filepath.Join(t.TempDir(), "patched")
	if err := os.MkdirAll(filepath.Join(tree, "bin"), 0o700); err != nil {
		t.Fatal(err)
	}
	compiler := `#!/bin/sh
set -eu
test "$PWD" = "$ROOT_EXPECTED" || { echo "not in checkout" >&2; exit 1; }
test -z "${GOOS:-}${GOARCH:-}" || { echo "cross target leaked" >&2; exit 1; }
case "$1" in
run)
    test "$2" = ./internal/le/go/stdpatch/goroot
    printf '%s\n' "$TREE_EXPECTED"
    ;;
build)
    test "${GOROOT:-}" = "$TREE_EXPECTED" || { echo "unpatched build" >&2; exit 1; }
    test "${GOTOOLCHAIN:-}" = local
    while test "$1" != -o; do shift; done
    shift
    printf '#!/bin/sh\nprintf "daemon: %%s\\n" "$*"\n' > "$1"
    chmod 700 "$1"
    ;;
*) echo "unexpected compiler invocation" >&2; exit 1 ;;
esac
`
	goPath := filepath.Join(tools, "go")
	if err := os.WriteFile(goPath, []byte(compiler), 0o700); err != nil { //nolint:gosec // stand-in compiler must execute
		t.Fatal(err)
	}
	if err := os.Symlink(goPath, filepath.Join(tree, "bin", "go")); err != nil {
		t.Fatal(err)
	}
	shell, err := exec.LookPath("sh")
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command(shell, filepath.Join(checkout, "ze"), "version") //nolint:gosec,noctx // bounded stand-in compiler, no actual build
	command.Dir = t.TempDir()
	command.Env = append(os.Environ(), "PATH="+tools+string(os.PathListSeparator)+os.Getenv("PATH"),
		"ROOT_EXPECTED="+checkout, "TREE_EXPECTED="+tree, "HOME="+t.TempDir(),
		"GOOS=unwanted", "GOARCH=unwanted")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("fresh launcher failed: %v\n%s", err, output)
	}
	if !strings.Contains(string(output), "daemon: version") {
		t.Fatalf("launcher did not run the freshly built daemon: %s", output)
	}
}
