// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- original cleanup diagnostics
// Related: pppoe_cleanup.go -- actual callback outcomes, not name-based inference

package iface

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const pppoeCleanupDoctorName = "pppoe-client-cleanup"
const pppoeCleanupDoctorCode = "doctor-pppoe-client-cleanup"
const pppoeCleanupFix = "ze request restart"
const pppoeCleanupReportBudget = time.Second

func init() {
	if err := diagnostic.Register(diagnostic.CodeMeta{
		Code: pppoeCleanupDoctorCode, Title: "PPPoE client cleanup needs attention",
		Description: "An original PPPoE client reported a cleanup error. Its owning plugin reports current retained cleanup separately from completed cleanup with an earlier error. Another client's successful cleanup is no evidence that the original retired. Inspect the cause before restarting Ze; only ending the owner process releases a descriptor whose first Close cannot be confirmed. The restart command restarts a supervised daemon; a foreground daemon must be started again after exit.",
		Examples:    []string{"ze show doctor", "ze show interface pppoe-client", pppoeCleanupFix},
	}); err != nil {
		panic("BUG: PPPoE cleanup diagnostic registration failed: " + err.Error())
	}
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: pppoeCleanupDoctorCode}); err != nil {
		panic("BUG: PPPoE cleanup event registration failed: " + err.Error())
	}
}

func pppoeCleanupDoctorDeclaration() rpc.DoctorCheckDecl {
	return rpc.DoctorCheckDecl{Name: pppoeCleanupDoctorName, Phase: rpc.DoctorPhaseRuntime,
		Codes: []string{pppoeCleanupDoctorCode}}
}

type pppoeCleanupReporter interface {
	RaiseIssue(context.Context, sdk.Issue) error
}

var _ pppoeCleanupReporter = (*sdk.Plugin)(nil)

// configurePPPoECleanupDiagnostics MUST run before the SDK starts. The runtime
// callback reads the registry in its actual plugin process on either transport.
// The SDK reporter MUST be installed only after startup admits engine calls.
func configurePPPoECleanupDiagnostics(plugin *sdk.Plugin, set *pppoeClientSet) {
	plugin.OnDoctorCheck(set.cleanupDoctor)
	plugin.OnStarted(func(ctx context.Context) error {
		// Existing clients can retain a failure from configuration or an earlier
		// engine. Publish it only after the current SDK connection is ready.
		set.mu.Lock()
		set.cleanupReporter = plugin
		clients := set.cleanupClientsLocked()
		set.mu.Unlock()
		work, cancel := context.WithTimeout(ctx, pppoeCleanupReportBudget)
		defer cancel()
		for _, client := range clients {
			if work.Err() != nil {
				break
			}
			client.mu.Lock()
			cause := client.cleanupErr
			client.mu.Unlock()
			client.reportCleanupFailure(work, cause)
		}
		return nil
	})
}

// cleanupClientsLocked MUST hold the existing registry mutex. Its caller MUST
// release that mutex before reading each client or invoking its SDK reporter.
func (set *pppoeClientSet) cleanupClientsLocked() []*PPPoEClient {
	clients := make([]*PPPoEClient, 0, len(set.active))
	for _, client := range set.active {
		clients = append(clients, client)
	}
	return clients
}

// cleanupDoctor reads only retained outcomes. It neither invokes cleanup nor
// checks a name, descriptor number or backend to infer native retirement.
// Snapshot locks are released before formatting any error or returning the RPC.
func (set *pppoeClientSet) cleanupDoctor(name string) ([]rpc.DoctorCheckDiagnostic, error) {
	if name != pppoeCleanupDoctorName {
		return nil, opererr.New("The interface plugin has no doctor check named "+strconv.Quote(name),
			"This name cannot inspect original PPPoE cleanup.", "ze show doctor")
	}
	set.mu.Lock()
	clients := set.cleanupClientsLocked()
	set.mu.Unlock()
	var rows []rpc.DoctorCheckDiagnostic
	for _, client := range clients {
		client.mu.Lock()
		cause := client.cleanupErr
		clientName, source := client.cfg.Name, client.cfg.SourceInterface
		client.mu.Unlock()
		if cause == nil {
			continue
		}
		rows = append(rows, rpc.DoctorCheckDiagnostic{Code: pppoeCleanupDoctorCode, Severity: "warning",
			Message: pppoeCleanupMessage(clientName, source, cause), Fix: pppoeCleanupRecovery(cause)})
	}
	return rows, nil
}

func pppoeCleanupMessage(name, source string, cause error) string {
	state := "Cleanup completed with an earlier error; native cleanup is not pending."
	next := "Review the retained earlier error in show warnings."
	if errors.Is(cause, ErrNativePPPPending) {
		state = "Original native removal is unconfirmed; replacement is blocked."
		next = "Inspect the original cause before restarting Ze; this ends its sessions."
	}
	return report.FitMessage(fmt.Sprintf("PPPoE client %s on source %s: %s Cause: %v. %s",
		name, source, state, cause, next))
}

func pppoeCleanupRecovery(cause error) string {
	if errors.Is(cause, ErrNativePPPPending) {
		return pppoeCleanupFix
	}
	return "ze show warnings" // Confirmed cleanup leaves history to inspect, not custody to release.
}

// reportCleanupFailure MUST follow publication of the actual cleanup result.
// It takes no registry/client lock across SDK I/O. One event per client per
// report window bounds history; the runtime check always reads current causes.
// Error events are never cleared by another client or by later healthy cleanup.
func (client *PPPoEClient) reportCleanupFailure(ctx context.Context, cause error) {
	if cause == nil || client.registry == nil {
		return
	}
	set := client.registry
	set.mu.Lock()
	reporter := set.cleanupReporter
	set.mu.Unlock()
	if reporter == nil {
		return // OnStarted reads retained outcomes once engine calls are admitted.
	}
	now := time.Now()
	client.mu.Lock()
	if now.Before(client.cleanupReportAt) {
		client.mu.Unlock()
		return
	}
	client.cleanupReportAt = now.Add(rpc.ReportRaiseWindow)
	name, source := client.cfg.Name, client.cfg.SourceInterface
	client.mu.Unlock()
	work, cancel := context.WithTimeout(ctx, pppoeCleanupReportBudget)
	defer cancel()
	if err := reporter.RaiseIssue(work, sdk.Issue{Severity: rpc.ReportError, Code: pppoeCleanupDoctorCode,
		Subject: "PPPoE client " + name, Message: pppoeCleanupMessage(name, source, cause), Fix: pppoeCleanupRecovery(cause)}); err != nil {
		client.logger.Warn("PPPoE client cleanup report was not acknowledged; inspect the original cleanup cause",
			"code", pppoeCleanupDoctorCode, "error", err, "fix", pppoeCleanupRecovery(cause))
	}
}
