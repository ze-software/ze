// Design: docs/architecture/isis/isis-13-cli-diag-interop.md -- engine-side render + clear for
// `show isis hostname/interface/spf-log` and `clear isis adjacency/counters`.
// Related: register.go -- OnExecuteCommand dispatches these commands here
// Related: lsdb_wiring.go -- databaseSnapshot/neighborSnapshot this complements
// Related: spf_wiring.go -- spfLogSnapshot this exposes via `show isis spf-log`
//
// RFC: rfc/short/rfc5301.md -- TLV 137 Dynamic Hostname (display only here; isis-6 advertises)
//
// This file is the isis-13 presentation layer: it reads the sibling engine
// snapshots (adjacency, LSDB, SPF) and renders the remaining `show isis` nouns
// (hostname, interface detail, spf-log) plus the two runtime clear actions. It
// originates no protocol state -- every render is a read-only projection of state
// the siblings own, and the clear actions only reset observable/learned state so
// the engine re-derives it.

package isis

import (
	"maps"
	"sort"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/plugins/isis/adjacency"
	"github.com/ze-software/ze/internal/plugins/isis/circuit"
	"github.com/ze-software/ze/internal/plugins/isis/lsdb"
	"github.com/ze-software/ze/internal/plugins/isis/packet"
)

// hostnameRow is one `show isis hostname` entry: the System ID -> dynamic
// hostname mapping learned from TLV 137 (RFC 5301). Flat value, JSON-tagged for
// the pipe machinery.
type hostnameRow struct {
	SystemID string `json:"system-id"`
	Hostname string `json:"hostname"`
	Level    string `json:"level"`
	Local    bool   `json:"local,omitempty"`
}

// hostnameSnapshot builds the `show isis hostname` view (spec-isis-13 AC-5): for
// every router LSP (fragment 0, pseudonode 0) held in either level, it extracts
// the Dynamic Hostname TLV (137, RFC 5301 sec 3) and maps the originator System
// ID to that name. Display only -- advertisement of the local hostname is isis-6.
// A System ID with no TLV 137 in its LSP is omitted (no name learned). The local
// node's own System ID is flagged so the operator can tell self from peers.
func (e *engine) hostnameSnapshot() []any {
	out := make([]any, 0)
	if e.lsdb == nil {
		return out
	}
	e.mu.Lock()
	own := e.cfg.SystemID
	e.mu.Unlock()

	// De-duplicate by (system-id, level): a node advertises one hostname; we read
	// it from fragment 0. Later fragments do not carry TLV 137.
	var keyBuf textbuf.Buffer
	seen := make(map[string]bool)
	rows := make([]hostnameRow, 0)
	for _, lvl := range []lsdb.Level{lsdb.Level1, lsdb.Level2} {
		for _, id := range e.lsdb.LSPIDs(lvl) {
			// Only router LSP fragment 0 carries the Dynamic Hostname (RFC 5301
			// sec 3); skip pseudo-node LSPs and non-zero fragments.
			if id.PseudonodeID() != 0 || id.LSPNumber() != 0 {
				continue
			}
			entry := e.lsdb.Lookup(lvl, id)
			if entry == nil || entry.IsPurged() {
				continue
			}
			lsp, err := entry.Decode()
			if err != nil {
				continue
			}
			name := hostnameFromLSP(&lsp)
			packet.ReleaseTLVs(lsp.TLVs)
			if name == "" {
				continue
			}
			sysTok := id.SystemID().String()
			key := keyBuf.Reset().Str(sysTok).Byte('|').Str(lvl.String()).String()
			if seen[key] {
				continue
			}
			seen[key] = true
			rows = append(rows, hostnameRow{
				SystemID: sysTok,
				Hostname: name,
				Level:    lvl.String(),
				Local:    id.SystemID() == own,
			})
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].SystemID != rows[j].SystemID {
			return rows[i].SystemID < rows[j].SystemID
		}
		return rows[i].Level < rows[j].Level
	})
	for _, r := range rows {
		out = append(out, r)
	}
	return out
}

// hostnameFromLSP returns the Dynamic Hostname (TLV 137) carried in lsp, or ""
// when absent. RFC 5301 sec 3: the value is a 1..255 byte 7-bit-ASCII hostname,
// not null-terminated; it is returned verbatim as a string. Non-printable bytes
// are dropped defensively so a malformed advertisement cannot corrupt the CLI
// output (security review: rendering bounds the data it does not own).
func hostnameFromLSP(lsp *packet.LSP) string {
	for i := range lsp.TLVs {
		t := &lsp.TLVs[i]
		if t.Type != packet.TLVDynamicHostname || len(t.Value) == 0 {
			continue
		}
		return sanitizeHostname(t.Value)
	}
	return ""
}

// sanitizeHostname renders the TLV 137 value as a printable ASCII string,
// dropping control and non-ASCII bytes. RFC 5301 sec 3 specifies 7-bit ASCII;
// anything outside printable ASCII is a malformed advertisement and is elided
// rather than emitted into the CLI.
func sanitizeHostname(v []byte) string {
	b := make([]byte, 0, len(v))
	for _, c := range v {
		if c >= 0x20 && c < 0x7f {
			b = append(b, c)
		}
	}
	return string(b)
}

