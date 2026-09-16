// Design: docs/architecture/diagnostics/path-mtu.md -- the search
// Related: mtu.go (the run that calls it), internal/core/probe (the DF socket,
// the error queue and the kernel's estimate the wire prober is bound to)
// RFC: rfc/short/rfc792.md -- the ICMP echo every probe is. RFC 1191 (Section 5,
// a zero Next-Hop MTU is the search signal), RFC 4821 (Section 7.6.4, a lost
// probe moves no bound) and RFC 8899 (Section 5.1.2, MAX_PROBES is 3) are
// quoted above the code that enforces them and are not enrolled (owner
// decision, 2026-09-11, docs/architecture/diagnostics/active-probes.md).
//
// The search measures the path MTU to ONE target: a DF sanity gate, then a
// router's reported figure confirmed on the wire, then the candidate ladder
// bisected by index, then the number line bisected. It is a pure algorithm
// over a prober, so the tests drive it over a model of a path, and the
// wire prober at the end of this file is the one binding to the probe
// layer.

package cmd

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"strconv"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/core/ikeprobe"
	"github.com/ze-software/ze/internal/core/probe"
)

// The sizes and the bounds of one search. A payload is the octets behind
// the ICMP header; a wire size is the whole datagram. Every loop in the
// search is bounded by one of these, and runProbeBudget is their sum.
const (
	// payloadMin is the floor probe: 68 octets of payload, the smallest
	// datagram the ported tool sends. A path that answers no probe of this
	// size is unmeasurable.
	payloadMin = 68
	// payloadMax is the largest payload the search sends, the ported tool's
	// MAX_PAYLOAD. Every wire size stays inside the 16-bit IP length.
	payloadMax = 65000
	// sanityPayload is the DF sanity gate: no interface carries a 10000-octet
	// payload unfragmented, so a reply to it means DF is not honored and no
	// figure this run produces means anything.
	sanityPayload = 10000
	// icmpOverheadIPv4 and icmpOverheadIPv6 convert a payload to a wire size:
	// the IP header (20 or 40) plus the 8-octet ICMP echo header.
	icmpOverheadIPv4 = 20 + 8
	icmpOverheadIPv6 = 40 + 8
	// probesPerSizeMax is how many times one size is sent before a silence
	// is believed. RFC 8899 Section 5.1.2: "MAX_PROBES represents the limit
	// for the number of consecutive probe attempts of any size. Search
	// algorithms benefit from a MAX_PROBES value greater than 1 because
	// this can provide robustness to isolated packet loss. The default value
	// of MAX_PROBES is 3." The literal is pinned by
	// TestSearchDoesNotMoveBoundsOnSingleSilence.
	probesPerSizeMax = 3
	// reportedFollowsMax caps how many new reported figures are followed: a
	// router that hands back a different number on every confirm is
	// followed this many times, then the search takes over.
	reportedFollowsMax = 6
	// probeWait bounds the wait for one probe's answer.
	probeWait = 2 * time.Second
	// ladderAttemptsMax is the attempts an index bisection of the candidate
	// ladder can take: bits.Len(len(candidateLadder)) = bits.Len(31) = 5.
	// TestSearchBudgetIsTheComputedWorstCase re-derives it.
	ladderAttemptsMax = 5
	// bisectionAttemptsMax is the attempts a bisection of the number line
	// can take: bits.Len(payloadMax + 1) = bits.Len(65001) = 16.
	bisectionAttemptsMax = 16
	// runProbeBudget is the worst case of one search, every attempt at its
	// probesPerSizeMax silences: the gate (1 attempt), the follows (2
	// attempts each, size and size + 1), the ladder, the floor probe (1),
	// the contradiction re-test (1) and the number line. 3 * (1 + 12 + 5 +
	// 1 + 1 + 16) = 108 probes, 216 seconds at probeWait with no answer.
	// R-2: the worst case is computable, and this is it.
	runProbeBudget = probesPerSizeMax * (1 + 2*reportedFollowsMax + ladderAttemptsMax + 1 + 1 + bisectionAttemptsMax)
	// readErrorsMax bounds consecutive read errors that turn out to quote
	// another flow's probe, so a descriptor failing every read cannot hold
	// a probe past its wait.
	readErrorsMax = 32
)

