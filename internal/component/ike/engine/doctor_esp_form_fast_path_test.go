package engine

import (
	"errors"
	"net"
	"net/netip"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// espFormFastPathDP answers a fixed fast path state, as a backend with a dual-form
// receiver does.
type espFormFastPathDP struct {
	bypassDP
	state    dataplane.ESPFormFastPathState
	stateErr error
}

func (d *espFormFastPathDP) ESPFormFastPathState() (dataplane.ESPFormFastPathState, error) {
	return d.state, d.stateErr
}

// TestDoctorESPFormFastPath drives the check through each state the fast path reports,
// through the dataplane registry the check reads (dataplane.Get), with a fake backend.
//
// VALIDATES: AC-10: a fast path that could not load is one warning under
// doctor-ipsec-esp-form-fast-path carrying the cause and the capability fix; under
// Attached, each refused link is one warning under doctor-ipsec-esp-form-link naming the
// link, the reason and the peer's own step in its message, whose fix line is the
// command that shows that link, or the interface listing for a name that cannot be
// pasted as one word, and a link subscription that ended is one more fast path
// warning; Idle, a backend that is not a reporter, and ErrNotSupported report nothing;
// a state that cannot be read, or the zero status, is a warning rather than silence, and
// so is no dataplane loaded at all, which in the engine's process means dataplane.Load
// failed. A fast path the firewall keeps off is one warning under
// doctor-ipsec-esp-form-firewall naming the blocking site, the rate bound, the drops
// above it, and the sentence and command the dataplane chose for the site's table; a
// ruleset Ze could not read or follow is the same
// code naming the cause, with the restart, reported in place of a stale site; Idle
// reports neither, and Unavailable reports its own warning alone, whatever the firewall
// reading, because no program reads the gate.
// PREVENTS: a box whose bare ESP runs rate-bounded through ze-espform with doctor
// silent about it, and a refused link reported as the whole fast path failing.
func TestDoctorESPFormFastPath(t *testing.T) {
	fastPath := "warning:" + diagnostic.CodeDoctorIPsecESPFormFastPath
	link := "warning:" + diagnostic.CodeDoctorIPsecESPFormLink
	unavailableCause := "load the ESP form rewrite program: operation not permitted; give ze the " +
		"CAP_BPF and CAP_NET_ADMIN capabilities, or run it on a kernel with TCX (Linux 6.6 or later)"
	bound := "at most " + strconv.Itoa(dataplane.ESPFormCompletePathRate) + " datagrams per second"
	refusedPPP := dataplane.ESPFormLinkRefusal{Link: "ppp0", Reason: "its link type ppp has no program variant"}
	refusedWG := dataplane.ESPFormLinkRefusal{Link: "wg0", Reason: "the kernel refused the attach: invalid argument"}
	// refusedOdd carries a name the kernel accepts (dev_valid_name refuses only '/', ':'
	// and white space) and that cannot be pasted as one CLI word.
	refusedOdd := dataplane.ESPFormLinkRefusal{Link: `ppp"1|x`, Reason: "its link type ppp has no program variant"}
	// peerStep is the peer's own step the link finding names in its message, never as
	// its fix line: the fix line is a command run on this box.
	peerStep := []string{"set " + espFormFastPathPeerEncap + " in the connection", "run " + espFormFastPathPeerLoad + " there"}
	firewallCode := "warning:" + diagnostic.CodeDoctorIPsecESPFormFirewall
	firewallSite := "inet ze_wan input (hook input) rule 7 \"block-all\": drop"
	dropped := "drops every datagram above that rate before XFRM sees it"
	// acceptFix is the command the dataplane chose for the site, and acceptHow the
	// sentence it chose beside it; the finding carries the fix as its fix line and the
	// sentence in its message, as they are.
	acceptFix := `sudo nft 'insert rule inet "filter" "input" meta l4proto esp accept'`
	acceptHow := "Table filter is not Ze's, so the command below inserts the accept at the head of its chain input."

	for _, tc := range []struct {
		name string
		// dp is the loaded backend; nil leaves none loaded, as a failed dataplane.Load does.
		dp   dataplane.Dataplane
		want string
		// contains holds, per finding, the texts its message must carry.
		contains [][]string
		fix      []string
	}{
		{
			name:     "the dataplane did not load",
			want:     fastPath,
			contains: [][]string{{"could not read", "the IPsec dataplane did not load"}},
			fix:      []string{espFormFastPathFixEngineLog},
		},
		{name: "not a reporter", dp: &bypassDP{}},
		{name: "not supported", dp: &espFormFastPathDP{stateErr: dataplane.ErrNotSupported}},
		{name: "idle", dp: &espFormFastPathDP{state: dataplane.ESPFormFastPathState{Status: dataplane.ESPFormFastPathIdle}}},
		{
			name: "attached, every link served",
			dp: &espFormFastPathDP{state: dataplane.ESPFormFastPathState{
				Status: dataplane.ESPFormFastPathAttached, FirewallReading: dataplane.ESPFormFirewallRead, Attached: []string{"eth0", "eth1"}, Rewritten4: 7,
			}},
		},
		{
			name: "unavailable",
			dp: &espFormFastPathDP{state: dataplane.ESPFormFastPathState{
				Status: dataplane.ESPFormFastPathUnavailable, FirewallReading: dataplane.ESPFormFirewallUnheld, Cause: unavailableCause,
			}},
			want:     fastPath,
			contains: [][]string{{"fast path is unavailable (" + unavailableCause + ")", "ze-espform", bound, "CAP_BPF", "sudo systemctl stop ze"}},
			fix:      []string{espFormFastPathFixCapabilities},
		},
		{
			name: "attached, two links refused",
			dp: &espFormFastPathDP{state: dataplane.ESPFormFastPathState{
				Status: dataplane.ESPFormFastPathAttached, FirewallReading: dataplane.ESPFormFirewallRead, Attached: []string{"eth0"},
				Refused: []dataplane.ESPFormLinkRefusal{refusedPPP, refusedWG},
			}},
			want: link + "," + link,
			contains: [][]string{
				append([]string{"does not serve link ppp0 (" + refusedPPP.Reason + ")", "Ze's own receiver for bare ESP", bound, "ze-espform, not ppp0",
					"shows the link's type and state"}, peerStep...),
				append([]string{"does not serve link wg0 (" + refusedWG.Reason + ")", "Ze's own receiver for bare ESP", bound, "ze-espform, not wg0",
					"shows the link's type and state"}, peerStep...),
			},
			fix: []string{"ze show interface name ppp0 detail", "ze show interface name wg0 detail"},
		},
		{
			name: "attached, a refused link whose name cannot be pasted",
			dp: &espFormFastPathDP{state: dataplane.ESPFormFastPathState{
				Status: dataplane.ESPFormFastPathAttached, FirewallReading: dataplane.ESPFormFirewallRead, Attached: []string{"eth0"},
				Refused: []dataplane.ESPFormLinkRefusal{refusedOdd},
			}},
			want: link,
			contains: [][]string{
				append([]string{"does not serve link " + refusedOdd.Link, "cannot be typed as one command word",
					"lists every interface"}, peerStep...),
			},
			fix: []string{espFormFastPathFixLinks},
		},
		{
			name: "attached, the link subscription ended",
			dp: &espFormFastPathDP{state: dataplane.ESPFormFastPathState{
				Status: dataplane.ESPFormFastPathAttached, FirewallReading: dataplane.ESPFormFirewallRead, Attached: []string{"eth0", "eth1"},
				Cause: "the link subscription ended", Refused: []dataplane.ESPFormLinkRefusal{refusedPPP},
			}},
			want: fastPath + "," + link,
			contains: [][]string{
				{"stopped following new links (the link subscription ended)", "on the 2 links it is attached to"},
				{"does not serve link ppp0"},
			},
			fix: []string{espFormFastPathFixRestart, "ze show interface name ppp0 detail"},
		},
		{
			name: "attached, the firewall could drop ESP",
			dp: &espFormFastPathDP{state: dataplane.ESPFormFastPathState{
				Status: dataplane.ESPFormFastPathAttached, FirewallReading: dataplane.ESPFormFirewallRead, Attached: []string{"eth0"},
				FirewallBlocked: true, FirewallSite: firewallSite, FirewallAcceptFix: acceptFix,
				FirewallAcceptHow: acceptHow,
			}},
			want: firewallCode,
			contains: [][]string{{
				"off because a firewall rule or policy on the input path could drop ESP",
				"The first site that could drop it: " + firewallSite,
				"the firewall still decides on it", bound, dropped, "ze-espform, not the link it arrived on",
				"add a first rule accepting protocol esp in that chain", "accept the rate bound",
				acceptHow,
			}},
			fix: []string{acceptFix},
		},
		{
			name: "attached, Ze stopped following nftables changes",
			dp: &espFormFastPathDP{state: dataplane.ESPFormFastPathState{
				Status: dataplane.ESPFormFastPathAttached, FirewallReading: dataplane.ESPFormFirewallWatchClosed, Attached: []string{"eth0"},
				FirewallCause: "Ze stopped following nftables changes",
			}},
			want: firewallCode,
			contains: [][]string{{
				"off for safety (Ze stopped following nftables changes)",
				"the firewall still decides on it", bound, dropped,
			}},
			fix: []string{espFormFastPathFixRestart},
		},
		{
			// A cause means the ruleset was not read, so a blocking site beside it is
			// stale: the cause alone is reported, once.
			name: "attached, the ruleset could not be read beside a stale site",
			dp: &espFormFastPathDP{state: dataplane.ESPFormFastPathState{
				Status: dataplane.ESPFormFastPathAttached, FirewallReading: dataplane.ESPFormFirewallUnreadable, Attached: []string{"eth0"},
				FirewallBlocked: true, FirewallSite: firewallSite,
				FirewallCause: "the nftables ruleset could not be read: netlink receive: no buffer space available",
			}},
			want: firewallCode,
			contains: [][]string{{
				"off for safety (the nftables ruleset could not be read: netlink receive: no buffer space available)",
			}},
			fix: []string{espFormFastPathFixRestart},
		},
		{
			// A fast path that did not load reads no gate, so the blocking site the last
			// judgment found decides nothing: the Unavailable warning is the only finding.
			// MUTATION: append espFormFirewallFindings(state) to the Unavailable finding
			// in espFormFastPathFindings: the blocking site is a second finding.
			name: "unavailable, and the firewall could drop ESP",
			dp: &espFormFastPathDP{state: dataplane.ESPFormFastPathState{
				Status: dataplane.ESPFormFastPathUnavailable, FirewallReading: dataplane.ESPFormFirewallRead, Cause: unavailableCause,
				FirewallBlocked: true, FirewallSite: firewallSite, FirewallAcceptFix: acceptFix,
				FirewallAcceptHow: acceptHow,
			}},
			want:     fastPath,
			contains: [][]string{{"fast path is unavailable"}},
			fix:      []string{espFormFastPathFixCapabilities},
		},
		{
			// Idle watches no SPI, so no bare ESP takes either path, and a firewall
			// verdict the state still carries is no finding.
			name: "idle, with a firewall verdict",
			dp: &espFormFastPathDP{state: dataplane.ESPFormFastPathState{
				Status: dataplane.ESPFormFastPathIdle, FirewallBlocked: true, FirewallSite: firewallSite,
			}},
		},
		{
			name:     "the state cannot be read",
			dp:       &espFormFastPathDP{stateErr: errors.New("read the rewrite counter: bad file descriptor")},
			want:     fastPath,
			contains: [][]string{{"could not read", "bad file descriptor"}},
			fix:      []string{espFormFastPathFixLog},
		},
		{
			name:     "the zero status",
			dp:       &espFormFastPathDP{},
			want:     fastPath,
			contains: [][]string{{"could not read", "the status unspecified"}},
			fix:      []string{espFormFastPathFixLog},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			loadESPFormBackend(t, tc.dp)
			// The last apply recorded the device opening, so only the fast path's
			// findings appear (TestDoctorESPFormTUN owns the device's).
			diags := checkIPsecESPFormFastPath(newESPFormTUNRecord(nil))
			if got := diagnosticCodes(diags); got != tc.want {
				t.Fatalf("diagnostics = %q, want %q (%v)", got, tc.want, diags)
			}
			for i, d := range diags {
				assertESPFormFinding(t, d, tc.contains[i], tc.fix[i])
			}
		})
	}
}

