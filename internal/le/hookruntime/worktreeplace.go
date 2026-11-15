// Design: docs/architecture/core-design.md -- native Bash hook policy
// Related: worktreecopy.go -- worktreeCopyJudge, the walk that asks which lines and paths concern a worktree
// Related: readgate.go -- insideRoot and resolveExisting, the checkout a path lies in
// Related: shellparse.go -- readWord, the value of a word
package hookruntime

import (
	"path/filepath"
	"slices"
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

// worktreeLineConcerns reports whether the guard reads a line at all: it
// starts under the checkout's .claude/, which holds its worktrees, or a word
// of it may name a path there (worktreeLineNamesClaude).
func worktreeLineConcerns(command string, parse commandParse, root, cwd string) bool {
	relative, inside := worktreeCheckoutRelative(root, cwd)
	if inside {
		if worktreeUnder(relative, worktreeClaudeDirectory) {
			return true
		}
	}
	return worktreeLineNamesClaude(command, parse)
}

// worktreeLineNamesClaude reports whether a word of a line may name a path
// under .claude/worktrees, spelled in one piece or not: a word with a
// `.claude` or a `worktrees` component (worktreeNamesClaude), in its value
// with the quotes removed (`'.claude/'worktrees`), or in its typed text when
// the hook cannot read its value. A path split across a `cd` (`cd .claude &&
// cp worktrees/...`), a glob or a variable still has such a component. A line
// the parser cannot read the way bash runs it is asked of its text.
func worktreeLineNamesClaude(command string, parse commandParse) bool {
	if !parse.parsed() {
		return worktreeTextNamesClaude(command)
	}
	if treeMisreadsLine(command, parse) != "" {
		return worktreeTextNamesClaude(command)
	}
	walk := &readerWalk{source: command}
	names := false
	syntax.Walk(parse.file, func(node syntax.Node) bool {
		word, ok := node.(*syntax.Word)
		if !ok {
			return !names
		}
		value, err := readWord(word)
		if err != nil {
			names = names || worktreeNamesClaude(walk.text(word))
			return !names
		}
		names = names || worktreeNamesClaude(value.text)
		return !names
	})
	return names
}

// worktreeTextNamesClaude reports whether the text of a line names .claude or
// worktrees anywhere.
func worktreeTextNamesClaude(text string) bool {
	if strings.Contains(text, worktreeClaudeDirectory) {
		return true
	}
	return strings.Contains(text, worktreeWorktreesComponent)
}

// worktreeNamesClaude reports whether a word has a component, between
// slashes, `=` and `:`, that is .claude or worktrees, or a glob or brace
// expansion that may match .claude (`.cl*`, `.{claude,git}`).
func worktreeNamesClaude(text string) bool {
	for _, component := range strings.FieldsFunc(text, worktreeComponentSeparator) {
		if slices.ContainsFunc(worktreeBraceAlternatives(component), worktreeComponentNamesClaude) {
			return true
		}
	}
	return false
}

// worktreeComponentSeparator reports whether a character ends a component of
// a word: a slash, or the `=` and `:` of an assignment or a path list.
func worktreeComponentSeparator(character rune) bool {
	return strings.ContainsRune("/=:", character)
}

// worktreeComponentNamesClaude reports whether one component is .claude or
// worktrees, or a glob that matches .claude. Without dotglob, bash matches a
// leading dot only with a literal one, so a glob that may match .claude
// starts with `.`, and `*` alone is no match. A glob that matches worktrees
// alone is not asked for: its parent is .claude, which a word, the line's
// directory, or a variable the hook cannot read names.
func worktreeComponentNamesClaude(component string) bool {
	if component == worktreeClaudeDirectory {
		return true
	}
	if component == worktreeWorktreesComponent {
		return true
	}
	if !strings.HasPrefix(component, ".") {
		return false
	}
	if !strings.ContainsAny(component, "*?[") {
		return false
	}
	matched, err := filepath.Match(component, worktreeClaudeDirectory)
	if err != nil {
		// A malformed pattern matches nothing: bash leaves it a word.
		return false
	}
	return matched
}

// worktreeBraceAlternatives answers the words the first brace group of a
// component expands to (`.{claude,git}` is `.claude` and `.git`), or the
// component alone when it holds none. A second group stays as typed in each
// alternative, so a word is never expanded past one level.
func worktreeBraceAlternatives(component string) []string {
	open := strings.IndexByte(component, '{')
	if open < 0 {
		return []string{component}
	}
	length := strings.IndexByte(component[open:], '}')
	if length < 0 {
		return []string{component}
	}
	head, body, tail := component[:open], component[open+1:open+length], component[open+length+1:]
	alternatives := strings.Split(body, ",")
	words := make([]string, 0, len(alternatives))
	for _, alternative := range alternatives {
		words = append(words, head+alternative+tail)
	}
	return words
}

// placeOf answers where a source word points, resolved against the directory
// its command runs in. A word that expands a variable the line bound to a
// worktree path is inside (namesInside), and so is a word the shell expands
// whose own components name .claude/worktrees, wherever the expansion lands
// (`"$X/z/.claude/worktrees/x/a.go"`). A word whose first component the
// shell expands (`$X/a`, `~user/a`) may name any path, so it is never joined
// to the directory: it is unknown, as is any other word the hook cannot read,
// or cannot resolve. A glob in a relative word matches names in the
// directory, so that word is joined to it.
func (judge *worktreeCopyJudge) placeOf(word worktreeWord, directory worktreeDirectory) worktreePlace {
	if word.inside {
		return worktreePlaceInside
	}
	if !word.exact {
		if worktreePath(filepath.Clean(word.token)) {
			return worktreePlaceInside
		}
		if worktreeHeadExpands(word.token) {
			return worktreePlaceUnknown
		}
	}
	path, resolved := worktreeResolve(word.token, directory)
	if !resolved {
		// A relative path from a directory the hook cannot name is placed
		// by its own components alone.
		if worktreePath(filepath.Clean(word.token)) {
			return worktreePlaceInside
		}
		return worktreePlaceUnknown
	}
	if judge.inWorktree(path) {
		return worktreePlaceInside
	}
	if word.exact {
		return worktreePlaceOutside
	}
	return worktreePlaceUnknown
}

// destinationOf answers where a destination word writes. A device is no file
// of a checkout. A word that starts with a plain `$name` the line placed
// under a worktree (placed), with no `..` after it, writes under that
// worktree, from anywhere for an absolute value and from the directory a
// relative value was bound in (placedUnder). A word that starts with an
// expansion the hook cannot read (`$X/...`), or holds a `..` after one, may
// land anywhere, and so may a relative word from a directory the hook cannot
// name: each is unplaced, which counts as the main tree, unless its own
// components name a worktree. A home head (`~/`, `$HOME/`) was written as
// the home directory when the hook reads one (homeDirectory). A line another
// host runs writes on that host (remoteDestinationOf). Any other word, the
// shell's expansions after a literal head included, is resolved from the
// directory its command runs in, through its links, and is in the main tree
// when it lands in the checkout outside .claude/worktrees and tmp/.
func (judge *worktreeCopyJudge) destinationOf(target worktreeWord, directory worktreeDirectory) worktreeDestination {
	if worktreeDevice(target) {
		return worktreeDestinationElsewhere
	}
	if judge.placedUnder(target, directory) {
		return worktreeDestinationElsewhere
	}
	if judge.remote {
		return remoteDestinationOf(target, directory)
	}
	if !worktreeResolvable(target) {
		return worktreeUnplacedUnlessWorktree(target.token)
	}
	path, resolved := worktreeResolve(target.token, directory)
	if !resolved {
		return worktreeUnplacedUnlessWorktree(target.token)
	}
	relative, inside := worktreeCheckoutRelative(judge.root, path)
	if !inside {
		return worktreeDestinationElsewhere
	}
	relative, inside = worktreeLinkedRelative(judge.root, path, relative)
	if !inside {
		return worktreeDestinationElsewhere
	}
	if worktreeUnder(relative, worktreesDirectory) {
		return worktreeDestinationElsewhere
	}
	if worktreeUnder(relative, worktreeScratchDirectory) {
		return worktreeDestinationElsewhere
	}
	return worktreeDestinationMain
}

// worktreeUnplacedUnlessWorktree answers a destination the hook cannot place
// by its own components: in a worktree when they name one, unplaced
// otherwise. A component inside a `${...}` is an operand of the expansion,
// never a component of the path (`${f#.claude/worktrees/x/}` cuts the
// worktree off), so only the text outside such groups is read.
func worktreeUnplacedUnlessWorktree(token string) worktreeDestination {
	if worktreePath(filepath.Clean(worktreeOutsideBraces(token))) {
		return worktreeDestinationElsewhere
	}
	return worktreeDestinationUnplaced
}

// worktreeOutsideBraces answers a word's text with each `${...}` group
// written as `$`, a nested group counted to its closing brace. A group left
// open drops the rest of the text, which names nothing then.
func worktreeOutsideBraces(token string) string {
	var outside strings.Builder
	depth := 0
	for index := 0; index < len(token); index++ {
		if depth == 0 {
			if strings.HasPrefix(token[index:], "${") {
				outside.WriteByte('$')
				depth = 1
				index++
				continue
			}
			outside.WriteByte(token[index])
			continue
		}
		switch token[index] {
		case '{':
			depth++
		case '}':
			depth--
		}
	}
	return outside.String()
}

// placedUnder reports whether a destination writes under the worktree path a
// variable at its head names (placed): it holds no `..` component that could
// climb out, and the path the variable names is the one the write reaches.
// A name bound only to absolute words names its path from anywhere. A name
// bound to a relative word (boundFrom) names it from the directory it was
// bound in, so the write must run from that same directory: one the hook
// names, or, on another host, where a relative path from a directory the hook
// cannot name is judged by its components alone, the same directory the hook
// names or the same one it cannot. A `cd` between the binding and the write
// (`out=a.go; cd ../../..; cat ... > "$out"`) leaves the destination
// unplaced, and a loop body that may change the directory leaves it one the
// hook cannot name, so a `$f` it binds from a relative glob stays unplaced
// there.
func (judge *worktreeCopyJudge) placedUnder(target worktreeWord, directory worktreeDirectory) bool {
	if target.placed == "" {
		return false
	}
	if worktreeClimbs(target.token) {
		return false
	}
	bound, relative := judge.boundFrom[target.placed]
	if !relative {
		return true
	}
	if bound.path != directory.path {
		return false
	}
	if judge.remote {
		return true
	}
	return directory.known()
}

// remoteDestinationOf answers where a destination of a line another host
// runs writes. That host's files are no file of this checkout: an absolute
// path and a home head (`~/`, `$HOME/`) are elsewhere, whether their
// components name a worktree or not, and so is a relative path after a `cd`
// that placed the line in a worktree (worktreeRemoteDirectory) while it stays
// in that worktree, or in another one, and so is a relative path that climbs
// from that worktree into the remote checkout's gitignored tmp/
// (`../../../tmp/out.log`), as the local tmp/ exemption reads it. A relative
// path that climbs out of the worktree anywhere else
// (`../../../internal/a.go`), a relative path from a directory the hook
// cannot name, and a word that starts with any other expansion, are unplaced
// unless their components name a worktree, so they count as the main tree
// when the line also read a worktree file.
func remoteDestinationOf(target worktreeWord, directory worktreeDirectory) worktreeDestination {
	if worktreeHomeHead(target.token) {
		return worktreeDestinationElsewhere
	}
	if !worktreeResolvable(target) {
		return worktreeUnplacedUnlessWorktree(target.token)
	}
	path, resolved := worktreeResolve(target.token, directory)
	if !resolved {
		return worktreeUnplacedUnlessWorktree(target.token)
	}
	if filepath.IsAbs(target.token) {
		return worktreeDestinationElsewhere
	}
	// A relative path resolved from the worktree a remote `cd` named
	// (worktreeRemoteTail), joined and cleaned, so a `..` that climbs out of
	// it leaves no worktree components.
	if worktreeRemoteTail(path) != "" {
		return worktreeDestinationElsewhere
	}
	// The tail models the remote checkout root as `/`, so a path resolved
	// from it that lies under `/tmp` is under that checkout's tmp/, which
	// holds no tracked work.
	if worktreeRemoteTail(directory.path) != "" {
		relative := strings.TrimPrefix(filepath.ToSlash(path), "/")
		if worktreeUnder(relative, worktreeScratchDirectory) {
			return worktreeDestinationElsewhere
		}
	}
	return worktreeUnplacedUnlessWorktree(target.token)
}

// worktreeHomeHead reports whether a word's text starts with the home
// directory the shell expands: `~` or `$HOME` (or `${HOME}`), alone or
// before a slash.
func worktreeHomeHead(token string) bool {
	for _, head := range []string{"~", "$HOME", "${HOME}"} {
		rest, found := strings.CutPrefix(token, head)
		if !found {
			continue
		}
		if rest == "" {
			return true
		}
		if strings.HasPrefix(rest, "/") {
			return true
		}
	}
	return false
}

// worktreeHeadExpands reports whether the first component of a word's text
// is one the shell expands into a path of its own: a parameter, a
// substitution, or a tilde.
func worktreeHeadExpands(token string) bool {
	head, _, _ := strings.Cut(token, "/")
	if strings.HasPrefix(head, "~") {
		return true
	}
	return strings.ContainsAny(head, "$`")
}

// worktreeClimbs reports whether a word's text holds a `..` component.
func worktreeClimbs(token string) bool {
	return slices.Contains(strings.Split(token, "/"), "..")
}

// inWorktree reports whether an absolute path is under a worktree: under the
// checkout's .claude/worktrees for a path in the checkout, and by its own
// components for a path outside it (another checkout's worktree) or on
// another host.
func (judge *worktreeCopyJudge) inWorktree(path string) bool {
	if judge.remote {
		return worktreePath(path)
	}
	relative, inside := worktreeCheckoutRelative(judge.root, path)
	if !inside {
		return worktreePath(path)
	}
	return worktreeUnder(relative, worktreesDirectory)
}

// worktreeDevice reports whether a destination read exactly is a device
// (`/dev/null`).
func worktreeDevice(target worktreeWord) bool {
	if !target.exact {
		return false
	}
	return strings.HasPrefix(filepath.Clean(target.token), "/dev/")
}

// worktreeResolvable reports whether the hook can place a destination: a
// word it read exactly, or one the shell expands only after a literal first
// component and that holds no `..` component, so the expansion stays under
// that component.
func worktreeResolvable(target worktreeWord) bool {
	if target.exact {
		return true
	}
	head, _, _ := strings.Cut(target.token, "/")
	if strings.ContainsAny(head, expandedPathCharacters+`"'\`) {
		return false
	}
	return !worktreeClimbs(target.token)
}

// worktreeLinkedRelative answers a destination inside the checkout where its
// links lead (resolveExisting, with the root's links resolved too), and
// whether that is still inside the checkout: tmp/l, a link to ../internal,
// is internal/. A root whose links cannot be resolved keeps relative, the
// path as typed, which the lexical test already placed inside.
func worktreeLinkedRelative(root, path, relative string) (string, bool) {
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return relative, true
	}
	linked, err := filepath.Rel(resolvedRoot, resolveExisting(path))
	if err != nil {
		return relative, true
	}
	linked = filepath.ToSlash(linked)
	return linked, insideRoot(linked)
}

// worktreeResolve answers the absolute, cleaned path a word names from
// directory, and false for a relative word from a directory the hook cannot
// name.
func worktreeResolve(token string, directory worktreeDirectory) (string, bool) {
	if filepath.IsAbs(token) {
		return filepath.Clean(token), true
	}
	if !directory.known() {
		return "", false
	}
	return filepath.Join(directory.path, token), true
}

// worktreeCheckoutRelative answers an absolute path relative to the checkout
// root, and whether it lies inside the checkout. A path that leaves the root
// is retried with symlinks resolved on both sides, as checkoutPath does, so a
// symlink alias of the checkout is still this checkout.
func worktreeCheckoutRelative(root, path string) (string, bool) {
	relative, err := filepath.Rel(root, path)
	if err == nil {
		relative = filepath.ToSlash(relative)
		if insideRoot(relative) {
			return relative, true
		}
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", false
	}
	relative, err = filepath.Rel(resolvedRoot, resolveExisting(path))
	if err != nil {
		return "", false
	}
	relative = filepath.ToSlash(relative)
	return relative, insideRoot(relative)
}

// worktreeUnder reports whether a checkout-relative path is directory or under
// it.
func worktreeUnder(relative, directory string) bool {
	if relative == directory {
		return true
	}
	return strings.HasPrefix(relative, directory+"/")
}

// worktreePath reports whether a cleaned path names .claude/worktrees or a
// path under it, by its components.
func worktreePath(path string) bool {
	return strings.Contains("/"+path+"/", "/"+worktreesDirectory+"/")
}

// worktreeRemoteDirectory answers the directory a `cd` or `pushd` on another
// host moves to, where the hook can test no directory and knows no checkout:
// when its one operand's components, joined to directory when it is relative
// and the hook names directory, name .claude/worktrees/<name>, the directory
// is that path from .claude/worktrees on (worktreeRemoteTail), which places
// the relative paths after it in that worktree; any other target is a
// directory the hook cannot name.
func worktreeRemoteDirectory(arguments []worktreeWord, directory worktreeDirectory) worktreeDirectory {
	operands := make([]string, 0, 1)
	for _, word := range arguments {
		if oneOf(word.token, "-L", "-P", "-e", "-@", "--") {
			continue
		}
		operands = append(operands, word.token)
	}
	if len(operands) != 1 {
		return worktreeDirectory{}
	}
	target := operands[0]
	if !filepath.IsAbs(target) {
		if directory.known() {
			if !worktreeHeadExpands(target) {
				target = filepath.Join(directory.path, target)
			}
		}
	}
	return worktreeDirectory{path: worktreeRemoteTail(filepath.Clean(target))}
}

// worktreeRemoteTail answers a cleaned path from its .claude/worktrees/<name>
// components on, as an absolute path (`~/z/.claude/worktrees/x/a` is
// `/.claude/worktrees/x/a`), and empty when no such components name a
// worktree. A path of another host is judged by its components alone, so
// what lies before them does not matter.
func worktreeRemoteTail(path string) string {
	_, tail, found := strings.Cut("/"+path+"/", "/"+worktreesDirectory+"/")
	if !found {
		return ""
	}
	tail = strings.TrimSuffix(tail, "/")
	name, _, _ := strings.Cut(tail, "/")
	if name == "" {
		return ""
	}
	return "/" + worktreesDirectory + "/" + tail
}
