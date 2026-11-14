// Design: docs/architecture/testing/interop.md -- requested End-of-RIB on a manual-eor peer.
// Related: check_manual_eor_predicate.go -- the pure predicates this checker decides with.
// Related: register_manual_eor.go -- the checker and process registrations.
// Related: check_rapid_reconnect_initial_sync.go -- the clear-and-wait model this checker follows.
package bgp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/interoplab"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// manualEORScenario proves, against FRR, that a peer configured
// `behavior { manual-eor true; }` receives no End-of-RIB nobody asked for, and
// receives the one a process asks for from inside the initial sync, after the
// route the same process announced first.
const manualEORScenario = "bgp-manual-eor-requested-frr"

const (
	// manualEORFirstPrefix is the route of the first session, on which the
	// process asks for no marker.
	manualEORFirstPrefix = "10.84.1.0/24"
	// manualEORSecondPrefix is the route of the second session, which the
	// process announces before it asks for the marker.
	manualEORSecondPrefix = "10.84.2.0/24"
	// manualEORRecordPath is the file in Ze's container where the process
	// writes one record per peer-UP event it handled.
	manualEORRecordPath = "/tmp/manual-eor-requests.json"
	// manualEORRequestBound stays under the engine's 5 s event delivery
	// timeout (defaultDeliveryTimeout, plugin/process/delivery.go). Past it the
	// engine stops waiting for the handler and starts the initial sync, so a
	// request still in flight could land after the sync: the process gives up
	// first, and the scenario fails instead of passing on the wrong timing.
	manualEORRequestBound = 4 * time.Second
	// manualEORHold is how long the checker watches for a marker that must not
	// come. A peer without manual-eor gets the sync's marker in the same drain
	// as the route that precedes it (sendInitialRoutes), so an automatic marker
	// written by mistake reaches FRR well inside this bound; the route decode
	// read in the same log is the proof that the log holds the session at all.
	manualEORHold = 10 * time.Second
	// manualEORSessionWait covers the first OPEN exchange, and after the clear
	// FRR's 1 s start timer, its 2 s connect timer (frr.conf) and Ze's redial.
	manualEORSessionWait = 90 * time.Second
	// manualEORLogWait bounds the wait for FRR to log a route and a marker.
	manualEORLogWait = 60 * time.Second
	// manualEOREvidenceBound bounds the FRR log read a failure carries, which
	// runs after a wait may have ended ctx.
	manualEOREvidenceBound = 20 * time.Second
	// manualEORStateEvent is the event kind the process subscribes to.
	manualEORStateEvent = "state"
)

