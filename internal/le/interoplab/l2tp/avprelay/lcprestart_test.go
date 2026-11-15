// Design: docs/labs/l2tp-interop.md -- restart observation serialization.
// Related: lcprestart.go -- the monotonic relay observation.
package main

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"
)

// TestRestartWireClockIsExplicit checks that the normal relay preserves its
// old JSON shape, while an armed probe labels its monotonic observation.
func TestRestartWireClockIsExplicit(t *testing.T) {
	event := wireObservation{"ze", "0002", "0002"}
	for _, probe := range []*restartProbe{nil, {started: time.Now().Add(-time.Second)}} {
		var output bytes.Buffer
		if err := encodeWire(json.NewEncoder(&output), event, probe); err != nil {
			t.Fatal(err)
		}
		var decoded map[string]json.RawMessage
		if err := json.Unmarshal(output.Bytes(), &decoded); err != nil {
			t.Fatal(err)
		}
		_, armed := decoded["restart"]
		if armed != (probe != nil) {
			t.Fatalf("restart marker present=%t for probe%v", armed, probe)
		}
		if probe != nil {
			var elapsed int64
			if err := json.Unmarshal(decoded["elapsed_nanos"], &elapsed); err != nil {
				t.Fatal(err)
			}
			if elapsed < int64(time.Second) {
				t.Fatalf("elapsed=%d, want at least1s", elapsed)
			}
		}
	}
}
