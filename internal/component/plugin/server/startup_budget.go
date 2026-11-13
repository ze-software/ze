// Design: docs/architecture/api/process-protocol.md -- bounded startup configuration.
// Related: startup.go -- accepted registrations and configure-stage waits.
// Related: restart.go -- the outgoing declaration bounds replacement startup.

package server

import (
	"time"

	"github.com/ze-software/ze/internal/component/config/transaction"
	plugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
)

func declaredConfigureBudget(registration *plugin.PluginRegistration) time.Duration {
	// A missing or trivial declaration grants no extension of ordinary bounds.
	if registration == nil || registration.ApplyBudget <= 0 {
		return 0
	}
	// Clamp integer seconds before converting: a hostile estimate may overflow
	// time.Duration even though the accepted configuration ceiling cannot.
	seconds := min(registration.ApplyBudget, transaction.MaxBudgetSeconds)
	return time.Duration(seconds) * time.Second
}

func previousConfigureBudget(proc *process.Process) time.Duration {
	// StageConfig records accepted registration; an unaccepted declaration
	// grants no extension to the replacement handshake.
	if proc == nil || proc.Stage() < plugin.StageConfig {
		return 0
	}
	return declaredConfigureBudget(proc.Registration())
}

func startupStageStall(coord *plugin.StartupCoordinator, proc *process.Process, complete, wait plugin.PluginStage) time.Duration {
	timeout := proc.Config().StageTimeout
	if timeout != 0 {
		return timeout
	}
	timeout = stageTimeoutFromEnv()
	if complete == plugin.StageConfig && wait == plugin.StageCapability {
		return coord.ConfigureStall(timeout)
	}
	return timeout
}

func restartConfigureHandshakeBound(proc *process.Process, configureBudget time.Duration) time.Duration {
	ordinary := restartHandshakeBound(proc)
	if proc.Config().StageTimeout != 0 {
		return ordinary
	}
	stage := stageTimeoutFromEnv()
	// Preserve the existing immediate-expiry meaning of nonpositive restart
	// bounds, rather than enabling a declaration to turn them into a wait.
	if stage <= 0 || configureBudget <= stage {
		return ordinary
	}
	return (restartHandshakeStages-1)*stage + configureBudget
}
