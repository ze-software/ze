// Design: plan/spec-policy-routing.md — platform stub for non-Linux

//go:build !linux

package policyroute

import "fmt"

type linuxRuleManager struct{}

func newRuleManager() (*linuxRuleManager, error) {
	return nil, fmt.Errorf("policyroute: ip rule management requires Linux")
}

func (rm *linuxRuleManager) close() {}

func (rm *linuxRuleManager) applyAll(_ *translationResult) error {
	return fmt.Errorf("policyroute: not supported on this platform")
}

func (rm *linuxRuleManager) removeAll(_ *translationResult) {}