// TestDoctorESPFormFirewallReading drives the firewall finding through each reading the
// fast path reports (dataplane.ESPFormFastPathState.FirewallReading), with the same
// cause text for every reading that carries one, so only the reading can choose the fix.
//
// VALIDATES: a watch that failed and that Ze opens again by itself (Rewatching) names
// no restart: its message says no restart is needed and that Ze tries at most 30
// seconds apart, and its fix is the dataplane's warning log, which names why the watch
// keeps failing; a reader or watch that could not open at start (Unreadable) and a
// watch that Close ended (WatchClosed) keep the restart, and say nothing reads the
// ruleset again until then; a dump that failed under a live watch (DumpFailed) says no
// restart is needed, that the next commit reads the ruleset again and that nothing
// reads it before, and names the restart as the command that reads it at once; a read
// ruleset reports its blocking site with the sentence and the command the dataplane
// chose for the site as its fix, a Ze defect with the log command when it chose none,
// or nothing; a stream of commits (Moving) whose FirewallMovingSince is
// dataplane.ESPFormFirewallMovingReportAfter or longer ago names the time it has kept
// changing since at least, that the fast path comes back only once a read finds no rule
// that can drop ESP, the rate bound and the two steps, with the command that shows each
// commit as its fix, while a younger stream reports nothing, one with no start time
// names a Ze defect, and one under Unavailable gives the Unavailable warning alone; no program loaded
// (Unheld) and the first judgment not yet
// published (Pending) report nothing; the zero reading and a value outside the set are
// a warning naming a Ze defect, never silence and never a guessed fix; an empty cause
// is named as missing rather than left blank.
// PREVENTS: the doctor telling the operator to restart ze while Ze is already watching
// the ruleset again by itself, and a fix chosen by matching the cause's text.
func TestDoctorESPFormFirewallReading(t *testing.T) {
	fastPath := "warning:" + diagnostic.CodeDoctorIPsecESPFormFastPath
	firewall := "warning:" + diagnostic.CodeDoctorIPsecESPFormFirewall
	cause := "the nftables watch failed (recvmsg: no buffer space available)"
	site := "inet ze_wan input (hook input) rule 7 \"block-all\": drop"
	restart := "Restart ze with the command below"
	// noRestart is scoped to the firewall reading, because under Attached it can sit
	// beside the subscription-ended finding, which does name a restart.
	noRestart := "For the firewall reading, no restart is needed"
	offForSafety := "off for safety (" + cause + ")"
	// sourceGuardHow is a sentence the dataplane chose for a site in a table a Ze
	// producer writes, which the finding carries as it is beside that producer's command.
	sourceGuardHow := "Table ze_ipsec_source_guard is written by Ze's IKE engine's tunnel-source guard."
	// now is the doctor's clock for every row, so a stream of commits has a known age.
	now := time.Date(2026, time.November, 12, 10, 0, 0, 0, time.UTC)
	before := espFormFirewallNow
	espFormFirewallNow = func() time.Time { return now }
	t.Cleanup(func() { espFormFirewallNow = before })
	oldStream := now.Add(-dataplane.ESPFormFirewallMovingReportAfter - time.Second)
	thresholdStream := now.Add(-dataplane.ESPFormFirewallMovingReportAfter)
	youngStream := now.Add(-dataplane.ESPFormFirewallMovingReportAfter + time.Second)

	for _, tc := range []struct {
		name  string
		state dataplane.ESPFormFastPathState
		want  string
		has   []string
		lacks []string
		fix   string
	}{
		{
			name: "rewatching",
			state: dataplane.ESPFormFastPathState{
				Status: dataplane.ESPFormFastPathAttached, Attached: []string{"eth0"},
				FirewallReading: dataplane.ESPFormFirewallRewatching, FirewallCause: cause,
			},
			want: firewall,
			has: []string{
				offForSafety, noRestart, "Ze watches the ruleset again by itself",
				"at most 30 seconds apart", "the command below lists why the watch keeps failing",
			},
			lacks: []string{restart, "systemctl"},
			fix:   espFormFastPathFixLog,
		},
		{
			name: "rewatching, no cause given",
			state: dataplane.ESPFormFastPathState{
				Status: dataplane.ESPFormFastPathAttached, Attached: []string{"eth0"},
				FirewallReading: dataplane.ESPFormFirewallRewatching,
			},
			want:  firewall,
			has:   []string{"off for safety (the dataplane gave no cause)", noRestart},
			lacks: []string{restart},
			fix:   espFormFastPathFixLog,
		},
		{
			name: "unreadable",
			state: dataplane.ESPFormFastPathState{
				Status: dataplane.ESPFormFastPathAttached, Attached: []string{"eth0"},
				FirewallReading: dataplane.ESPFormFirewallUnreadable, FirewallCause: cause,
			},
			want:  firewall,
			has:   []string{offForSafety, restart, "Nothing reads the ruleset again until ze restarts"},
			lacks: []string{noRestart},
			fix:   espFormFastPathFixRestart,
		},
		{
			name: "watch closed",
			state: dataplane.ESPFormFastPathState{
				Status: dataplane.ESPFormFastPathAttached, Attached: []string{"eth0"},
				FirewallReading: dataplane.ESPFormFirewallWatchClosed, FirewallCause: cause,
			},
			want:  firewall,
			has:   []string{offForSafety, restart, "Nothing reads the ruleset again until ze restarts"},
			lacks: []string{noRestart},
			fix:   espFormFastPathFixRestart,
		},
		{
			name: "read, a rule can drop ESP",
			state: dataplane.ESPFormFastPathState{
				Status: dataplane.ESPFormFastPathAttached, Attached: []string{"eth0"},
				FirewallReading: dataplane.ESPFormFirewallRead, FirewallBlocked: true, FirewallSite: site,
				FirewallAcceptFix: "show firewall ruleset ipsec_source_guard",
				FirewallAcceptHow: sourceGuardHow,
			},
			want:  firewall,
			has:   []string{"The first site that could drop it: " + site, sourceGuardHow},
			lacks: []string{restart, noRestart, "Ze defect"},
			fix:   "show firewall ruleset ipsec_source_guard",
		},
		{
			// A blocking site with no command is a Ze defect: the fix lists the
			// dataplane's warnings rather than a guessed command.
			name: "read, a rule can drop ESP, no command named",
			state: dataplane.ESPFormFastPathState{
				Status: dataplane.ESPFormFastPathAttached, Attached: []string{"eth0"},
				FirewallReading: dataplane.ESPFormFirewallRead, FirewallBlocked: true, FirewallSite: site,
			},
			want: firewall,
			has:  []string{"The first site that could drop it: " + site, "named no command for this site, which is a Ze defect"},
			fix:  espFormFastPathFixLog,
		},
		{
			// The watch still runs, so the next commit reads the ruleset again: no
			// restart is needed, and the restart is the command that reads it at once.
			name: "dump failed",
			state: dataplane.ESPFormFastPathState{
				Status: dataplane.ESPFormFastPathAttached, Attached: []string{"eth0"},
				FirewallReading: dataplane.ESPFormFirewallDumpFailed, FirewallCause: cause,
			},
			want: firewall,
			has: []string{
				offForSafety, noRestart, "the next commit to it, by Ze's firewall or by any other program, reads it again",
				"Nothing reads the ruleset before that commit", "To read it at once, restart ze with the command below",
			},
			lacks: []string{restart, "Nothing reads the ruleset again until ze restarts"},
			fix:   espFormFastPathFixRestart,
		},
		{
			// A stream of commits older than the report threshold is one warning naming
			// the time the ruleset has kept changing since at least, the condition the
			// fast path comes back on, the rate bound, and the two steps; its fix shows
			// each commit.
			// MUTATION: drop the ESPFormFirewallMoving case from espFormFirewallFindings:
			// the reading falls to the unknown-reading warning, whose message names a Ze
			// defect and whose fix is the log listing, not sudo nft monitor.
			// MUTATION: drop ", and that read finds no rule that can drop ESP" from the
			// message: it promises the fast path back once the commits pause, which a
			// stream that ends on a blocking rule does not keep.
			name: "moving, a stream older than the report threshold",
			state: dataplane.ESPFormFastPathState{
				Status: dataplane.ESPFormFastPathAttached, Attached: []string{"eth0"},
				FirewallReading: dataplane.ESPFormFirewallMoving, FirewallMovingSince: oldStream,
			},
			want: firewall,
			has: []string{
				"The nftables ruleset has kept changing since at least " + oldStream.Format(time.RFC3339) + ", 11 seconds ago",
				"the IPsec ESP form fast path stays off",
				"at most " + strconv.Itoa(dataplane.ESPFormCompletePathRate) + " datagrams per second",
				noRestart,
				"once the commits pause long enough for one read, and that read finds no rule that can drop ESP",
				"see which tool keeps committing",
				"make that tool batch its changes, or stop it", "fail2ban",
			},
			lacks: []string{restart, "Ze defect"},
			fix:   "sudo nft monitor",
		},
		{
			// A stream exactly at the report threshold reports: the threshold is the
			// first age the doctor reports, not the last it stays silent at.
			// MUTATION: make the age test in espFormFirewallMoving `<=` rather than `<`:
			// this row then reports nothing.
			name: "moving, a stream exactly at the report threshold",
			state: dataplane.ESPFormFastPathState{
				Status: dataplane.ESPFormFastPathAttached, Attached: []string{"eth0"},
				FirewallReading: dataplane.ESPFormFirewallMoving, FirewallMovingSince: thresholdStream,
			},
			want: firewall,
			has: []string{
				"The nftables ruleset has kept changing since at least " + thresholdStream.Format(time.RFC3339) + ", 10 seconds ago",
			},
			lacks: []string{"Ze defect"},
			fix:   "sudo nft monitor",
		},
		{
			// A fast path that did not load reads no gate, so a stream of commits decides
			// nothing, and the Moving warning's "no restart is needed" would sit beside the
			// Unavailable warning's unit install: the Unavailable warning is the only
			// finding. TestDoctorESPFormFastPathDetached proves the same of Detached.
			// MUTATION: append espFormFirewallFindings(state) to the Unavailable finding
			// in espFormFastPathFindings: the Moving warning is a second finding.
			name: "unavailable, with a stream older than the report threshold",
			state: dataplane.ESPFormFastPathState{
				Status: dataplane.ESPFormFastPathUnavailable, Cause: "no CAP_BPF",
				FirewallReading: dataplane.ESPFormFirewallMoving, FirewallMovingSince: oldStream,
			},
			want: fastPath,
		},
		{
			// A stream younger than the threshold is a burst the fast path outlasts.
			// MUTATION: drop the age test from espFormFirewallMoving, so every stream
			// reports: this row then reports a warning.
			name: "moving, a stream younger than the report threshold",
			state: dataplane.ESPFormFastPathState{
				Status: dataplane.ESPFormFastPathAttached, Attached: []string{"eth0"},
				FirewallReading: dataplane.ESPFormFirewallMoving, FirewallMovingSince: youngStream,
			},
		},
		{
			// A Moving reading with no start time is a Ze defect, never a date in year 1.
			// MUTATION: drop the since.IsZero test from espFormFirewallMoving: the
			// message names 0001-01-01T00:00:00Z as the start and no Ze defect.
			name: "moving, no start recorded",
			state: dataplane.ESPFormFastPathState{
				Status: dataplane.ESPFormFastPathAttached, Attached: []string{"eth0"},
				FirewallReading: dataplane.ESPFormFirewallMoving,
			},
			want:  firewall,
			has:   []string{"since a time the dataplane did not record (a Ze defect)"},
			lacks: []string{"0001-01-01"},
			fix:   "sudo nft monitor",
		},
		{
			name: "read, no rule drops ESP",
			state: dataplane.ESPFormFastPathState{
				Status: dataplane.ESPFormFastPathAttached, Attached: []string{"eth0"},
				FirewallReading: dataplane.ESPFormFirewallRead, GateOpen: true,
			},
		},
		{
			name: "pending",
			state: dataplane.ESPFormFastPathState{
				Status: dataplane.ESPFormFastPathAttached, Attached: []string{"eth0"},
				FirewallReading: dataplane.ESPFormFirewallPending,
			},
		},
		{
			// Unavailable loads no program, so the only finding is the fast path's own.
			name: "unheld",
			state: dataplane.ESPFormFastPathState{
				Status: dataplane.ESPFormFastPathUnavailable, Cause: "no CAP_BPF",
				FirewallReading: dataplane.ESPFormFirewallUnheld,
			},
			want: fastPath,
		},
		{
			name: "unspecified",
			state: dataplane.ESPFormFastPathState{
				Status: dataplane.ESPFormFastPathAttached, Attached: []string{"eth0"},
			},
			want:  firewall,
			has:   []string{"firewall reading unspecified", "Ze defect"},
			lacks: []string{restart},
			fix:   espFormFastPathFixLog,
		},
		{
			name: "a reading outside the set",
			state: dataplane.ESPFormFastPathState{
				Status: dataplane.ESPFormFastPathAttached, Attached: []string{"eth0"},
				FirewallReading: dataplane.ESPFormFirewallReading(200),
			},
			want:  firewall,
			has:   []string{"firewall reading invalid-esp-form-firewall-reading-200", "Ze defect"},
			lacks: []string{restart},
			fix:   espFormFastPathFixLog,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			loadESPFormBackend(t, &espFormFastPathDP{state: tc.state})
			diags := checkIPsecESPFormFastPath(newESPFormTUNRecord(nil))
			if got := diagnosticCodes(diags); got != tc.want {
				t.Fatalf("diagnostics = %q, want %q (%v)", got, tc.want, diags)
			}
			if tc.has == nil {
				return
			}
			// The firewall finding is the only one wherever a text is asked for.
			d := diags[0]
			assertESPFormFinding(t, d, tc.has, tc.fix)
			for _, text := range tc.lacks {
				if strings.Contains(d.Message, text) {
					t.Errorf("%s message carries %q, which this reading must not name: %q", d.Code, text, d.Message)
				}
			}
		})
	}
}

