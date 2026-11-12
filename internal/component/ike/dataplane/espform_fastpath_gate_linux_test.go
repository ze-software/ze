// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- one Child SA receives both ESP forms
// Related: espform_fastpath_gate_linux.go -- the firewall gate under test
//
// VALIDATES: the fast path's firewall gate: it opens only from a judgment that
// began after the last hold, the last release and the last commit the watch signaled,
// with no hold open, and found no nftables rule on the input path that can drop ESP; a
// ruleset that cannot be read, a watch that ended, or a blocking rule shuts it with
// the reason in the state; a hold shuts it before hold returns; a release has the
// worker judge again; every change of the ruleset the worker sees shuts it before the
// judgment that follows, and a ruleset that keeps moving keeps it shut while the
// worker waits in its loop between judgments.
// PREVENTS: a stale "safe" judgment opening the gate after a firewall commit landed
// under it (a rule that drops ESP would then see the peer's ESP as UDP), a dump run
// under the lock the doctor's state read needs, and a gate left open once Ze can no
// longer see the ruleset.
// METHOD: no kernel. A fake reader feeds the real judgment, a scripted judgment
// orders the races exactly, and the gate writer seam records every value written.
// Waits are on the recorded writes, never on a duration.

//go:build linux

package dataplane

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/nftables"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
)

// TestESPFormFastPathGateFollowsTheKernelRuleset: the gate follows the kernel's own
// ruleset through the real reader, watch and judgment. It opens over a namespace with
// no nftables table, shuts when a commit adds an inet input chain whose policy drops,
// naming that table and chain, and opens again once the table is deleted.
//
// METHOD: a fresh network namespace on a locked thread with one dummy link; the
// ruleset is committed through the vendored nftables library, and every wait polls the
// state the worker produces, bounded by fastPathTestWait, never a sleep.
//
//ze:guest-root
func TestESPFormFastPathGateFollowsTheKernelRuleset(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("requires privileged QEMU: loading a SchedCLS program, attaching it through TCX and committing an nftables ruleset need CAP_BPF and CAP_NET_ADMIN in the initial user namespace, which userns.Enter cannot grant; run `./le test qemu all`")
	}
	enterFastPathNetns(t)
	if err := netlink.LinkAdd(&netlink.Dummy{Name: fastPathTestDummy}); err != nil {
		t.Fatalf("add %s: %v", fastPathTestDummy, err)
	}
	dummy, err := netlink.LinkByName(fastPathTestDummy)
	if err != nil {
		t.Fatalf("find %s: %v", fastPathTestDummy, err)
	}
	if err := netlink.LinkSetUp(dummy); err != nil {
		t.Fatalf("bring %s up: %v", fastPathTestDummy, err)
	}

	f := newESPFormFastPath(nil)
	t.Cleanup(f.stop)
	f.start()

	st := waitForFastPath(t, f, "the gate open over a namespace with no nftables table", func(st ESPFormFastPathState) bool {
		return st.GateOpen && slices.Contains(st.Attached, fastPathTestDummy)
	})
	if !st.GateOpen {
		t.Fatalf("the gate is shut over a namespace with no nftables table: %+v", st)
	}

	ns, err := netns.Get()
	if err != nil {
		t.Fatalf("read the test's network namespace: %v", err)
	}
	t.Cleanup(func() { ns.Close() }) //nolint:errcheck // the namespace goes with the test
	conn, err := nftables.New(nftables.WithNetNSFd(int(ns)))
	if err != nil {
		t.Fatalf("open an nftables connection: %v", err)
	}
	const tableName, chainName = "zegatekernel", "zegateinput"
	table := conn.AddTable(&nftables.Table{Name: tableName, Family: nftables.TableFamilyINet})
	drop := nftables.ChainPolicyDrop
	conn.AddChain(&nftables.Chain{
		Name:     chainName,
		Table:    table,
		Hooknum:  nftables.ChainHookInput,
		Priority: nftables.ChainPriorityFilter,
		Type:     nftables.ChainTypeFilter,
		Policy:   &drop,
	})
	if err := conn.Flush(); err != nil {
		t.Fatalf("commit the inet table %s with an input chain that drops by policy: %v", tableName, err)
	}

	st = waitForFastPath(t, f, "the gate shut by an input chain whose policy drops", func(st ESPFormFastPathState) bool {
		return st.FirewallBlocked
	})
	if st.GateOpen {
		t.Errorf("the gate is open over an input chain whose policy drops: %+v", st)
	}
	if !strings.Contains(st.FirewallSite, tableName) || !strings.Contains(st.FirewallSite, chainName) {
		t.Errorf("FirewallSite %q does not name the table %s and the chain %s", st.FirewallSite, tableName, chainName)
	}

	conn.DelTable(table)
	if err := conn.Flush(); err != nil {
		t.Fatalf("delete the inet table %s: %v", tableName, err)
	}
	st = waitForFastPath(t, f, "the gate open again after the table was deleted", func(st ESPFormFastPathState) bool {
		return st.GateOpen
	})
	if st.FirewallBlocked {
		t.Errorf("the state is FirewallBlocked after the blocking table was deleted: %+v", st)
	}

	f.stop()
}

// gateTestWait bounds every wait for the worker to write the gate.
const gateTestWait = 5 * time.Second

// gateTestEpoch is the time a gateTestClock starts at.
var gateTestEpoch = time.Date(2026, time.November, 12, 10, 0, 0, 0, time.UTC)

// gateTestLive guards the reports of a harness callback that a leaked worker can still
// run after its test returned. errorf reports to the test only until the test's
// cleanup ended the guard, and end takes the same lock, so no report reaches the
// test once it is over.
type gateTestLive struct {
	mu    sync.Mutex
	ended bool
	t     *testing.T
}

// newGateTestLive answers a guard that the test's cleanup ends. Every cleanup
// registered after it runs first, so a stop registered later still reports while it
// waits.
func newGateTestLive(t *testing.T) *gateTestLive {
	live := &gateTestLive{t: t}
	t.Cleanup(live.end)
	return live
}

func (l *gateTestLive) end() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.ended = true
}

func (l *gateTestLive) errorf(format string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.ended {
		return
	}
	l.t.Errorf(format, args...)
}

// gateTestWrites records every value the fast path writes into its gate.
type gateTestWrites struct {
	mu     sync.Mutex
	values []uint32
	// overflow counts the writes wrote had no room for. A worker that writes without
	// end (a mutant that spins) then never blocks under the fast path's mu, and next
	// reports the overflow instead.
	overflow int
	wrote    chan uint32
}

// write is the writeGate seam. It runs under the fast path's mu, so it never blocks.
func (w *gateTestWrites) write(value uint32) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.values = append(w.values, value)
	select {
	case w.wrote <- value:
	default:
		w.overflow++
	}
	return nil
}

// overflowed answers how many writes wrote had no room for.
func (w *gateTestWrites) overflowed() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.overflow
}

// all answers a copy of the values written so far.
func (w *gateTestWrites) all() []uint32 {
	w.mu.Lock()
	defer w.mu.Unlock()
	return slices.Clone(w.values)
}

// next answers the next value written, waiting for the worker to write it.
func (w *gateTestWrites) next(t *testing.T) uint32 {
	t.Helper()
	if n := w.overflowed(); n > 0 {
		t.Fatalf("the worker wrote %d gate values past the %d the test had not read yet: it writes without end; written so far %v",
			n, cap(w.wrote), w.all())
	}
	select {
	case value := <-w.wrote:
		return value
	case <-time.After(gateTestWait):
		t.Fatalf("no gate write within %s; written so far %v", gateTestWait, w.all())
	}
	return 0
}

// gateTestReader is an nftables ruleset in memory. err, when set, fails every dump.
// generation, when set, answers each Generation call, numbered from zero; nil answers
// generation 1 to every call. rules, when set, runs at each GetRules call, numbered
// from zero, before the call answers, and may change the ruleset and the generation,
// which is how a test lands a commit between two dumps of one judgment.
type gateTestReader struct {
	tables          []*nftables.Table
	chains          []*nftables.Chain
	err             error
	generation      func(call int) uint32
	generationCalls int
	rules           func(call int)
	rulesCalls      int
}

func (r *gateTestReader) Generation() (uint32, error) {
	if r.err != nil {
		return 0, r.err
	}
	if r.generation == nil {
		return 1, nil
	}
	call := r.generationCalls
	r.generationCalls++
	return r.generation(call), nil
}

func (r *gateTestReader) ListTables() ([]*nftables.Table, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.tables, nil
}

func (r *gateTestReader) ListChains() ([]*nftables.Chain, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.chains, nil
}

func (r *gateTestReader) GetRules(_ *nftables.Table, _ *nftables.Chain) ([]*nftables.Rule, error) {
	if r.err != nil {
		return nil, r.err
	}
	if r.rules != nil {
		call := r.rulesCalls
		r.rulesCalls++
		r.rules(call)
	}
	return nil, nil
}

// gateTestBlocking is a ruleset whose inet input chain drops by policy and holds no
// rule, so nothing accepts ESP before the policy drops it.
func gateTestBlocking() *gateTestReader {
	table := &nftables.Table{Name: "zegate", Family: nftables.TableFamilyINet}
	drop := nftables.ChainPolicyDrop
	chain := &nftables.Chain{
		Name:     "zeinput",
		Table:    table,
		Hooknum:  nftables.ChainHookInput,
		Priority: nftables.ChainPriorityFilter,
		Type:     nftables.ChainTypeFilter,
		Policy:   &drop,
	}
	return &gateTestReader{tables: []*nftables.Table{table}, chains: []*nftables.Chain{chain}}
}

// gateTestVerdict is what one scripted judgment answers.
type gateTestVerdict struct {
	blocked bool
	site    espFormFirewallSite
	err     error
}

// gateTestJudge is the judge seam: pass answers the verdict of each call, numbered
// from zero, and may act on the fast path first, which is how a test lands a hold or
// a commit while a judgment reads.
type gateTestJudge struct {
	mu    sync.Mutex
	calls int
	pass  func(call int) gateTestVerdict
}

func (j *gateTestJudge) judge(espFormNFTReader) (bool, espFormFirewallSite, error) {
	j.mu.Lock()
	call := j.calls
	j.calls++
	j.mu.Unlock()
	v := j.pass(call)
	return v.blocked, v.site, v.err
}

func (j *gateTestJudge) count() int {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.calls
}

// gateTestSite is the site a scripted blocking judgment names.
var gateTestSite = espFormFirewallSite{
	Family: "inet", Table: "zegate", Chain: "zeinput", Hook: "input", Rule: "policy drop", Reason: "drop",
}

// newGateTestFastPath answers a fast path whose gate writes are recorded, reading the
// ruleset through reader and the commits through events, as start would leave it. Its
// watch is a gateTestWatch that answers no failure, as one Close ended does.
func newGateTestFastPath(t *testing.T, reader espFormNFTReader, events <-chan struct{}) (*espFormFastPath, *gateTestWrites) {
	t.Helper()
	return newGateTestFastPathWatched(t, reader, events, &gateTestWatch{})
}

