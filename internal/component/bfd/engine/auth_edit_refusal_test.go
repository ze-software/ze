// Design: docs/guide/bfd.md -- Editing live
// Related: shared_session.go -- reprofileEntryLocked, updateAuthLocked, authEditFix
//
// VALIDATES: a profile edit refused because another client of the session
// relies on its current authentication says what is true after the edit:
// every client keeps its session, and the fix it names is one the operator
// can apply, also when the other client names no bfd profile, as an OSPF
// interface cannot.
// PREVENTS: a refusal that tells an operator a client lost its session when
// none did, or sends an OSPF interface to a bfd profile it cannot name.
package engine

import (
	"errors"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/bfd/api"
	"github.com/ze-software/ze/internal/component/bfd/packet"
	"github.com/ze-software/ze/internal/core/clock"
)

// editSecret is the auth block an edit gives a profile.
var editSecret = &api.AuthSettings{Type: packet.AuthTypeKeyedSHA1, KeyID: 3, Secret: []byte("EDIT-SECRET")}

// assertEditRefusal checks err is ErrAuthMismatch carrying every want and
// none of the refused texts.
func assertEditRefusal(t *testing.T, err error, want, refused []string) {
	t.Helper()
	if !errors.Is(err, ErrAuthMismatch) {
		t.Fatalf("err %v, want ErrAuthMismatch", err)
	}
	msg := err.Error()
	for _, w := range want {
		if !strings.Contains(msg, w) {
			t.Errorf("refusal %q\nlacks %q", msg, w)
		}
	}
	for _, r := range refused {
		if strings.Contains(msg, r) {
			t.Errorf("refusal %q\ncarries %q", msg, r)
		}
	}
}

// TestReprofileRefusalBesideNoProfileClient shares one session between a
// client of profile "p" and a client naming no profile (an OSPF interface's
// request), then edits "p" to carry an auth block. The refusal names the
// fixes that client can take, the OSPF one included, and never asks to give
// "both profiles" one auth block.
// MUTATION: the fix "give both profiles the same auth block" for this case
// goes red, since the other client has no profile to edit.
func TestReprofileRefusalBesideNoProfileClient(t *testing.T) {
	l := NewLoop(&captureTransport{}, clock.RealClock{})
	if _, err := l.EnsureSession(timedReq("p", 300_000, 3)); err != nil {
		t.Fatalf("EnsureSession p: %v", err)
	}
	if _, err := l.EnsureSession(timedReq("", 300_000, 3)); err != nil {
		t.Fatalf("EnsureSession no profile: %v", err)
	}

	err := l.Reprofile(func(req api.SessionRequest) (api.SessionRequest, error) {
		req.Auth = editSecret
		return req, nil
	})

	assertEditRefusal(t, err, []string{
		`a client of bfd profile "p" and a client naming no bfd profile ask for different auth blocks`,
		"keeps the settings it ran before this commit",
		`restore the previous auth block of bfd profile "p"`,
		"set bfd { enabled false; }",
	}, []string{"both profiles", "gets no BFD session"})
}

// TestUpdateSessionRefusalBesideNoProfileClient edits a pinned session of
// profile "p" to carry an auth block while a client naming no profile shares
// the session. The refusal names the pinned session's profile and the OSPF
// fix, and never tells the no-profile client only to name a profile.
// MUTATION: the old fix ("name bfd profile %q on it", with no OSPF
// alternative) goes red.
func TestUpdateSessionRefusalBesideNoProfileClient(t *testing.T) {
	l := NewLoop(&captureTransport{}, clock.RealClock{})
	pinned := timedReq("p", 300_000, 3)
	h, err := l.EnsureSession(pinned)
	if err != nil {
		t.Fatalf("EnsureSession pinned: %v", err)
	}
	if _, err := l.EnsureSession(timedReq("", 300_000, 3)); err != nil {
		t.Fatalf("EnsureSession no profile: %v", err)
	}

	edited := pinned
	edited.Auth = editSecret
	assertEditRefusal(t, l.UpdateSession(h, edited), []string{
		`this pinned session's bfd profile "p" asks for another auth block than a client naming no bfd profile`,
		"keeps the settings it ran before this commit",
		"set bfd { enabled false; }",
	}, []string{"gets no BFD session"})
}

// TestUpdateSessionRefusalPinnedNamingNoProfile has a pinned session of
// "secure" (an auth block) share with a client of "other" carrying the same
// block, then edits the pinned session to name no profile, so it asks for no
// authentication. The refusal names the pinned session as naming no profile,
// never an empty profile name.
// MUTATION: the old text (bfd profile %q of the edited request) prints
// bfd profile "" and goes red.
func TestUpdateSessionRefusalPinnedNamingNoProfile(t *testing.T) {
	l := NewLoop(&captureTransport{}, clock.RealClock{})
	pinned := timedReq("secure", 300_000, 3)
	pinned.Auth = editSecret
	h, err := l.EnsureSession(pinned)
	if err != nil {
		t.Fatalf("EnsureSession pinned: %v", err)
	}
	other := timedReq("other", 300_000, 3)
	other.Auth = editSecret
	if _, err := l.EnsureSession(other); err != nil {
		t.Fatalf("EnsureSession other: %v", err)
	}

	edited := pinned
	edited.Profile = ""
	edited.Auth = nil
	assertEditRefusal(t, l.UpdateSession(h, edited), []string{
		`this pinned session, which names no bfd profile, asks for another auth block than a client of bfd profile "other"`,
		`name bfd profile "other" on this pinned session`,
	}, []string{`profile ""`, "gets no BFD session"})
}
