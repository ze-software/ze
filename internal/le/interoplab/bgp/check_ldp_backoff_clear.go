// Design: docs/architecture/testing/interop.md -- concurrent state validation.
// Related: check_ldp_init_unknown_tlv.go -- ldpTransportHost, the active-role transport address this scenario shares.
// Related: register_ldp_backoff_clear.go -- puts that address on ze's eth0 before ze starts.
// Related: check_ldp_keepalive_expiry.go -- the LDP scenario whose FRR state names this one reads.
// Related: check_special.go -- specialCheckers registry entry.
// Related: check_linklocal_multihop.go -- withFRRLogTail, the FRR log a failed wait carries.
// Related: check_extras.go -- zeCommand, how the checker asks ze its LDP neighbors.
package bgp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/interoplab"
)

// ldpBackoffClearScenario proves that an LDP session which reached OPERATIONAL
// leaves no setup backoff behind when it ends: after FRR clears the session,
// ze dials again on FRR's next Hello rather than after the 15 s that RFC 5036
// Section 2.5.3 puts after a NAK'd Initialization.
const ldpBackoffClearScenario = "ldp-backoff-clear-frr"

const (
	// ldpClearSettled is how long FRR must report the session OPERATIONAL
	// before the clear. A session that was never cleared then reports an
	// upTime at least this much longer than the time since the clear, so a
	// fresh upTime proves a re-establishment (ldpNeighbor.reestablishedWithin).
	ldpClearSettled = 3 * time.Second
	// ldpClearRedialBound is the longest the session may take to be OPERATIONAL
	// again after the clear. ze dials on each Hello it receives from FRR
	// (processDiscoveryPacket calls startSessionForAdj), and frr.conf sets
	// FRR's Hello interval to 1 s, so with the backoff released ze dials within
	// 1 s of the session end; the TCP connect and the Initialization and
	// KeepAlive exchange on one bridge take milliseconds, and the probe adds at
	// most ldpClearPoll and one docker exec. That path lands near 2 s. A session
	// end that records a setup failure (recordSetupFailure) forbids a dial for
	// setupRetryDelayMin, 15 s, after the end, so that path lands at 15 s or
	// later. 8 s leaves 6 s above the first and 7 s below the second.
	ldpClearRedialBound = 8 * time.Second
	// ldpClearRecoveryWait outlasts the 15 s backoff with margin, so a red run
	// reports the measured delay rather than a timeout.
	ldpClearRecoveryWait = 45 * time.Second
	ldpClearPoll         = 250 * time.Millisecond
	// ldpClearEstablishWait covers discovery, ze's first dial and FRR's
	// handling of it.
	ldpClearEstablishWait = 120 * time.Second
	// ldpClearLogLinesMax bounds each read of ze's log. The scenario logs a few
	// hundred lines, so both reads hold its whole history and the counts
	// compared in requireLDPBackoffReleased see every line.
	ldpClearLogLinesMax = 20000
	// ldpStateChangesMax bounds the state changes a failed wait reports. A
	// session that settles changes state a handful of times; one that flaps
	// every second for 120 s would otherwise print a line per flap.
	ldpStateChangesMax = 32
	// ldpEvidenceBound bounds the reads a failed wait makes for its evidence.
	// They run after the wait's own deadline, so they get a bound of their own.
	ldpEvidenceBound = 20 * time.Second
)

// zeShowLDPNeighbor is the command whose answer is ze's view of its LDP
// sessions, attached to a failed wait beside FRR's.
const zeShowLDPNeighbor = "show ldp neighbor"

// The messages ze's LDP plugin logs, counted in ze's log before and after the
// clear. Each is the msg of one call: Session.ReadLoop (Warn) on FRR's fatal
// Shutdown Notification, runSession (Info) when the session ends, and
// recordSetupFailure (Info) when it records a setup backoff.
const (
	ldpZeFatalNotification = "ldp: fatal notification received, closing the session"
	ldpZeSessionEnded      = "ldp: session ended"
	ldpZeSetupFailed       = "ldp: session setup failed, next attempt delayed"
)

