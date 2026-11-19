// Design: docs/architecture/l2tp/bng-5-pppoe.md -- canceled native socket custody
// Related: socket_linux.go -- namespace worker ownership handoff

//go:build linux

package vpp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp/pppoe/dataplane"
	"github.com/ze-software/ze/internal/core/report"
)

const socketCleanupCode = "doctor-pppoe-vpp-socket-cleanup"

// TestVPPControlSocketCanceledCleanupRetainsOriginalIssue cancels before the
// opener returns, joins the real worker, and checks its original Close failure.
func TestVPPControlSocketCanceledCleanupRetainsOriginalIssue(t *testing.T) {
	socketCleanupIsolate(t)
	request := socketCleanupRequest(49157)
	cause := errors.New("original endpoint close remains unconfirmed")
	socketCleanupCancel(t, request, cause)
	issue := socketCleanupIssue(t)
	if issue.Source == "" {
		t.Fatal("retained socket cleanup has no source")
	}
	if issue.Subject == "" {
		t.Fatal("retained socket cleanup has no original subject")
	}
	if !strings.Contains(issue.Message, cause.Error()) {
		t.Fatalf("retained issue lost the original cleanup cause: %+v", issue)
	}
	identity := issue.Subject + " " + issue.Message
	for _, coordinate := range []string{request.AccessName, request.Device, request.ControlDevice, request.Namespace} {
		if !strings.Contains(identity, coordinate) {
			t.Errorf("retained socket issue lost request coordinate %q: %+v", coordinate, issue)
		}
	}
	if !strings.Contains(issue.Message, strconv.Itoa(int(request.SessionID))) {
		t.Errorf("retained socket message lost original session ID: %+v", issue)
	}
	mac := fmt.Sprintf("%x", request.PeerMAC)
	if !strings.Contains(strings.ReplaceAll(identity, ":", ""), mac) {
		t.Errorf("retained socket issue lost original peer MAC: %+v", issue)
	}
	if issue.Fix == "" {
		t.Fatal("retained socket cleanup has no repair instruction")
	}
	if issue.Raised.IsZero() {
		t.Fatal("retained socket cleanup has no original raised time")
	}
}

// TestVPPControlSocketCanceledCleanupSurvivesSourceClear uses the acknowledged
// ordinary report paths, then verifies the original complete issue is unchanged.
func TestVPPControlSocketCanceledCleanupSurvivesSourceClear(t *testing.T) {
	socketCleanupIsolate(t)
	socketCleanupCancel(t, socketCleanupRequest(49158), errors.New("original cleanup failure"))
	first := socketCleanupIssue(t)
	if err := report.TryClearWarning(first.Source, first.Code, first.Subject); !errors.Is(err, report.ErrResourceWarningOwned) {
		t.Fatalf("ordinary clear did not refuse original resource custody: %v", err)
	}
	if err := report.TryRaiseWarningFix(first.Source, first.Code, first.Subject, "replacement cause", "replacement fix"); !errors.Is(err, report.ErrResourceWarningOwned) {
		t.Fatalf("ordinary overwrite did not refuse original resource custody: %v", err)
	}
	report.ClearWarning(first.Source, first.Code, first.Subject)
	report.ClearSource(first.Source)
	if got := socketCleanupIssue(t); !reflect.DeepEqual(got, first) {
		t.Fatalf("ordinary cleanup changed original socket issue: before=%+v after=%+v", first, got)
	}
}

// TestVPPControlSocketHealthySuccessKeepsEarlierFailure publishes a different
// useful owner and closes it without clearing the original failed departure.
func TestVPPControlSocketHealthySuccessKeepsEarlierFailure(t *testing.T) {
	socketCleanupIsolate(t)
	socketCleanupCancel(t, socketCleanupRequest(49159), errors.New("earlier endpoint close failed"))
	first := socketCleanupIssue(t)
	endpoint := &socketCleanupEndpoint{}
	opened := socketCleanupPublish(t, socketCleanupRequest(49160), endpoint)
	frame := []byte{0xc0, 0x21, 1, 1}
	n, err := opened.Write(frame)
	if err != nil {
		t.Fatalf("healthy published socket write failed: %v", err)
	}
	if n != len(frame) {
		t.Fatalf("healthy published socket wrote %d bytes, want %d", n, len(frame))
	}
	if !bytes.Equal(endpoint.written, frame) {
		t.Fatal("healthy published socket did not reach its original endpoint")
	}
	if err := opened.Close(); err != nil {
		t.Fatal(err)
	}
	if endpoint.closes != 1 {
		t.Fatalf("healthy endpoint closes=%d, want 1", endpoint.closes)
	}
	if got := socketCleanupIssue(t); !reflect.DeepEqual(got, first) {
		t.Fatalf("healthy successor changed earlier socket issue: before=%+v after=%+v", first, got)
	}
}

