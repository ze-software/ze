//go:build !linux

package policyroute

import "fmt"

type linuxRuleManager struct{}

func newRuleManager() (*linuxRuleManager, error) {
	return nil, fmt.Errorf("policyroute: ip rule management requires Linux")
}

func (rm *linuxRuleManager) close() {}

func (rm *linuxRuleManager) applyIPRules(_ []ipRuleSpec) error {
	return fmt.Errorf("policyroute: not supported on this platform")
}

func (rm *linuxRuleManager) removeIPRules(_ []ipRuleSpec) {}

func (rm *linuxRuleManager) applyAutoRoutes(_ []autoRouteSpec) error {
	return fmt.Errorf("policyroute: not supported on this platform")
}

func (rm *linuxRuleManager) removeAutoRoutes(_ []autoRouteSpec) {}

func (rm *linuxRuleManager) applyAll(_ *translationResult) error {
	return fmt.Errorf("policyroute: not supported on this platform")
}

func (rm *linuxRuleManager) removeAll(_ *translationResult) {}
