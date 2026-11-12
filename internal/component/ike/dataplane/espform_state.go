// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- one Child SA receives both ESP forms
// Detail: espform_fastpath_linux.go -- the fast path's attach lifecycle, which fills this state
// Related: espform_program_linux.go -- the rewrite program the fast path attaches
// RFC: rfc/short/rfc7296.md -- receive both ESP forms at any time (Section 2.23)

package dataplane

import (
	"net/netip"
	"strconv"
	"sync/atomic"
	"time"
)

// ESPFormCompletePathRate is the complete path's bound, in datagrams per second, the
// rate espFormRate sets, so a message about the complete path names the bound in force.
const ESPFormCompletePathRate = espFormRate

// ESPFormFastPathStatus is the state of the dual-form receiver's fast path, the
// tc-ingress program that turns bare ESP for a watched SPI into the ESP-in-UDP form
// its inbound state carries.
//
// No status leaves bare ESP unserved, but the complete path is bounded. A link the
// fast path does not serve, every link while the fast path is unavailable, and every
// link while its firewall gate is shut still has its bare ESP served by the complete
// path: the raw ESP reader re-presents it through ze-espform, at most espFormRate
// datagrams per second, and drops every datagram above that rate before it is written
// (espFormLimiter.allow, read by the reader loop in espform_linux.go). Those datagrams
// are lost.
type ESPFormFastPathStatus uint8

const (
	// ESPFormFastPathUnspecified is the zero value: nothing was read.
	ESPFormFastPathUnspecified ESPFormFastPathStatus = iota
	// ESPFormFastPathIdle means no SPI is watched, so nothing is loaded or attached.
	ESPFormFastPathIdle
	// ESPFormFastPathAttached means the program is loaded and attached to the links
	// ESPFormFastPathState.Attached lists, and links created later are attached as
	// they appear.
	ESPFormFastPathAttached
	// ESPFormFastPathUnavailable means the program could not load (no CAP_BPF or
	// CAP_NET_ADMIN, or a kernel without TCX), so the complete path carries all bare
	// ESP. ESPFormFastPathState.Cause says why; the doctor check names the fix.
	ESPFormFastPathUnavailable
	// ESPFormFastPathDetached means the program loaded and attached, then the kernel
	// refused a write that shuts its firewall gate, so the fast path detached it from
	// every link rather than let it rewrite bare ESP past an nftables rule that should
	// see it (detachLocked, espform_fastpath_gate_linux.go). The complete path carries
	// all bare ESP until the fast path starts again. ESPFormFastPathState.Cause names the
	// refused write; the doctor check names the restart.
	ESPFormFastPathDetached
)

// espFormFastPathStatusNames is the one declaration of each status's rendered name.
var espFormFastPathStatusNames = [...]string{
	ESPFormFastPathUnspecified: "unspecified",
	ESPFormFastPathIdle:        "idle",
	ESPFormFastPathAttached:    "attached",
	ESPFormFastPathUnavailable: "unavailable",
	ESPFormFastPathDetached:    "detached",
}

// String answers the rendered name; a value outside the set names its number.
func (s ESPFormFastPathStatus) String() string {
	if int(s) < len(espFormFastPathStatusNames) {
		return espFormFastPathStatusNames[s]
	}
	return "invalid-esp-form-fast-path-status-" + strconv.Itoa(int(s))
}

// ESPFormFirewallReading is how the fast path reads the nftables ruleset its firewall
// gate is judged from (espform_fastpath_gate_linux.go). It says why the gate is shut
// when it is not a rule: whether Ze reads the ruleset again by itself, or only a
// restart makes it read the ruleset again. The doctor check chooses its fix from it.
type ESPFormFirewallReading uint8