// newGateTestFastPathWatched is newGateTestFastPath with the watch the test ends.
//
// The test's cleanup fails it when a write had no room in the recorder: next reports
// an overflow only when the test calls it again, and a test that reads its last write
// would otherwise pass over a worker that went on writing. The check is registered
// before any worker's stop (gateTestWorkerLinked), so it runs after the worker
// returned, over every write it made, and it reports from the test's own cleanup,
// while the test is still live.
func newGateTestFastPathWatched(t *testing.T, reader espFormNFTReader, events <-chan struct{}, watch *gateTestWatch) (*espFormFastPath, *gateTestWrites) {
	t.Helper()
	f := newESPFormFastPath(nil)
	writes := &gateTestWrites{wrote: make(chan uint32, 64)}
	t.Cleanup(func() {
		if n := writes.overflowed(); n > 0 {
			t.Errorf("the worker wrote %d gate values past the %d the test had not read: it writes without end; written %v",
				n, cap(writes.wrote), writes.all())
		}
	})
	f.writeGate = writes.write
	f.mu.Lock()
	f.adoptFirewallLocked(espFormFirewallView{reader: reader, watch: watch, events: events}, nil)
	f.mu.Unlock()
	return f, writes
}

// gateTestWatch is a watch the test ends by closing its channel. err is what Err
// answers then: nil for a watch Close ended, a failure otherwise.
type gateTestWatch struct {
	mu     sync.Mutex
	err    error
	closes int
}

func (w *gateTestWatch) Err() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.err
}

func (w *gateTestWatch) Close() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.closes++
}

// fail sets the failure Err answers once the test closes the watch's channel.
func (w *gateTestWatch) fail(err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.err = err
}

// closeCount answers how many times the fast path closed the watch.
func (w *gateTestWatch) closeCount() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.closes
}

// gateTestTimer is one wait the worker armed: fire releases it.
type gateTestTimer struct {
	wait time.Duration
	fire chan time.Time
}

// gateTestClock is the rewatch clock. Every wait the worker arms is sent on armed and
// fires only when the test sends on its channel, so no test waits on a duration. Its
// time starts at gateTestEpoch and moves only when the test advances it.
type gateTestClock struct {
	armed chan gateTestTimer

	mu sync.Mutex
	at time.Time
	// overflow counts the waits armed had no room for: a worker that arms without
	// end never blocks, and next reports the overflow instead.
	overflow int
}

// newGateTestClock answers a clock whose overflow the test's cleanup checks, as
// newGateTestFastPathWatched checks its writes: next reports one only when the test
// calls it again. The check is registered before the worker's stop, so it runs after
// the worker returned.
func newGateTestClock(t *testing.T) *gateTestClock {
	t.Helper()
	c := &gateTestClock{armed: make(chan gateTestTimer, 16), at: gateTestEpoch}
	t.Cleanup(func() {
		c.mu.Lock()
		overflow := c.overflow
		c.mu.Unlock()
		if overflow > 0 {
			t.Errorf("the worker armed %d waits past the %d the test had not read: it arms without end", overflow, cap(c.armed))
		}
	})
	return c
}

func (c *gateTestClock) after(wait time.Duration) <-chan time.Time {
	timer := gateTestTimer{wait: wait, fire: make(chan time.Time, 1)}
	select {
	case c.armed <- timer:
	default:
		c.mu.Lock()
		c.overflow++
		c.mu.Unlock()
	}
	return timer.fire
}

// now is the rewatcher's clock.
func (c *gateTestClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.at
}

// advance moves the clock's time on by d.
func (c *gateTestClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.at = c.at.Add(d)
}

// next answers the next wait the worker arms, waiting for the worker to arm it.
func (c *gateTestClock) next(t *testing.T) gateTestTimer {
	t.Helper()
	c.mu.Lock()
	overflow := c.overflow
	c.mu.Unlock()
	if overflow > 0 {
		t.Fatalf("the worker armed %d waits past the %d the test had not read yet: it arms without end", overflow, cap(c.armed))
	}
	select {
	case timer := <-c.armed:
		return timer
	case <-time.After(gateTestWait):
		t.Fatalf("the worker armed no wait to watch the ruleset again within %s", gateTestWait)
	}
	return gateTestTimer{}
}

// gateTestOpen is what one attempt to watch again answers.
type gateTestOpen struct {
	watch  *gateTestWatch
	events chan struct{}
	err    error
}

// gateTestOpener answers each attempt to watch the ruleset again from its script, in
// order, and counts the attempts.
type gateTestOpener struct {
	mu      sync.Mutex
	opens   int
	answers []gateTestOpen
}

func (o *gateTestOpener) open() (espFormRulesetWatch, <-chan struct{}, error) {
	o.mu.Lock()
	attempt := o.opens
	o.opens++
	o.mu.Unlock()
	if attempt >= len(o.answers) {
		return nil, nil, errors.New("gateTestOpener: no answer scripted for this attempt")
	}
	answer := o.answers[attempt]
	if answer.err != nil {
		return nil, nil, answer.err
	}
	return answer.watch, answer.events, nil
}

func (o *gateTestOpener) count() int {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.opens
}

// gateTestState reads the fast path's state.
func gateTestState(t *testing.T, f *espFormFastPath) ESPFormFastPathState {
	t.Helper()
	st, err := f.state()
	if err != nil {
		t.Fatalf("read the fast path state: %v", err)
	}
	return st
}

// gateTestAttachedReading is what a test that marks the fast path Attached reads of
// it: the status, the cause and the gate's verdict.
type gateTestAttachedReading struct {
	status   ESPFormFastPathStatus
	cause    string
	gateOpen bool
}

// gateTestAttached reads the status, the cause and the gate's verdict under the fast
// path's mu. A fast path from newESPFormFastPath that a test marks Attached holds no
// rewrite counter map, and state reads that map under Attached, so such a test reads
// the fields state copies these from rather than call state. Production has no guard
// that answers zero counts for a missing map: state's caller is owed real counts or
// an error.
func gateTestAttached(f *espFormFastPath) gateTestAttachedReading {
	f.mu.Lock()
	defer f.mu.Unlock()
	return gateTestAttachedReading{status: f.status, cause: f.cause, gateOpen: f.gateOpen}
}

// gateTestWorker runs the worker's loop over a link subscription that sends nothing,
// and stops it at cleanup the way stop does: done closes, then the subscription. Its
// rewatch fails the test when the worker tries to watch again.
func gateTestWorker(t *testing.T, f *espFormFastPath, events <-chan struct{}) {
	t.Helper()
	gateTestWorkerRewatching(t, f, events, gateTestNoRewatch(t))
}

// gateTestNoRewatch is a rewatch for a test whose worker never watches again: an
// attempt or a wait it arms fails the test, while the test runs (gateTestLive). Its
// clock stands at gateTestEpoch.
func gateTestNoRewatch(t *testing.T) espFormRewatcher {
	live := newGateTestLive(t)
	return espFormRewatcher{
		open: func() (espFormRulesetWatch, <-chan struct{}, error) {
			live.errorf("the worker tried to watch the ruleset again")
			return nil, nil, errors.New("this test watches no ruleset again")
		},
		after: func(wait time.Duration) <-chan time.Time {
			live.errorf("the worker armed a wait of %s to watch the ruleset again", wait)
			return nil
		},
		now: func() time.Time { return gateTestEpoch },
	}
}

// gateTestWorkerRewatching runs the worker with rewatch, and answers the stop of
// gateTestWorker, which cleanup also runs; a second call does nothing.
func gateTestWorkerRewatching(t *testing.T, f *espFormFastPath, events <-chan struct{}, rewatch espFormRewatcher) (stop func()) {
	t.Helper()
	_, stop = gateTestWorkerLinked(t, f, events, rewatch)
	return stop
}

// gateTestWorkerLinked is gateTestWorkerRewatching that also answers the link
// subscription the worker reads. It is unbuffered, so a send on it returns only once
// the worker's loop took the update.
//
// stop closes done and the subscription once, then waits at most gateTestWait for the
// worker to return, on every call: a worker that never returns (a mutant whose serve
// keeps running) fails the test by name and leaks, rather than hang the test binary in
// the body's stop or in the cleanup's.
func gateTestWorkerLinked(t *testing.T, f *espFormFastPath, events <-chan struct{}, rewatch espFormRewatcher) (links chan<- netlink.LinkUpdate, stop func()) {
	t.Helper()
	updates := make(chan netlink.LinkUpdate)
	done := make(chan struct{})
	exited := make(chan struct{})
	live := newGateTestLive(t)
	go gateTestServe(f, updates, events, done, rewatch, exited)
	var once sync.Once
	stop = func() {
		once.Do(func() {
			close(done)
			close(updates)
		})
		select {
		case <-exited:
		case <-time.After(gateTestWait):
			live.errorf("the worker did not return within %s of its stop: serve kept running after done closed", gateTestWait)
		}
	}
	t.Cleanup(stop)
	return updates, stop
}

// gateTestStopInto runs stop, then closes stopped, so a test can bound how long stop
// takes.
func gateTestStopInto(stop func(), stopped chan<- struct{}) {
	defer close(stopped)
	stop()
}

// gateTestServe is the worker goroutine of gateTestWorker.
func gateTestServe(f *espFormFastPath, updates <-chan netlink.LinkUpdate, events, done <-chan struct{}, rewatch espFormRewatcher, exited chan<- struct{}) {
	defer close(exited)
	f.serve(updates, events, done, rewatch)
}

// TestESPFormFastPathGateOpensOnSafeRuleset: a ruleset with no table opens the gate,
// with no reason recorded.
func TestESPFormFastPathGateOpensOnSafeRuleset(t *testing.T) {
	f, writes := newGateTestFastPath(t, &gateTestReader{}, nil)
	if got := gateTestState(t, f).FirewallReading; got != ESPFormFirewallPending {
		t.Errorf("FirewallReading %s before the first judgment, want %s", got, ESPFormFirewallPending)
	}
	f.judgeFirewall(nil, nil)

	if got := writes.all(); !slices.Equal(got, []uint32{espFormFastPathGateOpen}) {
		t.Errorf("gate writes %v, want one write of %d", got, espFormFastPathGateOpen)
	}
	st := gateTestState(t, f)
	if st.FirewallBlocked || st.FirewallSite != "" || st.FirewallCause != "" {
		t.Errorf("state blocked %v, site %q, cause %q, want none", st.FirewallBlocked, st.FirewallSite, st.FirewallCause)
	}
	if st.FirewallReading != ESPFormFirewallRead {
		t.Errorf("FirewallReading %s after a judgment that read the ruleset, want %s", st.FirewallReading, ESPFormFirewallRead)
	}
	if !st.GateOpen {
		t.Errorf("state GateOpen false after the gate was opened")
	}
}

