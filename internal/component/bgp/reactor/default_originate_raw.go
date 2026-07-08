// Design: docs/architecture/core-design.md — default-originate raw-filter guard

package reactor

import "strings"

// filterRawInfo reports whether a named "<plugin>:<filter>" filter was declared
// with raw=true. *pluginserver.Server satisfies it via FilterInfo; the narrow
// interface keeps the raw-guard decision unit-testable without a live server.
type filterRawInfo interface {
	FilterInfo(pluginName, filterName string) ([]string, bool)
}

// defaultOriginateRejectsRawFilter reports whether the named default-originate
// filter ref was declared raw=true, in which case it must not gate the default
// route. A raw filter operates on the received UPDATE's wire bytes, but the
// default-originate route is synthetic and carries none: the filter would be
// handed empty hex and make a silently-wrong accept/reject decision. The caller
// fails closed (does not originate) and this logs a loud, actionable warning so
// the operator switches the binding to a text filter.
//
// A ref without a ':' is left alone: the caller's own colon check already fails
// it closed, so no lookup is attempted on a bogus split.
func defaultOriginateRejectsRawFilter(info filterRawInfo, ref, peerAddr string) bool {
	pluginName, filterName, ok := strings.Cut(ref, ":")
	if !ok {
		return false
	}
	if _, raw := info.FilterInfo(pluginName, filterName); raw {
		routesLogger().Warn("default-originate: filter declares raw=true but the synthetic default route has no wire bytes -- fail-closed (bind a text filter instead)",
			"peer", peerAddr, "filter", ref)
		return true
	}
	return false
}
