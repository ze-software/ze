// Design: docs/architecture/l2tp/subscriber-session-model.md -- refused transfer ownership
// Related: session_control.go -- one immutable admission rendezvous
package ppp

import (
	"reflect"
)

// closeRefusedTransfer consumes a REAL incoming transferred lease, including a
// canceled queued handoff. A duplicate existing lease is never acquired/closed.
// A failed valid cleanup stays in the EXISTING owners index as a cleanup-only
// owner; no admitted Control, protocol worker, completion event or registry is
// fabricated. The rejection event's reason is independent of its actual Close.
func (d *Driver) closeRefusedTransfer(start *StartSession) {
	owner := start.Dataplane
	if sessionResourceMissing(owner) {
		return
	}
	comparable := reflect.ValueOf(owner).Comparable()
	sameLease := comparable && start.Attempt.hasLease(owner)
	validAttempt := sameLease
	if sameLease {
		claimed, _ := start.Attempt.admissionState()
		if claimed {
			return
		}
	}
	if !comparable {
		// Invalid raw input cannot be admitted, but a failed real Close still
		// needs a comparable retained owner. This wrapper grants NO admission
		// authority and forwards only the exact supplied resource.
		wrapped := &refusedUncomparableOwner{SessionHandle: owner}
		if native, ok := owner.(NativeReleaseHandle); ok {
			owner = &refusedNativeOwner{refusedUncomparableOwner: wrapped, NativeReleaseHandle: native}
		} else {
			owner = wrapped
		}
	}
	identity := owner.Identity()
	var birth string
	if native, ok := owner.(NativeReleaseHandle); ok {
		birth = native.NativeReleaseBirthTag()
	}
	cleanup := &pppSession{driver: d, cleanupOnly: true, dataplane: owner, logger: d.logger,
		nativeIdentity: identity, nativeBirthTag: birth, nativeRetired: true}
	if validAttempt {
		cleanup.subscriberID = start.Attempt.SubscriberID()
		cleanup.attempt = start.Attempt
	}
	d.mu.Lock()
	if _, exists := d.owners[owner]; exists {
		d.mu.Unlock()
		if validAttempt {
			start.Attempt.finishUnowned()
		}
		return
	}
	if validAttempt {
		if err := start.Attempt.claim(cleanup, false); err != nil {
			d.mu.Unlock()
			return
		}
	}
	d.owners[owner] = cleanup
	d.mu.Unlock()
	cleanup.closeFiles()
	// No protocol run exists to reconcile here. Release confirmed first
	// success from the index; a failure waits for an explicit retained sweep.
	cleanup.nativeReleaseMu.Lock()
	d.releaseRetiredOwnerLocked(cleanup)
	cleanup.nativeReleaseMu.Unlock()
	if validAttempt {
		start.Attempt.finishRefusal()
	}
	// The retained cleanup owner, not this payload copy, now owns the lease.
	start.Dataplane = nil
	start.Frames = nil
}

// Only malformed, unadmittable raw resources need these private cleanup keys.
// The native variant preserves its real pending/birth capability unchanged.
type refusedUncomparableOwner struct{ SessionHandle }
type refusedNativeOwner struct {
	*refusedUncomparableOwner
	NativeReleaseHandle
}
