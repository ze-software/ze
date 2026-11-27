// Design: docs/architecture/firewall/firewall-irr.md -- reading the stored lists the irr plugin writes
// Overview: irr.go -- applyTables, the one path that programs what the reader read
// Related: doctor.go -- what the reader reports when a stored value cannot be used
//
// The firewall fetches nothing. The irr plugin (internal/plugins/irr) fetches,
// schedules and stores every AS-SET a consumer enforces; this plugin tells it
// which ones through its interest key, and reads what it stored through the
// shared reader in internal/component/resolve/irr/store (consumer.go), under
// the contract docs/architecture/irr-owner.md "What a consumer reads" states.

package irr

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/component/resolve/irr/store"
)

// noticeGenerationsMax bounds the notice generations queued between two
// passes. A burst past it is read as a gap: every pointer is read again.
const noticeGenerationsMax = 4096

// retryInterval is how long the reader waits before writing again an interest
// key the store refused, or applying again tables the firewall refused.
const retryInterval = time.Minute

// consumerState is what the reader needs of the SDK: reads of the irr
// plugin's keys, and the write and notice of its own interest key.
type consumerState interface {
	store.StateReader
	store.InterestPublisher
}

// storeReader reads the stored lists into the plugin's references, programs
// them, and writes the plugin's interest key. Reads and writes run on a pass,
// never under plug.mu, so a slow store never holds the lock show and the
// doctor read.
//
// Lifecycle: newStoreReader builds it; run is its one long-lived worker,
// started by OnStarted, and returns when its context is canceled. passNow runs
// one pass on the caller's goroutine, for a commit's apply. Passes are
// serialized by passMu. readAll and notice are safe for concurrent use and
// never block.
type storeReader struct {
	plug  *irrPlugin
	state consumerState

	// wake has depth 1: requests that arrive during a pass collapse into the
	// next one.
	wake chan struct{}

	mu          sync.Mutex
	all         bool                // read every reference again
	reapply     bool                // apply the tables even when no copy changed
	interest    bool                // write the interest key again
	sets        map[string]struct{} // AS-SETs a notice named since the last pass
	generations []uint32            // the generations those notices carried

	// passMu serializes passes: the worker's and a commit's.
	passMu sync.Mutex
	// seen is the highest generation the reader has applied. Written under
	// passMu; read by the doctor callback.
	seen atomic.Uint32
	// written is the interest value last stored; under passMu.
	written []byte
	// programmed is false until a pass applied the tables with success; under
	// passMu. The first pass after the start always applies.
	programmed bool
}

func newStoreReader(plug *irrPlugin, state consumerState) *storeReader {
	return &storeReader{
		plug:  plug,
		state: state,
		wake:  make(chan struct{}, 1),
		sets:  make(map[string]struct{}),
	}
}

// readAll asks for every reference to be read again and the interest key to be
// written: after the start.
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
	// refuse-by-default: guards readEvery -- a dataset notice that fails to decode already sets r.all so the next pass reads every stored list
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
	reapply     bool
	interest    bool
	sets        map[string]struct{}
	generations []uint32
}

func (r *storeReader) take() readerRequest {
	r.mu.Lock()
	defer r.mu.Unlock()
	request := readerRequest{all: r.all, reapply: r.reapply, interest: r.interest, sets: r.sets, generations: r.generations}
	r.all, r.reapply, r.interest = false, false, false
	r.sets = make(map[string]struct{})
	r.generations = nil
	return request
}

// run is the worker. It returns when ctx is canceled: the loop has no other
// end, by design, as it lives as long as the plugin.
func (r *storeReader) run(ctx context.Context) {
	retry := time.NewTimer(retryInterval)
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
		if r.pass(ctx) != nil {
			retry.Reset(retryInterval)
		}
	}
}

// passNow reads every reference, applies the tables and writes the interest
// key, on the caller's goroutine. A commit's apply calls it, so the commit
// answers whether its tables reached the firewall. It applies even when no
// copy changed: a commit can move a reference to another table, or add an
// interface binding, over the copies already enforced.
func (r *storeReader) passNow(ctx context.Context) error {
	r.mu.Lock()
	r.all, r.reapply, r.interest = true, true, true
	r.mu.Unlock()
	return r.pass(ctx)
}

