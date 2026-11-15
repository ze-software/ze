// Design: docs/architecture/bgp/filter-irr.md -- what the filter reports when it cannot use the stored lists
// Related: stored.go -- the reader that records each problem
// Related: register.go -- registers the codes at init
//
// What the filter knows is live state of its own process: which stored copy
// it could not read, whether its interest key was saved, which generation it
// has applied. The filter may run forked, so `show doctor` asks it through the
// doctor callback: the check is a runtime check declared at Stage 1
// (docs/architecture/doctor-and-health-checks.md, "The phase decides what a
// check may read"). The irr plugin reports its own fetch and store problems.

package filter_irr

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/ze-software/ze/internal/component/resolve/irr/store"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

const (
	// codeIRRStoredCopyUnreadable fires when a stored list, its pointer or an
	// AS number's binding exists and cannot be used, so the filter enforces
	// the list it read before, or none.
	codeIRRStoredCopyUnreadable = "doctor-bgp-irr-stored-copy-unreadable"
	// codeIRRStoredCopyBehind fires when the irr plugin raised a generation
	// the filter has not applied: a notice was not delivered, so the filter
	// may enforce an older copy than the one stored.
	codeIRRStoredCopyBehind = "doctor-bgp-irr-stored-copy-behind"
	// codeIRRInterestUnsaved fires when the filter's interest key was not
	// written, so the irr plugin may not fetch a newly configured AS-SET.
	codeIRRInterestUnsaved = "doctor-bgp-irr-interest-unsaved"
)

// storedCopyCheckName names the runtime check in the Stage 1 registration and
// in the doctor callback's dispatch.
const storedCopyCheckName = "bgp-filter-irr-stored-copies"

// storedCopyCheckDecl declares the runtime check. Codes is a literal, so
// `./le arch doctor-coverage check` reads what the check declares.
var storedCopyCheckDecl = sdk.DoctorCheckDecl{
	Name:  storedCopyCheckName,
	Phase: sdk.DoctorPhaseRuntime,
	Codes: []string{
		codeIRRStoredCopyUnreadable,
		codeIRRStoredCopyBehind,
		codeIRRInterestUnsaved,
	},
}

// doctorGenerationTimeout bounds the generation read a doctor answer makes.
const doctorGenerationTimeout = 5 * time.Second

const exampleShowDoctor = "ze show doctor"

// irrDiagnosticCodes is the explanation of each code, for
// `ze show diagnostic <code>`. It lives with the plugin, so removing the
// plugin removes the codes.
var irrDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:  codeIRRStoredCopyUnreadable,
		Title: "bgp-filter-irr cannot read a stored IRR list",
		Description: "The irr plugin stored a list for an AS-SET bgp-filter-irr enforces, and the filter could not read it, or " +
			"it failed its digest. The filter keeps the list it read before; with none, the peer's announcements are rejected " +
			"and held. Run 'update irr as-set <name>' to fetch and store the set again; the filter reads it on the notice.",
		Examples: []string{exampleShowDoctor, cmdShowIRR, "ze show diagnostic " + codeIRRStoredCopyUnreadable},
	},
	{
		Code:  codeIRRStoredCopyBehind,
		Title: "bgp-filter-irr missed a stored IRR list",
		Description: "The irr plugin stored a newer list than bgp-filter-irr has read: a notice did not reach the filter. The " +
			"filter catches up at the next notice. Run 'update irr all' to store every set again, which sends the notices.",
		Examples: []string{exampleShowDoctor, cmdShowIRR, "ze show diagnostic " + codeIRRStoredCopyBehind},
	},
	{
		Code:  codeIRRInterestUnsaved,
		Title: "bgp-filter-irr could not save the AS-SETs it enforces",
		Description: "The store refused the key through which bgp-filter-irr tells the irr plugin which AS-SETs and AS numbers " +
			"it enforces, so a newly configured one may not be fetched. The filter writes it again every minute. Fix the store " +
			"error in the message, or run 'update irr as-set <name>' to fetch a set now.",
		Examples: []string{exampleShowDoctor, "ze show irr", "ze show diagnostic " + codeIRRInterestUnsaved},
	},
}