// TestDoctorESPFormFastPathDetached tells the fast path's two ways of carrying no
// program apart: one that detached after a refused gate shut (Detached), and one that
// never loaded (Unavailable).
//
// VALIDATES: Detached is one warning under doctor-ipsec-esp-form-fast-path that says
// the firewall gate could not be shut, so the program was detached from every link and
// bare ESP takes the complete path at its rate bound, names the refused write, and
// gives the restart the dataplane's warning names (dataplane.ESPFormFixRestart) as its
// fix, with no capability or kernel step; a firewall reading beside it (a blocking
// site, a stream of commits) is no finding, because no program reads the gate; an
// empty cause is named as missing. A load failure (Unavailable) still gives the
// CAP_BPF, unit and kernel message with the capability install as its fix, and never
// says the program was detached.
// PREVENTS: an operator told to reinstall the unit with CAP_BPF, or to upgrade the
// kernel, for a program that had loaded and attached.
// MUTATION: drop the Detached case from espFormFastPathFindings: the status falls to
// the could-not-read warning with the log listing as its fix.
// MUTATION: answer espFormFastPathUnavailable for Detached in espFormFastPathFindings:
// the message names CAP_BPF and the fix is the capability install.
// MUTATION: append espFormFirewallFindings(state) to the Detached finding: the blocking
// site and the stream are second findings.
func TestDoctorESPFormFastPathDetached(t *testing.T) {
	fastPath := "warning:" + diagnostic.CodeDoctorIPsecESPFormFastPath
	detachCause := "the firewall gate could not be shut (map update: bad file descriptor), so the program " +
		"was detached from every link rather than rewrite bare ESP an nftables rule should see"
	loadCause := "load the rewrite program: operation not permitted"
	bound := "at most " + strconv.Itoa(dataplane.ESPFormCompletePathRate) + " datagrams per second"
	site := "inet ze_wan input (hook input) rule 7 \"block-all\": drop"
	now := time.Date(2026, time.November, 12, 10, 0, 0, 0, time.UTC)
	before := espFormFirewallNow
	espFormFirewallNow = func() time.Time { return now }
	t.Cleanup(func() { espFormFirewallNow = before })
	detachedText := []string{
		"detached itself from every link (" + detachCause + ")",
		"Ze could not shut its firewall gate", "removed the program",
		"goes through Ze's own receiver for bare ESP", bound, "Restart ze with the command below to load it again",
	}
	for _, tc := range []struct {
		name  string
		state dataplane.ESPFormFastPathState
		has   []string
		lacks []string
		fix   string
	}{
		{
			name: "detached",
			state: dataplane.ESPFormFastPathState{
				Status: dataplane.ESPFormFastPathDetached, Cause: detachCause,
				FirewallReading: dataplane.ESPFormFirewallRead,
			},
			has:   detachedText,
			lacks: []string{"CAP_BPF", "systemctl stop", "Linux 6.6", "unavailable"},
			fix:   dataplane.ESPFormFixRestart,
		},
		{
			name: "detached, beside a blocking site",
			state: dataplane.ESPFormFastPathState{
				Status: dataplane.ESPFormFastPathDetached, Cause: detachCause,
				FirewallReading: dataplane.ESPFormFirewallRead, FirewallBlocked: true, FirewallSite: site,
				FirewallAcceptFix: "show firewall ruleset wan",
			},
			has:   detachedText,
			lacks: []string{site},
			fix:   dataplane.ESPFormFixRestart,
		},
		{
			name: "detached, beside a stream of commits",
			state: dataplane.ESPFormFastPathState{
				Status: dataplane.ESPFormFastPathDetached, Cause: detachCause,
				FirewallReading:     dataplane.ESPFormFirewallMoving,
				FirewallMovingSince: now.Add(-dataplane.ESPFormFirewallMovingReportAfter - time.Second),
			},
			has:   detachedText,
			lacks: []string{"kept changing", "no restart is needed"},
			fix:   dataplane.ESPFormFixRestart,
		},
		{
			name:  "detached, no cause given",
			state: dataplane.ESPFormFastPathState{Status: dataplane.ESPFormFastPathDetached},
			has:   []string{"detached itself from every link (the dataplane gave no cause)"},
			fix:   dataplane.ESPFormFixRestart,
		},
		{
			name: "a load failure is still unavailable",
			state: dataplane.ESPFormFastPathState{
				Status: dataplane.ESPFormFastPathUnavailable, Cause: loadCause,
				FirewallReading: dataplane.ESPFormFirewallUnheld,
			},
			has:   []string{"fast path is unavailable (" + loadCause + ")", "CAP_BPF", "sudo systemctl stop ze", "Linux 6.6"},
			lacks: []string{"detached", "firewall gate"},
			fix:   espFormFastPathFixCapabilities,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			loadESPFormBackend(t, &espFormFastPathDP{state: tc.state})
			diags := checkIPsecESPFormFastPath(newESPFormTUNRecord(nil))
			if got := diagnosticCodes(diags); got != fastPath {
				t.Fatalf("diagnostics = %q, want %q (%v)", got, fastPath, diags)
			}
			d := diags[0]
			assertESPFormFinding(t, d, tc.has, tc.fix)
			for _, text := range tc.lacks {
				if strings.Contains(d.Message, text) {
					t.Errorf("%s message carries %q, which this status must not name: %q", d.Code, text, d.Message)
				}
			}
		})
	}
}

