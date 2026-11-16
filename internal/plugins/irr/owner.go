// Design: docs/architecture/irr-owner.md -- the worker that schedules, fetches and stores
// Related: datasets.go -- the store reads and writes the worker makes
// Related: schedule.go -- when each dataset is due
// Related: resolver.go -- the IRR and PeeringDB clients
// Related: command.go -- the commands the worker answers
// Related: doctor.go -- the findings the worker publishes
//
// One goroutine, the worker, owns everything the owner knows: the config, the
// interest of every consumer, the pointers, bindings and datasets read from
// the store, and the schedule. It is started by OnStarted and stopped by the
// cancellation of the plugin's context; stop waits for it (stopped). Nothing
// else mutates that state: a config install and an interest notice set a flag
// and wake it, and a command runs as a closure on it. What `show irr` and the
// doctor callback read is a snapshot the worker publishes under mu.

package irr

import (
	"context"
	"fmt"
	"runtime/debug"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/component/resolve/irr/store"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/opererr"
)

// stateAccess is the SDK's state RPCs, as the owner uses them.
type stateAccess interface {
	StateGet(ctx context.Context, key string) ([]byte, bool, error)
	StatePut(ctx context.Context, key string, data []byte) error
	StateRemove(ctx context.Context, key string) error
	StateList(ctx context.Context, prefix string) ([]string, error)
	StateIncrement(ctx context.Context, key string) (uint32, error)
}

// noticeEmitter is the SDK's emit-event, as the owner uses it.
type noticeEmitter interface {
	EmitEvent(ctx context.Context, namespace, eventType, direction, peerAddress, event string) (int, error)
}

// ownerRequest is a command run on the worker. reply has depth one, so the
// worker never waits for a caller that gave up.
type ownerRequest struct {
	run   func(ctx context.Context) commandAnswer
	reply chan commandAnswer
}

// commandAnswer is what a command closure returns to its caller.
type commandAnswer struct {
	data any
	err  error
}

// datasetInfo is what the owner keeps of a readable stored dataset. The
// prefixes themselves live in the resolver, which merges the next answer
// into them.
type datasetInfo struct {
	server      string
	ipv4Count   int
	ipv6Count   int
	refreshedAt time.Time
	checkedAt   time.Time
	staleSince  time.Time
}

// problem is a finding the doctor callback reports and the worker clears when
// the cause goes away.
type problem struct {
	code  string
	since time.Time
	err   error
	// fix is the command that fixes the finding, when problemFix would name
	// another one; empty means problemFix answers.
	fix string
}

// owner is the IRR owner. Create it with newOwner, configure it at any time,
// start the worker once with start after the startup handshake, and call
// stopped after the plugin's context is canceled. Safe for concurrent use.
type owner struct {
	state       stateAccess
	notices     noticeEmitter
	newResolver func(ownerConfig) resolver
	clock       func() time.Time

	// wake has depth one: a burst of config installs and interest notices is
	// one pass of the worker.
	wake     chan struct{}
	requests chan ownerRequest
	done     chan struct{}
	running  atomic.Bool

	// mu guards the hand-offs to the worker and the published view.
	mu            sync.Mutex
	pendingConfig *ownerConfig
	interestDirty bool
	view          ownerView

	// Below: owned by the worker goroutine.
	cfg       ownerConfig
	serverKey string
	res       resolver
	// interest is each consumer's stored interest, keyed by consumer name.
	interest map[string]store.Interest
	// pointers holds every stored pointer, readable dataset or not; datasets
	// only the readable datasets they name.
	pointers map[string]store.Pointer
	datasets map[string]datasetInfo
	// bindings is every binding read or written, valid for the current URL or
	// not (validBinding judges).
	bindings map[uint32]store.Binding
	// movingTo is the set a binding moves to once that set's dataset and
	// pointer are stored (bindTo): until then the stored binding names the
	// set the consumers enforce.
	movingTo map[uint32]string
	schedule schedule
	// discoverDue arms the PeeringDB question of a referenced AS number with no
	// valid binding.
	discoverDue map[uint32]time.Time
	// answeredAt and retryAt feed setState; see there.
	answeredAt map[string]time.Time
	retryAt    map[string]time.Time
	// heldSets and heldASNs are what `clear irr` removed: not fetched again
	// until `update irr` or an interest that newly names them.
	heldSets map[string]struct{}
	heldASNs map[uint32]struct{}
	// unannounced is each set whose pointer `clear irr` removed while the
	// generation could not be raised, by the server its pointer named: no
	// consumer was told, so the next clear of the set tells them.
	unannounced map[string]string
	// problems are the doctor findings, keyed by AS-SET or state key.
	problems map[string]problem
}