// fixFetchAll is the command that fetches and stores every referenced set
// again: the fix when no single AS-SET names the problem.
const fixFetchAll = "update irr all"

// fixFetchAgain is the command that fetches and stores asSet again, or every
// set when the AS number's set is not known yet.
func fixFetchAgain(asSet string) string {
	if asSet == "" {
		return fixFetchAll
	}
	return "update irr as-set " + asSet
}

// answerDoctorCheck is the plugin's doctor callback. It answers the one
// runtime check the plugin declares and refuses a name it did not declare.
func (plug *irrPlugin) answerDoctorCheck(name string) ([]sdk.DoctorCheckDiagnostic, error) {
	if name != storedCopyCheckName {
		return nil, fmt.Errorf("bgp-filter-irr: unknown doctor check %s", name)
	}
	findings := plug.unreadableFindings()
	if failure := plug.interestErr.Load(); failure != nil {
		var tb textbuf.Buffer
		tb.Str("bgp-filter-irr: the AS-SETs it enforces were not saved since ").Str(failure.since.UTC().Format(time.RFC3339))
		tb.Str(" (").Str(failure.err).Str(")")
		findings = append(findings, sdk.DoctorCheckDiagnostic{
			Code: codeIRRInterestUnsaved, Severity: string(diagnostic.SeverityWarning),
			Message: tb.String(), Fix: fixFetchAll,
		})
	}
	if behind, ok := plug.behindFinding(); ok {
		findings = append(findings, behind)
	}
	return findings, nil
}

// unreadableFindings is one finding per enrolled ASN whose stored value the
// reader could not use, sorted by AS number.
func (plug *irrPlugin) unreadableFindings() []sdk.DoctorCheckDiagnostic {
	plug.mu.RLock()
	defer plug.mu.RUnlock()
	asns := make([]uint32, 0, len(plug.byASN))
	for number, st := range plug.byASN {
		if st.readErr != "" || st.keyErr != "" {
			asns = append(asns, number)
		}
	}
	slices.Sort(asns)
	out := make([]sdk.DoctorCheckDiagnostic, 0, len(asns))
	for _, number := range asns {
		st := plug.byASN[number]
		reason := st.readErr
		if st.keyErr != "" {
			reason = st.keyErr
		}
		var tb textbuf.Buffer
		tb.Str("bgp-filter-irr: AS").Uint32(number).Str(" ").Str(st.asSet).Str(" stored list cannot be used (").Str(reason).Str(")")
		out = append(out, sdk.DoctorCheckDiagnostic{
			Code: codeIRRStoredCopyUnreadable, Severity: string(diagnostic.SeverityWarning),
			Message: tb.String(), Fix: fixFetchAgain(st.asSet),
		})
	}
	return out
}

// behindFinding compares the generation the irr plugin last raised with the
// one the reader applied. A generation that cannot be read is no finding here:
// the irr plugin's own check reports an unreadable store.
func (plug *irrPlugin) behindFinding() (sdk.DoctorCheckDiagnostic, bool) {
	if plug.reader == nil {
		return sdk.DoctorCheckDiagnostic{}, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), doctorGenerationTimeout)
	defer cancel()
	stored, err := store.ReadGeneration(ctx, plug.reader.state)
	if err != nil {
		return sdk.DoctorCheckDiagnostic{}, false
	}
	applied := plug.reader.seen.Load()
	if stored <= applied {
		return sdk.DoctorCheckDiagnostic{}, false
	}
	var tb textbuf.Buffer
	tb.Str("bgp-filter-irr: applied IRR generation ").Uint32(applied).Str(", the irr plugin stored ").Uint32(stored)
	return sdk.DoctorCheckDiagnostic{
		Code: codeIRRStoredCopyBehind, Severity: string(diagnostic.SeverityWarning),
		Message: tb.String(), Fix: fixFetchAll,
	}, true
}
