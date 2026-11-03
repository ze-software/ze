// Design: docs/architecture/testing/runner-architecture.md -- native editor proof artifacts.

package editorcase

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
)

// admissionExecutable reuses an explicitly supplied artifact. Standalone package
// runs build one process-owned named artifact once, as the other native proofs do.
func admissionExecutable(t *testing.T) string {
	t.Helper()
	if *nativeExecutable != "" {
		return *nativeExecutable
	}
	path, err := buildAdmissionExecutable()
	if err != nil {
		t.Fatal(err)
	}
	return path
}

var buildAdmissionExecutable = sync.OnceValues(func() (string, error) {
	root, err := filepath.Abs("../../..")
	if err != nil {
		return "", err
	}
	name := "editor-admission-proof-" + strconv.Itoa(os.Getpid())
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, filepath.Join(root, "le"), "--name", name, "help")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	var output bytes.Buffer
	cmd.Stdout, cmd.Stderr = &output, &output
	plugin.KillGroupOnCancel(cmd)
	if err := plugin.StartChild(cmd); err != nil {
		return "", err
	}
	if err := cmd.Wait(); err != nil {
		return "", fmt.Errorf("build native admission artifact: %w\n%s", err, output.Bytes())
	}
	return filepath.Join(root, "bin", "le-"+name, "le"), nil
})