// newOwner returns an owner over the plugin's state and notices, running the
// defaults until a config is installed.
func newOwner(state stateAccess, notices noticeEmitter, newResolver func(ownerConfig) resolver) *owner {
	o := &owner{
		state:       state,
		notices:     notices,
		newResolver: newResolver,
		clock:       time.Now,
		wake:        make(chan struct{}, 1),
		requests:    make(chan ownerRequest),
		done:        make(chan struct{}),
		interest:    make(map[string]store.Interest),
		pointers:    make(map[string]store.Pointer),
		datasets:    make(map[string]datasetInfo),
		bindings:    make(map[uint32]store.Binding),
		movingTo:    make(map[uint32]string),
		schedule:    newSchedule(),
		discoverDue: make(map[uint32]time.Time),
		answeredAt:  make(map[string]time.Time),
		retryAt:     make(map[string]time.Time),
		heldSets:    make(map[string]struct{}),
		heldASNs:    make(map[uint32]struct{}),
		unannounced: make(map[string]string),
		problems:    make(map[string]problem),
	}
	cfg := defaultOwnerConfig()
	o.pendingConfig = &cfg
	return o
}

// configure hands cfg to the worker. It is the config transaction's install:
// at boot, before the worker runs, it is the config the worker starts with.
func (o *owner) configure(cfg ownerConfig) error {
	o.mu.Lock()
	o.pendingConfig = &cfg
	o.mu.Unlock()
	o.poke()
	return nil
}

// interestChanged tells the worker a consumer rewrote its interest key. It
// never blocks, because it runs on the SDK's event loop.
func (o *owner) interestChanged() {
	o.mu.Lock()
	o.interestDirty = true
	o.mu.Unlock()
	o.poke()
}

func (o *owner) poke() {
	select {
	case o.wake <- struct{}{}:
	default:
	}
}

// start runs the worker until ctx is canceled. It MUST be called once, after
// the startup handshake (state RPCs are refused before it), and stopped MUST
// be called after ctx is canceled.
func (o *owner) start(ctx context.Context) {
	o.running.Store(true)
	go o.run(ctx)
}

// stopped waits for the worker started by start to return. It MUST be called
// after the context start took is canceled; with no worker it returns at once.
func (o *owner) stopped() {
	if o.running.Load() {
		<-o.done
	}
}

// do runs fn on the worker and returns its answer. Before start, and after
// the worker stopped, it refuses rather than waiting for nobody.
func (o *owner) do(fn func(ctx context.Context) commandAnswer) commandAnswer {
	if !o.running.Load() {
		return commandAnswer{err: errNotStarted()}
	}
	request := ownerRequest{run: fn, reply: make(chan commandAnswer, 1)}
	select {
	case o.requests <- request:
	case <-o.done:
		return commandAnswer{err: errNotStarted()}
	}
	select {
	case answer := <-request.reply:
		return answer
	case <-o.done:
		return commandAnswer{err: errNotStarted()}
	}
}

// errNotStarted refuses a command that reaches the owner outside its worker's
// life: inside the startup handshake or while the daemon stops.
func errNotStarted() error {
	return opererr.New("the irr plugin is not running",
		"It answers only after its startup has finished and before the daemon stops.",
		"show irr")
}

// run is the worker. It loads what the store holds, then loops until ctx is
// canceled: that loop has no other end, by design.
func (o *owner) run(ctx context.Context) {
	defer close(o.done)
	o.absorb(ctx)
	o.load(ctx)
	// The first view is published before the first fetch: show irr and the
	// doctor refuse until a view exists (showIRR), and a cold fetch can hold
	// the worker for the whole whois timeout.
	o.publishView()
	timer := time.NewTimer(time.Hour)
	defer timer.Stop()
	for {
		o.absorb(ctx)
		o.fetchDue(ctx)
		o.publishView()
		o.publishMetrics()
		o.arm(timer)
		select {
		case <-ctx.Done():
			return
		case <-o.wake:
		case request := <-o.requests:
			request.reply <- o.serve(ctx, request)
			o.publishView()
		case <-timer.C:
		}
	}
}

