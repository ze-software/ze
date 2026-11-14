// Design: docs/architecture/testing/interop.md -- a peer that stops reading, judged against a frozen BIRD.
// Related: register_peer_stops_reading.go -- specialCheckers registry entry.
// Related: check_special.go -- checkHoldtimeDeadPeer, the same freeze with nothing for ze to write.
// Related: check_rpki_reload.go -- applyZeReload, the reload this checker starts and does not wait for.
package bgp

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// peerStopsReadingScenario proves that ze ends a session whose peer stopped
// reading while ze still had routes to write to it, by its hold time, and
// that the session and the routes come back when the peer reads again.
//
// A frozen BIRD and an idle ze prove nothing about that: with nothing to
// write, no writer holds ze's write path, and the hold timer closes the
// connection on any build. So the checker freezes BIRD and then reloads ze
// with a config that adds more routes than the two sockets hold. Ze's writer
// blocks in the kernel, and the hold timer has to close the connection behind
// it.
const peerStopsReadingScenario = "peer-stops-reading-bird"

const (
	// peerStopsReadingBIRDHost is BIRD's host number in the lab network,
	// the 172.30.0.4 of bird.conf.
	peerStopsReadingBIRDHost = 4
	// peerStopsReadingInitialRoutes is how many routes ze.conf originates,
	// and peerStopsReadingReloadRoutes how many ze-reload.conf adds: two /8
	// split into /24.
	peerStopsReadingInitialRoutes = 2
	peerStopsReadingReloadRoutes  = 2 * 65536
	// peerStopsReadingBIRDRoutes is how many routes bird.conf originates.
	peerStopsReadingBIRDRoutes = 2

	// peerStopsReadingHold is ze.conf's receive-hold-time, which RFC 4271
	// Section 4.2 makes the session's hold time because BIRD's is larger.
	peerStopsReadingHold = 9 * time.Second
	// peerStopsReadingKeepalive is the longest BIRD leaves between two
	// KEEPALIVEs: a third of the hold time, less up to a quarter of jitter
	// (bgp_start_timer, proto/bgp/bgp.c). So ze's hold timer was last
	// restarted at most this long before BIRD froze.
	peerStopsReadingKeepalive = peerStopsReadingHold / 3
	// peerStopsReadingCloseBound is how long after the expiry ze may take to
	// close: one second to wait for the write path before it gives up the
	// NOTIFICATION, and one second to clear the connection afterwards.
	peerStopsReadingCloseBound = 2 * time.Second
	// peerStopsReadingWriteBudget is ze.fwd.write.deadline, which the lab
	// leaves at its default. A blocked write gives up this long after it
	// started, so a close that waits for the writer cannot come sooner than
	// this after the freeze.
	peerStopsReadingWriteBudget = 30 * time.Second
	// peerStopsReadingFreezeSlack is what the freeze itself may take: the
	// clock starts when Pause returns, a moment after BIRD stopped.
	peerStopsReadingFreezeSlack = time.Second

	// peerStopsReadingCloseEarliest is the soonest the hold timer can have
	// closed the session, measured from the freeze: 5 s. A close before it
	// has another cause, such as a reload that restarted the peer.
	peerStopsReadingCloseEarliest = peerStopsReadingHold - peerStopsReadingKeepalive - peerStopsReadingFreezeSlack
	// peerStopsReadingCloseLatest is halfway between the latest close on
	// hold expiry (hold time plus the close bound, 11 s) and the earliest
	// close that waited for the blocked write (the write budget, 30 s):
	// 20.5 s. It leaves each side more than nine seconds for a loaded host
	// to report late.
	peerStopsReadingCloseLatest = (peerStopsReadingHold + peerStopsReadingCloseBound + peerStopsReadingWriteBudget) / 2

	// peerStopsReadingCloseWait outlasts the write budget, so a build whose
	// close waits for the writer is reported with the time it took rather
	// than as a bare timeout.
	peerStopsReadingCloseWait = 60 * time.Second
	// peerStopsReadingSessionWait covers a first establishment and a state
	// read, as the suite's other session waits do.
	peerStopsReadingSessionWait = 90 * time.Second
	// peerStopsReadingReturnWait covers the second session. After the thaw
	// BIRD reads what the kernel still holds for the old connection, then
	// its end: bgp_sock_err restarts the protocol with no delay, while
	// bgp_hold_timeout reaching bgp_error first holds it for bird.conf's
	// error wait time, at most 10 s. BIRD then dials after its 5 s connect
	// delay (bgp_active), and ze redials on a backoff of at most 60 s.
	peerStopsReadingReturnWait = 120 * time.Second
	// peerStopsReadingSyncWait covers ze's initial update on the second
	// session, about 56 MiB of UPDATEs, and BIRD importing it.
	peerStopsReadingSyncWait = 180 * time.Second
	// peerStopsReadingEvidenceBound bounds the log reads a failure carries.
	// They run after a wait may have ended ctx, so they get their own.
	peerStopsReadingEvidenceBound = 20 * time.Second

	// peerStopsReadingLogLines is how much of each log is read. Ze logs at
	// WARN plus the peer logger's INFO lines, and BIRD only its state
	// changes, so both logs stay far below it.
	peerStopsReadingLogLines = 2000
	// peerStopsReadingQueueFloor is the least the kernel must hold unsent
	// toward the frozen BIRD: the size of a session's own write buffer. A
	// session that is read holds a few dozen octets at most.
	peerStopsReadingQueueFloor = 16 * 1024

	// peerStopsReadingEstablishedLog and peerStopsReadingClosedLog are the
	// two INFO lines of ze's peer logger the checker counts. "session
	// closed" is logged by Peer.sessionDown, once Session.Run has returned.
	peerStopsReadingEstablishedLog = "session established"
	peerStopsReadingClosedLog      = "session closed"
	// peerStopsReadingBIRDUpLog is what BIRD logs each time the protocol
	// reaches up under `debug { states }` (proto_log_state_change,
	// nest/proto.c).
	peerStopsReadingBIRDUpLog = birdZeProtocol + ": State changed to up"
	// peerStopsReadingBIRDRouteCount asks BIRD how many routes it holds
	// from ze. rt_show_cont (nest/rt-show.c) answers "<shown> of <read>
	// routes for <networks> networks in table <name>", where <shown> counts
	// the routes the protocol filter kept.
	peerStopsReadingBIRDRouteCount = "show route protocol " + birdZeProtocol + " count"
	// peerStopsReadingSocketTables prints the kernel's TCP socket tables of
	// the container. A session ze accepted on a dual-stack listener is in
	// the second one, which a kernel without IPv6 does not have.
	peerStopsReadingSocketTables = "cat /proc/net/tcp; cat /proc/net/tcp6 2>/dev/null; true"
	// peerStopsReadingBGPPort is the BGP port, as the socket tables print it.
	peerStopsReadingBGPPort = 179
)

