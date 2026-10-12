package bgp

import (
	"strings"
	"testing"
)

// bfdProfileEditCheckerBranches drives both polarities of every predicate the
// bgp-bfd-profile-edit-live-frr checker decides with, over FRR JSON shaped as
// FRR 10.3 prints it, so no container is needed.
func bfdProfileEditCheckerBranches(t *testing.T) {
	const before = `{"multihop":false,"peer":"172.30.0.2","id":11,"remote-id":22,"status":"up",` +
		`"remote-receive-interval":300,"remote-transmit-interval":300,"remote-detect-multiplier":3}`
	const after = `{"multihop":false,"peer":"172.30.0.2","id":11,"remote-id":22,"status":"up",` +
		`"remote-receive-interval":200,"remote-transmit-interval":200,"remote-detect-multiplier":5}`
	const counters = `{"peer":"172.30.0.2","control-packet-input":40,"session-up":1,"session-down":0}`

	if err := bfdRemoteTimersMatch(before, bfdProfileEditBefore); err != nil {
		t.Fatalf("configured timers refused: %v", err)
	}
	if err := bfdRemoteTimersMatch(after, bfdProfileEditAfter); err != nil {
		t.Fatalf("edited timers refused: %v", err)
	}
	if err := bfdRemoteTimersMatch(before, bfdProfileEditAfter); err == nil {
		t.Fatal("the old timers passed as the edited ones")
	}
	onlyInterval := strings.Replace(before, `"remote-transmit-interval":300`, `"remote-transmit-interval":200`, 1)
	onlyInterval = strings.Replace(onlyInterval, `"remote-receive-interval":300`, `"remote-receive-interval":200`, 1)
	if err := bfdRemoteTimersMatch(onlyInterval, bfdProfileEditAfter); err == nil {
		t.Fatal("an unchanged detect multiplier passed as the edited profile")
	}
	if err := bfdRemoteTimersMatch(strings.Replace(after, `"status":"up"`, `"status":"down"`, 1), bfdProfileEditAfter); err == nil {
		t.Fatal("a down session passed")
	}
	if err := bfdRemoteTimersMatch(`{"status":"up"}`, bfdProfileEditBefore); err == nil {
		t.Fatal("JSON missing the remote timers passed")
	}
	if err := bfdRemoteTimersMatch("not json", bfdProfileEditBefore); err == nil {
		t.Fatal("non-JSON output passed")
	}

	identity, err := parseBFDPeerIdentity(before, counters)
	if err != nil {
		t.Fatalf("parse identity: %v", err)
	}
	if identity.localDiscriminator != 11 || identity.remoteDiscriminator != 22 || identity.sessionDowns != 0 {
		t.Fatalf("identity = %+v, want discriminators 11/22 and no Down", identity)
	}
	if _, err := parseBFDPeerIdentity(before, `{"session-up":1}`); err == nil {
		t.Fatal("counters without session-down were read as zero Downs")
	}
	if _, err := parseBFDPeerIdentity(`{"status":"up","id":11}`, counters); err == nil {
		t.Fatal("a peer without remote-id was read")
	}

	identity.bgpGeneration = 1
	if err := requireBFDProfileEditContinuity(identity, identity); err != nil {
		t.Fatalf("an unchanged identity was refused: %v", err)
	}
	for name, mutate := range map[string]func(*bfdPeerIdentity){
		"FRR discriminator":  func(i *bfdPeerIdentity) { i.localDiscriminator++ },
		"ze discriminator":   func(i *bfdPeerIdentity) { i.remoteDiscriminator++ },
		"BFD Down counted":   func(i *bfdPeerIdentity) { i.sessionDowns++ },
		"BGP re-established": func(i *bfdPeerIdentity) { i.bgpGeneration++ },
	} {
		moved := identity
		mutate(&moved)
		if err := requireBFDProfileEditContinuity(identity, moved); err == nil {
			t.Errorf("%s passed as continuity", name)
		}
	}
}
