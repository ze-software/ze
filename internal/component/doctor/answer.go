// Design: docs/architecture/doctor-and-health-checks.md -- one producer answers ze doctor, doctor and show doctor
// Overview: doctor.go -- the readiness checks and the local drawing
// Related: kernel_capabilities.go -- the kernel-capabilities answer
// Related: register.go -- the local-data registration of doctor

package doctor

import (
	"errors"
	"os"
	"strings"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/kernelcap"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// doctorPath is the command the local route answers and the refusals name.
const doctorPath = "doctor"

// RuntimeChecks answers the doctor checks of the plugins a running daemon
// holds. The daemon route passes the dispatcher's; the local route passes nil,
// because the caller's own process runs no plugin, so it has no plugin check to
// ask. Answer reads nil as that fact, never as a failure to look.
type RuntimeChecks func() []diagnostic.Diagnostic

// readinessAnswer is the answer of the readiness checks. It marshals as the
// DoctorResult every surface already reads (`schema-version`, `ready`,
// `diagnostics`). asJSON is the local `--json` rendering flag: it is never
// marshalled, because on the daemon route the caller's pipe renders.
type readinessAnswer struct {
	diagnostic.DoctorResult
	asJSON bool
}

// kernelAnswer is the answer of the kernel-capabilities mode, marshalled as
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
// the caller's own host, with no plugin check, since no plugin runs here.
func answerLocal(validated command.ValidatedArgs) command.LocalAnswer {
	return Answer(validated, nil)
}

// Answer is the one producer of the doctor answer. `ze doctor` reaches it
// through the local-data route, and the daemon's `doctor` and `show doctor`
// reach it through internal/component/doctor/cmd with the plugin checks the
// dispatcher holds. It writes nothing: refusals and the help page ride in the
// Diagnostic, and a not-ready result is a payload with code 0, so the daemon
// route answers it as data, as `show doctor` always did. The local drawing
// (renderDoctor) exits 1 for it.
func Answer(validated command.ValidatedArgs, runtime RuntimeChecks) command.LocalAnswer {
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

	diags := runChecks(configPath)
	if runtime != nil {
		diags = append(diags, runtime()...)
	}
	return command.LocalAnswer{Payload: readinessAnswer{DoctorResult: diagnostic.NewDoctorResult(readyOf(diags), diags), asJSON: request.asJSON}}
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

// doctorHelpText renders the usage page, coloured only for an operator's own
// terminal: a daemon caller reads it as text on another surface.
func doctorHelpText(route command.Route) string {
	page := doctorHelpPage()
	var out strings.Builder
	page.WriteTo(&out, helpColored(route))
	return out.String()
}

// helpColored answers whether the help page carries ANSI colour: only on the
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
