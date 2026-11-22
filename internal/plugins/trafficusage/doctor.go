// Design: docs/architecture/traffic/traffic-usage.md -- traffic-usage runtime doctor check: the attach outcome per configured interface
// Related: monitor.go -- reconcileLocked records the outcomes this check reads

package trafficusage

import (
	"errors"
	"slices"
	"syscall"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// attachCheckName is the runtime doctor check this plugin declares at Stage 1
// and answers from its doctor callback.
const attachCheckName = "traffic-usage-attach"

// The codes the attach check reports. The log line that records the same
// outcome, when it changes, carries the same code.
const (
	// codeAttachRefused names an interface whose programs the kernel refused
	// to load or attach for lack of permission.
	codeAttachRefused = "doctor-traffic-usage-attach-refused"
	// codeTCXAbsent names an interface left unaccounted because this kernel or
	// build has no eBPF TCX attach point.
	codeTCXAbsent = "doctor-traffic-usage-tcx-absent"
	// codeAttachUnknown names an interface whose attach failed with an answer
	// that establishes no cause.
	codeAttachUnknown = "doctor-traffic-usage-attach-unknown"
	// codeInterfaceMissing names a configured interface no device resolves to.
	codeInterfaceMissing = "doctor-traffic-usage-interface-missing"
)

// attachCheckDecl declares the runtime check. It runs only where the plugin
// runs, through its doctor callback, because the outcomes live in the
// monitor's memory (docs/architecture/doctor-and-health-checks.md, "The
// runtime phase: checks only the daemon runs").
func attachCheckDecl() rpc.DoctorCheckDecl {
	return rpc.DoctorCheckDecl{
		Name:  attachCheckName,
		Phase: rpc.DoctorPhaseRuntime,
		Codes: []string{codeAttachRefused, codeTCXAbsent, codeAttachUnknown, codeInterfaceMissing},
	}
}

// exampleDoctorJSON is the doctor run every traffic-usage code's explanation shows first.
const exampleDoctorJSON = "ze doctor --json"

// trafficUsageDiagnosticCodes is the explanation metadata of the codes this
// plugin owns. register.go registers it from this plugin's init rather than
// from the always-on code table, so a build without the plugin holds none of
// them.
var trafficUsageDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:  codeAttachRefused,
		Title: "traffic-usage eBPF attach refused",
		Description: "traffic-usage is enabled and the kernel refused, for lack of permission, to load or attach " +
			"its eBPF TCX programs on a configured interface, so that interface is not accounted. The process " +
			"that loads them is the ze daemon, which needs CAP_BPF and CAP_NET_ADMIN. The finding's fix names " +
			"the systemd repair only when ze established that the daemon runs as the ze service, and otherwise " +
			"the capabilities to start ze with.",
		Examples:    []string{exampleDoctorJSON, "ze show diagnostic " + codeAttachRefused},
		HostVerdict: true,
	},
	{
		Code:  codeTCXAbsent,
		Title: "traffic-usage eBPF TCX unavailable",
		Description: "traffic-usage is enabled but this kernel or build cannot attach eBPF TCX programs (Linux " +
			"6.6 or later built with CONFIG_NET_XGRESS is required), so the plugin runs inert and no interface " +
			"is accounted. Run ze on such a kernel, or disable traffic-usage.",
		Examples:    []string{exampleDoctorJSON, "ze show diagnostic " + codeTCXAbsent},
		HostVerdict: true,
	},
	{
		Code:  codeAttachUnknown,
		Title: "traffic-usage eBPF attach failed",
		Description: "traffic-usage is enabled and attaching its eBPF TCX programs to a configured interface " +
			"failed with an answer that does not say why: neither a permission refusal nor a missing TCX " +
			"attach point. The finding carries the kernel's answer; a program the verifier refused is a Ze " +
			"defect to report. ze retries the attach every second.",
		Examples: []string{exampleDoctorJSON, "ze show diagnostic " + codeAttachUnknown},
	},
	{
		Code:  codeInterfaceMissing,
		Title: "traffic-usage interface not found",
		Description: "traffic-usage is configured to account an interface that no device resolves to, so it is " +
			"not accounted. ze attaches as soon as the interface appears; correct or delete the name otherwise.",
		Examples: []string{exampleDoctorJSON, "ze show diagnostic " + codeInterfaceMissing},
	},
}

// errNoTCX marks an attach error that means the kernel or the build has no
// eBPF TCX attach point. The attacher wraps it around the kernel's answer.
var errNoTCX = errors.New("the kernel has no eBPF TCX attach point (Linux 6.6 or later required)")

// errProgramRejected marks an attach error that is the verifier refusing the
// program. The verifier answers EACCES for a program it judges unsafe, which
// is not a permission refusal, so the attacher marks it before classification.
var errProgramRejected = errors.New("the kernel's verifier refused the accounting program")

// attachState is what the last attempt established about one configured
// interface. The zero value is no outcome, never a pass.
type attachState uint8

