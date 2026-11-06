// Design: docs/architecture/testing/runner-architecture.md -- child boundary fault proofs.

package editorcase

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	editortest "github.com/ze-software/ze/internal/component/cli/testing"
	"github.com/ze-software/ze/internal/component/plugin"
)

var admissionCoverage = flag.String("editor-admission-cover-dir", "", "directory retaining descriptor-child coverage profiles")

// TestEditorDiagnosticBackpressure proves the process owner never waits for a
// diagnostic write to finish before forcing cancellation. The helper calls the
// real watchControl with a full pipe, NOT Main: pipe admission is tested below
// and rejects it. This is an unresponsive-child proof, not a claim that the
// production regular file has pipe semantics or a bounded kernel write.
// MUTATION: omit child.force after the cooperative timer fires.
func TestEditorDiagnosticBackpressure(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	files, err := newCaseFiles(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	result := &editortest.TestResult{Error: "backpressure fixture"}
	defer files.finish(result)
	content := "# child-fixture:diagnostic-blocked\noption=mode:value=command\n"
	req := request{Content: &content, Deadline: time.Now().Add(20 * time.Second), Directory: files.work}
	if err := json.NewEncoder(files.handles[0]).Encode(req); err != nil {
		t.Fatal(err)
	}
	child, err := files.start(self)
	if err != nil {
		t.Fatal(err)
	}
	defer child.close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// EOF is retained in the pipe even if startup has not yet reached Read.
	cancel()
	started := time.Now()
	answer := make(chan error, 1)
	go func() { answer <- child.wait(ctx, files) }()
	limit := time.NewTimer(cooperativeGrace + 5*time.Second)
	defer limit.Stop()
	select {
	case err := <-answer:
		if err == nil {
			t.Fatal("blocked diagnostic passed")
		}
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("lost cancellation: %v", err)
		}
		if !strings.Contains(err.Error(), "signal: killed") {
			t.Fatalf("child did not need hard cancellation: %v", err)
		}
	case <-limit.C:
		child.force()
		<-answer
		t.Fatal("diagnostic write extended the cooperative grace; watchdog had to kill child")
	}
	if time.Since(started) < cooperativeGrace {
		t.Fatal("hard cancellation skipped cooperative grace")
	}
	data, err := os.ReadFile(files.handles[2].Name())
	if err != nil {
		t.Fatal(err)
	}
	if !blockedDiagnosticStack(data) {
		t.Fatalf("helper never reached blocked stack writer:\n%s", data)
	}
	if err := syscall.Kill(child.pid, 0); !errors.Is(err, syscall.ESRCH) {
		t.Fatalf("diagnostic writer survived reap: %v", err)
	}
	work := files.work
	handles := files.handles
	files.finish(result)
	if _, err := os.Stat(work); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("work survived cleanup: %v", err)
	}
	for _, file := range handles {
		if file == nil {
			continue
		}
		if _, err := file.Stat(); !errors.Is(err, os.ErrClosed) {
			t.Fatalf("descriptor survived cleanup: %v", err)
		}
	}
}

// blockedDiagnosticFixture substitutes backpressure at the writer call only.
// FD5 stays a regular file for an independent stack witness. The full pipe's
// reader remains live, so neither EPIPE nor a finalizer can release the writer.
func blockedDiagnosticFixture(req *request) int {
	reader, writer, err := os.Pipe()
	if err != nil {
		return 120
	}
	defer reader.Close() //nolint:errcheck // The owned child is forcibly terminated.
	defer writer.Close() //nolint:errcheck // The owned child is forcibly terminated.
	fd := int(writer.Fd())
	if err := unix.SetNonblock(fd, true); err != nil {
		return 121
	}
	buffer := make([]byte, 4096)
	for {
		_, err := unix.Write(fd, buffer)
		if errors.Is(err, unix.EAGAIN) {
			break
		}
		if err != nil {
			return 122
		}
	}
	if err := unix.SetNonblock(fd, false); err != nil {
		return 128
	}
	if err := unix.SetNonblock(6, true); err != nil {
		return 123
	}
	control := os.NewFile(6, "control")
	diagnostic := os.NewFile(5, "diagnostic-witness")
	ctx, cancel := context.WithDeadline(context.Background(), req.Deadline)
	defer cancel()
	joined := make(chan error, 1)
	go watchControl(control, writer, cancel, req.Deadline, make(chan struct{}), joined)
	<-ctx.Done()
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	limit := time.NewTimer(time.Second)
	defer limit.Stop()
	for {
		var stacks bytes.Buffer
		if err := pprof.Lookup("goroutine").WriteTo(&stacks, 2); err != nil {
			return 124
		}
		if blockedDiagnosticStack(stacks.Bytes()) {
			if _, err := diagnostic.Write(stacks.Bytes()); err != nil {
				return 125
			}
			<-joined
			runtime.KeepAlive(reader)
			return 126 // The full pipe must not unblock before the parent's kill.
		}
		select {
		case <-limit.C:
			return 127
		case <-ticker.C:
		}
	}
}

