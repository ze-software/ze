// Design: docs/architecture/testing/interop.md -- requested End-of-RIB on a manual-eor peer.
// Related: check_manual_eor.go -- the checker body and the requesting process.
// Related: check_rfc_predicate.go -- frrDecodeFields and noFamilyFRRLogMessage, the FRR log readers reused here.
package bgp

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// manualEORMarkerMessage is the message FRR 10.7.1 writes for each End-of-RIB
// it receives in IPv4 unicast from peer. bgpd/bgp_packet.c at frr-10.7.1:
// bgp_update_receive_eor opens with
//
//	zlog_info("%s: rcvd End-of-RIB for %s from %s in vrf %s", __func__,
//		  get_afi_safi_str(afi, safi, false), peer->host,
//		  vrf ? vrf->name : VRF_DEFAULT_NAME);
//
// before it tests PEER_STATUS_EOR_RECEIVED, so every marker is logged, the
// second of a session included. bgp_update_receive calls it for an UPDATE
// with no withdrawn routes, no attributes and no NLRI on a family the session
// negotiated (peer->afc).
func manualEORMarkerMessage(peer string) string {
	return "bgp_update_receive_eor: rcvd End-of-RIB for IPv4 Unicast from " + peer + " in vrf default"
}

// manualEORReading is one prefix and the markers in one stretch of FRR's log,
// each as the line index FRR wrote it at. FRR handles the UPDATEs of one
// connection in order and logs each as it handles it, so a smaller index is
// an earlier message on the wire.
type manualEORReading struct {
	routeSeen bool
	routeLine int
	markers   []int
}

// readManualEOR reads log for FRR's first decode of prefix from peer and for
// every End-of-RIB marker from peer. A route decode is FRR's `<peer> rcvd
// <prefix> IPv4 unicast` line (bgp_update, bgpd/bgp_route.c, under `debug bgp
// updates in`), matched by frrDecodeFields so a longer prefix or another
// neighbor cannot pass. A marker must be the whole native message inside
// FRR's log envelope (noFamilyFRRLogMessage), so text that merely contains it
// does not count.
func readManualEOR(log, peer, prefix string) manualEORReading {
	var reading manualEORReading
	marker := manualEORMarkerMessage(peer)
	index := 0
	for line := range strings.SplitSeq(log, "\n") {
		if message, ok := noFamilyFRRLogMessage(line); ok && message == marker {
			reading.markers = append(reading.markers, index)
		} else if !reading.routeSeen && frrDecodeFields(line, peer, prefix) != nil {
			reading.routeSeen = true
			reading.routeLine = index
		}
		index++
	}
	return reading
}

// requireNoManualEORMarker judges the first session: FRR decoded
// manualEORFirstPrefix, the only route that session announces, which proves
// the log read holds this session's UPDATEs, and decoded no End-of-RIB. The
// route is the positive half: a log FRR never wrote, or a read of another
// file, holds no marker either, and it must not pass as Ze withholding one.
func requireNoManualEORMarker(log, peer string) error {
	reading := readManualEOR(log, peer, manualEORFirstPrefix)
	if !reading.routeSeen {
		return errors.New("FRR's log holds no decode of " + manualEORFirstPrefix + " from " + peer + ", so it cannot show what Ze sent on the first session")
	}
	if len(reading.markers) != 0 {
		return fmt.Errorf("FRR decoded %d End-of-RIB markers from %s on the first session, where nothing asked for one: a manual-eor peer must receive no automatic marker", len(reading.markers), peer)
	}
	return nil
}

// manualEORRequestedReady reports whether the second session's stretch of the
// log already holds prefix and a marker after it, the point at which the
// checker starts its hold for a second marker.
func manualEORRequestedReady(log, peer, prefix string) bool {
	reading := readManualEOR(log, peer, prefix)
	if !reading.routeSeen {
		return false
	}
	for _, marker := range reading.markers {
		if marker > reading.routeLine {
			return true
		}
	}
	return false
}