// TestESPFormFastPathGateShutsOnBlockingRule: an input chain that drops by policy
// shuts the gate and names its table and chain.
func TestESPFormFastPathGateShutsOnBlockingRule(t *testing.T) {
	f, writes := newGateTestFastPath(t, gateTestBlocking(), nil)
	f.judgeFirewall(nil, nil)

	got := writes.all()
	if slices.Contains(got, espFormFastPathGateOpen) {
		t.Errorf("gate writes %v: the gate opened over a rule that drops ESP", got)
	}
	if len(got) == 0 || got[len(got)-1] != espFormFastPathGateClosed {
		t.Errorf("gate writes %v, want the last one %d", got, espFormFastPathGateClosed)
	}
	st := gateTestState(t, f)
	if !st.FirewallBlocked {
		t.Errorf("state is not FirewallBlocked over an input chain that drops by policy")
	}
	if !strings.Contains(st.FirewallSite, "zegate") || !strings.Contains(st.FirewallSite, "zeinput") {
		t.Errorf("FirewallSite %q does not name the table zegate and the chain zeinput", st.FirewallSite)
	}
	if st.FirewallCause != "" {
		t.Errorf("FirewallCause %q, want none: the ruleset was read", st.FirewallCause)
	}
	if st.FirewallReading != ESPFormFirewallRead {
		t.Errorf("FirewallReading %s over a blocking rule, want %s: the ruleset was read", st.FirewallReading, ESPFormFirewallRead)
	}
	if st.GateOpen {
		t.Errorf("state GateOpen over a rule that drops ESP")
	}
}

// TestESPFormFastPathGateRefusesASplicedRead: a commit that lands between two dumps of
// one judgment, and whose NEWGEN the watch has not yet signaled, never opens the gate.
// The first pass lists an inet input chain whose policy accepts; while it dumps that
// chain's rules, after the chain dump, a commit replaces the chain with one that
// drops by policy and moves the generation from 1 to 2. The second generation read
// sees the move, the gate shuts and judges again, and the second pass publishes the
// drop.
//
// METHOD: a fake reader whose first GetRules call swaps in the dropping ruleset and
// moves the generation; no events channel, so only the generation bracket can see the
// commit.
// MUTATION: skip the compare of the second Generation read with the first in
// espFormFirewallBlocks: the first pass publishes its safe read and the gate opens.
func TestESPFormFastPathGateRefusesASplicedRead(t *testing.T) {
	reader := gateTestBlocking()
	accept := nftables.ChainPolicyAccept
	reader.chains[0].Policy = &accept
	generation := uint32(1)
	reader.generation = func(int) uint32 { return generation }
	reader.rules = func(call int) {
		if call != 0 {
			return
		}
		// A commit lands between the chain dump and the rule dump of the first pass.
		blocking := gateTestBlocking()
		reader.tables, reader.chains = blocking.tables, blocking.chains
		generation = 2
	}
	f, writes := newGateTestFastPath(t, reader, nil)
	f.judgeFirewall(nil, nil)

	got := writes.all()
	if slices.Contains(got, espFormFastPathGateOpen) {
		t.Errorf("gate writes %v: the gate opened over a read a commit spliced", got)
	}
	if len(got) == 0 || got[len(got)-1] != espFormFastPathGateClosed {
		t.Errorf("gate writes %v, want the last one %d", got, espFormFastPathGateClosed)
	}
	st := gateTestState(t, f)
	if !st.FirewallBlocked || !strings.Contains(st.FirewallSite, "zeinput") {
		t.Errorf("state blocked %v, site %q: want the drop the commit added, judged on the second pass",
			st.FirewallBlocked, st.FirewallSite)
	}
	if reader.generationCalls != 4 {
		t.Errorf("Generation read %d times, want 4: two judgments, each bracketed", reader.generationCalls)
	}
	if reader.rulesCalls != 2 {
		t.Errorf("GetRules called %d times, want 2: the input chain's rules on each pass", reader.rulesCalls)
	}
}

// TestESPFormFastPathGateShutsOnDumpError: an open gate shuts when the next dump fails,
// and the state carries the failure.
func TestESPFormFastPathGateShutsOnDumpError(t *testing.T) {
	reader := &gateTestReader{}
	f, writes := newGateTestFastPath(t, reader, nil)
	f.judgeFirewall(nil, nil)
	if got := writes.all(); !slices.Equal(got, []uint32{espFormFastPathGateOpen}) {
		t.Fatalf("gate writes %v before the failing dump, want the gate open", got)
	}

	reader.err = errors.New("netlink receive: operation not permitted")
	f.firewallCommitted()
	f.judgeFirewall(nil, nil)

	// Opened, shut by the commit, then shut again by the judgment whose dump failed.
	if got := writes.all(); !slices.Equal(got, []uint32{espFormFastPathGateOpen, espFormFastPathGateClosed, espFormFastPathGateClosed}) {
		t.Errorf("gate writes %v, want it opened, shut by the commit, then shut by the failed dump", got)
	}
	st := gateTestState(t, f)
	if !strings.Contains(st.FirewallCause, "operation not permitted") {
		t.Errorf("FirewallCause %q does not carry the dump's failure", st.FirewallCause)
	}
	if st.FirewallBlocked {
		t.Errorf("state is FirewallBlocked, want a cause alone: no rule was read")
	}
	if st.FirewallReading != ESPFormFirewallDumpFailed {
		t.Errorf("FirewallReading %s after a failed dump, want %s: the watch still runs, so the next commit judges again", st.FirewallReading, ESPFormFirewallDumpFailed)
	}
	f.mu.Lock()
	lost := f.firewallLost
	f.mu.Unlock()
	if lost {
		t.Errorf("firewallLost set after a failed dump: the next commit would then judge nothing")
	}
}

// TestESPFormFastPathDumpFailureJudgedAgain: a dump that fails while the watch lives
// shuts the gate as DumpFailed, and the worker judges again on the next commit the
// watch signals and on the next release, so the gate opens with no restart once a dump
// reads the ruleset.
//
// METHOD: the worker's loop over a fake reader whose failure the test sets and clears;
// each step waits on the gate write the worker makes.
func TestESPFormFastPathDumpFailureJudgedAgain(t *testing.T) {
	for _, tc := range []struct {
		name string
		// rejudge asks for the next judgment, as a commit or a release does.
		rejudge func(f *espFormFastPath, events chan<- struct{}, writes *gateTestWrites, t *testing.T)
	}{
		{name: "a commit the watch signals", rejudge: func(_ *espFormFastPath, events chan<- struct{}, writes *gateTestWrites, t *testing.T) {
			events <- struct{}{}
			if got := writes.next(t); got != espFormFastPathGateClosed {
				t.Fatalf("the commit wrote %d, want %d: a commit shuts the gate before it is judged", got, espFormFastPathGateClosed)
			}
		}},
		{name: "a firewall commit's release", rejudge: func(f *espFormFastPath, _ chan<- struct{}, writes *gateTestWrites, t *testing.T) {
			release := f.hold()
			if got := writes.next(t); got != espFormFastPathGateClosed {
				t.Fatalf("the hold wrote %d, want %d", got, espFormFastPathGateClosed)
			}
			release()
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reader := &gateTestReader{err: errors.New("netlink receive: no buffer space available")}
			events := make(chan struct{}, 1)
			f, writes := newGateTestFastPath(t, reader, events)
			gateTestWorker(t, f, events)
			if got := writes.next(t); got != espFormFastPathGateClosed {
				t.Fatalf("the first judgment wrote %d over a failing dump, want %d", got, espFormFastPathGateClosed)
			}
			st := gateTestState(t, f)
			if st.FirewallReading != ESPFormFirewallDumpFailed {
				t.Fatalf("FirewallReading %s after a failed dump, want %s", st.FirewallReading, ESPFormFirewallDumpFailed)
			}
			if !strings.Contains(st.FirewallCause, "no buffer space available") {
				t.Errorf("FirewallCause %q does not carry the dump's failure", st.FirewallCause)
			}

			f.mu.Lock()
			reader.err = nil
			f.mu.Unlock()
			tc.rejudge(f, events, writes, t)
			if got := writes.next(t); got != espFormFastPathGateOpen {
				t.Fatalf("after the dump could read again the gate was written %d, want it open", got)
			}
			st = gateTestState(t, f)
			if st.FirewallReading != ESPFormFirewallRead || st.FirewallCause != "" {
				t.Errorf("FirewallReading %s, FirewallCause %q after a dump that read, want %s and no cause", st.FirewallReading, st.FirewallCause, ESPFormFirewallRead)
			}
		})
	}
}

// TestESPFormFastPathAcceptFixNamesTheChain: a blocking site carries, in the state, the
// fix and the sentence for its table: for a table Ze does not own, the nft insert at
// the head of the base chain, one single-quoted shell word whose table and chain are
// double-quoted nft identifiers, whatever spaces or shell characters the names hold,
// beside the owner-flag caveat; no command for a table or chain name holding a quote
// of either kind, a control character or DEL; for a ze_
// table, what the registered ESPFormOwnedTableAccept answers, given the site's family,
// table and chain. A judgment that finds no blocking site clears both.
//
// The row for a site whose chain named a jump path ("input -> wan-in") is gone: the
// judgment walks base chains alone, so no site carries that form any more.
func TestESPFormFastPathAcceptFixNamesTheChain(t *testing.T) {
	saved := espFormOwnedTableAccept.Load()
	t.Cleanup(func() { espFormOwnedTableAccept.Store(saved) })
	RegisterESPFormOwnedTableAccept(func(family, table, chain string) (string, string) {
		return "owned " + family + " " + table + " " + chain, "fix for " + table
	})
	const ownerFlag = "only the process that owns it can make it"
	for _, tc := range []struct {
		name    string
		site    espFormFirewallSite
		want    string
		howHas  []string
		howLack string
	}{
		{
			name:   "a table Ze does not own",
			site:   espFormFirewallSite{Family: "inet", Table: "filter", Chain: "input", Hook: "input", Rule: "policy drop", Reason: "drop"},
			want:   `sudo nft 'insert rule inet "filter" "input" meta l4proto esp accept'`,
			howHas: []string{"Table filter is not Ze's", "chain input", ownerFlag},
		},
		{
			name:   "names a shell would expand or nft would split",
			site:   espFormFirewallSite{Family: "ip", Table: "my $(rm -rf) table", Chain: "in;put *", Hook: "input", Rule: "handle 9", Reason: "drop"},
			want:   `sudo nft 'insert rule ip "my $(rm -rf) table" "in;put *" meta l4proto esp accept'`,
			howHas: []string{ownerFlag},
		},
		{
			name:    "a name holding a quote",
			site:    espFormFirewallSite{Family: "inet", Table: "it's", Chain: "input", Hook: "input", Rule: "policy drop", Reason: "drop"},
			want:    "sudo nft list ruleset",
			howHas:  []string{"prints no command to paste", ownerFlag},
			howLack: "insert rule inet",
		},
		{
			// MUTATION: delete the chain guard in espFormForeignTableAccept, or the
			// double-quote guard in espFormNFTQuotable, and the nft insert is printed.
			name:    "a chain name holding a double quote",
			site:    espFormFirewallSite{Family: "inet", Table: "filter", Chain: `in"put`, Hook: "input", Rule: "policy drop", Reason: "drop"},
			want:    "sudo nft list ruleset",
			howHas:  []string{"prints no command to paste", ownerFlag},
			howLack: "insert rule inet",
		},
		{
			// MUTATION: delete the r < ' ' guard in espFormNFTQuotable, and the nft
			// insert is printed with a tab inside the quoted table name.
			name:    "a table name holding a control character",
			site:    espFormFirewallSite{Family: "inet", Table: "fil\tter", Chain: "input", Hook: "input", Rule: "policy drop", Reason: "drop"},
			want:    "sudo nft list ruleset",
			howHas:  []string{"prints no command to paste", ownerFlag},
			howLack: "insert rule inet",
		},
		{
			// MUTATION: delete the r == 0x7f guard in espFormNFTQuotable, and the nft
			// insert is printed with DEL inside the quoted chain name.
			name:    "a chain name holding DEL",
			site:    espFormFirewallSite{Family: "inet", Table: "filter", Chain: "input\x7f", Hook: "input", Rule: "policy drop", Reason: "drop"},
			want:    "sudo nft list ruleset",
			howHas:  []string{"prints no command to paste", ownerFlag},
			howLack: "insert rule inet",
		},
		{
			name:   "a table Ze owns",
			site:   espFormFirewallSite{Family: "inet", Table: "ze_wan", Chain: "input", Hook: "input", Rule: "handle 7 \"block-all\"", Reason: "drop"},
			want:   "fix for ze_wan",
			howHas: []string{"owned inet ze_wan input"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, _ := newGateTestFastPath(t, &gateTestReader{}, nil)
			pass := 0
			judge := &gateTestJudge{pass: func(int) gateTestVerdict {
				pass++
				if pass == 1 {
					return gateTestVerdict{blocked: true, site: tc.site}
				}
				return gateTestVerdict{}
			}}
			f.judge = judge.judge
			f.judgeFirewall(nil, nil)
			st := gateTestState(t, f)
			if !st.FirewallBlocked || st.FirewallAcceptFix != tc.want {
				t.Errorf("blocked %v, FirewallAcceptFix %q, want blocked and %q", st.FirewallBlocked, st.FirewallAcceptFix, tc.want)
			}
			for _, text := range tc.howHas {
				if !strings.Contains(st.FirewallAcceptHow, text) {
					t.Errorf("FirewallAcceptHow %q does not carry %q", st.FirewallAcceptHow, text)
				}
			}
			if tc.howLack != "" && strings.Contains(st.FirewallAcceptHow, tc.howLack) {
				t.Errorf("FirewallAcceptHow %q carries %q", st.FirewallAcceptHow, tc.howLack)
			}

			f.firewallCommitted()
			f.judgeFirewall(nil, nil)
			if st := gateTestState(t, f); st.FirewallAcceptFix != "" || st.FirewallAcceptHow != "" {
				t.Errorf("FirewallAcceptFix %q, FirewallAcceptHow %q after a judgment found no blocking site, want none", st.FirewallAcceptFix, st.FirewallAcceptHow)
			}
		})
	}
}