// interfaceRow is one `show isis interface` entry (spec-isis-13 AC-4): the
// circuit name, level, type, metric, hello/hold timers, passive flag, and the
// DIS state on a broadcast circuit. Flat value, JSON-tagged for the pipes.
type interfaceRow struct {
	Name          string `json:"name"`
	Level         string `json:"level"`
	CircuitType   string `json:"circuit-type"`
	Metric        uint32 `json:"metric"`
	HelloInterval uint16 `json:"hello-interval"`
	HoldMulti     uint8  `json:"hold-multiplier"`
	Passive       bool   `json:"passive"`
	AdjacenciesUp int    `json:"adjacencies-up"`
	// DIS reports whether the local node is the elected DIS on this broadcast
	// circuit at either level ("l1", "l2", "l1-l2", or "" for P2P / not DIS).
	DIS string `json:"dis,omitempty"`
	// Authenticated reports whether an auth key chain is configured on the
	// circuit. It NEVER exposes key material (security review: no key leak).
	Authenticated bool `json:"authenticated,omitempty"`
}

// interfaceSnapshot builds the `show isis interface` view (spec-isis-13 AC-4).
// It merges the configured per-interface parameters (every configured circuit,
// including passive ones with no live socket) with the runtime DIS/adjacency
// state read from the live circuits. Passive circuits and circuits whose link is
// still down have no live circuit, so their DIS/adjacency columns are empty.
func (e *engine) interfaceSnapshot() []any {
	// Live circuits keyed by name for the DIS/adjacency overlay.
	e.circuitsMu.RLock()
	live := make(map[string]*circuit.Circuit, len(e.circuitByName))
	maps.Copy(live, e.circuitByName)
	e.circuitsMu.RUnlock()

	// Every configured interface (enabled or passive). running holds the opened
	// non-passive set; cfg.Interfaces holds the full configured set including
	// passive ones, so iterate cfg.Interfaces for completeness.
	e.mu.Lock()
	cfgIfaces := append([]InterfaceConfig(nil), e.cfg.Interfaces...)
	e.mu.Unlock()

	rows := make([]interfaceRow, 0, len(cfgIfaces))
	for _, ic := range cfgIfaces {
		if !ic.Enabled {
			continue
		}
		row := interfaceRow{
			Name:          ic.Name,
			Level:         ic.Level.String(),
			CircuitType:   ic.CircuitType.String(),
			Metric:        ic.Metric,
			HelloInterval: ic.HelloInterval,
			HoldMulti:     ic.HoldMult,
			Passive:       ic.Passive,
			Authenticated: ic.Level1.AuthKeyChain != "" || ic.Level2.AuthKeyChain != "",
		}
		if c := live[ic.Name]; c != nil {
			row.AdjacenciesUp = c.Table().UpCount()
			row.DIS = disState(c)
		}
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Name < rows[j].Name })
	out := make([]any, 0, len(rows))
	for _, r := range rows {
		out = append(out, r)
	}
	return out
}

// disState reports the levels on which the local node is the elected DIS for the
// circuit ("l1", "l2", "l1-l2", or "" when not DIS / P2P). The own-LSP star
// encoding and LAN CSNP cadence use the same LocalIsDIS query.
func disState(c *circuit.Circuit) string {
	l1 := c.LocalIsDIS(adjacency.Level1)
	l2 := c.LocalIsDIS(adjacency.Level2)
	switch {
	case l1 && l2:
		return "l1-l2"
	case l1:
		return "l1"
	case l2:
		return "l2"
	default:
		return ""
	}
}

// spfLogView renders the `show isis spf-log` rows (spec-isis-13 AC-6). It is a
// thin projection of the SPF Computer's bounded history (spf.SPFLogEntry),
// returned as []any so the pipe machinery treats each run as a row.
func (e *engine) spfLogView() []any {
	entries := e.spfLogSnapshot()
	out := make([]any, 0, len(entries))
	for _, en := range entries {
		out = append(out, en)
	}
	return out
}

// clearAdjacencies tears down every adjacency on every live circuit (the `clear
// isis adjacency` runtime action, spec-isis-13 AC-8). Neighbors re-form from the
// next Hello. It returns the number of adjacency records dropped. Losing an Up
// adjacency triggers the circuit's down hook (LSP re-origination, SPF re-run),
// so routes reconverge without a restart.
func (e *engine) clearAdjacencies() int {
	e.circuitsMu.RLock()
	circuits := make([]*circuit.Circuit, 0, len(e.circuitByName))
	for _, c := range e.circuitByName {
		circuits = append(circuits, c)
	}
	e.circuitsMu.RUnlock()
	total := 0
	for _, c := range circuits {
		total += c.Table().Clear()
	}
	return total
}

// clearCounters resets the IS-IS observational counters/logs that an operator
// can safely zero without disturbing routing (the `clear isis counters` runtime
// action, spec-isis-13 AC-8). The Prometheus series are monotonic process
// counters owned by their producing subsystems and are NOT reset here (resetting
// a Prometheus counter mid-process breaks rate() math); instead this clears the
// SPF-run history ring, the engine's diagnostic state that is purely a record of
// past events. The dispatcher's dropped-PDU tally is also a process counter and
// is left intact.
func (e *engine) clearCounters() {
	if e.spf != nil {
		e.spf.ResetSPFLog()
	}
}
