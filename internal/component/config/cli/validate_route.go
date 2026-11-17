// Design: docs/architecture/config/yang-config-design.md — `ze config validate` asks the running ze first
// Overview: cmd_validate.go — cmdValidate, which prints the verdict this file answers
// Related: config_supplied.go — suppliedValidation, the daemon command this file sends
// Related: config_data.go — dataValidate, the daemon command a config too large to send is named to
// Related: ../../doctor/local_route.go — the local `ze doctor`, which takes the same walk to the daemon
//
// A config that declares an external plugin can hold a section only that
// plugin's schema defines, and ze reads that schema from the plugin when
// `ze start` runs it. A command with no daemon starts no plugin (owner
// ruling), so it cannot judge such a section, and a running ze can. So
// `ze config validate` asks the running ze first for such a config: it sends
// the file's bytes in the `content` leaf of `show config supplied validation`,
// since the daemon reads a file a caller names only from its own config
// directories (command.CallerPath), and draws the daemon's verdict with its
// own renderer and exit codes. When no daemon answers it judges the config in
// its own process, and the refusal of an unknown name says why no daemon
// answered and the one command that gets an answer. A failed ask is never read
// as a verdict.
//
// A config that declares no such plugin is judged in this process and no
// daemon is asked: every schema it needs is compiled into this binary, so the
// verdict is complete here.

package cli

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/ze-software/ze/internal/component/cli/sshclient"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/configtext"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// The two daemon commands `ze config validate` sends, as
// ze-config-cli-cmd.yang declares them. pathSuppliedValidation takes the
// config's bytes (its keywords are suppliedContent and suppliedNameArg, which
// suppliedHandler reads); pathShowConfigValidation takes a file the daemon
// reads itself, in its `file` leaf (dataValidate). validationLine judges the
// supplied form by its model before it is sent, so a spelling here that
// drifted from the model is refused on this side.
const (
	pathSuppliedValidation   = "show config supplied validation"
	pathShowConfigValidation = "show config validation"
	leafFile                 = "file"
)

// The fix line of a failed ask, one per reason no running ze answered.
const (
	// fixStartDaemon starts ze, which reads each declared plugin's schema.
	fixStartDaemon = "ze start"
	// fixDaemonStatus says whether a ze runs, which one, and where it listens.
	fixDaemonStatus = "ze status"
	// fixValidateAsAccount asks the running ze from an account that can log
	// in to it; the config file follows.
	fixValidateAsAccount = "ze config validate --user <name> "
	// fixValidateFile checks a config file; the file follows.
	fixValidateFile = "ze config validate "
)

// daemonVerdict is what the running ze sent back for one validation line.
type daemonVerdict struct {
	// body is the dispatcher's JSON for the command.
	body []byte
	// refused is true when the daemon stated a failed status beside body,
	// which is how it answers a config it judges invalid.
	refused bool
	// address is the daemon's listener, for a message about its answer.
	address string
}

// validationAsk sends one validation line to the running ze, logging in as
// user ("" is the stored account, read with no prompt), and answers what it
// sent back. A failure is an *askFailure or an *sshclient.Unreached.
// askValidationOverSSH is the production ask; a test hands a stub.
type validationAsk func(user, line string) (daemonVerdict, error)

// askFailure is why no running ze answered, as an error.
type askFailure struct {
	unanswered config.DaemonUnanswered
}

func (e *askFailure) Error() string { return e.unanswered.Why }

// judgeConfig answers the verdict `ze config validate` prints for input, the
// config read from path.
//
// A config that declares an external plugin this process holds no schema of
// goes to the running ze first, and its answer is the verdict. When no ze
// answers, or the config declares no such plugin, the verdict is this
// process's own (runValidation), which says why no ze answered beside the
// refusal of a name no compiled-in schema defines.
func judgeConfig(input, path, user string, ask validationAsk) *validationResult {
	declared, err := config.PublishDeclaredSchemas(config.ConfigSource{Text: input, Path: path})
	if err != nil {
		// The schema does not build: runValidation reports that itself.
		return runValidation(input, path)
	}
	if !asksRunningDaemon(declared) {
		return runValidation(input, path)
	}
	result, err := askDaemonValidation(input, path, user, ask)
	if err == nil {
		return result
	}
	unanswered := unansweredOf(err, path)
	return runValidationUnanswered(input, path, &unanswered)
}

