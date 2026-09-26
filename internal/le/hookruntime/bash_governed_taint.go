// Design: docs/contributing/running-commands.md -- the Bash guard over plan/ and ai/rules/
// Related: bash.go -- governedShellWrite, the literal-path routes this file extends
package hookruntime

import (
	"regexp"
	"strings"
)

// The literal routes in bash.go see a governed tree only when its name sits
// beside the write. A loop reaches the same files through a variable, a pipe
// reaches them through xargs, and a cd reaches them through a relative path. On
// 2026-09-08 a `while read` loop rewrote 44 spec files that way. This file
// follows the name from the place that binds it to the place that writes
// through it. It is a taint over the command text, not a shell parser: a read
// into scratch stays free because the scratch variable never held a governed
// path.
//
// Each sink is a literal route with its trailing tree removed, so a flag
// cluster or argument class the literal route learns (`perl -0pi`,
// `sed --in-place`, a quoted `;`) reaches these routes with no second edit.
var (
	governedEditSink     = governedSink(governedSed)
	governedRedirectSink = governedSink(governedRedirect)
	governedTeeSink      = governedSink(governedTee)
	governedCopySink     = governedSink(governedCopy)
	governedSinks        = []string{governedEditSink, governedRedirectSink, governedTeeSink, governedCopySink}

	// governedTreeWord is plan or ai/rules as a whole word, bare or with a path
	// below it: `plan`, "plan/x.md", ./ai/rules. A bare word counts, because
	// `find plan` lists the tree as surely as `find plan/` does.
	governedTreeWord = `(?:^|[\s=(` + "`" + `])["']?(?:\./)?(?:plan|ai/rules)(?:/|["'\s;&|)` + "`" + `]|$)`
	governedTree     = regexp.MustCompile(governedTreeWord)

	// governedListing is a command that lists or searches a governed tree. The
	// bare word counts only as an argument of these commands, so a grep pattern
	// that happens to read "plan" is the one false match this admits.
	governedListing = regexp.MustCompile(`\b(?:find|ls|git[ \t]+ls-files|grep|rg|fd)\b[^|;&\n]*` + governedTreeWord)

	taintAssign = regexp.MustCompile(`(?:^|[\s;&|(])([A-Za-z_]\w*)=("[^"]*"|'[^']*'|\$\([^)]*\)|` +
		"`[^`]*`" + `|[^\s;&|()` + "`" + `]+)`)
	taintFor  = regexp.MustCompile(`\bfor[ \t]+([A-Za-z_]\w*)[ \t]+in([ \t][^;\n]*)`)
	taintRead = regexp.MustCompile(`\bwhile[ \t]+(?:IFS=\S*[ \t]+)?read[ \t]+(?:-[A-Za-z]+[ \t]+)*([A-Za-z_]\w*)`)
	// taintLoopFeed is the input redirect of a loop: `done < list` or
	// `done < <(find plan ...)`.
	taintLoopFeed = regexp.MustCompile(`\bdone[ \t]*<[^;&\n]*`)
	taintCd       = regexp.MustCompile(`(?:^|[\s;&|(])(?:cd|pushd)[ \t]*` + governedTreeWord)

	// governedFanOut is an in-place editor run by xargs or find -exec, whose
	// file list comes from the command before it.
	governedFanOut = regexp.MustCompile(`(?:\bxargs|-exec(?:dir)?)\b` + governedArgs + governedEditSink)
	// governedSubstitution is an in-place editor handed its files by a command
	// substitution that names a governed tree: `sed -i x $(find plan ...)`.
	governedSubstitution = regexp.MustCompile(governedEditSink + `["']?(?:\$\(|` + "`" + `)[^)` + "`" + `]*` + governedTreeWord)

	// governedRelativeSinks are the writes that land in a governed tree after a
	// cd into one: any in-place editor, and a redirect, tee, copy or move whose
	// target is relative. An absolute or variable target is judged elsewhere.
	governedRelative      = `[^\s/$&|;<>()'"~]`
	governedRelativeSinks = []*regexp.Regexp{
		regexp.MustCompile(governedEditSink),
		regexp.MustCompile(governedRedirectSink + governedRelative),
		regexp.MustCompile(governedTeeSink + governedRelative),
		regexp.MustCompile(governedCopySink + governedRelative),
	}
)

// governedSink returns a literal route's pattern with its trailing governed
// tree removed. A route that does not end in the tree is returned unchanged,
// so its sink still demands a governed path and fails narrow, never open.
// TestGovernedSinksDeriveFromLiteralRoutes refuses that drift.
func governedSink(literal *regexp.Regexp) string {
	source := literal.String()
	prefix, found := strings.CutSuffix(source, governedPath.String())
	if !found {
		return source
	}
	return prefix
}

