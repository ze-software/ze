// Design: docs/architecture/doctor-and-health-checks.md -- checks that read the running daemon's state
// Related: doctor_registry.go -- the checks every route runs
// Related: doctor_provider.go -- provider bridge for show doctor

package diagnostic

import (
	"cmp"
	"errors"
	"slices"
	"sync"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// DaemonDoctorCheck is a doctor check that judges state only the running
// daemon holds: a registry its plugins fill at runtime. It takes no context,
// because it reads that state where it lives.
//
// Only the daemon route runs it (`show doctor`, the daemon form of
// `ze doctor`). In the operator's own process the state it reads is empty,
// and an empty registry would answer "no problem" for a box it never looked
// at. Codes names every code Check can put in a diagnostic; each MUST resolve
// through Lookup.
type DaemonDoctorCheck struct {
	Name  string
	Codes []string
	Check func() []Diagnostic
}

var daemonDoctorChecks = struct {
	sync.Mutex
	entries []DaemonDoctorCheck
}{}

var errDaemonDoctorCheckIncomplete = errors.New("diagnostic: a daemon doctor check needs a name, a code and a check function")

// RegisterDaemonDoctorCheck adds a check the daemon route runs. Owner packages
// call it from init(). It refuses an unnamed check, one with no code or no
// function, and a name already registered.
func RegisterDaemonDoctorCheck(check DaemonDoctorCheck) error {
	if check.Name == "" {
		return errDaemonDoctorCheckIncomplete
	}
	if len(check.Codes) == 0 {
		return errDaemonDoctorCheckIncomplete
	}
	if check.Check == nil {
		return errDaemonDoctorCheckIncomplete
	}
	daemonDoctorChecks.Lock()
	defer daemonDoctorChecks.Unlock()
	for i := range daemonDoctorChecks.entries {
		if daemonDoctorChecks.entries[i].Name == check.Name {
			var tb textbuf.Buffer
			return errors.New(tb.Str("diagnostic: duplicate daemon doctor check ").Str(check.Name).String())
		}
	}
	check.Codes = slices.Clone(check.Codes)
	daemonDoctorChecks.entries = append(daemonDoctorChecks.entries, check)
	return nil
}

// DaemonDoctorChecks answers every registered daemon check, sorted by name.
// Safe for concurrent use.
func DaemonDoctorChecks() []DaemonDoctorCheck {
	daemonDoctorChecks.Lock()
	defer daemonDoctorChecks.Unlock()
	checks := make([]DaemonDoctorCheck, len(daemonDoctorChecks.entries))
	for i := range daemonDoctorChecks.entries {
		checks[i] = daemonDoctorChecks.entries[i]
		checks[i].Codes = slices.Clone(checks[i].Codes)
	}
	slices.SortFunc(checks, func(a, b DaemonDoctorCheck) int { return cmp.Compare(a.Name, b.Name) })
	return checks
}

// RunDaemonDoctorChecks runs every registered daemon check, in name order,
// and answers their findings together. Only the daemon route MUST call it:
// on any other route the state these checks read is empty.
func RunDaemonDoctorChecks() []Diagnostic {
	var diags []Diagnostic
	for _, check := range DaemonDoctorChecks() {
		diags = append(diags, check.Check()...)
	}
	return diags
}