// checkPeerStopsReading freezes BIRD, makes ze write to it, and judges when
// ze ended the session and what came back after the thaw.
//
// BIRD 3.4.0 sources the expectations name, read at the v3.4.0 tag the lab
// builds:
//   - `show protocols` prints "Established" for the session: bgp_get_status
//     (proto/bgp/bgp.c).
//   - the route count line: rt_show_cont (nest/rt-show.c).
//   - "ze_peer: State changed to up": proto_log_state_change (nest/proto.c).
//   - after the thaw: bgp_hold_timeout restarts the hold timer for 10 s when
//     the socket has input, which it has, so BIRD reads the old connection to
//     its end and bgp_sock_err takes the session down.
//
// The thaw runs on every exit path, so a failed run leaves no frozen
// container behind.
func checkPeerStopsReading(ctx context.Context, check *interoplab.CheckContext) (resultErr error) {
	if !check.Network.IPv4.IsValid() {
		return errors.New("the peer-stops-reading scenario has no selected IPv4 network")
	}
	lab := check.Lab
	// Lab.Query runs the command as written, so the address is this run's
	// network rather than the 172.30.0.0/24 the scenario files spell.
	birdAddress := networkHostAddress(check.Network, peerStopsReadingBIRDHost)
	assertion := 0
	fail := func(cause error) error {
		evidenceCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), peerStopsReadingEvidenceBound)
		defer cancel()
		return checkerFailure(evidenceCtx, lab, peerStopsReadingScenario, assertion, cause)
	}

	// The session is up and each side holds the other's routes.
	assertion++
	if err := waitContains(ctx, lab, peerBIRD, []string{cmdBirdc, "show protocols"}, peerStopsReadingSessionWait, birdZeProtocol, stateEstablished); err != nil {
		return fail(err)
	}
	assertion++
	if err := waitPeerStopsReadingBIRDRoutes(ctx, lab, peerStopsReadingInitialRoutes, peerStopsReadingSessionWait); err != nil {
		return fail(err)
	}
	assertion++
	if err := waitJSONFields(ctx, lab, "ze", zeCommand(zeShowBGPRIBStatus), peerStopsReadingSessionWait, nil, map[string]int{fieldRoutesIn: peerStopsReadingBIRDRoutes}); err != nil {
		return fail(err)
	}
	assertion++
	before, err := readPeerStopsReadingSessions(ctx, lab, birdAddress)
	if err != nil {
		return fail(err)
	}
	if err := before.openVerdict(); err != nil {
		return fail(err)
	}

	assertion++
	if err := lab.Pause(ctx, peerBIRD); err != nil {
		return fail(fmt.Errorf("freeze BIRD: %w", err))
	}
	frozenAt := time.Now()
	thawed := false
	defer func() {
		if thawed {
			return
		}
		thawErr := lab.Unpause(context.WithoutCancel(ctx), peerBIRD)
		if thawErr != nil && resultErr == nil {
			resultErr = fmt.Errorf("thaw BIRD during cleanup: %w", thawErr)
		}
	}()

	// The reload is started and not waited for: it delivers the new routes
	// on the running session (Peer.deliverStaticRouteDelta), so it is the
	// writer that blocks, and it ends only when the session does.
	assertion++
	if _, err := lab.Exec(ctx, "ze", []string{"cp", zeMountedReloadConfig, zeRunningConfig}, nil); err != nil {
		return fail(fmt.Errorf("stage the reload config in ze: %w", err))
	}
	if err := lab.Signal(ctx, "ze", signalHUP); err != nil {
		return fail(fmt.Errorf("reload ze: %w", err))
	}

	assertion++
	frozen, closeErr := observePeerStopsReadingClose(ctx, lab, birdAddress, before.zeClosed)
	closedAfter := time.Since(frozenAt)
	if thawErr := lab.Unpause(context.WithoutCancel(ctx), peerBIRD); thawErr != nil {
		return fail(fmt.Errorf("thaw BIRD: %w", thawErr))
	}
	thawed = true
	if closeErr != nil {
		return fail(closeErr)
	}
	assertion++
	if err := peerStopsReadingCloseVerdict(closedAfter); err != nil {
		return fail(interoplab.WithSearchedOutput(err, frozen.zeLogs))
	}
	assertion++
	if err := frozen.blockedWriterVerdict(); err != nil {
		return fail(interoplab.WithSearchedOutput(err, frozen.zeLogs))
	}

	// The session comes back, and it is a new one: BIRD reached up once more.
	assertion++
	if err := waitPeerStopsReadingBIRDUp(ctx, lab, before.birdUp+1); err != nil {
		return fail(err)
	}
	assertion++
	if err := waitZePeerState(ctx, lab, birdAddress, peerStopsReadingSessionWait); err != nil {
		return fail(err)
	}
	// BIRD holds ze's routes again, the reload's included: the whole initial
	// update of the second session reached it.
	assertion++
	if err := waitPeerStopsReadingBIRDRoutes(ctx, lab, peerStopsReadingInitialRoutes+peerStopsReadingReloadRoutes, peerStopsReadingSyncWait); err != nil {
		return fail(err)
	}
	// The sessions are counted once more after the routes arrived: a second
	// session that ended during its initial update shows here.
	assertion++
	after, err := readPeerStopsReadingSessions(ctx, lab, birdAddress)
	if err != nil {
		return fail(err)
	}
	if err := before.returnVerdict(after); err != nil {
		return fail(err)
	}
	return nil
}

