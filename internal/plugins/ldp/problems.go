// Design: docs/architecture/ldp/mpls-ldp.md -- the problems LDP reports to the operator
// Related: register.go -- discoverOnInterface, processDiscoveryPacket and runSession report through these
// Related: discovery.go -- AdjacencyTable carries the per-interface Hello discard record
//
// LDP reports three operating problems: an interface whose Hellos fail to
// send, malformed Hellos discarded on an interface, and a session that ended
// without Ze stopping it. Each one is a WARN line carrying its doctor code and
// a raise on the daemon's report bus under the same code, through the SDK, so
// the raise reaches the daemon whether LDP runs in it or forked. Every raise
// goes through the plugin report queue (internal/component/plugin/issuequeue),
// so no discovery or session goroutine waits for the daemon. The two discovery
// problems happen per packet, so each is counted per interface and logged once
// per condition, never once per packet.
package ldp

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/component/plugin/issuequeue"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/opererr"
)

const (
	codeLDPHelloSendFailed = "doctor-ldp-hello-send-failed"
	codeLDPHelloDiscarded  = "doctor-ldp-hello-discarded"
	codeLDPSessionEnded    = "doctor-ldp-session-ended"
)

// showNeighborCommand lists LDP's adjacencies and sessions, which is where an
// operator checks which neighbor a discovery or session problem concerns.
const showNeighborCommand = "ze show ldp neighbor"

// helloDiscardQuiet is how long an interface must go with no malformed Hello
// before the next one is logged again. A neighbor that keeps sending them
// sends one per Hello interval (5 seconds by default), so a stream is one
// condition, logged once, and a stream that returns after this long is a new
// one. It is also how often a Hello send warning the daemon refused is raised
// again while the Hellos keep failing (helloSendHealth).
const helloDiscardQuiet = time.Minute

// errDiscoveryTruncated, errHelloShorterThanMessageID and errHelloBeyondDatagram
// are the three malformed discovery datagrams processDiscoveryPacket finds before
// a decoder runs.
var (
	errDiscoveryTruncated        = errors.New("the datagram is shorter than an LDP PDU header and message header")
	errHelloShorterThanMessageID = errors.New("the Hello message length is shorter than its 4-octet Message ID")
	errHelloBeyondDatagram       = errors.New("the Hello message length runs past the end of the datagram")
)

// exampleDoctorJSON is the doctor run every LDP code's description shows first.
const exampleDoctorJSON = "ze doctor --json"

// ldpDiagnosticCodes is the explanation metadata of the codes LDP owns. It is
// registered here rather than in diagnostic.builtinCodes, so removing the LDP
// plugin removes its codes (ai/rules/plugins.md).
var ldpDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:  codeLDPHelloSendFailed,
		Title: "LDP Hellos fail to send on an interface",
		Description: "LDP could not send its Basic Discovery Hello (RFC 5036 Section 2.4.1) on an interface, so its neighbors " +
			"there stop hearing this router and each adjacency on the interface expires once its hold time passes. " +
			"The usual causes are an interface that is down or has no IPv4 address. The warning clears at the next " +
			"Hello that sends, and ze_ldp_hello_send_errors_total counts every failed send. Remedy: check the " +
			"interface with ze show interface name <interface> detail.",
		Examples: []string{exampleDoctorJSON, "ze show diagnostic doctor-ldp-hello-send-failed"},
	},
	{
		Code:  codeLDPHelloDiscarded,
		Title: "LDP discarded a malformed Hello",
		Description: "A datagram received on the LDP discovery port of an interface was not a well-formed LDP Hello, " +
			"and RFC 5036 Section 3.5.1.2.1 discards it, so the neighbor that sent it forms no adjacency from it. " +
			"ze_ldp_hello_discards_total counts every discard; the first one after a quiet minute is logged and " +
			"raised. Remedy: list the adjacencies with ze show ldp neighbor and check the LDP software and " +
			"configuration of the neighbor on that interface.",
		Examples: []string{exampleDoctorJSON, "ze show diagnostic doctor-ldp-hello-discarded"},
	},
	{
		Code:  codeLDPSessionEnded,
		Title: "An LDP session ended",
		Description: "An LDP session ended without Ze stopping it: the neighbor closed the connection or sent a fatal " +
			"Notification, its KeepAlives stopped arriving, or it sent a message Ze could not accept. Ze withdrew the " +
			"labels that neighbor advertised and dials it again while its Hellos arrive. Remedy: check the session " +
			"with ze show ldp neighbor.",
		Examples: []string{exampleDoctorJSON, "ze show diagnostic doctor-ldp-session-ended"},
	},
}