// pass serves one request: it reads what the request names, applies the
// tables when a copy changed or the last apply did not succeed, and writes the
// interest key when asked. It answers the apply's error first, then the
// interest write's, so the worker retries either.
func (r *storeReader) pass(ctx context.Context) error {
	r.passMu.Lock()
	defer r.passMu.Unlock()
	request := r.take()
	changed := false
	switch {
	case request.all:
		changed = r.readEvery(ctx)
		r.advance(request.generations)
	case r.advance(request.generations):
		// A generation was skipped: a notice was lost, so any pointer may have
		// moved (R-3).
		changed = r.readEvery(ctx)
	case len(request.sets) > 0:
		changed = r.readNamed(ctx, request.sets)
	}
	var applyErr error
	if changed || request.reapply || !r.programmed {
		applyErr = r.apply()
	}
	if !request.interest {
		return applyErr
	}
	if err := r.writeInterest(ctx); err != nil && applyErr == nil {
		return err
	}
	return applyErr
}

// apply programs the tables and records the outcome for the doctor callback.
func (r *storeReader) apply() error {
	err := r.applyRecovered()
	if err != nil {
		r.programmed = false
		r.plug.applyErr.Store(&applyFailure{err: err.Error(), since: time.Now()})
		if errors.Is(err, errOversized) {
			logger().Warn("firewall-irr: the firewall tables naming IRR sets were not programmed; retried in a minute",
				"error", err, "code", codeIRROversizedData, "fix", "show firewall irr")
			return err
		}
		logger().Warn("firewall-irr: the firewall tables naming IRR sets were not programmed; retried in a minute",
			"error", err, "code", codeIRRNotProgrammed, "fix", cmdUpdateIRRAll)
		return err
	}
	r.programmed = true
	r.plug.applyErr.Store(nil)
	return nil
}

