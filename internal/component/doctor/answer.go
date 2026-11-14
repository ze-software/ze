// Design: docs/architecture/doctor-and-health-checks.md -- one producer answers ze doctor and show doctor
// Overview: doctor.go -- the readiness checks and the local drawing
// Related: kernel_capabilities.go -- the kernel-capabilities answer
// Related: register.go -- the local-data registration of doctor
// Related: local_route.go -- the local route asks the running daemon first

package doctor

import (
	"errors"
	"os"
	"strings"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/kernelcap"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// doctorPath is the command the local route answers and the refusals name.
const doctorPath = "doctor"

// RuntimeChecks answers the checks that read a running daemon's live state:
// the runtime phase of the doctor registry and the doctor checks of the
// plugins the daemon runs. The daemon route passes the dispatcher's. The
// local route never calls Answer: it asks the daemon for its answer instead
// (answerLocalWith), because the caller's own process holds no live state to
// read. Answer reads nil as "no runtime check to add", which only a test
// passes, never as a failure to look.
//
// Answer hands it the tree its post-config phase judged (runChecks), nil when
// that phase did not run. The tree is the saved configuration doctor read,
// never the running one, so a runtime check uses it only to agree with a
// config-phase check that read the same file.
type RuntimeChecks func(judged *config.Tree) []diagnostic.Diagnostic

// readinessAnswer is the answer of the readiness checks. It marshals as the
// DoctorResult every surface already reads (`schema-version`, `ready`,
// `diagnostics`). asJSON is the local `--json` rendering flag: it is never
// marshaled, because on the daemon route the caller's pipe renders.
type readinessAnswer struct {
	diagnostic.DoctorResult
	asJSON bool
}

// kernelAnswer is the answer of the kernel-capabilities mode, marshaled as
// kernelCapabilitiesAnswer. asJSON is the local `--json` rendering flag.
type kernelAnswer struct {
	kernelCapabilitiesAnswer
	asJSON bool
}

// doctorRequest is what the caller asked: the words after doctor, read once.
type doctorRequest struct {
	asJSON     bool
	kernel     bool
	help       bool
	configPath string
}

// answerLocal is the local-data handler of doctor: the readiness answer of
// the caller's own host, asked of the daemon over SSH when one answers
// (local_route.go), and answered here with the doctor-runtime-not-checked row
// when none does. A test that builds its daemon in process hands that daemon
// to the ask instead of the SSH listener (SetDaemonExecForTest).
func answerLocal(validated command.ValidatedArgs) command.LocalAnswer {
	if exec := daemonExecOverride.Load(); exec != nil {
		return answerLocalWith(validated, func(line string) (diagnostic.DoctorResult, error) {
			return askDaemonOverride(*exec, line)
		})
	}
	return answerLocalWith(validated, askDaemonOverSSH)
}

// readinessFunc answers the readiness checks of one request, once its words
// are read and its config path is resolved for the route. Answer and the
// local route differ only here: where the readiness checks run.
type readinessFunc func(request doctorRequest, configPath string) command.LocalAnswer

// Answer is the daemon's producer of the doctor answer: `show doctor` reaches
// it through internal/component/doctor/cmd with the runtime checks the
// dispatcher holds, and `ze doctor` draws its answer when the daemon answers
// the local route's ask (local_route.go). Both share answerDoctor and
// runChecks. It writes nothing: refusals and the help page ride in the
// Diagnostic, and a not-ready result is a payload with code 0, so the daemon
// route answers it as data, as `show doctor` always did. The local drawing
// (renderDoctor) exits 1 for it.
func Answer(validated command.ValidatedArgs, runtime RuntimeChecks) command.LocalAnswer {
	return answerDoctor(validated, func(request doctorRequest, configPath string) command.LocalAnswer {
		diags, judged := runChecks(configPath)
		if runtime != nil {
			diags = append(diags, runtime(judged)...)
		}
		return readinessPayload(diags, request.asJSON)
	})
}

// answerDoctor reads the request, answers the help page, a refusal and the
// kernel-capabilities mode itself, resolves the config path for the route,
// and hands the readiness request to readiness.
func answerDoctor(validated command.ValidatedArgs, readiness readinessFunc) command.LocalAnswer {
	route := validated.Route()
	tokens := validated.Tokens()
	request, err := readDoctorRequest(tokens)
	if err != nil {
		return doctorRefusal(route, err.Error())
	}
	if request.help {
		return command.LocalAnswer{Diagnostic: doctorHelpText(route)}
	}

	// The kernel-capabilities mode asks about the kernel, not a configuration,
	// so a config path beside it is a contradiction rather than ignored input.
	if request.kernel {
		if request.configPath != "" {
			return doctorRefusal(route, kernelCapabilitiesKeyword+" takes no config file: "+request.configPath)
		}
		return command.LocalAnswer{Payload: kernelAnswer{kernelCapabilitiesAnswer: newKernelCapabilitiesAnswer(kernelcap.ProbeAll()), asJSON: request.asJSON}}
	}

	configPath := request.configPath
	if configPath != "" {
		// On the daemon route the file must lie in a config directory of the
		// daemon's host, and `-` (stdin) is refused: no surface carries a
		// stdin stream to the dispatcher. The local route reads it unchanged.
		resolved, pathErr := command.CallerPath(route, doctorPath, tokens, configPath)
		if pathErr != nil {
			return command.LocalRefusal(1, command.LocalErrorLine(pathErr))
		}
		configPath = resolved
	}

	return readiness(request, configPath)
}

// readinessPayload is the readiness answer over diags, ready when none is an
// error.
func readinessPayload(diags []diagnostic.Diagnostic, asJSON bool) command.LocalAnswer {
	return command.LocalAnswer{Payload: readinessAnswer{DoctorResult: diagnostic.NewDoctorResult(readyOf(diags), diags), asJSON: asJSON}}
}

// readyOf answers whether no diagnostic is an error.
func readyOf(diags []diagnostic.Diagnostic) bool {
	for i := range diags {
		if diags[i].Severity == diagnostic.SeverityError {
			return false
		}
	}
	return true
}

// readDoctorRequest reads the words after doctor. The config file is the value
// of the config keyword (ai/rules/cli.md: keyword before value), so no
// free-form token sits in a positional slot. `--json` is the local rendering
// flag; the daemon dispatcher refuses a flag before the handler runs.
func readDoctorRequest(args []string) (doctorRequest, error) {
	var request doctorRequest
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch arg {
		case "--json":
			request.asJSON = true
		case kernelCapabilitiesKeyword:
			request.kernel = true
		case configKeyword:
			if request.configPath != "" {
				return request, errors.New("config is named once; a second config keyword was given")
			}
			if index+1 >= len(args) {
				return request, errors.New("config needs a file: ze doctor config <file>")
			}
			index++
			// "-" alone is stdin (cliio); any other dash-leading token is an option, never a path.
			if strings.HasPrefix(args[index], "-") && args[index] != "-" {
				return request, errors.New("config needs a file, not the option " + args[index] + ": ze doctor config <file>")
			}
			request.configPath = args[index]
		case "help", "-h", "--help":
			request.help = true
			return request, nil
		default:
			return request, errors.New("unexpected argument: " + arg + "; a config file is named with ze doctor config <file>")
		}
	}
	return request, nil
}

