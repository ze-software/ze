// Design: docs/architecture/api/process-protocol.md -- bounded startup configuration.
// Related: startup_coordinator.go -- registration completion freezes one tier.

package plugin

import (
	"fmt"
	"time"
)

// DeclareConfigureBudget records one accepted registration's configure allowance.
// Callers MUST declare before completing Registration; completion MUST freeze
// the tier's maximum before any member enters Config. Zero declares trivial work.
func (c *StartupCoordinator) DeclareConfigureBudget(pluginID int, budget time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.failedPlugin >= 0 {
		return c.err
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