// registerLDPDiagnosticCodes registers LDP's codes and marks each as an event
// code, which the recent-events doctor check declares. It runs once, from
// registerLDP, so a refusal is a Ze defect.
func registerLDPDiagnosticCodes() {
	for _, meta := range ldpDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: register LDP diagnostic code: " + err.Error())
		}
	}
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: codeLDPHelloSendFailed}); err != nil {
		panic("BUG: register LDP hello send event: " + err.Error())
	}
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: codeLDPHelloDiscarded}); err != nil {
		panic("BUG: register LDP hello discard event: " + err.Error())
	}
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: codeLDPSessionEnded}); err != nil {
		panic("BUG: register LDP session end event: " + err.Error())
	}
}

// issueQueue is LDP's report queue. runLDPEngine starts it before discovery
// and MUST stop it after the sessions have stopped. Until it starts the
// pointer holds nil, whose queue refuses each report with
// issuequeue.ErrNotStarted: only a test that drives discovery without the
// engine raises then.
var issueQueue atomic.Pointer[issuequeue.Queue]

// helloSendHealth is one interface's record of whether its Hellos fail to
// send. discoverOnInterface owns it and is its only caller, on one goroutine,
// so it holds no lock; the report queue's worker touches only the refusal the
// run's raises record. It is not safe for concurrent use otherwise.
type helloSendHealth struct {
	ifName string
	// now is the clock the raise retry window reads: time.Now, which a test
	// replaces.
	now func() time.Time
	// failures is the length of the current run of failed sends; zero while
	// Hellos send.
	failures uint64
	// raises records the current run's warning raises, so one the daemon
	// refused is raised again; a new record per run, nil before the first.
	raises *issuequeue.RunRaises
}

// newHelloSendHealth constructs the record of interface ifName.
func newHelloSendHealth(ifName string) *helloSendHealth {
	return &helloSendHealth{ifName: ifName, now: time.Now}
}

// observe records the outcome of one Hello send. RFC 5036 Section 2.4.1 sends
// a Hello every Hello interval, so a cause that persists fails on every tick:
// every failure counts, the first one after a success WARNs and raises the
// warning, and the first success after a failure clears it, so a later
// failure WARNs again. While the daemon has refused the run's latest raise, a
// failure at least helloDiscardQuiet after it raises again, so an interface
// broken for hours still reaches show warnings.
func (h *helloSendHealth) observe(log *slog.Logger, err error) {
	if err != nil {
		h.failed(log, err)
		return
	}
	h.recovered(log)
}

func (h *helloSendHealth) failed(log *slog.Logger, err error) {
	if m := ldpMetricsPtr.Load(); m != nil {
		m.helloSendErrors.With(h.ifName).Inc()
	}
	h.failures++
	if h.failures == 1 {
		h.raises = &issuequeue.RunRaises{}
		issue := h.problem(err)
		reportErr := h.raiseWarning(log, issue)
		log.LogAttrs(context.Background(), slog.LevelWarn, issue.Problem(),
			issue.LogAttrs(slog.String("code", codeLDPHelloSendFailed),
				slog.String("interface", h.ifName),
				issuequeue.Outcome(reportErr))...)
		return
	}
	// The run's WARN line is written; only the raise is repeated, and only
	// while the daemon has refused the latest one, at most once per window.
	if !h.raises.RetryDue(h.now(), helloDiscardQuiet) {
		return
	}
	if reportErr := h.raiseWarning(log, h.problem(err)); reportErr != nil {
		h.warnUnlisted(log, reportErr)
	}
}

