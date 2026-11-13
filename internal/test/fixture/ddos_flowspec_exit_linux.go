//go:build linux

// Design: docs/guide/ddos-mitigation.md -- what each end of the FlowSpec plugin does to a live announcement
// Related: register_ddos_flowspec_exit_linux.go -- registers both drivers
// Related: plugin_fixture_06_ddos_linux.go -- the transit veth, the remote flood, fixture06ReloadDaemon

package fixture

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// ddosFlowspecExitVictim is the remote victim both FlowSpec exit tests flood.
// The FlowSpec responder defers a box-owned victim to on-host mitigation, so
// the victim sits behind the transit veth plugin/ddos-transit-forward-drop-setup
// builds, as for test/plugin/ddos-announce-rate-limit.ci. The detector names
// it as a host prefix (characterize.go characterizeTarget), so the rule the
// blackhole fallback announces is destination-ipv4 203.0.113.9/32 and nothing
// else.
const ddosFlowspecExitVictim = "203.0.113.9"

// ddosFlowspecExitRecord is the record key the FlowSpec plugin writes for its
// live announcement (internal/plugins/ddos/flowspec/register.go
// announcementKey). It is spelled here, not imported: a fixture links no
// plugin, and the key is a contract an operator reads in `ze data registered`.
const ddosFlowspecExitRecord = "meta/ddos-flowspec/announcement"

// ddosFlowspecParentRemovedConfig is what the operator commits: the .ci's file
// with the WHOLE `ddos` block deleted. The bgp block is byte-identical, attach
// block included, so the peer is neither torn down nor re-added
// (reactor/delivery_graph.go: a changed attach block travels through
// RemovePeer and AddPeer) and its session survives the reload: the withdrawal
// is the only UPDATE the reload can put on it. The probe's plugin block stays,
// or the reload would stop this driver before it read anything.
const ddosFlowspecParentRemovedConfig = `traffic {
	usage {
		enabled true
		track-ip true
		interfaces {
			interface zdd0p {
				enabled true
			}
			interface zdd0 {
				enabled true
			}
		}
	}
}

bgp {
	filter {
		import {
			final accept
		}
		export {
			final accept
		}
	}
	peer peer1 {
		connection {
			remote {
				ip 127.0.0.1
			}
			local {
				ip 127.0.0.2
				accept false
			}
		}
		session {
			asn {
				local 65000
				remote 65001
			}
			router-id 1.2.3.4
			family {
				ipv4/unicast { prefix { maximum 10000; } }
				ipv4/flow { prefix { maximum 10000; } }
			}
			capability {
				graceful-restart disable
			}
		}

		attach process ddos-flowspec {
			send [ update ]
		}
	}
}

plugin {
	external ddos-flowspec-parent-removed-probe {
		run "le test fixture plugin/ddos-flowspec-parent-removed-driver"
		encoder json
	}
}
`