// serve runs one command closure on the worker. A panic under it costs that
// command and nothing more: the caller gets an error answer, and the worker
// goes on serving. Unrecovered, it would end the worker goroutine, which the
// SDK's dispatch recovery does not cover, and with it the plugin process.
func (o *owner) serve(ctx context.Context, request ownerRequest) (answer commandAnswer) {
	defer func() {
		recovered := recover()
		if recovered == nil {
			return
		}
		logger().Error("irr: a command panicked on the worker, which keeps running",
			"panic", recovered, "stack", string(debug.Stack()), "code", diagnostic.CodeDoctorInternalDefect)
		answer = commandAnswer{err: opererr.New("the irr plugin failed to answer the command",
			"A defect in the irr plugin stopped the command; the stored lists did not change.",
			"show irr")}
	}()
	return request.run(ctx)
}

// arm sets timer to the earliest due fetch or discovery, or leaves it stopped
// when nothing is armed (refresh-interval 0 with every set stored).
func (o *owner) arm(timer *time.Timer) {
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
	next, ok := o.schedule.next()
	for _, at := range o.discoverDue {
		if !ok || at.Before(next) {
			next, ok = at, true
		}
	}
	if !ok {
		return
	}
	timer.Reset(max(next.Sub(o.clock()), 0))
}

// absorb installs a pending config and re-reads the interest keys after a
// notice, then plans again. It is where every hand-off reaches the worker.
func (o *owner) absorb(ctx context.Context) {
	o.mu.Lock()
	cfg := o.pendingConfig
	dirty := o.interestDirty
	o.pendingConfig, o.interestDirty = nil, false
	o.mu.Unlock()
	if cfg != nil {
		o.installConfig(ctx, *cfg)
	}
	if dirty {
		o.readInterest(ctx)
	}
	if cfg != nil || dirty {
		o.plan()
	}
}

// installConfig makes cfg the worker's config. A new server gets a new
// resolver and forgets the answers of the old one; the datasets of the old
// server stay enforced until the new server's copy flips each pointer
// (writeDataset removes the old dataset then). The new resolver is seeded
// with every stored dataset of the new server, as load seeds the first: a
// server changed and changed back still merges the next answer per family
// into the copy it stored (AC-7).
func (o *owner) installConfig(ctx context.Context, cfg ownerConfig) {
	serverKey, err := store.DatasetServer(cfg.server)
	if err != nil {
		// parseOwnerConfig refused this server at verify, so only a defect
		// reaches here; keep the config that is running.
		logger().Error("irr: a verified server cannot name a dataset, keeping the running config",
			"server", cfg.server, "error", err, "code", diagnostic.CodeDoctorInternalDefect)
		return
	}
	previous := o.serverKey
	o.cfg, o.serverKey = cfg, serverKey
	if o.res == nil || previous != serverKey {
		o.res = o.newResolver(cfg)
		clear(o.answeredAt)
		clear(o.retryAt)
		for asSet := range o.pointers {
			o.seedStored(ctx, asSet)
		}
		return
	}
	o.res.useClients(cfg)
}

// plan computes the schedule from what the worker knows, and drops the
// findings of what nothing names any more. It makes no call.
func (o *owner) plan() {
	now := o.clock()
	o.schedule = newSchedule()
	wantedSets := o.wantedSets()
	for asSet := range wantedSets {
		if _, held := o.heldSets[asSet]; held {
			continue
		}
		if due, ok := dueOf(o.stateOf(asSet), o.cfg, now); ok {
			o.schedule.set(asSet, due)
		}
	}
	wanted := o.wantedASNs()
	for asn := range o.discoverDue {
		if _, ok := wanted[asn]; !ok {
			delete(o.discoverDue, asn)
		}
	}
	for asn := range o.movingTo {
		if _, ok := wanted[asn]; !ok {
			delete(o.movingTo, asn)
		}
	}
	for asn := range wanted {
		if _, held := o.heldASNs[asn]; held {
			continue
		}
		if _, ok := o.validBinding(asn); ok {
			continue
		}
		if _, moving := o.movingTo[asn]; moving {
			// PeeringDB answered: the set it named is fetched, and the
			// binding is written once that set is stored. PeeringDB is asked
			// again when the set the stored binding names is due
			// (rediscoverBoundTo, over enforcedBinding).
			continue
		}
		if _, armed := o.discoverDue[asn]; !armed {
			o.discoverDue[asn] = now
		}
	}
	o.dropUnnamedFindings(wantedSets, wanted)
}

