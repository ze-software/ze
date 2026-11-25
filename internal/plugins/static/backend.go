// Design: docs/architecture/static-routes.md -- backend abstraction

package static

import (
	"strconv"

	"github.com/ze-software/ze/internal/core/opererr"
)

const (
	dataPlaneKernel     = "kernel"
	dataPlaneVPP        = "vpp"
	staticVPPChannelFix = "ze show doctor"
)

type routeBackend interface {
	applyRoute(r staticRoute) error
	removeRoute(r staticRoute) error
	listRoutes() ([]installedStaticRoute, error)
	close() error
}

type installedStaticRoute struct {
	prefix  string //nolint:unused // read in backend_linux.go
	nextHop string //nolint:unused // read in backend_linux.go
	table   uint32 //nolint:unused // read in backend_linux.go
}

// staticVPPChannelError builds the operator error returned when a named-table
// static route under the VPP data plane cannot open a GoVPP API channel.
func staticVPPChannelError(cause error) *opererr.Error {
	return opererr.Wrap(
		cause,
		"static named-table route cannot open VPP channel ("+doctorCodeVPPChannel+")",
		"interface backend is vpp, so static refuses to fall back to the Linux kernel for named-table routes",
		staticVPPChannelFix,
	)
}

// unrecognizedDataPlaneError builds the fail-closed operator error returned
// when a named-table route requests a data plane static does not implement.
func unrecognizedDataPlaneError(dataPlane string) *opererr.Error {
	return opererr.New(
		"static named-table route cannot use unrecognized data plane "+strconv.Quote(dataPlane),
		"named-table static routes follow the configured interface backend, which must be kernel or vpp",
		"set interface backend kernel",
	)
}
