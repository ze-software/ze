// Design: docs/architecture/traffic/cos-dynamic.md -- retained cleanup failures
//go:build ze_l2tp

package cos

import "testing"

// A failed first write and failed compensation have no admitted member yet.
// The eventual original Down must still recover that baseline without a reread.
func TestCoSLegacySharedRetainsUnpublishedCompensation(t *testing.T) {
	f := newSharedCoSTestOwner(t)
	f.failWrites = 2
	f.up(461, sharedCoSTestFirst)
	key := sessionKey{tunnelID: sharedCoSTestTunnel, sessionID: 461}
	if _, admitted := sessionStore.Load(key); admitted {
		t.Fatal("unconfirmed initial write published a member")
	}
	writes := f.writes
	f.up(462, sharedCoSTestDifferent)
	if f.writes != writes {
		t.Fatal("failed unpublished compensation admitted another writer")
	}
	f.failWrites = 1
	f.down(461)
	writes = f.writes
	f.up(463, sharedCoSTestAlias)
	if f.writes != writes {
		t.Fatal("failed compensation retry discarded its original custody")
	}
	f.down(461)
	f.requireMaps(f.original)
	if f.reads != 1 {
		t.Fatalf("unpublished compensation recaptured the baseline: reads=%d", f.reads)
	}
	f.up(464, sharedCoSTestFirst)
	f.requireMaps(f.shared)
	f.down(464)
	f.requireMaps(f.original)
}

// A departure on a reserved access must resolve that transaction first. Failed
// compensation leaves both the departing and surviving admitted uses intact.
func TestCoSLegacySharedDepartureRetainsFailedReservation(t *testing.T) {
	f := newSharedCoSTestOwner(t)
	f.up(471, sharedCoSTestFirst)
	f.up(472, sharedCoSTestAlias)
	t.Cleanup(func() { sessionStore.Delete(sessionKey{tunnelID: sharedCoSTestTunnel, sessionID: 473}) })
	token, err := f.command("prepare", 473, sharedCoSTestAlias, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.command("apply", 0, "", token); err != nil {
		t.Fatal(err)
	}
	f.failWrites = 1
	f.down(471)
	for _, session := range []uint16{471, 472, 473} {
		if _, retained := sessionStore.Load(sessionKey{tunnelID: sharedCoSTestTunnel, sessionID: session}); !retained {
			t.Fatalf("failed departure compensation lost member %d", session)
		}
	}
	writes := f.writes
	f.up(474, sharedCoSTestAlias)
	if f.writes != writes {
		t.Fatal("failed departure compensation admitted an interleaved writer")
	}
	f.down(471)
	f.requireMaps(f.shared)
	if _, retained := sessionStore.Load(sessionKey{tunnelID: sharedCoSTestTunnel, sessionID: 473}); retained {
		t.Fatal("confirmed compensation retained its newly joined member")
	}
	f.down(472)
	f.requireMaps(f.original)
}