// peerStopsReadingFrozen is what the checker read while BIRD was frozen.
type peerStopsReadingFrozen struct {
	// zeLogs is ze's log as the last probe read it.
	zeLogs string
	// socketSeen is true once a probe found ze's BGP connection in the
	// socket tables, and unsentOctetsMax is the largest send queue any
	// probe read for it.
	socketSeen      bool
	unsentOctetsMax uint64
	// socketErr is the last failure to read the socket tables.
	socketErr error
}

// observePeerStopsReadingClose waits for ze to log one more "session closed"
// for address than closedBefore, and on every probe reads how much ze's
// kernel holds unsent toward BIRD. The queue is read on each probe because
// the connection is gone from the tables by the probe that sees the close.
func observePeerStopsReadingClose(ctx context.Context, lab interoplab.CheckerLab, address string, closedBefore int) (peerStopsReadingFrozen, error) {
	var frozen peerStopsReadingFrozen
	tablesCommand := []string{"sh", "-c", peerStopsReadingSocketTables}
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     peerStopsReadingCloseWait,
		Interval:    time.Second,
		Description: "ze's session with the frozen BIRD closed",
	}, func(probeCtx context.Context) (string, error) {
		logs, logErr := lab.Logs(probeCtx, "ze", peerStopsReadingLogLines)
		if logErr != nil {
			return "", logErr
		}
		if !logs.Available {
			return "", errors.New("ze logs were not read")
		}
		frozen.zeLogs = logs.Text
		tables, tablesErr := lab.Query(probeCtx, "ze", tablesCommand, nil)
		frozen.recordSockets(tables, tablesErr)
		return logs.Text, nil
	}, func(logs string) bool {
		return logLinesWith(logs, peerStopsReadingClosedLog, address) > closedBefore
	})
	if err != nil {
		return frozen, fmt.Errorf("ze never logged %q for %s after BIRD froze, so the session with a peer that reads nothing never ended: %w", peerStopsReadingClosedLog, address, err)
	}
	return frozen, nil
}