// requireOneManualEORMarkerAfterRoute judges the second session's stretch of
// the log: FRR decoded manualEORSecondPrefix, the route that session
// announces, then exactly one End-of-RIB, the one the process asked for. No
// marker means the request was dropped; a marker before the route means it
// overtook the route the same process announced first; a second marker means
// Ze added an automatic one.
func requireOneManualEORMarkerAfterRoute(log, peer string) error {
	reading := readManualEOR(log, peer, manualEORSecondPrefix)
	if !reading.routeSeen {
		return errors.New("FRR's log holds no decode of " + manualEORSecondPrefix + " from " + peer + " on the second session")
	}
	if len(reading.markers) == 0 {
		return errors.New("FRR decoded no End-of-RIB from " + peer + " on the second session, where the process asked for one inside the initial sync: the requested marker was dropped")
	}
	if len(reading.markers) != 1 {
		return fmt.Errorf("FRR decoded %d End-of-RIB markers from %s on the second session, want exactly the one the process asked for: a manual-eor peer must receive no automatic marker", len(reading.markers), peer)
	}
	if reading.markers[0] < reading.routeLine {
		return errors.New("FRR decoded the End-of-RIB from " + peer + " before " + manualEORSecondPrefix + ", which the process announced before it asked for the marker")
	}
	return nil
}

// manualEORLogAfter answers the part of FRR's log written after before. The
// file is append-only for the life of the container, so a later read that
// does not start with before is a different or truncated file, and reading
// its tail as the second session would judge lines of an unknown origin.
func manualEORLogAfter(before, after string) (string, error) {
	tail, found := strings.CutPrefix(after, before)
	if !found {
		return "", errors.New("FRR's log no longer starts with what the checker read before the clear: the file was replaced or truncated")
	}
	return tail, nil
}

// manualEORRequest is one record the requesting process writes, one per
// peer-UP event it handled, in the order it handled them.
type manualEORRequest struct {
	Session   int    `json:"session"`
	Peer      string `json:"peer"`
	Prefix    string `json:"prefix"`
	EndOfRIB  bool   `json:"end-of-rib"`
	ElapsedMS int64  `json:"elapsed-ms"`
	Error     string `json:"error,omitempty"`
}

// manualEORSessionPlan is what the process does on each session: announce
// prefix, then ask for the End-of-RIB when endOfRIB is set. The first session
// asks for nothing, so it shows what Ze sends a manual-eor peer unasked;
// every later one asks, from inside the initial sync.
func manualEORSessionPlan(session int) (prefix string, endOfRIB bool) {
	if session == 1 {
		return manualEORFirstPrefix, false
	}
	return manualEORSecondPrefix, true
}

// requireManualEORRequests judges the process's records: exactly want
// sessions, each handled as manualEORSessionPlan says, answered inside the
// bound that keeps it under the engine's delivery timeout, and with no error.
// A record past that bound, or a missing one, means the requests may have
// landed after the sync started, so the second session would prove nothing
// about a marker asked for during it.
func requireManualEORRequests(output string, want int) error {
	var records []manualEORRequest
	if err := json.Unmarshal([]byte(output), &records); err != nil {
		return fmt.Errorf("decode the requesting process's records: %w", err)
	}
	if len(records) != want {
		return fmt.Errorf("the requesting process handled %d peer-UP events, want %d", len(records), want)
	}
	for index, record := range records {
		session := index + 1
		if record.Error != "" {
			return fmt.Errorf("the requesting process failed on session %d: %s", session, record.Error)
		}
		if record.Session != session {
			return fmt.Errorf("record %d names session %d", session, record.Session)
		}
		prefix, endOfRIB := manualEORSessionPlan(session)
		if record.Prefix != prefix {
			return fmt.Errorf("session %d announced %q, want %q", session, record.Prefix, prefix)
		}
		if record.EndOfRIB != endOfRIB {
			return fmt.Errorf("session %d asked for the End-of-RIB: %t, want %t", session, record.EndOfRIB, endOfRIB)
		}
		if record.ElapsedMS < 0 {
			return fmt.Errorf("session %d reports a negative handler time", session)
		}
		if time.Duration(record.ElapsedMS)*time.Millisecond >= manualEORRequestBound {
			return fmt.Errorf("session %d's requests took %d ms, at or past the %s bound", session, record.ElapsedMS, manualEORRequestBound)
		}
	}
	return nil
}
