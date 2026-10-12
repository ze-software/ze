// VALIDATES: a reload that changes a peer's BFD opt-in while a start of the
// BFD client is between EnsureSession and storing its handle leaves the
// client on the NEW opt-in: an edit ends with one hold under the new
// profile, a removal with none. A stop that lands during a start cancels it.
// Method: a gated api.Service holds the first EnsureSession until the test
// has run the reload's swap (or the stop), then lets it finish; no sleeps.
// PREVENTS: the in-flight start storing a handle built from the pre-swap
// opt-in after the reload's stop found no handle and the reload's start
// declined because a start was in progress.
package reactor

import (
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/component/bfd/api"
)

// gatedBFDService is a swapBFDService whose first EnsureSession signals
// entered and then waits for proceed, so a test can act while a start is in
// flight.
type gatedBFDService struct {
	swapBFDService
	once    sync.Once
	entered chan struct{}
	proceed chan struct{}
}

func newGatedBFDService() *gatedBFDService {
	return &gatedBFDService{entered: make(chan struct{}), proceed: make(chan struct{})}
}

func (s *gatedBFDService) EnsureSession(req api.SessionRequest) (api.SessionHandle, error) {
	first := false
	s.once.Do(func() { first = true })
	if first {
		close(s.entered)
		<-s.proceed
	}
	return s.swapBFDService.EnsureSession(req)
}

// inFlightStart starts the peer's BFD client on its own goroutine and returns
// once the start is inside EnsureSession, with a wait for its end.
func inFlightStart(p *Peer, svc *gatedBFDService) *sync.WaitGroup {
	var wg sync.WaitGroup
	wg.Go(p.startBFDClient)
	<-svc.entered
	return &wg
}

// TestBFDSwapDuringInFlightStartEdit edits the profile from "fast" to "slow"
// while the FSM's start is inside EnsureSession under "fast". Once it ends,
// the peer holds one session, asked for under "slow".
// MUTATION: startBFDClient storing its handle without checking for a stop
// during the start leaves the hold under "fast", so this test goes red.
func TestBFDSwapDuringInFlightStartEdit(t *testing.T) {
	svc := newGatedBFDService()
	api.SetService(svc)
	defer api.SetService(nil)
	p := NewPeer(minimalPeerSettings(nonStrictBFD("fast")))
	p.setState(PeerStateEstablished)

	wg := inFlightStart(p, svc)
	swapBFD(t, p, withBFD(p.settingsSnapshot(), nonStrictBFD("slow")))
	close(svc.proceed)
	wg.Wait()

	live, profiles := svc.state()
	if live != 1 || len(profiles) == 0 || profiles[len(profiles)-1] != "slow" {
		t.Fatalf("after the edit: live %d, profiles %v, want one hold, the last under slow", live, profiles)
	}
	p.stopBFDClient()
	if live, _ := svc.state(); live != 0 {
		t.Fatalf("live %d after the stop, want 0", live)
	}
}

// TestBFDSwapDuringInFlightStartRemoval removes the opt-in while the FSM's
// start is inside EnsureSession. Once it ends, the peer holds no session.
// MUTATION: the same unchecked store leaves one hold, so this test goes red.
func TestBFDSwapDuringInFlightStartRemoval(t *testing.T) {
	svc := newGatedBFDService()
	api.SetService(svc)
	defer api.SetService(nil)
	p := NewPeer(minimalPeerSettings(nonStrictBFD("fast")))
	p.setState(PeerStateEstablished)

	wg := inFlightStart(p, svc)
	swapBFD(t, p, withBFD(p.settingsSnapshot(), nil))
	close(svc.proceed)
	wg.Wait()

	if live, _ := svc.state(); live != 0 {
		t.Fatalf("live %d after the opt-in was removed, want 0", live)
	}
}

// TestBFDStopDuringInFlightStart stops the client, as the exit from
// Established does, while a start is inside EnsureSession. Once the start
// ends, the peer holds no session.
// MUTATION: the same unchecked store leaves one hold, so this test goes red.
func TestBFDStopDuringInFlightStart(t *testing.T) {
	svc := newGatedBFDService()
	api.SetService(svc)
	defer api.SetService(nil)
	p := NewPeer(minimalPeerSettings(nonStrictBFD("fast")))
	p.setState(PeerStateEstablished)

	wg := inFlightStart(p, svc)
	p.stopBFDClient()
	close(svc.proceed)
	wg.Wait()

	if live, _ := svc.state(); live != 0 {
		t.Fatalf("live %d after a stop during the start, want 0", live)
	}
}
