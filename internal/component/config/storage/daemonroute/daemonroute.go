// Design: docs/architecture/zefs-format.md -- the ze data CLI and its daemon forms
// Related: ../cli/main.go -- ze data rm, write and import, which send a busy mutation here
// Related: ../../cli/cmd_import.go -- ze config import, which sends a busy import here
// Related: ../cli/data_rpc_config.go -- handleDataConfigUpdate, the daemon command a config import becomes

// Package daemonroute is the route a local store mutation takes while a daemon
// owns the store.
//
// A local `ze data` or `ze config import` mutation opens the store as its
// writer. While a daemon owns the store that open answers storage.ErrBusy,
// because the store has one writer, and the work goes to the daemon that owns
// it instead: the daemon acts on its own store handle and the owner lock is
// never lifted. The route takes the steps the local `ze doctor` takes to reach
// `show doctor`: the daemon's address, a probe of its listener, a login
// without a prompt, one command asking for the dispatcher's JSON. The local
// command then prints the lines its local route prints.
//
// The route lives in a package of its own because two command packages send
// through it, `internal/component/config/cli` and
// `internal/component/config/storage/cli`, and neither imports the other.
package daemonroute

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net"
	"path/filepath"
	"strconv"

	"github.com/ze-software/ze/internal/component/cli/sshclient"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/core/configtext"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/redact"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/zefs"
)

// FixDaemonStatus says whether a daemon runs and where it listens.
const FixDaemonStatus = "ze status"

// Ask sends one command line to the daemon that owns the store in the config
// folder and answers the dispatcher's JSON for it. A failure is a *Refusal
// when the daemon answered and refused, and an *Unanswered when no daemon
// answered. OverSSH is the production ask; a test hands a stub.
type Ask func(folder, line string) ([]byte, error)

// Refusal is the daemon's own refusal of a command, as its handler worded it:
// what is wrong, why, and the fix on the last line.
type Refusal struct {
	Message string
}

func (e *Refusal) Error() string { return e.Message }

// Unanswered is why no daemon answered a command (Why, naming the daemon's
// address when it was resolved), the command that gets it answered (Fix), and
// the failure underneath (Cause, nil when Why says all).
type Unanswered struct {
	Why   string
	Fix   string
	Cause error
}

func (e *Unanswered) Error() string {
	if e.Cause == nil {
		return e.Why
	}
	var text textbuf.Buffer
	return text.Str(e.Why).Str(": ").Err(e.Cause).String()
}

// Send sends one command line to the daemon that owns the store at storePath
// (the live tree, `<folder>/database`), through ask, and decodes the JSON it
// answers into answer. It answers the daemon's refusal (*Refusal) or why no
// daemon answered (*Unanswered); an answer that is not JSON of answer's shape
// is the second, because nothing says the command ran. Every local mutation
// that meets a busy store sends its daemon command through this one function.
func Send(ask Ask, storePath, line string, answer any) error {
	body, err := ask(filepath.Dir(storePath), line)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, answer); err != nil {
		var why textbuf.Buffer
		why.Str("the daemon answered ").Str(redact.Command(line)).Str(" with something that is not its answer, so whether it ran is not known")
		return &Unanswered{Why: why.String(), Fix: CommandFix(line), Cause: err}
	}
	return nil
}

// OverSSH is the production Ask. It reads the daemon's address and the
// credentials from the store in folder, the store the local command was
// refused, so a store named with --path or --dir reaches its own daemon. It
// probes the listener before it loads any credential, so "no daemon runs" and
// "this account cannot log in" get different fixes.
func OverSSH(folder, line string) ([]byte, error) {
	host, port, err := sshclient.DaemonAddressOf(folder)
	if err != nil {
		return nil, &Unanswered{
			Why:   "the address of the daemon's SSH listener could not be read from the store",
			Fix:   FixDaemonStatus,
			Cause: err,
		}
	}
	address := net.JoinHostPort(host, port)
	if err := sshclient.ProbeDaemon(address); err != nil {
		var why textbuf.Buffer
		why.Str("no daemon listens at ").Str(address)
		return nil, &Unanswered{Why: why.String(), Fix: FixDaemonStatus, Cause: err}
	}
	creds, err := sshclient.ReadCredentialsNoPrompt(folder)
	if err != nil {
		var why textbuf.Buffer
		why.Str("a daemon listens at ").Str(address).Str(", but this account has no credentials to log in to it")
		return nil, &Unanswered{Why: why.String(), Fix: CommandFix(line), Cause: err}
	}
	var body bytes.Buffer
	answer, err := sshclient.ExecCommandStream(creds, sshclient.RawCommand(line), &body)
	if err == nil {
		return body.Bytes(), nil
	}
	// A stated verdict is the daemon's own answer; anything else is the
	// transport, or an answer cut short, and says nothing about what ran.
	if answer.Verdict == rpc.VerdictError {
		return nil, &Refusal{Message: err.Error()}
	}
	if answer.Verdict == rpc.VerdictAborted {
		return nil, &Refusal{Message: err.Error()}
	}
	var why textbuf.Buffer
	why.Str("the daemon at ").Str(address).Str(" did not answer ").Str(redact.Command(line))
	return nil, &Unanswered{Why: why.String(), Fix: CommandFix(line), Cause: err}
}