// TestDoctorESPFormSharedSPI drives the check through the collision of two Child SAs on
// two local addresses that drew one inbound SPI (dataplane.ESPFormSharedSPI), with a
// fake backend answering the state and the engine's active sessions naming the peers.
//
// VALIDATES: under Attached, each shared SPI is one warning under
// doctor-ipsec-esp-form-fast-path naming the SPI, both local addresses, the random draw,
// the complete path's bound from dataplane.ESPFormCompletePathRate and the drops above
// it; its fix is ze clear vpn ipsec sa peer <name> for the peer whose current or pending
// Child SA carries the SPI to the unserved address, matched on the address a net.IP holds
// in its 16-byte form too; a peer Ze cannot find, or whose name cannot be typed as one
// word, gets the peer listing as its fix, and its message says that clearing either peer
// whose Child SA carries the SPI ends the collision, naming no local address the listing
// does not show; no shared SPI reports nothing; Unavailable reports its own warning
// alone, because every SA's bare ESP takes the complete path there.
// PREVENTS: a collision that leaves one SA's bare ESP rate-bounded with doctor silent,
// a fix line that clears the peer whose SA the fast path serves, and a fallback that
// tells the operator to find the peer by a local address the listing never prints.
// MUTATION: drop the SharedSPIs loop from espFormFastPathAttached: the collision cases
// report nothing.
// MUTATION: drop the local address test from espFormChildCarries: in "the unserved SA's
// peer not found", peer west, whose SA on the served address carries the SPI, answers,
// so the fix clears west.
// MUTATION: compare addr rather than addr.Unmap() in espFormChildCarries: the 16-byte
// net.IP never matches, so "peer named" gets the listing as its fix.
// MUTATION: drop the ps.pendingChild test from espFormSharedSPIPeer: "peer named by its
// pending Child SA" finds no peer and gets the listing as its fix.
// MUTATION: drop the espFormOneWord test from espFormSharedSPI: "peer name not one word"
// gets a fix line carrying the quote.
func TestDoctorESPFormSharedSPI(t *testing.T) {
	const spi = 0x1234
	fastPath := "warning:" + diagnostic.CodeDoctorIPsecESPFormFastPath
	served, unserved := netip.MustParseAddr("203.0.113.9"), netip.MustParseAddr("203.0.113.10")
	shared := []dataplane.ESPFormSharedSPI{{SPI: spi, Served: served, Unserved: unserved}}
	attached := func(sharedSPIs []dataplane.ESPFormSharedSPI) dataplane.ESPFormFastPathState {
		return dataplane.ESPFormFastPathState{
			Status: dataplane.ESPFormFastPathAttached, Attached: []string{"eth0"}, GateOpen: true,
			FirewallReading: dataplane.ESPFormFirewallRead, SharedSPIs: sharedSPIs,
		}
	}
	// session builds a peer session whose current Child SA carries spi to local, its
	// address in the 16-byte form net.ParseIP answers.
	session := func(name, local string) *PeerSession {
		return &PeerSession{peerName: name, childSA: &ChildSA{InboundSPI: spi, LocalAddr: net.ParseIP(local)}}
	}
	bound := "at most " + strconv.Itoa(dataplane.ESPFormCompletePathRate) + " datagrams per second"
	collision := []string{
		"Two Child SAs on different local addresses drew the same random inbound SPI 4660, " +
			"one on 203.0.113.9 and one on 203.0.113.10",
		"fast path serves one SA per SPI, the one on 203.0.113.9",
		"bare ESP for the SA on 203.0.113.10 goes through Ze's own receiver for bare ESP",
		bound, "drops every datagram above that rate before XFRM sees it",
	}
	// The listing shows no local address, so the message names no address to find the
	// peer by: clearing either peer ends the collision.
	listing := []string{
		"The command below lists each peer with its inbound SPI, but not its local address",
		"Run " + espFormSharedSPIFixPeer + "<name> for either peer whose Child SA carries SPI 4660: " +
			"clearing either one ends the collision",
	}
	// pending builds a peer session whose current Child SA carries another SPI and whose
	// pending Child SA, the one a parallel handshake installed, carries spi to local.
	pending := func(name, local string) *PeerSession {
		return &PeerSession{
			peerName:     name,
			childSA:      &ChildSA{InboundSPI: spi + 1, LocalAddr: net.ParseIP(local)},
			pendingChild: &ChildSA{InboundSPI: spi, LocalAddr: net.ParseIP(local)},
		}
	}

	for _, tc := range []struct {
		name  string
		state dataplane.ESPFormFastPathState
		peers map[string]*PeerSession
		// want is the diagnostics' severities and codes, in order.
		want  string
		has   []string
		lacks []string
		fix   string
	}{
		{
			name:  "peer named",
			state: attached(shared),
			peers: map[string]*PeerSession{"east": session("east", "203.0.113.10"), "west": session("west", "203.0.113.9")},
			want:  fastPath,
			has:   append(slices.Clone(collision), "clears the SAs of peer east", "carries no traffic until"),
			fix:   espFormSharedSPIFixPeer + "east",
		},
		{
			name:  "peer named by its pending Child SA",
			state: attached(shared),
			peers: map[string]*PeerSession{"east": pending("east", "203.0.113.10"), "west": session("west", "203.0.113.9")},
			want:  fastPath,
			has:   append(slices.Clone(collision), "clears the SAs of peer east", "carries no traffic until"),
			fix:   espFormSharedSPIFixPeer + "east",
		},
		{
			name:  "the unserved SA's peer not found",
			state: attached(shared),
			peers: map[string]*PeerSession{"west": session("west", "203.0.113.9")},
			want:  fastPath,
			has:   append(slices.Clone(collision), append([]string{"Ze could not tell which peer holds it"}, listing...)...),
			fix:   espFormSharedSPIFixPeers,
		},
		{
			name:  "peer name not one word",
			state: attached(shared),
			peers: map[string]*PeerSession{`east"x`: session(`east"x`, "203.0.113.10")},
			want:  fastPath,
			has:   append(slices.Clone(collision), append([]string{`Peer east"x holds it, but its name cannot be typed as one command word`}, listing...)...),
			fix:   espFormSharedSPIFixPeers,
		},
		{
			name:  "no shared SPI",
			state: attached(nil),
			peers: map[string]*PeerSession{"east": session("east", "203.0.113.10")},
			want:  "",
		},
		{
			name: "unavailable reports its own warning alone",
			state: dataplane.ESPFormFastPathState{
				Status: dataplane.ESPFormFastPathUnavailable, Cause: "load the rewrite program: operation not permitted",
				FirewallReading: dataplane.ESPFormFirewallUnheld, SharedSPIs: shared,
			},
			peers: map[string]*PeerSession{"east": session("east", "203.0.113.10")},
			want:  fastPath,
			has:   []string{"fast path is unavailable"},
			lacks: []string{"same random inbound SPI", "203.0.113.10"},
			fix:   espFormFastPathFixCapabilities,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setActivePeers(tc.peers)
			t.Cleanup(func() { setActivePeers(nil) })
			loadESPFormBackend(t, &espFormFastPathDP{state: tc.state})
			diags := checkIPsecESPFormFastPath(newESPFormTUNRecord(nil))
			if got := diagnosticCodes(diags); got != tc.want {
				t.Fatalf("diagnostics = %q, want %q (%v)", got, tc.want, diags)
			}
			if len(diags) == 0 {
				return
			}
			d := diags[0]
			assertESPFormFinding(t, d, tc.has, tc.fix)
			for _, text := range tc.lacks {
				if strings.Contains(d.Message, text) {
					t.Errorf("%s message carries %q, which this state must not name: %q", d.Code, text, d.Message)
				}
			}
		})
	}
}