// doctorRefusal answers one error line and the usage page, exit 1: the bytes
// `ze doctor` has always written to stderr for a refused argument.
func doctorRefusal(route command.Route, reason string) command.LocalAnswer {
	var msg textbuf.Buffer
	msg.Str("error: ").Str(reason).Byte('\n')
	return command.LocalRefusal(1, msg.String()+doctorHelpText(route))
}

// doctorHelpText renders the usage page, colored only for an operator's own
// terminal: a daemon caller reads it as text on another surface.
func doctorHelpText(route command.Route) string {
	page := doctorHelpPage()
	var out strings.Builder
	page.WriteTo(&out, helpColored(route))
	return out.String()
}

// helpColored answers whether the help page carries ANSI color: only on the
// local route, and only when stderr, where the local route prints it, is a
// terminal.
func helpColored(route command.Route) bool {
	if !route.CallerLocal() {
		return false
	}
	return slogutil.UseColor(os.Stderr)
}

// renderDoctor is the local drawing of the doctor answer: the text report, or
// indented JSON under `--json`, and exit 1 when the box is not ready. Any
// other payload draws through the registered renderers.
func renderDoctor(path string, payload any) int {
	switch answer := payload.(type) {
	case readinessAnswer:
		if answer.asJSON {
			return outputJSON(answer.DoctorResult)
		}
		return outputText(answer.DoctorResult)
	case kernelAnswer:
		return outputKernelCapabilities(answer.kernelCapabilitiesAnswer, answer.asJSON)
	}
	return command.RenderLocalAnswer(path, payload)
}
