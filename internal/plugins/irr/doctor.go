// Design: docs/architecture/irr-owner.md -- the owner's runtime doctor check and its codes
// Related: owner.go -- the worker that records each finding
// Related: register.go -- registers the codes at init
//
// What the owner knows is live state: which stored value it could not read,
// which fetch keeps failing, which answer it could not store. Only the owner
// process holds it, and the owner may run forked, so `show doctor` asks it
// through the doctor callback: the check is a runtime check declared at
// Stage 1 (docs/architecture/doctor-and-health-checks.md, "The phase decides
// what a check may read"). A stopped owner is reported by the engine's own
// plugin-stopped check (diagnostic.CodeDoctorPluginStopped); the consumers
// keep enforcing the stored copies meanwhile.

package irr

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/component/resolve/irr/store"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

const (
	// codeIRRDatasetUnreadable fires when a stored dataset, pointer, binding or
	// interest key cannot be read or decoded, so it reads as absent.
	codeIRRDatasetUnreadable = "doctor-irr-dataset-unreadable"
	// codeIRRRefreshOverdue fires when a due refresh, or a PeeringDB question,
	// was not answered, so the stored copy is older than the operator's
	// refresh period asks for.
	codeIRRRefreshOverdue = "doctor-irr-refresh-overdue"
	// codeIRRDatasetOversized fires when the server answered a list the store
	// refuses (over 500,000 prefixes in a family), so the stored copy stays.
	codeIRRDatasetOversized = "doctor-irr-dataset-oversized"
	// codeIRRDatasetUnpublished fires when an answer was not stored, or the
	// consumers were not told of it, so they enforce an older copy.
	codeIRRDatasetUnpublished = "doctor-irr-dataset-unpublished"
)

// datasetCheckName names the runtime check in the Stage 1 registration and in
// the doctor callback's dispatch.
const datasetCheckName = "irr-datasets"

// datasetCheckDecl declares the runtime check. Codes is a literal, so
// `./le arch doctor-coverage check` reads what the check declares.
var datasetCheckDecl = sdk.DoctorCheckDecl{
	Name:  datasetCheckName,
	Phase: sdk.DoctorPhaseRuntime,
	Codes: []string{
		codeIRRDatasetUnreadable,
		codeIRRRefreshOverdue,
		codeIRRDatasetOversized,
		codeIRRDatasetUnpublished,
	},
}

const exampleDoctor = "ze show doctor"

// irrDiagnosticCodes is the explanation of each code, for
// `ze show diagnostic <code>`. It lives with the plugin, so removing the
// plugin removes the codes.
var irrDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:  codeIRRDatasetUnreadable,
		Title: "A stored IRR value cannot be read",
		Description: "A stored IRR dataset, the pointer naming it, an AS number's AS-SET binding, or a consumer's interest key " +
			"could not be read or failed its digest. Ze treats it as absent: the consumers keep the list they enforce, and the " +
			"irr plugin fetches the AS-SET again at once. Run 'update irr all' to fetch and store every referenced AS-SET now.",
		Examples: []string{exampleDoctor, cmdShowIRR, "ze show diagnostic " + codeIRRDatasetUnreadable},
	},
	{
		Code:  codeIRRRefreshOverdue,
		Title: "An IRR refresh is overdue",
		Description: "The IRR server, or PeeringDB, did not answer when an AS-SET was due. Ze keeps enforcing the stored list " +
			"and asks again every minute until it answers. Check that the box reaches system irr server on TCP port 43, then " +
			"run 'update irr as-set <name>'.",
		Examples: []string{exampleDoctor, cmdShowIRR, "ze show diagnostic " + codeIRRRefreshOverdue},
	},
	{
		Code:  codeIRRDatasetOversized,
		Title: "An IRR answer is too large to store",
		Description: "The IRR server answered an AS-SET with more than 500,000 prefixes in one family. Ze refuses the list whole " +
			"rather than cutting it, and keeps enforcing the stored copy. Narrow the AS-SET a consumer names, or run " +
			"'clear irr as-set <name>' to stop enforcing it.",
		Examples: []string{exampleDoctor, cmdShowIRR, "ze show diagnostic " + codeIRRDatasetOversized},
	},
	{
		Code:  codeIRRDatasetUnpublished,
		Title: "An IRR answer was not stored or not announced",
		Description: "The store refused an IRR dataset, its pointer or an AS number's binding, or the consumers were not told " +
			"of a stored dataset. They keep enforcing the copy stored before. Ze retries the store every minute; a consumer " +
			"that missed a notice catches up at the next one. Fix the store error in the message, then run " +
			"'update irr as-set <name>'.",
		Examples: []string{exampleDoctor, cmdShowIRR, "ze show diagnostic " + codeIRRDatasetUnpublished},
	},
}

// ownerView is what the worker publishes for show irr and the doctor
// callback.
type ownerView struct {
	// published is false until the worker's first publishView, after it read
	// the store. Before it, the zero show (no server, interval 0, no dataset)
	// and the empty findings are not answers, so both readers refuse instead.
	published bool
	show      showAnswer
	findings  []sdk.DoctorCheckDiagnostic
}

