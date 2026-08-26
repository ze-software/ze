// Design: docs/architecture/doctor-and-health-checks.md -- the port gate's answer
//
// report.go holds what `le port-defaults check` ANSWERS, apart from what
// produced it.
//
// The answer is the drift list plus the count of services it compared, which is
// structured data: `| json` feeds a script, `| match port-mismatch` keeps one
// reason. The report also renders ITSELF (Text), because the gate's page is
// what a person reads here (letools/leroot, Prose).

package portdefaults

import "github.com/ze-software/ze/internal/core/textbuf"

// Drift reason codes (stable, machine-readable). They are the script's own
// spellings, and they are what `| match` selects on.
const (
	// ReasonUnmapped: service in the Go table, no YANG module mapping.
	ReasonUnmapped = "unmapped-service"
	// ReasonUnreadable: the mapped YANG module could not be read.
	ReasonUnreadable = "unreadable-yang"
	// ReasonNoDefault: no single `refine port { default N }` in the module.
	ReasonNoDefault = "no-refine-default"
	// ReasonMismatch: the Go default and the YANG default disagree.
	ReasonMismatch = "port-mismatch"
	// ReasonStaleMap: module mapped but the service is no longer in the Go table.
	ReasonStaleMap = "stale-mapping"
	// ReasonUnknownReg: a registration spelling the gate cannot read, which is
	// refused rather than skipped.
	ReasonUnknownReg = "unknown-registration"
)

// Drift is one disagreement between the Go listener table and a YANG module,
// and it is one ROW of the answer. The keys are the script's, unchanged.
type Drift struct {
	Service  string `json:"service"`
	GoPort   int    `json:"go-port"`
	YANGPort int    `json:"yang-port"`
	File     string `json:"file,omitempty"`
	Reason   string `json:"reason"`
}

// Result is the whole answer of one run.
//
// Drifts is the only row set in it, and Checked is what tells a clean table
// from a read that enumerated nothing.
type Result struct {
	Drifts  []Drift `json:"drifts"`
	Checked int     `json:"services-checked"`
	Valid   bool    `json:"valid"`
}

// Text renders the result for a person, in the page the script printed: the
// heading, the count, then the drift list or the verdict. It ends in a newline.
func (r Result) Text() string {
	var tb textbuf.Buffer
	tb.Str("# Listener Port-Default Gate\n\n")
	tb.Str("Services checked: ").Int(int64(r.Checked)).Str("\n\n")

	if len(r.Drifts) == 0 {
		tb.Str("port-defaults: OK\n")
		return tb.String()
	}

	tb.Str("## Drift (").Int(int64(len(r.Drifts))).Str(")\n\n")
	for _, drift := range r.Drifts {
		tb.Str("  [").Str(drift.Reason).Str("] service=").Str(drift.Service).
			Str(" go-port=").Int(int64(drift.GoPort)).
			Str(" yang-port=").Int(int64(drift.YANGPort)).
			Byte(' ').Str(drift.File).Byte('\n')
	}
	tb.Str("\nport-defaults: FAILED\n")
	return tb.String()
}