// TestVPPControlSocketCanceledCloseSuccessReleasesReservation joins a canceled
// successful Close and proves that every report reservation remains available.
func TestVPPControlSocketCanceledCloseSuccessReleasesReservation(t *testing.T) {
	socketCleanupIsolate(t)
	socketCleanupCancel(t, socketCleanupRequest(49161), nil)
	if entries := report.Warnings(); len(entries) != 0 {
		t.Fatalf("successful canceled cleanup invented a warning: %+v", entries)
	}
	owners := socketCleanupFillReservations(t)
	_, _, capacity := report.SourceWarnings("socket-cleanup-test", "", "")
	if len(owners) != capacity {
		t.Fatalf("successful canceled cleanup retained a slot: available=%d cap=%d", len(owners), capacity)
	}
}

// TestVPPControlSocketPublishedOwnerKeepsDescriptor proves that handoff returns
// the exact live socket and leaves endpoint Close to its receiver.
func TestVPPControlSocketPublishedOwnerKeepsDescriptor(t *testing.T) {
	socketCleanupIsolate(t)
	endpoint := &socketCleanupEndpoint{}
	opened := socketCleanupPublish(t, socketCleanupRequest(49162), endpoint)
	if endpoint.closes != 0 {
		t.Fatal("namespace worker closed the published endpoint")
	}
	if err := opened.Close(); err != nil {
		t.Fatal(err)
	}
	if endpoint.closes != 1 {
		t.Fatalf("receiver endpoint closes=%d, want 1", endpoint.closes)
	}
	if entries := report.Warnings(); len(entries) != 0 {
		t.Fatalf("successful handoff invented a cleanup warning: %+v", entries)
	}
}

// TestVPPControlSocketReservationRefusesBeforeOpening fills the reserved capacity
// before admission, then frees one slot and proves useful admission still works.
func TestVPPControlSocketReservationRefusesBeforeOpening(t *testing.T) {
	socketCleanupIsolate(t)
	owners := socketCleanupFillReservations(t)
	ctx, cancel := context.WithCancel(t.Context())
	endpoint := &socketCleanupEndpoint{}
	opened := newControlSocket(endpoint)
	openCalls := 0
	result, done := startControlSocketThread(ctx, socketCleanupRequest(49163),
		func(_ context.Context, _ dataplane.Request, restored *bool) (*controlSocket, error) {
			*restored = true
			openCalls++
			return opened, nil
		})
	t.Cleanup(func() {
		cancel()
		socketCleanupWait(t, done, "full-capacity worker")
		if err := opened.Close(); err != nil {
			t.Error(err)
		}
	})
	answer := socketCleanupAnswer(t, result)
	socketCleanupWait(t, done, "full-capacity worker")
	if answer.socket != nil {
		t.Error("full diagnostic capacity admitted an unpublished native owner")
	}
	if answer.err == nil {
		t.Error("full diagnostic capacity returned no admission refusal")
	}
	if openCalls != 0 {
		t.Errorf("full diagnostic capacity reached the native opener %d times", openCalls)
	}
	if err := owners[0].Close(); err != nil {
		t.Fatal(err)
	}
	healthy := &socketCleanupEndpoint{}
	accepted := socketCleanupPublish(t, socketCleanupRequest(49164), healthy)
	if err := accepted.Close(); err != nil {
		t.Fatal(err)
	}
	if healthy.closes != 1 {
		t.Fatalf("available-capacity endpoint closes=%d, want 1", healthy.closes)
	}
}

func socketCleanupIsolate(t *testing.T) {
	t.Helper()
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
}

func socketCleanupRequest(id uint16) dataplane.Request {
	return dataplane.Request{AccessName: "access-original", Device: "dp-original",
		ControlDevice: "cp-original", Namespace: "ns-original", SessionID: id,
		PeerMAC: [6]byte{2, 0x11, 0x22, 0x33, 0x44, 0x55}}
}

