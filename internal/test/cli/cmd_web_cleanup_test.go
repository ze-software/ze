// Design: docs/architecture/testing/runner-architecture.md -- server cleanup errors.
package cli

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	webtesting "github.com/ze-software/ze/internal/component/web/testing"
	"github.com/ze-software/ze/internal/test/runner"
)

// TestWebServerCleanupReportsDirectoryFailure gives cleanup an owned non-directory
// parent so RemoveAll fails, rather than silently losing the cleanup diagnostic.
func TestWebServerCleanupReportsDirectoryFailure(t *testing.T) {
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, []byte("owned"), 0o600); err != nil {
		t.Fatal(err)
	}
	srv := &zeTestWebServer{tempDir: filepath.Join(file, "child")}
	if err := srv.stop(); err == nil || !strings.Contains(err.Error(), srv.tempDir) {
		t.Fatalf("cleanup lost the failed directory: %v", err)
	}
}

// webSeedStageContext cancels on observing the real store's private stage. It
// changes no storage result: the callback still runs against the real store and
// must reject cancellation before writing credentials. Err observes the stage
// before context.Cause's Go 1.27 fast path checks whether cancellation occurred.
type webSeedStageContext struct {
	context.Context
	dir    string
	cancel context.CancelFunc
	t      *testing.T
}

func (c webSeedStageContext) Err() error {
	entries, err := os.ReadDir(c.dir)
	if err != nil {
		c.t.Error(err)
		c.cancel()
	}
	for _, entry := range entries {
		if entry.IsDir() {
			c.cancel()
		}
	}
	return c.Context.Err()
}

// TestWebSeedCancellationAtPopulation observes the actual private stage before
// requesting cancellation; the storage callback must refuse and discard it.
func TestWebSeedCancellationAtPopulation(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	observed := webSeedStageContext{Context: ctx, dir: dir, cancel: cancel, t: t}
	if err := zeTestSeedWebUsers(observed, dir, true, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("population ignored cancellation at its private stage: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			t.Fatalf("canceled population retained or published a tree: %s", entry.Name())
		}
	}
}

// TestWebServerCleanupJoinsBoth exercises the primary and auxiliary owners with
// real children and checks directory release only after both Wait results exist.
func TestWebServerCleanupJoinsBoth(t *testing.T) {
	dir := t.TempDir()
	primary := exec.CommandContext(context.Background(), "/bin/sh", "-c", "exec sleep 60")
	auxiliary := exec.CommandContext(context.Background(), "/bin/sh", "-c", "exec sleep 60")
	srv := &zeTestWebServer{tempDir: filepath.Join(dir, "owned")}
	if err := os.Mkdir(srv.tempDir, 0o700); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := srv.stop(); err != nil {
			t.Error(err)
		}
	}()
	var err error
	srv.cmd, err = runner.StartServerProcess(primary)
	if err != nil {
		t.Fatal(err)
	}
	srv.aux, err = runner.StartServerProcess(auxiliary)
	if err != nil {
		t.Fatal(err)
	}
	if err := srv.stop(); err != nil {
		t.Fatal(err)
	}
	if primary.ProcessState == nil || auxiliary.ProcessState == nil {
		t.Fatal("server cleanup returned before joining both owners")
	}
	if _, err := os.Stat(srv.tempDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("server cleanup retained its directory: %v", err)
	}
}

// TestWebStartupFailureRoutes runs the actual startup functions with unavailable
// executables. The LG second-start failure must also join its successfully started peer.
func TestWebStartupFailureRoutes(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	peer := filepath.Join(t.TempDir(), "owned-peer")
	if err := os.WriteFile(peer, []byte("#!/bin/sh\nexec sleep 60\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, start := range []func() (*zeTestWebServer, error){
		func() (*zeTestWebServer, error) {
			return zeTestStartLGServer(context.Background(), zeTestWebBinaries{ze: missing, zeTest: missing}, "127.0.0.1:1", 2, nil)
		},
		func() (*zeTestWebServer, error) {
			return zeTestStartLGServer(context.Background(), zeTestWebBinaries{ze: missing, zeTest: peer}, "127.0.0.1:1", 2, nil)
		},
		func() (*zeTestWebServer, error) {
			return zeTestStartLGNoEngineServer(context.Background(), zeTestWebBinaries{zeTest: missing}, "127.0.0.1:1", nil)
		},
		func() (*zeTestWebServer, error) {
			return zeTestStartChaosServer(context.Background(), zeTestWebBinaries{chaos: missing}, "127.0.0.1:1", nil)
		},
		func() (*zeTestWebServer, error) {
			return zeTestStartWebServer(context.Background(), missing, "127.0.0.1:1", true, nil, nil)
		},
		func() (*zeTestWebServer, error) {
			return zeTestStartDaemonWebServer(context.Background(), missing, "127.0.0.1:1", nil, nil)
		},
	} {
		srv, err := start()
		if srv != nil {
			if stopErr := srv.stop(); stopErr != nil {
				t.Error(stopErr)
			}
			t.Fatal("failed startup published a server")
		}
		if err == nil || !strings.Contains(err.Error(), missing) {
			t.Fatalf("startup lost its missing executable: %v", err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := zeTestStartDaemonWebServer(ctx, missing, "127.0.0.1:1", nil, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("daemon startup lost cancellation: %v", err)
	}
	if err := zeTestSeedWebUsers(ctx, t.TempDir(), true, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("seeding lost cancellation: %v", err)
	}
}

// TestWebCanceledStartupDoesNotSeed makes bcrypt reject the input if it runs;
// an already canceled startup must return cancellation before that synchronous work.
func TestWebCanceledStartupDoesNotSeed(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	users := []webtesting.WBAuthUser{{Name: "owned", Password: strings.Repeat("x", 73)}}
	srv, err := zeTestStartWebServer(ctx, "/no-such-owned-server", "127.0.0.1:1", false, users, nil)
	if srv != nil {
		t.Fatal("canceled startup published a server")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled startup performed seeding instead: %v", err)
	}
}