// asksRunningDaemon reports whether the verdict of a config needs the running
// ze: the config declares an external plugin this process holds no schema of.
//
// declared.Offline is the guard against a loop: it is false in a process that
// holds a daemon's module set, and a daemon is never asked from inside one.
// The daemon serves both commands through runValidation, which asks nobody.
func asksRunningDaemon(declared *config.DeclaredSchemas) bool {
	if !declared.Offline() {
		return false
	}
	return len(declared.WithoutSchema()) > 0
}

// askDaemonValidation sends input to the running ze and answers its verdict,
// named for path as the local verdict is.
func askDaemonValidation(input, path, user string, ask validationAsk) (*validationResult, error) {
	line, err := validationLine(input, path)
	if err != nil {
		return nil, err
	}
	verdict, err := ask(user, line)
	if err != nil {
		return nil, err
	}
	return decodeDaemonValidation(verdict, path, askedCommand(line))
}

// validationLine is the line that asks the running ze about input.
//
// The content form comes first: `show config supplied validation content
// <base64> name <label>`, judged by the model as the daemon's validator
// judges it, so a config over the bound one SSH packet carries is refused
// here, before anything connects. The label is left out when the daemon would
// not take it (sentLabel, and the model's own bound on name), and the daemon
// then labels the config itself. A config over the bound goes by path to
// `show config validation` when the daemon reads that path itself
// (command.CallerPath on the remote route, the judgment the daemon makes). A
// config that is too large and lies elsewhere is not sent, and the failure
// names the directories the daemon reads from.
func validationLine(input, path string) (string, error) {
	encoded := base64.StdEncoding.EncodeToString([]byte(input))
	if label, labeled := sentLabel(path); labeled {
		words := []string{suppliedContent, encoded, suppliedNameArg, label}
		if _, err := command.ValidateModelArgs(pathSuppliedValidation, words, nil); err == nil {
			var line textbuf.Buffer
			line.Str(pathSuppliedValidation).Byte(' ').Str(suppliedContent).Byte(' ').Str(encoded).
				Byte(' ').Str(suppliedNameArg).Byte(' ').Str(configtext.QuoteCommandWord(label))
			return line.String(), nil
		}
	}
	// With no label, the model judges the content alone, so its refusal is the
	// content's.
	_, modelErr := command.ValidateModelArgs(pathSuppliedValidation, []string{suppliedContent, encoded}, nil)
	if modelErr == nil {
		var line textbuf.Buffer
		line.Str(pathSuppliedValidation).Byte(' ').Str(suppliedContent).Byte(' ').Str(encoded)
		return line.String(), nil
	}
	if !namesConfigFile(path) {
		return "", notSent(path, modelErr)
	}
	resolved, err := command.CallerPath(command.RouteRemote, pathShowConfigValidation, []string{path}, path)
	if err != nil {
		return "", notSent(path, modelErr)
	}
	var line textbuf.Buffer
	line.Str(pathShowConfigValidation).Byte(' ').Str(leafFile).Byte(' ').Str(configtext.QuoteCommandWord(resolved))
	return line.String(), nil
}

// askedCommand names the daemon command line asks, for a message about its
// answer. The line itself is never named whole, because it holds the config.
func askedCommand(line string) string {
	if strings.HasPrefix(line, pathSuppliedValidation+" ") {
		return pathSuppliedValidation
	}
	return pathShowConfigValidation
}

// namesConfigFile reports whether path names a file, as opposed to standard
// input.
func namesConfigFile(path string) bool {
	if path == "" {
		return false
	}
	return path != "-"
}

// sentLabel answers the label the daemon is given for the config at path: the
// file's base name, since the daemon's `name` leaf takes a filename with no
// directory and no control character (suppliedHandler, which judges it with
// storage.PlainConfigName). labeled is false for standard input, for a base
// name the daemon would refuse, and for one that starts with a dash, which
// never reaches a daemon handler (the dispatcher reads it as a flag). The
// daemon reads nothing from the label: it names the config in the daemon's
// own messages, and the verdict drawn here is named for path.
func sentLabel(path string) (label string, labeled bool) {
	if !namesConfigFile(path) {
		return "", false
	}
	base := filepath.Base(path)
	if strings.HasPrefix(base, "-") {
		return "", false
	}
	if !storage.PlainConfigName(base) {
		return "", false
	}
	for _, r := range base {
		if unicode.IsControl(r) {
			return "", false
		}
	}
	return base, true
}

