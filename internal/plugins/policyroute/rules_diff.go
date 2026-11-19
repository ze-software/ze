// Design: docs/architecture/policyroute/policy-routing.md -- a reinstall moves ip rules by difference
// Related: register.go -- applyPolicies, the only caller
// Related: rules_linux.go -- linuxRuleManager, the netlink implementation of ruleInstaller

package policyroute

import (
	"reflect"
	"slices"

	"github.com/ze-software/ze/internal/core/iprule"
)

// ruleInstaller is the part of the rule manager replaceRules drives. The
// netlink manager implements it (rules_linux.go); a test records the calls.
type ruleInstaller interface {
	applyAutoRoutes(routes []autoRouteSpec) error
	applyIPRules(rules []iprule.Mark) error
	removeIPRules(rules []iprule.Mark)
	removeAutoRoutes(routes []autoRouteSpec)
}

// policiesUnchanged reports whether installing next over the installed state
// would change nothing. installed is nil until a first install succeeded, and
// then nothing is installed to keep, so the answer is false. The policies
// carry their resolved firewall sets (PolicyMatch.SourceSet and the others),
// so a commit that changes a referenced set's elements is a change, and one
// that changes only a set no policy names is not.
func policiesUnchanged(installed *translationResult, current, next []PolicyRoute) bool {
	if installed == nil {
		return false
	}
	if len(current) == 0 && len(next) == 0 {
		return true
	}
	return reflect.DeepEqual(current, next)
}

// replaceRules moves the kernel from old's ip rules and auto routes to next's.
// It installs what next holds and old does not, then removes what old holds and
// next does not, and leaves alone what both hold. The routes go in before the
// rules that steer into their tables, and come out after them.
//
// When an install fails, what this call added is removed again and old's state
// is left as it was, so the caller reports the failure over an unchanged
// kernel. old is nil on a first install.
func replaceRules(rm ruleInstaller, old, next *translationResult) error {
	var oldRules []iprule.Mark
	var oldRoutes []autoRouteSpec
	if old != nil {
		oldRules = old.IPRules
		oldRoutes = old.AutoRoutes
	}

	addRoutes := missing(next.AutoRoutes, oldRoutes)
	addRules := missing(next.IPRules, oldRules)

	if err := rm.applyAutoRoutes(addRoutes); err != nil {
		rm.removeAutoRoutes(addRoutes)
		return err
	}
	if err := rm.applyIPRules(addRules); err != nil {
		rm.removeIPRules(addRules)
		rm.removeAutoRoutes(addRoutes)
		return err
	}

	rm.removeIPRules(missing(oldRules, next.IPRules))
	rm.removeAutoRoutes(missing(oldRoutes, next.AutoRoutes))
	return nil
}

// missing returns the members of from that keep does not hold, in from's
// order. Both lists are a few entries per policy rule, so the quadratic scan
// is bounded by the rule count the configuration holds.
func missing[T comparable](from, keep []T) []T {
	var out []T
	for _, item := range from {
		if !slices.Contains(keep, item) {
			out = append(out, item)
		}
	}
	return out
}
