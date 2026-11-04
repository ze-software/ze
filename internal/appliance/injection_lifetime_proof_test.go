//go:build linux || darwin

// Design: docs/architecture/appliance/builder.md -- cancellation inside image injection.
package appliance

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
	"syscall"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/test/runner"
)

// TestInjectionConsumerCancellationJoinsDescendants cancels after the actual
// InjectDatabase consumer starts its real mkfs external-command phase. Only the
// executable is isolated: a descendant holds both output and an observer socket.
// The consumer must preserve cancellation, join that descendant, retain a foreign
// sentinel and start no debugfs command. No filesystem is formatted or mounted.
func TestInjectionConsumerCancellationJoinsDescendants(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	work := t.TempDir()
	image := filepath.Join(work, "image")
	data := make([]byte, 8192)
	for i := range 16 {
		data[1024+i] = 0xaa
	}
	data[1024+32], data[1024+40] = 8, 15
	if err := os.WriteFile(image, data, 0o600); err != nil {
		t.Fatal(err)
	}
	database := filepath.Join(work, "database.zefs")
	if err := os.WriteFile(database, []byte("injection-lifetime-database"), 0o600); err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	mkfs := "#!/bin/sh\nexport INJECT_LIFETIME_ROLE=parent\nexec \"$INJECT_LIFETIME_SELF\" -test.run='^TestInjectionLifetimeProcess$'\n"
	if err := os.WriteFile(filepath.Join(bin, "mkfs.ext4"), []byte(mkfs), 0o700); err != nil {
		t.Fatal(err)
	}
	later := filepath.Join(work, "later")
	for _, name := range []string{"debugfs", "e2fsck"} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte("#!/bin/sh\nprintf reached >> \"$INJECT_LIFETIME_LATER\"\n"), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	useE2FSDir(t, bin)
	oldExecute := runExternalFn
	runExternalFn = runExternal
	t.Cleanup(func() { runExternalFn = oldExecute })
	t.Setenv("INJECT_LIFETIME_SELF", self)
	t.Setenv("INJECT_LIFETIME_ADDRESS", listener.Addr().String())
	t.Setenv("INJECT_LIFETIME_LATER", later)

	foreignCtx, stopForeign := context.WithCancel(t.Context())
	foreign := exec.CommandContext(foreignCtx, self, "-test.run=^TestInjectionLifetimeProcess$")
	foreign.Env = append(os.Environ(), "INJECT_LIFETIME_ROLE=descendant")
	plugin.KillGroupOnCancel(foreign)
	if err := plugin.StartChild(foreign); err != nil {
		stopForeign()
		t.Fatal(err)
	}
	defer func() {
		stopForeign()
		if err := foreign.Wait(); err != nil {
			var exited *exec.ExitError
			if !errors.As(err, &exited) {
				t.Error(err)
			}
		}
	}()
	sentinel := acceptInjectionLifetime(t, listener)
	defer sentinel.Close()
	var sentinelPID int
	if err := json.NewDecoder(sentinel).Decode(&sentinelPID); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		// The old API is deliberately executed for its semantic baseline, not
		// failed by reflection. This test-only dispatch disappears from neither
		// production behavior nor its real external command path.
		switch inject := any(InjectDatabase).(type) {
		case func(string, string) error:
			done <- inject(image, database)
		case func(context.Context, string, string, func(context.Context, string, ...string) ([]byte, error)) error:
			done <- inject(ctx, image, database, executeInjectionLifetime)
		default:
			done <- errors.New("unexpected injection consumer contract")
		}
	}()
	owned := acceptInjectionLifetime(t, listener)
	defer owned.Close()
	var ownedPID int
	if err := json.NewDecoder(owned).Decode(&ownedPID); err != nil {
		t.Fatal(err)
	}
	defer func() {
		owned.Close()
		cancel()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("injection did not join after releasing its fixture descendant")
		}
	}()
	cancel()
	select {
	case err := <-done:
		done <- err
		if !errors.Is(err, context.Canceled) {
			t.Errorf("injection lost originating cancellation: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("cancellation inside actual injection waited on its surviving descendant")
	}
	if err := owned.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	var one [1]byte
	if _, err := owned.Read(one[:]); err == nil {
		t.Error("injection descendant wrote after consumer return")
	} else if timeout, ok := err.(net.Error); ok && timeout.Timeout() {
		t.Error("injection descendant survived consumer return")
	}
	if err := sentinel.SetReadDeadline(time.Now().Add(100 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	if _, err := sentinel.Read(one[:]); err == nil {
		t.Error("foreign sentinel unexpectedly wrote")
	} else if timeout, ok := err.(net.Error); !ok || !timeout.Timeout() {
		t.Errorf("injection cancellation touched foreign sentinel: %v", err)
	}
	if _, err := os.Stat(later); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("injection started a later external phase: %v", err)
	}
}

func executeInjectionLifetime(ctx context.Context, name string, args ...string) (output []byte, resultErr error) {
	command := exec.CommandContext(ctx, name, args...)
	var stdout bytes.Buffer
	command.Stdout, command.Stderr = &stdout, io.Discard
	owner, err := runner.StartServerProcess(command)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(ctx.Err(), owner.Close()) }()
	if err := owner.Wait(); err != nil {
		return stdout.Bytes(), err
	}
	return stdout.Bytes(), nil
}

func acceptInjectionLifetime(t *testing.T, listener net.Listener) net.Conn {
	t.Helper()
	if err := listener.(*net.TCPListener).SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		t.Fatal(err)
	}
	connection, err := listener.Accept()
	if err != nil {
		t.Fatal(err)
	}
	if err := connection.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil {
		t.Fatal(err)
	}
	return connection
}

// TestInjectionLifetimeProcess supplies only an executable boundary; the actual
// injection consumer selects and starts it through the ordinary mkfs code path.
func TestInjectionLifetimeProcess(t *testing.T) {
	switch os.Getenv("INJECT_LIFETIME_ROLE") {
	case "parent":
		child := exec.Command(os.Args[0], "-test.run=^TestInjectionLifetimeProcess$")
		child.Env = append(os.Environ(), "INJECT_LIFETIME_ROLE=descendant")
		child.Stdout, child.Stderr = os.Stdout, os.Stderr
		if err := child.Start(); err != nil {
			t.Fatal(err)
		}
		if err := child.Wait(); err != nil {
			t.Fatal(err)
		}
	case "descendant":
		signal.Ignore(syscall.SIGTERM)
		connection, err := net.DialTimeout("tcp", os.Getenv("INJECT_LIFETIME_ADDRESS"), 5*time.Second)
		if err != nil {
			t.Fatal(err)
		}
		defer connection.Close()
		if err := json.NewEncoder(connection).Encode(os.Getpid()); err != nil {
			t.Fatal(err)
		}
		if err := connection.SetReadDeadline(time.Now().Add(30 * time.Second)); err != nil {
			t.Fatal(err)
		}
		var control [1]byte
		if _, err := connection.Read(control[:]); err == nil {
			t.Fatal("unexpected injection fixture control byte")
		}
		os.Exit(0)
	}
}
