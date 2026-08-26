// Design: ai/rules/cli.md -- one payload, and the operator picks the rendering
// Overview: l2tp.go -- the run that fills this in
// Related: vppifacereport.go -- the VPP interface proof's own payload
// Related: l2tppppreport.go -- the on-host L2TP PPP proof's own payload
//
// report.go is what a deployment proof answers. It is data, so `| json`,
// `| yaml` and `| table` each render it with no code here, and Text is the
// second reading of the same data for a person who typed no operator.

package deployment

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
}

// Text renders the run for a person, in the shape the Python original printed:
// one line for a pass, and the reason plus the daemon's last lines for a
// failure.
func (r L2TPReport) Text() string {
	var tb textbuf.Buffer

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
