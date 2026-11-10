// Design: docs/architecture/doctor-and-health-checks.md -- the local route asks the running daemon first
// Overview: answer.go -- the one producer and the request it reads
// Related: doctor.go -- runChecks, the local checks the fallback runs
//
// The local `ze doctor` answers for the box, and part of the box's state lives
// only in the running daemon: the runtime-phase checks and the plugins'
// doctor checks read it. So the local route asks the daemon for its
// `show doctor` answer over SSH, the client every `ze <verb>` daemon command
// uses, and draws that answer. When no daemon answers, it runs the local
// checks and adds one doctor-runtime-not-checked warning saying why and which
// command gets the daemon's answer. A failed ask is never read as a healthy
// runtime.

package doctor

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"strconv"
	"time"

	"github.com/ze-software/ze/internal/component/cli/sshclient"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// showDoctorCommand is the daemon form of ze doctor the local route sends.
const showDoctorCommand = "show doctor"

// daemonProbeTimeout bounds the TCP connect that tells "no daemon listens"
// from "this account cannot log in". A listener on the same host answers a
// connect at once; the bound is for an address that drops the SYN.
const daemonProbeTimeout = 2 * time.Second

// The fix line of the doctor-runtime-not-checked row, one per reason no
// daemon answered.
const (
	// fixStartDaemon starts the daemon, whose show doctor runs the runtime checks.
	fixStartDaemon = "ze start"
	// fixAskAsAccount asks the running daemon from an account that can log in to it.
	fixAskAsAccount = `ze cli --user <name> -c "show doctor"`
	// fixAskDaemon asks the running daemon and prints its own rendering of the answer.
	fixAskDaemon = "ze show doctor"
	// fixAskActiveConfig asks the running daemon about the config it holds.
	fixAskActiveConfig = "ze doctor"
	// fixDaemonStatus says whether a daemon runs and where it listens.
	fixDaemonStatus = "ze status"
)

// daemonAsk sends one show doctor command line to the running daemon and
// answers its readiness result. A failure is a *daemonUnanswered, which says
// why no daemon answered and the command that gets an answer.
// askDaemonOverSSH is the production ask; a test hands a stub.
type daemonAsk func(command string) (diagnostic.DoctorResult, error)

// daemonUnanswered is why no daemon answered the local route, and the
// command that gets the daemon's answer.
type daemonUnanswered struct {
	why string
	fix string
}

func (e *daemonUnanswered) Error() string { return e.why }

// answerLocalWith is the local route over one ask: the daemon's answer, drawn
// here, when the daemon answers; otherwise the local checks and the
// doctor-runtime-not-checked row.
func answerLocalWith(validated command.ValidatedArgs, ask daemonAsk) command.LocalAnswer {
	return answerDoctor(validated, func(request doctorRequest, configPath string) command.LocalAnswer {
		result, err := askDaemonDoctor(validated.Tokens(), configPath, ask)
		if err == nil {
			return command.LocalAnswer{Payload: readinessAnswer{DoctorResult: result, asJSON: request.asJSON}}
		}
		diags := runChecks(configPath)
		diags = append(diags, runtimeNotChecked(err))
		return readinessPayload(diags, request.asJSON)
	})
}

// askDaemonDoctor asks the daemon for the readiness answer of the request.
// A config file the daemon would not read (outside its config directories, or
// `-` for standard input) is not sent: the daemon would refuse it, or answer
// for another file, so the local route answers that file itself (R-5).
func askDaemonDoctor(tokens []string, configPath string, ask daemonAsk) (diagnostic.DoctorResult, error) {
	if configPath == "" {
		return ask(showDoctorCommand)
	}
	resolved, err := command.CallerPath(command.RouteRemote, doctorPath, tokens, configPath)
	if err != nil {
		var why textbuf.Buffer
		why.Str("config names ").Str(configPath).
			Str(", which the daemon does not read: it reads a config file only from its own config directories, and never standard input")
		return diagnostic.DoctorResult{}, &daemonUnanswered{why: why.String(), fix: fixAskActiveConfig}
	}
	var line textbuf.Buffer
	line.Str(showDoctorCommand).Byte(' ').Str(configKeyword).Byte(' ').Str(resolved)
	return ask(line.String())
}