// socketCleanupCancel owns the test opener's gate until the existing namespace
// worker finishes. It changes no namespace and opens no native descriptor.
func socketCleanupCancel(t *testing.T, request dataplane.Request, failure error) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	entered := make(chan struct{})
	release := make(chan struct{})
	unblock := sync.OnceFunc(func() { close(release) })
	endpoint := &socketCleanupEndpoint{failure: failure}
	opened := newControlSocket(endpoint)
	_, done := startControlSocketThread(ctx, request,
		func(_ context.Context, _ dataplane.Request, restored *bool) (*controlSocket, error) {
			*restored = true
			close(entered)
			<-release
			return opened, nil
		})
	t.Cleanup(func() {
		cancel()
		unblock()
		socketCleanupWait(t, done, "canceled namespace worker")
	})
	socketCleanupWait(t, entered, "gated namespace opener")
	cancel()
	unblock()
	socketCleanupWait(t, done, "canceled namespace worker")
	if endpoint.closes != 1 {
		t.Fatalf("unpublished endpoint closes=%d, want 1", endpoint.closes)
	}
}

func socketCleanupPublish(t *testing.T, request dataplane.Request, endpoint *socketCleanupEndpoint) *controlSocket {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	opened := newControlSocket(endpoint)
	result, done := startControlSocketThread(ctx, request,
		func(_ context.Context, _ dataplane.Request, restored *bool) (*controlSocket, error) {
			*restored = true
			return opened, nil
		})
	t.Cleanup(func() {
		cancel()
		socketCleanupWait(t, done, "published namespace worker")
		if err := opened.Close(); err != nil {
			t.Error(err)
		}
	})
	answer := socketCleanupAnswer(t, result)
	socketCleanupWait(t, done, "published namespace worker")
	if answer.err != nil {
		t.Fatalf("available-capacity socket was refused: %v", answer.err)
	}
	if answer.socket != opened {
		t.Fatal("namespace worker returned another socket owner")
	}
	if endpoint.closes != 0 {
		t.Fatal("namespace worker closed its transferred owner")
	}
	return answer.socket
}

func socketCleanupIssue(t *testing.T) report.Issue {
	t.Helper()
	entries := report.Warnings()
	if len(entries) != 1 {
		t.Fatalf("want one retained socket-cleanup warning after original Close failure; warnings=%+v", entries)
	}
	if entries[0].Code != socketCleanupCode {
		t.Fatalf("original socket cleanup used another diagnostic class: %+v", entries[0])
	}
	return entries[0]
}

func socketCleanupFillReservations(t *testing.T) []*report.ResourceWarning {
	t.Helper()
	_, _, capacity := report.SourceWarnings("socket-cleanup-test", "", "")
	owners := make([]*report.ResourceWarning, 0, capacity)
	t.Cleanup(func() {
		for _, owner := range owners {
			if err := owner.Close(); err != nil {
				t.Error(err)
			}
		}
	})
	for index := range capacity {
		owner, err := report.NewResourceWarning("socket-cleanup-test", strconv.Itoa(index))
		if err != nil {
			t.Fatalf("original canceled/handoff cleanup consumed an unused reservation at %d of %d: %v", index, capacity, err)
		}
		owners = append(owners, owner)
	}
	return owners
}

func socketCleanupWait(t *testing.T, done <-chan struct{}, subject string) {
	t.Helper()
	// The timer bounds a failed test's lifetime; assertions observe joined state.
	timer := time.NewTimer(10 * time.Second)
	defer timer.Stop()
	select {
	case <-done:
	case <-timer.C:
		t.Fatalf("%s did not finish", subject)
	}
}

func socketCleanupAnswer(t *testing.T, result <-chan socketResult) socketResult {
	t.Helper()
	timer := time.NewTimer(10 * time.Second)
	defer timer.Stop()
	select {
	case answer, ok := <-result:
		if !ok {
			t.Fatal("namespace worker ended without an admission result")
		}
		return answer
	case <-timer.C:
		t.Fatal("namespace worker did not answer its waiting receiver")
		return socketResult{}
	}
}

// socketCleanupEndpoint is used by the real worker and controlSocket.Close.
// It owns no native descriptor and is read only after the worker joins.
type socketCleanupEndpoint struct {
	failure error
	closes  int
	written []byte
}

func (*socketCleanupEndpoint) Read([]byte) (int, error) { return 0, io.EOF }
func (e *socketCleanupEndpoint) Write(data []byte) (int, error) {
	e.written = append(e.written, data...)
	return len(data), nil
}
func (*socketCleanupEndpoint) SetReadDeadline(time.Time) error { return nil }
func (e *socketCleanupEndpoint) Close() error                  { e.closes++; return e.failure }
