// Design: docs/architecture/config/yang-config-design.md — a local config command asks the running ze first
// Detail: validate_route.go — `ze config validate`, which draws the daemon's verdict document
// Detail: inspect_route.go — `ze config fmt`, `dump`, `diff` and `graph`, which draw the daemon's answer with their own renderer
// Related: config_supplied.go — the daemon commands this file sends
// Related: ../../doctor/local_route.go — the local `ze doctor`, which takes the same walk to the daemon
//
// A config that declares an external plugin can hold a section only that
// plugin's schema defines, and ze reads that schema from the plugin when
// `ze start` runs it. A command with no daemon starts no plugin (owner
// ruling), so it cannot judge such a section, and a running ze can. So a local
// config command asks the running ze first for such a config: it sends the
// bytes it read in the `content` leaf of the command's `show config supplied`
// form, since the daemon reads a file a caller names only from its own config
// directories (command.CallerPath), and draws the daemon's answer with its own
// renderer and exit codes. When no daemon answers it judges the config in its
// own process, and the refusal of an unknown name says why no daemon answered
// and the one command that gets an answer. A failed ask is never read as an
// answer.
//
// A config that declares no such plugin is judged in this process and no
// daemon is asked: every schema it needs is compiled into this binary, so the
// answer is complete here.

package cli