// TestDoctorESPFormSharedSPIPeerRacesNoMOBIKEMove proves the doctor's peer lookup reads a
// Child SA's address only while it holds the session's mu, the lock a MOBIKE move writes
// that address under.
//
// METHOD: one goroutine rewrites the current Child SA's LocalAddr under ps.mu, the way
// migrateMobikePair does after a move, while the test goroutine runs the lookup the
// doctor check makes. The race detector, which `./le test unit` runs with, reports a read
// of LocalAddr that the lock does not order against the write. Every answer must name
// the one peer there is, or none while the address is elsewhere.
// PREVENTS: a doctor check that races a MOBIKE move, reading a slice header half written.
// MUTATION: in espFormSharedSPIPeer, copy ps.childSA and ps.pendingChild under ps.mu and
// call espFormChildCarries after ps.mu.Unlock: the race detector reports the read of
// LocalAddr against the goroutine's write and fails the test.
func TestDoctorESPFormSharedSPIPeerRacesNoMOBIKEMove(t *testing.T) {
	const spi = 0x4321
	unserved := netip.MustParseAddr("203.0.113.10")
	elsewhere := net.ParseIP("203.0.113.11")
	child := &ChildSA{InboundSPI: spi, LocalAddr: net.ParseIP(unserved.String())}
	ps := &PeerSession{peerName: "east", childSA: child}
	setActivePeers(map[string]*PeerSession{"east": ps})
	t.Cleanup(func() { setActivePeers(nil) })

	stop := make(chan struct{})
	moved := make(chan struct{})
	go func() {
		defer close(moved)
		addrs := []net.IP{elsewhere, net.ParseIP(unserved.String())}
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			default:
			}
			ps.mu.Lock()
			child.LocalAddr = append(net.IP(nil), addrs[i%len(addrs)]...)
			ps.mu.Unlock()
		}
	}()
	for range 2000 {
		if peer, found := espFormSharedSPIPeer(spi, unserved); found && peer != "east" {
			t.Errorf("the lookup named peer %q, want east, the only session", peer)
		}
	}
	close(stop)
	<-moved
}