// recordSockets keeps what one read of the socket tables showed.
func (f *peerStopsReadingFrozen) recordSockets(tables string, err error) {
	if err != nil {
		f.socketErr = err
		return
	}
	octets, found := bgpSocketUnsentOctets(tables)
	if !found {
		return
	}
	f.socketSeen = true
	f.unsentOctetsMax = max(f.unsentOctetsMax, octets)
}

// blockedWriterVerdict answers whether the frozen phase put a blocked writer
// in front of the hold timer, which is what the scenario exists to exercise.
// Without it the close succeeds on any build, and the timing above proves
// nothing.
//
// Two readings decide it. The kernel held unsent octets toward BIRD, so ze
// wrote more than BIRD accepted. And ze logged no delivered Hold Timer
// Expired NOTIFICATION: ze logs "notification sent" only for a NOTIFICATION
// it wrote (Session.notificationWritten), and it can write one only when the
// write path is free, which a blocked writer never leaves it. The same line
// is what bgp-holdtime-deadpeer-frr requires when nothing is written.
func (f *peerStopsReadingFrozen) blockedWriterVerdict() error {
	if !f.socketSeen {
		if f.socketErr != nil {
			return fmt.Errorf("ze's socket tables were not read while BIRD was frozen, so whether the reload's routes filled the socket is unknown: %w", f.socketErr)
		}
		return errors.New("ze's socket tables showed no BGP connection while BIRD was frozen, so whether the reload's routes filled the socket is unknown")
	}
	if f.unsentOctetsMax < peerStopsReadingQueueFloor {
		return fmt.Errorf("ze's kernel held at most %d unsent octets toward the frozen BIRD, want at least %d: the reload's routes never filled the socket, so no writer was blocked when the hold timer expired and this run did not exercise the close behind a blocked writer. Check that the reload delivered the routes of ze-reload.conf", f.unsentOctetsMax, peerStopsReadingQueueFloor)
	}
	if holdNotificationSeen(f.zeLogs) {
		return errors.New("ze delivered a Hold Timer Expired NOTIFICATION to the frozen BIRD, which needs a free write path: every route of the reload had been written when the hold timer expired, so no writer was blocked and this run did not exercise the close behind a blocked writer. Add routes to ze-reload.conf until they exceed what the two sockets hold")
	}
	return nil
}

// peerStopsReadingCloseVerdict judges how long after the freeze ze logged
// the session closed.
func peerStopsReadingCloseVerdict(elapsed time.Duration) error {
	if elapsed < peerStopsReadingCloseEarliest {
		return fmt.Errorf("ze closed the session %s after BIRD froze, sooner than the %s the %s hold timer needs at the least: something other than the hold timer ended the session", elapsed, peerStopsReadingCloseEarliest, peerStopsReadingHold)
	}
	if elapsed >= peerStopsReadingCloseLatest {
		return fmt.Errorf("ze closed the session %s after BIRD froze, want under %s: the %s hold timer expired and the close must follow within %s, so this close waited for the blocked write to give up (the %s write budget) instead of closing the connection behind it (RFC 4271 Section 8.2.2, Event 10)", elapsed, peerStopsReadingCloseLatest, peerStopsReadingHold, peerStopsReadingCloseBound, peerStopsReadingWriteBudget)
	}
	return nil
}