// The bounds of the IKE prober. Each exchange holds the SA's request window
// (RFC 7296 Section 2.3: "An IKE endpoint MUST wait for a response to each
// of its messages before sending a subsequent message"), so no DPD, Delete
// or rekey leaves while one is out: exchanges, not octets, are what a run
// spends on a live tunnel.
const (
	// ikeWireMax is the largest datagram the IKE prober asks for. RFC 7296
	// Section 2: "All IKEv2 implementations MUST be able to send, receive,
	// and process IKE messages that are up to 1280 octets long, and they
	// SHOULD be able to send, receive, and process messages that are up to
	// 3000 octets long." The engine refuses a size above its interface MTU
	// by name, so between the two every probe stays under min(interface MTU,
	// 3000) (AC-6). An ICMP figure above it is not offered at all.
	ikeWireMax = 3000
	// ikeExchangesPerRunMax bounds the exchanges one run spends on one
	// tunnel: the confirm at the ICMP figure and, when it is too big, the
	// ladder below it. The engine bounds how long ONE exchange stays
	// outstanding (its retransmit budget); this bounds how long DPD is held
	// off across the run. The Boundary row: 1..16 exchanges per run;
	// TestIKEExchangeBudgetBoundary pins the 17th refused.
	ikeExchangesPerRunMax = 16
	// ikeProbeStopAtFirstSilence: an sa-failed outcome ends the IKE attempt
	// for that tunnel at once, and no smaller size is asked. On a path that
	// drops IP fragments a probe can take the tunnel down the way any IKE
	// request larger than the path would: the DF copy is too big, the
	// DF-clear copies are fragmented and lost, and after the full retransmit
	// budget the SA is deemed failed (RFC 7296 Section 2.1) and re-established
	// by the owner loop. The engine gains no probe-aware exit, so the
	// mitigation is here: the IKE prober is asked only at or below the ICMP
	// figure, and the first size that stays unanswered is the last one asked,
	// so a run fails a tunnel at most once. Read by ikeProber only; the ICMP
	// path has no failed SA to stop on. Owner ruling (a), 2026-09-16.
	ikeProbeStopAtFirstSilence = true
)

// candidateLadder is the 31 wire sizes real paths cluster on, largest
// first, tried inside the bracket before the number line. It is the ported
// tool's list, tuned on degraded customer paths; RFC 1191 Section 7 calls
// its own eleven plateaus "an implementation suggestion, NOT a
// specification or requirement" (owner decision, 2026-09-11).
var candidateLadder = [31]int{
	1500, 1492, 1480, 1476, 1472, 1468, 1462, 1460, 1458, 1454, 1452, 1450,
	1442, 1438, 1436, 1430, 1428, 1420, 1412, 1400, 1398, 1380, 1358, 1350,
	1300, 1280, 1260, 1200, 1100, 1024, 576,
}

// errProbeBudget is answered when a search asks for a probe past
// runProbeBudget. The loops are each bounded and the budget is their sum,
// so reaching it is a defect in the arithmetic above, reported rather than
// probed through.
var errProbeBudget = errors.New("mtu: the search asked for more probes than its budget")

// errIKEExchangeBudget ends the IKE attempt when the search asks for an
// exchange past ikeExchangesPerRunMax: the ICMP figure stands, unconfirmed,
// and the row carries this text.
var errIKEExchangeBudget = errors.New("the budget of " + strconv.Itoa(ikeExchangesPerRunMax) + " IKE exchanges was spent before the search converged")

// probeOutcome is what one DF probe came back with. Zero is unspecified,
// never an answer.
type probeOutcome uint8

const (
	probeOutcomeUnspecified probeOutcome = iota
	// probeReplied: an echo reply matched the probe. The size passes.
	probeReplied
	// probeRefusedReported: the probe was refused and the refusal carries a
	// usable next-hop MTU (probe.ErrQueueMTUReported).
	probeRefusedReported
	// probeRefusedUnreported: the probe was refused and the refusal carries
	// no usable value (probe.ErrQueueMTUUnreported). RFC 1191 Section 5 makes
	// a zero Next-Hop MTU an unmodified router's signal to search, so the
	// refusal is definitive and the value is nothing.
	probeRefusedUnreported
	// probeSilent: nothing came back inside probeWait.
	probeSilent
	// probeTooBig: the padded exchange was answered only on the copy sent
	// with Don't Fragment clear (ikeprobe.OutcomeTooBig). Unlike a refusal
	// off the error queue it can be a lost DF copy (a drop, or a peer in
	// IKE_REKEYED dropping the request), so the search retries it as it
	// retries a silence before it believes it (AC-3).
	probeTooBig
)

