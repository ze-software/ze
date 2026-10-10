// Design: docs/architecture/doctor-and-health-checks.md -- the kernel capability tier
// Detail: predicate.go -- the config predicates that say a subsystem is in use
// Detail: probe_linux.go, probe_other.go -- the native probes, one for each capability
// Related: cmd/ze/hub/startup_gate.go -- the sibling refusal, read at the first statement of run
//
// A subsystem declares the kernel capability it needs, a predicate that reports
// whether the running configuration uses it, and a native probe. When the
// configuration uses the subsystem and the host lacks the capability, ze doctor
// reports it, the daemon refuses to start, and ze config validate fails. A
// capability that declares Degrades is reported as a warning instead, and the
// daemon starts without what it names. When
// the configuration does not use the subsystem nothing is probed and nothing is
// reported: an absent feature nobody asked for is not a fault.
//
// This is the ze doctor tier of docs/architecture/doctor-and-health-checks.md,
// not a fourth one. The verdict is produced at read time, in the reader's own
// process, and it keeps no memory of a start. It cannot move into the plugin
// setup registry, whose records are written before main() and therefore cannot
// see the configuration this verdict depends on.

// Package kernelcap enrolls the kernel features a configured subsystem cannot
// work without, so ze doctor reports a missing one, the daemon refuses to
// start, a reload is refused and ze config validate fails, all from one verdict.
package kernelcap

