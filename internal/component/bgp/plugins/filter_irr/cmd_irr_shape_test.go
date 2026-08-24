// Design: docs/architecture/api/commands.md -- per-command answer shape
// Related: cmd_irr.go -- the declarations this file pins
// Related: command.go -- the producers each declared name is read from

package filter_irr

import (
	"encoding/json"
	"net/netip"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/command"
)

// TestIRRCommandsDeclareTheirShape holds the three `show bgp irr` commands to
// declaring what their answers hold.
//
// VALIDATES: AC-18, AC-20 for the irr branch.
// PREVENTS: three commands reaching no pre-dispatch refusal. Each is served by
// the bgp-filter-irr plugin PROCESS and registered by an in-core shim
// (cmd_irr.go), so each can declare today, and none did: `show bgp irr check`
// answers one document and accepted `| first 1`, which drops the key the bare
// command prints.
func TestIRRCommandsDeclareTheirShape(t *testing.T) {
	cases := []struct {
		path  string
		shape command.AnswerShape
	}{
		{cmdShowIRR, command.ShapeTab},
		{cmdShowIRRPrefix, command.ShapeMap},
		{cmdShowIRRCheck, command.ShapeDoc},
	}

	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			shape, declared := command.ShapeForCommand(tc.path)
			if !declared {
				t.Fatalf("%s declares no answer shape", tc.path)
			}
			if shape != tc.shape {
				t.Errorf("%s resolves to %s, want %s", tc.path, shape, tc.shape)
			}
		})
	}

	// `show bgp irr check` answers one document, so every row operator is
	// refused by name before the command runs.
	_, _, errMsg := command.ProcessPipesDefaultFormatChecked(cmdShowIRRCheck+" | first 1", "")
	if errMsg == "" {
		t.Errorf("%s | first 1 was accepted; the answer is one document", cmdShowIRRCheck)
	} else if !strings.Contains(errMsg, "first") || !strings.Contains(errMsg, "one document") {
		t.Errorf("refusal = %q, want it to name first and say the answer is one document", errMsg)
	}
}

// TestIRRStatusDisplaysDeclaredFields drives AC-17 through the surface an
// operator reaches.
//
// VALIDATES: AC-17.
// PREVENTS: `| display` answering nothing over a command that declares no
// order. The two names are keys of an "entries" row (showIRR, command.go), so
// the operator's own order reaches the renderer and the other row keys go.
func TestIRRStatusDisplaysDeclaredFields(t *testing.T) {
	_, format, errMsg := command.ProcessPipesDefaultFormatChecked(cmdShowIRR+" | display asn status", "")
	if errMsg != "" {
		t.Fatalf("%s | display asn status was refused: %s", cmdShowIRR, errMsg)
	}

	answer := format(irrStatusAnswer(t))
	asnAt := strings.Index(answer, "asn")
	statusAt := strings.Index(answer, "status")
	if asnAt < 0 || statusAt < 0 {
		t.Fatalf("%s | display asn status answered %q, want both fields", cmdShowIRR, answer)
	}
	if asnAt > statusAt {
		t.Errorf("%s | display asn status put status before asn: %q", cmdShowIRR, answer)
	}
	if strings.Contains(answer, "AS-EXAMPLE") {
		t.Errorf("%s | display kept the as-set column: %q", cmdShowIRR, answer)
	}
}

// TestDeclaredColumnsExistInPayload holds every column name this package
// declares to being a key its producer writes.
//
// VALIDATES: AC-21 for the irr branch.
// PREVENTS: the one failure with no signal. A declared name the payload never
// carries orders nothing and publishes a field that does not exist.
//
// The answers come from the REAL producers, run over an irrPlugin holding one
// resolved ASN and one whose lookup failed, so the keys compared against are
// the keys showIRR, renderPrefixes and showIRRCheck write rather than a fixture
// kept in step by hand.
//
// A name is compared against EVERY record the producer can write, not against
// one of them. A declaration names the union of the branches: "error" is
// written only for a failed lookup and "last-refresh" only for one that
// succeeded, and an order never hides or invents a key, so naming both is
// correct for both branches.
func TestDeclaredColumnsExistInPayload(t *testing.T) {
	cases := []struct {
		path    string
		records []map[string]any
	}{
		{cmdShowIRR, append(irrEntryRows(t), decodeIRR(t, irrStatusAnswer(t)))},
		{cmdShowIRRPrefix, []map[string]any{decodeIRR(t, irrAnswer(t, func(plug *irrPlugin) (string, any, error) {
			return plug.showIRRPrefix([]string{"192.0.2.1"})
		}))}},
		{cmdShowIRRCheck, []map[string]any{decodeIRR(t, irrAnswer(t, func(plug *irrPlugin) (string, any, error) {
			return plug.showIRRCheck([]string{"192.0.2.1", "198.51.100.0/24"})
		}))}},
	}

	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			orders := command.ColumnsForCommand(tc.path)
			if len(orders) == 0 {
				t.Fatalf("%s declares no column order", tc.path)
			}

			written := make(map[string]struct{}, 16)
			for _, record := range tc.records {
				for name := range record {
					written[name] = struct{}{}
				}
			}

			for _, order := range orders {
				for _, name := range order {
					if _, exists := written[name]; !exists {
						t.Errorf("%s declares column %q; the producer writes %v", tc.path, name, sortedNames(written))
					}
				}
			}
		})
	}
}