const (
	// ESPFormFirewallUnspecified is the zero value: nothing set it, which only a Ze
	// defect produces.
	ESPFormFirewallUnspecified ESPFormFirewallReading = iota
	// ESPFormFirewallUnheld means no program is loaded (Idle, Unavailable), so there is
	// no gate and nothing reads the ruleset.
	ESPFormFirewallUnheld
	// ESPFormFirewallPending means the ruleset is open and watched, and no judgment has
	// published yet. The gate stays shut until the first one, which the worker runs at
	// once after start.
	ESPFormFirewallPending
	// ESPFormFirewallRead means the last judgment read the ruleset. FirewallBlocked says
	// whether a rule on the input path can drop ESP.
	ESPFormFirewallRead
	// ESPFormFirewallUnreadable means the reader or the watch could not open at start,
	// so nothing reads or watches the ruleset. Nothing reads it again until the fast
	// path starts again: a restart of ze (ESPFormFixRestart), or the last Child SA
	// served in both ESP forms removed and one installed again. The gate stays shut.
	// FirewallCause says why.
	ESPFormFirewallUnreadable
	// ESPFormFirewallRewatching means the watch that signals each commit failed, and Ze
	// watches the ruleset again by itself, at most espFormNFTRewatchMax apart, then
	// judges it afresh. No restart is needed. FirewallCause names the last failure.
	ESPFormFirewallRewatching
	// ESPFormFirewallWatchClosed means Close ended the watch while the worker still read
	// it. Nothing watches the ruleset again until the next start, so the gate stays shut.
	ESPFormFirewallWatchClosed
	// ESPFormFirewallDumpFailed means the reader and the watch are open, but the last
	// dump of the ruleset failed. The watch still runs, so the next commit to the
	// ruleset, by Ze's firewall or by any other program, judges it again, and nothing
	// judges it before then. No restart is needed; one reads the ruleset at once.
	// FirewallCause names the dump's failure. A dump message the kernel flagged
	// interrupted (NLM_F_DUMP_INTR) is a commit during the dump, not a failure: it
	// counts as the ruleset moving, never as DumpFailed.
	ESPFormFirewallDumpFailed
	// ESPFormFirewallMoving means the reader and the watch are open, and every judgment
	// since FirewallMovingSince saw the ruleset change on each of its passes: a stream
	// of commits, such as fail2ban adding a set element every few milliseconds. Each
	// change shut the gate, and it stays shut while the stream lasts; the worker judges
	// again after a wait that doubles up to espFormNFTRejudgeMax, and the first
	// judgment that reads the ruleset with nothing moving replaces this reading, as
	// does any shut that records its own reading. No restart is needed. The doctor
	// reports a stream once FirewallMovingSince is ESPFormFirewallMovingReportAfter or
	// longer ago.
	ESPFormFirewallMoving
)

// ESPFormFirewallMovingReportAfter is how long a stream of nftables commits keeps the
// gate shut (ESPFormFirewallMoving), counted from FirewallMovingSince, before the
// doctor check reports it: at this age or older it reports, younger it does not. A
// shorter stream is the ordinary cost of a burst of commits: a firewall reload, a few
// set elements. Ten seconds is ten times espFormNFTRejudgeMax, the longest wait between
// two judgments, so when each judgment's dumps are quick, a stream the doctor reports
// defeated at least ten judgments in a row; slow dumps mean fewer.
const ESPFormFirewallMovingReportAfter = 10 * time.Second

// ESPFormFixMoving shows each commit to the nftables ruleset as it lands, which names
// the table and set a tool keeps changing, so the operator can tell which tool
// commits.
const ESPFormFixMoving = "sudo nft monitor"

// ESPFormFixRestart restarts ze: the fast path starts again, opens the nftables
// reader, joins the nftables monitor and judges the ruleset at once. systemctl needs
// root to restart a system unit. The dataplane's warnings and the doctor check both
// name it, so the two say the same thing.
const ESPFormFixRestart = "sudo systemctl restart ze"

// ESPFormFixListSAs lists every peer with its Child SA's inbound SPI, so an operator
// finds the peers whose Child SAs drew one inbound SPI. It shows no local address, so a
// message that names it says clearing either of those peers ends the collision. The
// dataplane's shared-SPI warnings and the doctor check both name it.
const ESPFormFixListSAs = "ze show vpn ipsec sa"

// ESPFormFixClearPeer, followed by a peer's name, tears down that peer's IKE SA and its
// Child SA and negotiates them again, so the new Child SA draws a new random inbound SPI.
// Ze has no command that rekeys one Child SA alone, so clearing its peer is the command
// that gives it a new SPI. The dataplane's shared-SPI warnings name it in their text and
// the doctor check on its fix line.
const ESPFormFixClearPeer = "ze clear vpn ipsec sa peer "

