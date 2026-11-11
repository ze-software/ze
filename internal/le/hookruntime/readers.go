// Design: docs/architecture/core-design.md -- native Bash hook policy
// Related: session.go -- the identity check that lets a line of readers run without the identity line
// Related: shellline.go -- readerLine, the line on which a guard may lift its text refusal
package hookruntime

import "strings"

// readerJudge admits one reader command by its arguments, quotes removed,
// and by whether the shell expands each of them (shellLine.expands).
type readerJudge func(arguments []string, expands []bool) bool

// readerCommands is the declared set of commands that run nothing they are
// handed: a command here, with the arguments its judge admits, reads neither
// identity variable and starts no le, test binary, script or other program.
// Two decisions rest on it. The identity check (bashAgentIdentity) lets a
// subagent's line made only of these run without its identity line, and a
// guard lifts its text refusal only on such a line (readerLine), where a
// quoted `go build` in a grep pattern is text.
//
// A judge is an allow-list. It admits the options and argument shapes it
// names and refuses every other, so an abbreviation a program would expand to
// a dangerous long option (`git grep --open=bash`, `sort --compress=sh`) is
// refused. A judge also refuses any word the shell expands, because its value
// is not the text the judge read: after `echo --ext-diff`, `$_` is
// `--ext-diff`. A nil judge marks a command none of whose options runs a
// program, writes a file, or evaluates an argument as code, so every argument
// is data: the GNU coreutils and grep option sets of basename, cat, cut,
// dirname, head, ls, stat, tail, tr and wc, grep and its two aliases, and the
// shell builtins cd, echo, pwd and true. Such a command's arguments are not
// refused when the shell expands them, because no value an expansion yields
// turns one into a run or a write, and refusing them would put the identity
// line on every `cat "$dir/x.log"`; only its command word must be text.
// diff carries a judge because its -l pipes the output through pr.
//
// A reader's own options write no file, and the shell's redirections are
// judged beside the table: a command that redirects its output into a file
// (shellLine.writesFile) is no reader, whatever its judge says.
//
// A command absent from the table needs the identity line and keeps every
// guard's text judgment, so the table is wrong only in the safe direction.
// Configuration the user owns stays trusted: a git diff driver or textconv
// program, a ripgrep config file named by the environment, and the go tool
// gopls starts, which reads neither identity variable.
var readerCommands = map[string]readerJudge{
	// `[` is the test builtin. A `!` lies outside the syntax the hook models
	// (modeledSyntax), so a negated `[ ! -f x ]` needs the line.
	"[":         bracketReads,
	"basename":  nil,
	catCommand:  nil,
	"cd":        nil,
	"cut":       nil,
	"diff":      diffOptions.reads,
	"dirname":   nil,
	"echo":      nil,
	"egrep":     nil,
	"fgrep":     nil,
	"find":      findReads,
	"git":       gitReads,
	"gopls":     goplsReads,
	"grep":      nil,
	"head":      nil,
	"ls":        nil,
	"printf":    printfReads,
	"pwd":       nil,
	"rg":        ripgrepOptions.reads,
	"sed":       sedReads,
	"sort":      sortOptions.reads,
	"stat":      nil,
	"tail":      nil,
	testCommand: testReads,
	"tr":        nil,
	"true":      nil,
	"uniq":      uniqReads,
	"wc":        nil,
}

// readerCommand reports whether one command of a line is a reader its judge
// admits: its first word is a member of readerCommands, written as text the
// shell does not expand, and the judge, if any, admits its arguments. expands
// is parallel to words.
func readerCommand(words []string, expands []bool) bool {
	if expands[0] {
		return false
	}
	judge, reader := readerCommands[words[0]]
	if !reader {
		return false
	}
	if judge == nil {
		return true
	}
	return judge(words[1:], expands[1:])
}

