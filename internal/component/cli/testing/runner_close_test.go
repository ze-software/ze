// Design: docs/architecture/testing/runner-architecture.md -- editor closure before publication.

package testing

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/cli"
)

// TestEditorModelBeforeStoreClose holds a real writer guard until model shutdown.
// Reversing the runner's close order blocks storage.Close on that guard; the
// bounded rescue releases it and reports the order failure rather than hanging.
// MUTATION: move the store Close defer after the model Close defer.
func TestEditorModelBeforeStoreClose(t *testing.T) {
	const observer = "close-order-proof"
	joined := make(chan error, 1)
	observed := false
	installCloseObserver(t, observer, func(_ Expectation, state State) error {
		model := state.(*headlessModel)
		guard, err := model.store.AcquireLock("test.conf")
		if err != nil {
			return err
		}
		observed = true
		go func() {
			limit := time.NewTimer(3 * time.Second)
			defer limit.Stop()
			select {
			case <-model.stopped:
				_, err := guard.ReadFile("test.conf")
				joined <- errors.Join(err, guard.Release())
			case <-limit.C:
				joined <- errors.Join(errors.New("store close waited for writer before model shutdown"), guard.Release())
			}
		}()
		return nil
	})
	result := RunETContentIn(context.Background(), closeProofContent(observer, "tree"), t.TempDir())
	if !observed {
		t.Fatalf("observer did not acquire writer: %+v", result)
	}
	if err := <-joined; err != nil {
		t.Fatal(err)
	}
	if !result.Passed {
		t.Fatalf("ordered close failed: %+v", result)
	}
	assertCloseSteps(t, result, observer)
}

// TestEditorBackendCloseError invalidates only the test's mmap descriptor after
// all authored assertions. storage.Close -> BlobStore.Close -> unloadBacking
// must return the real EBADF and the runner must retain the assertion trace.
// No store factory, backend stub, or production fault hook is involved.
// MUTATION: discard configStore.Close's returned error in the runner defer.
func TestEditorBackendCloseError(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("owned descriptor fault uses Linux procfs")
	}
	for _, fault := range []bool{false, true} {
		t.Run(strconv.FormatBool(fault), func(t *testing.T) {
			const observer = "backend-close-proof"
			directory := t.TempDir()
			observed := false
			installCloseObserver(t, observer, func(_ Expectation, state State) error {
				model := state.(*headlessModel)
				if _, err := model.store.ReadFile("test.conf"); err != nil {
					return err
				}
				observed = true
				if !fault {
					return nil
				}
				// This is the final step. The runner closes models then the store
				// synchronously, with no file opens between this fault and Close.
				return invalidateOwnedBacking(filepath.Join(directory, backupArtifactName))
			})
			result := RunETContentIn(context.Background(), closeProofContent(observer, "blob"), directory)
			if !observed {
				t.Fatalf("backend observer not reached: %+v", result)
			}
			assertCloseSteps(t, result, observer)
			if !fault {
				if !result.Passed {
					t.Fatalf("healthy backend close: %+v", result)
				}
				return
			}
			if result.Passed {
				t.Fatal("backend close error published a passing result")
			}
			if !strings.Contains(result.Error, "zefs: close:") {
				t.Fatalf("not the backend close producer: %+v", result)
			}
			if !strings.Contains(result.Error, "bad file descriptor") {
				t.Fatalf("lost backend cause: %+v", result)
			}
			if result.TempDir != directory {
				t.Fatalf("lost owner directory: %+v", result)
			}
		})
	}
}

// TestEditorTranscriptCloseError exercises the actual model-close error producer
// with an already-closed transcript, without inventing a model error hook.
func TestEditorTranscriptCloseError(t *testing.T) {
	const observer = "transcript-close-proof"
	file, err := os.Create(filepath.Join(t.TempDir(), "transcript"))
	if err != nil {
		t.Fatal(err)
	}
	installCloseObserver(t, observer, func(_ Expectation, state State) error {
		model := state.(*headlessModel)
		model.model.SetTranscript(cli.NewTranscriptWriter(file, "fixture", "local"))
		return file.Close()
	})
	result := RunETContentIn(context.Background(), closeProofContent(observer, "tree"), t.TempDir())
	assertCloseSteps(t, result, observer)
	if result.Passed {
		t.Fatal("model close error published a passing result")
	}
	if !strings.Contains(result.Error, "file already closed") {
		t.Fatalf("lost transcript close error: %+v", result)
	}
}

func closeProofContent(observer, mode string) string {
	return "tmpfs=test.conf:terminator=END\nbgp { router-id 1.2.3.4; }\nEND\noption=file:path=test.conf\noption=session:user=close-proof\noption=storage:value=" + mode + "\nexpect=dirty:false\nexpect=" + observer + "\n"
}

func installCloseObserver(t *testing.T, name string, fn func(Expectation, State) error) {
	t.Helper()
	previous, exists := validExpectationTypes[name]
	validExpectationTypes[name] = fn
	t.Cleanup(func() {
		if exists {
			validExpectationTypes[name] = previous
			return
		}
		delete(validExpectationTypes, name)
	})
}

func assertCloseSteps(t *testing.T, result *TestResult, observer string) {
	t.Helper()
	if len(result.Steps) != 2 {
		t.Fatalf("lost assertions: %+v", result)
	}
	if result.Steps[0].Assert != "dirty" {
		t.Fatalf("lost dirty assertion: %+v", result)
	}
	if result.Steps[1].Assert != observer {
		t.Fatalf("lost observer: %+v", result)
	}
	for _, step := range result.Steps {
		if !step.Passed {
			t.Fatalf("failure happened during assertions, not closure: %+v", result)
		}
	}
}

// invalidateOwnedBacking identifies only this test's exact artifact in this
// process, never a foreign descriptor. The real backend still owns its os.File
// and must observe the invalid descriptor when it closes the mmap backing.
func invalidateOwnedBacking(path string) error {
	entries, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		return err
	}
	for _, entry := range entries {
		target, err := os.Readlink(filepath.Join("/proc/self/fd", entry.Name()))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		if target != path {
			continue
		}
		descriptor, err := strconv.Atoi(entry.Name())
		if err != nil {
			return err
		}
		return syscall.Close(descriptor)
	}
	return fmt.Errorf("test backing descriptor not found: %s", path)
}
