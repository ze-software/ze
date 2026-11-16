// Design: docs/architecture/bgp/protocol-event-capture.md -- bounded input before file activation.
// Related: capture_replay.go -- active file encoding and per-peer capture registration.
// Related: peer_run.go -- selection activates a pending capture after incumbent cleanup.
package reactor

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/core/capture"
	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/version"
)

// protocolCapture is the immutable input installed before a Session reads TCP.
// Its private constructors expose only non-nil concrete pointers; nil alone
// means capture is disabled. The zero values of both variants are invalid.
type protocolCapture interface {
	recordMessage(time.Time, uint8, []byte, uint32, uint16)
	recordConfig(time.Time, string, string, []byte)
	recordSession(time.Time, string)
	Close()
	Path() string
	Drops() uint64
	peerAddress() string
	protocolCapture()
}

func (*sessionCapture) protocolCapture() {}

// captureInput owns one bounded input queue, replay header and close seal.
// Only newCaptureInput constructs it. Its zero value and nil pointer are
// invalid. Producers MUST stop with close; the owning worker MUST then drain
// or discard the closed queue and signal finished. Record methods never block
// on file I/O. sendMu orders selection and producer close as well as sends.
type captureInput struct {
	clock     clock.Clock
	onDrop    func()
	ch        chan *captureItem
	pool      sync.Pool
	finished  chan struct{}
	stop      chan struct{}
	startOnce sync.Once
	sendMu    sync.RWMutex
	closed    bool
	drops     atomic.Uint64
	path      string
	limit     int64
	onLimit   CaptureLimitAction
	// The worker alone finalizes proposal identity before opening a file.
	// Every other header field is immutable from construction.
	header capture.Header
	// Both timestamps derive from this immutable instant. The header stores
	// its wire representation; file activation keeps the original instant.
	started time.Time
}

func newCaptureInput(ps *PeerSettings, coalesce bool, c clock.Clock, onDrop func()) *captureInput {
	started := c.Now()
	input := &captureInput{
		clock: c, onDrop: onDrop,
		ch:       make(chan *captureItem, captureQueueDepth),
		finished: make(chan struct{}), stop: make(chan struct{}),
		path:    capturePath(ps.Capture, ps.Address),
		limit:   int64(ps.Capture.MaximumSize) << 20,
		onLimit: ps.Capture.OnLimit,
		started: started,
		header: capture.Header{
			Peer: ps.Address.String(), Started: started.UTC().Format(capture.TimeFormat),
			DaemonVersion: version.Short(), LocalAS: ps.LocalAS, PeerAS: ps.PeerAS,
			RouterID: ps.RouterID, Coalesce: coalesce,
		},
	}
	input.pool.New = func() any { return &captureItem{data: make([]byte, 0, 4096)} }
	return input
}

func (input *captureInput) Path() string        { return input.path }
func (input *captureInput) Drops() uint64       { return input.drops.Load() }
func (input *captureInput) peerAddress() string { return input.header.Peer }

// captureWorker is constructed together with its input. A worker MUST own that
// input for its entire lifetime; promotion runs the active file writer on the
// same pending worker rather than starting a second goroutine.
type captureWorker interface{ run() }

// start binds the one worker to this input. Its owner MUST call close even
// when no event was recorded; close MUST join this exact worker.
func (input *captureInput) start(worker captureWorker) {
	input.startOnce.Do(func() {
		go func() {
			defer close(input.finished)
			worker.run()
		}()
	})
}

// close seals producers and MUST join the worker started for this input.
// The caller MUST use the same worker supplied to start. Calling it before
// start is permitted and starts the drain/discard owner itself.
func (input *captureInput) close(worker captureWorker) {
	input.sendMu.Lock()
	if !input.closed {
		input.closed = true
		close(input.stop)
		close(input.ch)
	}
	input.sendMu.Unlock()
	input.start(worker)
	<-input.finished
}

// captureSelection is permission to open this initialized input's file. Its
// zero value is invalid. Ordinary construction owns it immediately; a pending
// capture receives it only after the selected Peer has joined incumbent cleanup.
type captureSelection struct{ input *captureInput }

// captureHeaderSource distinguishes a supplied replay identity from a factory
// Session that MUST first construct its immutable local OPEN proposal. Each
// pending capture constructor installs exactly one variant before Start.
type captureHeaderSource interface{ captureHeaderSource() }

// captureHeaderSupplied is valid at its zero value: the caller has supplied
// the complete immutable header inputs to the low-level capture constructor.
type captureHeaderSupplied struct{}

func (captureHeaderSupplied) captureHeaderSource() {}

// captureHeaderProposal receives the factory Session's one successful OPEN
// proposal. Only newProposalSessionCapture constructs it, paired with its
// publisher. Its zero value and nil pointer are invalid. Close MUST unblock
// its waiting worker even if the Session never prepares an OPEN.
type captureHeaderProposal struct{ ready <-chan *preparedOpen }

