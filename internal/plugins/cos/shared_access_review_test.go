// Design: docs/architecture/traffic/cos-dynamic.md -- safe shared compensation
// Related: shared_access_test.go -- existing modeled native boundary and events

//go:build ze_l2tp

package cos

import (
	"testing"

	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	coreCos "github.com/ze-software/ze/internal/core/cos"
)

// A committed transaction must remain recoverable after a same-user legacy
// update and its immediate compensation both fail. The native backend is
// healthy again before the original token retries, so a refusal then exposes
// owner-state handling rather than another injected native failure.
func TestCoSLegacySharedCompensationRecoversUncertainLegacyWrite(t *testing.T) {
	f := newSharedCoSTestOwner(t)
	f.up(461, sharedCoSTestFirst)
	f.requireMaps(f.shared)
	token, err := f.command("prepare", 461, sharedCoSTestDifferent, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, operation := range []string{"apply", "commit"} {
		if _, err := f.command(operation, 0, "", token); err != nil {
			t.Fatal(err)
		}
	}
	changed, found := coreCos.Lookup(sharedCoSTestDifferent)
	if !found {
		t.Fatal("the committed control profile disappeared")
	}
	f.requireMaps(changed)

	f.failWrites = 2
	emitCoSChange(t, f.bus, &l2tpevents.SessionCoSChangePayload{
		TunnelID: sharedCoSTestTunnel, SessionID: 461,
		AccessInterface: sharedCoSTestInterface, ProfileName: sharedCoSTestFirst,
	})
	// The legacy implementation makes one attempt; the shared-owner proposal
	// also attempts compensation. End the fault window in either case before
	// asking the already committed owner to recover.
	f.failWrites = 0
	if _, err := f.command("rollback", 0, "", token); err != nil {
		t.Fatalf("the original committed token could not recover after the native fault ended: %v", err)
	}
	f.requireMaps(f.shared)
	f.down(461)
	if _, retained := sessionStore.Load(sessionKey{tunnelID: sharedCoSTestTunnel, sessionID: 461}); retained {
		t.Fatal("successful original compensation left the departed user retained")
	}
}

// Prepare must refuse a native beforeimage that would make compensation
// incompatible with another admitted user. Restoring the healthy native maps
// permits the same request, so unconditional transaction refusal cannot pass.
func TestCoSLegacySharedPrepareRefusesIncompatibleNativeBeforeimage(t *testing.T) {
	f := newSharedCoSTestOwner(t)
	f.up(471, sharedCoSTestFirst)
	f.up(472, sharedCoSTestAlias)
	f.requireMaps(f.shared)
	f.installed = coreCos.Profile{
		IngressMap: map[uint32]uint32{4: 6}, EgressMap: map[uint32]uint32{6: 4},
	}
	if _, err := f.command("prepare", 471, sharedCoSTestFirst, ""); err == nil {
		t.Fatal("prepare accepted native compensation maps that conflict with the surviving user")
	}
	f.installed = coreCos.Profile{
		IngressMap: map[uint32]uint32{2: 5}, EgressMap: map[uint32]uint32{5: 2},
	}
	token, err := f.command("prepare", 471, sharedCoSTestFirst, "")
	if err != nil {
		t.Fatalf("the same request remained refused after native compatibility recovered: %v", err)
	}
	if _, err := f.command("rollback", 0, "", token); err != nil {
		t.Fatal(err)
	}
	f.requireMaps(f.shared)
}