func blockedDiagnosticStack(data []byte) bool {
	for stack := range bytes.SplitSeq(data, []byte("\n\n")) {
		if !bytes.Contains(stack, []byte("editorcase.watchControl")) {
			continue
		}
		if !bytes.Contains(stack, []byte("runtime/pprof.writeGoroutine")) {
			continue
		}
		if bytes.Contains(stack, []byte("internal/poll.(*FD).Write")) {
			return true
		}
	}
	return false
}

// TestEditorAdmissionChild is a subprocess-only entry so invalid/closed inherited
// descriptors reach actual Main without damaging the test driver's descriptors.
// Running m.Run in the child also permits a separate child coverage profile.
func TestEditorAdmissionChild(t *testing.T) {
	if len(flag.Args()) != 1 {
		t.Skip("subprocess fixture requires a descriptor case")
	}
	mode := flag.Args()[0]
	if descriptor, closed := strings.CutPrefix(mode, "closed-"); closed {
		fd, err := strconv.Atoi(descriptor)
		if err != nil {
			t.Fatal(err)
		}
		if err := syscall.Close(fd); err != nil {
			t.Fatal(err)
		}
	}
	if code := Main(nil); code != 1 {
		t.Fatalf("invalid descriptors accepted: exit %d", code)
	}
}

// TestEditorDescriptorAdmission rejects pipe request/result/diagnostic streams,
// a regular-file control channel, and each closed inherited descriptor. It
// requires no result publication, actual child reap, and parent-owned cleanup.
// MUTATION: return success for a regular-file control descriptor in mainFiles.
func TestEditorDescriptorAdmission(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"stream-3", "stream-4", "stream-5", "file-control", "closed-3", "closed-4", "closed-5", "closed-6"} {
		t.Run(mode, func(t *testing.T) {
			files, err := newCaseFiles(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			result := &editortest.TestResult{Error: "descriptor admission fixture"}
			defer files.finish(result)
			content := "option=mode:value=command\n"
			req := request{Content: &content, Deadline: time.Now().Add(10 * time.Second), Directory: files.work}
			if err := json.NewEncoder(files.handles[0]).Encode(req); err != nil {
				t.Fatal(err)
			}
			extra := append([]*os.File(nil), files.handles[:4]...)
			cause := "bad file descriptor"
			if descriptor, stream := strings.CutPrefix(mode, "stream-"); stream {
				fd, err := strconv.Atoi(descriptor)
				if err != nil {
					t.Fatal(err)
				}
				reader, writer, err := os.Pipe()
				if err != nil {
					t.Fatal(err)
				}
				defer reader.Close() //nolint:errcheck // Test-only invalid descriptor transport.
				defer writer.Close() //nolint:errcheck // Test-only invalid descriptor transport.
				extra[fd-3] = reader
				cause = "request, result and diagnostic descriptors must be regular files"
			}
			if mode == "file-control" {
				extra[3] = files.handles[0]
				cause = "control descriptor must be a pipe"
			}
			args := []string{"-test.run=^TestEditorAdmissionChild$", "-test.v"}
			if testing.CoverMode() != "" {
				profile := filepath.Join(filepath.Dir(files.work), "admission-cover.out")
				if *admissionCoverage != "" {
					file, err := os.CreateTemp(*admissionCoverage, "editor-admission-"+mode+"-*.out")
					if err != nil {
						t.Fatal(err)
					}
					profile = file.Name()
					if err := file.Close(); err != nil {
						t.Fatal(err)
					}
				}
				args = append(args, "-test.coverprofile="+profile)
				t.Logf("child count profile: %s", profile)
			}
			args = append(args, "--", mode)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, self, args...)
			plugin.KillGroupOnCancel(cmd)
			cmd.WaitDelay = childPipeWait
			cmd.ExtraFiles = extra
			var output bytes.Buffer
			cmd.Stdout, cmd.Stderr = &output, &output
			if err := plugin.StartChild(cmd); err != nil {
				t.Fatal(err)
			}
			if err := cmd.Wait(); err != nil {
				t.Fatalf("admission fixture: %v\n%s", err, output.Bytes())
			}
			if !strings.Contains(output.String(), cause) {
				t.Fatalf("wrong admission refusal:\n%s", output.Bytes())
			}
			if err := syscall.Kill(cmd.Process.Pid, 0); !errors.Is(err, syscall.ESRCH) {
				t.Fatalf("unreaped admission child: %v", err)
			}
			stat, err := files.handles[1].Stat()
			if err != nil {
				t.Fatal(err)
			}
			if stat.Size() != 0 {
				t.Fatal("invalid descriptors published a result")
			}
			if _, err := os.Stat(files.work); err != nil {
				t.Fatalf("child removed parent resources: %v", err)
			}
			work, handles := files.work, files.handles
			files.finish(result)
			if _, err := os.Stat(work); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("work survived owner cleanup: %v", err)
			}
			for _, file := range handles {
				if _, err := file.Stat(); !errors.Is(err, os.ErrClosed) {
					t.Fatalf("parent descriptor not closed: %v", err)
				}
			}
		})
	}
}

