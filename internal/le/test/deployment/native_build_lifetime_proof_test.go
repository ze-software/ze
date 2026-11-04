//go:build linux || darwin

// Design: docs/functional-tests.md -- the native PPP daemon build lifetime.
package testdeployment

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestNativeHostBuildCancellationJoinsDescendants invokes the actual native
// build producer. A PATH-isolated compiler creates a socket-observed descendant;
// cancellation must join that descendant and preserve its originating cause.
func TestNativeHostBuildCancellationJoinsDescendants(t *testing.T) {
	tree, _ := stageRootedBuildTree(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	body := "#!/bin/sh\nexport SCALE_LIFETIME_ROLE=parent\nexec \"$NATIVE_BUILD_SELF\" -test.run='^TestScaleLifetimeProcess$'\n"
	if err := os.WriteFile(filepath.Join(bin, "go"), []byte(body), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("NATIVE_BUILD_SELF", self)
	t.Setenv("SCALE_LIFETIME_ADDRESS", listener.Addr().String())
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- buildHostDaemon(ctx, tree, "tmp/native-owned-build", io.Discard)
	}()
	owned := acceptScaleLifetime(t, listener)
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
			t.Error("native build did not join after fixture release")
		}
	}()
	cancel()
	select {
	case err := <-done:
		done <- err
		if !errors.Is(err, context.Canceled) {
			t.Errorf("native build lost originating cancellation: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("native build waited on surviving descendant")
	}
	if err := owned.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	var one [1]byte
	if _, err := owned.Read(one[:]); err == nil {
		t.Error("native compiler descendant wrote after build return")
	} else if timeout, ok := err.(net.Error); ok && timeout.Timeout() {
		t.Error("native compiler descendant survived build return")
	}
}
