// Design: docs/architecture/chaos-web-dashboard.md — event replay and diff

package replay

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
)

// diffEvent holds the fields compared during diff.
type diffEvent struct {
	Seq       uint64 `json:"seq"`
	EventType string `json:"event-type"`
	PeerIndex int    `json:"peer-index"`
	Prefix    string `json:"prefix,omitempty"`
}

// Diff compares two NDJSON event logs line-by-line and reports the first
// divergence point. Compares event-type, peer-index, and prefix fields
// (skips time-offset-ms since timing varies between runs).
//
// Returns exit code: 0=identical, 1=divergence found, 2=error.
func Diff(r1, r2 io.Reader, w io.Writer) int {
	s1 := bufio.NewScanner(r1)
	s2 := bufio.NewScanner(r2)

	// Skip header lines.
	if !s1.Scan() || !s2.Scan() {
		writeErr(w, "error: one or both logs are empty\n")
		return 2
	}

	// Collect recent lines for context on divergence.
	const contextSize = 5
	var recent [contextSize]string
	var lineNum int

	for {
		has1 := s1.Scan()
		has2 := s2.Scan()

		if !has1 && !has2 {
			// Both exhausted — identical.
			if _, err := fmt.Fprintf(w, "logs identical (%d events)\n", lineNum); err != nil {
				return 2
			}
			return 0
		}

		if has1 != has2 {
			// One log is shorter.
			if _, err := fmt.Fprintf(w, "divergence: length mismatch after %d events (log1 has %s)\n",
				lineNum, lengthDesc(has1)); err != nil {
				return 2
			}
			return 1
		}

		lineNum++

		var ev1, ev2 diffEvent
		if err := json.Unmarshal(s1.Bytes(), &ev1); err != nil {
			writeErr(w, "error: parsing log1 line %d: %v\n", lineNum, err)
			return 2
		}
		if err := json.Unmarshal(s2.Bytes(), &ev2); err != nil {
			writeErr(w, "error: parsing log2 line %d: %v\n", lineNum, err)
			return 2
		}

		// Store for context.
		recent[lineNum%contextSize] = fmt.Sprintf("  seq %d: %s peer=%d prefix=%s",
			ev1.Seq, ev1.EventType, ev1.PeerIndex, ev1.Prefix)

		if ev1.EventType != ev2.EventType || ev1.PeerIndex != ev2.PeerIndex || ev1.Prefix != ev2.Prefix {
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

			if rw.err != nil {
				return 2
			}
			return 1
		}
	}
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
	_, rw.err = fmt.Fprintf(rw.w, format, args...)
}
