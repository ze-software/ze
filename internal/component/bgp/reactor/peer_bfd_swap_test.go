// VALIDATES: a non-strict BFD opt-in change is swapped onto a running peer
// (peerSettingsSwapPlan neutralizes it, applyHotSwappableSettings opens or
// closes the BFD client), a change involving strict mode restarts, and a
// rollback through the journal's snapshot returns the client to its previous
// state. Method: a fake api.Service that counts live holds and records each
// request's profile, driven through the same plan and apply the reload runs.
// PREVENTS: a BFD edit resetting the BGP session (AC-5, AC-6, AC-15, AC-19),
// a leaked BFD hold after removal, and a strict-mode change applied to a
// session whose OPEN and OpenConfirm gate were built for the old one (AC-7).
package reactor

import (
	"strings"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/component/bfd/api"
)

// swapBFDService is an api.Service that hands out a fresh handle per
// EnsureSession, counts the holds still live, and records the profile each
// request named, in order.
type swapBFDService struct {
	mu       sync.Mutex
	profiles []string
	live     int
}

func (s *swapBFDService) EnsureSession(req api.SessionRequest) (api.SessionHandle, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.profiles = append(s.profiles, req.Profile)
	s.live++
	return &fakeBFDHandle{key: req.Key(), ch: make(chan api.StateChange, 4)}, nil
}

func (s *swapBFDService) ReleaseSession(h api.SessionHandle) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.live--
	if fh, ok := h.(*fakeBFDHandle); ok {
		close(fh.ch)
	}
	return nil
}

func (s *swapBFDService) Snapshot() []api.SessionState { return nil }
func (s *swapBFDService) SessionDetail(_ string) (api.SessionState, bool) {
	return api.SessionState{}, false
}
func (s *swapBFDService) Profiles() []api.ProfileState { return nil }

// state answers the live hold count and the profiles asked for so far.
func (s *swapBFDService) state() (int, []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.live, append([]string(nil), s.profiles...)
}

func nonStrictBFD(profile string) *BFDSettings {
	return &BFDSettings{Enabled: true, Profile: profile}
}

func strictBFD(profile string) *BFDSettings {
	return &BFDSettings{Enabled: true, Strict: true, Profile: profile}
}

// withBFD answers a copy of current carrying the given opt-in, so a plan
// compares two settings that differ in BFD alone.
func withBFD(current *PeerSettings, bfd *BFDSettings) *PeerSettings {
	next := *current
	next.BFD = bfd
	return &next
}

// swapBFD runs the reload's plan and apply for next on a running peer and
// fails the test when the plan restarts instead.
func swapBFD(t *testing.T, p *Peer, next *PeerSettings) {
	t.Helper()
	copier, reason := peerSettingsSwapPlan(p.settingsSnapshot(), next, nil)
	if reason != "" {
		t.Fatalf("plan restarts (%s), want a swap", reason)
	}
	p.applyHotSwappableSettings(next, copier)
}

// TestNonStrictBFDOptInSwaps: every change between nil, disabled and enabled
// non-strict opt-ins, including a profile edit and a strict-only timer edit
// on a non-strict opt-in, is a swap.
func TestNonStrictBFDOptInSwaps(t *testing.T) {
	base := minimalPeerSettings(nil)
	cases := map[string][2]*BFDSettings{
		"added":            {nil, nonStrictBFD("fast")},
		"removed":          {nonStrictBFD("fast"), nil},
		"suspended":        {nonStrictBFD("fast"), {Profile: "fast"}},
		"resumed":          {{Profile: "fast"}, nonStrictBFD("fast")},
		"profile edited":   {nonStrictBFD("fast"), nonStrictBFD("slow")},
		"hold-time edited": {nonStrictBFD("fast"), {Enabled: true, Profile: "fast", HoldTime: 9}},
		"disabled strict":  {nil, {Strict: true}},
	}
	for name, pair := range cases {
		copier, reason := peerSettingsSwapPlan(withBFD(base, pair[0]), withBFD(base, pair[1]), nil)
		if reason != "" || copier == nil {
			t.Errorf("%s: reason %q, want a swap", name, reason)
		}
	}
}