// askDaemonOverSSH is the production daemonAsk. It probes the daemon's SSH
// listener before it loads any credential, so "no daemon runs" and "this
// account cannot log in" get different fixes, then asks over SSH for the
// dispatcher's JSON (`| raw`), whatever display format the operator set.
func askDaemonOverSSH(line string) (diagnostic.DoctorResult, error) {
	host, port, err := sshclient.DaemonAddress()
	if err != nil {
		var why textbuf.Buffer
		why.Str("the address of the daemon's SSH listener could not be read (").Err(err).Str(")")
		return diagnostic.DoctorResult{}, &daemonUnanswered{why: why.String(), fix: fixDaemonStatus}
	}
	address := net.JoinHostPort(host, port)
	if err := probeDaemonListener(address); err != nil {
		var why textbuf.Buffer
		why.Str("no daemon listens at ").Str(address).Str(" (").Err(err).Str(")")
		return diagnostic.DoctorResult{}, &daemonUnanswered{why: why.String(), fix: fixStartDaemon}
	}
	creds, err := sshclient.LoadCredentialsNoPrompt()
	if err != nil {
		var why textbuf.Buffer
		why.Str("a daemon listens at ").Str(address).Str(", but this account has no credentials to log in to it (").Err(err).Str(")")
		return diagnostic.DoctorResult{}, &daemonUnanswered{why: why.String(), fix: fixAskAsAccount}
	}
	output, err := sshclient.ExecCommandRaw(creds, line)
	if err != nil {
		var why textbuf.Buffer
		why.Str("the daemon at ").Str(address).Str(" did not answer ").Str(line).Str(" (").Err(err).Str(")")
		return diagnostic.DoctorResult{}, &daemonUnanswered{why: why.String(), fix: fixAskAsAccount}
	}
	return decodeDaemonDoctor(output, address)
}

// probeDaemonListener connects to the daemon's SSH listener and closes the
// connection at once: a refused or timed-out connect means no daemon listens.
func probeDaemonListener(address string) error {
	ctx, cancel := context.WithTimeout(context.Background(), daemonProbeTimeout)
	defer cancel()

	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return err
	}
	conn.Close() //nolint:errcheck // the probe only needed the connect to succeed
	return nil
}

// decodeDaemonDoctor reads the daemon's show doctor JSON. An answer that is
// not a doctor result of the schema this ze reads is no answer: drawing it
// would show an empty, not-ready report the daemon never gave.
func decodeDaemonDoctor(output, address string) (diagnostic.DoctorResult, error) {
	var result diagnostic.DoctorResult
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		var why textbuf.Buffer
		why.Str("the daemon at ").Str(address).Str(" answered show doctor with something that is not a doctor result (").Err(err).Str(")")
		return diagnostic.DoctorResult{}, &daemonUnanswered{why: why.String(), fix: fixAskDaemon}
	}
	if result.SchemaVersion != diagnostic.SchemaVersion {
		var why textbuf.Buffer
		why.Str("the daemon at ").Str(address).Str(" answered a doctor result of schema version ").
			Str(strconv.Itoa(result.SchemaVersion)).Str(", and this ze reads version ").Str(strconv.Itoa(diagnostic.SchemaVersion))
		return diagnostic.DoctorResult{}, &daemonUnanswered{why: why.String(), fix: fixAskDaemon}
	}
	if result.Diagnostics == nil {
		result.Diagnostics = []diagnostic.Diagnostic{}
	}
	return result, nil
}

// runtimeNotChecked is the warning row the local route adds when no daemon
// answered: the runtime checks did not run, why, and the command that gets
// the daemon's answer. An error that is not a *daemonUnanswered (a stub's)
// still names its text, with the login fix.
func runtimeNotChecked(err error) diagnostic.Diagnostic {
	why, fix := err.Error(), fixAskAsAccount
	if unanswered, ok := errors.AsType[*daemonUnanswered](err); ok {
		why, fix = unanswered.why, unanswered.fix
	}
	var message textbuf.Buffer
	message.Str("runtime checks did not run: they need the running daemon, and none answered: ").Str(why).
		Str("; this answer holds only the checks ze doctor runs in its own process")
	return diagnostic.Diagnostic{
		Code:     diagnostic.CodeDoctorRuntimeNotChecked,
		Severity: diagnostic.SeverityWarning,
		Message:  message.String(),
		Fix:      fix,
	}
}