// ESPFormFixLog lists the dual-form receiver's own warnings, which name the cause the
// doctor check reports.
const ESPFormFixLog = "ze show log recent level warn component ike.dataplane"

// espFormFirewallReadingNames is the one declaration of each reading's rendered name.
var espFormFirewallReadingNames = [...]string{
	ESPFormFirewallUnspecified: "unspecified",
	ESPFormFirewallUnheld:      "unheld",
	ESPFormFirewallPending:     "pending",
	ESPFormFirewallRead:        "read",
	ESPFormFirewallUnreadable:  "unreadable",
	ESPFormFirewallRewatching:  "rewatching",
	ESPFormFirewallWatchClosed: "watch-closed",
	ESPFormFirewallDumpFailed:  "dump-failed",
	ESPFormFirewallMoving:      "moving",
}

// String answers the rendered name; a value outside the set names its number.
func (r ESPFormFirewallReading) String() string {
	if int(r) < len(espFormFirewallReadingNames) {
		return espFormFirewallReadingNames[r]
	}
	return "invalid-esp-form-firewall-reading-" + strconv.Itoa(int(r))
}

// espFormNFTRewatchMax caps the wait between attempts to watch the ruleset again
// (espFormWatchLoop.nextWait, espform_fastpath_gate_linux.go). The gate stays shut
// while nothing watches, so the cap bounds how long bare ESP stays on the complete
// path after the kernel would accept the watch again. One attempt costs a socket, a
// group join and, when it opens, one dump of the ruleset, so one every thirty seconds
// costs a router nothing, and the doubling before it keeps a failure that persists (a
// capability taken away) to a few attempts a minute. It is declared here, beside the
// state, so the doctor check names it on every platform.
const espFormNFTRewatchMax = 30 * time.Second

// ESPFormFirewallRewatchMax is the longest wait between two attempts to watch the
// ruleset again after the watch failed, so a message about a failed watch names the
// bound in force.
const ESPFormFirewallRewatchMax = espFormNFTRewatchMax

// ESPFormLinkRefusal is one link the fast path does not serve. Its bare ESP is
// still served, by the complete path through ze-espform, rate-bounded.
type ESPFormLinkRefusal struct {
	// Link is the interface name, as the kernel reported it when the fast path met
	// the link.
	Link string
	// Reason says why the link has no program: a link type the program has no
	// variant for, or the kernel's refusal of the attach.
	Reason string
}

// ESPFormSharedSPI is one watched inbound SA whose SPI an SA watched before it carries to
// another local address. XFRM keys an inbound state on its destination, SPI and protocol
// (__xfrm_state_lookup), so the kernel installs both, and the complete path serves each.
// The fast path's map holds one target per SPI, the SA watched first
// (espFormRegistry.fastTarget), and its program rewrites only a datagram sent to that
// SA's local address, so the bare ESP of this SA takes the complete path, at most
// ESPFormCompletePathRate datagrams per second. Ze draws each inbound SPI at random with
// no check against the SPIs in use (generateESPSPI, engine/child.go), which is how two
// SAs come to share one; a new SPI for either SA ends it.
type ESPFormSharedSPI struct {
	// SPI is the inbound SPI both SAs carry.
	SPI uint32
	// Served is the local address of the SA the fast path serves for SPI.
	Served netip.Addr
	// Unserved is the local address of this SA, whose bare ESP takes the complete path.
	Unserved netip.Addr
}