// governedTaintWrite reports whether the command writes into a governed tree
// through a path it does not spell out: a variable bound to a governed path, a
// file list piped from a governed tree, or a directory changed into one.
//
// Each route is its own named test, so a reader can see which one fired.
func governedTaintWrite(command string) bool {
	if governedFanOutEdit(command) {
		return true
	}
	if governedSubstitution.MatchString(command) {
		return true
	}
	if governedCdWrite(command) {
		return true
	}
	for _, name := range taintedNames(command) {
		if variableWrite(command, name) {
			return true
		}
	}
	return false
}

// governedFanOutEdit reports an in-place editor run by xargs or find -exec in
// a statement whose earlier commands name a governed tree.
func governedFanOutEdit(command string) bool {
	for _, location := range governedFanOut.FindAllStringIndex(command, -1) {
		if namesGovernedTree(statementTail(command[:location[0]])) {
			return true
		}
	}
	return false
}

// governedCdWrite reports a write that lands in a governed tree because an
// earlier cd or pushd moved into it. The cd taints the rest of the command.
func governedCdWrite(command string) bool {
	location := taintCd.FindStringIndex(command)
	if location == nil {
		return false
	}
	rest := command[location[1]:]
	for _, sink := range governedRelativeSinks {
		if sink.MatchString(rest) {
			return true
		}
	}
	return false
}

// taintedNames returns every variable the command binds to a governed path,
// from each of the three sources.
func taintedNames(command string) []string {
	names := assignedNames(command)
	names = append(names, loopNames(command)...)
	return append(names, readNames(command)...)
}

// assignedNames returns each variable assigned a value that names a governed
// tree: `f=plan/x.md`, `d="ai/rules"`, `f=$(find plan -name x)`.
func assignedNames(command string) []string {
	var names []string
	for _, match := range taintAssign.FindAllStringSubmatch(command, -1) {
		if governedTree.MatchString("=" + match[2]) {
			names = append(names, match[1])
		}
	}
	return names
}

// loopNames returns each for-loop variable whose word list names a governed
// tree: `for f in plan/*.md`, `for f in $(find plan -name x)`.
func loopNames(command string) []string {
	var names []string
	for _, match := range taintFor.FindAllStringSubmatch(command, -1) {
		if governedTree.MatchString(match[2]) {
			names = append(names, match[1])
		}
	}
	return names
}

// readNames returns each `while read` variable whose loop is fed from a
// governed tree, by a pipe in front of it or by the loop's input redirect.
func readNames(command string) []string {
	fedFromTree := false
	for _, feed := range taintLoopFeed.FindAllString(command, -1) {
		if namesGovernedTree(feed) {
			fedFromTree = true
		}
	}
	var names []string
	for _, match := range taintRead.FindAllStringSubmatchIndex(command, -1) {
		name := command[match[2]:match[3]]
		if fedFromTree {
			names = append(names, name)
			continue
		}
		if pipedFromTree(statementTail(command[:match[0]])) {
			names = append(names, name)
		}
	}
	return names
}

// pipedFromTree reports whether a statement's text ends in a pipe and names a
// governed tree, so whatever reads the pipe reads governed paths.
func pipedFromTree(upstream string) bool {
	trimmed := strings.TrimSpace(upstream)
	if !strings.HasSuffix(trimmed, "|") {
		return false
	}
	return namesGovernedTree(trimmed)
}

// namesGovernedTree reports whether the text names a governed path, or lists
// or searches a governed tree by its bare name.
func namesGovernedTree(text string) bool {
	if governedPath.MatchString(text) {
		return true
	}
	return governedListing.MatchString(text)
}

// statementTail returns the text after the last statement separator (`;`, a
// newline, `&&` or `||`), which is the pipeline the text ends in.
func statementTail(text string) string {
	start := 0
	for _, separator := range []string{";", "\n", "&&", "||"} {
		index := strings.LastIndex(text, separator)
		if index < 0 {
			continue
		}
		end := index + len(separator)
		if end > start {
			start = end
		}
	}
	return text[start:]
}

// variableWrite reports whether any write sink targets the variable: an
// in-place editor, a redirect, tee, or a copy or move, with `$name` or
// `${name}` as the path, quoted or not.
func variableWrite(command, name string) bool {
	target := `["']?[^\s"'|;&$]*\$\{?` + regexp.QuoteMeta(name) + `\b`
	for _, sink := range governedSinks {
		if regexp.MustCompile(sink + target).MatchString(command) {
			return true
		}
	}
	return false
}
