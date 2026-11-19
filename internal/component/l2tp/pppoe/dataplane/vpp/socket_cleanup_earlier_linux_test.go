// Design: docs/architecture/l2tp/bng-5-pppoe.md -- unpublished cleanup causes
// Related: socket_linux.go -- namespace restoration and descriptor release

//go:build linux

package vpp

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/vishvananda/netns"
	"github.com/ze-software/ze/internal/component/l2tp/pppoe/dataplane"
	"github.com/ze-software/ze/internal/core/report"
)

// The real namespace helper returns nil after failed restoration, even when
// its unpublished endpoint was closed. Cancellation must retain that cause.
func TestVPPControlSocketCanceledRestoreFailureRetainsOriginalCause(t *testing.T) {
	socketCleanupIsolate(t)
	cause := errors.New("original namespace restoration refused")
	fixture := &socketCleanupNamespaceFixture{restoreCause: cause}
	fixture.cancel(t)
	if !errors.Is(fixture.resultErr, cause) {
		t.Fatalf("actual namespace producer lost restoration cause: %v", fixture.resultErr)
	}
	if fixture.resultSocket != nil {
		t.Fatal("failed namespace restoration published an endpoint")
	}
	if fixture.endpoint.closes != 1 {
		t.Fatalf("restoration rollback closes=%d, want 1", fixture.endpoint.closes)
	}
	if fixture.restored {
		t.Fatal("failed restoration marked the namespace restored")
	}
	fixture.requireNamespaceReleases(t)
	issue := socketCleanupIssue(t)
	if !strings.Contains(issue.Message, cause.Error()) {
		t.Fatalf("cancellation lost the original restoration failure: %+v", issue)
	}
}

// A namespace descriptor can fail to close after an ordinary canceled open.
// The native producer's joined cleanup cause must survive a nil socket result.
func TestVPPControlSocketCanceledNamespaceCloseRetainsOriginalCause(t *testing.T) {
	socketCleanupIsolate(t)
	cause := errors.New("original target namespace close refused")
	fixture := &socketCleanupNamespaceFixture{closeCause: cause, openCanceled: true}
	fixture.cancel(t)
	if !errors.Is(fixture.resultErr, context.Canceled) {
		t.Fatalf("actual producer lost original cancellation: %v", fixture.resultErr)
	}
	if !errors.Is(fixture.resultErr, cause) {
		t.Fatalf("actual producer lost original close failure: %v", fixture.resultErr)
	}
	if fixture.resultSocket != nil {
		t.Fatal("canceled native open returned a socket")
	}
	if !fixture.restored {
		t.Fatal("successful namespace restoration was not retained")
	}
	fixture.requireNamespaceReleases(t)
	issue := socketCleanupIssue(t)
	if !strings.Contains(issue.Message, cause.Error()) {
		t.Fatalf("cancellation swallowed joined cleanup failure: %+v", issue)
	}
}

// Cancellation with confirmed native cleanup creates no unresolved episode.
// The same real namespace producer must release all diagnostic capacity.
func TestVPPControlSocketCanceledNativeOpenReleasesUnusedReservation(t *testing.T) {
	socketCleanupIsolate(t)
	fixture := &socketCleanupNamespaceFixture{openCanceled: true}
	fixture.cancel(t)
	if !errors.Is(fixture.resultErr, context.Canceled) {
		t.Fatalf("actual producer did not reach canceled native open: %v", fixture.resultErr)
	}
	if !fixture.restored {
		t.Fatal("successful namespace restoration was not retained")
	}
	fixture.requireNamespaceReleases(t)
	if issues := report.Warnings(); len(issues) != 0 {
		t.Fatalf("confirmed native cleanup invented an unresolved warning: %+v", issues)
	}
	socketCleanupFillReservations(t)
}

// This complete per-attempt port owns only synthetic handles. The real worker
// and namespace cleanup producer run unchanged; no process namespace or native
// descriptor is opened, closed, moved or supplied by this fixture.
type socketCleanupNamespaceFixture struct {
	restoreCause error
	closeCause   error
	openCanceled bool
	endpoint     socketCleanupEndpoint
	resultErr    error
	resultSocket *controlSocket
	restored     bool
	releases     [2]int
}

func (fixture *socketCleanupNamespaceFixture) cancel(t *testing.T) {
	t.Helper()
	const original = netns.NsHandle(101)
	const target = netns.NsHandle(102)
	ctx, cancel := context.WithCancel(t.Context())
	entered := make(chan struct{})
	release := make(chan struct{})
	unblock := sync.OnceFunc(func() { close(release) })
	ops := socketNamespaceOps{
		current: func() (netns.NsHandle, error) { return original, nil },
		target:  func(string) (netns.NsHandle, error) { return target, nil },
		set: func(handle netns.NsHandle) error {
			if handle == original {
				return fixture.restoreCause
			}
			if handle != target {
				t.Errorf("native set addressed another namespace: %d", handle)
			}
			return nil
		},
		close: func(handle netns.NsHandle) error {
			if handle == original {
				fixture.releases[0]++
				return nil
			}
			if handle != target {
				t.Errorf("native close addressed another namespace: %d", handle)
				return errors.New("fixture received another namespace handle")
			}
			fixture.releases[1]++
			return fixture.closeCause
		},
		open: func(ctx context.Context, _ dataplane.Request) (*controlSocket, error) {
			close(entered)
			<-release
			if fixture.openCanceled {
				return nil, ctx.Err()
			}
			return newControlSocket(&fixture.endpoint), nil
		},
	}
	_, done := startControlSocketThread(ctx, socketCleanupRequest(49165),
		func(ctx context.Context, request dataplane.Request, restored *bool) (*controlSocket, error) {
			fixture.resultSocket, fixture.resultErr = openSocketInNamespaceWithOps(ctx, request, restored, ops)
			fixture.restored = *restored
			return fixture.resultSocket, fixture.resultErr
		})
	t.Cleanup(func() {
		cancel()
		unblock()
		socketCleanupWait(t, done, "original namespace cleanup producer")
	})
	socketCleanupWait(t, entered, "original namespace native open")
	cancel()
	unblock()
	socketCleanupWait(t, done, "original namespace cleanup producer")
}

func (fixture *socketCleanupNamespaceFixture) requireNamespaceReleases(t *testing.T) {
	t.Helper()
	if fixture.releases[0] != 1 {
		t.Fatalf("original namespace releases=%d, want 1", fixture.releases[0])
	}
	if fixture.releases[1] != 1 {
		t.Fatalf("target namespace releases=%d, want 1", fixture.releases[1])
	}
}