var frrShowLDPNeighborJSON = []string{cmdVtysh, "-c", "show mpls ldp neighbor json"}

// checkLDPBackoffClear brings the LDP session with FRR to OPERATIONAL, has FRR
// clear it, and requires ze to bring it back within ldpClearRedialBound, with
// no setup backoff recorded for its end.
//
// FRR 10.7.1's `clear mpls ldp neighbor <address>` (ldp_vty_clear_nbr, then
// nbr_clear_ctl) matches the neighbor's transport address and calls
// session_shutdown with S_SHUTDOWN: FRR sends a Shutdown Notification and
// closes the TCP connection, the Hello adjacency and the neighbor survive, and
// FRR arms no initialization delay of its own. FRR holds the passive role, so
// the redial measured here is ze's.
//
// The proof does not rest on wall time alone. ze's own log must show FRR's
// Shutdown reaching it and the session ending, and must show no setup backoff
// recorded for that end. FRR's upTime (show_nbr_msg_json, nbr_to_ctl) is 0
// outside OPERATIONAL and restarts on each entry to it, so a fresh upTime
// proves FRR saw the session come back.
func checkLDPBackoffClear(ctx context.Context, check *interoplab.CheckContext) error {
	if !check.Network.IPv4.IsValid() {
		return errors.New("the LDP backoff clear scenario has no selected IPv4 network")
	}
	// The transport address is on ze's eth0 before ze starts
	// (register_ldp_backoff_clear.go), so ze's first dial can bind it.
	zeTransport := networkHostAddress(check.Network, ldpTransportHost)

	probe := func(probeCtx context.Context) (string, error) {
		return check.Lab.Query(probeCtx, peerFRR, frrShowLDPNeighborJSON, nil)
	}
	establishTrail := ldpStateTrail{origin: "into the wait"}
	establishStart := time.Now()
	_, establishReport, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     ldpClearEstablishWait,
		Interval:    time.Second,
		Description: "FRR's LDP session with ze OPERATIONAL for 3 s",
	}, probe, func(output string) bool {
		neighbor, err := frrLDPNeighborAt(output, zeTransport)
		establishTrail.observe(neighbor, err, time.Since(establishStart))
		return err == nil && neighbor.settled()
	})
	if err != nil {
		cause := fmt.Errorf("the LDP session with ze never stayed OPERATIONAL for %s: %w", ldpClearSettled, err)
		return ldpWaitFailure(ctx, check.Lab, cause, establishReport, &establishTrail)
	}

	before, err := readZeLDPLog(ctx, check.Lab)
	if err != nil {
		return fmt.Errorf("read ze's log before the clear: %w", err)
	}

	var clear textbuf.Buffer
	clearCommand := []string{cmdVtysh, "-c", clear.Str("clear mpls ldp neighbor ").Str(zeTransport).String()}
	clearStart := time.Now()
	if _, err := check.Lab.Exec(ctx, peerFRR, clearCommand, nil); err != nil {
		return fmt.Errorf("clear FRR's LDP session with ze: %w", err)
	}
	recoveryTrail := ldpStateTrail{origin: "after the clear"}
	_, recoveryReport, recoveryErr := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     ldpClearRecoveryWait,
		Interval:    ldpClearPoll,
		Description: "FRR's LDP session with ze OPERATIONAL again after the clear",
	}, probe, func(output string) bool {
		neighbor, err := frrLDPNeighborAt(output, zeTransport)
		sinceClear := time.Since(clearStart)
		recoveryTrail.observe(neighbor, err, sinceClear)
		return err == nil && neighbor.reestablishedWithin(sinceClear)
	})
	redial := time.Since(clearStart)

	after, err := readZeLDPLog(ctx, check.Lab)
	if err != nil {
		return fmt.Errorf("read ze's log after the clear: %w", err)
	}
	if err := requireLDPBackoffReleased(countZeLDPLog(before), countZeLDPLog(after)); err != nil {
		cause := fmt.Errorf("%w (the probes ended %s after the clear)", err, redial.Round(time.Millisecond))
		if recoveryErr == nil {
			return cause
		}
		// The session also never came back: the backoff refusal is the
		// diagnosis, and the wait's evidence goes with it.
		cause = fmt.Errorf("%w; FRR's LDP session with ze was also not OPERATIONAL again within %s of the clear: %w", cause, ldpClearRecoveryWait, recoveryErr)
		return ldpWaitFailure(ctx, check.Lab, cause, recoveryReport, &recoveryTrail)
	}
	if recoveryErr != nil {
		cause := fmt.Errorf("FRR's LDP session with ze was not OPERATIONAL again within %s of the clear: %w", ldpClearRecoveryWait, recoveryErr)
		return ldpWaitFailure(ctx, check.Lab, cause, recoveryReport, &recoveryTrail)
	}
	return requireLDPRedialWithin(redial)
}

