//go:build linux || darwin

// Design: docs/architecture/testing/qemu-integration.md -- hugepages invocation ownership.
package testqemu

import (
	"bytes"
	"context"
	"encoding/json"
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
)

// TestHugepagesVerbRootReachesEveryOwnedPhase drives the registered verb to each
// real external-command boundary, then sends SIGHUP to le alone. Semantic tool
// fixtures construct the next phase's inputs and own a descendant while blocked;
// both must stop before the verb returns, without a later phase or a verdict.
// The normal control reaches both JSON assertions. This proves host lifecycle
// ownership, not a real image build, kernel reservation, or hardware boot.
func TestHugepagesVerbRootReachesEveryOwnedPhase(t *testing.T) {
	phases := []string{"host-build", "init", "image-build", "kernel", "memory"}
	for _, phase := range append(slices.Clone(phases), "normal") {
		t.Run(phase, func(t *testing.T) {
			self, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			work := t.TempDir()
			bin := filepath.Join(work, "bin")
			if err := os.Mkdir(bin, 0o700); err != nil {
				t.Fatal(err)
			}
			for _, name := range []string{"go", qemuBinary(hostArch()), "sshpass", "mkfs.ext4", "debugfs"} {
				body := "#!/bin/sh\nexport HP_ROOT_ROLE=" + name + "\nexec \"$HP_ROOT_SELF\" -test.run='^TestHugepagesRootProcess$' -- \"$@\"\n"
				if err := os.WriteFile(filepath.Join(bin, name), []byte(body), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			cmd := exec.CommandContext(ctx, self, "-test.run=^TestHugepagesRootProcess$")
			cmd.Env = append(os.Environ(), "HP_ROOT_ROLE=verb", "HP_ROOT_SELF="+self,
				"HP_ROOT_WORK="+work, "HP_ROOT_BLOCK="+phase, "ZE_REPO_ROOT="+work,
				"PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"),
				"DOCKER_HOST=unix:///nonexistent/hugepages-proof.sock")
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
					t.Error("verb fixture could not be joined")
				}
			}()
			if phase != "normal" {
				hugepagesRootAwait(t, filepath.Join(work, phase+".ready"))
				if err := cmd.Process.Signal(syscall.SIGHUP); err != nil {
					t.Fatal(err)
				}
			}
			select {
			case err := <-done:
				done <- err
				if phase == "normal" {
					if err != nil {
						t.Fatalf("normal verb failed: %v\n%s", err, output.String())
					}
					if !strings.Contains(output.String(), `"verdict":"pass"`) {
						t.Fatalf("normal verb did not assert the fixture answers: %s", output.String())
					}
				} else {
					if err == nil {
						t.Fatal("canceled verb answered success")
					}
					hugepagesRootStopped(t, work, phase)
					hugepagesRootStopped(t, work, phase+"-descendant")
					if strings.Contains(output.String(), `"verdict":`) {
						t.Errorf("canceled verb published a verdict: %s", output.String())
					}
				}
			case <-time.After(10 * time.Second):
				t.Fatal("verb did not finish bounded owned teardown")
			}
			if phase == "normal" || phase == "kernel" || phase == "memory" {
				hugepagesRootStopped(t, work, "vm")
				hugepagesRootStopped(t, work, "vm-descendant")
			}
			for index, name := range phases {
				_, err := os.Stat(filepath.Join(work, name+".entered"))
				want := phase == "normal" || index <= slices.Index(phases, phase)
				if want && err != nil {
					t.Errorf("expected reached phase %s: %v", name, err)
				}
				if !want && !os.IsNotExist(err) {
					t.Errorf("canceled verb reached later phase %s: %v", name, err)
				}
			}
		})
	}
}

// hugepagesRootAwait waits only for the fixture's reached-command handshake.
func hugepagesRootAwait(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("fixture never reached %s", path)
}

// hugepagesRootMark publishes a process event in this test's private directory.
func hugepagesRootMark(t *testing.T, name string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(os.Getenv("HP_ROOT_WORK"), name), []byte("reached\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}

// hugepagesRootSocket stays open until this process exits, including SIGKILL.
// The harness observes kernel socket closure rather than requiring graceful TERM.
func hugepagesRootSocket(t *testing.T, phase string) {
	t.Helper()
	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() }) //nolint:errcheck // Process exit also closes this observation socket.
	if err := os.WriteFile(filepath.Join(os.Getenv("HP_ROOT_WORK"), phase+".socket"), []byte(listener.Addr().String()), 0o600); err != nil {
		t.Fatal(err)
	}
}

// hugepagesRootStopped requires the process's kernel-owned observation socket to
// be gone when the action returns. No PID lookup or delayed cleanup proves this.
func hugepagesRootStopped(t *testing.T, work, phase string) {
	t.Helper()
	address, err := os.ReadFile(filepath.Join(work, phase+".socket"))
	if err != nil {
		t.Errorf("missing process observation for %s: %v", phase, err)
		return
	}
	// Not t.Context(): a canceled context fails the dial, and a failed dial is
	// this check's pass, so only the timeout may bound the probe.
	dialer := net.Dialer{Timeout: time.Second}
	connection, err := dialer.DialContext(context.Background(), "tcp", string(address))
	if err == nil {
		connection.Close() //nolint:errcheck // The successful dial is the failure being reported.
		t.Errorf("verb returned with owned %s process still alive", phase)
	}
}