// errNotLoaded refuses show irr and the doctor check before the worker has
// read the store: inside the startup handshake, or while its first read runs.
func errNotLoaded() error {
	return opererr.New("the irr plugin has not read its stored IRR datasets yet",
		"It answers from what it read from the store, and its first read runs once its startup has finished.",
		"show irr")
}

// answerDoctorCheck is the plugin's doctor callback. It answers the one
// runtime check the plugin declares and refuses a name it did not declare,
// and refuses before the worker's first view rather than report no finding.
func (o *owner) answerDoctorCheck(name string) ([]sdk.DoctorCheckDiagnostic, error) {
	if name != datasetCheckName {
		return nil, fmt.Errorf("irr: unknown doctor check %s", name)
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if !o.view.published {
		return nil, errNotLoaded()
	}
	return slices.Clone(o.view.findings), nil
}

// publishView rebuilds the snapshot from the worker's state. It runs on the
// worker only.
func (o *owner) publishView() {
	now := o.clock()
	index := o.consumerIndex()
	names := make(map[string]struct{}, len(o.pointers)+len(index))
	for asSet := range o.pointers {
		names[asSet] = struct{}{}
	}
	for asSet := range index {
		names[asSet] = struct{}{}
	}
	sets := make([]string, 0, len(names))
	for asSet := range names {
		sets = append(sets, asSet)
	}
	slices.Sort(sets)
	rows := make([]datasetRow, 0, len(sets))
	for _, asSet := range sets {
		rows = append(rows, o.rowOf(asSet, now, index[asSet]))
	}
	schedule := "automatic"
	if o.cfg.manual() {
		schedule = nextManual
	}
	view := ownerView{
		published: true,
		show: showAnswer{
			Server: o.cfg.server, RefreshInterval: uint32(o.cfg.refreshInterval / time.Second), //nolint:gosec // at most 86400
			Schedule: schedule, Datasets: rows,
		},
		findings: o.findings(),
	}
	o.mu.Lock()
	o.view = view
	o.mu.Unlock()
}

// consumerIndex maps each referenced AS-SET to the sorted consumers naming
// it, directly or through a bound AS number.
func (o *owner) consumerIndex() map[string][]string {
	index := make(map[string][]string)
	for consumer, in := range o.interest {
		seen := make(map[string]struct{}, len(in.ASSets)+len(in.ASNs))
		for _, asSet := range in.ASSets {
			seen[asSet] = struct{}{}
		}
		for _, number := range in.ASNs {
			if binding, ok := o.validBinding(number); ok {
				seen[binding.ASSet] = struct{}{}
			}
		}
		for asSet := range seen {
			index[asSet] = append(index[asSet], consumer)
		}
	}
	for asSet := range index {
		slices.Sort(index[asSet])
	}
	return index
}

// findings turns the recorded problems into doctor diagnostics, sorted by
// subject.
func (o *owner) findings() []sdk.DoctorCheckDiagnostic {
	subjects := make([]string, 0, len(o.problems))
	for subject := range o.problems {
		subjects = append(subjects, subject)
	}
	slices.Sort(subjects)
	out := make([]sdk.DoctorCheckDiagnostic, 0, len(subjects))
	for _, subject := range subjects {
		found := o.problems[subject]
		var tb textbuf.Buffer
		tb.Str("irr: ").Str(subject).Str(" ").Str(problemText(found.code)).Str(" since ")
		tb.Str(found.since.UTC().Format(time.RFC3339)).Str(" (").Err(found.err).Str(")")
		out = append(out, sdk.DoctorCheckDiagnostic{
			Code:     found.code,
			Severity: string(diagnostic.SeverityWarning),
			Message:  tb.String(),
			Fix:      fixOf(found, subject),
		})
	}
	return out
}

// problemText says what a code means for its subject.
func problemText(code string) string {
	switch code {
	case codeIRRDatasetUnreadable:
		return "cannot be read and is treated as absent"
	case codeIRRRefreshOverdue:
		return "was not answered when due, and the stored list stays enforced"
	case codeIRRDatasetOversized:
		return "answered a list too large to store, and the stored list stays enforced"
	case codeIRRDatasetUnpublished:
		return "was not stored or not announced, and the consumers enforce the copy stored before"
	}
	return "has a problem"
}

// fixOf is the command that fixes found: the one it was raised with, or else
// the one problemFix names for its code.
func fixOf(found problem, subject string) string {
	if found.fix != "" {
		return found.fix
	}
	return problemFix(found.code, subject)
}

// problemFix is the command that fixes a finding.
func problemFix(code, subject string) string {
	switch code {
	case codeIRRDatasetOversized:
		return "clear irr as-set " + subject
	case codeIRRRefreshOverdue, codeIRRDatasetUnpublished:
		if isASSetSubject(subject) {
			return "update irr as-set " + subject
		}
		if number, ok := strings.CutPrefix(subject, store.BindingPrefix); ok {
			// A binding's finding: PeeringDB did not answer, or the store
			// refused the binding.
			return "update irr asn " + number
		}
	case codeIRRDatasetUnreadable:
	}
	return "update irr all"
}

// isASSetSubject reports whether a finding's subject is an AS-SET rather than
// a state key: a key always holds a slash, an AS-SET name never does.
func isASSetSubject(subject string) bool {
	for i := range len(subject) {
		if subject[i] == '/' {
			return false
		}
	}
	return true
}