// String answers the wire spelling of the outcome, for the detail view. It
// is written into the payload and never compared.
func (o probeOutcome) String() string {
	switch o {
	case probeReplied:
		return "replied"
	case probeRefusedReported:
		return "refused-reported"
	case probeRefusedUnreported:
		return "refused-unreported"
	case probeSilent:
		return "silent"
	case probeTooBig:
		return "too-big"
	default:
		panic("BUG: probeOutcome written to the payload before it was set")
	}
}

// probeAnswer is one probe's answer. local is true when the refusal came
// from this host's kernel rather than a router, and mtu is the wire MTU a
// refusal reported, meaningful only under probeRefusedReported.
type probeAnswer struct {
	outcome probeOutcome
	local   bool
	mtu     uint32
}

// prober sends one DF probe carrying payload octets to the search's target
// and answers what came back. An error is a socket failure, never silence.
type prober interface {
	probe(ctx context.Context, payload int) (probeAnswer, error)
}

// searchMethod is how the answer was found: the seven meanings the ported
// tool prints. Zero is unspecified, never a method.
type searchMethod uint8

const (
	searchMethodUnspecified searchMethod = iota
	// searchViaICMP: a router reported the value and it was confirmed.
	searchViaICMP
	// searchViaLocalIfaceMTU: this host's kernel reported the value at send
	// and it was confirmed; the path is no tighter than the interface.
	searchViaLocalIfaceMTU
	// searchICMPUnderReported: a router's figure passed at one octet more,
	// so the search found the real value above it.
	searchICMPUnderReported
	// searchICMPOverReported: a router's figure failed at its own size, so
	// the search found the real value below it.
	searchICMPOverReported
	// searchICMPFiltered: no router ever reported a value; the ladder and
	// the number line found it.
	searchICMPFiltered
	// searchICMPKeptChanging: a new figure came back on every confirm until
	// the follow cap, and the search found it instead.
	searchICMPKeptChanging
	// searchForcedFullSearch: exhaustive discarded every report and the whole
	// range was searched with the cache bypassed.
	searchForcedFullSearch
)

func (m searchMethod) String() string {
	switch m {
	case searchViaICMP:
		return "via ICMP"
	case searchViaLocalIfaceMTU:
		return "via local iface MTU"
	case searchICMPUnderReported:
		return "ICMP under-reported"
	case searchICMPOverReported:
		return "ICMP over-reported"
	case searchICMPFiltered:
		return "ICMP filtered"
	case searchICMPKeptChanging:
		return "ICMP kept changing"
	case searchForcedFullSearch:
		return "forced full search"
	case searchMethodUnspecified:
		panic("BUG: searchMethod.String on an unspecified method")
	default:
		panic("BUG: searchMethod.String on an unknown method")
	}
}

// searchOutcome is what one search ended as. Zero is unspecified, never an
// outcome.
type searchOutcome uint8

const (
	searchOutcomeUnspecified searchOutcome = iota
	// searchMeasured: pathMTU and method hold the answer.
	searchMeasured
	// searchUnmeasurable: not even the floor probe was answered (AC-7).
	searchUnmeasurable
	// searchDFGateFailed: a 10000-octet payload was answered, so DF is not
	// honored and no figure is believed.
	searchDFGateFailed
)

func (o searchOutcome) String() string {
	switch o {
	case searchMeasured:
		return "measured"
	case searchUnmeasurable:
		return "unmeasurable"
	case searchDFGateFailed:
		return "df-gate-failed"
	case searchOutcomeUnspecified:
		panic("BUG: searchOutcome.String on an unspecified outcome")
	default:
		panic("BUG: searchOutcome.String on an unknown outcome")
	}
}

// probeRecord is one probe sent and what it answered, for the detail view.
type probeRecord struct {
	payload int
	answer  probeAnswer
}

// searchResult is what one search reports. pathMTU (wire octets) and
// method hold a value only under searchMeasured; probes counts every probe
// sent; lossy is set when two probes contradicted each other and the
// bracket was re-tested; records is every probe, bounded by runProbeBudget.
type searchResult struct {
	outcome searchOutcome
	pathMTU uint32
	method  searchMethod
	probes  int
	lossy   bool
	records []probeRecord
}

