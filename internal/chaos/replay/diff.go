// Design: docs/architecture/chaos-web-dashboard.md — event replay and diff

package replay

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"slices"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// diffReadFailed reports whether either log failed to read, and the exit code
// to return when it did. A truncated read is an error (2), never a verdict
// about how the two logs compare.
func diffReadFailed(s1, s2 *bufio.Scanner, w io.Writer) (int, bool) {
	if err := s1.Err(); err != nil {
		writeErr(w, "error: reading log1: %v\n", err)
		return 2, true
	}
	if err := s2.Err(); err != nil {
		writeErr(w, "error: reading log2: %v\n", err)
		return 2, true
	}
	return 0, false
}

// Diff compares two NDJSON event logs line-by-line and reports the first
// divergence point. For observations it compares the recorded scenario and
// the events through the checkpoint. Event timing varies between runs.
//
// Returns exit code: 0=identical, 1=divergence found, 2=error.
func Diff(r1, r2 io.Reader, w io.Writer) int {
	s1 := bufio.NewScanner(r1)
	s2 := bufio.NewScanner(r2)
	s1.Buffer(nil, 2*1024*1024)
	s2.Buffer(nil, 2*1024*1024)

	// Read the scenario header from each log.
	headers := s1.Scan()
	headers = s2.Scan() && headers
	if code, failed := diffReadFailed(s1, s2, w); failed {
		return code
	}
	if !headers {
		writeErr(w, "error: one or both logs are empty\n")
		return 2
	}
	var hdr1, hdr2 logHeader
	// Legacy comparisons have always treated the header as an opaque line.
	_ = json.Unmarshal(s1.Bytes(), &hdr1) //nolint:errcheck // absent legacy metadata leaves the legacy comparison in effect
	_ = json.Unmarshal(s2.Bytes(), &hdr2) //nolint:errcheck // absent legacy metadata leaves the legacy comparison in effect
	observation1, observation2 := hdr1.Version >= 2, hdr2.Version >= 2
	if !observation1 && len(s1.Bytes()) >= bufio.MaxScanTokenSize {
		writeErr(w, "error: reading log1: legacy header exceeds the line size limit\n")
		return 2
	}
	if !observation2 && len(s2.Bytes()) >= bufio.MaxScanTokenSize {
		writeErr(w, "error: reading log2: legacy header exceeds the line size limit\n")
		return 2
	}
	if observation1 && observation2 && !sameObservationScenario(hdr1, hdr2) {
		rw := reportWriter{w: w}
		rw.printf("divergence: observation settings differ\n")
		if rw.err != nil {
			return 2
		}
		return 1
	}

	// Collect recent lines for context on divergence.
	const contextSize = 5
	var recent [contextSize]string
	var lineNum int

	for {
		has1 := s1.Scan()
		has2 := s2.Scan()

		// A log that stops early is not a log that ended. Scan returns false on
		// EOF, on a read error, and on a line above bufio.MaxScanTokenSize
		// alike, so both verdicts below would be wrong: two logs truncated at
		// the same point read as identical, and one truncated log reads as a
		// length divergence in the other.
		if code, failed := diffReadFailed(s1, s2, w); failed {
			return code
		}
		if has1 && !observation1 && len(s1.Bytes()) >= bufio.MaxScanTokenSize {
			writeErr(w, "error: reading log1: legacy record exceeds the line size limit\n")
			return 2
		}
		if has2 && !observation2 && len(s2.Bytes()) >= bufio.MaxScanTokenSize {
			writeErr(w, "error: reading log2: legacy record exceeds the line size limit\n")
			return 2
		}
		if !has1 && observation1 {
			writeErr(w, "error: log1 ended before the observation checkpoint\n")
			return 2
		}
		if !has2 && observation2 {
			writeErr(w, "error: log2 ended before the observation checkpoint\n")
			return 2
		}

		if !has1 && !has2 {
			return diffIdentical(w, lineNum)
		}

		if has1 != has2 {
			// One log is shorter.
			if _, err := fmt.Fprintf(w, "divergence: length mismatch after %d events (log1 has %s)\n", //nolint:errcheck // output
				lineNum, lengthDesc(has1)); err != nil {
				return 2
			}
			return 1
		}

		lineNum++

		var ev1, ev2 logEvent
		if err := json.Unmarshal(s1.Bytes(), &ev1); err != nil {
			writeErr(w, "error: parsing log1 line %d: %v\n", lineNum, err)
			return 2
		}
		if err := json.Unmarshal(s2.Bytes(), &ev2); err != nil {
			writeErr(w, "error: parsing log2 line %d: %v\n", lineNum, err)
			return 2
		}

		// Store for context.
		var bCtx textbuf.Buffer
		recent[lineNum%contextSize] = bCtx.Reset().Str("  seq ").Uint(ev1.Seq).Str(": ").Str(ev1.EventType).Str(" peer=").Int(int64(ev1.PeerIndex)).Str(" prefix=").Str(ev1.Prefix).String()

		if !sameDiffEvent(ev1, ev2, observation1 || observation2) {
			rw := reportWriter{w: w}
			rw.printf("divergence at seq %d (event line %d):\n", ev1.Seq, lineNum)

			// Print context (last N matching lines).
			rw.printf("context:\n")
			for i := max(1, lineNum-contextSize+1); i < lineNum; i++ {
				idx := i % contextSize
				if recent[idx] != "" {
					rw.printf("%s\n", recent[idx])
				}
			}

			rw.printf("log1: %s peer=%d prefix=%s\n", ev1.EventType, ev1.PeerIndex, ev1.Prefix)
			rw.printf("log2: %s peer=%d prefix=%s\n", ev2.EventType, ev2.PeerIndex, ev2.Prefix)
			if observation1 || observation2 {
				writeDiffObservation(&rw, "log1", ev1)
				writeDiffObservation(&rw, "log2", ev2)
			}

			if rw.err != nil {
				return 2
			}
			return 1
		}
		if observation1 && observation2 && ev1.RecordType == "observation-end" {
			// Teardown cannot change the verdict. Read through it to detect I/O failures.
			for s1.Scan() {
			}
			for s2.Scan() {
			}
			if code, failed := diffReadFailed(s1, s2, w); failed {
				return code
			}
			return diffIdentical(w, lineNum-1)
		}
	}
}

