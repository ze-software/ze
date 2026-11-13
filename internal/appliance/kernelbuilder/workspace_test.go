// Design: docs/architecture/appliance/kernel-profiles.md -- worker workspace ownership.

package kernelbuilder

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

// TestRunWorkerConcurrentPatchedBuildsKeepLiveTree starts two actual workers
// against one build volume. A make boundary holds the first worker alive while
// the second extracts and exits; its extraction and cleanup must preserve the
// first worker's source marker. Only make and config merging are substituted.
func TestRunWorkerConcurrentPatchedBuildsKeepLiveTree(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	req := workspaceRequest(t)
	firstOutput := req.OutputDir
	firstTree := make(chan string, 1)
	secondTree := make(chan string, 1)
	release := make(chan struct{})
	done := make(chan error, 1)
	boundary := errors.New("fixture stopped at kernel make boundary")
	old := runWorkerCommand
	runWorkerCommand = func(ctx context.Context, req WorkerRequest, dir, name string, args ...string) error {
		if name == "tar" {
			return old(ctx, req, dir, name, args...)
		}
		if name != "make" {
			return nil
		}
		if !slices.Contains(args, "bzImage") {
			return nil
		}
		if req.OutputDir == firstOutput {
			if err := os.WriteFile(filepath.Join(dir, "live-worker"), []byte("first worker"), 0o600); err != nil {
				return err
			}
			firstTree <- dir
			select {
			case <-release:
				return boundary
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		secondTree <- dir
		return boundary
	}
	t.Cleanup(func() { runWorkerCommand = old })
	go workspaceRun(ctx, req, done)
	var first string
	joined := false
	defer func() {
		close(release)
		if joined {
			return
		}
		select {
		case err := <-done:
			if !errors.Is(err, boundary) {
				t.Errorf("first worker result = %v, want make boundary", err)
			}
			if first != "" {
				if _, err := os.Stat(first); !os.IsNotExist(err) {
					t.Errorf("first worker retained its extracted tree after failure: %v", err)
				}
			}
		case <-ctx.Done():
			t.Error("first worker did not join after release")
		}
	}()
	select {
	case first = <-firstTree:
	case err := <-done:
		joined = true
		t.Fatalf("first worker failed before make: %v", err)
	case <-ctx.Done():
		t.Fatal("first worker did not reach make")
	}
	req.OutputDir = filepath.Join(filepath.Dir(firstOutput), "second")
	if err := RunWorker(ctx, req); !errors.Is(err, boundary) {
		t.Fatalf("second worker result = %v, want make boundary", err)
	}
	second := <-secondTree
	if first == second {
		t.Errorf("concurrent workers share mutable source tree %s", first)
	}
	marker, err := os.ReadFile(filepath.Join(first, "live-worker"))
	if err != nil {
		t.Errorf("second extraction/teardown removed live first-worker source: %v", err)
	}
	if string(marker) != "first worker" {
		t.Errorf("live first-worker marker = %q, want first worker", marker)
	}
	if _, err := os.Stat(second); !os.IsNotExist(err) {
		t.Errorf("second worker retained its extracted tree after failure: %v", err)
	}
}

func workspaceRun(ctx context.Context, req WorkerRequest, done chan<- error) {
	done <- RunWorker(ctx, req)
}

func workspaceRequest(t *testing.T) WorkerRequest {
	t.Helper()
	root := t.TempDir()
	work := filepath.Join(root, "work")
	version := pinnedKernelVersion(t)
	plain := "linux-" + version
	writeFixture(t, root, filepath.Join("source", plain, "scripts", "Kbuild.include"), "fixture\n")
	writeFixture(t, root, filepath.Join("source", plain, ".config"), "CONFIG_FIXTURE=y\n")
	writeFixture(t, root, "profile.config", "CONFIG_FIXTURE=y\n")
	writeFixture(t, root, "profile.require", "CONFIG_FIXTURE\n")
	writeFixture(t, root, "patches/series", "# Active patch directory; no fixture C changes.\n")
	if err := os.MkdirAll(work, 0o750); err != nil {
		t.Fatal(err)
	}
	tarball := filepath.Join(work, kernelTarballName(version))
	cmd := exec.CommandContext(t.Context(), "tar", "-cJf", tarball, "-C", filepath.Join(root, "source"), plain)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("create tiny kernel tarball: %v: %s", err, output)
	}
	data, err := os.ReadFile(tarball)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(data)
	return WorkerRequest{
		Version: version, SourceSHA256: hex.EncodeToString(digest[:]),
		Arch: archAMD64, Profile: "runtime", Modules: modulesYes, Jobs: "1",
		WorkDir: work, BuildDir: filepath.Join(root, "build"),
		OutputDir:  filepath.Join(root, "output", "first"),
		PatchesDir: filepath.Join(root, "patches"),
		Fragments:  []string{filepath.Join(root, "profile.config")},
		Stdout:     io.Discard, Stderr: io.Discard,
	}
}
