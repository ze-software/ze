// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- one Child SA receives both ESP forms
// Overview: espform_fastpath_linux.go -- the fast path whose worker runs the judgments here
// Related: espform_firewall_linux.go -- espFormFirewallBlocks, the judgment of the ruleset, and the nftables watch
// Related: espform_program_linux.go -- the gate map the program reads before the packet
// Related: espform_firewall_hold.go -- ESPFormFirewallHolder, the hold a firewall commit takes

//go:build linux

package dataplane

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cilium/ebpf"
	"github.com/vishvananda/netns"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// The firewall gate. A datagram the program rewrites passes every netfilter hook as
// UDP, so an nftables rule on the input path that would drop the bare ESP never sees
// it. The gate is open only while a judgment of the kernel's ruleset found no such
// rule (espFormFirewallBlocks), and the manager keeps three facts for that:
//
//   - the epoch, which steps on every hold, every release and every change of the
//     ruleset the worker sees: a commit the watch signals, or a judgment whose
//     generation bracket moved. A judgment reads the epoch before it dumps the
//     ruleset and publishes only when the epoch is unchanged after, so a judgment
//     that began before the latest of those never opens the gate. Each change also
//     shuts the gate, as a hold does (firewallCommitted), so the gate is open only
//     over a ruleset some judgment published, never across a change the watch
//     delivered, however fast the commits come;
//   - the hold count. While a hold is open the gate stays shut, whatever a judgment
//     finds, and each release asks the worker for a judgment that begins after it;
//   - firewallLost. While the ruleset cannot be read or watched, a rule that lands
//     would go unseen, so the gate stays shut. A ruleset unread at start stays so
//     until stop. A watch that fails is opened again by the worker, after a wait
//     that doubles from espFormNFTRewatchFirst to espFormNFTRewatchMax; once it is
//     up, a fresh judgment covers every commit made while nothing watched.
//
// A dump that fails while the watch lives shuts the gate too (DumpFailed), and leaves
// firewallLost clear: the watch still signals each commit, so the next commit or the
// next release judges again, and nothing judges before then.
//
// A commit by another program is seen only after it landed: the window is the
// watch's latency, plus the end of the one dump the worker may be running when the
// signal arrives, after which the signal shuts the gate. Packets in that window are
// still decrypted and checked against the SA's selectors; what they miss is the
// firewall's decision. A stream of commits keeps the gate shut, and the worker waits
// between the judgments it runs into one (espFormNFTRejudgeFirst). From the first
// judgment of such a stream that published nothing, the state reads Moving, with the
// time that judgment ended (firewallMoving), until a judgment publishes or a shut
// records its own reading, so the doctor can report a stream that keeps the fast path
// off (ESPFormFirewallMovingReportAfter).
//
// One removal is never signaled. When the process that owns a table created with
// the owner flag (NFT_TABLE_F_OWNER) exits, the kernel removes that table outside any
// commit, with no generation bump and no NEWGEN (nft_rcv_nl_event and
// __nft_release_table, net/netfilter/nf_tables_api.c, v7.0). A gate that table shut
// stays shut until the next commit judges again. That costs the fast path, never the
// firewall's decision: an accept in one base chain does not stop another's, so
// removing a table can make no ruleset that was safe unsafe.

// espFormZeTablePrefix starts the kernel name of every table Ze owns, as
// firewall.StripZeTablePrefix (internal/component/firewall/accessor.go) strips it.
const espFormZeTablePrefix = "ze_"

// espFormFirewallAccept answers what the operator changes so the base chain site
// names no longer keeps the fast path off: how, for the doctor's message, and fix,
// the command. Ze rewrites a ze_ table, so an nft rule inserted there would not
// last: the registered ESPFormOwnedTableAccept says who writes it and what changes
// it. Any other table takes the nft rule itself.
func espFormFirewallAccept(site espFormFirewallSite) (how, fix string) {
	if !strings.HasPrefix(site.Table, espFormZeTablePrefix) {
		return espFormForeignTableAccept(site)
	}
	owned := espFormOwnedTableAccept.Load()
	// refuse-by-default: guards blockGateLocked -- an empty fix is the state it logs, and the doctor reports, as a Ze defect
	if owned == nil {
		return "", ""
	}
	return (*owned)(site.Family, site.Table, site.Chain)
}