// dropUnnamedFindings removes the findings of every AS-SET no consumer names
// and no pointer names, and of every binding of an AS number no consumer
// names: nothing fetches them again, so a finding there would never clear,
// and no consumer enforces what it describes. An unreadable finding under a
// stored key stays: that key is still stored and still unreadable.
func (o *owner) dropUnnamedFindings(wantedSets map[string]struct{}, wantedASNs map[uint32]struct{}) {
	for subject, found := range o.problems {
		if found.code == codeIRRDatasetUnreadable {
			continue
		}
		if number, ok := strings.CutPrefix(subject, store.BindingPrefix); ok {
			if asn, parsed := parseASNSegment(number); parsed {
				if _, named := wantedASNs[asn]; !named {
					delete(o.problems, subject)
				}
			}
			continue
		}
		if !isASSetSubject(subject) {
			continue
		}
		if _, named := wantedSets[subject]; named {
			continue
		}
		if _, pointed := o.pointers[subject]; pointed {
			continue
		}
		delete(o.problems, subject)
		delete(o.unannounced, subject)
	}
}

// stateOf returns what the schedule needs of asSet.
func (o *owner) stateOf(asSet string) setState {
	state := setState{answeredAt: o.answeredAt[asSet], retryAt: o.retryAt[asSet]}
	if info, ok := o.datasets[asSet]; ok && info.server == o.serverKey {
		state.stored, state.storedCheckedAt = true, info.checkedAt
	}
	return state
}

// wantedSets is every AS-SET a consumer enforces: named, or bound to an AS
// number it names (enforcedBinding), and every set such a binding is moving
// to.
func (o *owner) wantedSets() map[string]struct{} {
	wanted := make(map[string]struct{})
	for _, in := range o.interest {
		for _, asSet := range in.ASSets {
			wanted[asSet] = struct{}{}
		}
	}
	for asn := range o.wantedASNs() {
		if _, held := o.heldASNs[asn]; held {
			continue
		}
		if binding, ok := o.enforcedBinding(asn); ok {
			wanted[binding.ASSet] = struct{}{}
		}
		if target, moving := o.movingTo[asn]; moving {
			wanted[target] = struct{}{}
		}
	}
	return wanted
}

// wantedASNs is every bare AS number a consumer enforces.
func (o *owner) wantedASNs() map[uint32]struct{} {
	wanted := make(map[uint32]struct{})
	for _, in := range o.interest {
		for _, asn := range in.ASNs {
			wanted[asn] = struct{}{}
		}
	}
	return wanted
}

// validBinding returns the binding of asn when it was learned from the
// configured PeeringDB URL. A binding from another URL reads as absent, so a
// changed endpoint is asked again.
func (o *owner) validBinding(asn uint32) (store.Binding, bool) {
	binding, ok := o.bindings[asn]
	if !ok || binding.PeeringDBURL != o.cfg.peeringDBURL {
		return store.Binding{}, false
	}
	return binding, true
}

// enforcedBinding returns the binding the consumers enforce for asn: the valid
// binding, or, while asn moves (movingTo), the stored one, whatever PeeringDB
// URL it was learned from. store.ReadBinding does not judge the URL, and a
// move writes the new binding only once its set is stored, so after a changed
// `system peeringdb url` the old binding is what the consumers read until
// then: its set stays wanted and fetched when due, and its due time asks
// PeeringDB again (rediscoverBoundTo).
func (o *owner) enforcedBinding(asn uint32) (store.Binding, bool) {
	if binding, ok := o.validBinding(asn); ok {
		return binding, true
	}
	if _, moving := o.movingTo[asn]; !moving {
		return store.Binding{}, false
	}
	binding, ok := o.bindings[asn]
	return binding, ok
}

