// Design: docs/contributing/running-commands.md -- native lint cancellation reaches admission before any shared verdict is returned.
package golint

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/le/go/stdpatch/gen"
	"github.com/ze-software/ze/internal/le/job"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// TestLintAdmissionSignalHelper re-executes the real registered area Answer
// after activating the actual checkout's toolchain. Only the data repository
// and admission registry are the owned fixture; no Go source tree is copied.
func TestLintAdmissionSignalHelper(t *testing.T) {
	var arguments []string
	for index, argument := range os.Args {
		if argument == "--lint-admission-helper" {
			arguments = os.Args[index+1:]
			break
		}
	}
	if len(arguments) == 0 {
		return
	}
	if len(arguments) != 2 {
		t.Fatal("native lint helper needs actual checkout and owned fixture roots")
	}
	if _, err := gen.Activate(arguments[0]); err != nil {
		t.Fatalf("activate real checkout toolchain: %v", err)
	}
	t.Setenv("ZE_REPO_ROOT", arguments[1])
	env.ResetCache()
	_, code := Answer([]string{"run", "scope", "./live"})
	if code == 0 {
		t.Fatal("canceled native lint returned a successful verdict")
	}
	fmt.Println("LINT-NATIVE-CANCELED", code)
}

// lintQueueOutput observes the real admission banner before the parent sends
// SIGINT. Its mutex protects concurrent child output and parent rendezvous.
type lintQueueOutput struct {
	mu     sync.Mutex
	body   bytes.Buffer
	queued chan struct{}
	once   sync.Once
}

func (output *lintQueueOutput) Write(body []byte) (int, error) {
	output.mu.Lock()
	defer output.mu.Unlock()
	written, err := output.body.Write(body)
	if strings.Contains(output.body.String(), "[lint] waiting:") {
		output.once.Do(func() { close(output.queued) })
	}
	return written, err
}

func (output *lintQueueOutput) text() string {
	output.mu.Lock()
	defer output.mu.Unlock()
	return output.body.String()
}

// TestLintRegisteredQueueStopsOnInterrupt holds the only real admission slot,
// waits for the registered lint action's queue banner, then interrupts only its
// owned child. Cancellation must return before the foreign holder is released,
// without changing either of that holder's evidence files.
func TestLintRegisteredQueueStopsOnInterrupt(t *testing.T) {
	if _, err := exec.LookPath(lintProgram); err != nil {
		t.Fatalf("native lint prerequisite is unavailable: %v", err)
	}
	checkout, err := lepath.Root()
	if err != nil {
		t.Fatal(err)
	}
	pin, err := gen.Pin(checkout)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	writeLintFile(t, root, "go.mod", "module lintfixture\n\ngo "+strings.TrimPrefix(pin, "go")+"\n")
	writeLintFile(t, root, "feature-gates.txt", "ze_a internal/a\n")
	writeLintFile(t, root, configName, fixtureConfig)
	writeLintFile(t, root, "live/live.go", "package live\nconst Value = 7\n")
	writeLintFile(t, root, ".gitignore", "/tmp/\n/cache/\n")
	for _, arguments := range [][]string{
		{"init", "--quiet"},
		{"add", "go.mod", "feature-gates.txt", configName, "live/live.go", ".gitignore"},
		{"-c", "user.name=Lint Fixture", "-c", "user.email=lint-fixture@example.invalid", "-c", "commit.gpgsign=false", "-c", "core.hooksPath=/dev/null", "commit", "--quiet", "-m", "owned lint admission fixture"},
	} {
		cmd := exec.CommandContext(t.Context(), "git", arguments...)
		cmd.Dir = root
		cmd.Env = env.Without(os.Environ(), "GIT_DIR", "GIT_WORK_TREE", "GIT_COMMON_DIR", "GIT_INDEX_FILE", "GIT_OBJECT_DIRECTORY", "GIT_ALTERNATE_OBJECT_DIRECTORIES")
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("prepare owned admission fixture: %v\n%s", err, output)
		}
	}
	// Detach every spelling of the wrapping parent's slot, not merely one alias.
	for _, pair := range os.Environ() {
		if len(env.Without([]string{pair}, job.ParentKey)) == 0 {
			name, _, _ := strings.Cut(pair, "=")
			t.Setenv(name, "")
		}
	}
	env.ResetCache()
	t.Cleanup(env.ResetCache)
	admission, err := job.NewIn(root)
	if err != nil {
		t.Fatal(err)
	}
	admission.Slots = 1
	admission.MayAttach = false
	admission.Out, admission.Err = io.Discard, io.Discard
	holder, err := admission.Admit("foreign-fixture", []string{"owned-foreign-holder"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { holder.Release(0) })
	if holder.Kind != job.KindClaimed {
		t.Fatalf("foreign fixture did not claim the sole slot: %v", holder.Kind)
	}
	entry := filepath.Join(root, filepath.FromSlash(holder.Entry))
	log := filepath.Join(root, filepath.FromSlash(holder.Log))
	const sentinel = "foreign holder evidence must survive interrupt\n"
	if err := os.WriteFile(log, []byte(sentinel), 0o600); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(entry)
	if err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(executable, "-test.run=^TestLintAdmissionSignalHelper$", "--", "--lint-admission-helper", checkout, root)
	cmd.Env = env.Without(os.Environ(), job.ParentKey, job.SlotsKey, lepath.RootKey, "GIT_DIR", "GIT_WORK_TREE", "GIT_COMMON_DIR", "GIT_INDEX_FILE", "GIT_OBJECT_DIRECTORY", "GIT_ALTERNATE_OBJECT_DIRECTORIES")
	cmd.Env = append(cmd.Env, "ZE_RUN_SLOTS=1", "GOPROXY=off", "GOWORK=off", "GOFLAGS=-mod=mod")
	output := &lintQueueOutput{queued: make(chan struct{})}
	cmd.Stdout, cmd.Stderr = output, output
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	finished := false
	t.Cleanup(func() {
		if !finished {
			if err := cmd.Process.Kill(); err != nil {
				t.Logf("stop owned native lint helper: %v", err)
			}
			<-done
		}
	})
	select {
	case <-output.queued:
	case err := <-done:
		finished = true
		t.Fatalf("native lint exited before actual queue rendezvous: %v\n%s", err, output.text())
	case <-t.Context().Done():
		t.Fatalf("native lint never reached actual queue rendezvous: %s", output.text())
	}
	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	canceled := false
	select {
	case err := <-done:
		finished = true
		if err != nil {
			t.Errorf("registered action did not return its cancellation verdict: %v\n%s", err, output.text())
		}
		canceled = strings.Contains(output.text(), "LINT-NATIVE-CANCELED")
		if !strings.Contains(output.text(), "context canceled") {
			t.Errorf("native cancellation omitted admission cause: %s", output.text())
		}
	case <-timer.C:
		t.Error("registered lint kept waiting after SIGINT while foreign holder remained live")
	}
	after, err := os.ReadFile(entry)
	if err != nil {
		t.Errorf("interrupt removed foreign holder evidence: %v", err)
	} else if !bytes.Equal(before, after) {
		t.Error("interrupt changed foreign holder entry")
	}
	body, err := os.ReadFile(log)
	if err != nil {
		t.Errorf("interrupt removed foreign holder log: %v", err)
	} else if string(body) != sentinel {
		t.Errorf("interrupt changed foreign holder log: %q", body)
	}
	if !canceled {
		t.Errorf("no native cancellation verdict before foreign slot release: %s", output.text())
	}
}