// espFormForeignTableAccept answers the nft insert that accepts ESP at the head of
// the base chain of a table Ze does not own; meta l4proto matches ESP in every family
// the judgment walks (inet, ip, netdev, bridge), and the family is one of those words.
// The command is one shell word in single quotes, so the shell expands nothing in
// it, and the table and chain names are nft identifiers in double quotes, which nft
// accepts for any name (nft(8), "Lexical conventions"). A name holding either quote
// or a control character cannot be carried that way, so no command is printed for
// it: the message says so and the fix lists the ruleset.
//
// A table created with the owner flag (NFT_TABLE_F_OWNER) refuses a change from any
// other process; the site does not carry the table's flags, so the message says that
// such a table is changed only by the process that owns it.
func espFormForeignTableAccept(site espFormFirewallSite) (how, fix string) {
	if !espFormNFTQuotable(site.Table) {
		return espFormForeignTableUnquotable(site)
	}
	if !espFormNFTQuotable(site.Chain) {
		return espFormForeignTableUnquotable(site)
	}
	var cmd textbuf.Buffer
	cmd.Str("sudo nft 'insert rule ").Str(site.Family).Str(" \"").Str(site.Table).Str("\" \"").Str(site.Chain).
		Str("\" meta l4proto esp accept'")
	var why textbuf.Buffer
	why.Str("Table ").Str(site.Table).Str(" is not Ze's, so the command below inserts the accept at the head " +
		"of its chain ").Str(site.Chain).Str(".").Str(espFormOwnerFlagNote)
	return why.String(), cmd.String()
}

// espFormOwnerFlagNote closes every answer espFormForeignTableAccept gives: the site
// does not carry the table's flags, so the message says what the owner flag does.
const espFormOwnerFlagNote = " A table created with the owner flag, which `nft list ruleset` marks `# owner`, " +
	"refuses that change from any other process: only the process that owns it can make it."

// espFormForeignTableUnquotable answers espFormForeignTableAccept for a site whose table
// or chain name cannot be written as a quoted nft identifier: no command to paste, and
// the ruleset listing as the fix.
func espFormForeignTableUnquotable(site espFormFirewallSite) (how, fix string) {
	var why textbuf.Buffer
	why.Str("Table ").Str(strconv.Quote(site.Table)).Str(", which Ze does not own, or its chain ").
		Str(strconv.Quote(site.Chain)).Str(", has a name holding a quote or a control character, " +
		"so Ze prints no command to paste: insert a rule accepting meta l4proto esp at the head of " +
		"that chain with nft. The command below lists the ruleset with the names as nft spells them.").
		Str(espFormOwnerFlagNote)
	return why.String(), "sudo nft list ruleset"
}

// espFormNFTQuotable answers whether name can be written as a double-quoted nft
// identifier inside a single-quoted shell word: it holds no quote of either kind and
// no control character.
func espFormNFTQuotable(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		if r == '\'' {
			return false
		}
		if r == '"' {
			return false
		}
		if r < ' ' {
			return false
		}
		if r == 0x7f {
			return false
		}
	}
	return true
}

// espFormFirewallRereadWhen answers what makes Ze read the ruleset again while the
// gate is shut under reading, for the warning to say. Every reading is listed: the
// set is closed, Ze alone produces it.
func espFormFirewallRereadWhen(reading ESPFormFirewallReading) string {
	switch reading {
	case ESPFormFirewallUnreadable, ESPFormFirewallWatchClosed:
		return "nothing reads the ruleset again until ze restarts, or until no Child SA is served " +
			"in both ESP forms and one is installed again"
	case ESPFormFirewallDumpFailed:
		return "no restart is needed: the watch still runs, so the next commit to the ruleset, by " +
			"Ze's firewall or by any other program, reads it again, and nothing reads it before " +
			"then; a restart reads it at once"
	case ESPFormFirewallRewatching:
		return "no restart is needed: ze watches the ruleset again by itself, at most " +
			espFormNFTRewatchMax.String() + " apart, and reads it once the watch is up"
	case ESPFormFirewallRead:
		return "the next commit to the ruleset reads it again"
	case ESPFormFirewallUnspecified, ESPFormFirewallUnheld, ESPFormFirewallPending, ESPFormFirewallMoving:
		panic("BUG: esp-form: the gate was shut for a reason under the reading " + reading.String())
	}
	panic("BUG: esp-form: the gate was shut under an unknown firewall reading " + reading.String())
}

// espFormFirewallWatchEnded is the cause recorded when Close ended the watch while the
// worker still read it. Nothing watches the ruleset again until the next start.
const espFormFirewallWatchEnded = "the nftables watch ended, so a rule added from now on would go unseen; " +
	"the fast path stays shut until no Child SA is served in both ESP forms and one is installed again"

// espFormFirewallRewatchFix is the fix the warning of a failed watch names: the
// command that lists why the watch keeps failing, as the doctor finding names it.
// Ze watches the ruleset again by itself, so a restart is no fix.
const espFormFirewallRewatchFix = ESPFormFixLog