// TestESPFormOwnedTableAccept drives the answer the engine registers for a blocking site
// in a ze_ table (espFormOwnedTableAccept) through each kind of table, with the
// reservations this test binary holds: the IKE engine's own (register_tables.go) and the
// firewall component's conntrack helpers (firewall/register.go).
//
// VALIDATES: a table of the firewall config is answered with the three editor lines that
// add the accept-esp term to its chain and commit it, and a sentence saying the editor
// appends the term and has no command that moves it ahead of another; a reserved table
// is answered with the control its producer declared (the plugin, what decides its
// rules, and the command that shows that state), never with editor lines or an nft
// command; a ze_ table or chain name the config cannot hold, and the table named ze_
// alone, are answered with the ruleset listing.
// PREVENTS: the doctor handing the operator editor lines for a table a producer rewrites,
// an nft rule for a table Ze rewrites at the next commit, or editor lines naming a table
// the config cannot type.
func TestESPFormOwnedTableAccept(t *testing.T) {
	notConfig := "carries Ze's ze_ prefix, but no Ze producer reserved it"
	rewritten := "so neither an nft rule nor the config editor changes it for long"

	for _, tc := range []struct {
		name         string
		table, chain string
		has, lacks   []string
		fix          string
	}{
		{
			name:  "a table of the firewall config",
			table: "ze_wan", chain: "input",
			has: []string{
				"Table ze_wan is the table wan of Ze's firewall configuration",
				"Paste the lines below into ze config edit",
				"they add the term accept-esp, which accepts ESP, to chain input and commit it",
				"has no command that moves a term ahead of another",
				"accepts ESP only where no earlier term of the chain drops it",
			},
			lacks: []string{"sudo nft", rewritten},
			fix: "set firewall table wan chain input term accept-esp from protocol esp\n" +
				"set firewall table wan chain input term accept-esp then accept\n" +
				"commit now",
		},
		{
			name:  "the IKE engine's tunnel-source guard",
			table: "ze_ipsec_source_guard", chain: "postrouting",
			has: []string{
				"Table ze_ipsec_source_guard is written by Ze's IKE engine's tunnel-source guard",
				"its rules follow the IPsec site-to-site peers of the configuration and their tunnel addresses",
				rewritten, "The rule changes only with that state, which the command below shows.",
			},
			lacks: []string{"set firewall", "sudo nft", "accept-esp"},
			fix:   "show firewall ruleset ipsec_source_guard",
		},
		{
			name:  "the firewall component's conntrack helpers",
			table: "ze_conntrack_helpers", chain: "prerouting",
			has: []string{
				"Table ze_conntrack_helpers is written by Ze's firewall component's conntrack helpers",
				"its rules follow the helper modules of the system conntrack configuration",
				rewritten,
			},
			lacks: []string{"set firewall", "sudo nft", "accept-esp"},
			fix:   "show firewall ruleset conntrack_helpers",
		},
		{
			name:  "a table name the config cannot hold",
			table: "ze_bad name", chain: "input",
			has:   []string{"Table ze_bad name " + notConfig, "The command below lists the tables Ze applied."},
			lacks: []string{"set firewall", "sudo nft"},
			fix:   "show firewall ruleset",
		},
		{
			name:  "a chain name the config cannot hold",
			table: "ze_wan", chain: "in put",
			has:   []string{"Table ze_wan " + notConfig},
			lacks: []string{"set firewall", "sudo nft"},
			fix:   "show firewall ruleset",
		},
		{
			// MUTATION: delete the name == "" guard in espFormConfigName, and the
			// empty chain passes the loop, so editor lines naming no chain are printed.
			name:  "an empty chain name",
			table: "ze_wan", chain: "",
			has:   []string{"Table ze_wan " + notConfig},
			lacks: []string{"set firewall", "sudo nft"},
			fix:   "show firewall ruleset",
		},
		{
			// MUTATION: delete the len(name) > 255 guard in espFormConfigName, and a
			// chain of 256 letters, which the config cannot hold, gets editor lines.
			name:  "a chain name longer than the config holds",
			table: "ze_wan", chain: strings.Repeat("c", 256),
			has:   []string{"Table ze_wan " + notConfig},
			lacks: []string{"set firewall", "sudo nft"},
			fix:   "show firewall ruleset",
		},
		{
			// MUTATION: write len(name) >= 255 in espFormConfigName, and the longest
			// chain name the config holds is refused its editor lines.
			name:  "a chain name as long as the config holds",
			table: "ze_wan", chain: strings.Repeat("c", 255),
			has:   []string{"Table ze_wan is the table wan of Ze's firewall configuration"},
			lacks: []string{"sudo nft", rewritten},
			fix: "set firewall table wan chain " + strings.Repeat("c", 255) + " term accept-esp from protocol esp\n" +
				"set firewall table wan chain " + strings.Repeat("c", 255) + " term accept-esp then accept\n" +
				"commit now",
		},
		{
			// A config table named ze_ is the kernel table ze_ze_, so the table named
			// ze_ alone is no config table, though ze_ is a name the config holds.
			name:  "the table named ze_ alone",
			table: "ze_", chain: "input",
			has:   []string{"Table ze_ " + notConfig},
			lacks: []string{"set firewall", "sudo nft"},
			fix:   "show firewall ruleset",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			how, fix := espFormOwnedTableAccept("inet", tc.table, tc.chain)
			if fix != tc.fix {
				t.Errorf("fix = %q, want %q", fix, tc.fix)
			}
			for _, text := range tc.has {
				if !strings.Contains(how, text) {
					t.Errorf("how does not carry %q: %q", text, how)
				}
			}
			for _, text := range tc.lacks {
				if strings.Contains(how, text) {
					t.Errorf("how carries %q, which this table must not name: %q", text, how)
				}
			}
		})
	}
}

// loadESPFormBackend loads dp as the active dataplane, or leaves none loaded for a nil
// dp, as runEngine's process holds after dataplane.Load failed.
func loadESPFormBackend(t *testing.T, dp dataplane.Dataplane) {
	t.Helper()
	if dp != nil {
		loadFailClosedBackend(t, dp)
		return
	}
	if err := dataplane.CloseBackend(); err != nil {
		t.Fatalf("close backend: %v", err)
	}
	if dataplane.Get() != nil {
		t.Fatal("a dataplane is still loaded after CloseBackend")
	}
}

// stubESPFormTUN makes the engine's TUN device open (espFormTUNAvailable) answer err,
// and counts its calls.
func stubESPFormTUN(t *testing.T, err error) *int {
	t.Helper()
	calls := new(int)
	saved := espFormTUNAvailable
	espFormTUNAvailable = func() error {
		*calls++
		return err
	}
	t.Cleanup(func() { espFormTUNAvailable = saved })
	return calls
}

// espFormTUNTestRefusal is the operator error the device open answers inside a container
// started without the device, as espFormTUNOpenRefusal writes it.
func espFormTUNTestRefusal() *opererr.Error {
	return opererr.Wrap(errors.New("open /dev/net/tun: operation not permitted"),
		"the device cgroup refuses Ze access to the TUN device /dev/net/tun (c 10 200)",
		"a container started without the TUN device can hold the node but not open it",
		"in Docker: docker run --device /dev/net/tun (compose: devices: [/dev/net/tun])")
}