// notSent is the failure of a config that does not travel in one command and
// that the running ze does not read by path: why, and the command that checks
// a copy placed where the daemon reads.
func notSent(path string, modelErr error) error {
	dirs := daemonConfigDirs()
	var why textbuf.Buffer
	why.Str("it was not asked, because ").Str(configSubject(path)).
		Str(" does not travel in one command (").Err(modelErr).Str(")")
	if len(dirs) == 0 {
		why.Str(", and no config directory a running ze reads a file from is set on this host")
		return &askFailure{unanswered: config.DaemonUnanswered{Why: why.String(), Fix: fixDaemonStatus, Rerun: true}}
	}
	why.Str(", and a running ze reads a config file itself only from its config directories (").
		Str(strings.Join(dirs, ", ")).Str("). Copy the config into ").Str(dirs[0]).Str(" first")
	base := "<config-file>"
	if namesConfigFile(path) {
		base = filepath.Base(path)
	}
	return &askFailure{unanswered: config.DaemonUnanswered{
		Why: why.String(),
		Fix: fixValidateFile + filepath.Join(dirs[0], base),
	}}
}

// daemonConfigDirs answers the directories the daemon route reads a caller's
// file from, made absolute, from the registration CallerPath reads
// (callerPathDirs).
func daemonConfigDirs() []string {
	var dirs []string
	for _, dir := range callerPathDirs() {
		if dir == "" {
			continue
		}
		absolute, err := filepath.Abs(dir)
		if err != nil {
			continue
		}
		dirs = append(dirs, absolute)
	}
	return dirs
}

// configSubject names the config at path in a sentence.
func configSubject(path string) string {
	if !namesConfigFile(path) {
		return "the config"
	}
	return "config " + path
}

// askValidationOverSSH is the production validationAsk. It takes the walk the
// local `ze doctor` takes (sshclient.ReachDaemon), then asks over SSH for the
// dispatcher's JSON (`| raw`), whatever display format the operator set.
//
// The daemon answers a config it judges invalid with a failed status and the
// verdict document beside it, so the command's error alone decides nothing:
// the document does, and decodeDaemonValidation reads the two together.
func askValidationOverSSH(user, line string) (daemonVerdict, error) {
	reached, err := sshclient.ReachDaemon(user)
	if err != nil {
		return daemonVerdict{}, err
	}
	var body bytes.Buffer
	answer, err := sshclient.ExecCommandStream(reached.Credentials, sshclient.RawCommand(line), &body)
	if err == nil {
		return daemonVerdict{body: body.Bytes(), address: reached.Address}, nil
	}
	if answer.Verdict == rpc.VerdictError {
		return daemonVerdict{body: body.Bytes(), refused: true, address: reached.Address}, nil
	}
	if answer.Verdict == rpc.VerdictAborted {
		return daemonVerdict{body: body.Bytes(), refused: true, address: reached.Address}, nil
	}
	return daemonVerdict{}, notAnswered(reached.Address, askedCommand(line), err)
}

// notAnswered is the failure of an ask the daemon at address did not answer:
// a refused login, a connection that ended, an answer cut short. asked is the
// command, without its arguments.
func notAnswered(address, asked string, cause error) error {
	var why textbuf.Buffer
	why.Str("the ze at ").Str(address).Str(" did not answer ").Str(asked).
		Str(" (").Err(cause).Str(")")
	return &askFailure{unanswered: config.DaemonUnanswered{Why: why.String(), Fix: fixValidateAsAccount}}
}