func (*captureHeaderProposal) captureHeaderSource() {}

// captureProposalPublisher is the send receipt paired with a deferred header.
// Its zero value and nil pointer are invalid. prepareOpen MUST call publish
// only after successful construction, before sending any wire OPEN. The one
// slot keeps publication nonblocking; no worker or file lock is required.
type captureProposalPublisher struct{ ready chan<- *preparedOpen }

func (publisher *captureProposalPublisher) publish(proposal *preparedOpen) {
	select {
	case publisher.ready <- proposal:
	default: // The same one-shot proposal receipt is already queued.
	}
}

// pendingSessionCapture records immediately but owns no file before selection.
// It MUST start before candidate TCP processing. Selection and Close linearize
// under input.sendMu: selected-before-close drains to disk even if the worker
// runs later; close-before-selection discards without rotating an incumbent.
// The worker owns activation and file I/O. Close MUST join that worker.
type pendingSessionCapture struct {
	*captureInput
	selection    chan captureSelection
	headerSource captureHeaderSource
	registry     *Reactor
}

func (*pendingSessionCapture) protocolCapture() {}

func newPendingSessionCapture(ps *PeerSettings, coalesce bool, c clock.Clock, onDrop func(), registry *Reactor) *pendingSessionCapture {
	return &pendingSessionCapture{
		captureInput: newCaptureInput(ps, coalesce, c, onDrop),
		selection:    make(chan captureSelection, 1), registry: registry,
		headerSource: captureHeaderSupplied{},
	}
}

// newProposalSessionCapture constructs the factory's input and its proposal
// publisher together. The Session MUST retain that publisher before TCP;
// Start MUST follow construction and Close MUST join the waiting worker.
func newProposalSessionCapture(ps *PeerSettings, coalesce bool, c clock.Clock, onDrop func(), registry *Reactor) (*pendingSessionCapture, *captureProposalPublisher) {
	ready := make(chan *preparedOpen, 1)
	pending := newPendingSessionCapture(ps, coalesce, c, onDrop, registry)
	pending.headerSource = &captureHeaderProposal{ready: ready}
	return pending, &captureProposalPublisher{ready: ready}
}

func (pending *pendingSessionCapture) Start() { pending.captureInput.start(pending) }
func (pending *pendingSessionCapture) Close() { pending.captureInput.close(pending) }

// selectForPublication MUST follow incumbent capture.Close and Session
// selection. It never changes a reader callback or does file I/O. Safe against
// concurrent Close; a capture already retired cannot be activated afterwards.
func (pending *pendingSessionCapture) selectForPublication() {
	pending.sendMu.Lock()
	defer pending.sendMu.Unlock()
	if pending.closed {
		return // Retirement already sealed this optional debug input.
	}
	select {
	case pending.selection <- captureSelection{input: pending.captureInput}:
	default: // The single selected owner already requested activation.
	}
}

// captureRegistration separates an input waiting for selection from the
// receipt owning the physical file. Registry absence is a retired input.
type captureRegistration interface{ captureRegistration() }

// capturePrepared has no payload; its zero value is the valid unpublished
// registration. Only the registry constructs it.
type capturePrepared struct{}

func (capturePrepared) captureRegistration() {}

// captureFileOwner is one exclusive path receipt. Only a registered input can
// claim it, and its owner MUST close the file before releasing it. Its zero
// value and nil pointer are invalid; the registry creates every receipt.
type captureFileOwner struct {
	registry *Reactor
	target   protocolCapture
}

func (*captureFileOwner) captureRegistration() {}

var errCaptureRetired = errors.New("protocol capture input retired before file activation")
var errCaptureFileBusy = errors.New("the configured capture file is already owned by another session")

// claimCaptureFile transitions this prepared registration into a file owner
// under the same registry lock that protects every physical path reservation.
// A conflicting owner leaves it prepared. The worker MUST release a granted
// receipt after its file closes, including activation failure.
func (r *Reactor) claimCaptureFile(target *pendingSessionCapture) (*captureFileOwner, error) {
	r.sessionCapturesMu.Lock()
	defer r.sessionCapturesMu.Unlock()
	registration, present := r.sessionCaptures[target]
	if !present {
		return nil, errCaptureRetired
	}
	switch registration.(type) {
	case capturePrepared:
	case *captureFileOwner:
		return nil, errCaptureFileBusy // An input cannot own two activation receipts.
	}
	if r.captureFileOwnerLocked(target.Path()) != nil {
		return nil, errCaptureFileBusy
	}
	owner := &captureFileOwner{registry: r, target: target}
	r.sessionCaptures[target] = owner
	return owner, nil
}

// captureFileOwnerLocked projects physical ownership, not configuration-event
// registration. The caller MUST hold sessionCapturesMu. A prepared candidate
// does not prevent another selected Session from owning the file.
func (r *Reactor) captureFileOwnerLocked(path string) *captureFileOwner {
	for target, registration := range r.sessionCaptures {
		switch owner := registration.(type) {
		case capturePrepared:
		case *captureFileOwner:
			if target.Path() == path {
				return owner
			}
		}
	}
	return nil // No registered writer owns this exact path.
}

