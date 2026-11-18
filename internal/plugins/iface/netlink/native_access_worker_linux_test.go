// Design: docs/features/interfaces.md -- bounded callers and physical native join
//go:build linux

package ifacenetlink

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"

	"github.com/vishvananda/netlink"

	"github.com/ze-software/ze/internal/component/iface"
)

// A caller deadline MUST retain the same executing worker and refuse another
// mutation. Withhold native completion under synctest; release it only after
// observing the caller's timeout and an unsuccessful physical join.
func TestNativeAccessWorkerDeadlineRetainsOriginalOperation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		resources := &nativeVLANResources{ops: &nativeVLANOps{close: func() error { return nil }}}
		entered, resume := make(chan struct{}), make(chan struct{})
		release := sync.OnceFunc(func() { close(resume) })
		foreign := &nativeVLANResources{ops: &nativeVLANOps{close: func() error { return nil }}}
		t.Cleanup(func() {
			release()
			synctest.Wait()
			if err := resources.close(); err != nil {
				t.Error(err)
			}
			if err := foreign.close(); err != nil {
				t.Error(err)
			}
		})
		answer := make(chan error, 1)
		var mutations atomic.Int32
		go func() {
			_, err := resources.run(t.Context(), false, func(context.Context) (any, error) { close(entered); <-resume; mutations.Add(1); return struct{}{}, nil }, nil)
			answer <- err
		}()
		<-entered
		if err := <-answer; !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("bounded original caller: %v", err)
		}
		if mutations.Load() != 0 {
			t.Fatal("withheld native operation already completed")
		}
		if _, err := resources.run(t.Context(), false, func(context.Context) (any, error) { mutations.Add(100); return struct{}{}, nil }, nil); !errors.Is(err, iface.ErrNativeAccessPending) {
			t.Fatalf("unknown original allowed another mutation: %v", err)
		}
		if err := resources.join(t.Context()); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("withheld native IO falsely joined: %v", err)
		}
		if _, err := foreign.run(t.Context(), false, func(context.Context) (any, error) { mutations.Add(1000); return struct{}{}, nil }, nil); !errors.Is(err, iface.ErrNativeAccessPending) {
			t.Fatalf("foreign owner bypassed original mutation gate: %v", err)
		}
		release()
		synctest.Wait()
		if err := resources.join(t.Context()); err != nil {
			t.Fatalf("completed original did not join: %v", err)
		}
		if mutations.Load() != 1 {
			t.Fatalf("native mutation count %d", mutations.Load())
		}
		if _, err := resources.run(t.Context(), true, func(context.Context) (any, error) { return struct{}{}, nil }, nil); err != nil {
			t.Fatal(err)
		}
		if _, err := foreign.run(t.Context(), true, func(context.Context) (any, error) { return struct{}{}, nil }, nil); err != nil {
			t.Fatal(err)
		}
	})
}

// A late successful capture MUST release its unpublished reference exactly
// once. Cancel the original caller before allowing its worker to return it.
func TestNativeAccessWorkerDiscardsLateUnpublishedReference(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		resources := &nativeVLANResources{ops: &nativeVLANOps{close: func() error { return nil }}}
		entered, resume := make(chan struct{}), make(chan struct{})
		release := sync.OnceFunc(func() { close(resume) })
		t.Cleanup(func() {
			release()
			synctest.Wait()
			if err := resources.close(); err != nil {
				t.Error(err)
			}
		})
		ctx, cancel := context.WithCancel(t.Context())
		answer := make(chan error, 1)
		var released atomic.Int32
		original := new(int)
		go func() {
			_, err := resources.run(ctx, false, func(context.Context) (any, error) { close(entered); <-resume; return original, nil }, func(value any) error {
				if value != original {
					t.Error("discard substituted another reference")
				}
				released.Add(1)
				return nil
			})
			answer <- err
		}()
		<-entered
		cancel()
		if err := <-answer; !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled caller: %v", err)
		}
		if released.Load() != 0 {
			t.Fatal("unreturned native reference was released early")
		}
		release()
		synctest.Wait()
		if released.Load() != 1 {
			t.Fatalf("late native reference releases %d", released.Load())
		}
		if _, err := resources.run(t.Context(), true, func(context.Context) (any, error) { return struct{}{}, nil }, nil); err != nil {
			t.Fatal(err)
		}
	})
}

// Unknown parent deletion MUST preserve all original descendant receipts.
// A complete native graph alone grants no ownership of an unmarked foreign
// VLAN. The exact original backend is a positive control for a marked child.
func TestNativeAccessDeleteGraphRefusesUnmarkedForeignChild(t *testing.T) {
	rig := newNativeAccessRig(t)
	binding := rig.create(t, 316)
	parent := rig.links["p0"]
	var links []netlink.Link
	for _, link := range rig.links {
		links = append(links, cloneNativeAccessLink(link))
	}
	qosMapMu.Lock()
	retire, err := rig.backend.nativeVLANs.nativeVLANDeleteSet(parent, links)
	qosMapMu.Unlock()
	if err != nil {
		t.Fatalf("original unleased descendant: %v", err)
	}
	if len(retire) != 1 {
		t.Fatalf("original descendant census %d", len(retire))
	}
	foreign := &nativeVLANResources{}
	qosMapMu.Lock()
	_, err = foreign.nativeVLANDeleteSet(parent, links)
	qosMapMu.Unlock()
	if err == nil {
		t.Fatal("foreign marked descendant allowed a parent cascade")
	}
	for _, link := range links {
		if link.Attrs().Name == binding.OsName {
			link.Attrs().Alias = ""
		}
	}
	qosMapMu.Lock()
	_, err = foreign.nativeVLANDeleteSet(parent, links)
	qosMapMu.Unlock()
	if err == nil {
		t.Fatal("foreign unmarked uncertain descendant allowed a parent cascade")
	}
	if len(rig.backend.nativeVLANs.records) != 1 {
		t.Fatal("refused foreign deletion retired an original receipt")
	}
}