// readerOptions is the allow-list of a command that parses its options with
// getopt: single letters that may be grouped in one word, and long options.
type readerOptions struct {
	// short holds the letters that take no value.
	short string
	// shortValue holds the letters whose value is the rest of the word or,
	// when the word ends at the letter, the next word, as getopt reads a
	// required argument. Only letters the program certainly reads that way
	// are listed: a letter listed by mistake would hide the next word from
	// the judge.
	shortValue string
	// long holds the long options, compared whole up to any `=`. An
	// abbreviation getopt would expand is not in the set and is refused.
	long map[string]bool
}

// reads admits a command whose every option the allow-list holds.
func (options readerOptions) reads(arguments []string, expands []bool) bool {
	_, admitted := options.operands(arguments, expands)
	return admitted
}

// operands judges every argument and counts the operands: the words that are
// neither an option nor a short option's value. `--` does not end the
// judging, because a long option given its value as the next word can take
// `--` as that value and leave the word after it an option; a later operand
// that starts with `-` is refused, in the safe direction. A long option's
// value given as the next word is judged as an operand.
func (options readerOptions) operands(arguments []string, expands []bool) (int, bool) {
	count := 0
	for index := 0; index < len(arguments); index++ {
		if expands[index] {
			return 0, false
		}
		argument := arguments[index]
		switch {
		case argument == "--":
		case argument == "-":
			count++
		case strings.HasPrefix(argument, "--"):
			name, _, _ := strings.Cut(argument, "=")
			if !options.long[name] {
				return 0, false
			}
		case strings.HasPrefix(argument, "-"):
			takesNext, admitted := options.cluster(argument[1:])
			if !admitted {
				return 0, false
			}
			if !takesNext {
				continue
			}
			index++
			if index < len(arguments) && expands[index] {
				return 0, false
			}
		default:
			count++
		}
	}
	return count, true
}

// cluster judges the letters of one short-option word, and reports whether
// its last letter takes the next word as its value.
func (options readerOptions) cluster(letters string) (takesNext, admitted bool) {
	for index, letter := range letters {
		if strings.ContainsRune(options.shortValue, letter) {
			return index == len(letters)-1, true
		}
		if !strings.ContainsRune(options.short, letter) {
			return false, false
		}
	}
	return false, true
}

// longOptions answers a set of long options.
func longOptions(names ...string) map[string]bool {
	set := make(map[string]bool, len(names))
	for _, name := range names {
		set[name] = true
	}
	return set
}

// diffOptions is GNU diff's option set without -l and --paginate, which pipe
// the output through pr.
var diffOptions = readerOptions{
	short:      "0123456789abBcdeEfhHinNpPqrstTuvwyZ",
	shortValue: "CDFILSUWxX",
	long: longOptions("--brief", "--color", "--context", "--ed", "--exclude", "--exclude-from",
		"--expand-tabs", "--from-file", "--help", "--horizon-lines", "--ifdef", "--ignore-all-space",
		"--ignore-blank-lines", "--ignore-case", "--ignore-file-name-case", "--ignore-matching-lines",
		"--ignore-space-change", "--ignore-tab-expansion", "--ignore-trailing-space", "--initial-tab",
		"--label", "--left-column", "--line-format", "--minimal", "--new-file", "--new-group-format",
		"--new-line-format", "--no-dereference", "--no-ignore-file-name-case", "--normal",
		"--old-group-format", "--old-line-format", "--palette", "--rcs", "--recursive",
		"--report-identical-files", "--show-c-function", "--show-function-line", "--side-by-side",
		"--speed-large-files", "--starting-file", "--strip-trailing-cr", "--suppress-blank-empty",
		"--suppress-common-lines", "--tabsize", "--text", "--to-file", "--unchanged-group-format",
		"--unchanged-line-format", "--changed-group-format", "--unidirectional-new-file", "--unified",
		"--version", "--width"),
}

