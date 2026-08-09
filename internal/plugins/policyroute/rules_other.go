// Design: docs/architecture/policyroute/policy-routing.md -- platform stub for non-Linux

//go:build !linux

package policyroute

import (
	"errors"
)

var (
	errPolicyrouteIpRuleManagementRequiresLinux = errors.New("policyroute: ip rule management requires Linux")
	errPolicyrouteNotSupportedOnThisPlatform    = errors.New("policyroute: not supported on this platform")
)

type linuxRuleManager struct{}

func newRuleManager() (*linuxRuleManager, error) {
	return nil, errPolicyrouteIpRuleManagementRequiresLinux
}

func (rm *linuxRuleManager) close() {}

func (rm *linuxRuleManager) applyAll(_ *translationResult) error {
	return errPolicyrouteNotSupportedOnThisPlatform
}

func (rm *linuxRuleManager) removeAll(_ *translationResult) {}