// espFormNFTRewatchFirst is the wait before the first attempt to watch the ruleset
// again after the watch failed. Each attempt that fails doubles it, up to
// espFormNFTRewatchMax; a watch that opens starts the next failure at it again.
const espFormNFTRewatchFirst = time.Second

// espFormNFTJudgePasses is how many passes in a row one judgment runs while each sees
// the ruleset move, before it hands back to the worker's loop. The second pass follows
// the first at once: one commit landing during a dump is the usual reason a pass saw
// a move. A move on every pass is a stream of commits, which the loop meets with a
// wait (espFormNFTRejudgeFirst).
const espFormNFTJudgePasses = 2

// espFormNFTRejudgeFirst is the wait before the next judgment once one ended with
// every pass seeing the ruleset move. Each judgment that ends the same way doubles it,
// up to espFormNFTRejudgeMax, and one that settles starts the next stream at it again.
// The gate is shut through the wait, because each change the worker saw shut it.
// Without the wait a stream of commits (fail2ban adding a set element every few
// milliseconds, each element a commit that bumps the generation) has the worker dump
// the ruleset back to back and never publish; with it, a judgment runs at most
// espFormNFTJudgePasses dumps in each wait. The cap bounds how long after the stream
// ends the next judgment begins.
const espFormNFTRejudgeFirst = 50 * time.Millisecond

// espFormNFTRejudgeMax caps the doubling of espFormNFTRejudgeFirst.
const espFormNFTRejudgeMax = time.Second

// espFormJudgment is how one judgment of the ruleset ended (judgeFirewall). The zero
// value is never answered.
type espFormJudgment uint8

const (
	espFormJudgmentUnspecified espFormJudgment = iota
	// espFormJudgmentSettled: a pass published, or there is nothing to judge: done is
	// closed, or the ruleset is lost.
	espFormJudgmentSettled
	// espFormJudgmentMoving: espFormNFTJudgePasses passes in a row published nothing,
	// each because the ruleset moved or a hold or release stepped the epoch while it
	// read. The gate is shut.
	espFormJudgmentMoving
	// espFormJudgmentUnwatched: the watch's channel closed during a pass.
	espFormJudgmentUnwatched
)

// espFormFirewallWatchFailed answers the cause recorded while the watch is down after
// a failure, its own or that of an attempt to watch again. The wait before the next
// attempt is not in it, so a failure that repeats logs once.
func espFormFirewallWatchFailed(err error) string {
	return "the nftables watch failed (" + err.Error() + "), so a rule added now would go unseen; " +
		"the fast path stays shut until ze watches the ruleset again, which it tries at most " +
		espFormNFTRewatchMax.String() + " apart"
}

// espFormRulesetWatch is what the fast path holds of a running watch of the ruleset:
// an *espFormNFTWatch, or a test's fake.
type espFormRulesetWatch interface {
	// Err names why the watch's channel closed, nil when Close ended it. MUST be read
	// after the channel closed.
	Err() error
	// Close stops the watch. A watch that already ended closes what it still holds.
	Close()
}

// espFormFirewallView is what the fast path reads the ruleset through: the reader a
// judgment dumps with, and the watch whose channel signals each commit. The zero
// value holds nothing.
type espFormFirewallView struct {
	reader      espFormNFTReader
	closeReader func() error
	watch       espFormRulesetWatch // nil when nothing is watched
	events      <-chan struct{}     // the watch's channel, nil when nothing is watched
}

// espFormRewatcher is what the worker watches the ruleset again with: open starts a
// watch in the fast path's namespace, after answers a channel that fires once a wait
// passed (time.After, or a test's clock), and now answers that clock's time (time.Now,
// or the test's). The worker arms two waits with after: the one before it watches
// again, and the one before it judges a ruleset that kept moving. It reads now when a
// stream of commits begins, for the time the state reports with Moving.
type espFormRewatcher struct {
	open  func() (espFormRulesetWatch, <-chan struct{}, error)
	after func(time.Duration) <-chan time.Time
	now   func() time.Time
}

// espFormRewatchIn answers the opener that watches the ruleset of ns again. ns MUST
// stay open while the opener can be called.
func espFormRewatchIn(ns netns.NsHandle) func() (espFormRulesetWatch, <-chan struct{}, error) {
	return func() (espFormRulesetWatch, <-chan struct{}, error) {
		watch, events, err := startESPFormNFTWatch(ns)
		if err != nil {
			return nil, nil, err
		}
		return watch, events, nil
	}
}