// TestESPFormFastPathOwnedTableWithNoAnswerIsADefect: a blocking site in a ze_ table
// while no ESPFormOwnedTableAccept is registered names no command, the state the
// doctor reports as a Ze defect, rather than an nft rule Ze's next reconcile removes.
func TestESPFormFastPathOwnedTableWithNoAnswerIsADefect(t *testing.T) {
	saved := espFormOwnedTableAccept.Load()
	t.Cleanup(func() { espFormOwnedTableAccept.Store(saved) })
	espFormOwnedTableAccept.Store(nil)
	how, fix := espFormFirewallAccept(espFormFirewallSite{Family: "inet", Table: "ze_wan", Chain: "input"})
	if how != "" || fix != "" {
		t.Errorf("espFormFirewallAccept with no answer registered = %q, %q; want none", how, fix)
	}
}

// TestESPFormFastPathFailedShutDetaches: a write of 0 the gate map refuses leaves no
// fast path attached with an open gate: the state turns Detached, never the load
// failure's Unavailable, with the failure as its cause, GateOpen is clear, no link
// stays attached, and a later judgment that finds the ruleset safe writes nothing and
// opens nothing.
//
// METHOD: the writeGate seam accepts the first write of 1, then refuses every write;
// the fast path is marked Attached with an empty link map, as start leaves it before
// the first link arrives. It holds no rewrite counter map, so the test reads the
// status, the cause and the gate under mu (gateTestAttached), never through state.
// MUTATION: record ESPFormFastPathUnavailable in detachLocked: the status is the load
// failure's, and the doctor would name CAP_BPF for a program that had loaded.
// MUTATION: drop the Detached case from writeGateLocked: the judgment after the detach
// writes 1 into the gate through the seam.
func TestESPFormFastPathFailedShutDetaches(t *testing.T) {
	f, writes := newGateTestFastPath(t, &gateTestReader{}, nil)
	refuse := false
	f.writeGate = func(value uint32) error {
		if refuse {
			return errors.New("map update: bad file descriptor")
		}
		return writes.write(value)
	}
	f.mu.Lock()
	f.status = ESPFormFastPathAttached
	f.links = make(map[int]espFormAttachedLink)
	f.refused = make(map[int]ESPFormLinkRefusal)
	f.mu.Unlock()
	f.judgeFirewall(nil, nil)
	if !gateTestAttached(f).gateOpen {
		t.Fatalf("the gate did not open over a safe ruleset before the failed shut")
	}

	refuse = true
	release := f.hold()
	// Released at once, so the judgment below finds no hold open and would open the
	// gate, were the fast path still attached.
	release()
	st := gateTestAttached(f)
	if st.status != ESPFormFastPathDetached {
		t.Errorf("status %s after a refused shut, want %s", st.status, ESPFormFastPathDetached)
	}
	if !strings.Contains(st.cause, "bad file descriptor") || !strings.Contains(st.cause, "detached from every link") {
		t.Errorf("Cause %q does not name the refused write and the detach", st.cause)
	}
	if st.gateOpen {
		t.Errorf("GateOpen after a refused shut: the program would rewrite past a rule that should see ESP")
	}
	f.mu.Lock()
	attached := len(f.links)
	f.mu.Unlock()
	if attached != 0 {
		t.Errorf("%d links still attached after a refused shut, want 0", attached)
	}

	refuse = false
	before := len(writes.all())
	f.firewallCommitted()
	f.judgeFirewall(nil, nil)
	if got := writes.all()[before:]; len(got) != 0 {
		t.Errorf("a judgment after the detach wrote %v into the gate, want nothing", got)
	}
	if gateTestAttached(f).gateOpen {
		t.Errorf("GateOpen after a judgment that followed the detach")
	}
}

// TestESPFormFastPathSubscriptionEndKeepsTheDetachCause: a link subscription that ends
// after a refused shut detached the fast path leaves the detach's cause, the one the
// doctor reports for Detached, while under Attached its end is the cause.
//
// METHOD: two fast paths marked Attached with empty link maps, as start leaves them.
// The first has its shut refused through the writeGate seam, so it detaches, then
// meets the subscription's end (followEnded); the second meets the end while
// attached. Both are read under mu (gateTestAttached).
// MUTATION: drop the Attached test from followEnded: the detached fast path's cause
// becomes the subscription's end, and the doctor's Detached finding no longer names
// the refused write.
func TestESPFormFastPathSubscriptionEndKeepsTheDetachCause(t *testing.T) {
	ended := errors.New("the link subscription ended")

	detached, _ := newGateTestFastPath(t, &gateTestReader{}, nil)
	detached.writeGate = func(uint32) error { return errors.New("map update: bad file descriptor") }
	detached.mu.Lock()
	detached.status = ESPFormFastPathAttached
	detached.links = make(map[int]espFormAttachedLink)
	detached.refused = make(map[int]ESPFormLinkRefusal)
	detached.mu.Unlock()
	detached.firewallCommitted()
	before := gateTestAttached(detached)
	if before.status != ESPFormFastPathDetached {
		t.Fatalf("status %s after a refused shut, want %s: the rest of this test proves nothing", before.status, ESPFormFastPathDetached)
	}
	detached.followEnded(ended)
	after := gateTestAttached(detached)
	if after.status != ESPFormFastPathDetached {
		t.Errorf("status %s after the subscription ended, want %s", after.status, ESPFormFastPathDetached)
	}
	if after.cause != before.cause {
		t.Errorf("Cause %q after the subscription ended, want the detach's %q", after.cause, before.cause)
	}

	attached, _ := newGateTestFastPath(t, &gateTestReader{}, nil)
	attached.mu.Lock()
	attached.status = ESPFormFastPathAttached
	attached.mu.Unlock()
	attached.followEnded(ended)
	if got := gateTestAttached(attached).cause; !strings.HasPrefix(got, ended.Error()+";") {
		t.Errorf("Cause %q under Attached after the subscription ended, want it to name the end first", got)
	}
}

// TestESPFormFastPathStopShutsGateFirst: stop writes 0 into the gate before it closes
// the nftables watch, so the program never rewrites while nothing watches the ruleset.
//
// METHOD: the gate writer seam reads the watch's close count when 0 is written.
func TestESPFormFastPathStopShutsGateFirst(t *testing.T) {
	watch := &gateTestWatch{}
	f, writes := newGateTestFastPathWatched(t, &gateTestReader{}, nil, watch)
	f.judgeFirewall(nil, nil)
	if got := writes.all(); !slices.Equal(got, []uint32{espFormFastPathGateOpen}) {
		t.Fatalf("gate writes %v before stop, want the gate open", got)
	}
	closedAtShut := -1
	f.writeGate = func(value uint32) error {
		if value == espFormFastPathGateClosed && closedAtShut < 0 {
			closedAtShut = watch.closeCount()
		}
		return writes.write(value)
	}
	f.stop()
	if closedAtShut != 0 {
		t.Errorf("the watch had been closed %d times when stop shut the gate, want 0: the gate must shut first", closedAtShut)
	}
	if got := watch.closeCount(); got != 1 {
		t.Errorf("stop closed the watch %d times, want 1", got)
	}
}

// TestESPFormFastPathRereadWhen: the warning of each reading that shuts the gate for a
// cause says what makes Ze read the ruleset again: a restart for a reader or watch
// that start could not open and for a watch Close ended; for a dump that failed under
// a live watch, no restart, the next commit; for a failed watch, no restart, the
// rewatch bound.
func TestESPFormFastPathRereadWhen(t *testing.T) {
	for _, tc := range []struct {
		reading ESPFormFirewallReading
		has     []string
		lacks   string
	}{
		{ESPFormFirewallUnreadable, []string{"until ze restarts"}, "no restart is needed"},
		{ESPFormFirewallWatchClosed, []string{"until ze restarts"}, "no restart is needed"},
		{ESPFormFirewallDumpFailed, []string{"no restart is needed", "the next commit to the ruleset", "nothing reads it before then"}, "until ze restarts"},
		{ESPFormFirewallRewatching, []string{"no restart is needed", "30s apart"}, "until ze restarts"},
	} {
		got := espFormFirewallRereadWhen(tc.reading)
		for _, text := range tc.has {
			if !strings.Contains(got, text) {
				t.Errorf("%s: %q does not carry %q", tc.reading, got, text)
			}
		}
		if strings.Contains(got, tc.lacks) {
			t.Errorf("%s: %q carries %q", tc.reading, got, tc.lacks)
		}
	}
}