// icmpOverhead is the ICMP echo overhead for the family of target.
func icmpOverhead(target netip.Addr) int {
	if target.Is6() {
		return icmpOverheadIPv6
	}
	return icmpOverheadIPv4
}

// pathSearch is the state of one search: the bracket in payload octets
// (low, the largest that passed, and high, the smallest that failed, each 0
// while unknown) and what the last probe's refusal reported.
type pathSearch struct {
	ctx           context.Context
	prober        prober
	overhead      int
	low, high     int
	reported      uint32
	reportedLocal bool
	result        searchResult
}

// searchPathMTU measures the path MTU to target through p. exhaustive discards
// the reported figure and the bracket and searches the full range (AC-11);
// the caller opened p in probe.DFBypassCache for it.
func searchPathMTU(ctx context.Context, p prober, target netip.Addr, exhaustive bool) (searchResult, error) {
	s := pathSearch{ctx: ctx, prober: p, overhead: icmpOverhead(target)}

	// The DF sanity gate: a payload no interface carries MUST be refused,
	// or DF is not honored and every figure below would be the size of a
	// fragment. The refusal doubles as the first path-MTU query.
	passed, err := s.attempt(sanityPayload)
	if err != nil {
		return s.result, err
	}
	if passed {
		s.result.outcome = searchDFGateFailed
		return s.result, nil
	}
	origin := searchViaICMP
	if s.reportedLocal {
		origin = searchViaLocalIfaceMTU
	}
	reported := s.reported
	method := searchICMPFiltered
	if exhaustive {
		reported = 0
		s.low, s.high = 0, payloadMax+1
		method = searchForcedFullSearch
	}

	// Follow the reported figure: a later hop can clamp further, so each
	// failed confirm that hands back a new number is followed, at most
	// reportedFollowsMax times.
	follows := 0
	for reported != 0 && follows < reportedFollowsMax {
		follows++
		verdict, confirmErr := s.confirm(reported)
		if confirmErr != nil {
			return s.result, confirmErr
		}
		if verdict == confirmConfirmed {
			s.result.outcome = searchMeasured
			s.result.pathMTU = reported
			s.result.method = origin
			return s.result, nil
		}
		if s.reported != 0 {
			if s.reported != reported {
				// A new figure came off the wire whatever the first one was.
				reported = s.reported
				origin = searchViaICMP
				continue
			}
		}
		// The figure was wrong and no better one came back. A figure this
		// host's kernel reported is not an ICMP report, so a search that
		// follows it is "filtered", not "over-reported" (the ported tool
		// says over-reported here, which names a router that never spoke).
		if origin == searchViaICMP {
			method = searchICMPOverReported
			if verdict == confirmTooLow {
				method = searchICMPUnderReported
			}
		}
		reported = 0
	}
	if reported != 0 {
		method = searchICMPKeptChanging
	}

	if refineErr := s.refine(); refineErr != nil {
		return s.result, refineErr
	}
	if s.result.outcome == searchMeasured {
		s.result.method = method
	}
	return s.result, nil
}

// confirmVerdict is what confirm answers about a reported figure.
type confirmVerdict uint8

const (
	confirmVerdictUnspecified confirmVerdict = iota
	// confirmConfirmed: the size passed and one octet more failed (AC-8).
	confirmConfirmed
	// confirmTooHigh: the size itself failed, or cannot be sent.
	confirmTooHigh
	// confirmTooLow: one octet more passed too.
	confirmTooLow
)

// confirm tests a reported wire MTU exactly: it MUST pass and one octet
// more MUST fail before it is the answer (AC-8). A figure outside the
// sendable range cannot be confirmed and reads as too high.
func (s *pathSearch) confirm(wire uint32) (confirmVerdict, error) {
	payload := int(wire) - s.overhead
	if payload < payloadMin {
		return confirmTooHigh, nil
	}
	if payload+1 > payloadMax {
		return confirmTooHigh, nil
	}
	passed, err := s.attempt(payload)
	if err != nil {
		return confirmVerdictUnspecified, err
	}
	if !passed {
		return confirmTooHigh, nil
	}
	passed, err = s.attempt(payload + 1)
	if err != nil {
		return confirmVerdictUnspecified, err
	}
	if passed {
		return confirmTooLow, nil
	}
	return confirmConfirmed, nil
}

