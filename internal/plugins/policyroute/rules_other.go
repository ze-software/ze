// Design: docs/architecture/policyroute/policy-routing.md -- platform stub for non-Linux

//go:build !linux

package policyroute

import (
	"errors"

	"github.com/ze-software/ze/internal/core/iprule"
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

func (rm *linuxRuleManager) removeAll(_ *translationResult) {}

func (rm *linuxRuleManager) applyAutoRoutes(_ []autoRouteSpec) error {
	return errPolicyrouteNotSupportedOnThisPlatform
}

func (rm *linuxRuleManager) applyIPRules(_ []iprule.Mark) error {
	return errPolicyrouteNotSupportedOnThisPlatform
}

func (rm *linuxRuleManager) removeIPRules(_ []iprule.Mark) {}

func (rm *linuxRuleManager) removeAutoRoutes(_ []autoRouteSpec) {}
