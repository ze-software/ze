// Design: docs/labs/l2tp-interop.md -- the full PPPoL2TP diagnostic vocabulary
// Related: internal/le/deployment/l2tpdiag_linux.go -- the tunnel-only diagnostic vocabulary
// Detail: l2tpdiag_linux.go -- the Linux operations that fill this report
//
// Both L2TP diagnostics answer one report type. Text preserves the producer's
// terminal page. The other renderers expose the verdict, dumps, retained kernel
// objects, and exact operation transcript as data.

package deployment

// L2TPDiagnosticVerdict is the result of a diagnostic that reached a proof.
// The zero value is not a verdict. Setup and create errors return it with an
// operating error instead of publishing a failed kernel proof.
type L2TPDiagnosticVerdict string

const (
	L2TPDiagnosticWorking L2TPDiagnosticVerdict = "working"
	L2TPDiagnosticFailed  L2TPDiagnosticVerdict = "failed"
)

// L2TPDiagnosticObject is one kernel object created by a diagnostic.
// Retained is true when the producer issued no matching delete operation.
type L2TPDiagnosticObject struct {
	Kind     string `json:"kind"`
	ID       uint32 `json:"id"`
	PeerID   uint32 `json:"peer-id"`
	Retained bool   `json:"retained"`
}

// L2TPDiagnosticDump records one kernel readback. A failed dump is a note and
// does not replace the verdict reached by the remaining diagnostic steps.
type L2TPDiagnosticDump struct {
	Kind     string `json:"kind"`
	Messages int    `json:"messages"`
	Note     string `json:"note,omitempty"`
}

// l2tpDiagnosticReport is the shared answer of both L2TP diagnostics.
type l2tpDiagnosticReport struct {
	Diagnostic string                 `json:"diagnostic"`
	Verdict    L2TPDiagnosticVerdict  `json:"verdict,omitempty"`
	Dumps      []L2TPDiagnosticDump   `json:"dumps"`
	Retained   []L2TPDiagnosticObject `json:"retained"`
	Operations []string               `json:"operations"`
	Output     string                 `json:"output"`
}

// Text preserves the complete terminal page of the source diagnostic.
func (r l2tpDiagnosticReport) Text() string { return r.Output }

// diagnosticExitCode maps the proof vocabulary to the process contract.
func diagnosticExitCode(verdict L2TPDiagnosticVerdict, err error) int {
	if err != nil || verdict == "" || verdict == L2TPDiagnosticFailed {
		return 1
	}
	if verdict == L2TPDiagnosticWorking {
		return 0
	}
	return 1
}