// ESPFormFastPathState is what the fast path reports for doctor and tests.
type ESPFormFastPathState struct {
	Status ESPFormFastPathStatus
	// Cause is empty while the fast path works as designed. Under
	// ESPFormFastPathUnavailable it names the load failure alone, and under
	// ESPFormFastPathDetached the gate write the kernel refused, never the fix, which
	// the doctor check writes itself. A subscription that ends after the detach leaves
	// the detach's cause in place. Under
	// ESPFormFastPathAttached it is set only when the link subscription ended: the
	// links listed stay attached, and a link created after that is served by the
	// complete path alone.
	Cause string
	// Attached names the links the program is attached to, sorted. lo and
	// ze-espform never appear: a packet on lo is local, and one on ze-espform is
	// already encapsulated.
	Attached []string
	// Refused lists the links the fast path met and does not serve, sorted by link
	// name.
	Refused []ESPFormLinkRefusal
	// SharedSPIs lists, sorted by SPI and then by Unserved, each watched SA that the
	// fast path does not serve because an SA watched before it carries the same SPI to
	// another local address. It is read from the receiver's registry
	// (espFormRegistry.shared) under every status, so an entry appears with the Watch
	// of the second SA and goes with the Forget of either. Nil when no two watched SAs
	// share an SPI.
	SharedSPIs []ESPFormSharedSPI
	// Rewritten4 and Rewritten6 count the datagrams the program rewrote since it
	// was loaded, IPv4 and IPv6, summed over every CPU. Zero while nothing is
	// loaded.
	Rewritten4, Rewritten6 uint64
	// GateOpen is set while the program's firewall gate holds 1, the last value Ze
	// wrote into it and the kernel accepted, so the program rewrites. It is clear:
	//   - from the load until the first judgment of the ruleset finds it safe;
	//   - during a firewall commit Ze makes, until its release is judged;
	//   - after any change of the ruleset the watch delivers or a judgment sees, by Ze
	//     or by another program, until a judgment that began after it finds it safe;
	//   - while the ruleset keeps moving (FirewallReading Moving);
	//   - while FirewallBlocked or FirewallCause is set;
	//   - once a failed shut detached the program (Status Detached).
	// A test or check that needs the fast path to rewrite waits for it, not for
	// Attached alone.
	GateOpen bool
	// FirewallBlocked is set while the program is attached and its gate is shut
	// because an nftables rule on the input path (an ingress, prerouting or input
	// hook) can drop ESP. A rewritten datagram would pass that rule as UDP, so the
	// program leaves bare ESP as it arrived, and the complete path serves what the
	// rule lets through. FirewallSite names the rule.
	FirewallBlocked bool
	// FirewallSite names the first rule or chain policy that can drop ESP: its
	// family, table, chain, hook, rule and the reason. Empty unless FirewallBlocked.
	FirewallSite string
	// FirewallAcceptFix is the fix for the site FirewallSite names, chosen by who
	// writes its table (FirewallAcceptHow says which): for a table Ze does not own,
	// the nft command that inserts an accept of ESP at the head of the base chain;
	// for a table of Ze's firewall config, the editor lines that add an accept-esp
	// term to that chain and commit it; for a table a Ze plugin writes, the command
	// that shows the state its rules follow. Empty unless FirewallBlocked, and empty
	// beside a blocking site only when no Ze component registered an
	// ESPFormOwnedTableAccept, which only a Ze defect produces.
	FirewallAcceptFix string
	// FirewallAcceptHow says, for the doctor's message, who writes the table of
	// FirewallSite and what FirewallAcceptFix does there. Set and cleared with
	// FirewallAcceptFix.
	FirewallAcceptHow string
	// FirewallCause is set while the gate is shut because the ruleset could not be
	// read, or because the watch that signals each commit ended: the fast path then
	// cannot know whether a rule drops ESP. Empty when the ruleset was read.
	// FirewallReading says which of these it is; the cause is text for the operator
	// alone, never matched.
	FirewallCause string
	// FirewallReading is how the fast path reads the ruleset: Unheld while no program
	// is loaded, Pending until the first judgment, Read once a judgment read it,
	// Unreadable, Rewatching, WatchClosed or DumpFailed while FirewallCause is set, and
	// Moving while a stream of commits keeps every judgment from reading it. Under
	// Moving, FirewallBlocked and the fields that go with it are clear: the stream, not
	// a rule, is why the gate is shut.
	FirewallReading ESPFormFirewallReading
	// FirewallMovingSince is when the first judgment that met the current stream of
	// commits ended (ESPFormFirewallMoving), read from the worker's clock. It is not
	// when the stream began: the commits that judgment met came before it, so the
	// ruleset has kept changing since at least this time. Zero under every other
	// reading.
	FirewallMovingSince time.Time
	// CompletePathFailure is the receiver's last failure to open its complete path, the
	// raw ESP socket and the ze-espform device, read from the receiver under its lock
	// whatever the fast path's status. Nil when the last open succeeded, or when no open
	// failed yet. The fast path does not set it: it is the receiver's record.
	CompletePathFailure *ESPFormCompletePathFailure
}

