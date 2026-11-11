// Design: docs/architecture/core-design.md -- native post-command hook policy
// Related: postwrite.go -- postFormatGo, the format and lint step a Write or an Edit gets

package hookruntime

import (
	"bytes"
	stdcontext "context"
	"errors"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"mvdan.cc/sh/v3/syntax"

	"github.com/ze-software/ze/internal/le/gitenv"
)

// postBashWindow is how recently a Go file must have changed for the check
// after a Bash call to judge it. The hook payload carries no start time for the
// command, so the window stands in for "changed by this command": a write
// earlier than the window in a longer command is missed, and another session's
// write inside it is reported too, which the message says.
const postBashWindow = 2 * time.Minute

// postBashBudget bounds the git status and the gofmt run together. The hook's
// own timeout in .claude/settings.json is 10s.
const postBashBudget = 5 * time.Second

// ze point: quality/linting/fix-lint-issues-never-disable-a-linter
// postBashGofmt names the Go files a shell command just changed that gofmt
// would reshape. A Write or an Edit is formatted by postFormatGo; a sed, a
// heredoc or a scripted rename reaches the tree without it, which is how a
// misformatted line survived a rename batch until a scoped lint found it. It
// reports and never rewrites: the window cannot tell this session's write from
// another's, and another session's file is not this one's to change.
//
// Only a file the command names, by its path or by a directory holding it,
// blocks. The window alone blocked every session for whichever file another
// session left unformatted, and put that file in each one's context after
// every command. A file the command does not name is said at code 0, which
// the person watching sees and the agent does not.
//
// What a command names is read from its parse as bash (commandNamedPaths),
// never searched for in its text: the word `internal` inside a grep pattern
// named every file under internal/. A command that does not parse names
// nothing, because the check cannot answer for it, and the code-0 line then
// says the command was not read instead of claiming it names no file.
func postBashGofmt(ctx context) *verdict {
	timeout, cancel := stdcontext.WithTimeout(stdcontext.Background(), postBashBudget)
	defer cancel()

	changed, err := recentGoFiles(timeout, ctx.root, time.Now())
	if err != nil {
		return gofmtNotRun("the changed Go files could not be listed: " + err.Error())
	}
	if len(changed) == 0 {
		return nil
	}
	binary, err := exec.LookPath("gofmt")
	if err != nil {
		return gofmtNotRun("gofmt is not on PATH")
	}
	command := exec.CommandContext(timeout, binary, append([]string{"-l", "--"}, changed...)...) //nolint:gosec // gofmt from PATH, -l only, over paths git status listed
	command.Dir = ctx.root
	var stderr bytes.Buffer
	command.Stderr = &stderr
	output, err := command.Output()
	parse := ctx.commandParse()
	var namedPaths []namedPath
	if parse.parsed() {
		namedPaths = commandNamedPaths(parse.file, ctx.root)
	}
	named := make([]string, 0, 4)
	unnamed := 0
	for file := range strings.FieldsSeq(string(output)) {
		if commandNames(namedPaths, file) {
			named = append(named, file)
			continue
		}
		unnamed++
	}
	lines := make([]string, 0, len(named)+4)
	if len(named) != 0 {
		lines = append(lines, yellow+"⚠ gofmt: "+strconv.Itoa(len(named))+" Go file(s) this command names are not formatted"+reset)
		for _, file := range named {
			lines = append(lines, "  "+file)
		}
		lines = append(lines,
			"  The check cannot tell whose write it was, only that the file changed in the last "+postBashWindow.String()+".",
			"  If this command changed the file: gofmt -w <path>",
			"  If another session is editing it: leave the file as it is, and say in your report whose it is.")
	}
	if err != nil {
		lines = append(lines, yellow+"⚠ gofmt could not read every changed file, so those were NOT checked: "+err.Error()+reset)
		for line := range strings.SplitSeq(stderr.String(), "\n") {
			if line != "" && len(lines) < len(named)+6 {
				lines = append(lines, "  "+dim+line+reset)
			}
		}
	}
	if len(lines) != 0 {
		return &verdict{2, strings.Join(lines, "\n")}
	}
	if unnamed == 0 {
		return nil
	}
	count := dim + "gofmt: " + strconv.Itoa(unnamed) + " unformatted Go file(s) changed in the last " + postBashWindow.String()
	if !parse.parsed() {
		return &verdict{0, count + "; this command could not be read as bash (" + parse.failure + "), so whether it names one is not known. Check the Go files it changed with: gofmt -l <path>" + reset}
	}
	return &verdict{0, count + " that this command does not name" + reset}
}

// namedPath is one word of a command read as a path under the checkout.
type namedPath struct {
	// text is the path, slash-separated, cleaned and relative to the checkout.
	text string
	// pattern reports that bash expands the word, so text is matched as a
	// glob instead of compared.
	pattern bool
}

// matches reports whether the word is candidate, or a glob that matches it.
func (named namedPath) matches(candidate string) bool {
	if !named.pattern {
		return named.text == candidate
	}
	matched, err := path.Match(named.text, candidate)
	if err != nil {
		// Bash leaves a pattern it cannot read as the literal word.
		return named.text == candidate
	}
	return matched
}