// TestESPFormFastPathGateShutsWhenWatchEnds: the worker shuts an open gate when the
// watch's channel closes, and no later judgment opens it again.
func TestESPFormFastPathGateShutsWhenWatchEnds(t *testing.T) {
	events := make(chan struct{}, 1)
	f, writes := newGateTestFastPath(t, &gateTestReader{}, events)
	gateTestWorker(t, f, events)
	if got := writes.next(t); got != espFormFastPathGateOpen {
		t.Fatalf("the first judgment wrote %d, want the gate open", got)
	}

	close(events)
	if got := writes.next(t); got != espFormFastPathGateClosed {
		t.Fatalf("after the watch ended the gate was written %d, want %d", got, espFormFastPathGateClosed)
	}
	if st := gateTestState(t, f); st.FirewallCause != espFormFirewallWatchEnded {
		t.Errorf("FirewallCause %q, want %q", st.FirewallCause, espFormFirewallWatchEnded)
	}
	if got := gateTestState(t, f).FirewallReading; got != ESPFormFirewallWatchClosed {
		t.Errorf("FirewallReading %s after Close ended the watch, want %s", got, ESPFormFirewallWatchClosed)
	}

	// A judgment asked for later, as a release asks, finds the ruleset unwatched and
	// leaves the gate shut.
	before := len(writes.all())
	f.judgeFirewall(nil, nil)
	if got := writes.all()[before:]; slices.Contains(got, espFormFastPathGateOpen) {
		t.Errorf("a judgment after the watch ended wrote %v: the gate opened unwatched", got)
	}
}

// TestESPFormFastPathGateIgnoresJudgmentBeganBeforeHold: a hold taken while a
// judgment reads makes that judgment stale, and the one after it finds the hold
// open, so neither opens the gate.
func TestESPFormFastPathGateIgnoresJudgmentBeganBeforeHold(t *testing.T) {
	f, writes := newGateTestFastPath(t, &gateTestReader{}, nil)
	var release func()
	judge := &gateTestJudge{pass: func(call int) gateTestVerdict {
		if call == 0 {
			release = f.hold() // a firewall commit begins while the ruleset is read
		}
		return gateTestVerdict{}
	}}
	f.judge = judge.judge

	f.judgeFirewall(nil, nil)

	if got := writes.all(); !slices.Equal(got, []uint32{espFormFastPathGateClosed}) {
		t.Errorf("gate writes %v, want only the hold's %d", got, espFormFastPathGateClosed)
	}
	if got := judge.count(); got != 2 {
		t.Errorf("the ruleset was judged %d times, want 2: the stale judgment must be run again", got)
	}
	release()
}

// TestESPFormFastPathGateIgnoresJudgmentBeganBeforeCommit: a commit the watch
// signals while a judgment reads makes that judgment stale, so the gate follows the
// judgment after the commit, never the one before it.
func TestESPFormFastPathGateIgnoresJudgmentBeganBeforeCommit(t *testing.T) {
	events := make(chan struct{}, 1)
	f, writes := newGateTestFastPath(t, &gateTestReader{}, events)
	judge := &gateTestJudge{pass: func(call int) gateTestVerdict {
		if call == 0 {
			events <- struct{}{} // a commit adding a drop lands while the ruleset is read
			return gateTestVerdict{}
		}
		return gateTestVerdict{blocked: true, site: gateTestSite}
	}}
	f.judge = judge.judge

	f.judgeFirewall(events, nil)

	// The commit's shut, then the drop's: never the open of the safe judgment that
	// began before the commit.
	if got := writes.all(); !slices.Equal(got, []uint32{espFormFastPathGateClosed, espFormFastPathGateClosed}) {
		t.Errorf("gate writes %v, want %d twice, the commit's and the drop's: the safe judgment began before the commit", got, espFormFastPathGateClosed)
	}
	st := gateTestState(t, f)
	if !st.FirewallBlocked || st.FirewallSite != gateTestSite.String() {
		t.Errorf("state blocked %v at %q, want blocked at %q", st.FirewallBlocked, st.FirewallSite, gateTestSite.String())
	}
}

// TestESPFormFastPathHoldShutsGateBeforeReturning: hold writes the shut gate itself,
// so a commit that follows it never meets an open gate.
func TestESPFormFastPathHoldShutsGateBeforeReturning(t *testing.T) {
	f, writes := newGateTestFastPath(t, &gateTestReader{}, nil)
	f.judgeFirewall(nil, nil)
	if got := writes.all(); !slices.Equal(got, []uint32{espFormFastPathGateOpen}) {
		t.Fatalf("gate writes %v before the hold, want the gate open", got)
	}

	release := f.hold()
	if got := writes.all(); !slices.Equal(got, []uint32{espFormFastPathGateOpen, espFormFastPathGateClosed}) {
		t.Errorf("gate writes %v when hold returned, want it shut by then", got)
	}
	release()
}

// TestESPFormFastPathReleaseJudgesAgain: a release has the worker judge the ruleset
// again, which opens the gate the hold shut; a second call of the same release does
// nothing.
func TestESPFormFastPathReleaseJudgesAgain(t *testing.T) {
	events := make(chan struct{}, 1)
	f, writes := newGateTestFastPath(t, &gateTestReader{}, events)
	judge := &gateTestJudge{pass: func(int) gateTestVerdict { return gateTestVerdict{} }}
	f.judge = judge.judge
	gateTestWorker(t, f, events)
	if got := writes.next(t); got != espFormFastPathGateOpen {
		t.Fatalf("the first judgment wrote %d, want the gate open", got)
	}

	release := f.hold()
	if got := writes.next(t); got != espFormFastPathGateClosed {
		t.Fatalf("the hold wrote %d, want %d", got, espFormFastPathGateClosed)
	}
	judged := judge.count()
	release()
	if got := writes.next(t); got != espFormFastPathGateOpen {
		t.Fatalf("after the release the gate was written %d, want it open again", got)
	}
	if judge.count() <= judged {
		t.Errorf("the gate opened after the release with no new judgment")
	}

	release() // a second call does nothing
	f.mu.Lock()
	holds := f.holds
	f.mu.Unlock()
	if holds != 0 {
		t.Errorf("after one hold and two calls of its release %d holds are open, want 0", holds)
	}
}

// TestESPFormFastPathHoldOnBareBackend: a backend built without a receiver answers a
// release that does nothing, rather than a nil func or a panic.
func TestESPFormFastPathHoldOnBareBackend(t *testing.T) {
	release := (&xfrmBackend{}).HoldESPFormFastPath()
	if release == nil {
		t.Fatal("HoldESPFormFastPath answered a nil release")
	}
	release()
}

// TestESPFormFastPathRewatchesAfterWatchFailure: a watch that ends on a failure shuts
// the gate with that failure in FirewallCause and is closed. Only once the first wait
// passed does the worker try to watch again. An attempt that fails keeps the gate shut,
// takes its own failure as the cause and doubles the wait. The attempt that opens is
// followed by a judgment that opens the gate and clears the cause, the new watch's
// commits are judged, and the next failure starts at the first wait again.
//
// METHOD: a scripted opener and a clock whose waits fire only when the test says, so
// the order is exact and nothing waits on a duration.
func TestESPFormFastPathRewatchesAfterWatchFailure(t *testing.T) {
	reader := &gateTestReader{}
	events := make(chan struct{}, 1)
	watch := &gateTestWatch{}
	f, writes := newGateTestFastPathWatched(t, reader, events, watch)
	again := &gateTestWatch{}
	againEvents := make(chan struct{}, 1)
	opener := &gateTestOpener{answers: []gateTestOpen{
		{err: errors.New("join the nftables multicast group to watch the ruleset: operation not permitted")},
		{watch: again, events: againEvents},
	}}
	clock := newGateTestClock(t)
	gateTestWorkerRewatching(t, f, events, espFormRewatcher{open: opener.open, after: clock.after, now: clock.now})
	if got := writes.next(t); got != espFormFastPathGateOpen {
		t.Fatalf("the first judgment wrote %d, want the gate open", got)
	}

	watch.fail(errors.New("the nftables monitor failed: recvmsg: no buffer space available"))
	close(events)
	if got := writes.next(t); got != espFormFastPathGateClosed {
		t.Fatalf("after the watch failed the gate was written %d, want %d", got, espFormFastPathGateClosed)
	}
	timer := clock.next(t)
	if timer.wait != espFormNFTRewatchFirst {
		t.Errorf("the first wait is %s, want %s", timer.wait, espFormNFTRewatchFirst)
	}
	st := gateTestState(t, f)
	if !strings.Contains(st.FirewallCause, "no buffer space available") {
		t.Errorf("FirewallCause %q does not name the watch's failure", st.FirewallCause)
	}
	if st.GateOpen || st.FirewallBlocked {
		t.Errorf("state GateOpen %v, FirewallBlocked %v after the watch failed, want a shut gate and a cause alone", st.GateOpen, st.FirewallBlocked)
	}
	if st.FirewallReading != ESPFormFirewallRewatching {
		t.Errorf("FirewallReading %s after the watch failed, want %s: Ze watches again by itself", st.FirewallReading, ESPFormFirewallRewatching)
	}
	if got := watch.closeCount(); got != 1 {
		t.Errorf("the failed watch was closed %d times, want 1", got)
	}
	if got := opener.count(); got != 0 {
		t.Errorf("the worker tried to watch again %d times before the wait passed, want 0", got)
	}

	timer.fire <- time.Time{}
	if got := writes.next(t); got != espFormFastPathGateClosed {
		t.Fatalf("after a failed attempt the gate was written %d, want %d", got, espFormFastPathGateClosed)
	}
	timer = clock.next(t)
	if timer.wait != 2*espFormNFTRewatchFirst {
		t.Errorf("the wait after a failed attempt is %s, want %s", timer.wait, 2*espFormNFTRewatchFirst)
	}
	st = gateTestState(t, f)
	if !strings.Contains(st.FirewallCause, "operation not permitted") {
		t.Errorf("FirewallCause %q does not name the failed attempt's failure", st.FirewallCause)
	}
	if st.GateOpen {
		t.Errorf("state GateOpen after a failed attempt to watch again")
	}
	if st.FirewallReading != ESPFormFirewallRewatching {
		t.Errorf("FirewallReading %s after a failed attempt to watch again, want %s: the worker tries again", st.FirewallReading, ESPFormFirewallRewatching)
	}

	timer.fire <- time.Time{}
	if got := writes.next(t); got != espFormFastPathGateOpen {
		t.Fatalf("after the watch opened again the gate was written %d, want it open", got)
	}
	st = gateTestState(t, f)
	if st.FirewallCause != "" || st.FirewallBlocked || !st.GateOpen {
		t.Errorf("state cause %q, blocked %v, GateOpen %v after the watch opened again, want no cause and an open gate", st.FirewallCause, st.FirewallBlocked, st.GateOpen)
	}
	if st.FirewallReading != ESPFormFirewallRead {
		t.Errorf("FirewallReading %s after the watch opened again and the ruleset was judged, want %s", st.FirewallReading, ESPFormFirewallRead)
	}
	if got := opener.count(); got != 2 {
		t.Errorf("the worker tried to watch again %d times, want 2", got)
	}

	// A commit the new watch signals is judged: one that adds a drop shuts the gate.
	*reader = *gateTestBlocking()
	againEvents <- struct{}{}
	// The commit's shut, then the drop's.
	for _, step := range []string{"the commit itself", "the judgment of the drop it added"} {
		if got := writes.next(t); got != espFormFastPathGateClosed {
			t.Fatalf("after a commit on the new watch, %s wrote %d into the gate, want %d", step, got, espFormFastPathGateClosed)
		}
	}
	if st := gateTestState(t, f); !st.FirewallBlocked {
		t.Errorf("a blocking commit signaled by the new watch left the state not FirewallBlocked")
	}
	if got := gateTestState(t, f).FirewallReading; got != ESPFormFirewallRead {
		t.Errorf("FirewallReading %s after a blocking commit on the new watch, want %s", got, ESPFormFirewallRead)
	}

	// The watch that opened reset the wait, so its own failure waits the first wait.
	again.fail(errors.New("the nftables monitor failed: recvmsg: no buffer space available"))
	close(againEvents)
	if got := writes.next(t); got != espFormFastPathGateClosed {
		t.Fatalf("after the new watch failed the gate was written %d, want %d", got, espFormFastPathGateClosed)
	}
	if timer := clock.next(t); timer.wait != espFormNFTRewatchFirst {
		t.Errorf("the wait after the new watch failed is %s, want %s", timer.wait, espFormNFTRewatchFirst)
	}
	if got := gateTestState(t, f).FirewallReading; got != ESPFormFirewallRewatching {
		t.Errorf("FirewallReading %s after the new watch failed, want %s", got, ESPFormFirewallRewatching)
	}
}