// problem is the operator error of a failed Hello send.
func (h *helloSendHealth) problem(err error) *opererr.Error {
	return opererr.Wrap(err,
		"LDP cannot send its Hello on interface "+h.ifName,
		"The neighbors on this interface stop hearing this router, and each adjacency there expires once its "+
			"hold time passes; ze_ldp_hello_send_errors_total counts each failed send, and the next Hello that "+
			"sends clears this warning",
		"ze show interface name "+h.ifName+" detail")
}

// raiseWarning queues the run's warning and records the raise in the run, so a
// refusal, by the queue now or by the daemon later, arms the retry. It answers
// why the queue did not take the raise.
func (h *helloSendHealth) raiseWarning(log *slog.Logger, issue *opererr.Error) error {
	refused := h.raises.Offer(h.now())
	err := issueQueue.Load().RaiseWarning(codeLDPHelloSendFailed, h.ifName, issue.Problem(), issue.Fix(),
		func(err error) {
			refused(err)
			h.warnUnlisted(log, err)
		})
	if err != nil {
		refused(err)
	}
	return err
}

// warnUnlisted logs a raise of the Hello send warning that reached no report
// bus. It runs on the report queue's worker as well as on the discovery
// goroutine, and reads only ifName, which never changes.
func (h *helloSendHealth) warnUnlisted(log *slog.Logger, err error) {
	log.Warn("ldp: the hello send warning could not be raised, so show warnings does not list it; "+
		"it is raised again while the Hellos keep failing",
		"code", codeLDPHelloSendFailed, "interface", h.ifName, "error", err,
		"fix", "ze show interface name "+h.ifName+" detail")
}

func (h *helloSendHealth) recovered(log *slog.Logger) {
	failed := h.failures
	if failed == 0 {
		return
	}
	h.failures = 0
	h.raises = nil
	log.Info("ldp: hello sent again after failing", "interface", h.ifName, "failed-sends", failed)
	h.clearWarning(log)
}

// stop clears a warning still raised when discovery on the interface ends, so
// a removed interface leaves no warning behind. discoverOnInterface MUST call
// it once, when it returns.
func (h *helloSendHealth) stop(log *slog.Logger) {
	if h.failures == 0 {
		return
	}
	h.failures = 0
	h.raises = nil
	h.clearWarning(log)
}

// clearWarning queues the clear of the Hello send warning. A clear the queue
// or the daemon refuses leaves the warning listed, so it is logged at WARN
// under the same code.
func (h *helloSendHealth) clearWarning(log *slog.Logger) {
	if err := issueQueue.Load().ClearWarning(codeLDPHelloSendFailed, h.ifName, func(err error) {
		h.warnUncleared(log, err)
	}); err != nil {
		h.warnUncleared(log, err)
	}
}

// warnUncleared logs a clear of the Hello send warning that reached no report
// bus. Like warnUnlisted it runs on either goroutine.
func (h *helloSendHealth) warnUncleared(log *slog.Logger, err error) {
	log.Warn("ldp: the hello send warning could not be cleared, so show warnings may still list it",
		"code", codeLDPHelloSendFailed, "interface", h.ifName, "error", err, "fix", "ze show warnings")
}

// helloDiscardLog records, per interface, when discovery last discarded a
// malformed Hello, so a stream of them is logged once and a stream that
// returns after helloDiscardQuiet is logged again. It is keyed by configured
// interface name, and discoverOnInterface MUST forget its interface when it
// returns, which bounds the map by the configured interfaces. The zero value
// is ready to use. Safe for concurrent use.
type helloDiscardLog struct {
	mu   sync.Mutex
	last map[string]time.Time
}