// ddosFlowspecParentRemoved proves that deleting the whole `ddos` block puts
// the FlowSpec withdrawal on the wire during the reload.
//
// It floods the remote victim until `show ddos flowspec` reports the rule live
// AND the peer's updates-sent counter shows the announcement left, reloads the
// daemon with the parent block deleted, then waits for updates-sent to step
// again: nothing else can send this peer an UPDATE, because the bgp block did
// not change. The bytes of both UPDATEs are the peer's to assert, and which
// path withdrew is the .ci's log assertions.
//
// After the reload `show ddos flowspec` has no answer: the plugin that owns it
// is stopped by the same reload. The counter is read from the BGP engine,
// which the reload does not touch.
func ddosFlowspecParentRemoved(ctx context.Context, args []string) error {
	return p05Observe(ctx, args, "ddos-flowspec-parent-removed-probe", func(ctx context.Context, plugin *sdk.Plugin) error {
		if !waitPeerCounter03(ctx, plugin, "eor-sent", 1, 80) {
			return errors.New("the BGP session to the test peer never sent its End-of-RIB, so no FlowSpec rule could reach it")
		}
		established, err := peerCounter03(ctx, plugin, addrLoopback, "updates-sent")
		if err != nil {
			return fmt.Errorf("read updates-sent before the flood: %w", err)
		}

		// The steady flood, not a burst per poll round: the round's dispatches
		// would set the rate the detector grades (ddosFlowspecSteadyFlood).
		flood, err := startDdosFlowspecSteadyFlood(ddosFlowspecExitVictim)
		if err != nil {
			return err
		}
		defer flood.stop()

		announced := Poll(ctx, 200, 250*time.Millisecond, func() bool {
			if flood.exited() {
				return true
			}
			row, showErr := p05ShowMap(ctx, plugin, "show ddos flowspec")
			if showErr != nil || row["active"] != true {
				return false
			}
			updates, countErr := peerCounter03(ctx, plugin, addrLoopback, "updates-sent")
			return countErr == nil && updates > established
		})
		// The flood is stopped here, before the reload, as the burst-per-round
		// loop stopped it: the reload is then the only remover left.
		flood.stop()
		sent, floodErr := flood.result()
		if floodErr != nil {
			return floodErr
		}
		if !announced {
			return fmt.Errorf("ddos-flowspec announced nothing to the peer for remote victim %s within 50s of an unbroken flood (sent %d packets)",
				ddosFlowspecExitVictim, sent)
		}
		before, err := peerCounter03(ctx, plugin, addrLoopback, "updates-sent")
		if err != nil {
			return fmt.Errorf("read updates-sent after the announcement: %w", err)
		}
		if err := ddosFlowspecReport("FLOWSPEC-ANNOUNCED " + ddosFlowspecExitVictim); err != nil {
			return err
		}

		if err := fixture06ReloadDaemon(ddosFlowspecParentRemovedConfig); err != nil {
			return err
		}

		after := before
		withdrawn := Poll(ctx, 160, 250*time.Millisecond, func() bool {
			updates, countErr := peerCounter03(ctx, plugin, addrLoopback, "updates-sent")
			if countErr != nil {
				return false
			}
			after = updates
			return updates > before
		})
		if !withdrawn {
			return fmt.Errorf("deleting the parent ddos block sent the peer no UPDATE within 40s (updates-sent stayed at %d): the FlowSpec rule for %s is left at the peer with no plugin to withdraw it",
				after, ddosFlowspecExitVictim)
		}
		return ddosFlowspecReport("FLOWSPEC-WITHDRAWAL-SENT " + ddosFlowspecExitVictim)
	})
}

// ddosFlowspecOrphanFlood floods the remote victim until the daemon holds the
// FlowSpec plugin's record of the announcement, read back through the daemon
// (`show data cat`, internal/component/config/storage/cli dataCat).
//
// The record is the state the orphan test needs, and `show ddos flowspec` is
// not a barrier for it: the responder publishes the live rule before it writes
// the record (responder.go announce), so a kill between the two would leave no
// record. The answer is read until it names the victim, never for a time.
//
// The flood runs on its own worker at a steady pace (ddosFlowspecSteadyFlood)
// while this goroutine polls, so a slow `ze cli` round cannot lower the rate
// the detector grades.
func ddosFlowspecOrphanFlood(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("usage: plugin/ddos-flowspec-orphan-flood")
	}
	flood, err := startDdosFlowspecSteadyFlood(ddosFlowspecExitVictim)
	if err != nil {
		return err
	}
	defer flood.stop()

	pollCtx, cancel := context.WithTimeout(ctx, ddosFlowspecOrphanFloodBudget)
	defer cancel()

	command := "show data cat " + ddosFlowspecExitRecord
	want := ddosFlowspecExitVictim + "/32"
	environment := os.Environ()
	var last awaitAnswerRun
	recorded := Poll(pollCtx, 200, 250*time.Millisecond, func() bool {
		if flood.exited() {
			return true
		}
		last.code, last.stdout, last.stderr, last.err = runCaptured(pollCtx, environment, "", "ze", "cli", "-c", command)
		if last.err != nil {
			return false
		}
		return last.code == 0 && strings.Contains(last.stdout, want)
	})
	flood.stop()
	sent, floodErr := flood.result()
	if floodErr != nil {
		return floodErr
	}
	if !recorded {
		if last.err != nil {
			return fmt.Errorf("the daemon never held a FlowSpec record for %s within %s of an unbroken flood (sent %d packets); the last `ze cli -c %s` did not start: %w",
				want, ddosFlowspecOrphanFloodBudget, sent, command, last.err)
		}
		return fmt.Errorf("the daemon never held a FlowSpec record for %s within %s of an unbroken flood (sent %d packets); the last `ze cli -c %s` exited %d, stdout %q, stderr %q",
			want, ddosFlowspecOrphanFloodBudget, sent, command, last.code, awaitAnswerTail(last.stdout), awaitAnswerTail(last.stderr))
	}
	return ddosFlowspecReport("FLOWSPEC-RECORDED " + want)
}