// espFormWatchLoop is the worker's hold on the watch: the channel it reads commits
// from, while the watch is down after a failure the timer of the next attempt to
// watch again, and while the ruleset keeps moving the timer of the next judgment.
// Only the worker touches it, so it is not safe for concurrent use.
type espFormWatchLoop struct {
	rewatch espFormRewatcher
	events  <-chan struct{}  // the watch's commits, nil while nothing is watched
	retry   <-chan time.Time // the next attempt, nil unless the watch is down after a failure
	wait    time.Duration    // the last wait armed, zero once a watch opened
	// rejudge is the next judgment, armed when one ended Moving; nil otherwise.
	rejudge <-chan time.Time
	// rejudgeWait is the last wait armed for rejudge, zero once a judgment settled.
	rejudgeWait time.Duration
}

// nextWait answers the wait before the next attempt: espFormNFTRewatchFirst after a
// watch that opened, then double the last, capped at espFormNFTRewatchMax.
func (w *espFormWatchLoop) nextWait() time.Duration {
	w.wait = espFormDoubled(w.wait, espFormNFTRewatchFirst, espFormNFTRewatchMax)
	return w.wait
}

// nextRejudgeWait answers the wait before the next judgment of a ruleset that kept
// moving: espFormNFTRejudgeFirst after a judgment that settled, then double the last,
// capped at espFormNFTRejudgeMax.
func (w *espFormWatchLoop) nextRejudgeWait() time.Duration {
	w.rejudgeWait = espFormDoubled(w.rejudgeWait, espFormNFTRejudgeFirst, espFormNFTRejudgeMax)
	return w.rejudgeWait
}

// espFormDoubled answers the wait that follows last: first when no wait was armed
// (last is zero), else twice last, capped at ceiling.
func espFormDoubled(last, first, ceiling time.Duration) time.Duration {
	if last == 0 {
		return first
	}
	return min(2*last, ceiling)
}

// openESPFormFirewallView opens the reader in the calling thread's namespace, which is
// start's: its sockets are opened here and stay bound there, whichever thread dumps
// through them later. The watch is started in ns. A failure closes what was opened
// and answers why.
func openESPFormFirewallView(ns netns.NsHandle) (espFormFirewallView, error) {
	reader, closeReader, err := openESPFormNFTReader()
	if err != nil {
		return espFormFirewallView{}, fmt.Errorf("open the nftables ruleset reader: %w", err)
	}
	watch, events, err := startESPFormNFTWatch(ns)
	if err != nil {
		return espFormFirewallView{}, errors.Join(fmt.Errorf("watch the nftables ruleset for commits: %w", err), closeReader())
	}
	return espFormFirewallView{reader: reader, closeReader: closeReader, watch: watch, events: events}, nil
}

// closeFirewall closes the watch, then the reader. MUST be called without mu, after
// the worker returned.
func (f *espFormFastPath) closeFirewall(view espFormFirewallView) {
	if view.watch != nil {
		view.watch.Close()
	}
	if view.closeReader == nil {
		return
	}
	if err := view.closeReader(); err != nil {
		f.logger().Warn("esp-form: release the fast path's nftables reader",
			"error", err, "code", diagnostic.CodeDoctorIPsecESPFormFirewall)
	}
}

// adoptFirewallLocked records what start opened to read the ruleset. When err says it
// could not open, the gate stays shut until stop, with err as the cause. MUST be
// called with mu held.
func (f *espFormFastPath) adoptFirewallLocked(view espFormFirewallView, err error) {
	f.firewall = view
	if err == nil {
		f.firewallReading = ESPFormFirewallPending
		return
	}
	f.firewallLost = true
	f.shutGateLocked(ESPFormFirewallUnreadable, err.Error(), ESPFormFixRestart)
}

// judgeWatched judges the ruleset through judgeFirewall and answers how the judgment
// ended. One that settled forgets the wait; one that kept seeing the ruleset move,
// with the gate shut, records the stream of commits (firewallMoving) and arms the next
// judgment after a wait (nextRejudgeWait); and when the watch's channel closed during
// it, the end is answered as watchLost does. The wait runs in the worker's loop
// (serve), never here, so link updates are applied and stop is seen while it runs.
func (f *espFormFastPath) judgeWatched(w *espFormWatchLoop, done <-chan struct{}) {
	w.rejudge = nil
	switch f.judgeFirewall(w.events, done) {
	case espFormJudgmentSettled:
		w.rejudgeWait = 0
	case espFormJudgmentMoving:
		f.firewallMoving(w.rewatch.now())
		w.rejudge = w.rewatch.after(w.nextRejudgeWait())
	case espFormJudgmentUnwatched:
		f.watchLost(w)
	case espFormJudgmentUnspecified:
		panic("BUG: esp-form: a judgment of the nftables ruleset ended with no outcome")
	}
}