// release MUST follow file termination. The equality guard preserves a newer
// receipt if an old caller outlives the input's registry membership.
func (owner *captureFileOwner) release() {
	r := owner.registry
	r.sessionCapturesMu.Lock()
	if r.sessionCaptures[owner.target] == owner {
		r.sessionCaptures[owner.target] = capturePrepared{}
	}
	r.sessionCapturesMu.Unlock()
}

func (pending *pendingSessionCapture) runSelected(selected captureSelection) {
	switch source := pending.headerSource.(type) {
	case captureHeaderSupplied:
	case *captureHeaderProposal:
		var proposal *preparedOpen
		select {
		case proposal = <-source.ready:
		case <-pending.stop:
			// A queued proposal still belongs to a selected-before-close
			// input, even if this worker starts after Close sealed it.
			select {
			case proposal = <-source.ready:
			default:
				pending.discard()
				return
			}
		}
		pending.header.LocalAS = proposal.settings.LocalAS
		pending.header.PeerAS = proposal.settings.PeerAS
		pending.header.RouterID = proposal.message.BGPIdentifier
	}
	if pending.registry == nil {
		// Standalone constructors have no shared reactor namespace. Their
		// caller owns the directory, as for newSessionCapture.
		selected.run()
		return
	}
	owner, err := pending.registry.claimCaptureFile(pending)
	if err != nil {
		if !errors.Is(err, errCaptureRetired) {
			reportCaptureStartFailure(pending.header.Peer, pending.Path(), err)
		}
		pending.discard()
		return
	}
	defer owner.release()
	selected.run()
}

func (pending *pendingSessionCapture) run() {
	select {
	case selected := <-pending.selection:
		pending.runSelected(selected)
	case <-pending.stop:
		// A selected permission queued before Close wins even when both
		// signals became ready before this worker was scheduled.
		select {
		case selected := <-pending.selection:
			pending.runSelected(selected)
		default:
			pending.discard()
		}
	}
}

// discard is the owning worker's lifetime loop. Producer close ends it;
// at most captureQueueDepth retained events await release at that boundary.
func (input *captureInput) discard() {
	for item := range input.ch {
		input.pool.Put(item)
	}
}

func (selected captureSelection) run() {
	active, err := selected.open()
	if err != nil {
		reportCaptureStartFailure(selected.input.header.Peer, selected.input.path, err)
		// A failed optional capture must continue consuming until its owner
		// closes the producer. It cannot block or restart the BGP session.
		selected.input.discard()
		return
	}
	active.run()
}

const codeCaptureDirectory = "doctor-bgp-capture-directory"

func reportCaptureStartFailure(peer, path string, cause error) {
	err := opererr.Wrap(cause, "BGP protocol capture cannot open its configured file",
		"The BGP session continues without a protocol trace.", "ze doctor")
	slogutil.LazyLogger(captureLogSubsystem)().Error(err.Error(),
		"code", codeCaptureDirectory, "peer", peer, "file", path, "fix", "ze doctor")
	report.RaiseErrorFix(reportSourceBGP, codeCaptureDirectory, peer,
		report.FitMessage(err.Error()), "ze doctor", map[string]any{"file": path})
}

// prepareSessionCapture installs every Session's bounded input before TCP.
// Its owner MUST retire it or select it after joining incumbent capture cleanup.
// Reactor membership and registration MUST share r.mu: removalLocked closes
// the registered inputs under its write hold after deleting this exact Peer.
func (p *Peer) prepareSessionCapture(session *Session) {
	settings := p.settingsSnapshot()
	if !settings.Capture.Enabled {
		return
	}
	var onDrop func()
	if p.reactor != nil {
		if p.reactor.rmetrics != nil {
			label := settings.Address.String()
			metric := p.reactor.rmetrics.captureDroppedEvents
			onDrop = func() { metric.With(label).Inc() }
		}
	}
	pending, publisher := newProposalSessionCapture(settings, session.coalesceEnabled, p.clock, onDrop, p.reactor)
	if p.reactor != nil {
		// Construction may have started before removal. The actual registry
		// publication is admitted under the same lock as Peer membership, so
		// it either precedes the removal's close snapshot or is refused here.
		p.reactor.mu.RLock()
		if p.reactor.stopping {
			p.reactor.mu.RUnlock()
			pending.Close()
			return
		}
		if p.reactor.peers[settings.PeerKey()] != p {
			p.reactor.mu.RUnlock()
			pending.Close()
			return
		}
		p.reactor.registerCapture(pending)
		p.reactor.mu.RUnlock()
	}
	session.captureWriter = pending
	session.captureProposal = publisher
	pending.recordSession(p.clock.Now(), capture.SessionConnect)
	pending.Start()
}
