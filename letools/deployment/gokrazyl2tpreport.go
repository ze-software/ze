// Design: ai/rules/cli.md -- one payload, and the operator picks the rendering
// Related: l2tppppreport.go -- the on-host proof's own payload
// Related: gokrazylab.go -- the lab whose names this payload carries
//
// gokrazyl2tpreport.go is what the appliance L2TP PPP proof answers. It is data,
// so `| json`, `| yaml` and `| table` each render it with no code here. Text is
// the second reading of the same data for a person who typed no operator.

package deployment

import (
	"github.com/ze-software/ze/internal/core/textbuf"
)

// GokrazyL2TPReport is one run of the appliance L2TP PPP proof.
//
// A reader checks the two interfaces as evidence. The appliance's interface
// comes from its own serial console because nothing on the host can see inside
// the virtual machine. The peer's interface comes from the host kernel. Both
// names tell the reader which end each assertion was made at.
type GokrazyL2TPReport struct {
	Peer               string   `json:"peer"`
	Arch               string   `json:"arch"`
	Image              string   `json:"image"`
	Accel              string   `json:"accel"`
	Namespace          string   `json:"namespace"`
	ApplianceAddress   string   `json:"appliance-address"`
	ApplianceInterface string   `json:"appliance-interface"`
	PeerInterface      string   `json:"peer-interface"`
	LocalAddress       string   `json:"local-address"`
	PeerAddress        string   `json:"peer-address"`
	Proven             bool     `json:"proven"`
	Reason             string   `json:"reason"`
	LogTail            []string `json:"log-tail"`
}

// Text renders the run for a person in the shape that the Python original
// printed. For a pass, one line names both interfaces. For a failure, the output
// gives the reason and the appliance's last console lines.
func (r GokrazyL2TPReport) Text() string {
	var tb textbuf.Buffer

	if r.Proven {
		tb.Str("OK: gokrazy Ze appliance completed real L2TP PPP/IPCP with Ze ").
			Str(r.ApplianceInterface).Str(" and LAC ").Str(r.PeerInterface).
			Str(", dataplane ping, route inject, and clean teardown\n")
		return tb.String()
	}

	tb.Str("FAIL: ").Str(r.Reason).Byte('\n')
	if len(r.LogTail) > 0 {
		tb.Str("appliance console tail:\n")
		for _, line := range r.LogTail {
			tb.Str(line).Byte('\n')
		}
	}
	return tb.String()
}
