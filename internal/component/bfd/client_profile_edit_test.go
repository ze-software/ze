// Design: docs/guide/bfd.md -- Editing live
// Related: bfd.go -- applyConfig, reprofileClients
// Related: engine/shared_session.go -- Loop.Reprofile
//
// VALIDATES: spec-bgp-live-receive-binding-and-bfd-keep-session AC-25 for the
// protocol clients (BGP, static routes, OSPF): a reload that edits a bfd
// profile reaches every live session a client holds under that profile, in
// place. The clients ask through pluginService.EnsureSession, the entry point
// every one of them uses, naming only the profile; the edit arrives through
// applyConfig, the function the plugin's configure and reload callbacks run.
// PREVENTS: a client request that keeps the timers and authentication its
// profile resolved to when the client asked, so a profile edit changes nothing
// for a BGP peer, a static route or an OSPF neighbor until the session is torn
// down and asked for again.
package bfd

import (
	"errors"
	"net/netip"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/bfd/api"
	"github.com/ze-software/ze/internal/component/bfd/engine"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// clientEditSection is a config with profiles "p" and "q" (bodies given) and
// profile "secure" with the Keyed SHA1 auth block pinnedEditAuth names. It
// pins no session: every session comes from a client's request.
func clientEditSection(pBody, qBody string) string {
	return `{"bfd":{"profile":{` +
		`"p":{` + pBody + `},` +
		`"q":{` + qBody + `},` +
		`"secure":{"auth":{"type":"keyed-sha1","key-id":"7","secret":"EDIT-SECRET"}}}}}`
}

// clientEditRequest is what a protocol client asks for: the multi-hop session
// to pinnedEditPeer under profile, with no timers of its own.
func clientEditRequest(profile string) api.SessionRequest {
	return api.SessionRequest{
		Peer:    netip.MustParseAddr(pinnedEditPeer),
		Local:   netip.MustParseAddr("172.30.9.2"),
		Mode:    api.MultiHop,
		Profile: profile,
	}
}

// ensureClient asks svc for req the way a protocol client does.
func ensureClient(t *testing.T, svc *pluginService, req api.SessionRequest) {
	t.Helper()
	if _, err := svc.EnsureSession(req); err != nil {
		t.Fatalf("EnsureSession(profile %q): %v", req.Profile, err)
	}
}

// TestClientProfileTimerEditInPlace has a client ask under profile "p"
// (300ms x 3), then a reload edits "p" to 100ms x 5. The client's session
// keeps its local discriminator and runs Detect Mult 5.
// MUTATION: applyConfig not re-resolving the client requests keeps Detect
// Mult 3, so this test goes red.
func TestClientProfileTimerEditInPlace(t *testing.T) {
	svc, loop := pinnedEditService(t, clientEditSection(pinnedEditSlow, pinnedEditSlow))
	ensureClient(t, svc, clientEditRequest("p"))
	before := onlySession(t, loop)
	if before.DetectMult != 3 {
		t.Fatalf("precondition: Detect Mult %d, want 3", before.DetectMult)
	}

	applyPinnedEdit(t, svc, clientEditSection(pinnedEditFast, pinnedEditSlow))

	after := onlySession(t, loop)
	if after.LocalDiscr != before.LocalDiscr {
		t.Fatalf("the edit replaced the session: discriminator %d, want %d", after.LocalDiscr, before.LocalDiscr)
	}
	if after.DetectMult != 5 {
		t.Fatalf("after the edit: Detect Mult %d, want the edited 5", after.DetectMult)
	}
}

// TestClientProfileEditMovesFastest has two clients share one session, under
// "p" (300ms x 3) and "q" (100ms x 5): the session runs "q", the shorter
// Detection Time. A reload slows "q" to 300ms x 4, and the session then runs
// "p", now the faster, with Detect Mult 3.
// MUTATION: a reload that leaves the stored requests alone keeps "q" at
// 100ms x 5, so the session keeps Detect Mult 5 and names "q".
func TestClientProfileEditMovesFastest(t *testing.T) {
	svc, loop := pinnedEditService(t, clientEditSection(pinnedEditSlow, pinnedEditFast))
	ensureClient(t, svc, clientEditRequest("p"))
	ensureClient(t, svc, clientEditRequest("q"))
	before := onlySession(t, loop)
	if before.Refcount != 2 || before.Profile != "q" || before.DetectMult != 5 {
		t.Fatalf("precondition: refcount %d profile %q Detect Mult %d, want 2, \"q\", 5",
			before.Refcount, before.Profile, before.DetectMult)
	}

	slowerQ := `"detect-multiplier":"4","desired-min-tx-us":"300000","required-min-rx-us":"300000"`
	applyPinnedEdit(t, svc, clientEditSection(pinnedEditSlow, slowerQ))

	after := onlySession(t, loop)
	if after.LocalDiscr != before.LocalDiscr {
		t.Fatalf("the edit replaced the session: discriminator %d, want %d", after.LocalDiscr, before.LocalDiscr)
	}
	if after.Profile != "p" || after.DetectMult != 3 {
		t.Fatalf("after the edit: profile %q Detect Mult %d, want \"p\" with 3", after.Profile, after.DetectMult)
	}
}

// TestClientProfileAuthEditJoinsAgain has a client hold a session under "p",
// which has no auth block, so a request under "secure" (Keyed SHA1) for the
// same remote system is refused. A reload gives "p" the same auth block, and
// the same request joins the one session, at refcount 2.
// MUTATION: a reload that leaves the client's request alone keeps the session
// unauthenticated, so the join is refused again.
func TestClientProfileAuthEditJoinsAgain(t *testing.T) {
	svc, loop := pinnedEditService(t, clientEditSection(pinnedEditSlow, pinnedEditSlow))
	ensureClient(t, svc, clientEditRequest("p"))
	joiner := clientEditRequest("secure")
	if _, err := svc.EnsureSession(joiner); !errors.Is(err, engine.ErrAuthMismatch) {
		t.Fatalf("join before the edit: err %v, want ErrAuthMismatch", err)
	}
	before := onlySession(t, loop)

	applyPinnedEdit(t, svc, clientEditSection(pinnedEditSlow+pinnedEditAuth, pinnedEditSlow))

	ensureClient(t, svc, joiner)
	after := onlySession(t, loop)
	if after.Refcount != 2 || after.LocalDiscr != before.LocalDiscr {
		t.Fatalf("after the join: refcount %d discriminator %d, want 2 on discriminator %d",
			after.Refcount, after.LocalDiscr, before.LocalDiscr)
	}
}

// TestClientProfileAuthEditRefusedWhileShared has two clients share one
// unauthenticated session under "p" and "q". A reload giving "p" an auth block
// is refused with ErrAuthMismatch, since the "q" client runs the current, no
// authentication, and the session keeps it: a request under "q" still joins.
// The refusal says what an operator reads: both profiles, that every client
// keeps the session it ran before the commit, the fix in the config, and the
// route that releases the other client first.
// MUTATION: the join path's reason ("this client gets no BFD session") on the
// edit path, or a fix naming neither profile, turns this test red.
func TestClientProfileAuthEditRefusedWhileShared(t *testing.T) {
	svc, loop := pinnedEditService(t, clientEditSection(pinnedEditSlow, pinnedEditSlow))
	ensureClient(t, svc, clientEditRequest("p"))
	ensureClient(t, svc, clientEditRequest("q"))

	cfg := parseClientEdit(t, clientEditSection(pinnedEditSlow+pinnedEditAuth, pinnedEditSlow))
	runtimeStateGuard.Lock()
	err := svc.state.applyConfig(cfg)
	runtimeStateGuard.Unlock()
	if !errors.Is(err, engine.ErrAuthMismatch) {
		t.Fatalf("reload giving \"p\" an auth block: err %v, want ErrAuthMismatch", err)
	}
	msg := err.Error()
	for _, want := range []string{
		`after this commit a client of bfd profile "p" and a client of bfd profile "q" ask for different auth blocks`,
		"keeps the settings it ran before this commit, for every client",
		`to fix it, give bfd profile "p" and bfd profile "q" the same auth block`,
		`or first commit a change that releases a client of bfd profile "q", then commit this edit`,
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("refusal %q\nlacks %q", msg, want)
		}
	}
	if strings.Contains(msg, "gets no BFD session") {
		t.Errorf("refusal %q says a client loses its session, but every client keeps it", msg)
	}

	ensureClient(t, svc, clientEditRequest("q"))
	if got := onlySession(t, loop).Refcount; got != 3 {
		t.Fatalf("after the refused edit: refcount %d, want 3", got)
	}
}

