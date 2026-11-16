// Design: docs/labs/pppoe-interop.md -- the native build action owns cancellation.
// VALIDATES: SIGINT and SIGTERM reach the actual registered run action's signal
// root, cancel its builder, and leave the owned retention cleanup uncancelled.
// METHOD: signal an isolated copy of this test binary only after its fake image
// builder reports readiness. Neither a Docker daemon nor another process is used.
package buildvpp

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

const signalFixture = "ZE_VPP_BUILD_SIGNAL_FIXTURE"

func TestVPPBuildRunHandlesSignals(t *testing.T) {
	if os.Getenv(signalFixture) == "entry" {
		newImageBuilder = func() imageBuilder { return signalImageBuilder{} }
		payload, code := Answer([]string{"run"})
		if code != 0 {
			t.Fatalf("registered run action exited %d", code)
		}
		report, ok := payload.(Report)
		if !ok {
			t.Fatalf("registered action returned %T, want Report", payload)
		}
		if report.Image.Reference != "sha256:signal-fixture" {
			t.Fatalf("registered action lost the fixture image: %#v", report)
		}
		return
	}
	for _, stop := range []syscall.Signal{syscall.SIGINT, syscall.SIGTERM} {
		t.Run(stop.String(), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
			defer cancel()
			self, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			command := exec.CommandContext(ctx, self, "-test.run=^TestVPPBuildRunHandlesSignals$")
			command.Env = append(os.Environ(), signalFixture+"=entry")
			command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
			var stdout bytes.Buffer
			command.Stdout = &stdout
			stderr, err := command.StderrPipe()
			if err != nil {
				t.Fatal(err)
			}
			defer stderr.Close() //nolint:errcheck // Read-only pipe owned by this fixture.
			if err := command.Start(); err != nil {
				t.Fatal(err)
			}
			reaped := false
			defer func() {
				if reaped {
					return
				}
				if err := command.Process.Kill(); err != nil {
					if !errors.Is(err, os.ErrProcessDone) {
						t.Error(err)
					}
				}
				if err := command.Wait(); err != nil {
					t.Logf("owned fixture reaped during failure cleanup: %v", err)
				}
			}()
			scanner := bufio.NewScanner(stderr)
			awaitSignalMarker(t, scanner, "VPP_BUILD_READY")
			if err := command.Process.Signal(stop); err != nil {
				t.Fatal(err)
			}
			awaitSignalMarker(t, scanner, "VPP_BUILD_CANCELED")
			awaitSignalMarker(t, scanner, "VPP_BUILD_RELEASED")
			if err := command.Wait(); err != nil {
				reaped = true
				t.Fatalf("registered action failed after %s: %v, stdout %s", stop, err, stdout.String())
			}
			reaped = true
		})
	}
}

// Each fixture emits exactly three markers. The parent's process context bounds
// a missing marker; failure cleanup MUST kill and reap only this owned child.
func awaitSignalMarker(t *testing.T, scanner *bufio.Scanner, expected string) {
	t.Helper()
	if !scanner.Scan() {
		t.Fatalf("registered build did not emit %s: %v", expected, scanner.Err())
	}
	if scanner.Text() != expected {
		t.Fatalf("registered build emitted %q, want %q", scanner.Text(), expected)
	}
}

// signalImageBuilder is used only in the isolated fixture. Its blocked Build
// MUST end through the registered action's context; the parent's deadline kills
// the child if that contract breaks. A completed-export race then requires the
// real buildImage path to call ReleaseImage with an uncancelled context.
type signalImageBuilder struct{}

func (signalImageBuilder) Build(ctx context.Context, _ interoplab.ImageBuild) (interoplab.ImageResult, error) {
	if _, err := fmt.Fprintln(os.Stderr, "VPP_BUILD_READY"); err != nil {
		return interoplab.ImageResult{}, err
	}
	<-ctx.Done()
	if _, err := fmt.Fprintln(os.Stderr, "VPP_BUILD_CANCELED"); err != nil {
		return interoplab.ImageResult{}, err
	}
	return interoplab.ImageResult{
		Name: ImageName, Tag: ImageName, Reference: "sha256:signal-fixture", Built: true,
	}, nil
}

func (signalImageBuilder) ReleaseImage(ctx context.Context, _ interoplab.ImageResult) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if ctx.Done() != nil {
		return errors.New("retention cleanup still carries a cancelable context")
	}
	_, err := fmt.Fprintln(os.Stderr, "VPP_BUILD_RELEASED")
	return err
}