// refine narrows whatever bracket the search holds. The candidates inside
// it are bisected by index first, because real MTUs cluster on that list;
// a floor probe then decides measurable from not; a contradicted bracket is
// re-tested; and the number line inside the bracket is bisected last.
// Correct from any bracket, including an empty one.
func (s *pathSearch) refine() error {
	var shortlist [len(candidateLadder)]int
	count := 0
	for _, candidate := range candidateLadder {
		payload := candidate - s.overhead
		if payload <= s.low {
			continue
		}
		if s.high != 0 {
			if payload >= s.high {
				continue
			}
		}
		shortlist[count] = payload
		count++
	}
	// lo is the index known to fail and hi the index known to pass; index 0
	// is the largest candidate and untested, so lo starts below it. At most
	// ladderAttemptsMax attempts.
	lo, hi := -1, count
	for hi-lo > 1 {
		mid := (lo + hi) / 2
		passed, err := s.attempt(shortlist[mid])
		if err != nil {
			return err
		}
		if passed {
			hi = mid
		} else {
			lo = mid
		}
	}

	// A floor when nothing has passed yet: a path that answers no probe of
	// payloadMin octets is unmeasurable (AC-7).
	if s.low == 0 {
		passed, err := s.attempt(payloadMin)
		if err != nil {
			return err
		}
		if !passed {
			s.result.outcome = searchUnmeasurable
			return nil
		}
	}

	lo, hi = s.low, s.high
	if hi == 0 {
		hi = payloadMax + 1
	}
	// A size that passed sitting at or above one that failed means two
	// probes contradicted each other, which a lossy path produces even with
	// the retries. Re-test the passed size rather than report a size known
	// to fail; when it fails too, the bracket restarts from the floor.
	if hi <= lo {
		s.result.lossy = true
		passed, err := s.attempt(lo)
		if err != nil {
			return err
		}
		if passed {
			hi = payloadMax + 1
		} else {
			lo, hi = payloadMin-1, s.high
		}
	}
	// The number line, at most bisectionAttemptsMax attempts: the bracket
	// is never wider than payloadMax + 1.
	for hi-lo > 1 {
		mid := (lo + hi) / 2
		passed, err := s.attempt(mid)
		if err != nil {
			return err
		}
		if passed {
			lo = mid
		} else {
			hi = mid
		}
	}
	if lo < payloadMin {
		s.result.outcome = searchUnmeasurable
		return nil
	}
	s.result.outcome = searchMeasured
	s.result.pathMTU = uint32(lo + s.overhead)
	return nil
}

// attempt sends one size up to probesPerSizeMax times and answers whether
// it passed, moving the bracket only on an answer it believes. A reply
// passes. A refusal fails on the first answer: it is not a dropped echo,
// so a second send can only produce the same refusal. A silence is retried
// and moves nothing. RFC 4821 Section 7.6.4: "The presence of other losses
// near the loss of the probe may indicate that the probe was lost due to
// congestion rather than due to an MTU limitation. In this case, the state
// variables eff_pmtu, search_low, and search_high SHOULD NOT be updated,
// and the same-sized probe SHOULD be attempted again". RFC 8899 Section
// 5.1.3: "Some probe loss is expected while searching, therefore loss of a
// single probe is not an indication of a PMTU problem." Only the
// probesPerSizeMax-th silence is believed, and then the size fails. An IKE
// too-big is a DF copy that drew no answer, so it is retried the same way
// (the Boundary row: 1..3 probes per size, for both probers).
func (s *pathSearch) attempt(payload int) (bool, error) {
	for range probesPerSizeMax {
		if s.result.probes >= runProbeBudget {
			return false, errProbeBudget
		}
		answer, err := s.prober.probe(s.ctx, payload)
		if err != nil {
			return false, err
		}
		s.result.probes++
		s.result.records = append(s.result.records, probeRecord{payload: payload, answer: answer})
		s.reported = 0
		s.reportedLocal = false
		switch answer.outcome {
		case probeReplied:
			s.low = max(s.low, payload)
			return true, nil
		case probeRefusedReported:
			s.reported = answer.mtu
			s.reportedLocal = answer.local
			s.fail(payload)
			return false, nil
		case probeRefusedUnreported:
			s.reportedLocal = answer.local
			s.fail(payload)
			return false, nil
		case probeSilent, probeTooBig:
			continue
		case probeOutcomeUnspecified:
			panic("BUG: prober answered an unspecified outcome")
		default:
			panic("BUG: prober answered an unknown outcome")
		}
	}
	s.fail(payload)
	return false, nil
}

