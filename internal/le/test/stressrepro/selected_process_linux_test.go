//go:build linux

// Design: docs/functional-tests.md -- prepared native execution and joined cleanup.

package teststressrepro

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNativeStressProcessUsesPreparedEnvironment(t *testing.T) {
	root := t.TempDir()
	le := filepath.Join(root, "le")
	body := "#!/bin/sh\nprintf '%s\\n' \"$PATH\" \"$ZE_RUN_JOB\" \"$ZE_BIN\" \"$ZE_TEST_NETNS\" \"$STRESS_PREPARED_ONLY\"\n"
	if err := os.WriteFile(le, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	spec := invocation{
		root: root, harness: []string{"bgp", "plugin", "--draft"}, test: "native-selected",
		testBin: le, zeBin: "/prepared/ze", timeout: 2 * time.Second,
		environment: []string{"PATH=/prepared/bin:/usr/bin", "ZE_RUN_JOB=/prepared/parent", "ZE_BIN=/prepared/ze", "ZE_TEST_NETNS=1", "STRESS_PREPARED_ONLY=retained"},
	}
	result := (realProcessRunner{}).Invoke(context.Background(), spec)
	if !result.succeeded() {
		t.Fatalf("prepared native invocation failed: %+v", result)
	}
	want := "/prepared/bin:/usr/bin\n/prepared/parent\n/prepared/ze\n1\nretained\n"
	if result.output != want {
		t.Fatalf("prepared namespace/job/binary environment lost:\n got %q\nwant %q", result.output, want)
	}
}

func TestNativeStressProcessJoinsDescendantsAfterLeaderExit(t *testing.T) {
	root := t.TempDir()
	listener, socket := stressDescriptorSocket(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	result := make(chan processResult, 1)
	done := make(chan struct{})
	program := stressFixtureProgram(t)
	go func() {
		defer close(done)
		result <- runCommand(ctx, root, os.Environ(), program,
			"-test.run=^TestStressProcessHelper$", "--", "stress-process-fixture", "exit", socket)
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})
	descendant := receiveStressDescriptor(t, ctx, listener)
	t.Cleanup(func() { stopStressPidfd(t, descendant.file) })
	if !stressPidfdLive(t, descendant.file) {
		t.Fatal("controlled descendant exited before the leader was released")
	}
	descendant.acknowledge(t)
	joined := <-result
	if !joined.succeeded() {
		t.Fatalf("controlled leader failed before cleanup: %+v", joined)
	}
	if stressPidfdLive(t, descendant.file) {
		t.Fatal("successful invocation returned while its native descendant still runs")
	}
}

func TestNativeStressProcessCancellationJoinsDescendants(t *testing.T) {
	root := t.TempDir()
	listener, socket := stressDescriptorSocket(t)
	le := filepath.Join(root, "le")
	program := "'" + strings.ReplaceAll(stressFixtureProgram(t), "'", "'\\''") + "'"
	body := "#!/bin/sh\nexec " + program + " '-test.run=^TestStressProcessHelper$' -- stress-process-fixture wait '" + socket + "'\n"
	if err := os.WriteFile(le, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result := make(chan processResult, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		invokeStressCancellation(ctx, invocation{root: root, harness: []string{"bgp", "plugin"}, test: "selected", testBin: le, zeBin: "/prepared/ze", timeout: time.Minute}, result)
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})
	descendant := receiveStressDescriptor(t, ctx, listener)
	t.Cleanup(func() { stopStressPidfd(t, descendant.file) })
	if !stressPidfdLive(t, descendant.file) {
		t.Fatal("controlled descendant exited before cancellation")
	}
	descendant.acknowledge(t)
	cancel()
	joined := <-result
	if joined.code == 0 {
		t.Fatalf("canceled invocation reported success: %+v", joined)
	}
	if stressPidfdLive(t, descendant.file) {
		t.Fatal("canceled invocation returned while its native descendant still runs")
	}
}

func invokeStressCancellation(ctx context.Context, spec invocation, result chan<- processResult) {
	result <- (realProcessRunner{}).Invoke(ctx, spec)
}
