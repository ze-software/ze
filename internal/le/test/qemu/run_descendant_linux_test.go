//go:build linux

// Design: docs/architecture/testing/qemu-integration.md -- admission owns the guest compiler's descendants.
package testqemu

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/env"
	gotoolchain "github.com/ze-software/ze/internal/le/go/toolchain"
	"github.com/ze-software/ze/internal/le/job"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// TestQEMURunRootJoinsActualGoDescendants reaches the registered run action and
// real activated Go build. GOFLAGS -toolexec holds the real compiler version
// query, so the blocked process is Go's child, not a replacement for Go itself.
// SIGHUP must stop and join both generations before the invocation releases its
// claimed slot or returns inside a parent's slot. The resistant tool ignores
// TERM before publishing readiness; returning while it still runs fails.
func TestQEMURunRootJoinsActualGoDescendants(t *testing.T) {
	for _, mode := range []string{"inside", "claimed"} {
		for _, resistance := range []string{"normal", "ignore-term"} {
			t.Run(mode+"/"+resistance, func(t *testing.T) {
				self, err := os.Executable()
				if err != nil {
					t.Fatal(err)
				}
				command := exec.CommandContext(t.Context(), self, "-test.run=^TestQEMURunDescendantProcess$")
				command.Env = append(os.Environ(), "QEMU_DESCENDANT_ROLE=supervisor", "QEMU_DESCENDANT_MODE="+mode, "QEMU_DESCENDANT_RESISTANCE="+resistance)
				output, err := command.CombinedOutput()
				if err != nil {
					t.Fatalf("real Go descendant proof: %v\n%s", err, output)
				}
			})
		}
	}
}

type qemuDescendantObservation struct {
	PID      int
	GoPID    int
	Args     []string
	Parent   string
	Identity string
}

// TestQEMURunDescendantProcess isolates fixture adoption in a subprocess. The
// supervisor is a subreaper only for its own children: after a failing baseline
// it releases the held tool through its socket and reaps any adopted orphan,
// without signaling a numeric PID after the verb has been waited for.
func TestQEMURunDescendantProcess(t *testing.T) {
	switch os.Getenv("QEMU_DESCENDANT_ROLE") {
	case "supervisor":
		proveQEMUDescendant(t)
	case "verb":
		runQEMUDescendantVerb(t)
	case "tool":
		holdQEMUCompilerDescendant(t)
	}
}