// TestStrictBFDChangeRestarts: a change with strict mode on either side
// restarts, and the reason names BFD (AC-7).
func TestStrictBFDChangeRestarts(t *testing.T) {
	base := minimalPeerSettings(nil)
	cases := map[string][2]*BFDSettings{
		"strict added":      {nil, strictBFD("fast")},
		"strict removed":    {strictBFD("fast"), nil},
		"strict to loose":   {strictBFD("fast"), nonStrictBFD("fast")},
		"loose to strict":   {nonStrictBFD("fast"), strictBFD("fast")},
		"strict hold-time":  {strictBFD("fast"), {Enabled: true, Strict: true, Profile: "fast", HoldTime: 9}},
		"strict hold-down":  {strictBFD("fast"), {Enabled: true, Strict: true, Profile: "fast", HoldDown: 300}},
		"strict profile":    {strictBFD("fast"), strictBFD("slow")},
		"strict re-enabled": {{Strict: true}, strictBFD("")},
		"strict suspended":  {strictBFD(""), {Strict: true}},
	}
	for name, pair := range cases {
		_, reason := peerSettingsSwapPlan(withBFD(base, pair[0]), withBFD(base, pair[1]), nil)
		if !strings.Contains(reason, "BFD") {
			t.Errorf("%s: reason %q, want a restart naming BFD", name, reason)
		}
	}
}

// TestBFDSwapOpensAndClosesClient: on an Established peer the swap opens the
// client, and removing the opt-in releases it, leaving no hold behind.
func TestBFDSwapOpensAndClosesClient(t *testing.T) {
	svc := &swapBFDService{}
	api.SetService(svc)
	defer api.SetService(nil)
	p := NewPeer(minimalPeerSettings(nil))
	p.setState(PeerStateEstablished)

	swapBFD(t, p, withBFD(p.settingsSnapshot(), nonStrictBFD("fast")))
	if live, profiles := svc.state(); live != 1 || len(profiles) != 1 || profiles[0] != "fast" {
		t.Fatalf("after the opt-in: live %d, profiles %v, want one hold under fast", live, profiles)
	}
	swapBFD(t, p, withBFD(p.settingsSnapshot(), nil))
	if live, _ := svc.state(); live != 0 {
		t.Fatalf("after the removal: live %d, want 0", live)
	}
}

// TestBFDSwapOnPeerNotEstablishedOpensNothing: a non-strict peer opens BFD on
// entry to Established, so a swap onto a peer that is not there yet only
// writes the opt-in, which the FSM's start then reads.
func TestBFDSwapOnPeerNotEstablishedOpensNothing(t *testing.T) {
	svc := &swapBFDService{}
	api.SetService(svc)
	defer api.SetService(nil)
	p := NewPeer(minimalPeerSettings(nil))

	swapBFD(t, p, withBFD(p.settingsSnapshot(), nonStrictBFD("fast")))
	if live, _ := svc.state(); live != 0 {
		t.Fatalf("live %d on a peer not Established, want 0", live)
	}
	p.startBFDClient()
	if live, profiles := svc.state(); live != 1 || profiles[0] != "fast" {
		t.Fatalf("the FSM's start: live %d, profiles %v, want one hold under fast", live, profiles)
	}
	p.stopBFDClient()
}

