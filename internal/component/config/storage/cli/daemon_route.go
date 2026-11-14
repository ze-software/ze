// Design: docs/architecture/zefs-format.md -- the ze data CLI and its daemon forms
// Related: main.go -- cmdRm, the local command that sends a busy removal here
// Related: data_rpc.go -- handleDataKeyDelete, the daemon command a removal becomes
//
// A local `ze data` mutation opens the store as its writer. While a daemon owns
// the store that open answers storage.ErrBusy, because the store has one
// writer, and the work goes to the daemon that owns it instead: the daemon
// acts on its own store handle and the owner lock is never lifted. The route
// takes the steps the local `ze doctor` takes to reach `show doctor`: the
// daemon's address, a probe of its listener, a login without a prompt, one
// command asking for the dispatcher's JSON. The local command then prints the
// lines its local route prints.

package cli

import (
	"bytes"
	"encoding/json"
	"net"
	"path/filepath"

	"github.com/ze-software/ze/internal/component/cli/sshclient"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/redact"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// fixDaemonStatus says whether a daemon runs and where it listens.
const fixDaemonStatus = "ze status"

// daemonAsk sends one command line to the daemon that owns the store in the
// config folder and answers the dispatcher's JSON for it. A failure is a
// *daemonRefusal when the daemon answered and refused, and a
// *daemonUnanswered when no daemon answered. askDaemonOverSSH is the
// production ask; a test hands a stub.
type daemonAsk func(folder, line string) ([]byte, error)

// daemonRefusal is the daemon's own refusal of a command, as its handler worded
// it: what is wrong, why, and the fix on the last line.
type daemonRefusal struct {
	message string
}

func (e *daemonRefusal) Error() string { return e.message }

// daemonUnanswered is why no daemon answered a command, and the command that
// gets it answered. why names the daemon's address when it was resolved.
type daemonUnanswered struct {
	why   string
	fix   string
	cause error
}

func (e *daemonUnanswered) Error() string {
	if e.cause == nil {
		return e.why
	}
	var text textbuf.Buffer
	return text.Str(e.why).Str(": ").Err(e.cause).String()
}

// sendToDaemon sends one command line to the daemon that owns the store at
// storePath, through ask, and decodes the JSON it answers into answer. It
// answers the daemon's refusal (*daemonRefusal) or why no daemon answered
// (*daemonUnanswered); an answer that is not JSON of answer's shape is the
// second, because nothing says the command ran. Each local mutation that
// meets a busy store sends its daemon command through this one function.
func sendToDaemon(ask daemonAsk, storePath, line string, answer any) error {
	body, err := ask(filepath.Dir(storePath), line)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, answer); err != nil {
		var why textbuf.Buffer
		why.Str("the daemon answered ").Str(redact.Command(line)).Str(" with something that is not its answer, so whether it ran is not known")
		return &daemonUnanswered{why: why.String(), fix: daemonCommandFix(line), cause: err}
	}
	return nil
}

// askDaemonOverSSH is the production daemonAsk. It reads the daemon's address
// and the credentials from the store in folder, the store the local command
// was refused, so a store named with --path reaches its own daemon. It probes
// the listener before it loads any credential, so "no daemon runs" and "this
// account cannot log in" get different fixes.
func askDaemonOverSSH(folder, line string) ([]byte, error) {
	host, port, err := sshclient.DaemonAddressOf(folder)
	if err != nil {
		return nil, &daemonUnanswered{
			why:   "the address of the daemon's SSH listener could not be read from the store",
			fix:   fixDaemonStatus,
			cause: err,
		}
	}
	address := net.JoinHostPort(host, port)
	if err := sshclient.ProbeDaemon(address); err != nil {
		var why textbuf.Buffer
		why.Str("no daemon listens at ").Str(address)
		return nil, &daemonUnanswered{why: why.String(), fix: fixDaemonStatus, cause: err}
	}
	creds, err := sshclient.ReadCredentialsNoPrompt(folder)
	if err != nil {
		var why textbuf.Buffer
		why.Str("a daemon listens at ").Str(address).Str(", but this account has no credentials to log in to it")
		return nil, &daemonUnanswered{why: why.String(), fix: daemonCommandFix(line), cause: err}
	}
	var body bytes.Buffer
	answer, err := sshclient.ExecCommandStream(creds, sshclient.RawCommand(line), &body)
	if err == nil {
		return body.Bytes(), nil
	}
	// A stated verdict is the daemon's own answer; anything else is the
	// transport, or an answer cut short, and says nothing about what ran.
	if answer.Verdict == rpc.VerdictError {
		return nil, &daemonRefusal{message: err.Error()}
	}
	if answer.Verdict == rpc.VerdictAborted {
		return nil, &daemonRefusal{message: err.Error()}
	}
	var why textbuf.Buffer
	why.Str("the daemon at ").Str(address).Str(" did not answer ").Str(redact.Command(line))
	return nil, &daemonUnanswered{why: why.String(), fix: daemonCommandFix(line), cause: err}
}

// daemonCommandFix is the command that sends line to the daemon from an
// account that can log in to it. The line passes through redact.Command
// first, as in every text of this route an operator sees: a line that carries
// a content or a credential shows the placeholder, which the operator replaces
// with the value they hold.
func daemonCommandFix(line string) string {
	var fix textbuf.Buffer
	return fix.Str(`ze cli --user <name> -c "`).Str(redact.Command(line)).Str(`"`).String()
}

// unansweredRefusal is the refusal of a local mutation whose store is busy
// and whose daemon did not answer: the store's own refusal first, then why the
// work did not reach the daemon, then the command that gets it done. command
// is the local command the operator typed (ze data rm).
func unansweredRefusal(command string, busy error, unanswered *daemonUnanswered) *opererr.Error {
	var problem, why textbuf.Buffer
	problem.Str(command).Str(": ").Err(busy)
	why.Str("nothing more was changed: while a daemon owns the store, the work goes to that daemon, and ").Str(unanswered.why)
	return opererr.Wrap(unanswered.cause, problem.String(), why.String(), unanswered.fix)
}
