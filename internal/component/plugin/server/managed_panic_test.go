package server

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/crashlog"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/pkg/fleet"
)

// managedGuardLines counts the managed client guard's ERROR lines in the log ring.
func managedGuardLines(t *testing.T) int {
	t.Helper()
	entries, err := slogutil.GlobalLogRing().Snapshot(0, "ERROR", "hub.managed-server")
	if err != nil {
		t.Fatalf("log ring snapshot: %v", err)
	}
	lines := 0
	for _, e := range entries {
		if strings.Contains(e.Message, "an internal Ze defect ended this unit") {
			lines++
		}
	}
	return lines
}

// TestManagedServerPanicContained: a panic while the managed server serves one
// client (ManagedServer.handleConn, through a config reader that panics for
// that client's config-fetch) ends that client's connection only: its call
// fails, its name is released, and it can connect again. A second client
// connected beside it keeps fetching its config. The panic is counted on the
// real guard, logged and crash-recorded. Without the boundary the panic ends
// the test binary.
func TestManagedServerPanicContained(t *testing.T) {
	t.Cleanup(crashlog.SetCrashDirForTest(t.TempDir()))
	beforeCount, declared := managedPanics().guard.Panics(taskManagedClient)
	if !declared {
		t.Fatalf("task %q is not declared on the managed guard", taskManagedClient)
	}
	beforeLines := managedGuardLines(t)

	srv, err := NewManagedServer(ManagedServerConfig{
		Authority: newTestCA(t),
		Addrs:     []string{"127.0.0.1:0"},
		ClientSecrets: map[string]string{
			testClientName:  testClientSecret,
			testClient2Name: testClient2Secret,
		},
		ReadConfig: func(name string) ([]byte, error) {
			if name == testClientName {
				panic("boom: config reader defect")
			}
			return []byte(testClient2Config), nil
		},
	})
	if err != nil {
		t.Fatalf("NewManagedServer: %v", err)
	}
	if err := srv.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(srv.Stop)
	addr := srv.Addrs()[0].String()

	broken := dialTestClient(t, addr, testClientName, testClientSecret)
	healthy := dialTestClient(t, addr, testClient2Name, testClient2Secret)
	waitForConnected(t, srv, testClientName, true)
	waitForConnected(t, srv, testClient2Name, true)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if _, callErr := broken.CallRPC(ctx, fleet.VerbConfigFetch, fleet.ConfigFetchRequest{}); callErr == nil {
		t.Fatal("config-fetch on the panicking client answered; its connection should have closed")
	} else if errors.Is(callErr, context.DeadlineExceeded) {
		t.Fatal("config-fetch on the panicking client hung: the connection was left open")
	}
	waitForConnected(t, srv, testClientName, false)

	if resp := fetch(t, healthy, ""); resp.Config == "" {
		t.Errorf("healthy client fetch answered no config: %+v", resp)
	}
	// The released name is accepted again: the client's reconnect works.
	dialTestClient(t, addr, testClientName, testClientSecret)
	waitForConnected(t, srv, testClientName, true)

	afterCount, _ := managedPanics().guard.Panics(taskManagedClient)
	if afterCount != beforeCount+1 {
		t.Errorf("managed-client panics = %d, want %d: the panic was not counted", afterCount, beforeCount+1)
	}
	if lines := managedGuardLines(t); lines != beforeLines+1 {
		t.Errorf("guard ERROR lines = %d, want %d: the panic was not logged", lines, beforeLines+1)
	}
	crashes := crashlog.ListCrashes()
	if len(crashes) != 1 || crashes[0].Kind != crashlog.KindRecovered {
		t.Errorf("crash records = %+v, want one %q record", crashes, crashlog.KindRecovered)
	}
}