// consumersOf names, sorted, every consumer whose interest names asSet or an
// AS number bound to it.
func (o *owner) consumersOf(asSet string) []string {
	var names []string
	for consumer, in := range o.interest {
		if slices.Contains(in.ASSets, asSet) || o.bindsTo(in.ASNs, asSet) {
			names = append(names, consumer)
		}
	}
	slices.Sort(names)
	return names
}

// consumersOfASN names, sorted, every consumer whose interest names asn.
func (o *owner) consumersOfASN(asn uint32) []string {
	var names []string
	for consumer, in := range o.interest {
		if slices.Contains(in.ASNs, asn) {
			names = append(names, consumer)
		}
	}
	slices.Sort(names)
	return names
}

func (o *owner) bindsTo(asns []uint32, asSet string) bool {
	for _, asn := range asns {
		if binding, ok := o.enforcedBinding(asn); ok && binding.ASSet == asSet {
			return true
		}
	}
	return false
}

// fetchDue asks PeeringDB for every AS number due a discovery, then fetches
// every set due by now, one at a time.
func (o *owner) fetchDue(ctx context.Context) {
	now := o.clock()
	for asn, at := range o.discoverDue {
		if ctx.Err() != nil {
			return
		}
		if at.After(now) {
			continue
		}
		o.discoverASN(ctx, asn)
	}
	o.plan()
	wanted := o.wantedSets()
	// fetched is a guard: a set a moving binding made the worker fetch early
	// in this pass is not fetched a second time when the loop reaches it.
	fetched := make(map[string]struct{})
	for _, asSet := range o.schedule.dueBy(now) {
		if ctx.Err() != nil {
			return
		}
		if _, done := fetched[asSet]; done {
			continue
		}
		dueAt, _ := o.schedule.at(asSet)
		if o.rediscoverBoundTo(ctx, asSet, dueAt) {
			// A binding moves off the set: the set it moves to is fetched
			// first, and a stored copy flips the binding (writeDataset). The
			// set is then fetched only if something still names it.
			for _, target := range o.movesOff(asSet) {
				fetched[target] = struct{}{}
				_ = o.refreshSet(ctx, target) //nolint:errcheck // refreshSet logs, raises and re-arms every failure
			}
			wanted = o.wantedSets()
		}
		if _, still := wanted[asSet]; !still {
			continue
		}
		fetched[asSet] = struct{}{}
		_ = o.refreshSet(ctx, asSet) //nolint:errcheck // refreshSet logs, raises and re-arms every failure
	}
	o.plan()
}

// movesOff answers, sorted and once each, the sets the bindings of asSet are
// moving to.
func (o *owner) movesOff(asSet string) []string {
	var targets []string
	for asn, target := range o.movingTo {
		if binding, ok := o.bindings[asn]; ok && binding.ASSet == asSet {
			targets = append(targets, target)
		}
	}
	slices.Sort(targets)
	return slices.Compact(targets)
}

// rediscoverBoundTo asks PeeringDB again for each referenced AS number bound
// to asSet, now that its dataset is due (AC-11): PeeringDB is asked when the
// data is, never at start. A binding learned at or after dueAt is not asked
// again: it is the answer that made a cold set due. A binding still moving
// counts as bound to the set the consumers enforce (enforcedBinding), so a
// move that cannot complete is asked again at each due time. It reports
// whether a binding moved, or is moving, to another set.
func (o *owner) rediscoverBoundTo(ctx context.Context, asSet string, dueAt time.Time) bool {
	moved := false
	for asn := range o.wantedASNs() {
		binding, ok := o.enforcedBinding(asn)
		if !ok || binding.ASSet != asSet {
			continue
		}
		if !binding.DiscoveredAt.Before(dueAt.Truncate(time.Second)) {
			continue
		}
		o.discoverASN(ctx, asn)
		if now, ok := o.validBinding(asn); ok && now.ASSet != asSet {
			moved = true
		}
		if _, moving := o.movingTo[asn]; moving {
			moved = true
		}
	}
	return moved
}