// readZeLDPLog reads ze's log, refusing a log Docker could not read: the
// setup-backoff count is read for its absence, and an unreadable log holds
// none.
func readZeLDPLog(ctx context.Context, lab interoplab.CheckerLab) (string, error) {
	logs, err := lab.Logs(ctx, "ze", ldpClearLogLinesMax)
	if err != nil {
		return "", err
	}
	if !logs.Available {
		return "", errors.New("ze's log is not readable, so the setup backoff it records could not be read")
	}
	return logs.Text, nil
}

// zeLDPLogCounts is how many times ze's log carries each LDP message the
// scenario judges.
type zeLDPLogCounts struct {
	fatalNotifications int
	sessionsEnded      int
	setupFailures      int
}

// countZeLDPLog counts the judged messages in a read of ze's log.
func countZeLDPLog(zeLog string) zeLDPLogCounts {
	return zeLDPLogCounts{
		fatalNotifications: strings.Count(zeLog, ldpZeFatalNotification),
		sessionsEnded:      strings.Count(zeLog, ldpZeSessionEnded),
		setupFailures:      strings.Count(zeLog, ldpZeSetupFailed),
	}
}

// requireLDPBackoffReleased judges ze's log across the clear. ze must log a
// new fatal Notification, the Shutdown FRR sent, so the clear reached ze; a
// new session end at info, so the info lines are read and the next count's
// absence means something; and no new setup failure, because the session that
// ended had reached OPERATIONAL.
func requireLDPBackoffReleased(before, after zeLDPLogCounts) error {
	if after.fatalNotifications <= before.fatalNotifications {
		return fmt.Errorf("ze logged no %q after FRR's clear, so FRR's Shutdown Notification never reached ze", ldpZeFatalNotification)
	}
	if after.sessionsEnded <= before.sessionsEnded {
		return fmt.Errorf("ze logged no %q at info after FRR's clear, so the absence of a setup backoff in its log proves nothing: the scenario's ze.conf must keep `environment { log { ldp info } }`", ldpZeSessionEnded)
	}
	if after.setupFailures > before.setupFailures {
		return fmt.Errorf("ze logged %q %d time(s) after FRR cleared a session that had reached OPERATIONAL: its end recorded a setup backoff, which delays the redial by 15 s or more", ldpZeSetupFailed, after.setupFailures-before.setupFailures)
	}
	return nil
}

// requireLDPRedialWithin judges the time from the clear to FRR seeing the
// session OPERATIONAL again. redial is measured from before the clear was sent
// to after the probe that saw it, so it never undercounts the real delay.
func requireLDPRedialWithin(redial time.Duration) error {
	if redial > ldpClearRedialBound {
		return fmt.Errorf("FRR's LDP session with ze was OPERATIONAL again %s after the clear, beyond the %s bound: ze dials on each FRR Hello (1 s), so a later redial is a setup backoff left by a session that had reached OPERATIONAL", redial.Round(time.Millisecond), ldpClearRedialBound)
	}
	return nil
}

