// Design: docs/architecture/config/yang-config-design.md -- extensions that name a registration
// Related: main.go -- runYANGConfig, which calls refuseWronglyBuilt before the first pass
// Related: ../../../internal/component/plugin/server/build_registration_report.go -- the event and the last-stop record

package hub

import (
	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/yang"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// refuseWronglyBuilt judges the module set compiled into this ze before the
// config is parsed: a module that names a ze:validate function or a
// ze:reference source this ze does not hold refuses the start, because such a
// leaf would be checked by nothing or refuse every value. The refusal goes to
// the terminal and the log as the other start refusals do, is raised under
// doctor-build-registration-unheld, and is written to the last-stop record so
// the next `ze doctor` names it. It reports whether the start must end.
//
// The judge is the one the join applies to an external plugin's module
// (yang.RefuseUnheldCompiledRegistrations), so both sides refuse by one rule.
func refuseWronglyBuilt() bool {
	set, err := yang.DefaultLoader()
	if err != nil {
		// A set that does not resolve holds no entry to judge. The full parse
		// below refuses the start with the resolve error, as it always did.
		return false
	}
	refusal := zeconfig.RefuseUnheldRegistrations(set)
	if refusal == nil {
		return false
	}
	var tb textbuf.Buffer
	tb.Str("error: ze build: ").Err(refusal).Byte('\n')
	tb.StdErr() //nolint:errcheck // the start ends on the caller's next line
	logStartupFailure("ze build", refusal)
	pluginserver.ReportUnheldRegistration(refusal)
	pluginserver.RecordUnheldRegistrationStop(refusal)
	return true
}
