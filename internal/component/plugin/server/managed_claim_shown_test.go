package server

import (
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	pluginipc "github.com/ze-software/ze/internal/component/plugin/ipc"
)

// managedShownMax is the most bytes a shown claim holds: the name form's 64
// bytes and the three-byte elision mark.
const managedShownMax = 64 + len("...")

// checkShownClient fails unless client is valid UTF-8, holds only runes
// strconv prints as themselves, and stays within managedShownMax.
func checkShownClient(t *testing.T, client string) {
	t.Helper()
	if !utf8.ValidString(client) {
		t.Errorf("Client %q is not valid UTF-8", client)
	}
	for _, r := range client {
		if !strconv.IsPrint(r) {
			t.Errorf("Client %q holds the non-printable rune %U", client, r)
		}
	}
	if len(client) > managedShownMax {
		t.Errorf("Client %q is %d bytes, bound is %d", client, len(client), managedShownMax)
	}
}

// TestManagedRefusedClaimIsSafeToPrint guards the Client field authRefused
// stores: the claimed name is chosen by whoever reaches the managed port, and
// `show doctor` prints the record. Method: refuse a claim holding a terminal
// escape, a newline, invalid UTF-8 and a three-byte rune straddling the bound,
// under both causes, and check each recorded Client and Detail is printable
// and bounded; then dial the real listener with an escape in the name and
// check the recorded Client carries the escape as text. A configured name is
// still recorded under itself.
func TestManagedRefusedClaimIsSafeToPrint(t *testing.T) {
	srv, addr := startTestManagedServer(t)

	hostile := "\x1b[2J" + "evil\nrow" + "\xff\xfe" + strings.Repeat("a", 40) + "日本語" + strings.Repeat("b", 40)
	srv.authRefused("192.0.2.1:4000", &pluginipc.AuthRefusal{Claimed: hostile, Cause: pluginipc.AuthRefusedNoSecret})
	srv.authRefused("192.0.2.1:4001", &pluginipc.AuthRefusal{Claimed: "x" + hostile, Cause: pluginipc.AuthRefusedName})
	rejected := srv.doctor.snapshot().Rejected
	if len(rejected) != 2 {
		t.Fatalf("recorded %d refusals, want 2: %+v", len(rejected), rejected)
	}
	for _, rejection := range rejected {
		checkShownClient(t, rejection.Client)
		for _, r := range rejection.Detail {
			if !strconv.IsPrint(r) {
				t.Errorf("Detail %q holds the non-printable rune %U", rejection.Detail, r)
			}
		}
	}

	if authAs(t, addr, "\x1b[2Jwire", "") {
		t.Fatal("a name holding an escape authenticated")
	}
	got := waitForRejection(t, srv, `\x1b[2Jwire`)
	checkShownClient(t, got.Client)

	const wrongToken = "wrong-token-wrong-token-wrong-32"
	if authAs(t, addr, testClientName, wrongToken) {
		t.Fatal("a configured client with a wrong token authenticated")
	}
	if got := waitForRejection(t, srv, testClientName); got.Cause != ManagedRejectBadToken {
		t.Errorf("%s rejection = %+v, want ManagedRejectBadToken", testClientName, got)
	}
}