// TestESPFormFastPathClosedWatchIsNotRewatched: a watch that Close ended, whose Err
// answers nil, shuts the gate with espFormFirewallWatchEnded and is never watched
// again: the worker arms no wait and makes no attempt.
//
// METHOD: the worker is stopped after the end, so everything it did for the end is
// done before the test reads the clock and the opener.
func TestESPFormFastPathClosedWatchIsNotRewatched(t *testing.T) {
	events := make(chan struct{}, 1)
	watch := &gateTestWatch{}
	f, writes := newGateTestFastPathWatched(t, &gateTestReader{}, events, watch)
	opener := &gateTestOpener{}
	clock := newGateTestClock(t)
	stop := gateTestWorkerRewatching(t, f, events, espFormRewatcher{open: opener.open, after: clock.after, now: clock.now})
	if got := writes.next(t); got != espFormFastPathGateOpen {
		t.Fatalf("the first judgment wrote %d, want the gate open", got)
	}

	close(events)
	if got := writes.next(t); got != espFormFastPathGateClosed {
		t.Fatalf("after the watch ended the gate was written %d, want %d", got, espFormFastPathGateClosed)
	}
	stop()

	if got := len(clock.armed); got != 0 {
		t.Errorf("the worker armed %d waits to watch again after Close ended the watch, want 0", got)
	}
	if got := opener.count(); got != 0 {
		t.Errorf("the worker tried to watch again %d times after Close ended the watch, want 0", got)
	}
	if got := watch.closeCount(); got != 1 {
		t.Errorf("the ended watch was closed %d times, want 1", got)
	}
	st := gateTestState(t, f)
	if st.FirewallCause != espFormFirewallWatchEnded {
		t.Errorf("FirewallCause %q, want %q", st.FirewallCause, espFormFirewallWatchEnded)
	}
	if st.GateOpen {
		t.Errorf("state GateOpen after the watch ended")
	}
	if st.FirewallReading != ESPFormFirewallWatchClosed {
		t.Errorf("FirewallReading %s after Close ended the watch, want %s: nothing watches again until a restart", st.FirewallReading, ESPFormFirewallWatchClosed)
	}
}

// TestESPFormFastPathFirewallReadingAtStartAndStop: a fast path that holds no program
// reads no ruleset (Unheld); one whose reader or watch could not open at start is
// Unreadable with that failure as its cause; stop returns it to Unheld and clears the
// cause.
//
// METHOD: adoptFirewallLocked is what start calls with the view it opened, so the test
// drives it directly with a failure, and no kernel is touched.
func TestESPFormFastPathFirewallReadingAtStartAndStop(t *testing.T) {
	f := newESPFormFastPath(nil)
	if got := gateTestState(t, f).FirewallReading; got != ESPFormFirewallUnheld {
		t.Errorf("FirewallReading %s before start, want %s", got, ESPFormFirewallUnheld)
	}

	f.mu.Lock()
	f.adoptFirewallLocked(espFormFirewallView{}, errors.New("open the nftables ruleset reader: operation not permitted"))
	f.mu.Unlock()
	st := gateTestState(t, f)
	if st.FirewallReading != ESPFormFirewallUnreadable {
		t.Errorf("FirewallReading %s after the reader failed to open, want %s", st.FirewallReading, ESPFormFirewallUnreadable)
	}
	if !strings.Contains(st.FirewallCause, "operation not permitted") {
		t.Errorf("FirewallCause %q does not carry the open's failure", st.FirewallCause)
	}

	f.stop()
	st = gateTestState(t, f)
	if st.FirewallReading != ESPFormFirewallUnheld || st.FirewallCause != "" {
		t.Errorf("after stop FirewallReading %s, FirewallCause %q, want %s and no cause", st.FirewallReading, st.FirewallCause, ESPFormFirewallUnheld)
	}
}

// TestESPFormFastPathFirewallReadingString: every reading renders its own name, and a value
// outside the set names its number rather than a valid name.
func TestESPFormFastPathFirewallReadingString(t *testing.T) {
	for reading, want := range map[ESPFormFirewallReading]string{
		ESPFormFirewallUnspecified: "unspecified",
		ESPFormFirewallUnheld:      "unheld",
		ESPFormFirewallPending:     "pending",
		ESPFormFirewallRead:        "read",
		ESPFormFirewallUnreadable:  "unreadable",
		ESPFormFirewallRewatching:  "rewatching",
		ESPFormFirewallWatchClosed: "watch-closed",
		ESPFormFirewallDumpFailed:  "dump-failed",
		// MUTATION: drop the ESPFormFirewallMoving entry from espFormFirewallReadingNames:
		// the array ends at DumpFailed, and Moving renders
		// "invalid-esp-form-firewall-reading-8", not "moving".
		ESPFormFirewallMoving:      "moving",
		ESPFormFirewallReading(99): "invalid-esp-form-firewall-reading-99",
	} {
		if got := reading.String(); got != want {
			t.Errorf("ESPFormFirewallReading(%d).String() = %q, want %q", uint8(reading), got, want)
		}
	}
}

// TestESPFormFastPathRewatchWaitDoublesToCap: the wait between attempts to watch again
// starts at one second, doubles, stops at the thirty-second cap, and starts again at
// one second once a watch opened.
func TestESPFormFastPathRewatchWaitDoublesToCap(t *testing.T) {
	var loop espFormWatchLoop
	want := []time.Duration{
		time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second,
		16 * time.Second, 30 * time.Second, 30 * time.Second,
	}
	for i, wantWait := range want {
		if got := loop.nextWait(); got != wantWait {
			t.Errorf("wait %d is %s, want %s", i, got, wantWait)
		}
	}
	loop.wait = 0 // what watchAgain leaves once a watch opened
	if got := loop.nextWait(); got != time.Second {
		t.Errorf("the wait after a watch opened is %s, want %s", got, time.Second)
	}
}

// TestESPFormFastPathRejudgeWaitDoublesToCap: the wait before judging a ruleset that
// kept moving starts at fifty milliseconds, doubles, stops at the one-second cap, and
// starts again at fifty milliseconds once a judgment settled.
//
// MUTATION: cap nextRejudgeWait at espFormNFTRewatchMax rather than
// espFormNFTRejudgeMax: the sixth wait reads 1.6s, not 1s.
func TestESPFormFastPathRejudgeWaitDoublesToCap(t *testing.T) {
	var loop espFormWatchLoop
	want := []time.Duration{
		50 * time.Millisecond, 100 * time.Millisecond, 200 * time.Millisecond,
		400 * time.Millisecond, 800 * time.Millisecond, time.Second, time.Second,
	}
	for i, wantWait := range want {
		if got := loop.nextRejudgeWait(); got != wantWait {
			t.Errorf("wait %d is %s, want %s", i, got, wantWait)
		}
	}
	loop.rejudgeWait = 0 // what judgeWatched leaves once a judgment settled
	if got := loop.nextRejudgeWait(); got != 50*time.Millisecond {
		t.Errorf("the wait after a judgment settled is %s, want %s", got, 50*time.Millisecond)
	}
}

// gateTestMovingJudge is a judge seam for a ruleset that keeps moving: the first
// judgment reads it safe, and every later pass answers that the generation bracket
// moved, as when a commit lands during every pass.
func gateTestMovingJudge() *gateTestJudge {
	return &gateTestJudge{pass: func(call int) gateTestVerdict {
		if call == 0 {
			return gateTestVerdict{}
		}
		return gateTestVerdict{err: errESPFormRulesetChanged}
	}}
}

// gateTestMoving is a worker over gateTestMovingJudge with its first wait armed.
type gateTestMoving struct {
	f      *espFormFastPath
	writes *gateTestWrites
	judge  *gateTestJudge
	clock  *gateTestClock
	events chan<- struct{}
	links  chan<- netlink.LinkUpdate
	stop   func()
	timer  gateTestTimer // the first wait armed before judging the moving ruleset again
}

// gateTestMovingWorker runs the worker over gateTestMovingJudge until its first
// judgment opened the gate, signals one commit, and waits until the worker shut the
// gate for that commit and for each pass that saw the ruleset move, then armed the
// wait of the next judgment.
func gateTestMovingWorker(t *testing.T) gateTestMoving {
	t.Helper()
	commits := make(chan struct{}, 1)
	m := gateTestMoving{events: commits, judge: gateTestMovingJudge(), clock: newGateTestClock(t)}
	m.f, m.writes = newGateTestFastPath(t, &gateTestReader{}, commits)
	m.f.judge = m.judge.judge
	m.links, m.stop = gateTestWorkerLinked(t, m.f, commits, espFormRewatcher{open: (&gateTestOpener{}).open, after: m.clock.after, now: m.clock.now})
	if got := m.writes.next(t); got != espFormFastPathGateOpen {
		t.Fatalf("the first judgment wrote %d, want the gate open", got)
	}

	commits <- struct{}{}
	// The commit's shut, then one for each pass that saw the bracket move.
	for i := range 1 + espFormNFTJudgePasses {
		if got := m.writes.next(t); got != espFormFastPathGateClosed {
			t.Fatalf("write %d after the commit is %d, want %d: every change of the ruleset shuts the gate",
				i, got, espFormFastPathGateClosed)
		}
	}
	m.timer = m.clock.next(t)
	if m.timer.wait != espFormNFTRejudgeFirst {
		t.Errorf("the first wait before judging a moving ruleset again is %s, want %s", m.timer.wait, espFormNFTRejudgeFirst)
	}
	// Fired at cleanup, before the stop that cleanup also runs, so a worker that
	// waited for it in place still ends. The channel holds one, so this never blocks.
	fire := m.timer.fire
	t.Cleanup(func() {
		select {
		case fire <- time.Time{}:
		default:
		}
	})
	return m
}