// fail records payload as a size known to fail: the bracket's high moves
// down to it and never up.
func (s *pathSearch) fail(payload int) {
	if s.high == 0 {
		s.high = payload
		return
	}
	s.high = min(s.high, payload)
}

// wireProber is the prober bound to the probe layer: one DF socket, one
// target, one probe in flight at a time. A reply is matched by the socket's
// identifier and the probe's sequence, and a refusal is read off the error
// queue and matched by the echo it quotes, the way the ping session does:
// the raw socket sees every flow's errors, so an entry quoting another
// probe is left alone. The caller owns it and MUST call close.
type wireProber struct {
	sock       *probe.Socket
	dest       netip.Addr
	destAddr   net.IPAddr
	echoType   byte
	replyType  byte
	seq        uint16
	wait       time.Duration
	payload    []byte
	readBuffer []byte
}

// openWireProber opens the DF socket for target in mode df. The caller
// MUST close the prober.
func openWireProber(ctx context.Context, target netip.Addr, df probe.DFMode) (*wireProber, error) {
	family := probe.FamilyOf(target)
	sock, err := probe.OpenICMP(ctx, family, netip.Addr{}, df)
	if err != nil {
		return nil, err
	}
	w := &wireProber{
		sock:      sock,
		dest:      target,
		destAddr:  net.IPAddr{IP: target.AsSlice()},
		echoType:  8,
		replyType: 0,
		wait:      probeWait,
		// One zero-filled payload of the largest size and one read buffer,
		// allocated once for the whole search.
		payload:    make([]byte, payloadMax),
		readBuffer: make([]byte, payloadMax+icmpOverheadIPv6),
	}
	if family == probe.FamilyIPv6 {
		w.echoType, w.replyType = 128, 129
	}
	return w, nil
}

// close releases the socket. MUST be called once by the owner.
func (w *wireProber) close() error { return w.sock.Close() }

// probe sends one DF echo of payload octets and waits up to w.wait, which
// is probeWait outside a test, for its reply or its refusal.
func (w *wireProber) probe(ctx context.Context, payload int) (probeAnswer, error) {
	if payload > payloadMax {
		panic("BUG: the search asked for a probe above payloadMax")
	}
	w.seq++
	seq := w.seq
	packet := probe.BuildICMPEcho(w.echoType, w.sock.Identifier(), seq, w.payload[:payload])
	// The deadline covers the send as well as the reads: SetDeadline binds
	// both, and one left over from a probe that timed out would fail the
	// next send with a timeout of its own.
	deadline := time.Now().Add(w.wait)
	if ctxDeadline, ok := ctx.Deadline(); ok {
		if ctxDeadline.Before(deadline) {
			deadline = ctxDeadline
		}
	}
	if err := w.sock.SetDeadline(deadline); err != nil {
		return probeAnswer{}, fmt.Errorf("mtu: probe deadline: %w", err)
	}
	if _, err := w.sock.WriteTo(packet, &w.destAddr); err != nil {
		if errors.Is(err, syscall.EMSGSIZE) {
			return w.localRefusal(ctx)
		}
		return probeAnswer{}, fmt.Errorf("mtu: send %d-octet probe to %s: %w", payload, w.dest, err)
	}
	// Bounded by the deadline: every read returns by it, and a read error
	// that quotes another flow counts toward readErrorsMax.
	readErrors := 0
	for {
		n, from, err := w.sock.ReadFrom(w.readBuffer)
		if err != nil {
			if errors.Is(err, os.ErrDeadlineExceeded) {
				return probeAnswer{outcome: probeSilent}, nil
			}
			readErrors++
			if readErrors > readErrorsMax {
				return probeAnswer{}, fmt.Errorf("mtu: read probe reply from %s: %w", w.dest, err)
			}
			// Under IP_RECVERR a queued refusal makes one ordinary read
			// fail: drain the queue and look for the entry quoting this
			// probe. An entry quoting another probe leaves this one to wait.
			answer, found, drainErr := w.queuedRefusal(seq)
			if drainErr != nil {
				return probeAnswer{}, drainErr
			}
			if found {
				return answer, nil
			}
			continue
		}
		readErrors = 0
		if !w.isReplyTo(w.readBuffer[:n], from, seq) {
			continue
		}
		return probeAnswer{outcome: probeReplied}, nil
	}
}