func sameObservationScenario(a, b logHeader) bool {
	return a.Peers == b.Peers && a.Properties == b.Properties && a.ConvergenceDeadline == b.ConvergenceDeadline &&
		slices.Equal(a.IBGPPeers, b.IBGPPeers) && slices.EqualFunc(a.PeerFamilies, b.PeerFamilies, slices.Equal[[]string])
}

func sameDiffEvent(a, b logEvent, observation bool) bool {
	sameRouteEvent := a.EventType == b.EventType && a.PeerIndex == b.PeerIndex && a.Prefix == b.Prefix
	if !sameRouteEvent {
		return false
	}
	if !observation {
		return true
	}
	return a.RecordType == b.RecordType && a.Family == b.Family && a.Count == b.Count &&
		a.PlannedShutdown == b.PlannedShutdown && slices.Equal(a.WithdrawnPrefixes, b.WithdrawnPrefixes) &&
		a.ChaosAction == b.ChaosAction && maps.Equal(a.ChaosParams, b.ChaosParams)
}

func writeDiffObservation(w *reportWriter, name string, event logEvent) {
	w.printf("%s observation: record=%s family=%s count=%d planned-shutdown=%t withdrawn=%v action=%s params=%v\n",
		name, event.RecordType, event.Family, event.Count, event.PlannedShutdown, event.WithdrawnPrefixes,
		event.ChaosAction, event.ChaosParams)
}

func diffIdentical(w io.Writer, count int) int {
	var line textbuf.Buffer
	line.Str("logs identical (").Int(int64(count)).Str(" events)\n")
	if _, err := w.Write(line.Bytes()); err != nil {
		return 2
	}
	return 0
}

// lengthDesc returns "more" or "fewer" events.
func lengthDesc(log1HasMore bool) string {
	if log1HasMore {
		return "more events"
	}
	return "fewer events"
}

// reportWriter wraps an io.Writer and tracks the first error.
type reportWriter struct {
	w   io.Writer
	err error
}

func (rw *reportWriter) printf(format string, args ...any) {
	if rw.err != nil {
		return
	}
	_, rw.err = fmt.Fprintf(rw.w, format, args...) //nolint:errcheck // output
}
