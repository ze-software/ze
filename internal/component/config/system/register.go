// Design: docs/features/ai-first.md -- doctor check registration for the system component
// Related: doctor.go -- the four checks registered here
// Related: dns_source.go -- the router dns-source validator registered here
//
// The system component is not a plugin, so it registers through the component
// path (diagnostic.RegisterDoctorCheck) rather than a plugin Registration's
// DoctorChecks field. Keeping the checks and their registration under
// internal/component/config/system means removing the component removes its
// diagnostics with it.

package system

import (
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

func init() {
	registerSystemDoctorChecks()
	// The host name check runs wherever a config is validated (ze config
	// validate, a commit, a reload), before an applier sets the kernel name.
	if err := config.RegisterTreeValidator("system-host-name", validateSystemHostTree); err != nil {
		panic("BUG: system host name validator registration refused: " + err.Error())
	}
	// The router's dns-source list is refused at the same places: a list of
	// two sources with no mode, a received-interface with no received, a
	// class the router cannot use.
	if err := config.RegisterTreeValidator("system-dns-source", validateRouterDNSSourceTree); err != nil {
		panic("BUG: system dns-source validator registration refused: " + err.Error())
	}
	// The dns-source selection checks read the registry the consumers
	// publish into, which only the daemon fills, so they register in the
	// runtime phase: show doctor runs them, the local ze doctor never does.
	if err := diagnostic.RegisterDoctorCheck(dnsSourceDoctorCheck); err != nil {
		panic("BUG: dns-source doctor check registration refused: " + err.Error())
	}
}
