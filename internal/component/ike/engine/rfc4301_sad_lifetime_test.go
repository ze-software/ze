// VALIDATES: the RFC 4301 Section 4.4.2.1 SAD lifetime item Ze keeps for a Child SA:
// one time interval from the configured lifetime, and the indication of which action
// ends the SA at which moment (the soft time replaces it, the hard time terminates it).
// PREVENTS: a lifetime whose interval is not the one configured, a replace trigger that
// sits on or after the termination, and an SA that stays usable past its interval.

package engine

import (
	"testing"
	"time"
)

// sadLifetimeSeconds is the configured Child SA lifetime the tests build from.
const sadLifetimeSeconds = 3600

// TestRFC4301SADLifetimeIndicatesReplaceBeforeTerminate proves the lifetime Ze attaches
// to an SA holds the configured interval and says which action applies when: inside the
// interval, past the soft time, the SA is due for replacement and not for termination.
// Method: build the lifetime from 3600 s, check its hard time against the clock read
// around the build, then ask both actions at a moment between the soft and hard times.
func TestRFC4301SADLifetimeIndicatesReplaceBeforeTerminate(t *testing.T) {
	// RFC requirement: RFC4301-4.4.2.1-11 positive -- the lifetime built from a configured 3600 s interval ends that interval at its hard (terminate) time, places the soft (replace) time before it, and between the two reports replace due and terminate not due.
	interval := time.Duration(sadLifetimeSeconds) * time.Second
	before := time.Now()
	lifetime := newLifetimeState(sadLifetimeSeconds)
	after := time.Now()
	if lifetime == nil {
		t.Fatal("no lifetime built for a configured interval")
	}
	if lifetime.hardTime.Before(before.Add(interval)) || lifetime.hardTime.After(after.Add(interval)) {
		t.Errorf("hard time %v, want the configured interval %v after the build", lifetime.hardTime, interval)
	}
	if !lifetime.softTime.Before(lifetime.hardTime) {
		t.Fatalf("soft time %v is not before hard time %v: replacement would never precede termination",
			lifetime.softTime, lifetime.hardTime)
	}
	between := lifetime.softTime.Add(lifetime.hardTime.Sub(lifetime.softTime) / 2)
	if !lifetime.softExpired(between) {
		t.Error("between soft and hard time: replacement not due")
	}
	if lifetime.hardExpired(between) {
		t.Error("between soft and hard time: termination due, want replacement only")
	}
}

// TestRFC4301SADLifetimeTerminatesTheSAAtTheEndOfItsInterval proves an SA is never left
// usable past its interval and is never ended before it: at the hard time the lifetime
// reports termination, and before the soft time it reports neither action. Method: build
// the lifetime from 3600 s and ask both actions at the build time and at the hard time.
func TestRFC4301SADLifetimeTerminatesTheSAAtTheEndOfItsInterval(t *testing.T) {
	// RFC requirement: RFC4301-4.4.2.1-11 negative -- the lifetime built from a configured 3600 s interval reports termination due at its hard time, and neither replacement nor termination at the moment it was built.
	start := time.Now()
	lifetime := newLifetimeState(sadLifetimeSeconds)
	if lifetime == nil {
		t.Fatal("no lifetime built for a configured interval")
	}
	if lifetime.softExpired(start) {
		t.Error("at build time: replacement due before the interval ran")
	}
	if lifetime.hardExpired(start) {
		t.Error("at build time: termination due before the interval ran")
	}
	if !lifetime.hardExpired(lifetime.hardTime) {
		t.Error("at the hard time: termination not due, the SA would stay usable past its interval")
	}
}
