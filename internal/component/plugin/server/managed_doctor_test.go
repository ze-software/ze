package server

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	plugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/report"
)

// TestManagedListenerUnboundRecorded is AC-25's producer: a bind failure is
// recorded with its address and the kernel's error text, a failure on the
// plugin acceptor's own address is not (the local collision check reports it),
// and the record is published until Stop withdraws it. Before managed_doctor.go
// Start only logged the failure, so doctor had nothing to read.
func TestManagedListenerUnboundRecorded(t *testing.T) {
	s := &ManagedServer{acceptor: plugin.HubServerConfig{Host: "127.0.0.1", Port: 1790}}
	s.bindFailed("127.0.0.1:1790", errors.New("bind: address already in use"))
	s.bindFailed("192.0.2.7:1791", errors.New("bind: cannot assign requested address"))

	publishManagedDoctor(s)
	record, err := ManagedDoctorSnapshot()
	if err != nil {
		t.Fatalf("snapshot of a published server: %v", err)
	}
	if len(record.Unbound) != 1 {
		t.Fatalf("unbound = %+v, want only the non-colliding address", record.Unbound)
	}
	if got := record.Unbound[0]; got.Addr != "192.0.2.7:1791" || got.Error != "bind: cannot assign requested address" {
		t.Errorf("unbound row = %+v", got)
	}

	withdrawManagedDoctor(s)
	if _, err := ManagedDoctorSnapshot(); !errors.Is(err, ErrManagedNotServing) {
		t.Errorf("after withdraw: %v, want ErrManagedNotServing", err)
	}
}

// TestManagedClientRejectionRecorded is AC-26's producer: the last refusal of
// each client is kept with its cause and a bounded detail, and a later
// successful config ack clears it.
func TestManagedClientRejectionRecorded(t *testing.T) {
	var d managedDoctorState
	d.recordRejected("edge-2", ManagedRejectConfigRejected, string(make([]byte, 2*managedDetailMax)))
	d.recordRejected("edge-1", ManagedRejectNameConnected, "client edge-1 already connected")

	record := d.snapshot()
	if len(record.Rejected) != 2 || record.Rejected[0].Client != "edge-1" {
		t.Fatalf("rejections = %+v, want edge-1 then edge-2", record.Rejected)
	}
	if record.Rejected[0].Cause != ManagedRejectNameConnected {
		t.Errorf("edge-1 cause = %d", record.Rejected[0].Cause)
	}
	if len(record.Rejected[1].Detail) != managedDetailMax {
		t.Errorf("detail of %d bytes kept, bound is %d", len(record.Rejected[1].Detail), managedDetailMax)
	}

	d.clearRejected("edge-2")
	if record := d.snapshot(); len(record.Rejected) != 1 {
		t.Errorf("after a successful ack: %+v", record.Rejected)
	}
}

// TestManagedNotifyDroppedRaises is AC-27: a config-changed notification a
// connected client is not given raises doctor-managed-client-notify-dropped on
// the report bus, naming the client and the cause, with a fix. Before, it was
// a Warn line with no code that doctor never saw.
//
// Method: a real server and a connected client; the hub's read of that
// client's config is made to fail, so the notification cannot be built, and
// the change is announced. A full queue was the cause this test used to
// provoke: the queue is gone, a change is now held until it is pushed
// (TestManagedServeNotifyBurstDoesNotStarve), so the cause left to provoke on
// the hub's own side is the build.
func TestManagedNotifyDroppedRaises(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)

	var unreadable atomic.Bool
	srv, err := NewManagedServer(ManagedServerConfig{
		Authority: newTestCA(t),
		Listeners: []ManagedListener{{Addr: "127.0.0.1:0", Clients: map[string]string{testClientName: testClientSecret}}},
		ReadConfig: func(string) ([]byte, error) {
			if unreadable.Load() {
				return nil, errors.New("the store is unreadable")
			}
			return []byte(testClientConfig), nil
		},
	})
	if err != nil {
		t.Fatalf("NewManagedServer: %v", err)
	}
	if startErr := srv.Start(context.Background()); startErr != nil {
		t.Fatalf("Start: %v", startErr)
	}
	t.Cleanup(srv.Stop)
	mc := dialTestClient(t, srv.Addrs()[0].String(), testClientName, testClientSecret)
	_ = fetch(t, mc, "")
	waitForConnected(t, srv, testClientName, true)

	unreadable.Store(true)
	srv.NotifyConfigChanged(testClientName)

	got := waitForNotifyDropped(t, testClientName)
	if issues := report.Errors(0); len(issues) != 1 {
		t.Errorf("issues = %+v, want one", issues)
	}
	if got.Fix == "" {
		t.Error("the event carries no fix")
	}
	if !strings.Contains(got.Message, "building the notification failed") {
		t.Errorf("message %q does not name the cause", got.Message)
	}
}

// waitForNotifyDropped polls the report bus until it holds a notify-dropped
// event for client, and returns it. The notify worker raises it on its own
// goroutine, so the test waits for the state rather than for a time.
func waitForNotifyDropped(t *testing.T, client string) report.Issue {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		for _, issue := range report.Errors(0) {
			if issue.Code == CodeDoctorManagedClientNotifyDropped && issue.Subject == client {
				return issue
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("no notify-dropped event for %q on the report bus: %+v", client, report.Errors(0))
	return report.Issue{}
}