func proveQEMUDescendant(t *testing.T) {
	t.Helper()
	if err := unix.Prctl(unix.PR_SET_CHILD_SUBREAPER, 1, 0, 0, 0); err != nil {
		t.Fatalf("isolate fixture orphan reaping: %v", err)
	}
	checkout, err := lepath.Root()
	if err != nil {
		t.Fatal(err)
	}
	tree, _ := stageDockerLabTree(t)
	// An authored minimal entry gives the actual Go build a package to load;
	// the held compiler version query prevents any image or VM work.
	entryDir := filepath.Join(tree, "cmd", "ze")
	if err := os.MkdirAll(entryDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(entryDir, "main.go"), []byte("package main\nfunc main() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	commitTree(t, tree)
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	wrapper := filepath.Join(bin, "toolexec")
	toolScript := "#!/bin/sh\nif [ \"${1##*/}\" = compile ] && [ \"$2\" = -V=full ]; then\n export QEMU_DESCENDANT_ROLE=tool\n exec \"$QEMU_DESCENDANT_SELF\" -test.run=^TestQEMURunDescendantProcess$ -- \"$@\"\nfi\nexec \"$@\"\n"
	if err := os.WriteFile(wrapper, []byte(toolScript), 0o700); err != nil {
		t.Fatal(err)
	}
	reexec := filepath.Join(bin, "qemu-reexec")
	if err := os.WriteFile(reexec, []byte("#!/bin/sh\nexport QEMU_DESCENDANT_ROLE=verb\nexec \"$QEMU_DESCENDANT_SELF\" -test.run=^TestQEMURunDescendantProcess$ -- \"$@\"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	if err := listener.(*net.TCPListener).SetDeadline(time.Now().Add(30 * time.Second)); err != nil {
		t.Fatal(err)
	}
	parent := filepath.Join(t.TempDir(), "outer.job")
	const parentBody = "LABEL=outer-descendant-proof\n"
	if err := os.WriteFile(parent, []byte(parentBody), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	command := exec.CommandContext(ctx, self, "-test.run=^TestQEMURunDescendantProcess$", "--", "test", "qemu", "run", "command", "true")
	command.Env = append(env.Without(os.Environ(), job.ParentKey, "ze.repo.root"),
		"QEMU_DESCENDANT_ROLE=verb", "QEMU_DESCENDANT_SELF="+self,
		"QEMU_DESCENDANT_CHECKOUT="+checkout, "ZE_REPO_ROOT="+tree,
		"QEMU_DESCENDANT_REEXEC="+reexec, "QEMU_DESCENDANT_ADDRESS="+listener.Addr().String(),
		"QEMU_DESCENDANT_GOFLAGS="+strings.TrimSpace(os.Getenv("GOFLAGS")+" "+strconv.Quote("-toolexec="+wrapper)))
	if os.Getenv("QEMU_DESCENDANT_MODE") == "inside" {
		command.Env = append(command.Env, job.ParentVariable+"="+parent)
	}
	var output bytes.Buffer
	command.Stdout, command.Stderr = &output, &output
	plugin.KillGroupOnCancel(command)
	if err := plugin.StartChild(command); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	var connection net.Conn
	var observed qemuDescendantObservation
	defer func() {
		// Releasing this fixture's control socket ends an unpatched survivor.
		// This cleanup occurs only AFTER the regression assertions below.
		if connection != nil {
			connection.Close()
		}
		cancel()
		select {
		case <-done:
		case <-time.After(15 * time.Second):
			t.Error("fixture verb could not be joined during cleanup")
		}
		if observed.PID != 0 {
			if err := reapQEMUDescendant(observed.PID, 5*time.Second); err != nil {
				t.Errorf("reap the fixture's adopted compiler: %v", err)
			}
		}
	}()
	connection, err = listener.Accept()
	if err != nil {
		acceptErr := err
		cancel()
		waitErr := <-done
		done <- waitErr
		t.Fatalf("actual Go compiler did not reach -toolexec rendezvous: %v; verb exit: %v\n%s", acceptErr, waitErr, output.String())
	}
	if err := connection.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil {
		t.Fatal(err)
	}
	decoder := json.NewDecoder(connection)
	if err := decoder.Decode(&observed); err != nil {
		t.Fatalf("read actual compiler boundary: %v", err)
	}
	if observed.PID <= 1 || observed.GoPID <= 1 || observed.GoPID == command.Process.Pid || len(observed.Args) != 2 || filepath.Base(observed.Args[0]) != "compile" || observed.Args[1] != "-V=full" {
		t.Fatalf("fixture did not reach an actual Go compiler descendant: %+v", observed)
	}
	if os.Getenv("QEMU_DESCENDANT_MODE") == "inside" {
		if observed.Parent != parent || observed.Identity != parentBody {
			t.Fatalf("compiler lost its inherited identity: %+v", observed)
		}
	} else if !strings.Contains(observed.Identity, "LABEL="+runJobLabel+"\n") {
		t.Fatalf("compiler lost its claimed identity: %+v", observed)
	}
	if err := command.Process.Signal(syscall.SIGHUP); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		done <- err
		var exited *exec.ExitError
		if !errors.As(err, &exited) || exited.ExitCode() != 129 {
			t.Fatalf("verb lost its originating SIGHUP status: %v\n%s", err, output.String())
		}
	case <-time.After(15 * time.Second):
		t.Fatal("verb did not join its owned build within the existing teardown grace")
	}
	if err := connection.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	var stopped string
	if err := decoder.Decode(&stopped); !errors.Is(err, io.EOF) {
		t.Errorf("verb returned while actual Go compiler descendant still held its socket: event=%q error=%v", stopped, err)
	}
	if err := syscall.Kill(observed.GoPID, 0); !errors.Is(err, syscall.ESRCH) {
		t.Errorf("verb returned before its actual Go child was reaped: pid=%d error=%v", observed.GoPID, err)
	}
	if body, err := os.ReadFile(parent); err != nil || string(body) != parentBody {
		t.Errorf("foreign parent changed: %q, %v", body, err)
	}
	if os.Getenv("QEMU_DESCENDANT_MODE") == "claimed" {
		if _, err := os.Stat(observed.Parent); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("joined invocation did not release its own claimed slot: %v", err)
		}
	}
}

func runQEMUDescendantVerb(t *testing.T) {
	t.Helper()
	root := os.Getenv("ZE_REPO_ROOT")
	if err := os.Setenv("ZE_REPO_ROOT", os.Getenv("QEMU_DESCENDANT_CHECKOUT")); err != nil {
		t.Fatal(err)
	}
	if err := os.Setenv("PATH", filepath.Join(os.Getenv("GOROOT"), "bin")+string(os.PathListSeparator)+os.Getenv("PATH")); err != nil {
		t.Fatal(err)
	}
	env.ResetCache()
	if _, err := gotoolchain.New(os.Getenv("QEMU_DESCENDANT_CHECKOUT")); err != nil {
		t.Fatal(err)
	}
	if err := os.Setenv("ZE_REPO_ROOT", root); err != nil {
		t.Fatal(err)
	}
	if err := os.Setenv("GOFLAGS", os.Getenv("QEMU_DESCENDANT_GOFLAGS")); err != nil {
		t.Fatal(err)
	}
	env.ResetCache()
	runSelf = func() (string, error) { return os.Getenv("QEMU_DESCENDANT_REEXEC"), nil }
	separator := slices.Index(os.Args, "--")
	if separator < 0 || len(os.Args) < separator+4 || !slices.Equal(os.Args[separator+1:separator+3], []string{"test", "qemu"}) {
		t.Fatalf("wrong registered command words: %q", os.Args)
	}
	_, code := Answer(os.Args[separator+3:])
	os.Exit(code)
}

func holdQEMUCompilerDescendant(t *testing.T) {
	t.Helper()
	if os.Getenv("QEMU_DESCENDANT_RESISTANCE") == "ignore-term" {
		signal.Ignore(syscall.SIGTERM)
	}
	connection, err := net.DialTimeout("tcp", os.Getenv("QEMU_DESCENDANT_ADDRESS"), 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	entry := os.Getenv(job.ParentVariable)
	body, err := os.ReadFile(entry)
	if err != nil {
		t.Fatal(err)
	}
	separator := slices.Index(os.Args, "--")
	if separator < 0 {
		t.Fatal("missing actual compiler arguments")
	}
	if err := json.NewEncoder(connection).Encode(qemuDescendantObservation{os.Getpid(), os.Getppid(), os.Args[separator+1:], entry, string(body)}); err != nil {
		t.Fatal(err)
	}
	if err := connection.SetReadDeadline(time.Now().Add(45 * time.Second)); err != nil {
		t.Fatal(err)
	}
	// EOF is a fixture-only release after assertions. Product cancellation must
	// instead terminate this process, closing the socket from the other end.
	var release [1]byte
	if _, err := connection.Read(release[:]); !errors.Is(err, io.EOF) {
		t.Fatalf("fixture control ended without owned release: %v", err)
	}
}

func reapQEMUDescendant(pid int, bound time.Duration) error {
	deadline := time.Now().Add(bound)
	for {
		var status unix.WaitStatus
		waited, err := unix.Wait4(pid, &status, unix.WNOHANG, nil)
		if waited == pid {
			return nil
		}
		if errors.Is(err, unix.ECHILD) && errors.Is(syscall.Kill(pid, 0), syscall.ESRCH) {
			return nil
		}
		if err != nil && !errors.Is(err, unix.ECHILD) && !errors.Is(err, unix.EINTR) {
			return err
		}
		if !time.Now().Before(deadline) {
			return errors.New("fixture compiler has not exited and been reaped")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