// ESPFormCompletePathReopenMax is the longest wait between two attempts of the receiver
// to open its complete path again after its reader stopped, so a message about a failed
// open names the bound in force. The receiver declares it for Linux (espFormReopenWaitMax),
// and the doctor builds on every platform, so the value is declared here.
const ESPFormCompletePathReopenMax = 30 * time.Second

// ESPFormCompletePathFailure is one failure to open the complete path. Until the complete
// path opens, the receiver refuses every Child SA that must receive both ESP forms, at
// install and at a MOBIKE move, with this error. The receiver records it when a Watch,
// an acquire or its own reopen after the reader stopped (espFormReceiver.revive) cannot
// open the paths, and clears it on the next open that succeeds. Never changed once built.
type ESPFormCompletePathFailure struct {
	// Err is the open's error. Every failure the host can cause is an operator error
	// that carries its own fix (openESPFormPaths); another error is a Ze defect.
	Err error
	// At is when the open failed, read from the receiver's clock.
	At time.Time
}

// ESPFormOwnedTableAccept answers, for a blocking site in a table whose kernel name
// starts with ze_, what the operator changes to change the rule there: how, a
// sentence for the doctor's message naming who writes the table, and fix, the
// command or editor lines FirewallAcceptFix carries. Ze rewrites such a table, so
// an nft rule inserted there does not last, and who writes it is the firewall
// registry's to say, which the dataplane does not import: the IKE engine registers
// the answer from its register file.
type ESPFormOwnedTableAccept func(family, table, chain string) (how, fix string)

// espFormOwnedTableAccept is the registered ESPFormOwnedTableAccept, nil until one
// registered.
var espFormOwnedTableAccept atomic.Pointer[ESPFormOwnedTableAccept]

// RegisterESPFormOwnedTableAccept records accept as the answer for a blocking site in
// a ze_ table. MUST be called from init(). Safe for concurrent use.
func RegisterESPFormOwnedTableAccept(accept ESPFormOwnedTableAccept) {
	espFormOwnedTableAccept.Store(&accept)
}

// ESPFormReaderStopped is one stop of the receiver's raw ESP reader on a socket that
// failed while Child SAs that must receive both ESP forms were served, which the
// receiver opens again by itself. The stop is transient: once the reader opens again
// nothing is left for a doctor check to read, so the engine raises it on the report bus
// (ai/rules/user-facing-errors.md). Never changed once built.
type ESPFormReaderStopped struct {
	// Err is the read error that stopped the reader.
	Err error
	// At is when the receiver saw the stop, read from the receiver's clock.
	At time.Time
}

// ESPFormReaderStoppedSink takes one reader stop. It is called from the receiver's
// revive worker, with no receiver lock held, once per stop.
type ESPFormReaderStoppedSink func(ESPFormReaderStopped)

// espFormReaderStoppedSink is the registered ESPFormReaderStoppedSink, nil until one
// registered.
var espFormReaderStoppedSink atomic.Pointer[ESPFormReaderStoppedSink]

// RegisterESPFormReaderStopped records sink as the taker of every reader stop, and a nil
// sink removes it. The IKE engine registers it once its plugin exists (runEngine), so the
// stop reaches the daemon's report bus through the plugin's own report-issue request,
// in-process and forked alike: the dataplane runs in the engine's process and holds no
// route to the bus. Safe for concurrent use.
func RegisterESPFormReaderStopped(sink ESPFormReaderStoppedSink) {
	if sink == nil {
		espFormReaderStoppedSink.Store(nil)
		return
	}
	espFormReaderStoppedSink.Store(&sink)
}

// ESPFormFastPathReporter is implemented by a backend whose dual-form receiver has a
// fast path. A backend that does not implement it has no fast path, and its bare ESP
// is served by the complete path alone.
type ESPFormFastPathReporter interface {
	// ESPFormFastPathState reads the fast path's state, or an error when it cannot
	// read the program's counters.
	ESPFormFastPathState() (ESPFormFastPathState, error)
}