// ldpWaitFailure joins to a failed wait the evidence that explains it: what
// the wait measured, ze's answer to `show ldp neighbor`, and the tail of FRR's
// log, which carries ldpd's event and message debug lines (frr.conf). A
// timeout alone shows FRR's last answer, which cannot say why the session came
// up late or which side ended it. The reads run after the wait's deadline may
// have ended ctx, so they get a bound of their own.
func ldpWaitFailure(ctx context.Context, lab interoplab.CheckerLab, cause error, report interoplab.WaitReport, trail *ldpStateTrail) error {
	evidenceCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), ldpEvidenceBound)
	defer cancel()

	var text textbuf.Buffer
	text.Str(ldpWaitSummary(report, trail)).
		Str("\nRead ze's neighbor view and FRR's ldpd log below: the side that logged a Notification, a refused message or a closed connection held the session back.").
		Str("\n--- ze `").Str(zeShowLDPNeighbor).Str("` ---\n")
	answer, err := lab.Query(evidenceCtx, "ze", zeCommand(zeShowLDPNeighbor), nil)
	if err != nil {
		text.Str("(ze did not answer: ").Err(err).Byte(')')
	} else {
		text.Str(interoplab.OutputExcerpt(answer))
	}
	return withFRRLogTail(evidenceCtx, lab, fmt.Errorf("%w\n%s", cause, text.String()))
}

// ldpWaitSummary renders what a failed wait measured: the probes it made, how
// long it ran, how many probes could not read FRR, and the states the trail
// recorded.
func ldpWaitSummary(report interoplab.WaitReport, trail *ldpStateTrail) string {
	var text textbuf.Buffer
	text.Str("The wait made ").Int(int64(report.Attempts)).
		Str(" probes over ").Str(report.Elapsed.Round(time.Millisecond).String()).
		Str(", and ").Int(int64(report.TransientFailures)).Str(" of them could not read FRR's neighbors.")
	if len(trail.changes) == 0 {
		text.Str(" No probe read FRR's answer, so FRR's view of the session is unknown.")
		return text.String()
	}
	text.Str(" FRR reported ze's session as ")
	for index, change := range trail.changes {
		if index > 0 {
			text.Str(", then ")
		}
		text.Str(change.state).Byte(' ').Str(change.at.Round(time.Millisecond).String()).Byte(' ').Str(trail.origin)
	}
	if trail.dropped > 0 {
		text.Str(", and ").Int(int64(trail.dropped)).Str(" later changes not shown")
	}
	text.Byte('.')
	return text.String()
}

// The labels a trail records beside FRR's own state names.
const (
	ldpStateUnread    = "no readable session at ze's transport address"
	ldpStateRestarted = " (upTime restarted)"
)

// ldpStateChange is one state FRR reported for ze's session, at its offset
// from the trail's origin.
type ldpStateChange struct {
	state string
	at    time.Duration
}

// ldpStateTrail records the states FRR reported for ze's session across one
// wait, one entry for each change, so a failed wait says whether the session
// never came up, came up late, or flapped. A probe whose answer holds no
// readable session at ze's address is recorded as ldpStateUnread. An
// OPERATIONAL reading whose upTime is shorter than the previous one is a
// session that ended and came back between two probes, and is recorded with
// ldpStateRestarted. At most ldpStateChangesMax changes are kept; later ones
// are counted. Not safe for concurrent use: interoplab.Wait calls its
// readiness predicate on the caller's goroutine.
type ldpStateTrail struct {
	origin   string // how an offset reads: "into the wait", "after the clear"
	changes  []ldpStateChange
	dropped  int
	observed bool // label and previous hold the last probe's reading
	label    string
	previous ldpNeighbor
}