// TestEditorStartFailure reaches real exec ENOENT/EACCES/ENOEXEC errors. Nothing
// may publish a child result or retain the parent's work/descriptors on refusal.
// MUTATION: omit work removal in caseFiles.finish.
func TestEditorStartFailure(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("exec errno and owned-FD census use Linux")
	}
	for _, mode := range []string{"missing", "permission", "format"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			executable := filepath.Join(t.TempDir(), "child")
			if mode != "missing" {
				permissions := os.FileMode(0o600)
				if mode == "format" {
					permissions = 0o700
				}
				if err := os.WriteFile(executable, []byte("not an executable image\n"), permissions); err != nil {
					t.Fatal(err)
				}
			}
			before, err := pipeDescriptors()
			if err != nil {
				t.Fatal(err)
			}
			result := Run(context.Background(), executable, "option=mode:value=command\n", root, 1)
			after, err := pipeDescriptors()
			if err != nil {
				t.Fatal(err)
			}
			for pipe, count := range after {
				if count > before[pipe] {
					t.Fatalf("failed start leaked a pipe descriptor: %s", pipe)
				}
			}
			if result.Passed {
				t.Fatal("start failure passed")
			}
			if !strings.Contains(result.Error, "start editor child:") {
				t.Fatalf("wrong refusal: %+v", result)
			}
			cause := syscall.ENOENT.Error()
			if mode == "permission" {
				cause = syscall.EACCES.Error()
			}
			if mode == "format" {
				cause = syscall.ENOEXEC.Error()
			}
			if !strings.Contains(result.Error, cause) {
				t.Fatalf("lost exec cause: %+v", result)
			}
			if result.TempDir != "" {
				t.Fatalf("work cleanup incomplete: %+v", result)
			}
			entries, err := os.ReadDir(root)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 1 {
				t.Fatalf("expected retained failure artifacts: %v", entries)
			}
			artifact := filepath.Join(root, entries[0].Name())
			if _, err := os.Stat(filepath.Join(artifact, "work")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("work survived failed start: %v", err)
			}
			for _, name := range []string{"result.json", "child.log"} {
				data, err := os.ReadFile(filepath.Join(artifact, name))
				if err != nil {
					t.Fatal(err)
				}
				if len(data) != 0 {
					t.Fatalf("failed exec published %s: %s", name, data)
				}
			}
			if err := noOwnedDescriptors(root); err != nil {
				t.Fatal(err)
			}
			// At the spawn seam a failed exec returns no child/Wait ownership.
			// The caller still owns all six descriptors and must close them.
			files, err := newCaseFiles(root)
			if err != nil {
				t.Fatal(err)
			}
			defer files.finish(&editortest.TestResult{Error: "failed-start fixture"})
			child, startErr := files.start(executable)
			if child != nil {
				child.close()
				t.Fatal("failed exec returned live child ownership")
			}
			if startErr == nil {
				t.Fatal("failed exec returned no error")
			}
			handles := files.handles
			files.finish(&editortest.TestResult{Error: "failed-start fixture"})
			for _, file := range handles {
				if _, err := file.Stat(); !errors.Is(err, os.ErrClosed) {
					t.Fatalf("failed-start descriptor remains open: %v", err)
				}
			}
		})
	}
}

// pipeDescriptors counts only this test process's pipe endpoints, so the
// inherited-control pair is covered as well as the named artifact handles.
func pipeDescriptors() (map[string]int, error) {
	entries, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		return nil, err
	}
	pipes := make(map[string]int)
	for _, entry := range entries {
		target, err := os.Readlink("/proc/self/fd/" + entry.Name())
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if strings.HasPrefix(target, "pipe:[") {
			pipes[target]++
		}
	}
	return pipes, nil
}

func noOwnedDescriptors(root string) error {
	entries, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		return err
	}
	for _, entry := range entries {
		target, err := os.Readlink("/proc/self/fd/" + entry.Name())
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		if strings.HasPrefix(target, root+string(os.PathSeparator)) {
			return fmt.Errorf("owned descriptor leaked: %s -> %s", entry.Name(), target)
		}
	}
	return nil
}