// ripgrepOptions is ripgrep's option set without --pre, --pre-glob, -z and
// --search-zip, which run a program over each file, and --hostname-bin and
// --hyperlink-format, which name one. No letter is a value letter: a value
// given as the next word is judged as an operand, and a value attached to its
// letter (`-g*.go`, `-m5`) is judged as more option letters, which refuses
// it wherever a letter of the value is not in the set, in the safe
// direction.
var ripgrepOptions = readerOptions{
	short: "0123456789abcdefghijlmnopqrstuvwxABCEFHILMNPSTUV",
	long: longOptions("--after-context", "--before-context", "--binary", "--byte-offset",
		"--case-sensitive", "--color", "--colors", "--column", "--context", "--count",
		"--count-matches", "--crlf", "--encoding", "--engine", "--file", "--files",
		"--files-with-matches", "--files-without-match", "--fixed-strings", "--follow", "--glob",
		"--heading", "--help", "--hidden", "--iglob", "--ignore-case", "--invert-match", "--json",
		"--line-number", "--line-regexp", "--max-columns", "--max-columns-preview", "--max-count",
		"--max-depth", "--max-filesize", "--multiline", "--multiline-dotall", "--no-config",
		"--no-filename", "--no-heading", "--no-ignore", "--no-ignore-dot", "--no-ignore-global",
		"--no-ignore-parent", "--no-ignore-vcs", "--no-line-number", "--no-messages", "--null",
		"--one-file-system", "--only-matching", "--passthru", "--pcre2", "--pretty", "--quiet",
		"--regexp", "--replace", "--smart-case", "--sort", "--sortr", "--stats", "--text",
		"--threads", "--trim", "--type", "--type-list", "--type-not", "--unrestricted",
		"--version", "--vimgrep", "--with-filename", "--word-regexp"),
}

// sortOptions is GNU sort's option set without --compress-program, which
// runs a program, -o and --output, which write a file, -T and
// --temporary-directory, and the file-reading --files0-from and
// --random-source.
var sortOptions = readerOptions{
	short:      "bcCdfghiMmnrRsuVz",
	shortValue: "ktS",
	long: longOptions("--buffer-size", "--check", "--debug", "--dictionary-order",
		"--field-separator", "--general-numeric-sort", "--help", "--human-numeric-sort",
		"--ignore-case", "--ignore-leading-blanks", "--ignore-nonprinting", "--key", "--merge",
		"--month-sort", "--numeric-sort", "--parallel", "--random-sort", "--reverse", "--sort",
		"--stable", "--unique", "--version", "--version-sort", "--zero-terminated"),
}

// uniqOptions is GNU uniq's option set.
var uniqOptions = readerOptions{
	short:      "cdDiuz",
	shortValue: "fsw",
	long: longOptions("--all-repeated", "--check-chars", "--count", "--group", "--help",
		"--ignore-case", "--repeated", "--skip-chars", "--skip-fields", "--unique", "--version",
		"--zero-terminated"),
}

// uniqReads admits a uniq with at most one operand: a second operand is the
// file uniq writes.
func uniqReads(arguments []string, expands []bool) bool {
	operands, admitted := uniqOptions.operands(arguments, expands)
	if !admitted {
		return false
	}
	return operands <= 1
}

// gitReadVerbs are the git commands a reader may run, directly after `git`.
// An option before the verb, such as `-C` or `-c`, is refused with the verb.
var gitReadVerbs = longOptions("blame", "cat-file", "describe", "diff", "grep", "log", "ls-files",
	"ls-tree", "merge-base", "rev-list", "rev-parse", "show", "status")