// watchLost answers the end of the watch: the gate shuts, the ended watch is closed,
// and when it ended on a failure the next attempt to watch again is armed. A watch
// that Close ended is not watched again. A judgment armed after a moving ruleset is
// dropped: with no watch there is nothing to judge until one opens, and the judgment
// that follows an open (watchAgain) begins after it.
func (f *espFormFastPath) watchLost(w *espFormWatchLoop) {
	w.events = nil
	w.rejudge, w.rejudgeWait = nil, 0
	ended, failed := f.firewallWatchEnded()
	ended.Close()
	if !failed {
		return
	}
	w.retry = w.rewatch.after(w.nextWait())
}

// watchAgain is one attempt to watch the ruleset again, when the wait armed after a
// failure passed. An attempt that fails records why and arms a longer wait. One that
// opens adopts the watch and then judges the ruleset with a dump that begins after
// the watch is up, so every commit made while nothing watched is in what it reads.
func (f *espFormFastPath) watchAgain(w *espFormWatchLoop, done <-chan struct{}) {
	w.retry = nil
	select {
	case <-done:
		return
	default:
	}
	watch, events, err := w.rewatch.open()
	if err != nil {
		f.firewallRewatchFailed(err)
		w.retry = w.rewatch.after(w.nextWait())
		return
	}
	w.wait = 0
	f.firewallWatched(watch, events)
	w.events = events
	f.judgeWatched(w, done)
}

// judgeFirewall judges the ruleset and publishes the first pass that read it with
// nothing moving. The dump runs holding no lock. A change of the ruleset a pass sees,
// a commit the watch signaled before the pass ended, a generation bracket that moved
// (errESPFormRulesetChanged) or a dump message the kernel flagged interrupted
// (errESPFormDumpInterrupted; espFormRulesetMoved names both), shuts the gate and
// steps the epoch (firewallCommitted), and the pass publishes nothing; nor does a
// pass during which a hold or a release stepped the epoch, and the hold shut the
// gate. The next pass follows at once. After espFormNFTJudgePasses passes in a row
// that published nothing it answers Moving, and the worker's loop records the stream
// and judges again after a wait (judgeWatched), the gate shut meanwhile.
//
// The bracket catches a commit that starts and ends inside one pass without waiting
// for the watch, but it does not refuse every splice. nf_tables_commit
// (net/netfilter/nf_tables_api.c, v7.0) stores the bumped generation at line 10875,
// "smp_store_release(&net->nft.base_seq, base_seq);", before it flips the generation
// cursor that decides which rules are active, at 10880, "net->nft.gencursor =
// nft_gencursor_next(net);", and before its commit loop applies table and chain
// updates, at 10896, "nf_tables_table_disable(net, table);", and 10912,
// "nft_chain_commit_update(nft_trans_container_chain(trans));". GETGEN is answered
// under RCU (nf_tables_getgen is NFNL_CB_RCU, line 9944), not under the commit mutex.
// So a pass's first read can answer the new number while its dumps still read the
// old state, and both reads agree. The NEWGEN is sent after that loop, at 11083,
// "nf_tables_gen_notify(net, skb, NFT_MSG_NEWGEN);", so the watch signals every such
// commit once it applied, and the signal shuts the gate and judges again: here when it
// arrives before the pass ends, in the worker's loop after. A judgment that such a
// commit spliced is therefore open for at most the watch's latency, the same window
// as a foreign commit that lands while the gate is open.
//
// It answers Unwatched, recording nothing, when the watch's channel closed: the caller
// answers that (watchLost). It answers Settled at once when done is closed or the
// ruleset is lost.
func (f *espFormFastPath) judgeFirewall(events, done <-chan struct{}) espFormJudgment {
	judge := f.judge
	if judge == nil {
		judge = espFormFirewallBlocks
	}
	for range espFormNFTJudgePasses {
		select {
		case <-done:
			return espFormJudgmentSettled
		default:
		}
		f.mu.Lock()
		reader, epoch, lost := f.firewall.reader, f.epoch, f.firewallLost
		f.mu.Unlock()
		if lost {
			return espFormJudgmentSettled
		}

		blocked, site, err := judge(reader)

		select {
		case _, ok := <-events:
			if !ok {
				return espFormJudgmentUnwatched
			}
			f.firewallCommitted()
			continue
		default:
		}
		if espFormRulesetMoved(err) {
			// A commit landed during this pass, and its NEWGEN may still be on its way
			// to the watch: shut the gate and judge again, as for a signaled commit,
			// publishing nothing read across it.
			f.firewallCommitted()
			continue
		}
		if f.publishFirewall(epoch, blocked, site, err) {
			return espFormJudgmentSettled
		}
	}
	return espFormJudgmentMoving
}

