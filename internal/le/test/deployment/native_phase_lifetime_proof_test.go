//go:build linux || darwin

// Design: docs/architecture/testing/qemu-integration.md -- native appliance process ownership.
package testdeployment

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
)

// TestNativeProofContextEntryAvailable reports the absent production context
// contract explicitly, without a compile failure or bypassing the kernel gate.
func TestNativeProofContextEntryAvailable(t *testing.T) {
	if _, ok := any(NewL2TPPPP(t.TempDir())).(interface {
		Run(context.Context) (L2TPPPPReport, error)
	}); !ok {
		t.Fatal("feature absent: native l2tp-ppp Run has no originating context")
	}
	if _, ok := any(newGokrazyLab("native-context-proof")).(interface {
		setup(context.Context) error
	}); !ok {
		t.Fatal("feature absent: gokrazy lab setup has no originating context")
	}
}

// TestNativeAppliancePhaseCancellationJoinsDescendants drives the actual
// database initialization and appliance boot producers. Only external commands
// and privileged kernel listings are isolated; production phase code starts,
// watches and stops the processes. A foreign sentinel must remain connected.
func TestNativeAppliancePhaseCancellationJoinsDescendants(t *testing.T) {
	for _, phase := range []string{"database", "boot"} {
		t.Run(phase, func(t *testing.T) {
			listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close() //nolint:errcheck // The test owns this listening socket.
			self, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			t.Setenv("SCALE_LIFETIME_ADDRESS", listener.Addr().String())
			foreignCtx, stopForeign := context.WithCancel(t.Context())
			defer stopForeign()
			foreign := exec.CommandContext(foreignCtx, self, "-test.run=^TestScaleLifetimeProcess$")
			foreign.Env = append(os.Environ(), "SCALE_LIFETIME_ROLE=descendant")
			plugin.KillGroupOnCancel(foreign)
			if err := plugin.StartChild(foreign); err != nil {
				t.Fatal(err)
			}
			defer func() {
				stopForeign()
				if err := foreign.Wait(); err != nil {
					if _, exited := errors.AsType[*exec.ExitError](err); !exited {
						t.Error(err)
					}
				}
			}()
			sentinel := acceptScaleLifetime(t, listener)
			defer sentinel.Close() //nolint:errcheck // The test owns this accepted socket.
			var sentinelPID int
			if err := json.NewDecoder(sentinel).Decode(&sentinelPID); err != nil {
				t.Fatal(err)
			}

			bin := t.TempDir()
			t.Setenv("NATIVE_PHASE_SELF", self)
			t.Setenv("NATIVE_PHASE_LATER", filepath.Join(bin, "later"))
			parent := "#!/bin/sh\nif [ \"$1\" = data ]; then touch \"$NATIVE_PHASE_LATER\"; exit 0; fi\nexport SCALE_LIFETIME_ROLE=parent\nexec \"$NATIVE_PHASE_SELF\" -test.run='^TestScaleLifetimeProcess$'\n"
			for _, name := range []string{"host", "qemu-system-x86_64"} {
				if err := os.WriteFile(filepath.Join(bin, name), []byte(parent), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			// The isolated namespaces contain no pre-existing PPP/L2TP objects.
			if err := os.WriteFile(filepath.Join(bin, "ip"), []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			done := make(chan error, 1)
			go func() {
				if phase == "database" {
					_, err := prepareProofDatabase(ctx, filepath.Join(bin, "host"), bin, "template", io.Discard)
					done <- err
					return
				}
				g := newGokrazyL2TP(bin)
				g.Arch, g.Accel, g.Progress = ArchAMD64, "tcg", io.Discard
				g.BootWait = 30 * time.Second
				_, err := g.observe(ctx, GokrazyL2TPReport{}, "image", bin)
				done <- err
			}()
			owned := acceptScaleLifetime(t, listener)
			defer owned.Close() //nolint:errcheck // The release below may already have closed it.
			var ownedPID int
			if err := json.NewDecoder(owned).Decode(&ownedPID); err != nil {
				t.Fatal(err)
			}
			defer func() {
				owned.Close() //nolint:errcheck // Releasing the descendant; only the join below is asserted.
				cancel()
				select {
				case <-done:
				case <-time.After(5 * time.Second):
					t.Error("production phase did not join after fixture release")
				}
			}()
			cancel()
			select {
			case err := <-done:
				done <- err
				if !errors.Is(err, context.Canceled) {
					t.Errorf("production phase lost originating cancellation: %v", err)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("production phase waited on surviving descendant")
			}
			if err := owned.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
				t.Fatal(err)
			}
			var one [1]byte
			if _, err := owned.Read(one[:]); err == nil {
				t.Error("owned descendant wrote after phase return")
			} else if timeout, ok := errors.AsType[net.Error](err); ok && timeout.Timeout() {
				t.Error("owned descendant survived production phase return")
			}
			if err := sentinel.SetReadDeadline(time.Now().Add(100 * time.Millisecond)); err != nil {
				t.Fatal(err)
			}
			if _, err := sentinel.Read(one[:]); err == nil {
				t.Error("foreign sentinel unexpectedly wrote")
			} else if timeout, ok := errors.AsType[net.Error](err); !ok || !timeout.Timeout() {
				t.Errorf("production phase touched foreign sentinel: %v", err)
			}
			if _, err := os.Stat(filepath.Join(bin, "later")); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("later database phase executed: %v", err)
			}
		})
	}
}