// checkManualEORRequested judges two sessions. On the first the process
// announced a route and asked for nothing: FRR must decode the route and no
// marker, through a hold. The checker then clears the session. On the second
// the process announced a route and asked for the marker, both from inside
// its peer-UP handler: FRR must decode the route, then exactly one marker.
//
// Every judgement reads FRR's own state: its log (readManualEOR) and its
// established count (connectionsEstablished, which bgp_establish increments).
// The process's records in Ze's container prove each request was answered
// inside the peer-UP delivery, which the engine awaits before it starts the
// initial sync (deliverPeerState, internal/component/bgp/server/events.go).
func checkManualEORRequested(ctx context.Context, check *interoplab.CheckContext) error {
	if !check.Network.IPv4.IsValid() {
		return errors.New("the manual-eor scenario has no selected IPv4 network")
	}
	// Lab.Query runs the command as written, so the address is this run's
	// network rather than the 172.30.0.0/24 the scenario files spell.
	zeAddress := networkHostAddress(check.Network, zeHostNumber)
	assertion := 0
	fail := func(cause error) error {
		evidenceCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), manualEOREvidenceBound)
		defer cancel()
		return checkerFailure(evidenceCtx, check.Lab, manualEORScenario, assertion, withFRRLogTail(evidenceCtx, check.Lab, cause))
	}
	readLog := func(probeCtx context.Context) (string, error) {
		return check.Lab.Query(probeCtx, peerFRR, []string{cmdCat, frrLogPath}, nil)
	}

	// Assertion 1. The first session is Established, and it is FRR's first.
	assertion++
	if err := waitManualEORSession(ctx, check.Lab, zeAddress, 1); err != nil {
		return fail(err)
	}

	// Assertion 2. FRR decoded the first session's route.
	assertion++
	if _, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: manualEORLogWait, Interval: time.Second,
		Description: "FRR's decode of " + manualEORFirstPrefix + " from ze",
	}, readLog, func(log string) bool {
		return readManualEOR(log, zeAddress, manualEORFirstPrefix).routeSeen
	}); err != nil {
		return fail(err)
	}

	// Assertion 3. Through the hold, FRR decodes no End-of-RIB, and the session
	// it judged is still the first.
	assertion++
	if err := holdManualEOR(ctx); err != nil {
		return fail(err)
	}
	before, err := readLog(ctx)
	if err != nil {
		return fail(err)
	}
	if err := requireNoManualEORMarker(before, zeAddress, manualEORFirstPrefix); err != nil {
		return fail(err)
	}
	if err := requireManualEORGeneration(ctx, check.Lab, zeAddress, 1); err != nil {
		return fail(err)
	}

	// Assertion 4. The process handled exactly the first peer-UP, as planned.
	assertion++
	if err := readManualEORRequests(ctx, check.Lab, 1); err != nil {
		return fail(err)
	}

	// Assertion 5. FRR resets the session.
	assertion++
	if _, err := check.Lab.Exec(ctx, peerFRR, []string{cmdVtysh, "-c", "clear bgp " + zeAddress}, nil); err != nil {
		return fail(fmt.Errorf("clear FRR's first session with ze: %w", err))
	}

	// Assertion 6. The second session is Established, and it is FRR's second.
	assertion++
	if err := waitManualEORSession(ctx, check.Lab, zeAddress, 2); err != nil {
		return fail(err)
	}

	// Assertion 7. After the clear, FRR decodes the second route and a marker
	// after it.
	assertion++
	if _, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: manualEORLogWait, Interval: time.Second,
		Description: "FRR's decode of " + manualEORSecondPrefix + " then an End-of-RIB from ze",
	}, readLog, func(log string) bool {
		tail, tailErr := manualEORLogAfter(before, log)
		return tailErr == nil && manualEORRequestedReady(tail, zeAddress, manualEORSecondPrefix)
	}); err != nil {
		return fail(err)
	}

	// Assertion 8. Through the hold, that marker stays the only one, and the
	// session is still the second.
	assertion++
	if err := holdManualEOR(ctx); err != nil {
		return fail(err)
	}
	after, err := readLog(ctx)
	if err != nil {
		return fail(err)
	}
	tail, err := manualEORLogAfter(before, after)
	if err != nil {
		return fail(err)
	}
	if err := requireOneManualEORMarkerAfterRoute(tail, zeAddress, manualEORSecondPrefix); err != nil {
		return fail(err)
	}
	if err := requireManualEORGeneration(ctx, check.Lab, zeAddress, 2); err != nil {
		return fail(err)
	}

	// Assertion 9. The process asked for that marker inside the second
	// session's peer-UP delivery, so with the initial sync not yet started.
	assertion++
	if err := readManualEORRequests(ctx, check.Lab, 2); err != nil {
		return fail(err)
	}

	// Assertion 10. FRR holds the second session's route from ze.
	assertion++
	route := operation{kind: opFRRRoute, argument: manualEORSecondPrefix, timeout: manualEORLogWait}
	if err := runOperation(ctx, check.Network, check.Lab, &route); err != nil {
		return fail(err)
	}
	return nil
}

// holdManualEOR waits manualEORHold, or until ctx ends. The marker it guards
// against is cumulative in FRR's log, so the one read after the hold sees any
// marker written during it.
func holdManualEOR(ctx context.Context) error {
	hold := time.NewTimer(manualEORHold)
	defer hold.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-hold.C:
		return nil
	}
}

// waitManualEORSession waits until FRR reports its session with ze
// Established with at least session establishments, then requires exactly
// that many: a higher count is a session that ended without the checker's
// clear, and its process records would not match the plan.
func waitManualEORSession(ctx context.Context, lab interoplab.CheckerLab, zeAddress string, session uint64) error {
	var description textbuf.Buffer
	description.Str("FRR's session ").Uint(session).Str(" with ze Established")
	if _, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: manualEORSessionWait, Interval: time.Second,
		Description: description.String(),
	}, func(probeCtx context.Context) (uint64, error) {
		return queryFRRSessionGeneration(probeCtx, lab, zeAddress)
	}, func(generation uint64) bool {
		return generation >= session
	}); err != nil {
		return err
	}
	return requireManualEORGeneration(ctx, lab, zeAddress, session)
}

// requireManualEORGeneration requires FRR's session with ze Established with
// exactly session establishments.
func requireManualEORGeneration(ctx context.Context, lab interoplab.CheckerLab, zeAddress string, session uint64) error {
	generation, err := queryFRRSessionGeneration(ctx, lab, zeAddress)
	if err != nil {
		return err
	}
	if generation != session {
		return fmt.Errorf("FRR counts %d established sessions with ze, want %d: a session ended that the checker did not clear", generation, session)
	}
	return nil
}

// readManualEORRequests reads the process's records from Ze's container and
// requires want sessions handled as planned.
func readManualEORRequests(ctx context.Context, lab interoplab.CheckerLab, want int) error {
	output, err := lab.Query(ctx, "ze", []string{cmdCat, manualEORRecordPath}, nil)
	if err != nil {
		return fmt.Errorf("read the requesting process's records: %w", err)
	}
	return requireManualEORRequests(output, want)
}