import (
	"errors"
	"slices"
	"sort"
	"sync"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// State is the answer one probe gives about one kernel capability.
//
// Zero is StateUnspecified so a Result nobody wrote can never read as a
// verdict. A probe that cannot reach its evidence returns StateUnknown and says
// why; it MUST NOT return StatePresent (ai/rules/evidence.md).
type State uint8

const (
	// StateUnspecified is the zero value. No probe ran, or a probe returned no
	// verdict. It is never a pass and never a refusal.
	StateUnspecified State = iota
	// StatePresent means the kernel holds the capability.
	StatePresent
	// StateAbsent means the kernel does not hold the capability.
	StateAbsent
	// StateUnknown means the probe could not reach its evidence, so neither
	// presence nor absence was established.
	StateUnknown
	// StateDenied means a security policy confining this process (an
	// AppArmor profile, an SELinux label) refused a step the probe needs, so
	// the kernel was never asked. It is told apart from StateUnknown because
	// the fix is the host's policy, not the kernel and not ze's privileges,
	// and the Reason names that policy and the grant.
	StateDenied
)

// String names the state for a log line or a test failure.
func (s State) String() string {
	switch s {
	case StatePresent:
		return "present"
	case StateAbsent:
		return "absent"
	case StateUnknown:
		return "unknown"
	case StateDenied:
		return "denied"
	case StateUnspecified:
		return "unspecified"
	default:
		panic("BUG: invalid kernel capability state")
	}
}

// Result is one probe's answer and the evidence behind it.
type Result struct {
	State State
	// Reason is why the kernel gave this answer. It is nil for a probed
	// StatePresent, errForced for a forced one, and set for every other state, so a message can separate "the kernel lacks the
	// feature" from "this process was not allowed to ask".
	Reason error
}

// errProbeNoVerdict is what an enrolled probe that returned StateUnspecified is
// reported as. That is a ze defect rather than a host fault, so it is reported
// as cannot-determine and never as absence: refusing a start on it would stop a
// working router because of a bug in the probe.
var errProbeNoVerdict = errors.New("the capability probe returned no verdict")

// Capability is one subsystem's kernel requirement.
//
// The owning package fills it in and calls MustRegister from its own init(). No
// shared package enumerates subsystems: removing the owner removes the
// capability with it.
type Capability struct {
	// Subsystem names what needs the capability, in lower-kebab: "mpls", "ipsec".
	Subsystem string
	// Component is the lower-kebab owner name carried onto the doctor check row.
	Component string
	// Kernel names the kernel feature, spelled as the build-time symbol so this
	// enrolment reads against internal/appliance/kernelreq.go.
	Kernel string
	// ConfigLeaf names the configuration that required the capability, so the
	// operator is told which part of their config asked for it.
	ConfigLeaf string
	// Degrades is empty for a capability the subsystem cannot work without: its
	// absence is an error, and the daemon refuses to start. A capability whose
	// absence only weakens the subsystem names, here, what the operator loses
	// without it; its absence is then a warning carrying that text, and the
	// daemon starts.
	Degrades string
	// CodeAbsent and CodeUnknown are the diagnostic codes for the two faulty
	// states. Both must be registered in internal/core/diagnostic/codes.go.
	CodeAbsent  string
	CodeUnknown string
	// Order places this capability's check among the registered doctor checks.
	Order int
	// InUse reports whether the configuration uses the subsystem. A false answer
	// ends the evaluation and nothing is probed.
	InUse func(*config.Tree) bool
	// Probe reads the host. It uses netlink, procfs or a syscall, and it MUST
	// NOT execute an external binary: a second dependency that can be absent for
	// its own reasons is the fault this enrolment removes, not a way to find it.
	Probe func() Result
}

var capabilities = struct {
	sync.Mutex
	entries []Capability
	names   map[string]struct{}
}{names: make(map[string]struct{})}

// MustRegister enrolls one capability and registers the doctor check that
// reports it. It panics on an invalid or duplicate enrolment, which only a ze
// defect can produce: every caller is an init() with a literal.
func MustRegister(capability Capability) {
	if err := validate(capability); err != nil {
		panic("BUG: kernelcap: " + err.Error())
	}

	capabilities.Lock()
	if _, exists := capabilities.names[capability.Subsystem]; exists {
		capabilities.Unlock()
		panic("BUG: kernelcap: duplicate capability " + capability.Subsystem)
	}
	capabilities.names[capability.Subsystem] = struct{}{}
	capabilities.entries = append(capabilities.entries, capability)
	capabilities.Unlock()

	// One call from the owner registers both the enrolment and its doctor row.
	// A second call would be a second declaration of the same fact, and the two
	// would drift (ai/rules/principles.md).
	var name textbuf.Buffer
	check := diagnostic.DoctorCheck{
		Name:         name.Str("kernel-capability-").Str(capability.Subsystem).String(),
		Phase:        diagnostic.DoctorPhasePostConfig,
		Order:        capability.Order,
		Component:    capability.Component,
		Dependencies: []string{"config-tree", "kernel"},
		Platforms:    []string{diagnostic.DoctorPlatformAny},
		Codes:        []string{capability.CodeAbsent, capability.CodeUnknown, diagnostic.CodeDoctorKernelCapabilityForced},
		Check:        checkFor(capability.Subsystem),
	}
	if err := diagnostic.RegisterDoctorCheck(check); err != nil {
		panic("BUG: kernelcap: register doctor check: " + err.Error())
	}
}

// enrolForTest adds a capability to the enrolment WITHOUT registering a doctor
// check for it. The doctor check registry is process-global and refuses a
// duplicate name, so a test that enrolls the same subsystem in two cases would
// panic on the second. Test use only.
func enrolForTest(capability Capability) {
	capabilities.Lock()
	defer capabilities.Unlock()
	capabilities.names[capability.Subsystem] = struct{}{}
	capabilities.entries = append(capabilities.entries, capability)
}

// ResetForTest empties the enrolment. Test use only.
func ResetForTest() {
	capabilities.Lock()
	defer capabilities.Unlock()
	capabilities.entries = nil
	capabilities.names = make(map[string]struct{})
}

// Enrolled returns the enrolled subsystem names, sorted. It is what a test uses
// to prove an owner registered, and what a report uses to name the set.
func Enrolled() []string {
	capabilities.Lock()
	defer capabilities.Unlock()
	names := make([]string, 0, len(capabilities.entries))
	for i := range capabilities.entries {
		names = append(names, capabilities.entries[i].Subsystem)
	}
	slices.Sort(names)
	return names
}

// KernelSymbols returns the build symbol of every enrolled capability, sorted
// and without repeats. It is what a test compares with the kernel profile's
// require manifest, so Ze's own kernel provides everything Ze enrolls.
func KernelSymbols() []string {
	capabilities.Lock()
	defer capabilities.Unlock()
	symbols := make([]string, 0, len(capabilities.entries))
	for i := range capabilities.entries {
		symbols = append(symbols, capabilities.entries[i].Kernel)
	}
	slices.Sort(symbols)
	return slices.Compact(symbols)
}

// Evaluate returns the diagnostics every enrolled subsystem produces for tree.
//
// An absent capability is a SeverityError, which ze doctor already exits 1 on
// and which the startup and validate gates refuse on, unless the capability
// declares that its absence only Degrades the subsystem: that absence is a
// SeverityWarning. A capability that could
// not be DETERMINED is a SeverityWarning and never a refusal: refusing on an
// unreadable probe turns a working deployment into a dead one.
func Evaluate(tree *config.Tree) []diagnostic.Diagnostic {
	if tree == nil {
		return nil
	}

	capabilities.Lock()
	entries := append([]Capability(nil), capabilities.entries...)
	capabilities.Unlock()

	sort.Slice(entries, func(i, j int) bool { return entries[i].Subsystem < entries[j].Subsystem })

	var diags []diagnostic.Diagnostic
	for i := range entries {
		if d, produced := evaluateOne(&entries[i], tree); produced {
			diags = append(diags, d)
		}
	}
	return diags
}

// Row is one enrolled capability's answer when every enrolment is probed with
// the configuration ignored (ProbeAll). It is the JSON a Docker-host check reads,
// so its State is the spelled word, never the numeric enum.
type Row struct {
	Subsystem string `json:"subsystem"`
	Kernel    string `json:"kernel"`
	State     string `json:"state"`
	Reason    string `json:"reason,omitempty"`
}

// ProbeAll probes EVERY enrolled capability, whether or not any configuration
// uses its subsystem, and returns one row per enrolment in Enrolled order.
//
// Evaluate asks "does this configuration's subsystem have its feature". This
// asks "does this kernel hold every feature Ze can use", which is what a host
// that runs Ze's labs must answer, so InUse is the one switch between the two.
// A probe that gave no verdict is reported unknown with errProbeNoVerdict:
// StateUnspecified is never spelled as an answer, and never as present.
func ProbeAll() []Row {
	capabilities.Lock()
	entries := append([]Capability(nil), capabilities.entries...)
	capabilities.Unlock()

	sort.Slice(entries, func(i, j int) bool { return entries[i].Subsystem < entries[j].Subsystem })

	rows := make([]Row, 0, len(entries))
	for i := range entries {
		result := probe(&entries[i])
		if result.State == StateUnspecified {
			result = Result{State: StateUnknown, Reason: errProbeNoVerdict}
		}
		row := Row{Subsystem: entries[i].Subsystem, Kernel: entries[i].Kernel, State: result.State.String()}
		if result.Reason != nil {
			row.Reason = result.Reason.Error()
		}
		rows = append(rows, row)
	}
	return rows
}

// Refuse returns the error a caller that must not run on a missing capability
// prints, or nil when every enrolled subsystem the configuration uses is
// supported by this host.
//
// EVERY failing subsystem is named, not the first one. An operator who repairs
// one fault and restarts to meet the next pays a whole boot for each fault
// after the first, which is why the sibling plugin setup gate names them all
// too (cmd/ze/hub/startup_gate.go).
func Refuse(tree *config.Tree) error {
	diags := Evaluate(tree)

	var text textbuf.Buffer
	failures := 0
	for i := range diags {
		if diags[i].Severity != diagnostic.SeverityError {
			continue
		}
		if failures > 0 {
			text.Str("; ")
		}
		text.Str(diags[i].Message)
		failures++
	}
	if failures == 0 {
		return nil
	}
	return errors.New(text.String())
}

// evaluateOne runs one capability against tree. The bool says whether a
// diagnostic was produced, so a present capability adds nothing.
func evaluateOne(capability *Capability, tree *config.Tree) (diagnostic.Diagnostic, bool) {
	if !capability.InUse(tree) {
		return diagnostic.Diagnostic{}, false
	}

	result := probe(capability)
	if result.State == StatePresent {
		return forcedPresent(capability, result)
	}

	var text textbuf.Buffer
	text.Str(capability.Subsystem)
	if result.State == StateAbsent {
		text.Str(": the kernel holds no ").Str(capability.Kernel).
			Str(", which ").Str(capability.ConfigLeaf).Str(" requires")
		if result.Reason != nil {
			text.Str(": ").Err(result.Reason)
		}
		// A capability that only degrades its subsystem never refuses: the
		// daemon starts, and the warning says what is lost.
		severity := diagnostic.SeverityError
		if capability.Degrades != "" {
			severity = diagnostic.SeverityWarning
			text.Str("; ze starts, and ").Str(capability.Degrades)
		}
		return diagnostic.Diagnostic{
			Code:     capability.CodeAbsent,
			Severity: severity,
			Message:  text.String(),
		}, true
	}

	reason := result.Reason
	if reason == nil {
		reason = errProbeNoVerdict
	}
	if result.State == StateDenied {
		return deniedDiagnostic(capability, reason), true
	}
	text.Str(": cannot determine whether the kernel holds ").Str(capability.Kernel).
		Str(", which ").Str(capability.ConfigLeaf).Str(" requires: ").Err(reason).
		Str("; ze starts")
	// A degrading capability's subsystem works either way, so the warning
	// names the loss instead of claiming the subsystem may fail.
	if capability.Degrades == "" {
		text.Str(", and the subsystem may not work")
	} else {
		text.Str(", and without it ").Str(capability.Degrades)
	}
	return diagnostic.Diagnostic{
		Code:     capability.CodeUnknown,
		Severity: diagnostic.SeverityWarning,
		Message:  text.String(),
	}, true
}

// deniedDiagnostic is the answer for a probe the host's security policy
// refused (D-7). The kernel was never asked, so it is a WARNING and ze starts,
// as for a probe that could not ask; it carries its own code because the fix is
// the host's policy, which reason names with the grant the probe needs.
func deniedDiagnostic(capability *Capability, reason error) diagnostic.Diagnostic {
	var text textbuf.Buffer
	text.Str(capability.Subsystem).Str(": cannot ask whether the kernel holds ").Str(capability.Kernel).
		Str(", which ").Str(capability.ConfigLeaf).Str(" requires: the host's security policy denied the probe: ").
		Err(reason).Str("; ze starts")
	if capability.Degrades == "" {
		text.Str(", and the subsystem may not work")
	} else {
		text.Str(", and without it ").Str(capability.Degrades)
	}
	return diagnostic.Diagnostic{
		Code:     diagnostic.CodeDoctorKernelCapabilityDenied,
		Severity: diagnostic.SeverityWarning,
		Message:  text.String(),
	}
}

// forcedPresent is the answer for a present capability: nothing when a probe
// ran, and a warning when a test forced it. A probed present has no Reason; a
// forced one carries errForced (forcedState), so a forced verdict is never
// read as the host's kernel. The warning never refuses a start.
func forcedPresent(capability *Capability, result Result) (diagnostic.Diagnostic, bool) {
	if result.Reason == nil {
		return diagnostic.Diagnostic{}, false
	}
	var text textbuf.Buffer
	text.Str(capability.Subsystem).Str(": the kernel is taken to hold ").Str(capability.Kernel).
		Str(" without probing it: ").Err(result.Reason)
	return diagnostic.Diagnostic{
		Code:     diagnostic.CodeDoctorKernelCapabilityForced,
		Severity: diagnostic.SeverityWarning,
		Message:  text.String(),
	}, true
}

// checkFor returns the doctor check function for one enrolled subsystem. The
// doctor runner passes the tree as any, so it is asserted back here.
func checkFor(subsystem string) diagnostic.DoctorCheckFunc {
	return func(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
		tree, ok := ctx.Tree.(*config.Tree)
		if !ok || tree == nil {
			return nil
		}

		capabilities.Lock()
		var found Capability
		hit := false
		for i := range capabilities.entries {
			if capabilities.entries[i].Subsystem == subsystem {
				found = capabilities.entries[i]
				hit = true
				break
			}
		}
		capabilities.Unlock()
		if !hit {
			return nil
		}

		d, produced := evaluateOne(&found, tree)
		if !produced {
			return nil
		}
		return []diagnostic.Diagnostic{d}
	}
}

func validate(capability Capability) error {
	if capability.Subsystem == "" {
		return errors.New("missing subsystem")
	}
	if capability.Component == "" {
		return errors.New("missing component for " + capability.Subsystem)
	}
	if capability.Kernel == "" {
		return errors.New("missing kernel feature for " + capability.Subsystem)
	}
	if capability.ConfigLeaf == "" {
		return errors.New("missing config leaf for " + capability.Subsystem)
	}
	if capability.CodeAbsent == "" || capability.CodeUnknown == "" {
		return errors.New("missing diagnostic code for " + capability.Subsystem)
	}
	if capability.InUse == nil {
		return errors.New("missing in-use predicate for " + capability.Subsystem)
	}
	if capability.Probe == nil {
		return errors.New("missing probe for " + capability.Subsystem)
	}
	return nil
}
