// Design: docs/architecture/api/update-syntax.md -- partial route command results.
package update

import "github.com/ze-software/ze/internal/component/plugin"

// Counts describe work already completed before cause; the failure cannot erase it.
func updateRouteFailure(announced, withdrawn int, warnings []string, cause error) (*plugin.Response, error) {
	return &plugin.Response{Status: plugin.StatusError, Error: cause.Error(), Data: &plugin.RouteResult{
		Announced: uint32(announced), //nolint:gosec // bounded by parsed NLRI count
		Withdrawn: uint32(withdrawn), //nolint:gosec // bounded by parsed NLRI count
		Warnings:  warnings, Failure: cause.Error(),
	}}, cause
}
