// Design: docs/contributing/documentation-testing.md -- native catalog cancellation.
// Related: actions.go -- Answer, the registered consumer these probes invoke.

package docyangcontract

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/env"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	"github.com/ze-software/ze/internal/test/runner"
)

const catalogCancelProbe = "ZE_YANG_CATALOG_CANCEL_PROBE"

// TestCatalogCancellationConsumerHelper enters the registered action in an
// isolated process so the parent can deliver a real terminal signal.
func TestCatalogCancellationConsumerHelper(t *testing.T) {
	if os.Getenv(catalogCancelProbe) != "action" {
		return
	}
	env.ResetCache()
	payload, code := Answer([]string{"doc-drift"})
	result := struct {
		Code   int
		Report any
	}{Code: code, Report: payload}
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	resultPath := os.Getenv("ZE_YANG_CATALOG_RESULT")
	if err := os.WriteFile(resultPath+".pending", raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(resultPath+".pending", resultPath); err != nil {
		t.Fatal(err)
	}
	// Keep the helper's group identity pinned until the parent finishes its
	// assertions. Its outer owner MUST NOT clean descendants before those
	// assertions can distinguish product teardown from fixture cleanup.
	var release [1]byte
	if _, err := os.Stdin.Read(release[:]); err != nil && !errors.Is(err, io.EOF) {
		t.Fatal(err)
	}
}

// TestCatalogActionCancellation owns real native build and catalog subprocesses
// through Answer. The build probe gates an actual compiler under go build; the
// run probe gates the actual freshly built daemon at its invocation boundary.
// Cancellation must return a failure verdict, join both recorded owned processes,
// preserve an unrelated sentinel, and never advance beyond the canceled stage.
func TestCatalogActionCancellation(t *testing.T) {
	for _, stage := range []string{"build", "run"} {
		t.Run(stage, func(t *testing.T) {
			probe := newCatalogCancellationProbe(t, stage)
			probe.waitFile(t, probe.ready, 5*time.Minute)
			pids := probe.pids(t)
			if err := probe.command.Process.Signal(syscall.SIGTERM); err != nil {
				t.Fatal(err)
			}
			probe.wait(t, 5*time.Second)
			raw, err := os.ReadFile(probe.result)
			if err != nil {
				t.Errorf("Answer did not record its cancellation verdict: %v; output: %s", err, probe.output.String())
			} else {
				var result struct{ Code int }
				if err := json.Unmarshal(raw, &result); err != nil {
					t.Fatal(err)
				}
				if result.Code == 0 {
					t.Error("canceled catalog action answered success")
				}
			}
			for _, pid := range pids {
				if catalogProbeAlive(t, pid) {
					t.Errorf("owned %s process %d survived Answer's return", stage, pid)
				}
			}
			if !catalogProbeAlive(t, probe.sentinel.Process.Pid) {
				t.Error("cancellation killed the foreign sentinel")
			}
			if _, err := os.Stat(probe.next); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("canceled %s advanced to the next stage: %v", stage, err)
			}
		})
	}
}

// TestCatalogActionNormalInvocation releases the same run gate without a signal.
// The untouched daemon must execute and the real consumer must parse its catalog;
// unrelated standing documentation findings are not turned into catalog failures.
func TestCatalogActionNormalInvocation(t *testing.T) {
	probe := newCatalogCancellationProbe(t, "run")
	probe.waitFile(t, probe.ready, 5*time.Minute)
	if err := os.WriteFile(probe.release, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	probe.wait(t, time.Minute)
	probe.waitFile(t, probe.next, time.Second)
	raw, err := os.ReadFile(probe.result)
	if err != nil {
		t.Fatalf("normal Answer recorded no verdict: %v; output: %s", err, probe.output.String())
	}
	var result struct {
		Code   int
		Report DriftReport
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	for _, issue := range result.Report.Issues {
		if issue.Message == "could not generate or parse the live per-command catalog" {
			t.Fatalf("normal catalog invocation failed: %s", issue.Detail)
		}
	}
}

type catalogCancellationProbe struct {
	command                      *exec.Cmd
	sentinel                     *exec.Cmd
	output                       catalogProbeOutput
	ready, release, next, result string
	done                         <-chan struct{}
}

func newCatalogCancellationProbe(t *testing.T, stage string) *catalogCancellationProbe {
	t.Helper()
	root, err := lepath.Root()
	if err != nil {
		t.Fatal(err)
	}
	goBinary, err := exec.LookPath("go")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	// Root preserves ZE_REPO_ROOT's explicit path, including a symlink. This
	// names the same checkout and native inputs without inheriting its sibling
	// publication/wiki corpus, which this process-cancellation proof does not own.
	invocationRoot := filepath.Join(dir, "checkout")
	if err := os.Symlink(root, invocationRoot); err != nil {
		t.Fatal(err)
	}
	probe := &catalogCancellationProbe{
		ready: filepath.Join(dir, "ready"), release: filepath.Join(dir, "release"),
		next: filepath.Join(dir, "next"), result: filepath.Join(dir, "result"),
	}
	gateWait := "while [ ! -f " + catalogShellQuote(probe.release) + " ]; do sleep 0.02; done\n"
	gate := "sleep 120 &\nchild=$!\nprintf '%s %s\\n' \"$$\" \"$child\" > " + catalogShellQuote(probe.ready+".pending") + "\n" +
		"mv " + catalogShellQuote(probe.ready+".pending") + " " + catalogShellQuote(probe.ready) + "\n" +
		gateWait + "kill \"$child\"\nwait \"$child\" 2>/dev/null\nprintf next > " + catalogShellQuote(probe.next) + "\n"
	childEnv := append(os.Environ(), catalogCancelProbe+"=action", "ZE_REPO_ROOT="+invocationRoot,
		"ZE_YANG_CATALOG_RESULT="+probe.result)
	if stage == "build" {
		tool := filepath.Join(dir, "compiler-gate")
		// One tool owns readiness; competing compilers wait on the same release
		// without replacing that complete atomic process-identity record.
		toolGate := "case \"$*\" in *-V=full*) exec \"$@\" ;; esac\n" +
			"if mkdir " + catalogShellQuote(probe.ready+".claim") + " 2>/dev/null; then\n" +
			gate + "else\n" + gateWait + "fi\nexec \"$@\"\n"
		writeCatalogProbeScript(t, tool, toolGate)
		childEnv = append(childEnv, "GOFLAGS=-a -toolexec="+tool)
	} else {
		wrapper := filepath.Join(dir, "go")
		body := "if [ \"$1\" != build ]; then exec " + catalogShellQuote(goBinary) + " \"$@\"; fi\n" +
			"previous=\nbinary=\nfor argument do\nif [ \"$previous\" = -o ]; then binary=$argument; fi\nprevious=$argument\ndone\n" +
			catalogShellQuote(goBinary) + " \"$@\" || exit $?\n" +
			"mv \"$binary\" \"$binary.real\"\ncat > \"$binary\" <<'ZE_CATALOG_GATE'\n#!/bin/sh\n" +
			gate + "exec \"$0.real\" \"$@\"\nZE_CATALOG_GATE\nchmod 700 \"$binary\"\n"
		writeCatalogProbeScript(t, wrapper, body)
		childEnv = append(childEnv, "PATH="+dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	}
	probe.sentinel = exec.CommandContext(t.Context(), "sleep", "600")
	if err := probe.sentinel.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := probe.sentinel.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
			t.Error(err)
		}
		_ = probe.sentinel.Wait() // The sentinel is deliberately killed during cleanup.
	})
	probe.command = exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestCatalogCancellationConsumerHelper$")
	probe.command.Env = childEnv
	probe.command.Dir = root
	input, release := io.Pipe()
	probe.command.Stdin = input
	probe.command.Stdout = &probe.output
	probe.command.Stderr = &probe.output
	owner, err := runner.StartServerProcess(probe.command)
	if err != nil {
		_ = input.Close()
		_ = release.Close()
		t.Fatal(err)
	}
	// Close MUST follow every successful StartServerProcess, after assertions
	// and before temporary files are removed; it joins the sole reaper.
	t.Cleanup(func() {
		if err := release.Close(); err != nil {
			t.Error(err)
		}
		if err := owner.Close(); err != nil {
			t.Error(err)
		}
		if err := input.Close(); err != nil {
			t.Error(err)
		}
	})
	probe.done = owner.Done()
	return probe
}

