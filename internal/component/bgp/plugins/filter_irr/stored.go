// Design: docs/architecture/bgp/filter-irr.md -- reading the stored lists the irr plugin writes
// Overview: filter_irr.go -- publishLocked, the one publish path the reader feeds
// Related: doctor.go -- what the reader reports when a stored value cannot be used
//
// The filter fetches nothing. The irr plugin (internal/plugins/irr) fetches,
// schedules and stores every AS-SET a consumer enforces; the filter tells it
// which ones through its interest key, and reads what it stored through the
// shared reader in internal/component/resolve/irr/store (consumer.go).

package filter_irr

import (
	"context"
	"errors"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/component/resolve/irr"
	"github.com/ze-software/ze/internal/component/resolve/irr/store"
)

// noticeGenerationsMax bounds the notice generations queued between two
// passes. A burst past it is read as a gap: every pointer is read again.
const noticeGenerationsMax = 4096

// interestRetryInterval is how long the reader waits before writing again an
// interest key the store refused.
const interestRetryInterval = time.Minute

// consumerState is what the reader needs of the SDK: reads of the irr
// plugin's keys, and the write and notice of its own interest key.
type consumerState interface {
	store.StateReader
	store.InterestPublisher
}

// storeReader reads the stored lists into the filter's states and publishes
// them, and writes the filter's interest key. Reads and writes run on its one
// worker, never on the configure path or under plug.mu, so a slow store never
// delays a publish or a filter verdict (AC-18).
//
// Lifecycle: newStoreReader builds it; run is its one long-lived worker,
// started by OnStarted, and returns when its context is canceled. readAll and
// notice are safe for concurrent use with each other and with run, and never
// block.
type storeReader struct {
	plug  *irrPlugin
	state consumerState

	// wake has depth 1: requests that arrive during a pass collapse into the
	// next one.
	wake chan struct{}

	mu          sync.Mutex
	all         bool                // read every enrolled ASN again
	interest    bool                // write the interest key again
	sets        map[string]struct{} // AS-SETs a notice named since the last pass
	generations []uint32            // the generations those notices carried

	// seen is the highest generation the reader has applied. Written by the
	// worker only; read by the doctor callback.
	seen atomic.Uint32
	// written is the interest value last stored; worker only.
	written []byte
}

func newStoreReader(plug *irrPlugin, state consumerState) *storeReader {
	return &storeReader{
		plug:  plug,
		state: state,
		wake:  make(chan struct{}, 1),
		sets:  make(map[string]struct{}),
	}
}

// readAll asks for every enrolled ASN to be read again and the interest key to
// be written: after the start and after every configure.
func (r *storeReader) readAll() {
	r.mu.Lock()
	r.all, r.interest = true, true
	r.mu.Unlock()
	r.poke()
}

// notice queues the AS-SET an irr/dataset-stored notice names. A notice that
// cannot be read is answered by reading everything again: it said something
// changed, and not what.
func (r *storeReader) notice(event string) {
	decoded, err := store.DecodeDatasetNotice(event)
	r.mu.Lock()
	switch {
	case err != nil:
		r.all = true
	case len(r.generations) >= noticeGenerationsMax:
		r.all = true
	default:
		r.sets[decoded.ASSet] = struct{}{}
		r.generations = append(r.generations, decoded.Generation)
	}
	r.mu.Unlock()
	// refuse-by-default: guards readEvery -- unreadable notice already sets r.all so pass runs readEvery to reconcile every stored list and report any unreadable store copy
	if err != nil {
		logger().Debug("irr: a dataset notice could not be read, every stored list is read again", "error", err)
	}
	r.poke()
}

func (r *storeReader) poke() {
	select {
	case r.wake <- struct{}{}:
	default:
	}
}

// readerRequest is what one pass takes from the queue.
type readerRequest struct {
	all         bool
	interest    bool
	sets        map[string]struct{}
	generations []uint32
}

func (r *storeReader) take() readerRequest {
	r.mu.Lock()
	defer r.mu.Unlock()
	request := readerRequest{all: r.all, interest: r.interest, sets: r.sets, generations: r.generations}
	r.all, r.interest = false, false
	r.sets = make(map[string]struct{})
	r.generations = nil
	return request
}

// run is the worker. It returns when ctx is canceled: the loop has no other
// end, by design, as it lives as long as the filter.
func (r *storeReader) run(ctx context.Context) {
	retry := time.NewTimer(interestRetryInterval)
	retry.Stop()
	defer retry.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-r.wake:
		case <-retry.C:
			r.mu.Lock()
			r.interest = true
			r.mu.Unlock()
		}
		if !r.pass(ctx) {
			retry.Reset(interestRetryInterval)
		}
	}
}

// pass serves one request. It reports false when the interest key could not
// be written, so the worker tries again later.
func (r *storeReader) pass(ctx context.Context) bool {
	request := r.take()
	switch {
	case request.all:
		r.readEvery(ctx)
		r.advance(request.generations)
	case r.advance(request.generations):
		// A generation was skipped: a notice was lost, so any pointer may have
		// moved (R-3).
		r.readEvery(ctx)
	case len(request.sets) > 0:
		r.readNamed(ctx, request.sets)
	}
	if !request.interest {
		return true
	}
	return r.writeInterest(ctx)
}