// decodeDaemonValidation reads what the daemon sent back for the command
// asked as a verdict named for path. Anything that is not a validation result
// this ze reads (diagnostic.DecodeValidateResult) is no answer, and neither is
// a document that disagrees with the status beside it (a valid verdict under a
// failed status, an invalid one under none): drawing it would print a verdict
// the daemon never gave. A daemon that refused the command itself (a denied
// account, an argument it does not know, a daemon that predates the command)
// sent no document, so it lands here too and is no answer.
func decodeDaemonValidation(verdict daemonVerdict, path, asked string) (*validationResult, error) {
	document, err := diagnostic.DecodeValidateResult(verdict.body)
	if err != nil {
		return nil, foreignAnswer(verdict.address, asked, err.Error())
	}
	if document.Valid == verdict.refused {
		return nil, foreignAnswer(verdict.address, asked, "a verdict that disagrees with the status beside it")
	}
	result := &validationResult{
		Valid:       document.Valid,
		Path:        path,
		Diagnostics: document.Diagnostics,
	}
	// A verdict of a config with no bgp block carries no summary, and the
	// renderer draws none.
	if document.Config == nil {
		return result, nil
	}
	summary, err := daemonSummary(document.Config)
	if err != nil {
		return nil, foreignAnswer(verdict.address, asked, "a config summary this ze does not read ("+err.Error()+")")
	}
	result.Config = summary
	return result, nil
}

// daemonSummary reads the summary of a daemon's verdict, which arrives as
// decoded JSON, into the type the local renderer draws.
func daemonSummary(decoded any) (*validationSummary, error) {
	encoded, err := json.Marshal(decoded)
	if err != nil {
		return nil, err
	}
	var summary validationSummary
	if err := json.Unmarshal(encoded, &summary); err != nil {
		return nil, err
	}
	return &summary, nil
}

// foreignAnswer is the failure of an ask the daemon answered with what, for
// the command asked.
func foreignAnswer(address, asked, what string) error {
	var why textbuf.Buffer
	why.Str("the ze at ").Str(address).Str(" answered ").Str(asked).Str(" with ").Str(what)
	return &askFailure{unanswered: config.DaemonUnanswered{Why: why.String(), Fix: fixValidateAsAccount}}
}

// unansweredOf words err, the failure of an ask about the config at path, for
// the operator: why no running ze answered and the one command that gets an
// answer. A fix that is the validate command is completed with the file. An
// error that is neither an askFailure nor an *sshclient.Unreached (a stub's)
// keeps its text, with the account fix.
func unansweredOf(err error, path string) config.DaemonUnanswered {
	unanswered := unreachedOrAsked(err)
	if unanswered.Fix != fixValidateAsAccount {
		return unanswered
	}
	file := "<config-file>"
	if namesConfigFile(path) {
		file = path
	}
	unanswered.Fix = fixValidateAsAccount + file
	return unanswered
}

// unreachedOrAsked reads err as the failure of the ask or of the walk to the
// daemon, each step of the walk with its own fix.
func unreachedOrAsked(err error) config.DaemonUnanswered {
	if failure, ok := errors.AsType[*askFailure](err); ok {
		return failure.unanswered
	}
	unreached, ok := errors.AsType[*sshclient.Unreached](err)
	if !ok {
		return config.DaemonUnanswered{Why: err.Error(), Fix: fixValidateAsAccount}
	}
	var why textbuf.Buffer
	switch unreached.Step {
	case sshclient.ReachStepAddress:
		why.Str("the address of ze's SSH listener could not be read (").Err(unreached.Cause).Str(")")
		return config.DaemonUnanswered{Why: why.String(), Fix: fixDaemonStatus, Rerun: true}
	case sshclient.ReachStepListener:
		why.Str("no ze listens at ").Str(unreached.Address).Str(" (").Err(unreached.Cause).Str(")")
		return config.DaemonUnanswered{Why: why.String(), Fix: fixStartDaemon, Rerun: true}
	case sshclient.ReachStepCredentials:
		why.Str("a ze listens at ").Str(unreached.Address).
			Str(", but this account has no credentials to log in to it (").Err(unreached.Cause).Str(")")
		return config.DaemonUnanswered{Why: why.String(), Fix: fixValidateAsAccount}
	case sshclient.ReachStepUnspecified:
		panic("BUG: sshclient.ReachDaemon answered a failure that names no step")
	default:
		panic("BUG: unknown sshclient.ReachStep")
	}
}
