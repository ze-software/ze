// Design: docs/architecture/doctor-and-health-checks.md -- runtime doctor check for dataplane drift
// Related: drift.go -- dataplane drift registry and CheckAll

package drift

import (
	"context"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// CodeDoctorDataplaneDrift is the doctor diagnostic code emitted when a
// registered dataplane drift checker observes intent-vs-actual divergence or
// fails to read live dataplane state.
const CodeDoctorDataplaneDrift = "doctor-dataplane-drift"

const (
	dataplaneDriftCheckName  = "dataplane-drift"
	dataplaneDriftComponent  = "drift"
	dataplaneDriftDependency = "dataplane-drift"
	dataplaneDriftOrder      = 860
	dataplaneDriftFix        = "ze show system drift"
)

func init() {
	if err := diagnostic.Register(diagnostic.CodeMeta{
		Code:  CodeDoctorDataplaneDrift,
		Title: "Dataplane state drifted from Ze intent",
		Description: "One or more dataplane subsystems (such as firewall, fib-kernel, or interface) reported a divergence between Ze's intended state and the programmed kernel/dataplane state, or failed to read live dataplane state. " +
			"Run `ze show system drift` to inspect per-subsystem findings.",
		Examples: []string{
			"ze show doctor",
			"ze show system drift",
			"ze show diagnostic doctor-dataplane-drift",
		},
	}); err != nil {
		panic("BUG: drift: doctor code registration refused: " + err.Error())
	}

	if err := diagnostic.RegisterDoctorCheck(diagnostic.DoctorCheck{
		Name:         dataplaneDriftCheckName,
		Phase:        diagnostic.DoctorPhaseRuntime,
		Order:        dataplaneDriftOrder,
		Component:    dataplaneDriftComponent,
		Dependencies: []string{dataplaneDriftDependency},
		Platforms:    []string{diagnostic.DoctorPlatformAny},
		Codes:        []string{CodeDoctorDataplaneDrift},
		Check:        checkDataplaneDrift,
	}); err != nil {
		panic("BUG: drift: doctor check registration refused: " + err.Error())
	}
}

func checkDataplaneDrift(_ diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	reports := CheckAll(context.Background())
	var diags []diagnostic.Diagnostic
	var tb textbuf.Buffer

	for i := range reports {
		r := &reports[i]
		switch r.Status {
		case StatusClean, StatusUnavailable:
			continue
		case StatusDrifted:
			count := r.Drifted
			if count == 0 {
				count = len(r.Findings)
			}
			tb.Reset().Str("subsystem ").Str(r.Subsystem).
				Str(" has dataplane drift: ").Int(int64(count)).
				Str(" finding(s) across ").Int(int64(r.Checked)).Str(" checked object(s)")
			diags = append(diags, diagnostic.Diagnostic{
				Code:     CodeDoctorDataplaneDrift,
				Severity: diagnostic.SeverityWarning,
				Message:  tb.String(),
				Fix:      dataplaneDriftFix,
			})
		case StatusError:
			tb.Reset().Str("subsystem ").Str(r.Subsystem).Str(" drift check failed")
			if r.Error != "" {
				tb.Str(": ").Str(r.Error)
			}
			diags = append(diags, diagnostic.Diagnostic{
				Code:     CodeDoctorDataplaneDrift,
				Severity: diagnostic.SeverityWarning,
				Message:  tb.String(),
				Fix:      dataplaneDriftFix,
			})
		default:
			// Plugins supply an open set of status strings over RPC; ignore unknown values.
		}
	}
	return diags
}