// commandNames answers whether one of a command's named paths is file, a slash
// path relative to the checkout, or one of its parent directories. The
// comparison is of whole paths, so `internal/le/archive` does not name a file
// under `internal/le/arch`. The checkout itself is never a candidate: every
// command runs in it, so it would name every file.
func commandNames(named []namedPath, file string) bool {
	for candidate := file; candidate != "." && candidate != "/"; candidate = path.Dir(candidate) {
		for _, word := range named {
			if word.matches(candidate) {
				return true
			}
		}
	}
	return false
}

// commandNamedPaths answers the paths a parsed command names: every word of
// the line, wherever it stands (an argument, a for loop's item, an
// assignment's value, a redirection's target), read by wordPath.
//
// A read-only line names nothing: it cannot have changed a file. A line is
// read-only when every command on it, a nested substitution's included, is a
// reader (readerCommand) and no redirection may write a Go file of the
// checkout (redirectWritesGo). A reader under an assignment (`LC_ALL=C grep`)
// is not taken for one (readerCall). The exemption is all or nothing: on any
// other line every word names, a reader's too, because `cd pkg && sed -i ...
// x.go` and `cd pkg && cat > x.go <<EOF` name their file only through the
// word of cd.
//
// What it does not see, each said at code 0 like any file the command does
// not name:
//   - a word bash expands other than as a plain glob (a parameter, a command
//     substitution, arithmetic, a tilde, braces): its literal parts are not
//     the argument (wordValue.text), so it names nothing;
//   - a path inside a longer word: `dd of=x.go`, an attached short option
//     value (`-ox.go`), the script of `bash -c '...'`;
//   - a path that arrives through a variable or xargs;
//   - a relative word after a `cd` on the same line: every word is read
//     against the checkout root, so only the word of cd names;
//   - a line whose tree holds fewer commands than bash runs, as when a
//     comment ends in a backslash and the parser joins the next line into it.
//
// On a line that is not read-only, a word inside `${x:-word}`, `(( ))`,
// `[[ ]]` or a case pattern is a candidate like any other.
func commandNamedPaths(file *syntax.File, root string) []namedPath {
	var words []namedPath
	readOnly := true
	syntax.Walk(file, func(node syntax.Node) bool {
		switch node := node.(type) {
		case *syntax.CallExpr:
			if !readerCall(node) {
				readOnly = false
			}
		case *syntax.Redirect:
			if redirectWritesGo(node, root) {
				readOnly = false
			}
		case *syntax.Word:
			if named, naming := wordPath(node, root); naming == wordNamesPath {
				words = append(words, named)
			}
		}
		return true
	})
	if readOnly {
		return nil
	}
	return words
}

// redirectWritesGo reports whether a redirection may write a Go file of the
// checkout, the only way a line of readers changes one. A target outside the
// checkout (`2>/dev/null`), a descriptor (`2>&1`, `>&-`) and a known target
// that is not Go source (`> tmp/out.txt`) write none. A target the hook
// cannot know (`> "$out"`, `> ../x.go`) may be one, and so may a glob, so
// each counts.
func redirectWritesGo(redirect *syntax.Redirect, root string) bool {
	if !redirectWrites(redirect.Op) {
		return false
	}
	if redirect.Word == nil {
		return false
	}
	target, naming := wordPath(redirect.Word, root)
	if naming != wordNamesPath {
		return naming == wordNotKnown
	}
	if target.pattern {
		return true
	}
	return strings.HasSuffix(target.text, ".go")
}

// readerCall reports whether one command of the line is a reader. A command
// that is only assignments runs nothing. One that carries an assignment before
// its name is no reader here: the variable can change what the program does.
func readerCall(call *syntax.CallExpr) bool {
	if len(call.Args) == 0 {
		return true
	}
	if len(call.Assigns) != 0 {
		return false
	}
	words := make([]string, len(call.Args))
	expands := make([]bool, len(call.Args))
	for index, argument := range call.Args {
		value, err := readWord(argument)
		// refuse-by-default: guards commandNamedPaths -- a command with a word the hook cannot read is no reader, so every word of the line names
		if err != nil {
			return false
		}
		words[index] = value.text
		expands[index] = value.expands
	}
	return readerCommand(words, expands)
}

// redirectWrites reports whether a redirection can write the file its word
// names.
func redirectWrites(operator syntax.RedirOperator) bool {
	// Only the operators that read are listed, so one this code does not know
	// is taken to write.
	//exhaustive:ignore
	switch operator {
	case syntax.RdrIn, syntax.DplIn, syntax.Hdoc, syntax.DashHdoc, syntax.WordHdoc:
		return false
	}
	return true
}

// wordNaming is what one word of a command says about a path of the checkout.
type wordNaming uint8

const (
	// The zero value is no answer: wordPath never returns it.
	_ wordNaming = iota
	// wordNamesPath: the word is a path under the checkout.
	wordNamesPath
	// wordNamesNothing: the word is known, and is no path under the checkout.
	wordNamesNothing
	// wordNotKnown: the hook cannot tell what bash passes for the word.
	wordNotKnown
)