// firstAfterQuiet records a discard on ifName at now and reports whether it
// is the first one after at least helloDiscardQuiet with none.
func (d *helloDiscardLog) firstAfterQuiet(ifName string, now time.Time) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.last == nil {
		d.last = make(map[string]time.Time)
	}
	previous, seen := d.last[ifName]
	d.last[ifName] = now
	return !seen || now.Sub(previous) >= helloDiscardQuiet
}

// forget drops ifName's record. discoverOnInterface MUST call it when it
// returns.
func (d *helloDiscardLog) forget(ifName string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.last, ifName)
}

// noteHelloDiscard counts a malformed Hello that processDiscoveryPacket
// discarded on ifName, and logs and raises the first one after a quiet period.
// RFC 5036 Section 3.5.1.2.1: "Malformed LDP PDUs or messages that are part of
// the LDP Discovery mechanism are handled by silently discarding them." Silent
// means no Notification to the sender; the record stays local.
func noteHelloDiscard(log *slog.Logger, adjTable *AdjacencyTable, ifName string, err error) {
	if m := ldpMetricsPtr.Load(); m != nil {
		m.helloDiscards.With(ifName).Inc()
	}
	if !adjTable.discards.firstAfterQuiet(ifName, time.Now()) {
		return
	}
	issue := opererr.Wrap(err,
		"LDP discarded a malformed Hello received on interface "+ifName,
		"RFC 5036 discards it, so the neighbor that sent it forms no adjacency from it; "+
			"ze_ldp_hello_discards_total counts each discard, and only the first after a quiet minute is logged",
		showNeighborCommand)
	reportErr := issueQueue.Load().RaiseError(codeLDPHelloDiscarded, ifName, issue.Problem(), issue.Fix(),
		func(err error) {
			log.Warn("ldp: a discarded Hello could not be raised on the daemon's report bus, so show errors does not list it",
				"code", codeLDPHelloDiscarded, "interface", ifName, "error", err, "fix", showNeighborCommand)
		})
	log.LogAttrs(context.Background(), slog.LevelWarn, issue.Problem(),
		issue.LogAttrs(slog.String("code", codeLDPHelloDiscarded),
			slog.String("interface", ifName),
			issuequeue.Outcome(reportErr))...)
}

// reportSessionEnd reports why the session runSession served ended. A session
// Ze stopped (an expired adjacency, a reload, a KeepAlive it could not send,
// the engine stopping) ended because the code that stopped it decided so, and
// that code logs its own reason, so this is an INFO line. Any other end lost
// the neighbor's labels without Ze choosing it, so it WARNs and raises an
// error event, once per session end: sessions are set up again on the Hello
// cadence, with the setup backoff, so this never runs per packet.
func reportSessionEnd(log *slog.Logger, sess *Session, err error) {
	peer := sess.PeerAddr().String()
	if sess.stopped() {
		log.Info("ldp: session closed", "peer", peer, "reason", err)
		return
	}
	issue := opererr.Wrap(err,
		"LDP session with "+peer+" ended",
		"Ze withdrew the labels this neighbor advertised, and dials it again while its Hellos arrive",
		showNeighborCommand)
	reportErr := issueQueue.Load().RaiseError(codeLDPSessionEnded, peer, issue.Problem(), issue.Fix(),
		func(err error) {
			log.Warn("ldp: an ended session could not be raised on the daemon's report bus, so show errors does not list it",
				"code", codeLDPSessionEnded, "peer", peer, "error", err, "fix", showNeighborCommand)
		})
	log.LogAttrs(context.Background(), slog.LevelWarn, issue.Problem(),
		issue.LogAttrs(slog.String("code", codeLDPSessionEnded),
			slog.String("peer", peer),
			issuequeue.Outcome(reportErr))...)
}
