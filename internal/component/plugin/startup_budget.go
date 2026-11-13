// Design: docs/architecture/api/process-protocol.md -- bounded startup configuration.
// Related: startup_coordinator.go -- registration completion freezes one tier.

package plugin

import (
	"errors"
	"fmt"
	"time"
)

// ErrStartupTierFailed answers a declaration that arrived after another member
// of the tier had already failed it. It says nothing about the declaring plugin:
// that plugin met the tier's abort before it reached the barrier, and a caller
// MUST NOT record the wrapped failure as the declaring plugin's own.
var ErrStartupTierFailed = errors.New("startup tier already failed")

// DeclareConfigureBudget records one accepted registration's configure allowance.
// Callers MUST declare before completing Registration; completion MUST freeze
// the tier's maximum before any member enters Config. Zero declares trivial work.
//
// The answer when the tier already failed wraps ErrStartupTierFailed around the
// tier's failure. Every other refusal judges this plugin's own declaration and
// never wraps ErrStartupTierFailed. The tier's state is read under the same
// mutex that records a failure, so no failure can land between the two reads.
func (c *StartupCoordinator) DeclareConfigureBudget(pluginID int, budget time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.failedPlugin >= 0 {
		return fmt.Errorf("%w: %w", ErrStartupTierFailed, c.err)
	}
	if c.currentStage != StageRegistration {
		return fmt.Errorf("configure allowance for plugin %d declared after registration", pluginID)
	}
	if pluginID < 0 || pluginID >= c.pluginCount {
		return fmt.Errorf("configure allowance declared for unknown plugin %d", pluginID)
	}
	if budget < 0 {
		return fmt.Errorf("configure allowance for plugin %d is negative: %s", pluginID, budget)
	}
	if c.configureDeclared[pluginID] || c.stageComplete[pluginID] {
		return fmt.Errorf("configure allowance for plugin %d already declared or registration complete", pluginID)
	}
	c.configureDeclared[pluginID] = true
	c.configureBudget = max(c.configureBudget, budget)
	return nil
}

// ConfigureStall chooses the frozen configure allowance without changing an
// explicit operator timeout or a disabled stall check at its caller.
func (c *StartupCoordinator) ConfigureStall(stall time.Duration) time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	// Registration advancement freezes the maximum under this same mutex.
	// Other stages and a deliberately disabled stall retain their own bounds.
	if c.currentStage == StageConfig && stall > 0 {
		return max(stall, c.configureBudget)
	}
	return stall
}