// wordPath reads one word as a path under the checkout.
//
// A word starting with `-` that holds `=` offers what follows the first `=`,
// and one without `=` names nothing. `./`, a trailing slash and an absolute
// path under root are spelling, so the word is cleaned and made relative. Go's
// `dir/...` names dir. The checkout itself (`.`, `./...`, its absolute path)
// and an absolute path outside it name nothing. An absolute path is compared
// as spelled, so one that reaches the checkout through a symlink reads as
// outside.
//
// Three kinds of word are not known. A word readWord cannot give a value for.
// A word bash expands other than as a plain glob (globOnly): wordValue.text
// then holds only the word's literal parts, which nothing may take for the
// argument. A relative word that climbs with `..`: it is outside the checkout
// only when the shell stands at its root, and an earlier cd may have moved it.
func wordPath(word *syntax.Word, root string) (namedPath, wordNaming) {
	value, err := readWord(word)
	if err != nil {
		return namedPath{}, wordNotKnown
	}
	if value.expands && !globOnly(word) {
		return namedPath{}, wordNotKnown
	}
	text := value.text
	if strings.HasPrefix(text, "-") {
		_, optionValue, found := strings.Cut(text, "=")
		if !found {
			return namedPath{}, wordNamesNothing
		}
		text = optionValue
	}
	absolute := filepath.IsAbs(text)
	if absolute {
		relative, relErr := filepath.Rel(root, text)
		if relErr != nil {
			return namedPath{}, wordNotKnown
		}
		text = relative
	}
	text = strings.TrimSuffix(path.Clean(filepath.ToSlash(text)), "/...")
	switch {
	case text == ".", text == "...":
		return namedPath{}, wordNamesNothing
	case text == "..", strings.HasPrefix(text, "../"):
		if absolute {
			return namedPath{}, wordNamesNothing
		}
		return namedPath{}, wordNotKnown
	}
	return namedPath{text: text, pattern: value.expands}, wordNamesPath
}

// globOnly reports whether a word bash expands is made only of unquoted
// literals with no backslash, tilde or brace, so that pathname globbing (`*`,
// `?`, `[...]`) is its only expansion and its text is the pattern bash
// matches. It is read from the word's parts: a parameter, a substitution or a
// quoted part is a part of another kind. The literals can be several, because
// the bash lexer ends a literal before a `[` inside a word
// (vendor/mvdan.cc/sh/v3/syntax/lexer.go, advanceLitNone), so `pkg/[ab].go`
// is two of them. A glob word that also carries a quote or a backslash is
// left out, and names nothing.
func globOnly(word *syntax.Word) bool {
	for _, part := range word.Parts {
		literal, isLiteral := part.(*syntax.Lit)
		if !isLiteral {
			return false
		}
		if strings.ContainsAny(literal.Value, `\~{`) {
			return false
		}
	}
	return true
}

// gofmtNotRun is the verdict for a format check that could not judge the tree.
// It blocks for the reason lintNotRun does: only a blocking post-tool verdict
// reaches the agent.
func gofmtNotRun(why string) *verdict {
	return &verdict{2, yellow + "⚠ gofmt did NOT check this command's Go edits: " + why + reset}
}

// recentGoFiles answers the uncommitted Go files under root, tracked or not,
// whose content changed within postBashWindow of now, as paths relative to
// root. vendor/ and generated source are left out: neither is written by hand,
// and postFormatGo leaves generated source as its generator wrote it too.
func recentGoFiles(timeout stdcontext.Context, root string, now time.Time) ([]string, error) {
	command := exec.CommandContext(timeout, "git", "status", "--porcelain=v1", "-z", "--untracked-files=all")
	command.Dir = root
	// The deadline SIGKILLs git, and a status killed while it refreshes the index
	// leaves an orphaned .git/index.lock that blocks every session's git writes.
	command.Env = gitenv.ReadOnly(os.Environ())
	status, err := command.Output()
	if err != nil {
		return nil, err
	}
	entries := strings.Split(string(status), "\x00")
	recent := make([]string, 0, 8)
	for index := 0; index < len(entries); index++ {
		entry := entries[index]
		if len(entry) < 4 {
			continue
		}
		// A rename or a copy carries its source path as the next entry.
		if entry[0] == 'R' || entry[0] == 'C' {
			index++
		}
		file := entry[3:]
		if !strings.HasSuffix(file, ".go") {
			continue
		}
		if strings.HasPrefix(file, "vendor/") {
			continue
		}
		full := filepath.Join(root, filepath.FromSlash(file))
		info, statErr := os.Stat(full)
		if errors.Is(statErr, os.ErrNotExist) {
			continue
		}
		if statErr != nil {
			return nil, statErr
		}
		if now.Sub(info.ModTime()) > postBashWindow || !info.Mode().IsRegular() {
			continue
		}
		body, readErr := os.ReadFile(full) //nolint:gosec // a path git status listed under the checkout
		if readErr != nil {
			return nil, readErr
		}
		if generatedMarkerRE.Match(body) {
			continue
		}
		recent = append(recent, file)
	}
	return recent, nil
}
