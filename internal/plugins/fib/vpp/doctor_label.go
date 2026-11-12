// Design: docs/architecture/mpls/mpls-kernel.md -- label-switching entries on the VPP data plane
// Related: mplsentry.go -- warnPathMTULocked records the dropped path MTU this check reports
// Related: register_doctor.go -- registers the explanation of each code this check reports
// Related: mplsstale.go -- the stale-label sweep whose failures this check reports
//
// The fib-vpp-label-entries check reports what the writer's own state knows
// and no label source can: an entry installed without the path MTU it
// carried, because VPP has no per-route MTU, and a label route a previous
// run left in MPLS table 0 that the stale-label sweep (mplsstale.go) could
// not remove or could not look for. It runs in the runtime phase,
// through fib-vpp's own doctor callback, because the writer lives in fib-vpp's
// process, which may be a forked one. An entry the writer refused reaches its
// label source as an error, so the source's own check reports it under
// mplsfib.CodeDoctorNoWriter.
package fibvpp

import (
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// labelEntriesCheckName is the runtime doctor check fib-vpp answers itself.
const labelEntriesCheckName = "fib-vpp-label-entries"

// codePathMTUUnenforced names label entries installed without their path MTU.
const codePathMTUUnenforced = "doctor-fib-vpp-mpls-path-mtu"

// codeStaleLabel names a label entry a previous fib-vpp writer left in VPP
// that this writer could not remove, or could not look for.
const codeStaleLabel = "doctor-fib-vpp-mpls-stale-label"

// labelFindingsMax bounds the stale label routes one answer lists; the rest
// are counted in one more finding.
const labelFindingsMax = 32

// staleLabelFix is the command that runs the sweep again: the restarted
// daemon opens a new writer, which reads MPLS table 0 and removes every
// label route no source re-claims.
const staleLabelFix = "ze signal restart"

// labelEntriesCheckDecl declares the runtime check in fib-vpp's Stage 1
// registration, so `show doctor` asks fib-vpp's process for it.
var labelEntriesCheckDecl = rpc.DoctorCheckDecl{
	Name:  labelEntriesCheckName,
	Phase: rpc.DoctorPhaseRuntime,
	Codes: []string{codePathMTUUnenforced, codeStaleLabel},
}

// staleLabelCodeMeta is the explanation `ze show diagnostic` prints for
// codeStaleLabel.
var staleLabelCodeMeta = diagnostic.CodeMeta{
	Code:  codeStaleLabel,
	Title: "A label route a previous run installed in VPP is still forwarding",
	Description: "VPP keeps its FIB when Ze stops, so the label routes a previous fib-vpp writer installed in MPLS " +
		"table 0 survive a Ze restart. When fib-vpp opens its writer it reads MPLS table 0, asks every label source " +
		"(LDP, RSVP-TE, OSPF-SR, BGP EPE) to re-apply the entries it wants, and after 30 seconds removes every label " +
		"route with an unreserved in-label that no source re-claimed. This ERROR means VPP refused to remove one, or " +
		"MPLS table 0 could not be read, so a label nobody advertises may still forward packets. A push (a labeled " +
		"IP route) is not in MPLS table 0: the stale IP route sweep removes one a previous run left, and reports " +
		"what it could not remove under doctor-fib-vpp-stale-route. Remedy: restart Ze with `ze signal restart`, which opens a new writer and runs the sweep again; " +
		"the finding names VPP's refusal when it repeats.",
	Examples: []string{"ze show doctor", "ze show diagnostic doctor-fib-vpp-mpls-stale-label"},
}

// pathMTUCodeMeta is the explanation `ze show diagnostic` prints for
// codePathMTUUnenforced.
var pathMTUCodeMeta = diagnostic.CodeMeta{
	Code:  codePathMTUUnenforced,
	Title: "Label entries on VPP are installed without their path MTU",
	Description: "A label source (RSVP-TE) asked for a label entry bounded by a path MTU, and VPP has no per-route MTU, " +
		"so fib-vpp installed the entry without it. Labeled frames on that LSP are bounded by the MTU of the outgoing " +
		"interface alone: a frame larger than the path MTU the LSP reserved leaves the interface and is dropped further " +
		"along the path rather than refused here. This is a WARNING. Remedy: set the MTU of each MPLS interface to the " +
		"smallest path MTU the LSPs need, with `set interface <type> <name> mtu <bytes>` (for example `set interface ethernet eth0 mtu 1500`).",
	Examples: []string{"ze show doctor", "ze show diagnostic doctor-fib-vpp-mpls-path-mtu"},
}

// pathMTUFix is the command that bounds labeled frames by the path MTU.
const pathMTUFix = "set interface <type> <name> mtu <bytes>"

// labelDoctorFindings answers the fib-vpp-label-entries check for this
// writer. Safe for concurrent use: it takes f.mu.
func (f *fibVPP) labelDoctorFindings() []rpc.DoctorCheckDiagnostic {
	f.mu.Lock()
	defer f.mu.Unlock()
	var findings []rpc.DoctorCheckDiagnostic
	if f.pathMTUWarned {
		findings = append(findings, rpc.DoctorCheckDiagnostic{
			Code:     codePathMTUUnenforced,
			Severity: string(diagnostic.SeverityWarning),
			Message: "fib-vpp: label entries are installed without the path MTU their label source asked for, " +
				"because VPP has no per-route MTU; labeled frames are bounded by the interface MTU alone",
			Fix: pathMTUFix,
		})
	}
	return append(findings, f.staleLabelFindingsLocked()...)
}

// staleLabelFindingsLocked reports a table 0 the writer could not read, and
// the leftover label routes the sweep could not remove, at most
// labelFindingsMax of them and a count of the rest. Caller holds f.mu.
func (f *fibVPP) staleLabelFindingsLocked() []rpc.DoctorCheckDiagnostic {
	var findings []rpc.DoctorCheckDiagnostic
	if f.labelDumpErr != nil {
		var tb textbuf.Buffer
		tb.Str("fib-vpp: MPLS table 0 could not be read when the writer opened (").Str(f.labelDumpErr.Error()).
			Str("), so label routes a previous run left in VPP are neither known nor removed and may still forward")
		findings = append(findings, staleLabelFinding(tb.String()))
	}
	failures := f.staleLabelFailuresLocked()
	shown := min(len(failures), labelFindingsMax)
	for _, p := range failures[:shown] {
		var tb textbuf.Buffer
		tb.Str("fib-vpp: in-label ").Int(int64(p.inLabel)).Str(" (").Str(eosName(p.eos)).
			Str(") was left in MPLS table 0 by a previous run, no label source claims it, and VPP refused to remove it (").
			Str(f.labelSweepFailures[p].Error()).Str("), so it still forwards")
		findings = append(findings, staleLabelFinding(tb.String()))
	}
	if rest := len(failures) - shown; rest > 0 {
		var tb textbuf.Buffer
		tb.Str("fib-vpp: ").Int(int64(rest)).
			Str(" more label routes left in MPLS table 0 by a previous run could not be removed")
		findings = append(findings, staleLabelFinding(tb.String()))
	}
	return findings
}

// staleLabelFinding is one codeStaleLabel error carrying message.
func staleLabelFinding(message string) rpc.DoctorCheckDiagnostic {
	return rpc.DoctorCheckDiagnostic{
		Code:     codeStaleLabel,
		Severity: string(diagnostic.SeverityError),
		Message:  message,
		Fix:      staleLabelFix,
	}
}

// eosName names a stack position the way VPP's `show mpls fib` does.
func eosName(eos uint8) string {
	if eos == 1 {
		return "eos"
	}
	return "neos"
}
