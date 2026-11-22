// Design: docs/architecture/plugin/rib-storage-design.md -- selected capture health.
// Related: rib_advertise_capture.go -- the first failure and its reporting obligation.
package rib

import (
	"context"
	"strconv"
	"time"

	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const (
	advertiseDoctorName   = "bgp-rib-selected-output"
	advertiseReportBudget = 5 * time.Second
)

// advertiseDoctorDeclaration names the one runtime callback that reads this
// plugin's capture state, including when the plugin runs in another process.
func advertiseDoctorDeclaration() rpc.DoctorCheckDecl {
	return rpc.DoctorCheckDecl{Name: advertiseDoctorName, Phase: rpc.DoctorPhaseRuntime,
		Codes: []string{codeDoctorBGPSelectedOutput}}
}

// advertiseDoctorCheck reads the latched failure without applying a repair or
// reporting it again. Safe for concurrent use with capture and issue delivery.
func (r *RIBManager) advertiseDoctorCheck(name string) ([]rpc.DoctorCheckDiagnostic, error) {
	if name != advertiseDoctorName {
		return nil, opererr.New("The BGP RIB has no doctor check named "+strconv.Quote(name),
			"The requested check cannot inspect selected routes.", "ze show doctor")
	}
	r.advertise.mu.Lock()
	failure := r.advertise.failure
	r.advertise.mu.Unlock()
	findings := r.sourceLeases.doctorFailures()
	findings = append(findings, r.selectedDispatch.doctorFailures()...)
	if failure != nil {
		findings = append(findings, rpc.DoctorCheckDiagnostic{Code: codeDoctorBGPSelectedOutput,
			Severity: "error", Message: advertiseFailureMessage(failure), Fix: r.advertiseRepair()})
	}
	return findings, nil
}

func advertiseFailureMessage(failure error) string {
	var message textbuf.Buffer
	return message.Reset().Str("BGP selected routes are unavailable because their retained snapshot could not be recorded: ").
		Str(failure.Error()).String()
}

// advertiseRepair uses the engine-acknowledged instance name, never the SDK's
// implementation name. Without Stage 1 identity, restarting Ze is unambiguous.
func (r *RIBManager) advertiseRepair() string {
	if r.plugin != nil {
		if name := r.plugin.InstanceName(); name != "" {
			return process.RestartCommand(name)
		}
	}
	return "ze request restart"
}

// reportAdvertiseAfterElection MUST be deferred before selection locks are
// taken. The return path MUST release every shard/peer/FlowSpec lock first.
// A healthy election reads one atomic flag and creates no context or request.
func (r *RIBManager) reportAdvertiseAfterElection() {
	if !r.advertise.reportNeeded.Load() {
		return
	}
	r.reportAdvertiseIssue(context.Background())
}

// reportAdvertiseIssue sends the latched failure through the SDK so the daemon
// receives it on either transport. Callers MUST hold no selection or peer lock.
// At most one request is in flight; an unacknowledged failure stays owed and
// retries no more than once per report-bus window. Safe for concurrent use.
func (r *RIBManager) reportAdvertiseIssue(ctx context.Context) {
	if !r.advertise.reportNeeded.Load() {
		return
	}
	if r.plugin == nil {
		return // A test manager has no daemon to report to; its doctor still reads the failure.
	}
	r.advertise.mu.Lock()
	if !r.advertise.reportNeeded.Load() {
		r.advertise.mu.Unlock()
		return
	}
	if r.advertise.reporting {
		r.advertise.mu.Unlock()
		return
	}
	if time.Now().Before(r.advertise.reportAt) {
		r.advertise.mu.Unlock()
		return
	}
	r.advertise.reporting = true
	failure := r.advertise.failure
	r.advertise.mu.Unlock()

	message, fix := advertiseFailureMessage(failure), r.advertiseRepair()
	logger().Warn("BGP selected routes are unavailable", "cause", failure,
		"code", codeDoctorBGPSelectedOutput, "fix", fix)
	ctx, cancel := context.WithTimeout(ctx, advertiseReportBudget)
	err := r.plugin.RaiseIssue(ctx, sdk.Issue{Severity: rpc.ReportError,
		Code: codeDoctorBGPSelectedOutput, Subject: "selected routes", Message: message, Fix: fix})
	cancel()
	r.advertise.mu.Lock()
	r.advertise.reporting = false
	if err == nil {
		r.advertise.reportNeeded.Store(false)
	}
	if err != nil {
		r.advertise.reportAt = time.Now().Add(rpc.ReportRaiseWindow)
	}
	r.advertise.mu.Unlock()
	if err != nil {
		logger().Warn("BGP selected-route failure was not acknowledged by the daemon", "error", err,
			"code", codeDoctorBGPSelectedOutput, "fix", fix)
	}
}
