// Design: docs/architecture/core-design.md -- a service whose configuration cannot be served is not started, and never stops ze
//
// Always on. The start path (main.go, runYANGConfig) records here each service
// it leaves down because its configuration cannot be served, and reports it:
// one error line on the log under codeDoctorServiceNotStarted, the same text
// on the terminal, and one finding of the runtime doctor check
// register_services_not_started.go registers, which `ze doctor` reads from the
// running daemon.
//
// The check is a runtime check because what it reports is a decision this
// daemon took at start, from its own environment and command line as well as
// its config: the local `ze doctor` process holds none of that.
//
// Related: register_services_not_started.go -- the doctor check that reads the record
// Related: register_l2tp.go -- registerBNGSubsystems, which returns per-subsystem serviceNotStarted records
// Related: service_tls.go -- reportWebNotStarted, the certificate form of the same report
// Related: lg_transport.go -- reportLGNotStarted and reportLGServerDown, the looking-glass forms
// Related: doctor_codes.go -- the explanation of codeDoctorServiceNotStarted

package hub

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"sync"

	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// The names a report gives the services the start path can leave down.
const (
	serviceNameWeb   = "web server"
	serviceNameMCP   = "MCP server"
	serviceNameREST  = "REST API"
	serviceNameGRPC  = "gRPC API"
	serviceNameL2TP  = "L2TP subsystem"
	serviceNamePPPoE = "PPPoE subsystem"
)

// keepsRunning ends every effect: the operator reads that ze itself is up.
const keepsRunning = "; ze and its other services keep running"

// serviceNotStarted is one service, or one part of a service, that this start
// left down because its configuration cannot be served.
type serviceNotStarted struct {
	// code is the doctor diagnostic code logged and reported for this record.
	// Empty defaults to codeDoctorServiceNotStarted.
	code string
	// service names what is down, as the operator knows it.
	service string
	// effect is what the start did about it, as one clause.
	effect string
	// problem is what is wrong, why, and the fix.
	problem *opererr.Error
}

func (d serviceNotStarted) doctorCode() string {
	if d.code != "" {
		return d.code
	}
	return codeDoctorServiceNotStarted
}

// notStarted answers the record of a service the start does not start.
// problem MUST NOT be nil.
func notStarted(service string, problem *opererr.Error) serviceNotStarted {
	return serviceNotStarted{
		service: service,
		effect:  "the " + service + " is not started" + keepsRunning,
		problem: problem,
	}
}

// dataPlaneExcluded answers the record of one data-plane-mismatched feature
// that LoadConfigForStart left off in memory so the daemon starts on its active
// data plane and reports the mismatch under doctor-dataplane-mismatch.
func dataPlaneExcluded(e zeconfig.ExcludedDataPlane) serviceNotStarted {
	return serviceNotStarted{
		code:    zeconfig.CodeDoctorDataPlaneMismatch,
		service: "data-plane feature",
		effect:  "that feature is left off" + keepsRunning,
		problem: opererr.New(e.Problem, e.Why, e.Fix),
	}
}

// servicesNotStarted holds what this daemon's start left down. The start path
// appends, and the runtime doctor check reads. Safe for concurrent use through
// its mutex.
var servicesNotStarted struct {
	sync.Mutex
	list []serviceNotStarted
}

// resetServicesNotStarted empties the record. The start path MUST call it
// before it decides any service, so that a second start in one process (an
// in-place restart, a test) reports only its own decisions.
func resetServicesNotStarted() {
	servicesNotStarted.Lock()
	defer servicesNotStarted.Unlock()
	servicesNotStarted.list = nil
}

// servicesNotStartedSnapshot answers a copy of the record.
func servicesNotStartedSnapshot() []serviceNotStarted {
	servicesNotStarted.Lock()
	defer servicesNotStarted.Unlock()
	return append([]serviceNotStarted(nil), servicesNotStarted.list...)
}

// reportServiceNotStarted tells the operator that this start runs without one
// service: an error line on log under the doctor code the runtime check
// reports the same service with, the same text on console, and the record that
// check reads.
func reportServiceNotStarted(log *slog.Logger, console io.Writer, down serviceNotStarted) {
	servicesNotStarted.Lock()
	servicesNotStarted.list = append(servicesNotStarted.list, down)
	servicesNotStarted.Unlock()

	if down.code == zeconfig.CodeDoctorDataPlaneMismatch {
		log.LogAttrs(context.Background(), slog.LevelError, down.problem.Problem(),
			down.problem.LogAttrs(slog.String("code", zeconfig.CodeDoctorDataPlaneMismatch),
				slog.String("service", down.service), slog.String("effect", down.effect))...)
	} else {
		log.LogAttrs(context.Background(), slog.LevelError, down.problem.Problem(),
			down.problem.LogAttrs(slog.String("code", codeDoctorServiceNotStarted),
				slog.String("service", down.service), slog.String("effect", down.effect))...)
	}
	var line textbuf.Buffer
	line.Str("error: ").Str(down.effect).Str(": ").Str(down.problem.Error()).Byte('\n')
	if _, err := console.Write(line.Bytes()); err != nil {
		log.Warn("the report of a service that is not started could not be written to the terminal",
			"code", codeDoctorServiceNotStarted, "service", down.service, "error", err)
	}
}

