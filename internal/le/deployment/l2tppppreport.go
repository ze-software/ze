// Design: ai/rules/cli.md -- one payload, and the operator picks the rendering
// Overview: l2tpppp.go -- the run that fills this in
// Related: report.go -- the container L2TP proof's own payload
// Related: gokrazyl2tpreport.go -- the same proof's payload on the appliance
//
// l2tppppreport.go defines the answer from the on-host L2TP PPP proof. It is
// data, so `| json`, `| yaml` and `| table` each render it without code here.
// Text is a second reading of the same data for a person who typed no operator.

package deployment

import (
	"github.com/ze-software/ze/internal/core/textbuf"
)

// L2TPPPPReport is one run of the on-host L2TP PPP proof.
//
// The two interface names are evidence that a reader checks. A PPP session that
// completed IPCP left a pppN interface in each namespace. Each interface
// carries the address pair. The names tell the reader of a green run which
// kernel objects the proof asserted about.
//
// LogTail contains the daemon's last lines only when the proof did NOT complete.
// It is the evidence for a failure. A run that passed has nothing to explain.
type L2TPPPPReport struct {
	Peer         string   `json:"peer"`
	ZeNamespace  string   `json:"ze-namespace"`
	LACNamespace string   `json:"lac-namespace"`
	ZeInterface  string   `json:"ze-interface"`
	LACInterface string   `json:"lac-interface"`
	LocalAddress string   `json:"local-address"`
	PeerAddress  string   `json:"peer-address"`
	Proven       bool     `json:"proven"`
	Reason       string   `json:"reason"`
	LogTail      []string `json:"log-tail"`
}

// Text renders the run for a person in the shape that the Python original
// printed. A pass is one line that names both interfaces. A failure contains
// the reason and the daemon's last lines.
func (r L2TPPPPReport) Text() string {
	var tb textbuf.Buffer

	if r.Proven {
		tb.Str("OK: real ").Str(r.Peer).Str(" peer completed PPP LCP, IPCP, Ze ").Str(r.ZeInterface).
			Str(" and LAC ").Str(r.LACInterface).
			Str(" address assignment, dataplane ping, route inject, and clean teardown\n")
		return tb.String()
	}

	tb.Str("FAIL: ").Str(r.Reason).Byte('\n')
	if len(r.LogTail) > 0 {
		tb.Str("ze log tail:\n")
		for _, line := range r.LogTail {
			tb.Str(line).Byte('\n')
		}
	}
	return tb.String()
}