// peerStopsReadingSessions counts the sessions each side has logged.
type peerStopsReadingSessions struct {
	zeEstablished int
	zeClosed      int
	birdUp        int
}

// readPeerStopsReadingSessions counts the sessions in both logs. A log that
// was not read is an error, never a count of zero.
func readPeerStopsReadingSessions(ctx context.Context, lab interoplab.CheckerLab, address string) (peerStopsReadingSessions, error) {
	zeLogs, err := lab.Logs(ctx, "ze", peerStopsReadingLogLines)
	if err != nil {
		return peerStopsReadingSessions{}, fmt.Errorf("read ze's log to count its sessions: %w", err)
	}
	if !zeLogs.Available {
		return peerStopsReadingSessions{}, errors.New("ze logs were not read, so its sessions were not counted")
	}
	birdLogs, err := lab.Logs(ctx, peerBIRD, peerStopsReadingLogLines)
	if err != nil {
		return peerStopsReadingSessions{}, fmt.Errorf("read BIRD's log to count its sessions: %w", err)
	}
	if !birdLogs.Available {
		return peerStopsReadingSessions{}, errors.New("BIRD logs were not read, so its sessions were not counted")
	}
	return countPeerStopsReadingSessions(zeLogs.Text, birdLogs.Text, address), nil
}

// countPeerStopsReadingSessions counts the sessions with address in ze's log
// and the times the protocol reached up in BIRD's.
func countPeerStopsReadingSessions(zeLogs, birdLogs, address string) peerStopsReadingSessions {
	return peerStopsReadingSessions{
		zeEstablished: logLinesWith(zeLogs, peerStopsReadingEstablishedLog, address),
		zeClosed:      logLinesWith(zeLogs, peerStopsReadingClosedLog, address),
		birdUp:        logLinesWith(birdLogs, peerStopsReadingBIRDUpLog),
	}
}

// openVerdict requires that the counts describe one session that is up now:
// both sides logged an establishment, and ze closed every session but the
// last. A count of zero on either side also means the scenario's logging is
// off, which the later counts would then read as "nothing happened".
func (s peerStopsReadingSessions) openVerdict() error {
	if s.zeEstablished == 0 {
		return fmt.Errorf("ze's log holds no %q line although BIRD reports the session Established: ze.conf no longer logs bgp.reactor.peer at info, so the checker cannot count ze's sessions", peerStopsReadingEstablishedLog)
	}
	if s.birdUp == 0 {
		return fmt.Errorf("BIRD's log holds no %q line although it reports the session Established: bird.conf no longer sets `debug { states }`, so the checker cannot count BIRD's sessions", peerStopsReadingBIRDUpLog)
	}
	if s.zeEstablished != s.zeClosed+1 {
		return fmt.Errorf("ze logged %d sessions established and %d closed before the freeze, want exactly one session still open", s.zeEstablished, s.zeClosed)
	}
	return nil
}

// returnVerdict requires that after holds exactly one more session than s on
// each side: the frozen one ended, one new session came up, and it is still
// up after its initial update.
func (s peerStopsReadingSessions) returnVerdict(after peerStopsReadingSessions) error {
	if after.zeClosed != s.zeClosed+1 {
		return fmt.Errorf("ze logged %d sessions closed since the freeze, want exactly one: the session that returned did not stay up", after.zeClosed-s.zeClosed)
	}
	if after.zeEstablished != s.zeEstablished+1 {
		return fmt.Errorf("ze logged %d sessions established since the freeze, want exactly one", after.zeEstablished-s.zeEstablished)
	}
	if after.birdUp != s.birdUp+1 {
		return fmt.Errorf("BIRD reached up %d times since the freeze, want exactly once: the session that returned did not stay up", after.birdUp-s.birdUp)
	}
	return nil
}

