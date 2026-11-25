//go:build linux

// Design: docs/architecture/testing/interop.md -- only the atomic migration request mutates.
// Related: socket_linux.go -- vendor framing; migration_test.go -- fixed UAPI layout.
package main

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/vishvananda/netlink/nl"
	"golang.org/x/sys/unix"
)

// TestStateDumpRequestHasNoFixedQueryBody matches xfrm_dump_sa's zero-offset attributes.
func TestStateDumpRequestHasNoFixedQueryBody(t *testing.T) {
	message := stateDumpRequest().Serialize()
	if len(message) != unix.NLMSG_HDRLEN {
		t.Fatalf("GETSA dump has %d payload bytes, want no fixed query body", len(message)-unix.NLMSG_HDRLEN)
	}
	if binary.NativeEndian.Uint16(message[4:6]) != nl.XFRM_MSG_GETSA {
		t.Fatal("readback is not GETSA")
	}
	if binary.NativeEndian.Uint16(message[6:8])&unix.NLM_F_DUMP != unix.NLM_F_DUMP {
		t.Fatal("readback is not a dump")
	}
}

// TestMigrationRequestKeepsFixedHeaderBeforeENCAP catches netlink attribute Data/RawData reordering.
func TestMigrationRequestKeepsFixedHeaderBeforeENCAP(t *testing.T) {
	for _, encap := range []encapsulation{{form: formBare}, {form: formUDP, sourcePort: 4500, targetPort: 4501}} {
		request, err := migrationRequest(fixtureState(), encap)
		if err != nil {
			t.Fatal(err)
		}
		message := request.Serialize()
		want, err := migrationPayload(fixtureState(), encap)
		if err != nil {
			t.Fatal(err)
		}
		if binary.NativeEndian.Uint16(message[4:6]) != migrateStateMessage {
			t.Fatal("request does not select MIGRATE_STATE")
		}
		if binary.NativeEndian.Uint16(message[6:8])&unix.NLM_F_ACK == 0 {
			t.Fatal("migration does not require an ACK")
		}
		if !bytes.Equal(message[unix.NLMSG_HDRLEN:], want) {
			t.Fatal("vendor framing reordered or changed the fixed header and ENCAP attribute")
		}
	}
}
