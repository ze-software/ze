// Design: ai/rules/cli.md -- one payload, and the operator picks the rendering
// Overview: hugepages.go -- the run that fills this in
// Related: boot.go -- where the verdict is decided
//
// report.go defines the answer from a QEMU proof. The answer is data. Thus,
// `| json`, `| yaml` and `| table` each render it with no code here. Text
// presents the same data to a person who typed no operator.
//
// The report puts what was ASKED for beside what was OBSERVED. The requested
// values are the page size, the page count and the memory the appliance was
// configured with. The observed values are the returned kernel command line
// and the pages that the kernel actually reserved. A verdict alone does not
// distinguish a run that proved the reservation from one that asked for none.

package qemu

import (
	"github.com/ze-software/ze/internal/core/textbuf"
)

// Verdict is what one QEMU proof concluded.
//
// The zero value is Unspecified. Thus, a run that ends before a conclusion
// cannot appear as a pass. Verdict is a typed number instead of a `skipped`
// boolean and a `passed` boolean. Those booleans are two fields for one fact,
// and their false-false combination has no name.
//
// internal/le/deployment declares an Outcome of the same shape for its scenarios.
// The two are NOT shared yet. This migration uses the same rule that it applied
// to its action tables. The third area that needs this shape lifts it into
// internal/le/leroot. It does not write a third.
type Verdict uint8

const (
	// VerdictUnspecified is a run that reached no conclusion.
	VerdictUnspecified Verdict = iota
	// VerdictPass is a run whose appliance carried the reservation and whose
	// kernel honored it.
	VerdictPass
	// VerdictSkip is a run that this machine was unable to perform. The cause is an
	// absent prerequisite or software emulation that was too slow.
	VerdictSkip
	// VerdictFail is a run whose appliance did not carry the reservation, or
	// whose kernel did not honor it.
	VerdictFail
)

// String answers the word a person and a JSON document both read.
func (v Verdict) String() string {
	switch v {
	case VerdictPass:
		return "pass"
	case VerdictSkip:
		return "skip"
	case VerdictFail:
		return "fail"
	case VerdictUnspecified:
		return "unspecified"
	}
	return "unspecified"
}

// MarshalJSON writes the word rather than the number, so `| json` and `| yaml`
// carry a value a reader and a script can both use.
func (v Verdict) MarshalJSON() ([]byte, error) {
	var tb textbuf.Buffer
	return []byte(tb.Quoted(v.String()).String()), nil
}

// ReportPrefix opens every line the proof prints. It is the word the functional
// suite greps for, so it is stated once here rather than in each sentence.
const ReportPrefix = "VPP-HUGEPAGES-QEMU: "

// HugepagesReport is one run of the boot-time hugepage reservation proof.
//
// ConsoleTail contains the appliance's serial console only when the appliance
// never answers under a hardware accelerator. It is the only evidence of why
// the appliance never answered. A run that got an answer has nothing to
// explain.
type HugepagesReport struct {
	Verdict     Verdict  `json:"verdict"`
	Reason      string   `json:"reason,omitempty"`
	Arch        string   `json:"arch"`
	Accelerator string   `json:"accelerator"`
	PageSize    string   `json:"page-size"`
	PageToken   string   `json:"page-token"`
	Reservation string   `json:"reservation"`
	Pages       uint64   `json:"pages"`
	MemoryMiB   uint64   `json:"memory-mib"`
	Cmdline     string   `json:"cmdline,omitempty"`
	PagesTotal  uint64   `json:"hugepages-total"`
	ConsoleTail []string `json:"console-tail,omitempty"`
}

// Text renders the run for a person in the shape that the Python original
// printed. One prefixed line contains the verdict. If a failure has a serial
// console, the console follows that line.
func (r HugepagesReport) Text() string {
	var tb textbuf.Buffer
	tb.Str(ReportPrefix)

	switch r.Verdict {
	case VerdictPass:
		tb.Str("PASS cmdline has hugepages=").Uint(r.Pages).
			Str(", hugepages-total=").Uint(r.PagesTotal).Byte('\n')
		return tb.String()
	case VerdictSkip:
		tb.Str("SKIP ").Str(r.Reason).Byte('\n')
		return tb.String()
	case VerdictFail, VerdictUnspecified:
		tb.Str("FAIL ").Str(r.Reason).Byte('\n')
	}

	if len(r.ConsoleTail) > 0 {
		tb.Str("  serial console tail:\n")
		for _, line := range r.ConsoleTail {
			tb.Str("    ").Str(line).Byte('\n')
		}
	}
	return tb.String()
}