// espFormRulesetMoved answers whether err says the ruleset changed while a pass read
// it: the generation bracket moved (errESPFormRulesetChanged), or the kernel flagged a
// message of a dump NLM_F_DUMP_INTR (errESPFormDumpInterrupted). Either is a commit,
// never a failed read, so the pass counts as moved: a stream of such commits meets the
// wait between judgments, not a DumpFailed reading that the next pass replaces at once.
func espFormRulesetMoved(err error) bool {
	if errors.Is(err, errESPFormRulesetChanged) {
		return true
	}
	return errors.Is(err, errESPFormDumpInterrupted)
}

// publishFirewall publishes one judgment that read the epoch epoch, and answers
// false, publishing nothing, when the epoch has stepped since. A judgment that could
// not read the ruleset, or found a rule that can drop ESP, shuts the gate; one that
// found none opens it while no hold is open. A dump that failed is DumpFailed, not
// Unreadable: the watch that let this judgment run still signals each commit, so the
// next one judges again, and firewallLost stays clear. A dump the kernel flagged
// interrupted never reaches here: judgeFirewall counts it as a move.
func (f *espFormFastPath) publishFirewall(epoch uint64, blocked bool, site espFormFirewallSite, err error) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.epoch != epoch {
		return false
	}
	if err != nil {
		f.shutGateLocked(ESPFormFirewallDumpFailed, err.Error(), ESPFormFixRestart)
		return true
	}
	if blocked {
		how, fix := espFormFirewallAccept(site)
		f.blockGateLocked(site.String(), how, fix)
		return true
	}
	f.clearFirewallLocked()
	if f.holds == 0 {
		f.writeGateLocked(espFormFastPathGateOpen)
	}
	return true
}

// firewallCommitted answers a change of the ruleset the worker saw: a commit the watch
// signaled, or a pass whose generation bracket moved. As a hold does, it steps the
// epoch, so no judgment that began before the change publishes, and writes 0 into the
// gate, so the gate is never open across a change until a judgment that began after
// it publishes. The firewall reading is left as it is: the gate is shut pending that
// judgment, the state a hold leaves.
func (f *espFormFastPath) firewallCommitted() {
	f.mu.Lock()
	f.epoch++
	f.writeGateLocked(espFormFastPathGateClosed)
	f.mu.Unlock()
}

// firewallMoving records a judgment that ended Moving. The first of a stream of such
// judgments sets the Moving reading, with now, the time that first judgment ended: the
// commits it met began before it, so the ruleset has kept changing since at least
// then. A later one keeps that time. The gate is already shut: each change the judgment saw shut it.
func (f *espFormFastPath) firewallMoving(now time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.rulesetMoving {
		return
	}
	f.rulesetMoving, f.firewallMovingSince = true, now
}

// endMovingLocked ends the stream of commits firewallMoving recorded, because a
// judgment published or a shut recorded its own reading. MUST be called with mu held.
func (f *espFormFastPath) endMovingLocked() {
	f.rulesetMoving, f.firewallMovingSince = false, time.Time{}
}

// firewallWatchEnded shuts the gate: with no watch, a rule added from now on would go
// unseen. It hands the ended watch back for the worker to close without mu, and
// answers whether the watch ended on a failure, which the worker answers by watching
// again. A watch that Close ended leaves the gate shut until stop.
func (f *espFormFastPath) firewallWatchEnded() (ended espFormRulesetWatch, failed bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	ended = f.firewall.watch
	if ended == nil {
		panic("BUG: esp-form: the nftables watch's channel closed while the fast path held no watch")
	}
	f.firewall.watch, f.firewall.events = nil, nil
	f.epoch++
	f.firewallLost = true
	err := ended.Err()
	if err == nil {
		f.shutGateLocked(ESPFormFirewallWatchClosed, espFormFirewallWatchEnded, ESPFormFixRestart)
		return ended, false
	}
	f.shutGateLocked(ESPFormFirewallRewatching, espFormFirewallWatchFailed(err), espFormFirewallRewatchFix)
	return ended, true
}

