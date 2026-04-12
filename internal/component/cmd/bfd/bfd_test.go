package bfd

import (
	"encoding/json"
	"strings"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	bfdapi "codeberg.org/thomas-mangin/ze/internal/plugins/bfd/api"
)

// stubService is a tiny api.Service impl that returns canned snapshot
// data. Used to drive the CLI handlers without spinning up the real
// BFD engine.
type stubService struct {
	sessions []bfdapi.SessionState
	profiles []bfdapi.ProfileState
}

func (s *stubService) EnsureSession(_ bfdapi.SessionRequest) (bfdapi.SessionHandle, error) {
	return nil, nil //nolint:nilnil // unused in these tests
}
func (s *stubService) ReleaseSession(_ bfdapi.SessionHandle) error { return nil }
func (s *stubService) Snapshot() []bfdapi.SessionState             { return s.sessions }
func (s *stubService) SessionDetail(peer string) (bfdapi.SessionState, bool) {
	for i := range s.sessions {
		if s.sessions[i].Peer == peer {
			return s.sessions[i], true
		}
	}
	return bfdapi.SessionState{}, false
}
func (s *stubService) Profiles() []bfdapi.ProfileState { return s.profiles }

func withStubService(t *testing.T, svc bfdapi.Service) {
	t.Helper()
	prev := bfdapi.GetService()
	bfdapi.SetService(svc)
	t.Cleanup(func() { bfdapi.SetService(prev) })
}

// VALIDATES: handleShowSessions returns StatusDone with a JSON array
// containing every session surfaced by Service.Snapshot, in the order
// Service returned them. A stub service is wired in so the handler
// path runs without the real engine.
// PREVENTS: regressions in the CLI -> pluginserver -> bfd.api call
// chain that would silently return an empty array.
func TestHandleShowSessions(t *testing.T) {
	withStubService(t, &stubService{sessions: []bfdapi.SessionState{
		{Peer: "203.0.113.1", VRF: "default", Mode: "single-hop", State: "up"},
		{Peer: "203.0.113.2", VRF: "default", Mode: "single-hop", State: "down"},
	}})
	resp, err := handleShowSessions(nil, nil)
	if err != nil {
		t.Fatalf("handleShowSessions: %v", err)
	}
	if resp.Status != plugin.StatusDone {
		t.Fatalf("status = %q, want %q", resp.Status, plugin.StatusDone)
	}
	data, _ := resp.Data.(string)
	var out []bfdapi.SessionState
	if err := json.Unmarshal([]byte(data), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(out) != 2 || out[0].Peer != "203.0.113.1" || out[1].Peer != "203.0.113.2" {
		t.Fatalf("payload = %+v", out)
	}
}

// VALIDATES: handleShowSessions returns StatusError when the BFD
// plugin has not published its Service (e.g., plugin not loaded).
// PREVENTS: bare nil dereference if a future refactor drops the nil
// check.
func TestHandleShowSessions_ServiceUnavailable(t *testing.T) {
	withStubService(t, nil)
	resp, err := handleShowSessions(nil, nil)
	if err != nil {
		t.Fatalf("handleShowSessions: %v", err)
	}
	if resp.Status != plugin.StatusError {
		t.Fatalf("status = %q, want %q", resp.Status, plugin.StatusError)
	}
	msg, _ := resp.Data.(string)
	if !strings.Contains(msg, "bfd: plugin not loaded") {
		t.Fatalf("message = %q", msg)
	}
}

// VALIDATES: handleShowSession with an unknown peer returns a
// StatusError containing "no session for peer", matching AC-3.
// PREVENTS: silent empty response for typos.
func TestHandleShowSession_NotFound(t *testing.T) {
	withStubService(t, &stubService{sessions: []bfdapi.SessionState{{Peer: "203.0.113.1"}}})
	resp, err := handleShowSession(nil, []string{"198.51.100.9"})
	if err != nil {
		t.Fatalf("handleShowSession: %v", err)
	}
	if resp.Status != plugin.StatusError {
		t.Fatalf("status = %q, want %q", resp.Status, plugin.StatusError)
	}
	msg, _ := resp.Data.(string)
	if !strings.Contains(msg, "no session for peer") {
		t.Fatalf("message = %q", msg)
	}
}

// VALIDATES: handleShowSession rejects invalid peer arguments at parse
// time so operators see a descriptive error instead of "not found".
// PREVENTS: confusing error when the operator types garbage.
func TestHandleShowSession_InvalidPeer(t *testing.T) {
	withStubService(t, &stubService{})
	resp, err := handleShowSession(nil, []string{"not-an-ip"})
	if err != nil {
		t.Fatalf("handleShowSession: %v", err)
	}
	if resp.Status != plugin.StatusError {
		t.Fatalf("status = %q, want %q", resp.Status, plugin.StatusError)
	}
	msg, _ := resp.Data.(string)
	if !strings.Contains(msg, "invalid peer") {
		t.Fatalf("message = %q", msg)
	}
}

// VALIDATES: handleShowProfile returns the full list when no name is
// supplied, and the single matching profile when a name is given.
// Mismatched names return a StatusError.
// PREVENTS: AC-4 / AC-5 regressions in the profile handler.
func TestHandleShowProfile(t *testing.T) {
	svc := &stubService{profiles: []bfdapi.ProfileState{
		{Name: "fast", DetectMult: 3, DesiredMinTxUs: 50_000, RequiredMinRxUs: 50_000},
		{Name: "slow", DetectMult: 3, DesiredMinTxUs: 1_000_000, RequiredMinRxUs: 1_000_000},
	}}
	withStubService(t, svc)

	// AC-5: empty args returns every profile.
	resp, err := handleShowProfile(nil, nil)
	if err != nil {
		t.Fatalf("handleShowProfile empty: %v", err)
	}
	if resp.Status != plugin.StatusDone {
		t.Fatalf("empty status = %q", resp.Status)
	}
	data, _ := resp.Data.(string)
	var all []bfdapi.ProfileState
	if err := json.Unmarshal([]byte(data), &all); err != nil {
		t.Fatalf("unmarshal all: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("all len = %d", len(all))
	}

	// AC-4: specific profile returns single object.
	resp, err = handleShowProfile(nil, []string{"fast"})
	if err != nil {
		t.Fatalf("handleShowProfile fast: %v", err)
	}
	if resp.Status != plugin.StatusDone {
		t.Fatalf("fast status = %q", resp.Status)
	}
	data, _ = resp.Data.(string)
	var one bfdapi.ProfileState
	if err := json.Unmarshal([]byte(data), &one); err != nil {
		t.Fatalf("unmarshal fast: %v", err)
	}
	if one.Name != "fast" || one.DesiredMinTxUs != 50_000 {
		t.Fatalf("fast payload = %+v", one)
	}

	// Unknown name: error.
	resp, err = handleShowProfile(nil, []string{"nope"})
	if err != nil {
		t.Fatalf("handleShowProfile nope: %v", err)
	}
	if resp.Status != plugin.StatusError {
		t.Fatalf("nope status = %q, want %q", resp.Status, plugin.StatusError)
	}
}