// TestESPFormFastPathGateStaysShutWhileCommitsKeepComing: a stream of commits shuts an
// open gate at the first change and keeps it shut while the commits continue, so the
// gate is never left open over a ruleset no judgment published. Once a judgment ends
// with every pass seeing the ruleset move, the worker arms a wait rather than dump
// again at once, and the next judgment that ends the same way doubles it.
//
// METHOD: the worker's loop over gateTestMovingJudge. The test signals one commit; every
// pass after the first judgment answers that the bracket moved. The clock fires each
// armed wait only when the test says.
// MUTATION: drop the writeGateLocked(espFormFastPathGateClosed) from firewallCommitted:
// the signaled commit and the moved passes then write nothing, the gate stays open
// over the moving ruleset, and the wait for the commit's shut times out.
func TestESPFormFastPathGateStaysShutWhileCommitsKeepComing(t *testing.T) {
	m := gateTestMovingWorker(t)
	if gateTestState(t, m.f).GateOpen {
		t.Errorf("state GateOpen while the ruleset keeps moving")
	}

	m.timer.fire <- time.Time{}
	for i := range espFormNFTJudgePasses {
		if got := m.writes.next(t); got != espFormFastPathGateClosed {
			t.Fatalf("write %d of the judgment after the wait is %d, want %d", i, got, espFormFastPathGateClosed)
		}
	}
	// The worker arms the next wait once the judgment is over.
	if next := m.clock.next(t); next.wait != 2*espFormNFTRejudgeFirst {
		t.Errorf("the wait after a second judgment of a moving ruleset is %s, want %s", next.wait, 2*espFormNFTRejudgeFirst)
	}
	if got := m.writes.all(); slices.Contains(got[1:], espFormFastPathGateOpen) {
		t.Errorf("gate writes %v: the gate opened while the ruleset kept moving", got)
	}
	if got := m.judge.count(); got != 1+2*espFormNFTJudgePasses {
		t.Errorf("the ruleset was judged %d times, want %d: the first judgment, then %d passes in each of two judgments",
			got, 1+2*espFormNFTJudgePasses, espFormNFTJudgePasses)
	}
	if gateTestState(t, m.f).GateOpen {
		t.Errorf("state GateOpen while the ruleset keeps moving")
	}
}

// TestESPFormFastPathMovingRulesetWaitsInTheLoop: the wait armed after a ruleset that
// kept moving runs in the worker's loop, not in the judgment. The worker applies a link
// update while it runs, a commit signaled during it shuts the gate and judges nothing,
// because the armed judgment begins after that commit, and a stop during it returns
// without judging again.
//
// METHOD: gateTestMovingWorker up to the first armed wait, which the clock never fires
// before the stop. The link subscription is unbuffered, so a send on it returns only
// once the loop took the update, after whatever the loop was doing.
// MUTATION: in judgeWatched, wait for the armed judgment in place (receive from
// w.rejudge, then judge again) instead of returning to serve: the loop never takes the
// link update, and the test fails at that send.
func TestESPFormFastPathMovingRulesetWaitsInTheLoop(t *testing.T) {
	m := gateTestMovingWorker(t)
	judged := m.judge.count()

	gateTestSendLink(t, m.links, "while the wait before the next judgment runs")

	m.events <- struct{}{}
	if got := m.writes.next(t); got != espFormFastPathGateClosed {
		t.Fatalf("a commit during the wait wrote %d, want %d", got, espFormFastPathGateClosed)
	}
	// Taken only once the loop finished with the commit.
	gateTestSendLink(t, m.links, "after a commit during the wait")
	if got := m.judge.count(); got != judged {
		t.Errorf("a commit during the wait was judged at once (%d judgments, want %d): the armed judgment begins after it",
			got, judged)
	}

	stopped := make(chan struct{})
	go gateTestStopInto(m.stop, stopped)
	select {
	case <-stopped:
	case <-time.After(gateTestWait):
		t.Fatalf("stop did not return within %s while the wait before the next judgment ran", gateTestWait)
	}
	if got := m.judge.count(); got != judged {
		t.Errorf("the ruleset was judged %d times by the stop, want %d", got, judged)
	}
}

// gateTestSendLink sends one link update to the worker, failing the test when the loop
// does not take it within gateTestWait. The fast path is not attached, so apply
// ignores it.
func gateTestSendLink(t *testing.T, links chan<- netlink.LinkUpdate, when string) {
	t.Helper()
	select {
	case links <- netlink.LinkUpdate{}:
	case <-time.After(gateTestWait):
		t.Fatalf("the worker took no link update %s within %s: the wait starves the loop", when, gateTestWait)
	}
}

// TestESPFormFastPathInterruptedDumpCountsAsMoving: a dump message the kernel flagged
// NLM_F_DUMP_INTR on every pass is a stream of commits, not a failed read. The
// judgment answers Moving; the worker arms the wait before the next judgment, which
// doubles on the next one; the reading becomes Moving and never DumpFailed, so Ze
// never warns, nor the doctor reports, that a dump failed.
//
// METHOD: a judge seam whose every pass answers errESPFormDumpInterrupted wrapped as
// espFormDumpMessage wraps it: first one judgment called directly, then the worker's
// loop with a clock whose waits fire only when the test says.
// MUTATION: drop the errESPFormDumpInterrupted test from espFormRulesetMoved: the first
// pass publishes DumpFailed and the judgment settles, so the direct judgment answers
// Settled, the worker writes the gate once and arms no wait, and the test fails
// waiting for the second pass's write.
func TestESPFormFastPathInterruptedDumpCountsAsMoving(t *testing.T) {
	interrupted := gateTestVerdict{err: fmt.Errorf("list the chains: %w", errESPFormDumpInterrupted)}
	judge := &gateTestJudge{pass: func(int) gateTestVerdict { return interrupted }}

	direct, _ := newGateTestFastPath(t, &gateTestReader{}, nil)
	direct.judge = judge.judge
	if got := direct.judgeFirewall(nil, nil); got != espFormJudgmentMoving {
		t.Errorf("a judgment whose every dump the kernel flagged interrupted answered %d, want Moving (%d)", got, espFormJudgmentMoving)
	}
	if got := gateTestState(t, direct).FirewallReading; got == ESPFormFirewallDumpFailed {
		t.Errorf("FirewallReading %s after interrupted dumps: an interrupted dump is a commit, not a failed read", got)
	}

	events := make(chan struct{}, 1)
	f, writes := newGateTestFastPath(t, &gateTestReader{}, events)
	f.judge = judge.judge
	clock := newGateTestClock(t)
	gateTestWorkerRewatching(t, f, events, espFormRewatcher{open: (&gateTestOpener{}).open, after: clock.after, now: clock.now})
	for judgment, wantWait := range []time.Duration{espFormNFTRejudgeFirst, 2 * espFormNFTRejudgeFirst} {
		for pass := range espFormNFTJudgePasses {
			if got := writes.next(t); got != espFormFastPathGateClosed {
				t.Fatalf("judgment %d, pass %d wrote %d, want %d: each interrupted dump shuts the gate as a commit does",
					judgment, pass, got, espFormFastPathGateClosed)
			}
		}
		timer := clock.next(t)
		if timer.wait != wantWait {
			t.Errorf("judgment %d armed a wait of %s, want %s", judgment, timer.wait, wantWait)
		}
		st := gateTestState(t, f)
		if st.FirewallReading != ESPFormFirewallMoving {
			t.Errorf("judgment %d left FirewallReading %s, want %s", judgment, st.FirewallReading, ESPFormFirewallMoving)
		}
		if st.FirewallCause != "" || st.GateOpen {
			t.Errorf("judgment %d left FirewallCause %q, GateOpen %v, want no cause and a shut gate", judgment, st.FirewallCause, st.GateOpen)
		}
		if judgment == 0 {
			timer.fire <- time.Time{}
		}
	}
}

// TestESPFormFastPathMovingReadingRecordsTheStream: the first judgment of a stream of
// commits that ends Moving sets the Moving reading with the worker clock's time and
// hides the blocking site the judgment before it found; a later Moving judgment keeps
// that time; the judgment that reads the ruleset with nothing moving replaces Moving
// with Read and the site, and clears the time.
//
// METHOD: the worker's loop over a judge seam that reads a blocking rule while the test
// says the ruleset rests, and answers that the generation bracket moved while it says
// the ruleset moves. The clock's waits fire, and its time advances, only when the test
// says.
// MUTATION: drop the firewallMoving call from judgeWatched: the state reads Read with
// the site once the stream began, and FirewallMovingSince stays zero.
// MUTATION: make firewallMoving set firewallMovingSince on every call: the second
// Moving judgment moves FirewallMovingSince five seconds on.
// MUTATION: drop the endMovingLocked call from blockGateLocked: the state still reads
// Moving after the judgment that read the rule again.
func TestESPFormFastPathMovingReadingRecordsTheStream(t *testing.T) {
	var moving atomic.Bool
	judge := &gateTestJudge{pass: func(int) gateTestVerdict {
		if moving.Load() {
			return gateTestVerdict{err: errESPFormRulesetChanged}
		}
		return gateTestVerdict{blocked: true, site: gateTestSite}
	}}
	events := make(chan struct{}, 1)
	f, writes := newGateTestFastPath(t, &gateTestReader{}, events)
	f.judge = judge.judge
	clock := newGateTestClock(t)
	gateTestWorkerRewatching(t, f, events, espFormRewatcher{open: (&gateTestOpener{}).open, after: clock.after, now: clock.now})
	if got := writes.next(t); got != espFormFastPathGateClosed {
		t.Fatalf("the first judgment wrote %d over a blocking rule, want %d", got, espFormFastPathGateClosed)
	}
	if st := gateTestState(t, f); st.FirewallReading != ESPFormFirewallRead || !st.FirewallBlocked || !st.FirewallMovingSince.IsZero() {
		t.Fatalf("before the stream: reading %s, blocked %v, moving since %v, want %s, blocked and no time",
			st.FirewallReading, st.FirewallBlocked, st.FirewallMovingSince, ESPFormFirewallRead)
	}

	moving.Store(true)
	events <- struct{}{}
	// The commit's shut, then one for each pass that saw the bracket move.
	for i := range 1 + espFormNFTJudgePasses {
		if got := writes.next(t); got != espFormFastPathGateClosed {
			t.Fatalf("write %d after the commit is %d, want %d", i, got, espFormFastPathGateClosed)
		}
	}
	timer := clock.next(t)
	st := gateTestState(t, f)
	if st.FirewallReading != ESPFormFirewallMoving || !st.FirewallMovingSince.Equal(gateTestEpoch) {
		t.Errorf("after the first Moving judgment: reading %s since %v, want %s since %v",
			st.FirewallReading, st.FirewallMovingSince, ESPFormFirewallMoving, gateTestEpoch)
	}
	if st.FirewallBlocked || st.FirewallSite != "" || st.FirewallAcceptFix != "" {
		t.Errorf("under Moving the state carries the last site: blocked %v, site %q, fix %q, want none",
			st.FirewallBlocked, st.FirewallSite, st.FirewallAcceptFix)
	}

	clock.advance(5 * time.Second)
	timer.fire <- time.Time{}
	for i := range espFormNFTJudgePasses {
		if got := writes.next(t); got != espFormFastPathGateClosed {
			t.Fatalf("pass %d of the second Moving judgment wrote %d, want %d", i, got, espFormFastPathGateClosed)
		}
	}
	timer = clock.next(t)
	if got := gateTestState(t, f).FirewallMovingSince; !got.Equal(gateTestEpoch) {
		t.Errorf("after the second Moving judgment FirewallMovingSince is %v, want %v: it is the end of the first judgment that met the stream", got, gateTestEpoch)
	}

	moving.Store(false)
	timer.fire <- time.Time{}
	if got := writes.next(t); got != espFormFastPathGateClosed {
		t.Fatalf("the judgment after the stream wrote %d over the blocking rule, want %d", got, espFormFastPathGateClosed)
	}
	st = gateTestState(t, f)
	if st.FirewallReading != ESPFormFirewallRead || !st.FirewallMovingSince.IsZero() {
		t.Errorf("after the stream ended: reading %s since %v, want %s and no time", st.FirewallReading, st.FirewallMovingSince, ESPFormFirewallRead)
	}
	if !st.FirewallBlocked || st.FirewallSite != gateTestSite.String() {
		t.Errorf("after the stream ended: blocked %v at %q, want blocked at %q", st.FirewallBlocked, st.FirewallSite, gateTestSite.String())
	}
}