// observe records one probe's reading of ze's session, taken at offset at.
func (t *ldpStateTrail) observe(neighbor ldpNeighbor, readErr error, at time.Duration) {
	label := ldpStateUnread
	if readErr == nil {
		label = neighbor.state
	}
	changed := !t.observed || label != t.label
	restarted := !changed && readErr == nil && neighbor.upTime < t.previous.upTime
	t.observed = true
	t.label = label
	t.previous = neighbor
	switch {
	case restarted:
		label += ldpStateRestarted
	case !changed:
		return
	}
	if len(t.changes) >= ldpStateChangesMax {
		t.dropped++
		return
	}
	t.changes = append(t.changes, ldpStateChange{state: label, at: at})
}

// frrLDPNeighbors is the answer of FRR's `show mpls ldp neighbor json`:
// show_nbr_msg_json writes one object per neighbor under "neighbors".
type frrLDPNeighbors struct {
	Neighbors []frrLDPNeighborJSON `json:"neighbors"`
}

type frrLDPNeighborJSON struct {
	State            string `json:"state"`
	TransportAddress string `json:"transportAddress"`
	UpTime           string `json:"upTime"`
}

// ldpNeighbor is one FRR LDP neighbor with its upTime parsed.
type ldpNeighbor struct {
	state  string
	upTime time.Duration
}

// frrLDPNeighborAt decodes FRR's neighbor list and answers the neighbor whose
// transport address is transport. It refuses an answer that does not decode,
// lists no such neighbor, or carries an upTime it cannot read.
func frrLDPNeighborAt(output, transport string) (ldpNeighbor, error) {
	var answer frrLDPNeighbors
	if err := json.Unmarshal([]byte(output), &answer); err != nil {
		return ldpNeighbor{}, fmt.Errorf("decode FRR's LDP neighbors: %w", err)
	}
	for _, neighbor := range answer.Neighbors {
		if neighbor.TransportAddress != transport {
			continue
		}
		upTime, err := parseFRRUpTime(neighbor.UpTime)
		if err != nil {
			return ldpNeighbor{}, err
		}
		return ldpNeighbor{state: neighbor.State, upTime: upTime}, nil
	}
	return ldpNeighbor{}, fmt.Errorf("FRR lists no LDP neighbor at transport address %s", transport)
}

// parseFRRUpTime reads ldpd's log_time form for an age under one day,
// HH:MM:SS. The day and week forms are refused: no session in this scenario
// lives that long.
func parseFRRUpTime(upTime string) (time.Duration, error) {
	fields := strings.Split(upTime, ":")
	if len(fields) != 3 {
		return 0, fmt.Errorf("FRR's LDP upTime %q is not HH:MM:SS", upTime)
	}
	var seconds time.Duration
	for index, unit := range []time.Duration{time.Hour, time.Minute, time.Second} {
		value, err := strconv.Atoi(fields[index])
		if err != nil {
			return 0, fmt.Errorf("FRR's LDP upTime %q is not HH:MM:SS: %w", upTime, err)
		}
		if value < 0 {
			return 0, fmt.Errorf("FRR's LDP upTime %q holds a negative field", upTime)
		}
		seconds += time.Duration(value) * unit
	}
	return seconds, nil
}

// settled reports whether the session has been OPERATIONAL for ldpClearSettled.
func (n ldpNeighbor) settled() bool {
	if n.state != ldpStateOperational {
		return false
	}
	return n.upTime >= ldpClearSettled
}

// reestablishedWithin reports whether the session is OPERATIONAL with an
// upTime no longer than sinceClear, the time since the clear was sent. upTime
// is whole seconds rounded down, so a session that came back after the clear
// always passes, and one up since before it, settled for ldpClearSettled, is
// always at least 2 s too old to.
func (n ldpNeighbor) reestablishedWithin(sinceClear time.Duration) bool {
	if n.state != ldpStateOperational {
		return false
	}
	return n.upTime <= sinceClear
}
