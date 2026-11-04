// Design: docs/guide/benchmarking.md -- raw measurement presence.
package perf

import (
	"encoding/json"
	"testing"
)

// TestConvergencePresenceValueContainers verifies the JSON method set for raw
// values, pointers, slices and maps, so no value form invents a zero measurement.
func TestConvergencePresenceValueContainers(t *testing.T) {
	for _, field := range []string{"", `,"convergence-ms":null`, `,"convergence-ms":0`, `,"convergence-ms":7`} {
		var row Result
		if err := json.Unmarshal([]byte(`{"dut-name":"ze","routes-lost":1`+field+`}`), &row); err != nil {
			t.Fatal(err)
		}
		for _, form := range []struct {
			name   string
			value  any
			unwrap func(json.RawMessage) json.RawMessage
		}{
			{"value", row, func(data json.RawMessage) json.RawMessage { return data }},
			{"pointer", &row, func(data json.RawMessage) json.RawMessage { return data }},
			{"slice", []Result{row}, func(data json.RawMessage) json.RawMessage {
				var rows []json.RawMessage
				if err := json.Unmarshal(data, &rows); err != nil {
					t.Fatal(err)
				}
				return rows[0]
			}},
			{"map", map[string]Result{"row": row}, func(data json.RawMessage) json.RawMessage {
				var rows map[string]json.RawMessage
				if err := json.Unmarshal(data, &rows); err != nil {
					t.Fatal(err)
				}
				return rows["row"]
			}},
		} {
			t.Run(form.name+field, func(t *testing.T) {
				data, err := json.Marshal(form.value)
				if err != nil {
					t.Fatal(err)
				}
				var fields map[string]json.RawMessage
				if err := json.Unmarshal(form.unwrap(data), &fields); err != nil {
					t.Fatal(err)
				}
				value, present := fields["convergence-ms"]
				wantPresent := field == `,"convergence-ms":0` || field == `,"convergence-ms":7`
				if present != wantPresent {
					t.Fatalf("presence changed: %s", data)
				}
				if wantPresent {
					want := "0"
					if field == `,"convergence-ms":7` {
						want = "7"
					}
					if string(value) != want {
						t.Errorf("convergence = %s, want %s", value, want)
					}
				}
				if string(fields["routes-lost"]) != "1" {
					t.Fatal("raw diagnostic loss erased")
				}
			})
		}
	}
}