// applyRecovered runs applyTables and turns a panic under it, in a set builder
// or a firewall backend, into the error of that apply. The pass then ends as
// a failed apply does: the copies read stay recorded, the doctor reports the
// tables not programmed, and the next pass applies again. Unrecovered, the
// panic would end the worker goroutine, which the SDK's dispatch recovery does
// not cover, and with it the plugin process.
func (r *storeReader) applyRecovered() (err error) {
	defer func() {
		recovered := recover()
		if recovered == nil {
			return
		}
		logger().Error("firewall-irr: programming the firewall tables naming IRR sets panicked; the stored lists stay read and the apply is retried",
			"panic", recovered, "stack", string(debug.Stack()), "code", codeIRRNotProgrammed, "fix", cmdUpdateIRRAll)
		err = fmt.Errorf("firewall-irr: apply panicked: %v", recovered)
	}()
	return r.plug.applyTables()
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

// readEvery reads every reference. The generation is read first, so a pointer
// written while the pass runs carries a generation above it and its notice is
// no gap.
func (r *storeReader) readEvery(ctx context.Context) bool {
	generation, err := store.ReadGeneration(ctx, r.state)
	if err != nil {
		logger().Warn("irr: the irr plugin's generation cannot be read, the stored lists are read anyway",
			"error", err, "code", codeIRRStoredCopyBehind, "fix", cmdUpdateIRRAll)
	} else {
		r.seen.Store(max(r.seen.Load(), generation))
	}
	return r.read(ctx, func(readTarget) bool { return true })
}

// readNamed reads the references to a set a notice named, and every ASN
// reference: a binding that moved is announced only by the notice of the set
// it moved to.
func (r *storeReader) readNamed(ctx context.Context, sets map[string]struct{}) bool {
	return r.read(ctx, func(target readTarget) bool {
		if target.configured == "" {
			return true
		}
		_, named := sets[target.configured]
		return named
	})
}

// readTarget is what the reader captured of one reference before reading.
type readTarget struct {
	name       string
	configured string
	asn        uint32
	current    *enforcedCopy // the copy enforced now, nil for none
}

// readOutcome is what reading one reference left to apply.
type readOutcome struct {
	unchanged bool // the pointer names the copy enforced now
	copy      *enforcedCopy
	notStored bool  // nothing is stored: enforce no list
	err       error // a stored value could not be used: keep what is enforced
}

// read reads every reference wanted selects and records what it found. It
// reports whether an enforced copy changed.
func (r *storeReader) read(ctx context.Context, wanted func(readTarget) bool) bool {
	targets := r.plug.readTargets(wanted)
	outcomes := make(map[string]readOutcome, len(targets))
	for _, target := range targets {
		outcomes[target.name] = r.readTarget(ctx, target)
	}
	return r.plug.record(targets, outcomes)
}

// readTarget reads one reference: its binding for an ASN reference, then the
// set's pointer, and the dataset unless the pointer names the copy enforced
// now.
func (r *storeReader) readTarget(ctx context.Context, target readTarget) readOutcome {
	asSet := target.configured
	if asSet == "" {
		binding, err := store.ReadBinding(ctx, r.state, target.asn)
		if err != nil {
			return outcomeOf(err)
		}
		asSet = binding.ASSet
	}
	pointer, err := store.ReadPointer(ctx, r.state, asSet)
	if err != nil {
		return outcomeOf(err)
	}
	if target.current != nil && target.current.asSet == asSet && target.current.pointer == pointer {
		return readOutcome{unchanged: true}
	}
	stored, err := store.ReadDataset(ctx, r.state, asSet, pointer)
	if err != nil {
		return readOutcome{err: err}
	}
	return readOutcome{copy: copyOf(target.name, asSet, stored)}
}

// outcomeOf maps a failed read: "nothing stored" is an answer, every other
// error a stored value that cannot be used.
func outcomeOf(err error) readOutcome {
	if errors.Is(err, store.ErrNotStored) {
		return readOutcome{notStored: true}
	}
	return readOutcome{err: err}
}

// readTargets captures, under plug.mu, every keyed reference wanted selects.
func (plug *irrPlugin) readTargets(wanted func(readTarget) bool) []readTarget {
	plug.mu.RLock()
	defer plug.mu.RUnlock()
	targets := make([]readTarget, 0, len(plug.refs))
	for name, st := range plug.refs {
		if st.keyErr != "" {
			continue
		}
		target := readTarget{name: name, configured: st.configured, asn: st.asn, current: st.copy}
		if wanted(target) {
			targets = append(targets, target)
		}
	}
	slices.SortFunc(targets, func(a, b readTarget) int {
		switch {
		case a.name < b.name:
			return -1
		case a.name > b.name:
			return 1
		}
		return 0
	})
	return targets
}

// record applies the outcomes to the references still configured and reports
// whether an enforced copy changed. A reference a configure removed meanwhile
// is skipped: its read answers a config that is gone.
func (plug *irrPlugin) record(targets []readTarget, outcomes map[string]readOutcome) bool {
	plug.mu.Lock()
	defer plug.mu.Unlock()
	changed := false
	for _, target := range targets {
		st, ok := plug.refs[target.name]
		if !ok || st.configured != target.configured || st.asn != target.asn {
			continue
		}
		outcome := outcomes[target.name]
		switch {
		case outcome.err != nil:
			st.readErr = outcome.err.Error()
			logger().Warn("firewall-irr: a stored IRR list cannot be used, the firewall keeps what it enforces",
				"name", target.name, "error", outcome.err, "code", codeIRRStoredCopyUnreadable, "fix", fetchCommand(st.ref))
		case outcome.notStored:
			st.readErr = ""
			if st.copy != nil {
				st.copy = nil
				changed = true
			}
			logger().Info("firewall-irr: no list is stored yet, the rules naming it are held back until the irr plugin stores one",
				"name", target.name, "fetch", fetchCommand(st.ref))
		case outcome.unchanged:
			st.readErr = ""
		default:
			st.readErr = ""
			st.copy = outcome.copy
			changed = true
		}
	}
	return changed
}

// interest is what the plugin enforces, for its interest key: every keyed
// AS-SET and every AS number its references name, each once, sorted. The
// references are a map, so without the sort two passes over one config
// encode two orders, writeInterest reads that as a change, and every pass
// rewrites the key and notifies the irr plugin; two references to one set
// (or one AS number) would name it twice. The BGP filter's interest builds
// the same shape.
func (plug *irrPlugin) interest() store.Interest {
	plug.mu.RLock()
	defer plug.mu.RUnlock()
	in := store.Interest{ASSets: []string{}, ASNs: []uint32{}}
	for _, st := range plug.refs {
		switch {
		case st.keyErr != "":
		case st.configured != "":
			in.ASSets = append(in.ASSets, st.configured)
		default:
			in.ASNs = append(in.ASNs, st.asn)
		}
	}
	slices.Sort(in.ASSets)
	in.ASSets = slices.Compact(in.ASSets)
	slices.Sort(in.ASNs)
	in.ASNs = slices.Compact(in.ASNs)
	return in
}

// writeInterest stores the sets and bare AS numbers the plugin enforces, when
// they changed, and tells the irr plugin.
func (r *storeReader) writeInterest(ctx context.Context) error {
	interest := r.plug.interest()
	encoded, err := store.EncodeInterest(interest)
	if err == nil && slices.Equal(encoded, r.written) {
		return nil
	}
	if err == nil {
		err = store.PublishInterest(ctx, r.state, pluginName, interest)
	}
	if err != nil {
		r.plug.interestErr.Store(&interestFailure{err: err.Error(), since: time.Now()})
		logger().Warn("firewall-irr: the AS-SETs this plugin enforces were not saved, so the irr plugin may not refresh them; retried in a minute",
			"error", err, "code", codeIRRInterestUnsaved, "fix", cmdUpdateIRRAll)
		return err
	}
	r.written = encoded
	r.plug.interestErr.Store(nil)
	return nil
}