// parseClientEdit parses section the way a config reload does, for a test
// that reads the error applyConfig answers.
func parseClientEdit(t *testing.T, section string) *pluginConfig {
	t.Helper()
	cfg, err := parseSections([]sdk.ConfigSection{{Root: configRoot, Data: section}})
	if err != nil {
		t.Fatalf("parseSections: %v", err)
	}
	return cfg
}

// TestClientProfileAuthEditTogether has two clients share one unauthenticated
// session under "p" and "q". One reload gives both profiles the same auth
// block, and the session takes it: the edits are judged together, so neither
// client is refused for relying on the authentication the other leaves. A
// request under "secure", the same auth block, then joins at refcount 3.
// MUTATION: editing the clients one at a time refuses the first edit, since
// the other client still holds the old request, and fails the reload.
func TestClientProfileAuthEditTogether(t *testing.T) {
	svc, loop := pinnedEditService(t, clientEditSection(pinnedEditSlow, pinnedEditSlow))
	ensureClient(t, svc, clientEditRequest("p"))
	ensureClient(t, svc, clientEditRequest("q"))

	applyPinnedEdit(t, svc, clientEditSection(pinnedEditSlow+pinnedEditAuth, pinnedEditSlow+pinnedEditAuth))

	ensureClient(t, svc, clientEditRequest("secure"))
	if got := onlySession(t, loop).Refcount; got != 3 {
		t.Fatalf("after the edit and the join: refcount %d, want 3", got)
	}
}