// The steady flood's pace: one burst of ddosFlowspecFloodBurst packets every
// ddosFlowspecFloodInterval, 16,000 packets a second. Both FlowSpec drivers
// use it.
//
// It has to be rated CRITICAL, because both .ci files set characterize-enable
// false: no AttackCharacterized ever arrives, so the only announce path is the
// blackhole fallback, which responder.go onDetected takes for
// ddosevent.SeverityCritical alone. A medium or high detection logs "awaiting
// characterization before announcing" and announces nothing.
//
// The grade is set ONCE, when the attack activates, and never revised:
// detector.go onAttackStart grades peakRxPps against the threshold
// (ddosevent.GradeSeverity, critical at 5x or more). peakRxPps is the highest
// one-second sample above the threshold since the last clear (applyTick, with
// check-interval 1 one sample per evaluation, intervalPeak). The threshold is
// max(p99 x threshold-multiplier, absolute-floor) (detect/baseline.go
// Threshold), and an above-threshold sample never enters the baseline, so on
// the quiet pre-flood baseline it is the .ci's absolute-floor of 100 pps.
//
// The .ci files set confirm-duration 2, so activation needs two consecutive
// samples above the floor, and the second is a whole second inside the flood.
// Critical needs a peak of 500 pps, and that whole second holds about 16,000:
// one-eighth of one burst is enough. The first sample crossing the floor can
// hold any fragment of a burst, which is why one sample alone (confirm-duration
// 1, floor 1000) was rated high under load.
//
// The pace used to be one burst per poll round, so a round that ran a whole
// `ze cli` process or several dispatches set the rate the detector graded.
const (
	ddosFlowspecFloodBurst    = 4000
	ddosFlowspecFloodInterval = 250 * time.Millisecond
)

// ddosFlowspecOrphanFloodBudget bounds the wait for the record, the CLI run in
// flight included, so the window the failure message names is the one the
// helper kept.
const ddosFlowspecOrphanFloodBudget = 50 * time.Second

// ddosFlowspecSteadyFlood floods one remote victim at the steady pace above, on
// a worker goroutine of its own that owns the socket: the socket is opened for
// it, and only the worker writes and closes it.
//
// Lifecycle: startDdosFlowspecSteadyFlood starts the worker, stop ends it and
// waits for it to exit, and result reads what it sent and why it ended. The
// caller MUST call stop on every path (a deferred stop covers the early ones),
// and stop is idempotent so the explicit call and the deferred one can both
// run. result MUST be called only after stop returned.
//
// Safe for concurrent use in that order only: sent and err are written by the
// worker and read after done is closed, which orders the two.
type ddosFlowspecSteadyFlood struct {
	victim   string
	quit     chan struct{}
	done     chan struct{}
	stopOnce sync.Once
	sent     int   // packets the socket accepted; read only after done
	err      error // the send error that ended the worker; read only after done
}

