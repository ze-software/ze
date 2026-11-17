//go:build linux

// Design: docs/architecture/l2tp/bng-5-pppoe.md -- contained post-PADS ownership
package pppoe

import (
	"testing"
	"testing/synctest"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/core/taskguard"
)

// TestCorePPPoEPanicWaitsForRealDown invokes the actual containment task's
// post-PADS cleanup for an already queued/admitted original attempt.
func TestCorePPPoEPanicWaitsForRealDown(t *testing.T) {
	swapPPPoEPanics(t)
	synctest.Test(t, func(t *testing.T) {
		fixture := newCoreRetentionFixture(t, 7)
		session, _, control := fixture.admit(t, 82)
		opened := session.Opened
		fixture.server.admitting = admission{session: session, padsSent: true}
		func() {
			defer pppoePanics().discovery.Contain(func() taskguard.Context { return taskguard.Context{Unit: "queued post-PADS admission"} }, func(*taskguard.Panic) { fixture.server.endAdmissionAfterPanic() })
			panic("post-PADS retained-worker proof")
		}()
		synctest.Wait()
		assertCoreAdmissionHeld(t, fixture, session, opened)
		if fixture.frames.Load() != 1 {
			t.Fatal("post-PADS panic did not publish exactly one PADT")
		}
		down, ok := fixture.next(t).(ppp.EventSessionDown)
		if !ok || down.Control != control {
			t.Fatal("panic cleanup lost its original terminal event")
		}
		fixture.sub.handlePPPEvent(down)
		select {
		case <-session.closeDone:
		default:
			t.Fatal("panic cleanup never completed its original admission")
		}
		if fixture.frames.Load() != 1 {
			t.Fatal("late Down duplicated panic PADT")
		}
	})
}