// runManualEORProcess is the scenario's process. It subscribes to state
// events and, from inside its handler for each peer-UP, announces the
// session's route and, from the second session on, asks for the IPv4 unicast
// End-of-RIB (manualEORSessionPlan). The handler returns only once every
// request is answered, and the SDK acknowledges the delivery only after the
// handler returns (sdk.Plugin.OnEvent), so the requests land before the
// engine starts the initial sync: Peer.enterEstablished closes the session's
// queueing gate before it publishes Established, deliverPeerState waits for
// each process's acknowledgement, and only then does the FSM callback start
// the sync (the same chain fixture10EORDuringInitialSync relies on).
//
// Each handled event appends a record to manualEORRecordPath, which the
// checker reads. A failed request is recorded with its error, written to
// stderr with the ZE-OBSERVER-FAIL sentinel, and returned to the engine.
func runManualEORProcess(name string) error {
	plugin, err := sdk.NewFromEnv(name)
	if err != nil {
		return fmt.Errorf("connect %s to engine: %w", name, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var mu sync.Mutex
	var records []manualEORRequest
	plugin.SetStartupSubscriptions([]string{manualEORStateEvent}, nil, "")
	plugin.OnEvent(func(event string) error {
		peer, up, decodeErr := manualEORPeerUp(event)
		if decodeErr != nil {
			return decodeErr
		}
		if !up {
			return nil
		}
		mu.Lock()
		defer mu.Unlock()
		record := manualEORRequest{Session: len(records) + 1, Peer: peer.String()}
		requestErr := requestManualEOR(ctx, plugin, &record)
		if requestErr != nil {
			record.Error = requestErr.Error()
			// A failed stderr write loses only the sentinel: the record below
			// carries the same failure to the checker.
			var sentinel textbuf.Buffer
			_ = sentinel.Str("ZE-OBSERVER-FAIL: ").Str(record.Error).Str("\n").StdErr()
		}
		records = append(records, record)
		if writeErr := writeJSON(manualEORRecordPath, records); writeErr != nil {
			return errors.Join(requestErr, writeErr)
		}
		return requestErr
	})
	runErr := plugin.Run(ctx, &sdk.Registration{})
	if errors.Is(runErr, context.Canceled) {
		return nil
	}
	return runErr
}

// requestManualEOR makes one session's requests, each answered before the
// next is sent, inside manualEORRequestBound, and fills record with what it
// did and how long it took.
func requestManualEOR(ctx context.Context, plugin *sdk.Plugin, record *manualEORRequest) error {
	prefix, endOfRIB := manualEORSessionPlan(record.Session)
	record.Prefix = prefix
	requestCtx, cancel := context.WithTimeout(ctx, manualEORRequestBound)
	defer cancel()

	started := time.Now()
	announced, _, err := plugin.UpdateRoute(requestCtx, record.Peer, "update text origin igp nhop self nlri add "+prefix)
	if err != nil {
		return fmt.Errorf("announce %s to %s: %w", prefix, record.Peer, err)
	}
	if announced != 1 {
		return fmt.Errorf("announce %s to %s counted %d routes, want 1", prefix, record.Peer, announced)
	}
	if endOfRIB {
		if _, _, err := plugin.UpdateRoute(requestCtx, record.Peer, "update text nlri eor"); err != nil {
			return fmt.Errorf("ask for the IPv4 unicast End-of-RIB to %s: %w", record.Peer, err)
		}
		record.EndOfRIB = true
	}
	record.ElapsedMS = time.Since(started).Milliseconds()
	return nil
}

// manualEORPeerUp reports whether event is the state event saying a peer
// reached Established, and which peer. An event that does not decode is an
// error: it says nothing about whether a peer came up.
func manualEORPeerUp(event string) (netip.Addr, bool, error) {
	var envelope struct {
		BGP struct {
			Message struct {
				Type string `json:"type"`
			} `json:"message"`
			Peer struct {
				Remote struct {
					Address netip.Addr `json:"address"`
				} `json:"remote"`
			} `json:"peer"`
			State string `json:"state"`
		} `json:"bgp"`
	}
	if err := json.Unmarshal([]byte(event), &envelope); err != nil {
		return netip.Addr{}, false, fmt.Errorf("decode state event: %w", err)
	}
	if envelope.BGP.Message.Type != manualEORStateEvent {
		return netip.Addr{}, false, nil
	}
	if envelope.BGP.State != "up" {
		return netip.Addr{}, false, nil
	}
	if !envelope.BGP.Peer.Remote.Address.IsValid() {
		return netip.Addr{}, false, errors.New("peer-UP state event names no remote address")
	}
	return envelope.BGP.Peer.Remote.Address, true, nil
}