// gitOptions is the option set a git read verb is admitted with. It leaves
// out --ext-diff, --textconv, --output, -O and --open-files-in-pager, which
// run a program or write a file. git expands an abbreviated long option, so
// only the whole names here pass. No letter is a value letter: a value is
// judged as the next word.
var gitOptions = readerOptions{
	short: "0123456789abcdefghilmnopqrstuvwzABCEFGHILMNPRSUW",
	long: longOptions("--abbrev", "--abbrev-commit", "--abbrev-ref", "--absolute-git-dir",
		"--after", "--all", "--all-match", "--always", "--ancestry-path", "--and", "--author",
		"--author-date-order", "--basic-regexp", "--batch", "--batch-check", "--before",
		"--binary", "--boundary", "--branch", "--break", "--cached", "--cherry-mark",
		"--cherry-pick", "--color", "--committer", "--contains", "--count", "--date",
		"--date-order", "--decorate", "--deleted", "--diff-filter", "--directory", "--dirstat",
		"--error-unmatch", "--exact-match", "--exclude-standard", "--exit-code",
		"--extended-regexp", "--files-with-matches", "--files-without-match", "--find-copies",
		"--find-renames", "--first-parent", "--fixed-strings", "--follow", "--format",
		"--full-diff", "--full-index", "--full-name", "--full-tree", "--function-context",
		"--git-common-dir", "--git-dir", "--graph", "--grep", "--heading", "--histogram",
		"--ignore-all-space", "--ignore-blank-lines", "--ignore-case", "--ignore-space-change",
		"--ignored", "--invert-grep", "--invert-match", "--is-ancestor", "--is-inside-work-tree",
		"--left-right", "--line-number", "--line-porcelain", "--long", "--max-count",
		"--max-depth", "--merges", "--minimal", "--modified", "--name-only", "--name-status",
		"--no-abbrev", "--no-color", "--no-decorate", "--no-ext-diff", "--no-index",
		"--no-merges", "--no-patch", "--no-renames", "--no-textconv", "--not", "--numstat",
		"--oneline", "--only-matching", "--or", "--others", "--parents", "--patch",
		"--patience", "--perl-regexp", "--porcelain", "--pretty", "--quiet", "--raw",
		"--relative", "--relative-date", "--reverse", "--root", "--short", "--shortstat",
		"--show-cdup", "--show-prefix", "--show-toplevel", "--since", "--source", "--stage",
		"--staged", "--stat", "--summary", "--symbolic", "--symbolic-full-name", "--tags",
		"--text", "--topo-order", "--unified", "--until", "--untracked", "--untracked-files",
		"--verify", "--word-diff", "--word-regexp"),
}

// gitReads admits a git read verb with the options gitOptions holds.
func gitReads(arguments []string, expands []bool) bool {
	if len(arguments) == 0 {
		return false
	}
	if expands[0] {
		return false
	}
	if !gitReadVerbs[arguments[0]] {
		return false
	}
	return gitOptions.reads(arguments[1:], expands[1:])
}

// goplsReadSubcommands are the gopls subcommands that only read. Left out:
// codeaction, codelens and execute, which run a command; fix, format,
// imports and rename, which write with -w; serve, remote and mcp, which
// start a server; bug, which opens a browser; and vulncheck.
var goplsReadSubcommands = longOptions("call_hierarchy", "check", "definition", "folding_ranges",
	"help", "highlight", "implementation", "links", "prepare_rename", "references", "semtok",
	"signature", "symbols", "version", "workspace_symbol")

// goplsReadFlags are the flags of the read subcommands, without their dashes.
var goplsReadFlags = longOptions("d", "declaration", "json", "markdown", "matcher", "severity")

// goplsReads admits a gopls read subcommand, named first so no global flag
// (`-remote`, `-logfile`) comes before it, with the flags goplsReadFlags holds.
func goplsReads(arguments []string, expands []bool) bool {
	if len(arguments) == 0 {
		return false
	}
	if expands[0] {
		return false
	}
	if !goplsReadSubcommands[arguments[0]] {
		return false
	}
	for index, argument := range arguments[1:] {
		if expands[index+1] {
			return false
		}
		if !strings.HasPrefix(argument, "-") {
			continue
		}
		name, _, _ := strings.Cut(strings.TrimLeft(argument, "-"), "=")
		if !goplsReadFlags[name] {
			return false
		}
	}
	return true
}