// checkServicesNotStarted is the runtime doctor check: one error finding per
// service this daemon's start left down, in the order the start decided them.
// A daemon that started every service it was asked for answers nothing.
func checkServicesNotStarted(diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	down := servicesNotStartedSnapshot()
	if len(down) == 0 {
		return nil
	}
	diags := make([]diagnostic.Diagnostic, 0, len(down))
	for _, service := range down {
		diags = append(diags, serviceNotStartedDiagnostic(service))
	}
	return diags
}

// serviceNotStartedDiagnostic lowers one record to a doctor finding: the
// problem, why and what the start did as the message, the fix on its own line,
// and the cause, when there is one, as what was found.
func serviceNotStartedDiagnostic(down serviceNotStarted) diagnostic.Diagnostic {
	var message textbuf.Buffer
	diag := diagnostic.Diagnostic{
		Code:     down.doctorCode(),
		Severity: diagnostic.SeverityError,
		Message:  message.Str(down.problem.Problem()).Byte(' ').Str(down.problem.Why()).Str("; ").Str(down.effect).String(),
		Fix:      down.problem.Fix(),
	}
	if cause := down.problem.Unwrap(); cause != nil {
		diag.Actual = cause.Error()
	}
	return diag
}

// listenUnreadable answers the operator error of a service whose address list
// comes from an environment variable that does not parse. key is the variable
// as the operator sets it, value what it holds, and example a value it takes.
// The service is not started on the addresses its config block names instead:
// the variable outranks the block, so that would bind addresses the operator
// overrode.
func listenUnreadable(service, key, value, example string, cause error) *opererr.Error {
	return opererr.Wrap(cause,
		"The "+service+" cannot read the addresses it binds: "+key+" holds \""+value+"\".",
		key+" outranks the addresses the config names, and its value is not a list of listen endpoints",
		"Set "+key+" to endpoints such as "+example+", or unset it, then restart ze.")
}

// splitListenHostPort reads the one host:port an API transport's listen
// variable holds. The problem is nil when it reads, and otherwise the operator
// error of that transport, naming the variable, its value and the fix.
func splitListenHostPort(service, key, value, example string) (host, port string, problem *opererr.Error) {
	host, port, err := net.SplitHostPort(value)
	if err != nil {
		return "", "", listenUnreadable(service, key, value, example, err)
	}
	return host, port, nil
}

// configBlockDisplay formats a config tree path such as "environment/web" or
// "bus/hub/server[main]" in the space-separated syntax an operator writes in
// configuration and commands ("environment web", "bus hub server main").
func configBlockDisplay(path string) string {
	var tb textbuf.Buffer
	needSpace := false
	for i := range len(path) {
		switch ch := path[i]; ch {
		case '/', '[', ']':
			if tb.Len() > 0 {
				needSpace = true
			}
		default:
			if needSpace {
				tb.Byte(' ')
				needSpace = false
			}
			tb.Byte(ch)
		}
	}
	return tb.String()
}

// blockUnservable answers the operator error of a service whose config block
// is inconsistent, as cause says. block is the block as the operator writes
// it, and configPath the file `ze config validate` reads the same refusal from.
func blockUnservable(service, block, configPath string, cause error) *opererr.Error {
	return opererr.Wrap(cause,
		"The "+service+" cannot start: `"+block+"` cannot be served as written.",
		"ze never starts a service on an approximation of its configuration",
		"Correct `"+block+"` as `ze config validate "+configPath+"` reports, then restart ze.")
}

// buildFailed answers the operator error of a service whose listener could not
// be built at start, as cause says.
func buildFailed(service, block string, cause error) *opererr.Error {
	return opererr.Wrap(cause,
		"The "+service+" could not be started.",
		"its listener could not be built from `"+block+"` and the environment variables that override it",
		"Correct what the cause names in `"+block+"`, then restart ze.")
}

// lgRangesUnused answers the record of a looking glass that serves without the
// names of its next-hop ranges because the list cannot be used as written.
// The names label next hops the looking glass shows anyway, so the list is the
// narrowest part that cannot be served: the looking glass itself serves
// everything a restart could serve from the same config.
func lgRangesUnused(cause error) serviceNotStarted {
	fix := "Correct `environment looking-glass next-hop-range`, then restart ze."
	// Every refusal of the list is an operator error whose fix names the entry
	// to delete (zeconfig.ExtractLGNextHopRanges). Keep that fix, which is
	// narrower than the one above.
	if refusal, isOperatorError := errors.AsType[*opererr.Error](cause); isOperatorError {
		fix = refusal.Fix() + ", then restart ze."
	}
	return serviceNotStarted{
		service: "looking glass next-hop range names",
		effect:  "the looking glass serves without next-hop range names" + keepsRunning,
		problem: opererr.Wrap(cause,
			"The looking glass cannot use its next-hop range names.",
			"`environment looking-glass next-hop-range` cannot be used as written, and ze never shows names from part of a list",
			fix),
	}
}
