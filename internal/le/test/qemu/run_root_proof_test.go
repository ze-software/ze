//go:build linux || darwin

package testqemu

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/env"
	gotoolchain "github.com/ze-software/ze/internal/le/go/toolchain"
	"github.com/ze-software/ze/internal/le/job"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// TestQEMURunRootStopsTheActualBuild sends SIGHUP to the real verb while its
// admitted compiler command is blocked. The compiler reports its inherited
// admission identity over a socket and reports TERM before exiting. The verb
// must join that command, preserve the parent, and report the originating signal.
// Both an inherited LABEL-only parent and a freshly claimed/reexecuted slot are
// exercised; killing the verb alone or returning before the command stops fails.
func TestQEMURunRootStopsTheActualBuild(t *testing.T) {
	for _, mode := range []string{"inside", "claimed"} {
		t.Run(mode, func(t *testing.T) {
			checkout, err := lepath.Root()
			if err != nil {
				t.Fatal(err)
			}
			tree, _ := stageDockerLabTree(t)
			bin := t.TempDir()
			self, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			for name, role := range map[string]string{"go": "compiler", "qemu-reexec": "verb"} {
				body := "#!/bin/sh\n"
				if name == "go" {
					body += "if [ \"$1\" != build ]; then exec \"$QEMU_ROOT_PROOF_GO\" \"$@\"; fi\n"
				}
				body += "export QEMU_ROOT_PROOF_ROLE=" + role + "\nexec \"$QEMU_ROOT_PROOF_SELF\" -test.run='^TestQEMURunRootProcess$' -- \"$@\"\n"
				if err := os.WriteFile(filepath.Join(bin, name), []byte(body), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close() //nolint:errcheck // The test owns this listening socket.
			parent := filepath.Join(t.TempDir(), "parent.job")
			const parentBody = "LABEL=outer-proof\n"
			if err := os.WriteFile(parent, []byte(parentBody), 0o600); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			cmd := exec.CommandContext(ctx, self, "-test.run=^TestQEMURunRootProcess$")
			cmd.Env = append(env.Without(env.Without(os.Environ(), job.ParentKey), "ze.repo.root"),
				"QEMU_ROOT_PROOF_ROLE=verb", "QEMU_ROOT_PROOF_SELF="+self,
				"QEMU_ROOT_PROOF_BIN="+bin, "QEMU_ROOT_PROOF_ADDRESS="+listener.Addr().String(),
				"QEMU_ROOT_PROOF_CHECKOUT="+checkout, "ZE_REPO_ROOT="+tree)
			if mode == "inside" {
				cmd.Env = append(cmd.Env, job.ParentVariable+"="+parent)
			}
			var output bytes.Buffer
			cmd.Stdout, cmd.Stderr = &output, &output
			plugin.KillGroupOnCancel(cmd)
			if err := plugin.StartChild(cmd); err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() { done <- cmd.Wait() }()
			defer func() {
				cancel()
				select {
				case <-done:
				case <-time.After(10 * time.Second):
					t.Error("proof process group could not be joined")
				}
			}()
			if tcp, ok := listener.(*net.TCPListener); ok {
				if err := tcp.SetDeadline(time.Now().Add(20 * time.Second)); err != nil {
					t.Fatal(err)
				}
			}
			connection, err := listener.Accept()
			if err != nil {
				cancel()
				select {
				case exited := <-done:
					done <- exited
					t.Fatalf("compiler command did not start: %v; helper exit: %v\n%s", err, exited, output.String())
				case <-time.After(10 * time.Second):
					t.Fatalf("compiler command did not start: %v; helper could not be joined", err)
				}
			}
			defer connection.Close() //nolint:errcheck // The test owns this accepted socket.
			if err := connection.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil {
				t.Fatal(err)
			}
			decoder := json.NewDecoder(connection)
			var started qemuRootBuildObservation
			if err := decoder.Decode(&started); err != nil {
				t.Fatalf("read command boundary: %v", err)
			}
			if !slices.Contains(started.Arguments, "build") {
				t.Fatalf("observed non-build command: %q", started.Arguments)
			}
			if mode == "inside" {
				if started.Parent != parent || started.Identity != parentBody {
					t.Fatalf("compiler lost LABEL-only admission: %+v", started)
				}
			} else if !strings.Contains(started.Identity, "LABEL="+runJobLabel+"\n") {
				t.Fatalf("compiler did not inherit the claimed qemu-run slot: %+v", started)
			}
			if err := cmd.Process.Signal(syscall.SIGHUP); err != nil {
				t.Fatal(err)
			}
			var stopped string
			if err := decoder.Decode(&stopped); err != nil {
				t.Fatalf("verb did not stop its active build command: %v", err)
			}
			if stopped != "stopped by TERM" {
				t.Fatalf("command reported %q", stopped)
			}
			select {
			case err := <-done:
				done <- err // The deferred owner observes the same completed Wait.
				var exited *exec.ExitError
				if !errors.As(err, &exited) || exited.ExitCode() != 129 {
					t.Fatalf("verb did not retain SIGHUP status: %v\n%s", err, output.String())
				}
			case <-time.After(10 * time.Second):
				t.Fatal("verb did not join the stopped build")
			}
			if mode == "claimed" {
				if _, err := os.Stat(started.Parent); !os.IsNotExist(err) {
					t.Fatalf("consumed slot was not released after join: %v", err)
				}
			}
			if body, err := os.ReadFile(parent); err != nil || string(body) != parentBody {
				t.Fatalf("foreign parent changed: %q, %v", body, err)
			}
		})
	}
}

type qemuRootBuildObservation struct {
	Arguments []string
	Parent    string
	Identity  string
}

// TestQEMURunRootProcess is the subprocess boundary for the verb and the active
// compiler fixture. Ordinary package runs do not activate either private role.
func TestQEMURunRootProcess(t *testing.T) {
	role := os.Getenv("QEMU_ROOT_PROOF_ROLE")
	if role == "" {
		return
	}
	if role == "compiler" {
		// The handler stays installed until os.Exit, so a second TERM cannot
		// replace the reported exit 143 with a signal death.
		caught := make(chan os.Signal, 1)
		signal.Notify(caught, syscall.SIGTERM)
		awaitQEMURootCompilerStop(t, caught)
		os.Exit(143)
	}
	root := os.Getenv("ZE_REPO_ROOT")
	checkout := os.Getenv("QEMU_ROOT_PROOF_CHECKOUT")
	if err := os.Setenv("PATH", filepath.Join(os.Getenv("GOROOT"), "bin")+string(os.PathListSeparator)+os.Getenv("PATH")); err != nil {
		t.Fatal(err)
	}
	if err := os.Setenv("ZE_REPO_ROOT", checkout); err != nil {
		t.Fatal(err)
	}
	env.ResetCache()
	if _, err := gotoolchain.New(checkout); err != nil {
		t.Fatal(err)
	}
	compiler, err := filepath.EvalSymlinks(filepath.Join(os.Getenv("GOROOT"), "bin", "go"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Setenv("QEMU_ROOT_PROOF_GO", compiler); err != nil {
		t.Fatal(err)
	}
	if err := os.Setenv("ZE_REPO_ROOT", root); err != nil {
		t.Fatal(err)
	}
	env.ResetCache()
	if err := os.Setenv("PATH", os.Getenv("QEMU_ROOT_PROOF_BIN")+string(os.PathListSeparator)+os.Getenv("PATH")); err != nil {
		t.Fatal(err)
	}
	env.ResetCache()
	runSelf = func() (string, error) { return filepath.Join(os.Getenv("QEMU_ROOT_PROOF_BIN"), "qemu-reexec"), nil }
	_, code := runQEMUHere(leaction.Arguments{keywordCommand: {"true"}})
	os.Exit(code)
}

// awaitQEMURootCompilerStop is the compiler role of TestQEMURunRootProcess: it
// reports its argv and job identity, then returns once TERM arrives, so its
// deferred close runs before the caller exits the process.
func awaitQEMURootCompilerStop(t *testing.T, caught <-chan os.Signal) {
	dialer := net.Dialer{Timeout: 10 * time.Second}
	connection, err := dialer.DialContext(t.Context(), "tcp", os.Getenv("QEMU_ROOT_PROOF_ADDRESS"))
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close() //nolint:errcheck // The process exits next; the test side observes the stream.
	parent := os.Getenv(job.ParentVariable)
	identity, err := os.ReadFile(parent)
	if err != nil {
		t.Fatal(err)
	}
	encoder := json.NewEncoder(connection)
	if err := encoder.Encode(qemuRootBuildObservation{os.Args, parent, string(identity)}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-caught:
		if err := encoder.Encode("stopped by TERM"); err != nil {
			t.Fatal(err)
		}
	case <-time.After(40 * time.Second):
		t.Fatal("compiler fixture was never stopped")
	}
}