const (
	// attachStateUnspecified is the zero value: nothing was recorded.
	attachStateUnspecified attachState = iota
	// attachStateAttached means both programs are attached.
	attachStateAttached
	// attachStateDown means the interface is present and down: it carries no
	// traffic, so nothing was attempted.
	attachStateDown
	// attachStateMissing means no device resolves the configured name.
	attachStateMissing
	// attachStateRefused means the kernel refused permission to load or attach.
	attachStateRefused
	// attachStateAbsent means the kernel or build has no TCX attach point.
	attachStateAbsent
	// attachStateUnknown means the attach failed and its answer names no cause.
	attachStateUnknown
)

// String names the state for a test failure.
func (s attachState) String() string {
	switch s {
	case attachStateAttached:
		return "attached"
	case attachStateDown:
		return "down"
	case attachStateMissing:
		return "missing"
	case attachStateRefused:
		return "refused"
	case attachStateAbsent:
		return "absent"
	case attachStateUnknown:
		return "unknown"
	case attachStateUnspecified:
		return "unspecified"
	}
	return "invalid"
}

// classifyAttachError reads the kernel's answer to a failed attach. A missing
// TCX attach point is checked first, and a verifier refusal before the
// permission errnos, because the verifier answers EACCES for an unsafe program.
// Anything else establishes no cause, which is Unknown and never a guess.
func classifyAttachError(err error) attachState {
	if errors.Is(err, errNoTCX) {
		return attachStateAbsent
	}
	if errors.Is(err, errProgramRejected) {
		return attachStateUnknown
	}
	if errors.Is(err, syscall.EPERM) {
		return attachStateRefused
	}
	if errors.Is(err, syscall.EACCES) {
		return attachStateRefused
	}
	return attachStateUnknown
}

// errNotConfigured is the doctor callback's answer before the plugin received
// its configuration: no attach was attempted yet, so no outcome can be read.
var errNotConfigured = errors.New("traffic-usage has not received its configuration yet, so no attach outcome exists to report")

// doctorFindings answers the runtime check from the recorded outcomes: one
// finding per configured interface that is not accounted for a reason an
// operator can act on. A down interface carries no traffic, so it reports
// nothing. With traffic-usage disabled nothing is configured, so nothing is
// recorded and nothing is reported. Safe for concurrent use.
func (m *Monitor) doctorFindings() ([]rpc.DoctorCheckDiagnostic, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cfg == nil {
		return nil, errNotConfigured
	}
	names := make([]string, 0, len(m.outcomes))
	for name := range m.outcomes {
		names = append(names, name)
	}
	slices.Sort(names)
	var findings []rpc.DoctorCheckDiagnostic
	for _, name := range names {
		outcome := m.outcomes[name]
		switch outcome.state {
		case attachStateAttached, attachStateDown:
			continue
		case attachStateMissing, attachStateRefused, attachStateAbsent, attachStateUnknown:
			findings = append(findings, outcome.finding(name))
		case attachStateUnspecified:
			return nil, errors.New("traffic-usage recorded no attach outcome for interface " + name)
		}
	}
	return findings, nil
}

// finding builds the doctor finding for one failed outcome. doctorFindings
// passes only the four failed states; the unknown finding is the answer for
// any other.
func (o attachOutcome) finding(name string) rpc.DoctorCheckDiagnostic {
	var tb textbuf.Buffer
	switch o.state {
	case attachStateMissing:
		tb.Str("traffic-usage is configured to account interface ").Str(name).
			Str(", but no interface by that name exists, so it is not accounted; ze attaches when it appears")
		return rpc.DoctorCheckDiagnostic{
			Code:     codeInterfaceMissing,
			Severity: string(diagnostic.SeverityWarning),
			Message:  tb.String(),
			Fix:      "delete traffic usage interfaces interface " + name,
		}
	case attachStateRefused:
		tb.Str("traffic-usage cannot account interface ").Str(name).
			Str(": the kernel refused the ze daemon permission to load or attach the eBPF programs (").
			Str(o.cause).Str("); the daemon needs CAP_BPF and CAP_NET_ADMIN")
		return rpc.DoctorCheckDiagnostic{
			Code:     codeAttachRefused,
			Severity: string(diagnostic.SeverityWarning),
			Message:  tb.String(),
			Fix:      o.remedy,
		}
	case attachStateAbsent:
		tb.Str("traffic-usage cannot account interface ").Str(name).
			Str(": this kernel cannot attach eBPF TCX programs (").Str(o.cause).
			Str("), so the plugin runs inert; run ze on Linux 6.6 or later built with CONFIG_NET_XGRESS")
		return rpc.DoctorCheckDiagnostic{
			Code:     codeTCXAbsent,
			Severity: string(diagnostic.SeverityWarning),
			Message:  tb.String(),
			Fix:      "set traffic usage enabled false",
		}
	case attachStateUnknown, attachStateAttached, attachStateDown, attachStateUnspecified:
	}
	tb.Str("traffic-usage cannot account interface ").Str(name).
		Str(": attaching the eBPF programs failed (").Str(o.cause).
		Str("), and that answer does not say whether the kernel, the program or the daemon's privileges refused it; ze retries every second")
	return rpc.DoctorCheckDiagnostic{
		Code:     codeAttachUnknown,
		Severity: string(diagnostic.SeverityWarning),
		Message:  tb.String(),
		Fix:      "ze show log recent level error component traffic.usage",
	}
}