// CommandFix is the command that sends line to the daemon from an account
// that can log in to it. The line passes through redact.Command first, as in
// every text of this route an operator sees: a line that carries a content or
// a credential shows the placeholder, which the operator replaces with the
// value they hold.
func CommandFix(line string) string {
	var fix textbuf.Buffer
	return fix.Str(`ze cli --user <name> -c "`).Str(redact.Command(line)).Str(`"`).String()
}

// UnansweredRefusal is the refusal of a local mutation whose store is busy and
// whose daemon did not answer: the store's own refusal first, then why the
// work did not reach the daemon, then the command that gets it done. command
// is the local command the operator typed (ze data rm).
func UnansweredRefusal(command string, busy error, unanswered *Unanswered) *opererr.Error {
	var problem, why textbuf.Buffer
	problem.Str(command).Str(": ").Err(busy)
	why.Str("nothing more was changed: while a daemon owns the store, the work goes to that daemon, and ").Str(unanswered.Why)
	return opererr.Wrap(unanswered.Cause, problem.String(), why.String(), unanswered.Fix)
}

// The daemon command a config import becomes, and its two keywords, as
// ze-data-cmd.yang declares them and handleDataConfigUpdate reads them.
// ConfigLine judges every line it builds by that model before it is sent, so
// a spelling here that drifted from the model is refused on the caller's side.
const (
	pathUpdateDataConfig = "update data config"
	keywordName          = "name"
	keywordContent       = "content"
)

// ConfigLine is the `update data config name <name> content <base64>` line
// that imports data as the config name, judged first by the model as the
// daemon's validator judges it. A content over the bound one SSH packet
// carries is refused here, before anything connects, with the daemon's own
// refusal: the bound in bytes and the route for a larger config.
func ConfigLine(name string, data []byte) (string, error) {
	encoded := base64.StdEncoding.EncodeToString(data)
	tokens := []string{keywordName, name, keywordContent, encoded}
	if _, err := command.ValidateModelArgs(pathUpdateDataConfig, tokens, nil); err != nil {
		return "", err
	}
	var line textbuf.Buffer
	line.Str(pathUpdateDataConfig).Byte(' ').Str(keywordName).Byte(' ').Str(configtext.QuoteCommandWord(name)).
		Byte(' ').Str(keywordContent).Byte(' ').Str(encoded)
	return line.String(), nil
}

// ConfigAnswer is the answer of `update data config`, as far as a local
// command reads it: the config it imported and the bytes it imported.
type ConfigAnswer struct {
	Name  string `json:"name"`
	Bytes int    `json:"bytes"`
}

// Mismatch refuses an answer that names another config or another byte count
// than the import of octets bytes into name sent: nothing then says the
// config holds the file, so the file is not counted. It answers nil when the
// answer confirms the import. command is the local command the operator typed.
func (a ConfigAnswer) Mismatch(command, name string, octets int) *opererr.Error {
	if a.Name == name {
		if a.Bytes == octets {
			return nil
		}
	}
	var problem, fix textbuf.Buffer
	problem.Str(command).Str(": the daemon answered the import of ").Str(strconv.Quote(a.Name)).
		Str(" (").Int(int64(a.Bytes)).Str(" bytes), not of ").Str(name).
		Str(" (").Int(int64(octets)).Str(" bytes)")
	fix.Str("ze data cat ").Str(configtext.QuoteCommandWord(zefs.KeyFileActive.Key(name)))
	return opererr.New(problem.String(), "whether "+name+" holds the file is not known, so it is not counted", fix.String())
}