// TestDoctorESPFormTUN drives the TUN device finding of the same check from the outcome
// the last apply recorded (recordESPFormTUN), and proves the check never opens the
// device itself.
//
// VALIDATES: a recorded refusal is one error under doctor-ipsec-esp-form-tun, carrying
// the refusal's problem and cause, the consequence (every Child SA on UDP port 4500
// refused) and the refusal's own fix, while no SPI is watched (an idle receiver, or a
// fast path state that cannot be read); a refusal with no fix of its own keeps its text
// and lists the receiver's warnings; a recorded refusal while the receiver watches an
// SPI (attached or unavailable fast path) reports nothing, because the receiver opened
// and holds the device; no attempt recorded (no peer configured), a device that opened,
// and ErrNotSupported report nothing; a backend that is not a reporter reports nothing;
// a record that names no outcome is a warning saying the state could not be read. In
// every case espFormTUNAvailable is called zero times.
// PREVENTS: every Child SA on UDP port 4500 refused at install while doctor is silent;
// a doctor check that creates /dev/net/tun, because a runtime check only reads; a
// stale refusal reported for a device the receiver holds.
func TestDoctorESPFormTUN(t *testing.T) {
	tunCode := "error:" + diagnostic.CodeDoctorIPsecESPFormTUN
	tunUnread := "warning:" + diagnostic.CodeDoctorIPsecESPFormTUN
	fastPath := "warning:" + diagnostic.CodeDoctorIPsecESPFormFastPath
	refusal := espFormTUNTestRefusal()
	refused := newESPFormTUNRecord(refusal)
	idle := dataplane.ESPFormFastPathState{Status: dataplane.ESPFormFastPathIdle}
	attached := dataplane.ESPFormFastPathState{Status: dataplane.ESPFormFastPathAttached, FirewallReading: dataplane.ESPFormFirewallRead, Attached: []string{"eth0"}}
	unavailable := dataplane.ESPFormFastPathState{Status: dataplane.ESPFormFastPathUnavailable, FirewallReading: dataplane.ESPFormFirewallUnheld, Cause: "no CAP_BPF"}
	consequence := "so every Child SA on UDP port 4500 is refused at install"

	for _, tc := range []struct {
		name string
		dp   dataplane.Dataplane
		// tun is what the last apply recorded; nil is no attempt.
		tun  *espFormTUNRecord
		want string
		// tunText holds the texts the device finding, the last one, must carry.
		tunText []string
		tunFix  string
	}{
		{
			name: "idle, a refusal recorded", dp: &espFormFastPathDP{state: idle}, tun: refused,
			want:    tunCode,
			tunText: []string{"Ze cannot create a usable ze-espform interface", refusal.Problem(), "operation not permitted", consequence},
			tunFix:  refusal.Fix(),
		},
		{
			name: "the state cannot be read, a refusal recorded", dp: &espFormFastPathDP{stateErr: errors.New("bad file descriptor")},
			tun:     refused,
			want:    fastPath + "," + tunCode,
			tunText: []string{refusal.Problem(), consequence},
			tunFix:  refusal.Fix(),
		},
		{
			name: "a refusal with no fix of its own", dp: &espFormFastPathDP{state: idle},
			tun:     newESPFormTUNRecord(errors.New("dataplane: close /dev/net/tun after the probe opened it: bad file descriptor")),
			want:    tunCode,
			tunText: []string{"close /dev/net/tun after the probe opened it: bad file descriptor", consequence},
			tunFix:  espFormTUNFixLog,
		},
		{
			name: "a record naming no outcome", dp: &espFormFastPathDP{state: idle}, tun: &espFormTUNRecord{},
			want:    tunUnread,
			tunText: []string{"could not read the outcome", "Ze defect"},
			tunFix:  espFormFastPathFixEngineLog,
		},
		{name: "attached, a refusal recorded earlier", dp: &espFormFastPathDP{state: attached}, tun: refused},
		{name: "fast path unavailable, a refusal recorded earlier", dp: &espFormFastPathDP{state: unavailable}, tun: refused, want: fastPath},
		{name: "idle, no attempt recorded", dp: &espFormFastPathDP{state: idle}},
		{name: "idle, the device opened", dp: &espFormFastPathDP{state: idle}, tun: newESPFormTUNRecord(nil)},
		{name: "idle, not supported here", dp: &espFormFastPathDP{state: idle}, tun: newESPFormTUNRecord(dataplane.ErrNotSupported)},
		{name: "not a reporter, a refusal recorded", dp: &bypassDP{}, tun: refused},
	} {
		t.Run(tc.name, func(t *testing.T) {
			loadESPFormBackend(t, tc.dp)
			calls := stubESPFormTUN(t, errors.New("test: the doctor check opened the device"))
			diags := checkIPsecESPFormFastPath(tc.tun)
			if got := diagnosticCodes(diags); got != tc.want {
				t.Fatalf("diagnostics = %q, want %q (%v)", got, tc.want, diags)
			}
			if *calls != 0 {
				t.Errorf("the doctor check opened the device %d times, want 0: a runtime check only reads", *calls)
			}
			if tc.tunText != nil {
				assertESPFormFinding(t, diags[len(diags)-1], tc.tunText, tc.tunFix)
			}
		})
	}
}

// TestRecordESPFormTUN drives the apply path's attempt (recordESPFormTUN) with a stand-in
// device open, through applyConfig and directly.
//
// VALIDATES: an apply that configures a peer, under a dataplane with a dual-form
// receiver, opens the device exactly once and records the outcome the doctor reads; a
// refusal is logged once at WARN carrying the refusal's problem, its fix and
// code=doctor-ipsec-esp-form-tun, and the apply still succeeds; a device that opens logs
// nothing; an apply with no peer, or under a dataplane with no dual-form receiver, makes
// no attempt and clears an earlier record.
// PREVENTS: a TUN refusal that no apply records, so the doctor never reports it; a
// refusal that fails the commit of a peer that never floats to UDP port 4500; a stale
// refusal reported after its peers were removed.
func TestRecordESPFormTUN(t *testing.T) {
	refusal := espFormTUNTestRefusal()
	tunWarn := "ike: Ze cannot create a usable ze-espform interface"

	t.Run("an apply through applyConfig records the refusal and commits", func(t *testing.T) {
		loadFailClosedBackend(t, &espFormCatchAllDP{state: dataplane.ESPFormFastPathState{Status: dataplane.ESPFormFastPathIdle}})
		state := applyTestState(t)
		log, logged := cfxLogger()
		state.log = log
		calls := stubESPFormTUN(t, refusal)
		if err := state.applyConfig(testIPsecConfig(applyTestPeer("127.0.0.1")), applyReload); err != nil {
			t.Fatalf("applyConfig = %v, want nil: the device refuses per SA, never the commit", err)
		}
		if *calls != 1 {
			t.Fatalf("the apply opened the device %d times, want 1", *calls)
		}
		record := state.espFormTUN.Load()
		if record == nil || record.status != espFormTUNRefused || !errors.Is(record.err, refusal) {
			t.Fatalf("recorded %+v, want the refusal", record)
		}
		lines := cirWarnLines(logged, tunWarn)
		if len(lines) != 1 {
			t.Fatalf("%d WARN lines name the device, want 1:\n%s", len(lines), logged.String())
		}
		for _, want := range []string{
			"code=" + diagnostic.CodeDoctorIPsecESPFormTUN,
			"fix=\"" + refusal.Fix() + "\"",
			refusal.Problem(),
		} {
			if !strings.Contains(lines[0], want) {
				t.Errorf("the WARN line does not carry %q: %s", want, lines[0])
			}
		}
	})

	t.Run("each case of the attempt", func(t *testing.T) {
		peers := testIPsecConfig(applyTestPeer("127.0.0.1"))
		reporter := &espFormFastPathDP{state: dataplane.ESPFormFastPathState{Status: dataplane.ESPFormFastPathIdle}}
		for _, tc := range []struct {
			name    string
			dp      dataplane.Dataplane
			cfg     *ipsec.IPsecConfig
			openErr error
			opens   int
			// status is the recorded outcome; espFormTUNStatusUnspecified means no record.
			status espFormTUNStatus
			warns  int
		}{
			{name: "a peer, the device opens", dp: reporter, cfg: peers, opens: 1, status: espFormTUNOpened},
			{name: "a peer, the device refused", dp: reporter, cfg: peers, openErr: refusal, opens: 1, status: espFormTUNRefused, warns: 1},
			{name: "a peer, a plain error", dp: reporter, cfg: peers, openErr: errors.New("close: bad file descriptor"), opens: 1, status: espFormTUNRefused, warns: 1},
			{name: "a peer, not supported here", dp: reporter, cfg: peers, openErr: dataplane.ErrNotSupported, opens: 1, status: espFormTUNUnsupported},
			{name: "no peer", dp: reporter, cfg: testIPsecConfig(), openErr: refusal},
			{name: "no dual-form receiver", dp: &bypassDP{}, cfg: peers, openErr: refusal},
		} {
			t.Run(tc.name, func(t *testing.T) {
				loadESPFormBackend(t, tc.dp)
				log, logged := cfxLogger()
				state := &ikeEngineState{log: log}
				// An earlier apply's refusal, which a run that makes no attempt clears.
				state.espFormTUN.Store(newESPFormTUNRecord(refusal))
				calls := stubESPFormTUN(t, tc.openErr)
				state.recordESPFormTUN(tc.cfg)
				if *calls != tc.opens {
					t.Errorf("opened the device %d times, want %d", *calls, tc.opens)
				}
				record := state.espFormTUN.Load()
				switch {
				case tc.status == espFormTUNStatusUnspecified && record != nil:
					t.Errorf("recorded %+v, want no record", record)
				case tc.status != espFormTUNStatusUnspecified && (record == nil || record.status != tc.status):
					t.Errorf("recorded %+v, want status %d", record, tc.status)
				}
				lines := cirWarnLines(logged, tunWarn)
				if len(lines) != tc.warns {
					t.Fatalf("%d WARN lines, want %d:\n%s", len(lines), tc.warns, logged.String())
				}
				for _, line := range lines {
					if !strings.Contains(line, "code="+diagnostic.CodeDoctorIPsecESPFormTUN) {
						t.Errorf("the WARN line carries no doctor code: %s", line)
					}
				}
			})
		}
	})
}