// advance applies the generations of the notices in hand and reports whether
// one skipped a generation the reader has not seen.
func (r *storeReader) advance(generations []uint32) bool {
	slices.Sort(generations)
	seen := r.seen.Load()
	gap := false
	for _, generation := range generations {
		if generation > seen+1 {
			gap = true
		}
		seen = max(seen, generation)
	}
	r.seen.Store(seen)
	return gap
}

// readEvery reads every enrolled ASN. The generation is read first, so a
// pointer written while the pass runs carries a generation above it and its
// notice is no gap.
func (r *storeReader) readEvery(ctx context.Context) {
	generation, err := store.ReadGeneration(ctx, r.state)
	if err != nil {
		logger().Warn("irr: the irr plugin's generation cannot be read, the stored lists are read anyway",
			"error", err, "code", codeIRRStoredCopyBehind, "fix", fixFetchAll)
	} else {
		r.seen.Store(max(r.seen.Load(), generation))
	}
	r.read(ctx, func(readTarget) bool { return true })
}

// readNamed reads the ASNs filtered by a set a notice named, and every ASN
// whose set PeeringDB chooses: a binding that moved is announced only by the
// notice of the set it moved to.
func (r *storeReader) readNamed(ctx context.Context, sets map[string]struct{}) {
	r.read(ctx, func(target readTarget) bool {
		if target.configured == "" {
			return true
		}
		_, named := sets[target.configured]
		return named
	})
}

// readTarget is what the reader captured of one enrolled ASN before reading.
type readTarget struct {
	asn        uint32
	configured string
	pointer    store.Pointer // the pointer of the copy enforced now, zero for none
	asSet      string        // the set of the copy enforced now
}

// readOutcome is what reading one AS-SET left to apply.
type readOutcome struct {
	asSet     string
	unchanged bool // the pointer names the copy enforced now
	stored    store.StoredSet
	list      *irrPrefixList
	notStored bool  // nothing is stored: enforce no list
	err       error // a stored value could not be used: keep what is enforced
}

// read reads every enrolled ASN wanted selects and applies what it found. One
// set named by several ASNs is read once per pass.
func (r *storeReader) read(ctx context.Context, wanted func(readTarget) bool) {
	targets := r.plug.readTargets(wanted)
	outcomes := make(map[uint32]readOutcome, len(targets))
	bySet := make(map[string]readOutcome)
	for _, target := range targets {
		asSet := target.configured
		if asSet == "" {
			binding, err := store.ReadBinding(ctx, r.state, target.asn)
			if err != nil {
				outcomes[target.asn] = readOutcome{asSet: target.asSet, notStored: errors.Is(err, store.ErrNotStored), err: answerOf(err)}
				continue
			}
			asSet = binding.ASSet
		}
		outcome, done := bySet[asSet]
		if !done || outcome.unchanged {
			// An unchanged answer is relative to the copy one ASN enforces;
			// another ASN on the same set may enforce an older one.
			outcome = r.readSet(ctx, asSet, target)
			bySet[asSet] = outcome
		}
		outcomes[target.asn] = outcome
	}
	if r.plug.apply(targets, outcomes) {
		r.plug.readmitAfterPublish()
	}
}

// answerOf maps "nothing stored" to no error, because it is an answer: the
// owner holds nothing for the name. Every other error is returned.
func answerOf(err error) error {
	if errors.Is(err, store.ErrNotStored) {
		return nil
	}
	return err
}

// readSet reads asSet's pointer, and its dataset unless the pointer names the
// copy target enforces now.
func (r *storeReader) readSet(ctx context.Context, asSet string, target readTarget) readOutcome {
	pointer, err := store.ReadPointer(ctx, r.state, asSet)
	if err != nil {
		return readOutcome{asSet: asSet, notStored: errors.Is(err, store.ErrNotStored), err: answerOf(err)}
	}
	if asSet == target.asSet && pointer == target.pointer {
		return readOutcome{asSet: asSet, unchanged: true}
	}
	stored, err := store.ReadDataset(ctx, r.state, asSet, pointer)
	if err != nil {
		return readOutcome{asSet: asSet, err: err}
	}
	entries, err := prefixListFromIRR(irr.PrefixList{IPv4: stored.Dataset.IPv4(), IPv6: stored.Dataset.IPv6()})
	if err != nil {
		return readOutcome{asSet: asSet, err: err}
	}
	return readOutcome{asSet: asSet, stored: stored, list: &irrPrefixList{entries: entries}}
}

// writeInterest stores the sets and bare AS numbers the filter enforces, when
// they changed, and tells the irr plugin. It reports false when the store or
// the notice refused it.
func (r *storeReader) writeInterest(ctx context.Context) bool {
	interest := r.plug.interest()
	encoded, err := store.EncodeInterest(interest)
	if err == nil && slices.Equal(encoded, r.written) {
		return true
	}
	if err == nil {
		err = store.PublishInterest(ctx, r.state, pluginName, interest)
	}
	if err != nil {
		r.plug.interestErr.Store(&interestFailure{err: err.Error(), since: time.Now()})
		logger().Warn("irr: the AS-SETs this filter enforces were not saved, so the irr plugin may not fetch a newly configured one; retried in a minute",
			"error", err, "code", codeIRRInterestUnsaved, "fix", fixFetchAll)
		return false
	}
	r.written = encoded
	r.plug.interestErr.Store(nil)
	return true
}