// startDdosFlowspecSteadyFlood opens the flood socket and starts the worker.
func startDdosFlowspecSteadyFlood(victim string) (*ddosFlowspecSteadyFlood, error) {
	socket, err := fixture06OpenRemoteFlood(victim)
	if err != nil {
		return nil, err
	}
	f := &ddosFlowspecSteadyFlood{victim: victim, quit: make(chan struct{}), done: make(chan struct{})}
	go f.run(socket)
	return f, nil
}

// run is the worker: a burst, then a wait for the next period or for stop. A
// send error ends it, and result hands that error to the helper.
func (f *ddosFlowspecSteadyFlood) run(socket *fixture06Flood) {
	defer close(f.done)
	defer socket.Close()
	ticker := time.NewTicker(ddosFlowspecFloodInterval)
	defer ticker.Stop()
	// Bounded by quit: the loop ends when stop closes it, or at a send error.
	for {
		if err := f.burst(socket); err != nil {
			f.err = err
			return
		}
		select {
		case <-f.quit:
			return
		case <-ticker.C:
		}
	}
}

// burst writes one burst and counts each packet the socket accepted.
func (f *ddosFlowspecSteadyFlood) burst(socket *fixture06Flood) error {
	for range ddosFlowspecFloodBurst {
		if _, err := socket.socket.Write(socket.payload); err != nil {
			return fmt.Errorf("the flood to %s stopped after %d packets: %w", f.victim, f.sent, err)
		}
		f.sent++
	}
	return nil
}

// exited reports whether the worker has already ended on its own, which only a
// send error does, so the poll stops waiting on a flood that is not running.
func (f *ddosFlowspecSteadyFlood) exited() bool {
	select {
	case <-f.done:
		return true
	default:
		return false
	}
}

// stop ends the worker and waits for it to exit. Idempotent.
func (f *ddosFlowspecSteadyFlood) stop() {
	f.stopOnce.Do(func() { close(f.quit) })
	<-f.done
}

// result is what the worker sent, and the send error that ended it, nil when
// stop ended it. MUST be called after stop.
func (f *ddosFlowspecSteadyFlood) result() (sent int, err error) {
	return f.sent, f.err
}

// ddosFlowspecOrphanCode is the doctor code of a FlowSpec rule no responder
// owns (internal/core/diagnostic CodeDoctorDDoSFlowspecOrphanAnnouncement),
// spelled here for the same reason as ddosFlowspecExitRecord: it is what an
// operator reads in `ze doctor`.
const ddosFlowspecOrphanCode = "doctor-ddos-flowspec-orphan-announcement"

// ddosFlowspecOrphanCleared waits until the fix `ze doctor` printed has
// cleared the finding: in one round, the daemon answers `show doctor` with no
// ddos-flowspec orphan finding AND answers `show data cat` for the record with
// the store's own refusal for a key that does not exist
// (ddosFlowspecRecordMissing). Both are asked in the same round, and the doctor
// answer must exit zero, so a daemon that is down never reads as cleared. Any
// other failed read of the record (a refused command, a daemon that is down, a
// store error) is not the record's absence, so it reads as not cleared yet.
//
// The restart the fix runs answers once the new process reached its running
// stage, and the plugin withdraws the record from OnStarted after that, so the
// clearing is a state to wait for, not one the restart's answer orders.
func ddosFlowspecOrphanCleared(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("usage: plugin/ddos-flowspec-orphan-cleared")
	}
	environment := os.Environ()
	doctorCommand := "show doctor | json"
	recordCommand := "show data cat " + ddosFlowspecExitRecord
	var doctor, record awaitAnswerRun
	cleared := Poll(ctx, 120, 250*time.Millisecond, func() bool {
		doctor.code, doctor.stdout, doctor.stderr, doctor.err = runCaptured(ctx, environment, "", "ze", "cli", "-c", doctorCommand)
		if doctor.err != nil {
			return false
		}
		if doctor.code != 0 {
			return false
		}
		if strings.Contains(doctor.stdout, ddosFlowspecOrphanCode) {
			return false
		}
		record.code, record.stdout, record.stderr, record.err = runCaptured(ctx, environment, "", "ze", "cli", "-c", recordCommand)
		if record.err != nil {
			return false
		}
		if record.code == 0 {
			return false
		}
		return ddosFlowspecRecordMissing(record.stdout) || ddosFlowspecRecordMissing(record.stderr)
	})
	if !cleared {
		// Each start error is wrapped beside the outcome, and errors.Join drops
		// the nil ones, so a command that started names no start error.
		var doctorStart, recordStart error
		if doctor.err != nil {
			doctorStart = fmt.Errorf("the last `ze cli -c %s` did not start: %w", doctorCommand, doctor.err)
		}
		if record.err != nil {
			recordStart = fmt.Errorf("the last `ze cli -c %s` did not start: %w", recordCommand, record.err)
		}
		return errors.Join(fmt.Errorf("the fix ze doctor printed did not clear %s within 30s (cleared means a doctor answer with no such finding, and the store's not-exist refusal for the record); the last `ze cli -c %s` exited %d, stdout %q; the last `ze cli -c %s` exited %d, stdout %q, stderr %q",
			ddosFlowspecOrphanCode, doctorCommand, doctor.code, awaitAnswerTail(doctor.stdout),
			recordCommand, record.code, awaitAnswerTail(record.stdout), awaitAnswerTail(record.stderr)),
			doctorStart, recordStart)
	}
	return ddosFlowspecReport("FLOWSPEC-REPORT-CLEARED " + ddosFlowspecExitVictim + "/32")
}