// TestIRRDeclaresNoAddressField holds the irr branch to declaring no address
// field, so `| resolve` and `| origin` are refused by name.
//
// VALIDATES: the naming half of the Critical Review Checklist for this branch.
// PREVENTS: publishing an operator that decorates nothing. The one value in the
// irr answers that holds addresses is "peers", an ARRAY of address strings
// inside an entry row (showIRR), and resolveJSON walks past an array element:
// it decorates a map VALUE that passes netip.ParseAddr and nothing else
// (internal/component/command/pipe_resolve.go). "prefix" and every element of
// "prefixes" is a prefix, which netip.ParseAddr refuses.
func TestIRRDeclaresNoAddressField(t *testing.T) {
	for _, path := range []string{cmdShowIRR, cmdShowIRRPrefix, cmdShowIRRCheck} {
		if fields := command.AddressFieldsForCommand(path); len(fields) > 0 {
			t.Errorf("%s declares address fields %v; no field of its answer holds a bare address", path, fields)
		}
	}

	_, _, errMsg := command.ProcessPipesDefaultFormatChecked(cmdShowIRR+" | resolve", "")
	if errMsg == "" {
		t.Errorf("%s | resolve was accepted; it decorates nothing", cmdShowIRR)
	} else if !strings.Contains(errMsg, "resolve") || !strings.Contains(errMsg, "IP address") {
		t.Errorf("refusal = %q, want it to name resolve and the missing IP address field", errMsg)
	}
}

// resolvedIRRPlugin is one plugin holding one ASN that resolved and one whose
// lookup failed, so both branches showIRR can take are in its answer: the
// resolved entry carries "last-refresh" and the failed one carries "error".
func resolvedIRRPlugin() *irrPlugin {
	return &irrPlugin{
		config:      &irrConfig{Server: "whois.example.net"},
		lastRefresh: time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC),
		nextRefresh: time.Date(2026, 8, 24, 1, 0, 0, 0, time.UTC),
		byASN: map[uint32]*asnState{
			65001: {
				asn:       65001,
				asSet:     "AS-EXAMPLE",
				lastOK:    time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC),
				v4Count:   1,
				v6Count:   0,
				peerAddrs: []string{"192.0.2.1"},
				list: &irrPrefixList{entries: []prefixEntry{
					{prefix: netip.MustParsePrefix("198.51.100.0/24"), ge: 24, le: 24},
				}},
			},
			65002: {
				asn:       65002,
				asSet:     "AS-BROKEN",
				lastErr:   "irr: lookup prefixes AS65002 (IPv4): connection refused",
				peerAddrs: []string{"192.0.2.2"},
			},
		},
	}
}

// irrAnswer runs one producer over resolvedIRRPlugin and answers its payload.
func irrAnswer(t *testing.T, produce func(*irrPlugin) (string, any, error)) string {
	t.Helper()

	status, data, err := produce(resolvedIRRPlugin())
	if err != nil {
		t.Fatalf("the producer answered an error: %v", err)
	}
	if status != statusDone {
		t.Fatalf("the producer answered status %q, want %q", status, statusDone)
	}
	raw, isRaw := data.(json.RawMessage)
	if !isRaw {
		t.Fatalf("the producer answered %T, want json.RawMessage", data)
	}
	return string(raw)
}

// irrStatusAnswer is the `show bgp irr` payload showIRR writes.
func irrStatusAnswer(t *testing.T) string {
	t.Helper()
	return irrAnswer(t, func(plug *irrPlugin) (string, any, error) { return plug.showIRR() })
}

// irrEntryRows answers every "entries" row of the `show bgp irr` payload, which
// is one row for each ASN and so one for each branch showIRR can take.
func irrEntryRows(t *testing.T) []map[string]any {
	t.Helper()

	envelope := decodeIRR(t, irrStatusAnswer(t))
	rows, isRows := envelope["entries"].([]any)
	if !isRows || len(rows) == 0 {
		t.Fatalf("the show bgp irr answer carries no entries: %v", envelope)
	}
	records := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		record, isRecord := row.(map[string]any)
		if !isRecord {
			t.Fatalf("an entry is %T, want a record", row)
		}
		records = append(records, record)
	}
	return records
}

// decodeIRR reads a payload as an operator's chain reads an answer.
func decodeIRR(t *testing.T, payload string) map[string]any {
	t.Helper()

	var decoded map[string]any
	if err := json.Unmarshal([]byte(payload), &decoded); err != nil {
		t.Fatalf("the payload does not decode: %v", err)
	}
	return decoded
}

// sortedNames names a key set for a failure message.
func sortedNames(keys map[string]struct{}) []string {
	names := make([]string, 0, len(keys))
	for name := range keys {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