// firewallRewatchFailed records an attempt to watch the ruleset again that failed.
// The gate is already shut; the cause takes the attempt's failure, logged once for
// each new reason, and the reading stays Rewatching: the worker tries again.
func (f *espFormFastPath) firewallRewatchFailed(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.shutGateLocked(ESPFormFirewallRewatching, espFormFirewallWatchFailed(err), espFormFirewallRewatchFix)
}

// firewallWatched adopts a watch the worker opened again. The epoch steps, so no
// judgment that began while nothing watched publishes, and the ruleset counts as
// watched again. The gate stays shut, with the failure as its cause and the reading
// Rewatching, until the judgment the worker runs next publishes.
func (f *espFormFastPath) firewallWatched(watch espFormRulesetWatch, events <-chan struct{}) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.firewall.watch, f.firewall.events = watch, events
	f.epoch++
	f.firewallLost = false
}

// blockGateLocked writes 0 into the gate because the judgment read the ruleset and
// found a rule or policy at site that can drop ESP. how and fix are what the operator
// changes there (espFormFirewallAccept), kept as the state's FirewallAcceptHow and
// FirewallAcceptFix for the doctor check. A change of site is logged once. An empty
// fix names no command, which only a Ze defect produces (no ESPFormOwnedTableAccept
// registered), and the warning says so. MUST be called with mu held.
func (f *espFormFastPath) blockGateLocked(site, how, fix string) {
	f.writeGateLocked(espFormFastPathGateClosed)
	f.endMovingLocked()
	if f.firewallReading == ESPFormFirewallRead && f.firewallBlocked && f.firewallSite == site && f.firewallCause == "" {
		return
	}
	f.firewallReading = ESPFormFirewallRead
	f.firewallBlocked, f.firewallSite, f.firewallCause = true, site, ""
	f.firewallAcceptHow, f.firewallAcceptFix = how, fix
	if fix == "" {
		how = "Ze named no command for this site, which is a Ze defect"
		fix = ESPFormFixLog
	}
	f.logger().Warn("esp-form: an nftables rule on the input path can drop ESP, so the fast path "+
		"leaves bare ESP as it arrived; that rule sees it, and the complete path re-presents what "+
		"it lets through via "+espFormTUNName+", rate-bounded; only if that ESP should be "+
		"accepted, change that rule as how says",
		"site", site, "datagrams-per-second-max", espFormRate, "how", how,
		"fix", fix, "code", diagnostic.CodeDoctorIPsecESPFormFirewall)
}

// shutGateLocked writes 0 into the gate because the ruleset could not be read or
// watched, and records cause, with the fix the warning names. reading is never
// ESPFormFirewallRead, which blockGateLocked records. A change of cause is logged
// once, with what makes Ze read the ruleset again (espFormFirewallRereadWhen). The
// site is cleared: no rule was read. MUST be called with mu held.
func (f *espFormFastPath) shutGateLocked(reading ESPFormFirewallReading, cause, fix string) {
	f.writeGateLocked(espFormFastPathGateClosed)
	f.endMovingLocked()
	// The site is set only with firewallBlocked (blockGateLocked), so a state that is
	// not blocked holds none, and the site needs no compare of its own.
	if f.firewallReading == reading && !f.firewallBlocked && f.firewallCause == cause {
		return
	}
	f.firewallReading = reading
	f.firewallBlocked, f.firewallSite, f.firewallCause = false, "", cause
	f.firewallAcceptHow, f.firewallAcceptFix = "", ""
	f.logger().Warn("esp-form: the fast path cannot tell whether an nftables rule drops ESP, so it "+
		"leaves bare ESP as it arrived, and the complete path re-presents it via "+espFormTUNName+
		", rate-bounded",
		"cause", cause, "reread", espFormFirewallRereadWhen(reading), "datagrams-per-second-max", espFormRate,
		"fix", fix, "code", diagnostic.CodeDoctorIPsecESPFormFirewall)
}

// clearFirewallLocked records a judgment that read the ruleset and found no rule that
// can drop ESP: the reading becomes Read, the reason the gate was shut is forgotten,
// and when there was one, its end is logged. MUST be called with mu held.
func (f *espFormFastPath) clearFirewallLocked() {
	f.endMovingLocked()
	f.firewallReading = ESPFormFirewallRead
	if !f.firewallBlocked && f.firewallCause == "" {
		return
	}
	f.firewallBlocked, f.firewallSite, f.firewallCause = false, "", ""
	f.firewallAcceptHow, f.firewallAcceptFix = "", ""
	f.logger().Info("esp-form: no nftables rule on the input path can drop ESP now, " +
		"so the fast path rewrites bare ESP again while no firewall commit holds its gate")
}