// gateTestStream is a worker in a stream of commits whose first judgment ended Moving:
// its judge answers that the generation bracket moved while moving is set, and what
// rest points at otherwise; timer is the wait armed before the next judgment.
type gateTestStream struct {
	f      *espFormFastPath
	writes *gateTestWrites
	moving *atomic.Bool
	rest   *atomic.Pointer[gateTestVerdict]
	timer  gateTestTimer
}

// gateTestStartStream runs the worker over a judge that reads rest while the ruleset
// rests, waits for the first judgment's write, which must be first, then sets moving,
// signals one commit, and waits for the commit's shut, one shut for each pass that saw
// the bracket move, and the wait armed once the judgment ended Moving. It fails the
// test unless the state then reads Moving since gateTestEpoch, so a test of how a
// stream ends starts from a stream.
func gateTestStartStream(t *testing.T, rest gateTestVerdict, first uint32) *gateTestStream {
	t.Helper()
	s := &gateTestStream{moving: &atomic.Bool{}, rest: &atomic.Pointer[gateTestVerdict]{}}
	s.rest.Store(&rest)
	judge := &gateTestJudge{pass: func(int) gateTestVerdict {
		if s.moving.Load() {
			return gateTestVerdict{err: errESPFormRulesetChanged}
		}
		return *s.rest.Load()
	}}
	events := make(chan struct{}, 1)
	s.f, s.writes = newGateTestFastPath(t, &gateTestReader{}, events)
	s.f.judge = judge.judge
	clock := newGateTestClock(t)
	gateTestWorkerRewatching(t, s.f, events, espFormRewatcher{open: (&gateTestOpener{}).open, after: clock.after, now: clock.now})
	if got := s.writes.next(t); got != first {
		t.Fatalf("the first judgment wrote %d, want %d", got, first)
	}

	s.moving.Store(true)
	events <- struct{}{}
	for i := range 1 + espFormNFTJudgePasses {
		if got := s.writes.next(t); got != espFormFastPathGateClosed {
			t.Fatalf("write %d after the commit is %d, want %d", i, got, espFormFastPathGateClosed)
		}
	}
	s.timer = clock.next(t)
	if st := gateTestState(t, s.f); st.FirewallReading != ESPFormFirewallMoving || !st.FirewallMovingSince.Equal(gateTestEpoch) {
		t.Fatalf("the stream did not begin: reading %s since %v, want %s since %v",
			st.FirewallReading, st.FirewallMovingSince, ESPFormFirewallMoving, gateTestEpoch)
	}
	return s
}

// end sets what the judge reads once the ruleset rests, ends the stream, and fires the
// armed wait, so the worker's next judgment reads rest with nothing moving.
func (s *gateTestStream) end(rest gateTestVerdict) {
	s.rest.Store(&rest)
	s.moving.Store(false)
	s.timer.fire <- time.Time{}
}

// TestESPFormFastPathMovingStreamEndsOnASafeRead: a stream of commits that ends on a
// judgment that reads the ruleset and finds no rule that can drop ESP replaces Moving
// with Read, opens the gate and clears FirewallMovingSince.
//
// METHOD: gateTestStartStream over a ruleset that is safe at rest; the wait after the
// first Moving judgment fires with the ruleset at rest and safe again.
// MUTATION: drop the endMovingLocked call from clearFirewallLocked: the gate opens, but
// the state still reads Moving since gateTestEpoch.
func TestESPFormFastPathMovingStreamEndsOnASafeRead(t *testing.T) {
	s := gateTestStartStream(t, gateTestVerdict{}, espFormFastPathGateOpen)
	s.end(gateTestVerdict{})
	if got := s.writes.next(t); got != espFormFastPathGateOpen {
		t.Fatalf("the judgment after the stream wrote %d over a safe ruleset, want %d", got, espFormFastPathGateOpen)
	}
	st := gateTestState(t, s.f)
	if st.FirewallReading != ESPFormFirewallRead || !st.FirewallMovingSince.IsZero() {
		t.Errorf("after the stream ended on a safe read: reading %s since %v, want %s and no time",
			st.FirewallReading, st.FirewallMovingSince, ESPFormFirewallRead)
	}
	if !st.GateOpen || st.FirewallBlocked || st.FirewallCause != "" {
		t.Errorf("after the stream ended on a safe read: GateOpen %v, blocked %v, cause %q, want open, not blocked, no cause",
			st.GateOpen, st.FirewallBlocked, st.FirewallCause)
	}
}

// TestESPFormFastPathMovingStreamEndsOnAFailedDump: a stream of commits that ends on a
// judgment whose dump failed, not interrupted by a commit but failed, replaces Moving
// with DumpFailed and the dump's failure as the cause, keeps the gate shut, and clears
// FirewallMovingSince.
//
// METHOD: gateTestStartStream over a ruleset that is safe at rest; the wait after the
// first Moving judgment fires with the judge answering a failed dump.
// MUTATION: drop the endMovingLocked call from shutGateLocked: the state still reads
// Moving since gateTestEpoch, and carries no cause.
func TestESPFormFastPathMovingStreamEndsOnAFailedDump(t *testing.T) {
	s := gateTestStartStream(t, gateTestVerdict{}, espFormFastPathGateOpen)
	failure := errors.New("list the chains: netlink receive: no buffer space available")
	s.end(gateTestVerdict{err: failure})
	if got := s.writes.next(t); got != espFormFastPathGateClosed {
		t.Fatalf("the judgment after the stream wrote %d over a failed dump, want %d", got, espFormFastPathGateClosed)
	}
	st := gateTestState(t, s.f)
	if st.FirewallReading != ESPFormFirewallDumpFailed || !st.FirewallMovingSince.IsZero() {
		t.Errorf("after the stream ended on a failed dump: reading %s since %v, want %s and no time",
			st.FirewallReading, st.FirewallMovingSince, ESPFormFirewallDumpFailed)
	}
	if st.FirewallCause != failure.Error() || st.GateOpen {
		t.Errorf("after the stream ended on a failed dump: cause %q, GateOpen %v, want %q and a shut gate",
			st.FirewallCause, st.GateOpen, failure.Error())
	}
}

// TestESPFormFastPathStopEndsTheMovingReading: stop during a stream of commits clears
// the Moving reading with the rest of the firewall state: the reading is Unheld,
// FirewallMovingSince is zero, and the gate is shut.
//
// METHOD: gateTestStartStream, then stop while the worker waits to judge again. The
// worker runs under the harness's own done, which stop does not hold, so it stays
// idle in that wait until the test's cleanup stops it.
// MUTATION: drop the endMovingLocked call from stop: the state reads Moving since
// gateTestEpoch after stop.
func TestESPFormFastPathStopEndsTheMovingReading(t *testing.T) {
	s := gateTestStartStream(t, gateTestVerdict{}, espFormFastPathGateOpen)
	s.f.stop()
	st := gateTestState(t, s.f)
	if st.FirewallReading != ESPFormFirewallUnheld || !st.FirewallMovingSince.IsZero() {
		t.Errorf("after stop during a stream: reading %s since %v, want %s and no time",
			st.FirewallReading, st.FirewallMovingSince, ESPFormFirewallUnheld)
	}
	if st.GateOpen {
		t.Errorf("GateOpen after stop during a stream")
	}
}

// TestESPFormFastPathRefusedCommitShutDetaches: a change of the ruleset the worker sees
// (firewallCommitted) whose write of 0 the gate map refuses detaches the fast path, as a
// refused hold does: the state turns Detached with the failure as its cause,
// GateOpen is clear, and no link stays attached.
//
// METHOD: as TestESPFormFastPathFailedShutDetaches, with the refused write made by
// firewallCommitted rather than by a hold; the state is read under mu
// (gateTestAttached), because the fast path marked Attached holds no counter map.
// MUTATION: in firewallCommitted, write 0 through f.writeGate and drop its error
// rather than call writeGateLocked: the status stays Attached and GateOpen stays set
// over a gate the map never shut.
func TestESPFormFastPathRefusedCommitShutDetaches(t *testing.T) {
	f, writes := newGateTestFastPath(t, &gateTestReader{}, nil)
	var refuse atomic.Bool
	f.writeGate = func(value uint32) error {
		if refuse.Load() {
			return errors.New("map update: bad file descriptor")
		}
		return writes.write(value)
	}
	f.mu.Lock()
	f.status = ESPFormFastPathAttached
	f.links = make(map[int]espFormAttachedLink)
	f.refused = make(map[int]ESPFormLinkRefusal)
	f.mu.Unlock()
	f.judgeFirewall(nil, nil)
	if !gateTestAttached(f).gateOpen {
		t.Fatalf("the gate did not open over a safe ruleset before the refused shut")
	}

	refuse.Store(true)
	f.firewallCommitted()
	st := gateTestAttached(f)
	if st.status != ESPFormFastPathDetached {
		t.Errorf("status %s after a commit whose shut the map refused, want %s", st.status, ESPFormFastPathDetached)
	}
	if !strings.Contains(st.cause, "bad file descriptor") || !strings.Contains(st.cause, "detached from every link") {
		t.Errorf("Cause %q does not name the refused write and the detach", st.cause)
	}
	if st.gateOpen {
		t.Errorf("GateOpen after a commit whose shut the map refused: the program would rewrite past the new ruleset unjudged")
	}
	f.mu.Lock()
	attached := len(f.links)
	f.mu.Unlock()
	if attached != 0 {
		t.Errorf("%d links still attached after the refused shut, want 0", attached)
	}
}
