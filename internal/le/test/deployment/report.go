// Design: ai/rules/cli.md -- one payload, and the operator picks the rendering
// Overview: l2tp.go -- the run that fills this in
// Related: vppifacereport.go -- the VPP interface proof's own payload
// Related: l2tppppreport.go -- the on-host L2TP PPP proof's own payload
//
// report.go is what a deployment proof answers. It is data, so `| json`,
// `| yaml` and `| table` each render it with no code here, and Text is the
// second reading of the same data for a person who typed no operator.

package testdeployment

import (
	"github.com/ze-software/ze/internal/core/textbuf"
)

// L2TPReport is one run of the L2TP peer proof.
//
// LogTail is the daemon's last lines, and it is filled only when the session
// did NOT establish: it is the evidence for a failure, and a run that passed
// has nothing to explain.
type L2TPReport struct {
	Peer        string   `json:"peer"`
	Image       string   `json:"image"`
	Container   string   `json:"container"`
	Established bool     `json:"established"`
	LogTail     []string `json:"log-tail"`
	// ContainerRemoval names why the proof's container could not be removed;
	// empty when it was. A container left behind fails the verb.
	ContainerRemoval string `json:"container-removal-error,omitempty"`
}

// Text renders the run for a person, in the shape the Python original printed:
// one line for a pass, and the reason plus the daemon's last lines for a
// failure.
func (r L2TPReport) Text() string {
	var tb textbuf.Buffer

	writeContainerRemoval(&tb, r.Container, r.ContainerRemoval)
	if r.Established {
		tb.Str("OK: a real ").Str(r.Peer).Str(" peer established an L2TP session\n")
		return tb.String()
	}

	tb.Str("FAIL: ").Str(r.Peer).Str(" session establishment not observed\n")
	if len(r.LogTail) > 0 {
		tb.Str("ze log tail:\n")
		for _, line := range r.LogTail {
			tb.Str(line).Byte('\n')
		}
	}
	return tb.String()
}

// proofNotReached is what a deployment proof answers when its run returned an
// error: a step it could not perform, or a Ctrl-C reaching the verb's signal
// root. The run reached no verdict, so this payload carries none, neither the
// partial report's "established: false" nor an unreached scenario's FAIL. It
// keeps only the container the run started and why it could not be removed,
// because a leftover is still the operator's to clean. The error itself is
// reported on stderr by the answer function.
type proofNotReached struct {
	Container        string `json:"container,omitempty"`
	ContainerRemoval string `json:"container-removal-error,omitempty"`
}

// Text renders only the container removal a reader must act on.
func (r proofNotReached) Text() string {
	var tb textbuf.Buffer
	writeContainerRemoval(&tb, r.Container, r.ContainerRemoval)
	return tb.String()
}

// writeContainerRemoval names a proof container the run could not remove,
// ahead of the verdict a reader looks for. The removal text already ends with
// the command that removes it (removeContainer), so it is not repeated here.
func writeContainerRemoval(tb *textbuf.Buffer, container, removal string) {
	if removal == "" {
		return
	}
	tb.Str("FAIL: container ").Str(container).Str(" was not removed: ").Str(removal).Str("\n")
}