// isReplyTo reports whether one datagram is the echo reply to probe seq
// from the target: the socket's identifier, the sequence and the source
// all match, the same three checks the ping session makes.
func (w *wireProber) isReplyTo(datagram []byte, from net.Addr, seq uint16) bool {
	id, replySeq, ok := probe.ParseEchoReply(datagram, w.replyType)
	if !ok {
		return false
	}
	if id != w.sock.Identifier() {
		return false
	}
	if replySeq != seq {
		return false
	}
	ipAddr, ok := from.(*net.IPAddr)
	if !ok {
		return false
	}
	fromAddr, ok := netip.AddrFromSlice(ipAddr.IP)
	if !ok {
		return false
	}
	return fromAddr.Unmap() == w.dest
}

// queuedRefusal drains the error queue and answers the refusal quoting
// probe seq when one is there. found is false when every entry was about
// another flow or another probe of ours already given up on.
func (w *wireProber) queuedRefusal(seq uint16) (answer probeAnswer, found bool, err error) {
	id := w.sock.Identifier()
	drainErr := w.sock.DrainErrors(func(q probe.QueuedError) {
		if found {
			return
		}
		if !q.SizeRefusalOf(id) {
			return
		}
		if q.Echo.Seq != seq {
			return
		}
		found = true
		answer = refusalAnswer(&q, false)
	})
	if drainErr != nil {
		return probeAnswer{}, false, fmt.Errorf("mtu: read the error queue: %w", drainErr)
	}
	return answer, found, nil
}

// localRefusal is the answer for a send this host's kernel refused with
// EMSGSIZE: the kernel queued a local entry carrying its estimate before
// WriteTo returned, and this prober is the only drainer, so that entry is
// this probe's. When the drain finds none, the estimate is read off the
// route with probe.KernelPathMTU; when that holds none either, the refusal
// stands and reports nothing.
func (w *wireProber) localRefusal(ctx context.Context) (probeAnswer, error) {
	var local *probe.QueuedError
	var entries [probe.ErrQueueDrainMax]probe.QueuedError
	count := 0
	drainErr := w.sock.DrainErrors(func(q probe.QueuedError) {
		if count < len(entries) {
			entries[count] = q
			count++
		}
	})
	if drainErr != nil {
		return probeAnswer{}, fmt.Errorf("mtu: read the error queue after a refused send: %w", drainErr)
	}
	for i := range entries[:count] {
		if entries[i].Local {
			local = &entries[i]
			break
		}
	}
	if local != nil {
		return refusalAnswer(local, true), nil
	}
	estimate, err := probe.KernelPathMTU(ctx, w.dest)
	if err != nil {
		return probeAnswer{outcome: probeRefusedUnreported, local: true}, nil //nolint:nilerr // no estimate is an answer by name, not a failure
	}
	return probeAnswer{outcome: probeRefusedReported, local: true, mtu: estimate}, nil
}

// ikeProber is the prober bound to a live IKE SA: each probe is one padded
// INFORMATIONAL exchange the IKE engine runs on the SA's own socket, asked
// for through the ikeprobe leaf so this package never imports the engine. The
// search speaks in ICMP payload octets and the engine in datagram octets, so
// the adapter adds the ICMP overhead back: the size on the wire is what a
// path MTU is a property of, whichever protocol carries it.
//
// A fit is a reply and a too-big is retried like a silence, the two answers
// the search brackets on. Three things end the attempt instead of answering:
// an exchange past ikeExchangesPerRunMax (errIKEExchangeBudget, refused
// before the leaf is asked), a refusal or a failed SA (ikeProbeEnded, by
// name), and a leaf with no engine registered (ErrNotRegistered, passed
// through). ceiling is the ICMP figure in wire octets: the search is built
// so no size above it is asked, and asking one is a defect.
//
// The engine sends the largest datagram the SA's cipher suite produces at or
// below the size asked (a CBC suite sends on a 16-octet grid), never above,
// and names that size in its answer. A fit at the size sent is a fit the
// search reads at the size asked, which is a true lower bound; fitOctets
// keeps the largest size that fit as sent, and fitAsked the ask that
// produced it, so the row reports the size the path is proven to carry.
type ikeProber struct {
	peer      string
	overhead  int
	df        probe.DFMode
	ceiling   int
	ask       ikeprobe.Prober
	exchanges int
	fitOctets int
	fitAsked  int
}