// waitPeerStopsReadingBIRDUp waits until BIRD has logged the protocol up at
// least want times.
func waitPeerStopsReadingBIRDUp(ctx context.Context, lab interoplab.CheckerLab, want int) error {
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     peerStopsReadingReturnWait,
		Interval:    2 * time.Second,
		Description: "BIRD's session with ze up again after the thaw",
	}, func(probeCtx context.Context) (string, error) {
		logs, logErr := lab.Logs(probeCtx, peerBIRD, peerStopsReadingLogLines)
		if logErr != nil {
			return "", logErr
		}
		if !logs.Available {
			return "", errors.New("BIRD logs were not read")
		}
		return logs.Text, nil
	}, func(logs string) bool {
		return logLinesWith(logs, peerStopsReadingBIRDUpLog) >= want
	})
	if err != nil {
		return fmt.Errorf("BIRD never logged %q a second time after the thaw, so the session did not come back: %w", peerStopsReadingBIRDUpLog, err)
	}
	return nil
}

// waitPeerStopsReadingBIRDRoutes waits until BIRD holds exactly want routes
// from ze.
func waitPeerStopsReadingBIRDRoutes(ctx context.Context, lab interoplab.CheckerLab, want int, timeout time.Duration) error {
	command := []string{cmdBirdc, peerStopsReadingBIRDRouteCount}
	pending := errors.New("no probe read BIRD's route count")
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     timeout,
		Interval:    2 * time.Second,
		Description: "BIRD's routes from ze",
	}, func(probeCtx context.Context) (string, error) {
		return lab.Query(probeCtx, peerBIRD, command, nil)
	}, func(output string) bool {
		held, countErr := birdShownRouteCount(output)
		if countErr != nil {
			pending = countErr
			return false
		}
		pending = fmt.Errorf("BIRD holds %d routes from ze", held)
		return held == want
	})
	if err != nil {
		return fmt.Errorf("BIRD never held exactly %d routes from ze (last reading: %w): %w", want, pending, err)
	}
	return nil
}

// birdShownRouteCount reads <shown> out of BIRD's "<shown> of <read> routes
// for <networks> networks in table <name>" line. An answer without that line
// is an error, never a count of zero.
func birdShownRouteCount(output string) (int, error) {
	for line := range strings.SplitSeq(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		if fields[1] != "of" {
			continue
		}
		if fields[3] != "routes" {
			continue
		}
		shown, err := strconv.Atoi(fields[0])
		if err != nil {
			return 0, fmt.Errorf("BIRD's route count line %q does not open with a number: %w", line, err)
		}
		return shown, nil
	}
	return 0, errors.New("BIRD's answer holds no route count line")
}

// bgpSocketUnsentOctets answers the largest send queue among the BGP
// connections in the kernel's TCP socket tables (/proc/net/tcp and tcp6):
// the octets ze wrote that the peer has not acknowledged. found is false
// when the tables list no BGP connection.
//
// A row is "<n>: <local>:<port> <remote>:<port> <state> <tx>:<rx> ...", every
// number in hexadecimal (tcp4_seq_show and tcp6_seq_show in the kernel). A
// listening socket has remote port zero and is not a connection.
func bgpSocketUnsentOctets(tables string) (octets uint64, found bool) {
	for line := range strings.SplitSeq(tables, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		localPort, localOK := socketTablePort(fields[1])
		remotePort, remoteOK := socketTablePort(fields[2])
		if !localOK {
			continue
		}
		if !remoteOK {
			continue
		}
		if remotePort == 0 {
			continue
		}
		if localPort != peerStopsReadingBGPPort && remotePort != peerStopsReadingBGPPort {
			continue
		}
		queue, _, queueOK := strings.Cut(fields[4], ":")
		if !queueOK {
			continue
		}
		unsent, err := strconv.ParseUint(queue, 16, 64)
		if err != nil {
			continue
		}
		found = true
		octets = max(octets, unsent)
	}
	return octets, found
}

// socketTablePort reads the port of an "<address>:<port>" column of the
// socket tables. The header row and any row that is not a socket answer
// false.
func socketTablePort(column string) (uint64, bool) {
	separator := strings.LastIndexByte(column, ':')
	if separator < 0 {
		return 0, false
	}
	port, err := strconv.ParseUint(column[separator+1:], 16, 16)
	if err != nil {
		return 0, false
	}
	return port, true
}

// logLinesWith counts the lines of logs that hold every needle.
func logLinesWith(logs string, needles ...string) int {
	count := 0
	for line := range strings.SplitSeq(logs, "\n") {
		if containsAll(line, needles) {
			count++
		}
	}
	return count
}