// findPrimaries are the find options, tests and actions a reader may use.
// Left out: -exec, -execdir, -ok and -okdir, which run a command; -delete;
// -fprint, -fprint0, -fprintf and -fls, which write a file; -D and -O.
var findPrimaries = longOptions("-H", "-L", "-P", "-a", "-amin", "-and", "-anewer", "-atime",
	"-cmin", "-cnewer", "-context", "-ctime", "-daystart", "-depth", "-empty", "-executable",
	"-false", "-follow", "-fstype", "-gid", "-group", "-help", "-ignore_readdir_race", "-ilname",
	"-iname", "-inum", "-ipath", "-iregex", "-iwholename", "-links", "-lname", "-ls",
	"-maxdepth", "-mindepth", "-mmin", "-mount", "-mtime", "-name", "-newer", "-newerct",
	"-newermt", "-noignore_readdir_race", "-noleaf", "-nogroup", "-not", "-nouser", "-nowarn",
	"-o", "-or", "-path", "-perm", "-print", "-print0", "-printf", "-prune", "-quit",
	"-readable", "-regex", "-regextype", "-samefile", "-size", "-true", "-type", "-uid",
	"-used", "-user", "-version", "-warn", "-wholename", "-writable", "-xdev", "-xtype")

// findReads admits a find whose every word that starts with `-` is a primary
// findPrimaries holds, or a signed number such as the `-1` of `-mtime -1` or
// the `-10k` of `-size -10k`.
func findReads(arguments []string, expands []bool) bool {
	for index, argument := range arguments {
		if expands[index] {
			return false
		}
		if len(argument) < 2 || argument[0] != '-' {
			continue
		}
		if findPrimaries[argument] {
			continue
		}
		if !signedCount(argument[1:]) {
			return false
		}
	}
	return true
}

// signedCount reports whether a word after its sign is a count with at most
// one unit letter, `1` or `10k`.
func signedCount(word string) bool {
	digits := strings.TrimLeft(word, "0123456789")
	if len(digits) == len(word) {
		return false
	}
	if digits == "" {
		return true
	}
	if len(digits) != 1 {
		return false
	}
	return strings.ContainsRune("bcwkMG", rune(digits[0]))
}

// printfReads admits a printf whose first word is no option: printf takes
// only -v, which assigns its output to a variable and so evaluates an array
// subscript (`printf -v 'a[$(cmd)]' x`). A first word the shell expands could
// be -v, and is refused.
func printfReads(arguments []string, expands []bool) bool {
	if len(arguments) == 0 {
		return true
	}
	if expands[0] {
		return false
	}
	if arguments[0] == "--" {
		return true
	}
	return !strings.HasPrefix(arguments[0], "-")
}

// testOperators are the test builtin's operators. Left out: -v and -R, which
// evaluate an array subscript (`test -v 'a[$(cmd)]'`).
var testOperators = longOptions("-a", "-b", "-c", "-d", "-e", "-ef", "-eq", "-f", "-g", "-G",
	"-ge", "-gt", "-h", "-k", "-L", "-le", "-lt", "-n", "-N", "-ne", "-nt", "-o", "-O", "-ot",
	"-p", "-r", "-s", "-S", "-t", "-u", "-w", "-x", "-z")

// testReads admits a test whose every word is text the shell does not expand,
// and whose every word that starts with `-` is an operator testOperators
// holds. With no expansion and no -v or -R, the builtin evaluates nothing it
// is handed: its integer comparisons read a number and never an arithmetic
// expression.
func testReads(arguments []string, expands []bool) bool {
	for index, argument := range arguments {
		if expands[index] {
			return false
		}
		if len(argument) < 2 || argument[0] != '-' {
			continue
		}
		if !testOperators[argument] {
			return false
		}
	}
	return true
}

// bracketReads admits a `[` that ends with `]` and whose expression testReads
// admits.
func bracketReads(arguments []string, expands []bool) bool {
	last := len(arguments) - 1
	if last < 0 {
		return false
	}
	if arguments[last] != "]" {
		return false
	}
	if expands[last] {
		return false
	}
	return testReads(arguments[:last], expands[:last])
}