func (p *catalogCancellationProbe) wait(t *testing.T, bound time.Duration) {
	t.Helper()
	p.waitFile(t, p.result, bound)
}

func (p *catalogCancellationProbe) waitFile(t *testing.T, path string, bound time.Duration) {
	t.Helper()
	timer := time.NewTimer(bound)
	defer timer.Stop()
	tick := time.NewTicker(20 * time.Millisecond)
	defer tick.Stop()
	for {
		if _, err := os.Stat(path); err == nil {
			return
		}
		select {
		case <-p.done:
			t.Fatalf("catalog helper exited before %s: %s", filepath.Base(path), p.output.String())
		case <-timer.C:
			// Failure-only diagnostics keep the original success bound intact.
			// Signal the retained os.Process handle, never a marker's numeric PID.
			if err := p.command.Process.Signal(syscall.SIGQUIT); err != nil && !errors.Is(err, os.ErrProcessDone) {
				t.Logf("request helper timeout stack: %v", err)
			}
			select {
			case <-p.done:
			case <-time.After(2 * time.Second):
				t.Log("helper did not finish its timeout stack diagnostic")
			}
			_, nextErr := os.Stat(p.next)
			t.Fatalf("catalog helper did not reach %s (next-stage marker: %v): %s", filepath.Base(path), nextErr, p.output.String())
		case <-tick.C:
		}
	}
}

func (p *catalogCancellationProbe) pids(t *testing.T) []int {
	t.Helper()
	raw, err := os.ReadFile(p.ready)
	if err != nil {
		t.Fatal(err)
	}
	fields := strings.Fields(string(raw))
	if len(fields) != 2 {
		t.Fatalf("owned process readiness has %d identities: %q", len(fields), raw)
	}
	pids := make([]int, 0, len(fields))
	for _, field := range fields {
		pid, err := strconv.Atoi(field)
		if err != nil || pid <= 0 {
			t.Fatalf("invalid owned process identity %q", field)
		}
		pids = append(pids, pid)
	}
	return pids
}

func catalogProbeAlive(t *testing.T, pid int) bool {
	t.Helper()
	command := exec.CommandContext(t.Context(), "ps", "-o", "stat=", "-p", strconv.Itoa(pid))
	raw, err := command.Output()
	if err != nil {
		if exit, ok := errors.AsType[*exec.ExitError](err); ok && exit.ExitCode() == 1 && len(raw) == 0 {
			return false
		}
		t.Fatalf("query process %d: %v", pid, err)
	}
	state := strings.TrimSpace(string(raw))
	return state != "" && !strings.HasPrefix(state, "Z")
}

func catalogShellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func writeCatalogProbeScript(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body), 0o700); err != nil {
		t.Fatal(err)
	}
}

// catalogProbeOutput permits timeout diagnostics while os/exec still drains output.
type catalogProbeOutput struct {
	mu     sync.Mutex
	buffer bytes.Buffer
}

func (o *catalogProbeOutput) Write(raw []byte) (int, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.buffer.Write(raw)
}

func (o *catalogProbeOutput) String() string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.buffer.String()
}