func (p *ikeProber) probe(ctx context.Context, payload int) (probeAnswer, error) {
	wire := payload + p.overhead
	if wire > p.ceiling {
		panic("BUG: an IKE probe above the ICMP figure was asked for; the search descends from it and never rises")
	}
	if p.exchanges >= ikeExchangesPerRunMax {
		return probeAnswer{}, errIKEExchangeBudget
	}
	result, err := p.ask(ctx, ikeprobe.Request{Peer: p.peer, WireOctets: uint16(wire), DF: p.df})
	if err != nil {
		return probeAnswer{}, err
	}
	// An exchange is a message that left: a refusal built nothing and
	// spends none.
	switch result.Outcome {
	case ikeprobe.OutcomeFits:
		sent := p.spend(&result, wire)
		if sent > p.fitOctets {
			p.fitOctets = sent
			p.fitAsked = wire
		}
		return probeAnswer{outcome: probeReplied}, nil
	case ikeprobe.OutcomeTooBig:
		p.spend(&result, wire)
		return probeAnswer{outcome: probeTooBig}, nil
	case ikeprobe.OutcomeSAFailed:
		sent := p.spend(&result, wire)
		// The SA is being re-established; a further size would be refused
		// sa-down, and the one that failed it is what the row reports.
		if ikeProbeStopAtFirstSilence {
			return probeAnswer{}, &ikeProbeEnded{result: result, wire: sent}
		}
		return probeAnswer{outcome: probeSilent}, nil
	case ikeprobe.OutcomeRefused:
		if result.Refusal == ikeprobe.RefusalRekeyed {
			// The exchange left and was lost to the peer's rekey (AC-10): it
			// counts, says nothing about the path, and attempt asks the size
			// again on the SA that replaced the retired one.
			p.spend(&result, wire)
			return probeAnswer{outcome: probeSilent}, nil
		}
		return probeAnswer{}, &ikeProbeEnded{result: result, wire: wire}
	case ikeprobe.OutcomeUnspecified:
		panic("BUG: the IKE prober answered an unspecified outcome")
	default:
		panic("BUG: the IKE prober answered an unknown outcome")
	}
}

// spend counts one exchange and answers the size the engine sent for it. An
// outcome about a datagram names that size; none, or one above the size
// asked, is a Ze defect rather than an answer about the path.
func (p *ikeProber) spend(result *ikeprobe.Result, asked int) int {
	sent := int(result.WireOctets)
	if sent == 0 || sent > asked {
		panic("BUG: the IKE prober answered an exchange without the size it sent, or one above the size asked")
	}
	p.exchanges++
	return sent
}

// ikeProbeEnded is the error an IKE probe ends the search with when the
// engine produced no answer about the path: it refused by name, or the SA
// failed under the exchange of wire octets. The text names the outcome, the
// refusal's condition, or the size the SA failed at, and the row and the note
// carry it as written (AC-13: the row says sa-failed and names the size).
type ikeProbeEnded struct {
	result ikeprobe.Result
	wire   int
}

func (e *ikeProbeEnded) Error() string {
	if e.result.Outcome == ikeprobe.OutcomeRefused {
		return e.result.Outcome.String() + ": " + e.result.Refusal.String()
	}
	return e.result.Outcome.String() + " at " + strconv.Itoa(e.wire) + " octets: no copy of the padded exchange was answered inside the retransmit budget, so the IKE SA was deemed failed (RFC 7296 Section 2.1) and is being re-established"
}

// refusalAnswer converts one queued refusal to the search's answer: the
// three outcomes of probe.QueuedError become reported or unreported.
func refusalAnswer(q *probe.QueuedError, local bool) probeAnswer {
	if q.Outcome == probe.ErrQueueMTUReported {
		return probeAnswer{outcome: probeRefusedReported, local: local, mtu: q.MTU}
	}
	return probeAnswer{outcome: probeRefusedUnreported, local: local}
}