// espFormCatchAllDP is a backend with a dual-form receiver that also holds the SPD
// catch-all, so a whole applyConfig runs against it.
type espFormCatchAllDP struct {
	catchAllDP
	state dataplane.ESPFormFastPathState
}

func (d *espFormCatchAllDP) ESPFormFastPathState() (dataplane.ESPFormFastPathState, error) {
	return d.state, nil
}

// assertESPFormFinding checks that one finding's message carries every text in
// contains and that its fix is fix, alone.
func assertESPFormFinding(t *testing.T, d rpc.DoctorCheckDiagnostic, contains []string, fix string) {
	t.Helper()
	for _, text := range contains {
		if !strings.Contains(d.Message, text) {
			t.Errorf("%s message does not carry %q: %q", d.Code, text, d.Message)
		}
	}
	if d.Fix != fix {
		t.Errorf("%s fix = %q, want %q", d.Code, d.Fix, fix)
	}
}

// TestDoctorESPFormFastPathRegistered reads the engine's Stage 1 declarations and finds
// the check declaring the three ESP form codes at the runtime phase, and reads the ike
// registry row and finds no check of that name there.
//
// VALIDATES: the check is declared in the sdk.Registration runEngine sends at Stage 1
// (engineDoctorCheckDecls), at rpc.DoctorPhaseRuntime, so show doctor asks the engine's
// own process, where the dataplane lives; it declares the codes the dual-form
// receiver's Warn lines name and the TUN code; no registry.DoctorCheckDef of the ike row
// carries the name, so refuseDoctorCheckDeclaredTwice admits the declaration.
// PREVENTS: a check run in the process that answers doctor, which with no daemon is the
// operator's, where dataplane.Get() is nil and the box reads healthy; a check declared
// twice, which Stage 1 refuses and which would fail the engine's start.
func TestDoctorESPFormFastPathRegistered(t *testing.T) {
	var found bool
	for _, decl := range engineDoctorCheckDecls() {
		if decl.Name != "ipsec-esp-form-fast-path" {
			continue
		}
		found = true
		if decl.Phase != rpc.DoctorPhaseRuntime {
			t.Errorf("phase = %q, want %q", decl.Phase, rpc.DoctorPhaseRuntime)
		}
		want := []string{
			diagnostic.CodeDoctorIPsecESPFormFastPath,
			diagnostic.CodeDoctorIPsecESPFormLink,
			diagnostic.CodeDoctorIPsecESPFormTUN,
			diagnostic.CodeDoctorIPsecESPFormFirewall,
		}
		if strings.Join(decl.Codes, ",") != strings.Join(want, ",") {
			t.Errorf("codes = %v, want %v", decl.Codes, want)
		}
	}
	if !found {
		t.Fatal("the engine's Stage 1 registration declares no ipsec-esp-form-fast-path doctor check")
	}
	declared := map[string]bool{}
	for _, decl := range engineDoctorCheckDecls() {
		declared[decl.Name] = true
	}
	for _, check := range registry.PluginDoctorChecks() {
		if check.PluginName == "ike" && declared[check.Name] {
			t.Errorf("the ike registry row also holds %s, so Stage 1 refuses it as declared twice", check.Name)
		}
	}
}

// TestEngineDoctorCheckDispatch drives the engine's doctor callback by name.
//
// VALIDATES: a declared name reaches its check, reading the outcome the last apply
// recorded (here, a refused device and an idle receiver, so the refusal is reported)
// without opening the device; ipsec-child-install reaches its check and reads the peer
// sessions; a name the engine did not declare is an error naming it, never an empty
// answer.
// PREVENTS: a declared check the callback cannot answer, and an unknown name read as a
// healthy box.
func TestEngineDoctorCheckDispatch(t *testing.T) {
	loadESPFormBackend(t, &espFormFastPathDP{state: dataplane.ESPFormFastPathState{Status: dataplane.ESPFormFastPathIdle}})
	calls := stubESPFormTUN(t, errors.New("test: the doctor callback opened the device"))
	state := &ikeEngineState{peers: map[string]*PeerSession{}}

	diags, err := state.answerDoctorCheck(espFormFastPathCheckName)
	if err != nil || len(diags) != 0 {
		t.Fatalf("before any apply: diagnostics %v, error %v; want none and no error", diags, err)
	}

	state.espFormTUN.Store(newESPFormTUNRecord(errors.New("open /dev/net/tun: no such device")))
	diags, err = state.answerDoctorCheck(espFormFastPathCheckName)
	if err != nil {
		t.Fatalf("answer: %v", err)
	}
	if got, want := diagnosticCodes(diags), "error:"+diagnostic.CodeDoctorIPsecESPFormTUN; got != want {
		t.Fatalf("with a refusal recorded: diagnostics = %q, want %q", got, want)
	}
	if *calls != 0 {
		t.Errorf("the doctor callback opened the device %d times, want 0", *calls)
	}

	ps := &PeerSession{peerName: "site-b"}
	ps.childRefused.Store(&childRefusal{err: errors.New("netlink: operation not supported"), at: time.Now()})
	state.peers["site-b"] = ps
	diags, err = state.answerDoctorCheck(childInstallCheckName)
	if err != nil {
		t.Fatalf("answer %s: %v", childInstallCheckName, err)
	}
	if got, want := diagnosticCodes(diags), "error:"+diagnostic.CodeDoctorIPsecChildInstallRefused; got != want {
		t.Fatalf("%s: diagnostics = %q, want %q", childInstallCheckName, got, want)
	}

	diags, err = state.answerDoctorCheck("ipsec-no-such-check")
	if err == nil {
		t.Fatalf("an undeclared name answered %v and no error", diags)
	}
	if !strings.Contains(err.Error(), "ipsec-no-such-check") {
		t.Errorf("the error does not name the check: %v", err)
	}
}