// TestNonStrictBFDProfileEditReopens: a profile edit releases the running
// client and opens one under the new profile (AC-15 at the reactor).
func TestNonStrictBFDProfileEditReopens(t *testing.T) {
	svc := &swapBFDService{}
	api.SetService(svc)
	defer api.SetService(nil)
	p := NewPeer(minimalPeerSettings(nonStrictBFD("fast")))
	p.setState(PeerStateEstablished)
	p.startBFDClient()

	swapBFD(t, p, withBFD(p.settingsSnapshot(), nonStrictBFD("slow")))
	live, profiles := svc.state()
	if live != 1 || len(profiles) != 2 || profiles[1] != "slow" {
		t.Fatalf("after the edit: live %d, profiles %v, want one hold, the second under slow", live, profiles)
	}
	// A change BFD does not read on a non-strict peer reopens nothing.
	swapBFD(t, p, withBFD(p.settingsSnapshot(), &BFDSettings{Enabled: true, Profile: "slow", HoldTime: 9}))
	if live, profiles := svc.state(); live != 1 || len(profiles) != 2 {
		t.Fatalf("after a hold-time edit: live %d, profiles %v, want the same single hold", live, profiles)
	}
	p.stopBFDClient()
}

// TestBFDSwapRollbackRestoresClient: the journal's undo, the snapshot read
// before the apply, returns both the opt-in and the client (AC-11, R-7).
func TestBFDSwapRollbackRestoresClient(t *testing.T) {
	svc := &swapBFDService{}
	api.SetService(svc)
	defer api.SetService(nil)
	p := NewPeer(minimalPeerSettings(nonStrictBFD("fast")))
	p.setState(PeerStateEstablished)
	p.startBFDClient()

	for name, bfd := range map[string]*BFDSettings{"removed": nil, "edited": nonStrictBFD("slow")} {
		next := withBFD(p.settingsSnapshot(), bfd)
		copier, reason := peerSettingsSwapPlan(p.settingsSnapshot(), next, nil)
		if reason != "" {
			t.Fatalf("%s: plan restarts (%s)", name, reason)
		}
		previous := p.hotSwappableSnapshot(copier)
		p.applyHotSwappableSettings(next, copier)
		p.applyHotSwappableSettings(previous, copier)
		if got := p.bfdSettings(); got == nil || got.Profile != "fast" {
			t.Fatalf("%s: opt-in after rollback %+v, want fast", name, got)
		}
		live, profiles := svc.state()
		if live != 1 || profiles[len(profiles)-1] != "fast" {
			t.Fatalf("%s: after rollback live %d, profiles %v, want one hold under fast", name, live, profiles)
		}
	}
	p.stopBFDClient()
}

// TestBFDSwapRacesEstablishedStart runs the swap against the FSM's own start
// and stop of the client, under -race (R-1, R-2): no data race, and once both
// stop and the peer leaves Established, no hold is left.
func TestBFDSwapRacesEstablishedStart(t *testing.T) {
	svc := &swapBFDService{}
	api.SetService(svc)
	defer api.SetService(nil)
	p := NewPeer(minimalPeerSettings(nil))
	p.setState(PeerStateEstablished)

	var wg sync.WaitGroup
	wg.Go(func() {
		for range 200 {
			p.startBFDClient()
			_ = p.bfdStrict()
			p.stopBFDClient()
		}
	})
	for i := range 200 {
		bfd := nonStrictBFD("fast")
		if i%2 == 1 {
			bfd = nil
		}
		swapBFD(t, p, withBFD(p.settingsSnapshot(), bfd))
	}
	wg.Wait()
	p.stopBFDClient()
	if live, _ := svc.state(); live != 0 {
		t.Fatalf("live %d after every start was stopped, want 0", live)
	}
}

// TestBFDSwapRacesStrictNegotiation runs the swap against the session's
// strict-mode readers, under -race (R-1): the session reads the opt-in
// through the peer's locked accessor.
func TestBFDSwapRacesStrictNegotiation(t *testing.T) {
	p := NewPeer(minimalPeerSettings(nil))
	session := p.newSession()

	var wg sync.WaitGroup
	wg.Go(func() {
		for range 200 {
			session.applyBFDStrictNegotiation()
		}
	})
	for i := range 200 {
		bfd := nonStrictBFD("fast")
		if i%2 == 1 {
			bfd = nil
		}
		swapBFD(t, p, withBFD(p.settingsSnapshot(), bfd))
	}
	wg.Wait()
}