import (
	"bytes"
	"encoding/base64"
	"errors"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/ze-software/ze/internal/component/cli/sshclient"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/configtext"
	"github.com/ze-software/ze/internal/core/helpfmt"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// The account option of this route. The fix line of each step of the walk to
// the daemon is sshclient's (Unreached.Explain), shared with the shell route of
// a path this binary does not hold.
const (
	// accountOptionUsage is the option that names the account a command asks
	// the running ze from, as a help page and a fix line write it. Every
	// command of this route declares it.
	accountOptionUsage = "--user <name>"
	// accountOption is accountOptionUsage between the command and its
	// arguments.
	accountOption = " " + accountOptionUsage + " "
)

// accountOptionHelp is the entry of the account option in the help page of
// every command of this route.
func accountOptionHelp() helpfmt.HelpEntry {
	return helpfmt.HelpEntry{
		Name: accountOptionUsage,
		Desc: "Account that asks the running ze, for a config that declares an external plugin",
	}
}

// daemonVerdict is what the running ze sent back for one line.
type daemonVerdict struct {
	// body is the dispatcher's JSON for the command.
	body []byte
	// refused is true when the daemon stated a failed status beside body,
	// which is how it answers a config it refuses.
	refused bool
	// refusal is the text the daemon gave with a failed status, without the
	// "error: " a surface puts in front of it.
	refusal string
	// warnings is the text a command that completed wrote for the operator
	// beside its answer, one line each (the parse warnings of a dump).
	warnings string
	// address is the daemon's listener, for a message about its answer.
	address string
}

// daemonAsk sends one line to the running ze, logging in as user ("" is the
// stored account, read with no prompt), and answers what it sent back. A
// failure is an *askFailure or an *sshclient.Unreached. askOverSSH is the
// production ask; a test hands a stub.
type daemonAsk func(user, line string) (daemonVerdict, error)

// routeAsk is the ask the registered handlers of fmt, dump, diff and graph
// send through: a handler is registered once, at init, and takes no ask of its
// own. It is askOverSSH in a ze binary. A test replaces it before it calls a
// handler, so no unit test opens a connection; it is never written while a
// handler runs.
var routeAsk daemonAsk = askOverSSH

// askFailure is why no running ze answered, as an error.
type askFailure struct {
	unanswered config.DaemonUnanswered
	// account is true when the fix is the asking command itself, run from an
	// account that can log in; suppliedQuestion.unanswered writes that command.
	account bool
}

func (e *askFailure) Error() string { return e.unanswered.Why }

// suppliedQuestion is what one local config command asks the running ze: the
// bytes it read, the daemon command that takes them, and how the operator
// spells the command, for the messages of a failed ask.
type suppliedQuestion struct {
	// supplied is the `show config supplied` command that takes the bytes.
	supplied string
	// content is the bytes its content leaf carries.
	content []byte
	// labelPath is the file content was read from, whose base name labels it
	// in the daemon's own messages. It is "" for a command whose supplied form
	// takes no label.
	labelPath string
	// flags are the flag leaves sent after the content, as the model spells
	// them.
	flags []string
	// byPath is the daemon command that reads files itself, sent when content
	// does not travel in one command. It is "" when the command has none for
	// the inputs it was given.
	byPath string
	// files are the files byPath reads, as the operator typed them.
	files []string
	// fileLeaf is true when byPath takes its file in the `file` leaf, and
	// false when it takes its files as plain words.
	fileLeaf bool
	// local is the command the operator runs, without its arguments.
	local string
	// operands are the arguments the operator gave local, as typed.
	operands string
	// subject names what is asked about in a sentence.
	subject string
	// placeholders name the files in a fix line when files is empty.
	placeholders []string
}

// needsDaemon reports whether the answer about sources should ask a running ze
// first: they declare an external endpoint while this process has not yet fixed
// its module set. It starts no external program and fixes no module set, so a
// running daemon is always asked before offline query mode runs (AC-18).
func (q suppliedQuestion) needsDaemon(sources []config.ConfigSource) bool {
	return config.HasUnresolvedExternalPlugins(sources...)
}

// line is the line that asks the running ze.
//
// The content form comes first: `<supplied> content <base64> name <label>
// <flags>`, judged by the model as the daemon's validator judges it, so
// content over the bound one SSH packet carries is refused here, before
// anything connects. The label is left out when the daemon would not take it
// (sentLabel, and the model's own bound on name), and the daemon then labels
// the config itself. Content over the bound goes by path to byPath when the
// daemon reads every file itself (command.CallerPath on the remote route, the
// judgment the daemon makes). Content that is too large and lies elsewhere is
// not sent, and the failure names the directories the daemon reads from.
func (q suppliedQuestion) line() (string, error) {
	encoded := base64.StdEncoding.EncodeToString(q.content)
	if label, labeled := sentLabel(q.labelPath); labeled {
		words := append([]string{suppliedContent, encoded, suppliedNameArg, label}, q.flags...)
		if _, err := command.ValidateModelArgs(q.supplied, words, nil); err == nil {
			var line textbuf.Buffer
			line.Str(q.supplied).Byte(' ').Str(suppliedContent).Byte(' ').Str(encoded).
				Byte(' ').Str(suppliedNameArg).Byte(' ').Str(configtext.QuoteCommandWord(label))
			return q.withFlags(&line), nil
		}
	}
	// With no label, the model judges the content alone, so its refusal is the
	// content's.
	words := append([]string{suppliedContent, encoded}, q.flags...)
	_, modelErr := command.ValidateModelArgs(q.supplied, words, nil)
	if modelErr == nil {
		var line textbuf.Buffer
		line.Str(q.supplied).Byte(' ').Str(suppliedContent).Byte(' ').Str(encoded)
		return q.withFlags(&line), nil
	}
	if q.byPath == "" {
		return "", q.notSent(modelErr)
	}
	if len(q.files) == 0 {
		return "", q.notSent(modelErr)
	}
	var line textbuf.Buffer
	line.Str(q.byPath)
	for _, file := range q.files {
		resolved, err := command.CallerPath(command.RouteRemote, q.byPath, q.files, file)
		if err != nil {
			return "", q.notSent(modelErr)
		}
		if q.fileLeaf {
			line.Byte(' ').Str(leafFile)
		}
		line.Byte(' ').Str(configtext.QuoteCommandWord(resolved))
	}
	return q.withFlags(&line), nil
}

// withFlags ends line with the flag leaves of the question.
func (q suppliedQuestion) withFlags(line *textbuf.Buffer) string {
	for _, flag := range q.flags {
		line.Byte(' ').Str(flag)
	}
	return line.String()
}

// asked names the daemon command line asks, for a message about its answer.
// The line itself is never named whole, because it holds the config.
func (q suppliedQuestion) asked(line string) string {
	if strings.HasPrefix(line, q.supplied+" ") {
		return q.supplied
	}
	return q.byPath
}

// typed is the command the operator ran, as they typed it.
func (q suppliedQuestion) typed() string {
	return q.local + " " + q.operands
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
// own messages, and the answer drawn here is named for path.
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

// notSent is the failure of content that does not travel in one command and
// that the running ze does not read by path: why, with the bound the model
// states, and the command that works on a copy placed where the daemon reads.
//
// The failure is one part of the refusal the operator reads, whose one fix is
// the failure's own, so it holds the bound alone (sendBound) and never the
// model's refusal whole: that refusal ends with a fix line of its own, for a
// daemon command the operator did not type.
func (q suppliedQuestion) notSent(modelErr error) error {
	dirs := daemonConfigDirs()
	var why textbuf.Buffer
	why.Str("it was not asked, because ").Str(q.subject).
		Str(" does not travel in one command (").Str(sendBound(modelErr)).Str(")")
	if len(dirs) == 0 {
		why.Str(", and no config directory a running ze reads a file from is set on this host")
		return &askFailure{unanswered: config.DaemonUnanswered{
			Why: why.String(), Fix: sshclient.FixDaemonStatus, Rerun: true, Again: q.typed(),
		}}
	}
	names := q.placeholders
	if len(q.files) > 0 {
		names = make([]string, 0, len(q.files))
		for _, file := range q.files {
			names = append(names, filepath.Base(file))
		}
	}
	copied := "the config"
	if len(names) > 1 {
		copied = "the configs"
	}
	why.Str(", and a running ze reads a config file itself only from its config directories (").
		Str(strings.Join(dirs, ", ")).Str("). Copy ").Str(copied).Str(" into ").Str(dirs[0]).Str(" first")
	var fix textbuf.Buffer
	fix.Str(q.local)
	for _, name := range names {
		fix.Byte(' ').Str(filepath.Join(dirs[0], name))
	}
	return &askFailure{unanswered: config.DaemonUnanswered{Why: why.String(), Fix: fix.String()}}
}

// sendBound says, in one sentence with no line break, the bound modelErr
// refused the content of a question for. modelErr is the refusal of
// command.ValidateModelArgs.
//
// A value the model refuses states its bound in the reason of its fault (the
// error-message of the content leaf in ze-config-cli-cmd.yang), whose first
// sentence is the bound and whose next advises the local command, which the
// operator already ran. Any other refusal is named by its first line, the
// problem: the lines after it are the reason and the fix of a daemon command.
func sendBound(modelErr error) string {
	fault, refusesValue := errors.AsType[*command.ValueArgumentError](modelErr)
	if !refusesValue {
		problem, _, _ := strings.Cut(modelErr.Error(), "\n")
		return problem
	}
	reason, _, _ := strings.Cut(fault.Why(), "\n")
	bound, _, _ := strings.Cut(reason, ". ")
	return strings.TrimSuffix(bound, ".")
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

// askOverSSH is the production daemonAsk. It takes the walk the local
// `ze doctor` takes (sshclient.ReachDaemon), then asks over SSH for the
// dispatcher's JSON (`| raw`), whatever display format the operator set.
//
// The daemon answers a config it refuses with a failed status, and a
// validation with the verdict document beside it, so the command's error alone
// decides nothing: the caller reads the status, the text and the document
// together.
func askOverSSH(user, line string) (daemonVerdict, error) {
	reached, err := sshclient.ReachDaemon(user)
	if err != nil {
		return daemonVerdict{}, err
	}
	var body bytes.Buffer
	answer, err := sshclient.ExecCommandStream(reached.Credentials, sshclient.RawCommand(line), &body)
	if err == nil {
		return daemonVerdict{body: body.Bytes(), warnings: answer.Warnings, address: reached.Address}, nil
	}
	if answer.Verdict == rpc.VerdictError {
		return daemonVerdict{body: body.Bytes(), refused: true, refusal: err.Error(), address: reached.Address}, nil
	}
	if answer.Verdict == rpc.VerdictAborted {
		return daemonVerdict{body: body.Bytes(), refused: true, refusal: err.Error(), address: reached.Address}, nil
	}
	return daemonVerdict{}, notAnswered(reached.Address, askedCommand(line), err)
}

// askedCommand names the daemon command a line asks, without its arguments:
// the words before the content leaf of a supplied form, and the three words
// of a command that reads a file itself (`show config <subject>`).
func askedCommand(line string) string {
	if asked, _, supplied := strings.Cut(line, " "+suppliedContent+" "); supplied {
		return asked
	}
	words := strings.Fields(line)
	if len(words) > 3 {
		words = words[:3]
	}
	return strings.Join(words, " ")
}

// notAnswered is the failure of an ask the daemon at address did not answer:
// a refused login, a connection that ended, an answer cut short. asked is the
// command, without its arguments.
func notAnswered(address, asked string, cause error) error {
	var why textbuf.Buffer
	why.Str("the ze at ").Str(address).Str(" did not answer ").Str(asked).
		Str(" (").Err(cause).Str(")")
	return &askFailure{unanswered: config.DaemonUnanswered{Why: why.String()}, account: true}
}

// foreignAnswer is the failure of an ask the daemon answered with what, for
// the command asked.
func foreignAnswer(address, asked, what string) error {
	var why textbuf.Buffer
	why.Str("the ze at ").Str(address).Str(" answered ").Str(asked).Str(" with ").Str(what)
	return &askFailure{unanswered: config.DaemonUnanswered{Why: why.String()}, account: true}
}

// unanswered words err, the failure of the ask, for the operator: why no
// running ze answered and the one command that gets an answer. A fix that is
// the asking command is written as the operator typed it, with the account
// option. An error that is neither an askFailure nor an *sshclient.Unreached
// (a stub's) keeps its text, with the account fix.
func (q suppliedQuestion) unanswered(err error) config.DaemonUnanswered {
	unanswered, account := unreachedOrAsked(err)
	if account {
		unanswered.Fix = q.local + accountOption + q.operands
	}
	if unanswered.Rerun {
		unanswered.Again = q.typed()
	}
	return unanswered
}

// unreachedOrAsked reads err as the failure of the ask or of the walk to the
// daemon, each step of the walk with its own fix. account is true when the
// fix is the asking command run from an account that can log in, which the
// caller writes.
func unreachedOrAsked(err error) (unanswered config.DaemonUnanswered, account bool) {
	if failure, ok := errors.AsType[*askFailure](err); ok {
		return failure.unanswered, failure.account
	}
	unreached, ok := errors.AsType[*sshclient.Unreached](err)
	if !ok {
		return config.DaemonUnanswered{Why: err.Error()}, true
	}
	why, fix := unreached.Explain()
	if fix == "" {
		// A ze listens and this account cannot log in to it: the fix is the
		// asking command from another account, which the caller writes.
		return config.DaemonUnanswered{Why: why}, true
	}
	return config.DaemonUnanswered{Why: why, Fix: fix, Rerun: true}, false
}