// discoverASN asks PeeringDB for the set of asn and binds it (bindTo). It is
// the one AS-SET discovery Ze runs for a bare AS number. A PeeringDB failure
// keeps a valid binding; with none, the literal AS<asn> is bound, so the set
// is fixed until the next due time rather than flipping per outage. A failure
// is a doctor finding under the binding's key until PeeringDB answers.
func (o *owner) discoverASN(ctx context.Context, asn uint32) {
	delete(o.discoverDue, asn)
	defer func() {
		if recovered := recover(); recovered != nil {
			o.discoverPanicked(asn, recovered)
		}
	}()
	asSet, err := o.res.discover(ctx, asn)
	if err != nil {
		o.raise(store.BindingKey(asn), codeIRRRefreshOverdue, err)
		if _, ok := o.validBinding(asn); ok {
			logger().Warn("irr: PeeringDB did not answer, keeping the stored AS-SET",
				"asn", asn, "error", err, "code", codeIRRRefreshOverdue, "fix", "update irr asn "+asnText(asn))
			return
		}
		logger().Warn("irr: PeeringDB did not answer and nothing is bound, querying the AS number itself",
			"asn", asn, "error", err, "code", codeIRRRefreshOverdue, "fix", "update irr asn "+asnText(asn))
		asSet = literalASSet(asn)
	} else {
		o.resolve(store.BindingKey(asn), codeIRRRefreshOverdue)
	}
	o.bindTo(ctx, asn, asSet)
}

// bindTo binds asn to asSet. The consumers read the stored binding, and one
// naming a set nothing stored yet reads as no list (store.ErrNotStored), so a
// binding that moves from a stored binding to a set with no pointer is not
// written: it waits in movingTo, the set is fetched, and writeDataset writes
// it once that set's pointer is stored. Until then the stored binding still
// names the set the consumers enforce. A first binding has no list to keep and
// is written at once. A move PeeringDB takes back, naming the stored set
// again, writes that binding again: the write clears the finding of the move
// that could not complete (reportStuckMoves).
func (o *owner) bindTo(ctx context.Context, asn uint32, asSet string) {
	_, wasMoving := o.movingTo[asn]
	delete(o.movingTo, asn)
	stored, bound := o.bindings[asn]
	if bound && stored.ASSet == asSet {
		if stored.PeeringDBURL != o.cfg.peeringDBURL {
			o.writeBinding(ctx, asn, asSet)
			return
		}
		if wasMoving {
			o.writeBinding(ctx, asn, asSet)
		}
		return
	}
	if _, pointed := o.pointers[asSet]; bound && !pointed {
		o.movingTo[asn] = asSet
		return
	}
	o.writeBinding(ctx, asn, asSet)
}

// discoverPanicked records a PeeringDB question about asn that panicked: a
// defect in a client or a parser, which costs that question and nothing more.
// The binding stored before stays, and the question is asked again in a
// minute rather than on the next pass, which would panic in a loop.
func (o *owner) discoverPanicked(asn uint32, recovered any) {
	o.discoverDue[asn] = o.clock().Add(retryInterval)
	o.raise(store.BindingKey(asn), codeIRRRefreshOverdue, fmt.Errorf("the PeeringDB question about AS%d panicked: %v", asn, recovered))
	countOutcome(outcomePanic)
	logger().Error("irr: the PeeringDB question about an AS number panicked, the stored AS-SET stays and the question is asked again in a minute",
		"asn", asn, "panic", recovered, "stack", string(debug.Stack()),
		"code", codeIRRRefreshOverdue, "fix", "update irr asn "+asnText(asn))
}