// sedLongOptions are the sed long options a reader may use, --expression
// aside. Left out: --in-place, --file, --line-length and --follow-symlinks.
var sedLongOptions = longOptions("--debug", "--help", "--null-data", "--posix", "--quiet",
	"--regexp-extended", "--sandbox", "--separate", "--silent", "--unbuffered", "--version")

// sedReads admits a sed whose options sedLongOptions and the letters
// `nErsuz` hold, and whose every script sedScriptReads admits: the value of
// each -e and --expression, or the first operand when there is none. -i
// (writes the file), -f (a script the hook cannot read) and -l are refused.
func sedReads(arguments []string, expands []bool) bool {
	scripts := make([]string, 0, 2)
	operands := make([]string, 0, 2)
	for index := 0; index < len(arguments); index++ {
		if expands[index] {
			return false
		}
		argument := arguments[index]
		switch {
		case argument == "--":
		case argument == "-":
			operands = append(operands, argument)
		case argument == "--expression":
			index++
			if index == len(arguments) {
				return false
			}
			if expands[index] {
				return false
			}
			scripts = append(scripts, arguments[index])
		case strings.HasPrefix(argument, "--expression="):
			scripts = append(scripts, strings.TrimPrefix(argument, "--expression="))
		case strings.HasPrefix(argument, "--"):
			if !sedLongOptions[argument] {
				return false
			}
		case strings.HasPrefix(argument, "-"):
			script, takesNext, admitted := sedCluster(argument[1:])
			if !admitted {
				return false
			}
			if takesNext {
				index++
				if index == len(arguments) {
					return false
				}
				if expands[index] {
					return false
				}
				script = arguments[index]
			}
			if takesNext || script != "" {
				scripts = append(scripts, script)
			}
		default:
			operands = append(operands, argument)
		}
	}
	if len(scripts) == 0 {
		if len(operands) == 0 {
			return false
		}
		scripts = append(scripts, operands[0])
	}
	for _, script := range scripts {
		if !sedScriptReads(script) {
			return false
		}
	}
	return true
}

// sedCluster judges the letters of one sed short-option word. An `e` takes
// the rest of the word as its script or, when the word ends there, the next
// word (takesNext).
func sedCluster(letters string) (script string, takesNext, admitted bool) {
	for index, letter := range letters {
		if letter == 'e' {
			rest := letters[index+1:]
			return rest, rest == "", true
		}
		if !strings.ContainsRune("nErsuz", letter) {
			return "", false, false
		}
	}
	return "", false, true
}

// sedScriptReads admits a sed script made of commands separated by `;`, each
// an optional address or address range (a line number, `$` or `/regex/`)
// and one of `p`, `P`, `d`, `q`, `=` or a substitution with no `e` or `w`
// flag (sedSubstitution). Every other command, a newline included, is
// refused, so `e`, `w`, `r` and `W`, which run a command or touch a file,
// never pass. Each round consumes at least its command letter, so the loop
// is bounded by the script's length.
func sedScriptReads(script string) bool {
	rest := script
	for {
		rest = strings.TrimLeft(rest, " \t")
		if rest == "" {
			return true
		}
		var admitted bool
		rest, admitted = sedAddresses(rest)
		if !admitted {
			return false
		}
		rest = strings.TrimLeft(rest, " \t")
		if rest == "" {
			return false
		}
		rest, admitted = sedCommandReads(rest)
		if !admitted {
			return false
		}
		rest = strings.TrimLeft(rest, " \t")
		if rest == "" {
			return true
		}
		if rest[0] != ';' {
			return false
		}
		rest = rest[1:]
	}
}