// hugepagesRootBlock owns a descendant in the command's inherited process group.
// A stop receipt is written only after the descendant has actually been joined.
func hugepagesRootBlock(t *testing.T, phase string) {
	t.Helper()
	// The handler is never stopped: it stays installed until os.Exit, so a
	// second signal cannot replace the fixture's exit 0 with a signal death.
	caught := make(chan os.Signal, 1)
	signal.Notify(caught, syscall.SIGTERM, os.Interrupt)
	hugepagesRootSocket(t, phase)
	child := exec.CommandContext(t.Context(), os.Getenv("HP_ROOT_SELF"), "-test.run=^TestHugepagesRootProcess$")
	child.Env = append(os.Environ(), "HP_ROOT_ROLE=descendant", "HP_ROOT_PHASE="+phase)
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	hugepagesRootAwait(t, filepath.Join(os.Getenv("HP_ROOT_WORK"), phase+"-descendant.ready"))
	hugepagesRootMark(t, phase+".ready")
	select {
	case stopped := <-caught:
		if phase == "vm" {
			if err := child.Process.Signal(stopped); err != nil {
				t.Fatal(err)
			}
		}
		if err := child.Wait(); err != nil {
			t.Fatal(err)
		}
		hugepagesRootMark(t, phase+".stopped")
		os.Exit(0)
	case <-time.After(25 * time.Second):
		t.Fatal("command fixture never received teardown")
	}
}

// TestHugepagesRootProcess supplies semantic tools only in explicitly selected
// subprocess roles. The verb role calls the existing public action dispatcher.
func TestHugepagesRootProcess(t *testing.T) {
	role := os.Getenv("HP_ROOT_ROLE")
	if role == "" {
		return
	}
	if role == "descendant" {
		// The handler is never stopped: it stays installed until os.Exit, so a
		// second signal cannot replace the fixture's exit 0 with a signal death.
		caught := make(chan os.Signal, 1)
		signal.Notify(caught, syscall.SIGTERM, os.Interrupt)
		phase := os.Getenv("HP_ROOT_PHASE")
		hugepagesRootSocket(t, phase+"-descendant")
		hugepagesRootMark(t, phase+"-descendant.ready")
		select {
		case <-caught:
			hugepagesRootMark(t, phase+"-descendant.stopped")
			os.Exit(0)
		case <-time.After(25 * time.Second):
			t.Fatal("descendant never received teardown")
		}
	}
	if role == "verb" {
		settings := map[string]string{
			"ze.vpp.hp.arch": hostArch(), "ze.vpp.hp.keep": "0",
			"ze.vpp.hp.aarch64.bios": os.Getenv("HP_ROOT_SELF"),
			"ze.vpp.hp.pagesize":     DefaultPageSize, "ze.vpp.hp.size": DefaultReservation,
			"ze.vpp.hp.memory": DefaultMemory, "ze.vpp.hp.ssh.port": "0",
		}
		for key, value := range settings {
			if err := os.Setenv(key, value); err != nil {
				t.Fatal(err)
			}
		}
		env.ResetCache()
		report, code := Answer([]string{"vpp-hugepages"})
		if code == 0 {
			if err := json.NewEncoder(os.Stdout).Encode(report); err != nil {
				t.Fatal(err)
			}
		}
		os.Exit(code)
	}
	args := os.Args[slices.Index(os.Args, "--")+1:]
	phase := role
	if role == "go" {
		if len(args) == 0 || args[0] != "build" {
			t.Fatalf("unexpected compiler invocation: %q", args)
		}
		phase = "host-build"
	}
	if role == "host" {
		if len(args) != 3 || args[0] != "appliance" || args[2] != ApplianceName {
			t.Fatalf("unexpected appliance invocation: %q", args)
		}
		phase = args[1]
		if phase == "build" {
			phase = "image-build"
		}
	}
	if role == "sshpass" {
		hugepagesRootAwait(t, filepath.Join(os.Getenv("HP_ROOT_WORK"), "vm.ready"))
		query := args[len(args)-1]
		phase = "kernel"
		if query == MemoryQuery {
			phase = "memory"
		} else if query != KernelQuery {
			t.Fatalf("unexpected appliance query: %q", query)
		}
	}
	if strings.HasPrefix(role, "qemu-system-") {
		hugepagesRootBlock(t, "vm")
	}
	hugepagesRootMark(t, phase+".entered")
	if phase == os.Getenv("HP_ROOT_BLOCK") {
		hugepagesRootBlock(t, phase)
	}
	if phase == "host-build" {
		index := slices.Index(args, "-o")
		if index < 0 {
			t.Fatalf("compiler lacks output: %q", args)
		}
		body := "#!/bin/sh\nexport HP_ROOT_ROLE=host\nexec \"$HP_ROOT_SELF\" -test.run='^TestHugepagesRootProcess$' -- \"$@\"\n"
		if err := os.WriteFile(args[index+1], []byte(body), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if phase == "init" {
		dir := filepath.Join(os.Getenv("ZE_APPLIANCE_DIR"), ApplianceName)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "appliance.json"), []byte(`{"image":{}}`), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if phase == "image-build" {
		path := filepath.Join(os.Getenv("ZE_APPLIANCE_DIR"), ApplianceName, "ze-fixture.img")
		if err := os.WriteFile(path, []byte("host lifecycle fixture, not an appliance image"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if phase == "kernel" {
		if _, err := os.Stdout.WriteString(`{"cmdline":"default_hugepagesz=2M hugepagesz=2M hugepages=64"}`); err != nil {
			t.Fatal(err)
		}
	}
	if phase == "memory" {
		if _, err := os.Stdout.WriteString(`{"hugepages-total":64}`); err != nil {
			t.Fatal(err)
		}
	}
	os.Exit(0)
}