// ddosFlowspecRecordMissing reports whether one stream of a `show data cat`
// answer carries the store's own refusal for a record key that does not
// exist. No other failure counts: a refused command, a daemon that is down and
// a store error each answer with another text.
//
// catAnswer (internal/component/config/storage/cli storage_data.go) answers a
// failed ReadKey with storageRefusal, one line, "error: " and the storage
// error. The store words a missing key in one of two ways:
//
//   - the live tree store (option=storage:value=disk): openNode
//     (storage/tree.go) wraps ENOENT as "storage node <path>: no such file or
//     directory". <path> ends in the key, or in the first of its folders that
//     is absent, because treeEncoding.walk opens each folder the same way;
//   - a blob store: zefs BlobStore.readFile answers an fs.PathError,
//     "read <key>: file does not exist".
//
// The SSH route writes the refusal on stderr as the command wrote it
// (ssh/answer.go writeExecError), and the client may trim its "error: "
// prefix, so both streams are read and the prefix is optional.
func ddosFlowspecRecordMissing(stream string) bool {
	const treeMissing = ": no such file or directory"
	blobMissing := "read " + ddosFlowspecExitRecord + ": file does not exist"
	for line := range strings.Lines(stream) {
		line = strings.TrimPrefix(strings.TrimSpace(line), "error: ")
		if line == blobMissing {
			return true
		}
		node, isNode := strings.CutPrefix(line, "storage node ")
		if !isNode {
			continue
		}
		path, isMissing := strings.CutSuffix(node, treeMissing)
		if !isMissing {
			continue
		}
		if ddosFlowspecRecordPath(path) {
			return true
		}
	}
	return false
}

// ddosFlowspecRecordPath reports whether path, a tree store node, is the
// record key or one of its folders: the key itself, meta/ddos-flowspec or
// meta, under the store root.
func ddosFlowspecRecordPath(path string) bool {
	// Bounded by the key's depth: each round drops its last segment.
	key := ddosFlowspecExitRecord
	for {
		if strings.HasSuffix(path, "/"+key) {
			return true
		}
		cut := strings.LastIndexByte(key, '/')
		if cut < 0 {
			return false
		}
		key = key[:cut]
	}
}

// ddosFlowspecReport writes one marker line on stderr. It goes to stderr: a
// plugin's stdout is the protocol channel the encoder owns.
func ddosFlowspecReport(marker string) error {
	var tb textbuf.Buffer
	if _, err := fmt.Fprintln(os.Stderr, tb.Str(marker).String()); err != nil {
		return fmt.Errorf("report %s: %w", marker, err)
	}
	return nil
}