// refreshSet fetches asSet now, stores what the answer leaves to enforce and
// re-arms the set. It returns the fetch's error, or the store's. A panic under
// the fetch or the store is recovered here, per fetch (fetchPanicked). Every
// outcome that leaves a binding moving to asSet is reported, after the
// recovery (reportStuckMoves).
func (o *owner) refreshSet(ctx context.Context, asSet string) (err error) {
	now := o.clock()
	defer o.reportStuckMoves(ctx, asSet)
	defer func() {
		if recovered := recover(); recovered != nil {
			err = o.fetchPanicked(asSet, now, recovered)
		}
	}()
	entry, err := o.res.refresh(ctx, asSet)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil && !isEmptyAnswer(err) {
		o.retryAt[asSet] = now.Add(retryInterval)
		o.raise(asSet, codeIRRRefreshOverdue, err)
		countOutcome(outcomeError)
		logger().Warn("irr: the IRR server did not answer, the stored list stays enforced and the fetch is retried in a minute",
			"as-set", asSet, "server", o.cfg.server, "error", err,
			"code", codeIRRRefreshOverdue, "fix", "update irr as-set "+asSet)
		return err
	}
	delete(o.retryAt, asSet)
	o.resolve(asSet, codeIRRRefreshOverdue)
	if entry == nil || (len(entry.IPv4) == 0 && len(entry.IPv6) == 0) {
		// The server answered with nothing and nothing is kept: no dataset is
		// written, an empty list being a reject-everything list, and the set
		// waits a full interval like any answer.
		o.answeredAt[asSet] = now
		countOutcome(outcomeEmpty)
		return err
	}
	if storeErr := o.writeDataset(ctx, asSet, *entry, now); storeErr != nil {
		countOutcome(outcomeStoreFailed)
		return storeErr
	}
	delete(o.answeredAt, asSet)
	// An empty answer over a stored list writes the kept list again, marked
	// stale, and still counts as empty: a stored count would read as fresh
	// data nobody confirmed.
	if isEmptyAnswer(err) {
		countOutcome(outcomeEmpty)
		return err
	}
	countOutcome(outcomeStored)
	return err
}

// reportStuckMoves raises a doctor-irr-dataset-unpublished finding under the
// binding of each AS number moving to asSet, once a fetch of asSet left it with
// no pointer: PeeringDB named asSet, and the server gave no list Ze can store
// for it (no answer, an empty one, one over the caps, a refused write). The
// stored binding then still names the set the consumers enforce, and without
// the finding the move would wait unseen. writeBinding clears it once the
// binding is written; the move is tried again when the enforced set is due.
func (o *owner) reportStuckMoves(ctx context.Context, asSet string) {
	if ctx.Err() != nil {
		return
	}
	if _, pointed := o.pointers[asSet]; pointed {
		return
	}
	for asn, target := range o.movingTo {
		if target != asSet {
			continue
		}
		enforced := o.bindings[asn].ASSet
		err := fmt.Errorf("PeeringDB names %s for AS%d, but the IRR server gave no list of %s that Ze can store, so AS%d stays bound to %s",
			asSet, asn, asSet, asn, enforced)
		o.raise(store.BindingKey(asn), codeIRRDatasetUnpublished, err)
		logger().Warn("irr: an AS number cannot move to the AS-SET PeeringDB names, the consumers keep enforcing the AS-SET bound before",
			"asn", asn, "as-set", enforced, "moving-to", asSet,
			"code", codeIRRDatasetUnpublished, "fix", "update irr asn "+asnText(asn))
	}
}

// fetchPanicked records a fetch of asSet that panicked: a defect in a client
// or a parser, which costs that fetch and nothing more. The worker keeps
// running, the pointer still names the copy stored before (writeDataset flips
// it only after the dataset is stored), and the set is retried in a minute like
// a failed fetch.
func (o *owner) fetchPanicked(asSet string, now time.Time, recovered any) error {
	err := fmt.Errorf("the fetch of %s panicked: %v", asSet, recovered)
	o.retryAt[asSet] = now.Add(retryInterval)
	o.raise(asSet, codeIRRRefreshOverdue, err)
	countOutcome(outcomePanic)
	logger().Error("irr: the fetch of an AS-SET panicked, the stored list stays enforced and the fetch is retried in a minute",
		"as-set", asSet, "panic", recovered, "stack", string(debug.Stack()),
		"code", codeIRRRefreshOverdue, "fix", "update irr as-set "+asSet)
	return err
}

// raise records a doctor finding under key, keeping the time it first held;
// problemFix names its fix.
func (o *owner) raise(key, code string, err error) {
	o.raiseFix(key, code, err, "")
}

// raiseFix records a doctor finding under key whose fix is not the one
// problemFix names for its code.
func (o *owner) raiseFix(key, code string, err error, fix string) {
	since := o.clock()
	if previous, ok := o.problems[key]; ok && previous.code == code {
		since = previous.since
	}
	o.problems[key] = problem{code: code, since: since, err: err, fix: fix}
}

// resolve clears the finding of key when it carries code.
func (o *owner) resolve(key, code string) {
	if previous, ok := o.problems[key]; ok && previous.code == code {
		delete(o.problems, key)
	}
}
