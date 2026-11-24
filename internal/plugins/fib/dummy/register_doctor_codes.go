// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by fib-dummy.
// Related: register.go -- plugin registration and runtime invalid-change warning raise
// Related: labels.go -- label refusal logging under mplsfib.CodeDoctorNoWriter
package fibdummy

import "github.com/ze-software/ze/internal/core/diagnostic"

const (
	// codeDoctorHoldsRoutes reports that `fib { dummy { } }` holds selected
	// system RIB routes in memory and writes them to no forwarding table.
	codeDoctorHoldsRoutes = "doctor-fib-dummy-holds-routes"

	// codeDoctorInvalidChange reports that fib-dummy received a
	// (system-rib, best-change) entry carrying an invalid prefix.
	codeDoctorInvalidChange = "doctor-fib-dummy-invalid-change"
)

var holdsRoutesCodeMeta = diagnostic.CodeMeta{
	Code:  codeDoctorHoldsRoutes,
	Title: "System RIB routes are held by fib-dummy and not programmed",
	Description: "The configuration contains `fib { dummy { } }`, so fib-dummy receives the routes " +
		"the system RIB selects, holds them in memory, and writes them to no forwarding table (kernel or VPP). " +
		"Named-table static routes, policy routing, the IPsec tunnel-source guard, and interface addresses are " +
		"outside the system RIB and untouched. When route producers that declare NeedsDataPlane are selected this " +
		"is a WARNING; with none selected it is INFO. Remedy: delete fib dummy to let Ze program selected routes " +
		"into the data plane.",
	Examples: []string{
		"ze show fib dummy",
		"ze show doctor",
		"ze show diagnostic " + codeDoctorHoldsRoutes,
	},
}

var invalidChangeCodeMeta = diagnostic.CodeMeta{
	Code:  codeDoctorInvalidChange,
	Title: "fib-dummy received a best-change entry with an invalid prefix",
	Description: "A (system-rib, best-change) batch delivered a route change whose prefix is invalid. " +
		"fib-dummy counts it in invalid-changes, holds it nowhere, and raises this warning once to surface " +
		"a route-producer defect. Inspect the counter with `ze show fib dummy counters` and active warnings " +
		"with `ze show warnings`.",
	Examples: []string{
		"ze show fib dummy counters",
		"ze show warnings",
		"ze show diagnostic " + codeDoctorInvalidChange,
	},
}

func init() {
	if err := diagnostic.Register(holdsRoutesCodeMeta); err != nil {
		panic("BUG: fib-dummy: register holds-routes diagnostic code: " + err.Error())
	}
	if err := diagnostic.Register(invalidChangeCodeMeta); err != nil {
		panic("BUG: fib-dummy: register invalid-change diagnostic code: " + err.Error())
	}
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: codeDoctorInvalidChange}); err != nil {
		panic("BUG: fib-dummy: register invalid-change event code: " + err.Error())
	}
	if err := diagnostic.RegisterDoctorCheck(holdsRoutesDoctorCheck); err != nil {
		panic("BUG: fib-dummy: register holds-routes doctor check: " + err.Error())
	}
}