// writeGateLocked writes value into the gate. While no program is loaded there is no
// gate to write, and a collection loaded later starts shut; once a failed shut
// detached the program (Detached), nothing reads the gate either. MUST be called
// with mu held, so writes reach the map in the order the epoch orders them.
//
// A failed write of 1 leaves the gate shut, which is safe: bare ESP stays on the
// complete path. A failed write of 0 would leave an open gate the fast path cannot
// shut, so the program would rewrite bare ESP past a rule that should see it: the
// fast path detaches from every link instead (detachLocked).
func (f *espFormFastPath) writeGateLocked(value uint32) {
	var err error
	switch {
	case f.status == ESPFormFastPathUnavailable, f.status == ESPFormFastPathDetached:
		// Nothing is attached, so nothing reads a gate and none is open. Under Detached
		// the collection is still loaded until stop, and a write the kernel refused is
		// why it detached, so nothing is written.
		f.gateOpen = false
		return
	case f.writeGate != nil:
		err = f.writeGate(value)
	case f.gate != nil:
		err = f.gate.Update(uint32(espFormFastPathGateKey), value, ebpf.UpdateAny)
	default:
		// No program is loaded, so nothing reads a gate and none is open.
		f.gateOpen = false
		return
	}
	if err == nil {
		f.gateOpen = value != espFormFastPathGateClosed
		return
	}
	if value == espFormFastPathGateClosed {
		f.detachLocked(err)
		return
	}
	f.logger().Warn("esp-form: the fast path could not open its firewall gate, so bare ESP stays on "+
		"the complete path through "+espFormTUNName+", rate-bounded",
		"value", value, "error", err, "fix", ESPFormFixRestart,
		"code", diagnostic.CodeDoctorIPsecESPFormFirewall)
}

// detachLocked answers a gate the fast path could not shut: it detaches the program
// from every link, so no rewrite runs past a rule that should see bare ESP, and
// records Detached with the failure as the cause: not Unavailable, which is a program
// that never loaded and whose fix is a capability or a kernel, while this one loaded and
// attached, and a restart loads it again. The collection stays loaded until stop, which
// closes it; set, remove and writeGateLocked write nothing while Detached, apply
// attaches no link created later, and the complete path, which reads only the
// receiver's registry, serves every watched SPI and follows every retarget. MUST be
// called with mu held.
func (f *espFormFastPath) detachLocked(err error) {
	for ifindex, attached := range f.links {
		if closeErr := attached.tcx.Close(); closeErr != nil {
			f.logger().Warn("esp-form: detach the fast path from a link after its firewall gate could not shut",
				"link", attached.name, "error", closeErr, "code", diagnostic.CodeDoctorIPsecESPFormFastPath)
		}
		delete(f.links, ifindex)
	}
	f.gateOpen = false
	var cause textbuf.Buffer
	cause.Str("the firewall gate could not be shut (").Err(err).
		Str("), so the program was detached from every link rather than rewrite bare ESP an nftables rule should see")
	f.status, f.cause = ESPFormFastPathDetached, cause.String()
	f.logger().Warn("esp-form: the fast path could not shut its firewall gate, so it detached from every "+
		"link, and all bare ESP is re-presented through "+espFormTUNName+" by the complete path, rate-bounded",
		"cause", f.cause, "datagrams-per-second-max", espFormRate, "fix", ESPFormFixRestart,
		"code", diagnostic.CodeDoctorIPsecESPFormFastPath)
}

// hold shuts the gate for the length of a firewall commit. Before it returns it has
// written 0 into the gate, when a program is loaded, and stepped the epoch, so no
// judgment that began before it can open the gate. A hold taken while no program
// is loaded still counts, so a start during it stays shut.
//
// The release it answers MUST be called once, after the commit returned, whether the
// commit succeeded or not; a second call does nothing. The release steps the epoch
// and asks the worker for a judgment that begins after it, which opens the gate
// again when the ruleset is safe and no other hold is open.
//
// hold and the release take mu alone, never life, and never wait on the worker.
// Safe for concurrent use.
func (f *espFormFastPath) hold() (release func()) {
	f.mu.Lock()
	f.holds++
	f.epoch++
	f.writeGateLocked(espFormFastPathGateClosed)
	f.mu.Unlock()

	var once sync.Once
	return func() { once.Do(f.releaseHold) }
}

// releaseHold ends one hold and asks the worker to judge again.
func (f *espFormFastPath) releaseHold() {
	f.mu.Lock()
	f.holds--
	f.epoch++
	f.mu.Unlock()
	select {
	case f.kick <- struct{}{}:
	default:
		// A judgment is already asked for, and the worker reads the epoch only once
		// it takes the ask, after this release.
	}
}