// sedAddresses reads an optional address or `first,last` range from the
// start of a script command and answers what follows it.
func sedAddresses(script string) (string, bool) {
	rest, found, admitted := sedAddress(script)
	if !admitted {
		return "", false
	}
	if !found {
		return script, true
	}
	if !strings.HasPrefix(rest, ",") {
		return rest, true
	}
	rest, found, admitted = sedAddress(rest[1:])
	if !admitted {
		return "", false
	}
	if !found {
		return "", false
	}
	return rest, true
}

// sedAddress reads one address: a line number, `$`, or `/regex/` with an
// optional `I` or `M` flag.
func sedAddress(script string) (rest string, found, admitted bool) {
	if script == "" {
		return script, false, true
	}
	if script[0] == '$' {
		return script[1:], true, true
	}
	if digits := strings.TrimLeft(script, "0123456789"); len(digits) != len(script) {
		return digits, true, true
	}
	if script[0] != '/' {
		return script, false, true
	}
	after, closed := sedDelimited(script[1:], true)
	if !closed {
		return "", false, false
	}
	return strings.TrimLeft(after, "IM"), true, true
}

// sedCommandReads reads one admitted command letter, or a substitution.
func sedCommandReads(script string) (string, bool) {
	switch script[0] {
	case 'p', 'P', 'd', 'q', '=':
		return script[1:], true
	case 's':
		return sedSubstitution(script[1:])
	}
	return "", false
}

// sedSubstitution reads `/regex/replacement/flags` after an `s`: the
// delimiter is `/`, and the flags are `g`, `p`, `i`, `I`, `m`, `M` and a
// count. The `e` flag runs the pattern space as a command and `w` writes a
// file, so neither is admitted.
func sedSubstitution(script string) (string, bool) {
	if !strings.HasPrefix(script, "/") {
		return "", false
	}
	replacement, closed := sedDelimited(script[1:], true)
	if !closed {
		return "", false
	}
	flags, closed := sedDelimited(replacement, false)
	if !closed {
		return "", false
	}
	return strings.TrimLeft(flags, "gpiImM0123456789"), true
}

// sedDelimited answers the text after the `/` that closes a regex or a
// replacement. A backslash escapes the next character. In a regex, a
// bracket expression holding a `/` is refused: sed implementations disagree
// on whether such a `/` closes the regex, and the judge must find the same
// end as the sed that runs. A newline is refused.
func sedDelimited(text string, regex bool) (string, bool) {
	for index := 0; index < len(text); index++ {
		switch text[index] {
		case '\n':
			return "", false
		case '\\':
			index++
			if index == len(text) {
				return "", false
			}
			if text[index] == '\n' {
				return "", false
			}
		case '[':
			if !regex {
				continue
			}
			end, admitted := sedBracketEnd(text, index)
			if !admitted {
				return "", false
			}
			index = end
		case '/':
			return text[index+1:], true
		}
	}
	return "", false
}

// sedBracketEnd answers the index of the `]` that closes the bracket
// expression opening at open, read as POSIX reads it: a `]` first in the
// list is a member, and `[:`, `[.` and `[=` open a class that ends at `:]`,
// `.]` or `=]`. It refuses a bracket expression holding a `/` or a newline
// before that `]`, so a reader that ends the expression at its first `]`
// finds no `/` inside it either.
func sedBracketEnd(text string, open int) (int, bool) {
	index := open + 1
	if index < len(text) && text[index] == '^' {
		index++
	}
	if index < len(text) && text[index] == ']' {
		index++
	}
	for ; index < len(text); index++ {
		switch text[index] {
		case '/', '\n':
			return 0, false
		case ']':
			return index, true
		case '[':
			if index+1 == len(text) {
				return 0, false
			}
			class := text[index+1]
			if !strings.ContainsRune(":.=", rune(class)) {
				continue
			}
			end := strings.Index(text[index+2:], string(class)+"]")
			if end < 0 {
				return 0, false
			}
			if strings.ContainsAny(text[index+2:index+2+end], "/\n") {
				return 0, false
			}
			index += end + 3
		}
	}
	return 0, false
}
