// Design: docs/architecture/core-design.md -- native Bash hook policy
// Related: bash.go -- bashRootBuild, the guard that renders this judgment
// Related: shellline.go -- lineCommands, the commands, separators and expansions this judges
// Related: readers.go -- readerCommand, the readers a judged build line may hold
package hookruntime

import (
	"errors"
	"go/build"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"syscall"
)

// goBuildOutcome is what bashRootBuild decides about the `go build` calls of
// one line.
type goBuildOutcome int

const (
	// goBuildUnspecified is the zero value, which no judgment answers.
	goBuildUnspecified goBuildOutcome = iota
	// goBuildAllowed is a line whose every `go build` writes nothing to the
	// working directory: it names an output under the checkout's bin/, or
	// compiles packages that produce no binary.
	goBuildAllowed
	// goBuildWholeTree is a build of every package of the module, or, on the
	// text path (goBuildText) where the line does not parse, of any `...`
	// pattern, which the text cannot tell from the module's whole tree. The
	// no-whole-tree rule forbids it whatever its output.
	goBuildWholeTree
	// goBuildBinary is a build that writes its result outside bin/, or one
	// whose packages or output the hook cannot place.
	goBuildBinary
	// goBuildToolchain is a line judged by its text (goBuildText) that names
	// -toolexec, -overlay or -modfile, which make the go command run or read
	// what the line names, whatever its -o says.
	goBuildToolchain
)

// goBuildToolchainFlag matches a -toolexec, -overlay or -modfile flag in a
// line's text, with one dash or two, joined to its value by `=` or spaced,
// and after `=` or a quote, so inside a GOFLAGS assignment too. `-mod=` is
// another flag and does not match.
var goBuildToolchainFlag = regexp.MustCompile(`(?:^|[\s='"])--?(toolexec|overlay|modfile)(?:[\s='"]|$)`)

// goBuildToolchainEffect says what each flag goBuildToolchainFlag matches
// makes the toolchain run or read.
var goBuildToolchainEffect = map[string]string{
	"toolexec": "makes the go command run the program it names for every compile and link step",
	"overlay":  "makes the go command read the files its JSON file names in place of the sources the hook reads",
	"modfile":  "makes the go command read the module file it names in place of go.mod",
}

// goBuildWalkMax bounds how many directories one `/...` pattern visits. A
// pattern that reaches more without finding two packages is refused, so the
// hook's cost per call stays bounded whatever the pattern names.
const goBuildWalkMax = 2048

// goBuildTextWholeTree is the text judgment's whole-tree build: a `go build`
// followed, within the same command text, by a `...` pattern.
var goBuildTextWholeTree = regexp.MustCompile(`go\s+build\s[^;&|\n]*\.{3}`)

// goBuildValueFlags are the go build flags whose value is the next word when
// no `=` joins it to the flag.
var goBuildValueFlags = map[string]bool{
	"-C": true, "-asmflags": true, "-buildmode": true, "-compiler": true,
	"-covermode": true, "-coverpkg": true, "-gccgoflags": true, "-gcflags": true,
	"-installsuffix": true, "-ldflags": true, "-mod": true, "-modfile": true,
	"-o": true, "-overlay": true, "-p": true, "-pgo": true, "-pkgdir": true,
	"-tags": true, "-toolexec": true,
}

// buildCheckout is what one hook call reads of the checkout once: its root,
// the root with every symbolic link resolved, and the module path its go.mod
// declares, "" when that cannot be read, which resolves no import path.
type buildCheckout struct {
	root     string
	physical string
	module   string
}

// goBuildArgs is what one `go build` names.
type goBuildArgs struct {
	output    string
	hasOutput bool
	// directory is the -C value, the directory the build runs in, relative to
	// the shell's working directory.
	directory string
	tags      []string
	packages  []string
	// cwd is the shell's logical working directory when the build runs ($PWD,
	// which the go command reads relative packages from): the checkout root,
	// or the directory a literal cd earlier on the line moved to, and "" when
	// the hook cannot tell.
	cwd string
	// physical is cwd with every symbolic link resolved, the directory the
	// kernel resolves a relative -o and -C from, and "" when cwd is.
	physical string
	// moved reports a cd earlier on the line.
	moved bool
	// textJudged reports a flag that hands the go command a program to run
	// (-toolexec) or a file that replaces what the hook reads (-overlay,
	// -modfile), so the line is judged by its text (goBuildText), which
	// refuses a line that names one of the three whatever its -o
	// (spec-agent-economy-6-shell-parser AC-34). A flag that reaches the
	// build only through the hook's own GOFLAGS is not in the line's text,
	// so there the text verdict is the one the -o path decides. The parse
	// only declines to lift that verdict for a flag whose effect it cannot
	// read.
	textJudged bool
}

// parseGoBuild reads the arguments after `go build`. A flag spelled with two
// dashes is the same flag, as the go command reads it.
func parseGoBuild(arguments []string) goBuildArgs {
	var args goBuildArgs
	for index := 0; index < len(arguments); index++ {
		argument := arguments[index]
		if !strings.HasPrefix(argument, "-") {
			args.packages = append(args.packages, argument)
			continue
		}
		name, value, joined := strings.Cut(strings.TrimPrefix(argument, "-"), "=")
		name = "-" + strings.TrimPrefix(name, "-")
		if !joined && goBuildValueFlags[name] && index+1 < len(arguments) {
			index++
			value = arguments[index]
		}
		switch name {
		case "-o":
			args.output, args.hasOutput = value, true
		case "-C":
			args.directory = value
		case "-tags":
			args.tags = strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == ' ' })
		case "-toolexec", "-overlay", "-modfile":
			args.textJudged = true
		}
	}
	return args
}

// goFlags answers the flags GOFLAGS sets for a build: the hook's own
// environment first, then an assignment on the command line before the build,
// which the go command reads in the same way. Each is a space-separated list
// of `-flag=value` words.
func goFlags(before []string) []string {
	flags := strings.Fields(os.Getenv("GOFLAGS"))
	for _, word := range before {
		if value, found := strings.CutPrefix(word, "GOFLAGS="); found {
			flags = append(flags, strings.Fields(value)...)
		}
	}
	return flags
}

// binOutput reports whether a path relative to the checkout root is under
// bin/ or under a session's own bin/ directory.
func binOutput(output string) bool {
	clean := filepath.ToSlash(filepath.Clean(output))
	if clean == "bin" || strings.HasPrefix(clean, "bin/") {
		return true
	}
	return goBuildSession.MatchString("-o " + clean + "/")
}

// insideCheckout answers a directory relative to the checkout root, and false
// when it lies outside, compared by path as the go command reads it: a
// symbolic link such as /proc/self/cwd is outside.
func insideCheckout(root, path string) (string, bool) {
	relative, err := filepath.Rel(root, filepath.Clean(path))
	if err != nil {
		return "", false
	}
	if relative == ".." || strings.HasPrefix(relative, "../") {
		return "", false
	}
	return relative, true
}

// goBuildBase answers the directory a build's relative packages resolve from:
// the shell's logical working directory, which the go command reads from
// $PWD, or, when the build names -C, the directory the go command changes to,
// which it then reads physically (goBuildDirectory). It answers false when
// the hook cannot tell.
func goBuildBase(args goBuildArgs) (string, bool) {
	if args.directory != "" {
		return goBuildDirectory(args)
	}
	if args.cwd == "" {
		return "", false
	}
	return args.cwd, true
}

// goBuildOutputBase answers the directory the kernel resolves a relative -o
// from: the physical working directory, or the -C directory.
func goBuildOutputBase(args goBuildArgs) (string, bool) {
	if args.directory != "" {
		return goBuildDirectory(args)
	}
	if args.physical == "" {
		return "", false
	}
	return args.physical, true
}

// goBuildDirectory answers the -C directory with every symbolic link
// resolved: the go command changes to it with chdir, which the kernel
// resolves from the physical working directory.
func goBuildDirectory(args goBuildArgs) (string, bool) {
	base := args.physical
	if filepath.IsAbs(args.directory) {
		base = "/"
	}
	if base == "" {
		return "", false
	}
	return physicalPath(base, args.directory, true)
}

// expandedPathCharacters are the characters a shell expands in a path, so a
// path holding one is not the path the go command receives.
const expandedPathCharacters = "$`*?[~{"

// outputInBin reports whether the -o value resolves to the checkout's bin/ or
// a session bin/ directory, from the directory the build runs in, and whether
// it could be resolved at all. The path is resolved as the kernel resolves
// it, every symbolic link followed, both as typed and cleaned, because the
// go command may clean it before it writes: a `link/..` then climbs from the
// link's target in one reading and not in the other, and both must land in
// bin/. The checkout root is compared with its links resolved too.
func outputInBin(checkout buildCheckout, args goBuildArgs) (inBin, resolved bool) {
	if strings.ContainsAny(args.output, expandedPathCharacters) {
		return false, false
	}
	base := "/"
	if !filepath.IsAbs(args.output) {
		var ok bool
		base, ok = goBuildOutputBase(args)
		if !ok {
			return false, false
		}
	}
	for _, output := range []string{args.output, filepath.Clean(args.output)} {
		target, ok := physicalPath(base, output, false)
		if !ok {
			return false, false
		}
		relative, inside := insideCheckout(checkout.physical, target)
		if !inside {
			return false, true
		}
		if !binOutput(relative) {
			return false, true
		}
	}
	return true, true
}

// physicalPath answers the path a name reaches from base as the kernel
// resolves it: every symbolic link it meets is followed, so a `..` after a
// link climbs from the link's target. A component that does not exist yet is
// joined as written, because the go command creates it as a directory, and a
// `..` after it climbs back out of it. existing requires every component to
// exist. It answers false when a component cannot be read, is a link that
// cannot be resolved, or is missing where existing asks. The loop is bounded
// by the components of name.
func physicalPath(base, name string, existing bool) (string, bool) {
	resolved := filepath.Clean(base)
	missing := false
	for part := range strings.SplitSeq(name, "/") {
		if part == "" || part == "." {
			continue
		}
		if part == ".." {
			resolved = filepath.Dir(resolved)
			continue
		}
		next := filepath.Join(resolved, part)
		if missing {
			resolved = next
			continue
		}
		info, err := os.Lstat(next)
		if errors.Is(err, fs.ErrNotExist) && !existing {
			missing, resolved = true, next
			continue
		}
		if err != nil {
			return "", false
		}
		if info.Mode()&fs.ModeSymlink == 0 {
			resolved = next
			continue
		}
		target, err := filepath.EvalSymlinks(next)
		if err != nil {
			return "", false
		}
		resolved = target
	}
	return resolved, true
}

// wholeTreePattern reports whether a package pattern names every package of
// the module: `./...`, `...` or `all`, or a `/...` pattern whose directory,
// cleaned, is the module root, by import path or by path.
func wholeTreePattern(checkout buildCheckout, args goBuildArgs, pattern string) bool {
	if oneOf(pattern, "./...", "...", "all") {
		return true
	}
	base, recursive := strings.CutSuffix(pattern, "/...")
	if !recursive {
		return false
	}
	dir, ok := goBuildPackageDir(checkout, args, recursiveBase(base))
	if !ok {
		return false
	}
	return dir == filepath.Clean(checkout.root)
}

// recursiveBase answers the directory part of a `/...` pattern: `//...`
// leaves "", which names the filesystem root.
func recursiveBase(base string) string {
	if base == "" {
		return "/"
	}
	return base
}

// goBuildPackage is one package a build names, with the directory it resolves
// to and whether it is a main package.
type goBuildPackage struct {
	name string
	dir  string
	main bool
}

// judgeGoBuild decides what one `go build` writes, and the advice its refusal
// carries. The go command writes a binary only for a single main package: a
// single non-main package and several packages are compiled and discarded
// ("go help build"), so those pass. Everything the hook cannot place is
// judged as a build that could write a binary, because a guard that cannot
// see what it builds must not clear it: a pattern or a package it cannot
// resolve to a directory of the checkout, an output it cannot resolve, a
// build that resolves no package at all, and a build after a cd with no
// output under bin/.
func judgeGoBuild(checkout buildCheckout, args goBuildArgs) (goBuildOutcome, string) {
	for _, pattern := range args.packages {
		if wholeTreePattern(checkout, args, pattern) {
			return goBuildWholeTree, adviceLine("`"+pattern+"` compiles every package of the module, which is the release check's job, never a session's.") +
				adviceLine("Use: ./le repo changed scope print both, then go build only the packages it names")
		}
		base, recursive := strings.CutSuffix(pattern, "/...")
		if !recursive {
			continue
		}
		if _, ok := goBuildPackageDir(checkout, args, recursiveBase(base)); !ok {
			return goBuildBinary, adviceLine("`"+pattern+"` names no package directory under the checkout, so the hook cannot tell what it compiles.") +
				adviceLine("Use: name the packages from the checkout root: go build ./internal/<area>/...")
		}
	}
	use := adviceLine("Use: " + strings.TrimSpace("go build -o bin/"+outputName(checkout, args)+" "+strings.Join(args.packages, " ")))
	if args.hasOutput {
		inBin, resolved := outputInBin(checkout, args)
		if inBin {
			return goBuildAllowed, ""
		}
		if !resolved {
			return goBuildBinary, adviceLine("-o "+args.output+" cannot be resolved: it holds a character the shell expands, a link the hook cannot resolve, or it is relative to a directory a cd moved to that the hook cannot follow.") +
				adviceLine("Use: run it from the checkout root, or name the directory with -C: go build -C <dir> -o <path to bin/> <packages>")
		}
		return goBuildBinary, adviceLine("-o "+args.output+" writes the result outside the checkout's bin/.") + use
	}
	if args.moved {
		return goBuildBinary, adviceLine("It runs after a cd, so the hook cannot tell which directory its packages are in.") +
			adviceLine("Use: run it from the checkout root, or name the directory with -C: go build -C <dir> <packages>")
	}
	if len(args.packages) == 0 {
		return goBuildBinary, adviceLine("It names no package, so it builds the current directory into it.") +
			adviceLine("Use: go build -o bin/<name> ./cmd/<name> for a binary, or name the packages to compile: go build ./internal/<area>/")
	}
	packages, unresolved := goBuildPackages(checkout, args)
	if unresolved != "" {
		return goBuildBinary, adviceLine(unresolved+", so the hook cannot tell whether it builds a binary.") + use
	}
	if len(packages) == 0 {
		return goBuildBinary, adviceLine("No Go file of "+strings.Join(args.packages, " ")+" is built under its tags, so the hook cannot tell what it writes.") +
			adviceLine("Use: name the tags the packages need: go build -tags <tags> "+strings.Join(args.packages, " "))
	}
	if len(packages) != 1 {
		return goBuildAllowed, ""
	}
	if !packages[0].main {
		return goBuildAllowed, ""
	}
	return goBuildBinary, adviceLine(packages[0].name+" is a main package, so the binary lands in the working directory.") +
		adviceLine("Use: go build -o bin/"+filepath.Base(packages[0].dir)+" "+packages[0].name)
}

// adviceLine renders one line of a build refusal's advice.
func adviceLine(text string) string {
	return "  " + red + "→" + reset + " " + text + "\n"
}

// outputName answers the binary name the advice puts under bin/: the last
// element of the one package named, or the placeholder when there are several.
func outputName(checkout buildCheckout, args goBuildArgs) string {
	if len(args.packages) != 1 {
		return "<name>"
	}
	if dir, ok := goBuildPackageDir(checkout, args, args.packages[0]); ok {
		return filepath.Base(dir)
	}
	return filepath.Base(args.packages[0])
}

// goBuildPackageLimit is how many packages goBuildPackages reads before it
// stops: two already mean the go command writes nothing.
const goBuildPackageLimit = 2

// goBuildPackages answers the packages the build names, at most
// goBuildPackageLimit, or the reason one cannot be resolved. A directory
// that belongs to a module nested below the checkout root is not resolved,
// and neither is a `/...` pattern under the checkout's tmp/, where scratch Go
// files collect: a walk never reads there, from its start directory either.
func goBuildPackages(checkout buildCheckout, args goBuildArgs) ([]goBuildPackage, string) {
	reader := packageReader{checkout: checkout, args: args, context: build.Default, seen: map[string]bool{}}
	reader.context.BuildTags = args.tags
	scratch := filepath.Join(checkout.root, "tmp")
	for _, pattern := range args.packages {
		base, recursive := strings.CutSuffix(pattern, "/...")
		if recursive {
			base = recursiveBase(base)
		}
		dir, ok := goBuildPackageDir(checkout, args, base)
		if !ok {
			return nil, "`" + pattern + "` names no package directory under the checkout"
		}
		if strings.Contains(base, "...") {
			return nil, "`" + pattern + "` is a pattern the hook does not expand"
		}
		if _, under := insideCheckout(scratch, dir); under && recursive {
			// A walk never reads the scratch tree, from its start either.
			return nil, "`" + pattern + "` is under tmp/, where scratch Go files collect"
		}
		if nestedModule(checkout.root, dir) {
			return nil, "`" + pattern + "` belongs to a module nested below the checkout root"
		}
		if !recursive {
			if problem := reader.add(dir, pattern); problem != "" {
				return nil, problem
			}
			continue
		}
		if problem := reader.walk(dir, pattern); problem != "" {
			return nil, problem
		}
		if len(reader.packages) == goBuildPackageLimit {
			break
		}
	}
	return reader.packages, ""
}

// nestedModule reports whether a directory inside the checkout, or one of its
// parents below the root, holds a go.mod of its own. The walk up is bounded by
// the directory's depth below the root.
func nestedModule(root, dir string) bool {
	root = filepath.Clean(root)
	for current := filepath.Clean(dir); current != root; current = filepath.Dir(current) {
		if holdsGoMod(current) {
			return true
		}
		if filepath.Dir(current) == current {
			return false
		}
	}
	return false
}

// holdsGoMod reports whether a directory holds a go.mod file.
func holdsGoMod(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "go.mod"))
	return err == nil
}

// packageReader collects the packages of one build, under its tags.
type packageReader struct {
	checkout buildCheckout
	args     goBuildArgs
	context  build.Context
	seen     map[string]bool
	packages []goBuildPackage
	// visited counts the directories walk has read, bounded by goBuildWalkMax.
	visited int
}

// add reads one directory as a package. A directory with no Go file the tags
// select is no package, and the go command compiles nothing there.
func (reader *packageReader) add(dir, name string) string {
	if len(reader.packages) == goBuildPackageLimit {
		return ""
	}
	if reader.seen[dir] {
		return ""
	}
	found, err := reader.context.ImportDir(dir, 0)
	if _, none := errors.AsType[*build.NoGoError](err); none {
		return ""
	}
	if err != nil {
		return "`" + name + "` could not be read as a package: " + err.Error()
	}
	reader.seen[dir] = true
	reader.packages = append(reader.packages, goBuildPackage{name: name, dir: dir, main: found.IsCommand()})
	return ""
}

// walk reads the directories a `/...` pattern reaches below dir, the way the
// go command walks them, and stops once goBuildPackageLimit packages are
// found. It skips testdata, vendor, every directory whose name starts with
// `.` or `_`, a directory that holds its own go.mod (another module, which
// the go command does not enter either), and the checkout's tmp/. It refuses
// a walk that visits more than goBuildWalkMax directories.
func (reader *packageReader) walk(dir, pattern string) string {
	problem := ""
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			return nil
		}
		if path != dir && reader.skips(path, entry.Name()) {
			return filepath.SkipDir
		}
		reader.visited++
		if reader.visited > goBuildWalkMax {
			problem = "`" + pattern + "` reaches more than " + strconv.Itoa(goBuildWalkMax) + " directories before two packages"
			return filepath.SkipAll
		}
		problem = reader.add(path, packageName(reader.args, path))
		if problem != "" {
			return filepath.SkipAll
		}
		if len(reader.packages) == goBuildPackageLimit {
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil {
		return "`" + pattern + "` could not be walked: " + err.Error()
	}
	return problem
}

// skips reports whether a `/...` pattern leaves a directory out.
func (reader *packageReader) skips(path, name string) bool {
	if anyPrefix(name, ".", "_") {
		return true
	}
	if oneOf(name, testdataDir, "vendor") {
		return true
	}
	if filepath.Clean(path) == filepath.Join(reader.checkout.root, "tmp") {
		return true
	}
	return holdsGoMod(path)
}

// packageName answers a walked package directory as the relative path a
// caller types, `./cmd/ze`.
func packageName(args goBuildArgs, dir string) string {
	base, ok := goBuildBase(args)
	if !ok {
		return dir
	}
	relative, err := filepath.Rel(base, dir)
	if err != nil {
		return dir
	}
	return "./" + filepath.ToSlash(relative)
}

// goBuildPackageDir resolves one package argument to its directory: a
// relative or absolute path, or an import path inside the checkout's module.
// It answers false for anything else, a standard-library package and a word
// holding an unexpanded variable included, for a path outside the checkout,
// for a relative path when the hook cannot tell the directory it is relative
// to, and for a path that is not a directory.
func goBuildPackageDir(checkout buildCheckout, args goBuildArgs, pkg string) (string, bool) {
	var dir string
	switch {
	case pkg == "." || pkg == ".." || anyPrefix(pkg, "./", "../"):
		base, ok := goBuildBase(args)
		if !ok {
			return "", false
		}
		dir = filepath.Join(base, pkg)
	case filepath.IsAbs(pkg):
		dir = filepath.Clean(pkg)
	default:
		if checkout.module == "" {
			return "", false
		}
		if pkg == checkout.module {
			dir = checkout.root
			break
		}
		relative, inside := strings.CutPrefix(pkg, checkout.module+"/")
		if !inside {
			return "", false
		}
		dir = filepath.Join(checkout.root, filepath.FromSlash(relative))
	}
	if _, inside := insideCheckout(checkout.root, dir); !inside {
		return "", false
	}
	info, err := os.Stat(dir)
	if err != nil {
		return "", false
	}
	return filepath.Clean(dir), info.IsDir()
}

// modulePath answers the module path the checkout's go.mod declares, and ""
// when it cannot be read, which resolves no import path.
func modulePath(root string) string {
	body, err := os.ReadFile(filepath.Join(root, "go.mod")) //nolint:gosec // the go.mod of the checkout root the hook judges
	if err != nil {
		return ""
	}
	for line := range strings.SplitSeq(string(body), "\n") {
		if module, found := strings.CutPrefix(strings.TrimSpace(line), "module "); found {
			return strings.Trim(strings.TrimSpace(module), `"`)
		}
	}
	return ""
}

// goBuildLine judges every `go build` a line runs. Its text judgment
// (goBuildText) is the verdict, and the words are read instead only on a
// reader+build line: one whose every command is a reader its judge admits
// (readerCommand) or a build, so nothing but the builds can run a build, and
// each one is judged by its arguments. A build is a command whose words,
// after any GOFLAGS assignments, open with `go build`. The text judgment
// stays the verdict for every other line: one outside the shell syntax the
// hook models (modeledSyntax), whose commands the splitter need not see as
// bash runs them; one holding a shell, a launcher (`time`, `env`, `xargs`),
// an interpreter or any command the reader set does not hold, which may run
// a build its quotes hide; a build behind another assignment (goBuildAt),
// with a word the shell expands, or with a -toolexec, -overlay or -modfile
// flag (goBuildArgs), which the text judgment then refuses when the line's
// text names it; and
// a line with a redirection that writes a file, with or without a build,
// because a reader could write the package the hook reads before the build
// compiles it, or the configuration a later reader runs. A
// literal cd earlier on the line moves the directory relative packages and
// outputs resolve from, when cdFollowed shows it runs before the build in the
// same shell.
func goBuildLine(root, command string) (goBuildOutcome, string) {
	if !modeledSyntax(command) {
		return goBuildText(command)
	}
	line, parsed := lineCommands(command)
	if !parsed {
		return goBuildText(command)
	}
	if line.substitutes {
		return goBuildText(command)
	}
	builds, writes := 0, false
	for index, words := range line.commands {
		segment := shellWords(line.segments[index])
		writes = writes || !writesOnlyDevNull(segment)
		if readerCommand(words, line.expands[index]) {
			continue
		}
		if _, found := goBuildAt(beforeRedirection(segment)); found {
			// A word the shell expands (`$_`, `GOFLAGS=$X`, a glob) is not the
			// text the hook reads: it may carry a -toolexec flag or another
			// package, so the build is judged by its text.
			// An -o value the shell expands can leave bin/ (`bin/$X` with X
			// holding `../`), and the text judgment would allow it for its
			// `-o bin/` spelling, so it is refused here, stricter than HEAD.
			if outputExpands(words, line.expands[index]) {
				return goBuildBinary, strings.TrimSuffix(adviceLine("The -o path holds a word the shell expands, so the hook cannot tell whether the binary lands under bin/.")+
					adviceLine("Use: go build -o bin/<name> ./cmd/<name>, with a literal name"), "\n")
			}
			if slices.Contains(line.expands[index], true) {
				return goBuildText(command)
			}
			builds++
			continue
		}
		return goBuildText(command)
	}
	// A line that writes a file is no reader line, whether or not it builds:
	// a reader could write the package the hook reads, or the configuration a
	// later reader runs.
	if writes {
		return goBuildText(command)
	}
	if builds == 0 {
		return goBuildAllowed, ""
	}
	physicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return goBuildBinary, strings.TrimSuffix(adviceLine("The checkout root cannot be resolved ("+err.Error()+"), so the hook cannot tell where the build writes.")+
			adviceLine("Use: go build -o bin/<name> ./cmd/<name>, from the checkout root"), "\n")
	}
	checkout := buildCheckout{root: root, physical: physicalRoot, module: modulePath(root)}
	// A group or a subshell can undo a cd before the build runs, so on a line
	// holding one no cd is followed.
	grouped := strings.ContainsAny(command, "(){}")
	cwd, physical, moved := filepath.Clean(root), physicalRoot, false
	for index, words := range line.commands {
		if words[0] == "cd" {
			moved = true
			if grouped || !cdFollowed(line.separators, index) {
				cwd, physical = "", ""
				continue
			}
			cwd, physical = cdTarget(cwd, words[1:], line.expands[index][1:])
			continue
		}
		segment := beforeRedirection(shellWords(line.segments[index]))
		at, found := goBuildAt(segment)
		if !found {
			continue
		}
		args := parseGoBuild(append(goFlags(segment[:at]), segment[at+2:]...))
		if args.textJudged {
			return goBuildText(command)
		}
		args.cwd, args.physical, args.moved = cwd, physical, moved
		outcome, advice := judgeGoBuild(checkout, args)
		if outcome != goBuildAllowed {
			return outcome, strings.TrimSuffix(advice, "\n")
		}
	}
	return goBuildAllowed, ""
}

// outputExpands reports whether a build command's -o value, given as the next
// word or joined with `=`, one dash or two, holds a word the shell expands.
func outputExpands(words []string, expands []bool) bool {
	for index, word := range words {
		if !strings.HasPrefix(word, "-") {
			continue
		}
		name, _, joined := strings.Cut(strings.TrimPrefix(word, "-"), "=")
		if strings.TrimPrefix(name, "-") != "o" {
			continue
		}
		if joined {
			if expands[index] {
				return true
			}
			continue
		}
		if index+1 < len(words) && expands[index+1] {
			return true
		}
	}
	return false
}

// writesOnlyDevNull reports whether a command's redirections write nothing
// but /dev/null or another descriptor: `2>&1`, `>&2`, `>/dev/null`,
// `2> /dev/null`, `&>/dev/null`. `>& file` writes the file. The words are
// shellWords of the command's text, where a redirection keeps its `>`.
func writesOnlyDevNull(words []string) bool {
	for index := 0; index < len(words); index++ {
		at := strings.IndexByte(words[index], '>')
		if at < 0 {
			continue
		}
		target := strings.TrimLeft(words[index][at:], ">|")
		if target == "" || target == "&" {
			index++
			if index == len(words) {
				return false
			}
			target += words[index]
		}
		if target == "/dev/null" {
			continue
		}
		if target == "&/dev/null" {
			continue
		}
		descriptor, dup := strings.CutPrefix(target, "&")
		if !dup {
			return false
		}
		if descriptor == "" {
			return false
		}
		if strings.Trim(descriptor, "0123456789-") != "" {
			return false
		}
	}
	return true
}

// goBuildAt answers where `go build` opens a command's words, after any
// leading GOFLAGS assignments, and false when the command is no build. Any
// other assignment makes the command no build, so its line is judged by its
// text: a variable can name a program the go command runs (CC, CXX,
// GOTOOLCHAIN) or change which files a package holds (GOOS, GOARCH,
// CGO_ENABLED, GOEXPERIMENT), and the hook reads the packages under none of
// them.
func goBuildAt(words []string) (int, bool) {
	at := 0
	for at < len(words) && shellAssignment(words[at]) {
		if !strings.HasPrefix(words[at], "GOFLAGS=") {
			return 0, false
		}
		at++
	}
	if at+1 >= len(words) {
		return 0, false
	}
	if filepath.Base(words[at]) != "go" {
		return 0, false
	}
	return at, words[at+1] == "build"
}

// shellAssignment reports whether a word is a `NAME=value` assignment, which
// the shell reads as an environment setting for the command that follows.
func shellAssignment(word string) bool {
	name, _, found := strings.Cut(word, "=")
	if !found || name == "" {
		return false
	}
	for index, character := range name {
		if character == '_' || ('A' <= character && character <= 'Z') || ('a' <= character && character <= 'z') {
			continue
		}
		if index != 0 && '0' <= character && character <= '9' {
			continue
		}
		return false
	}
	return true
}

// cdFollowed reports whether the cd at index certainly runs, in the shell
// that runs the commands after it: it starts the line or follows `;` or a
// newline, and `;`, `&&` or a newline ends it. A cd after `&&` or `||` may
// not run; one before `||` leaves the next command where the shell was when
// it succeeds; and one in a pipeline or behind `&` runs in a subshell.
func cdFollowed(separators []string, index int) bool {
	if index > 0 && !oneOf(separators[index-1], ";", "\n") {
		return false
	}
	return oneOf(separators[index], ";", "&&", "\n")
}

// searchPermission is the access(2) mode bit that asks whether a directory
// can be entered (X_OK).
const searchPermission = 0x1

// cdTarget answers the logical and the physical directory a cd moves to from
// the logical directory cwd, and "" for both when the hook cannot tell: an
// unknown cwd, a CDPATH in the environment, no operand or several, `cd -`,
// `cd -P` (which resolves links before it reads `..`), an operand the shell
// expands, and a target that is not a directory the shell can enter, where
// the cd fails and the shell stays where it was. A cd is logical by default:
// it joins the operand to $PWD and cleans `..` as text, and the physical
// directory is that path with its links resolved.
func cdTarget(cwd string, arguments []string, expands []bool) (string, string) {
	if cwd == "" {
		return "", ""
	}
	if os.Getenv("CDPATH") != "" {
		return "", ""
	}
	operands := make([]string, 0, 1)
	for index, argument := range arguments {
		if expands[index] {
			return "", ""
		}
		if oneOf(argument, "-L", "-e", "-@", "--") {
			continue
		}
		operands = append(operands, argument)
	}
	if len(operands) != 1 {
		return "", ""
	}
	target := operands[0]
	if strings.HasPrefix(target, "-") {
		return "", ""
	}
	if strings.ContainsAny(target, expandedPathCharacters+"\\") {
		return "", ""
	}
	logical := target
	if !filepath.IsAbs(logical) {
		logical = filepath.Join(cwd, logical)
	}
	physical, err := filepath.EvalSymlinks(logical)
	if err != nil {
		return "", ""
	}
	info, err := os.Stat(physical)
	if err != nil {
		return "", ""
	}
	if !info.IsDir() {
		return "", ""
	}
	if syscall.Access(physical, searchPermission) != nil {
		return "", ""
	}
	return logical, physical
}

// goBuildText judges a line by its text: one that names `go build` and a
// -toolexec, -overlay or -modfile flag is refused whatever its output, one
// that names `go build` with a `...` pattern is a whole-tree build, and one
// that names `go build` and no output under bin/ or a session bin/ directory
// is refused.
func goBuildText(command string) (goBuildOutcome, string) {
	if !goBuildCall.MatchString(command) {
		return goBuildAllowed, ""
	}
	if match := goBuildToolchainFlag.FindStringSubmatch(command); match != nil {
		flag := "-" + match[1]
		return goBuildToolchain, strings.TrimSuffix(adviceLine("`"+flag+"` "+goBuildToolchainEffect[match[1]]+
			", so the hook cannot tell what the build runs or writes. Drop the flag and build into bin/.")+
			adviceLine("Use: go build -o bin/<name> ./cmd/<name>"), "\n")
	}
	if goBuildTextWholeTree.MatchString(command) {
		// The text cannot tell `./...` from `./internal/x/...`, so the advice
		// names a tree of packages and not the module's whole tree.
		return goBuildWholeTree, strings.TrimSuffix(adviceLine("A `...` pattern compiles every package under its directory. "+
			"A session builds only the packages its change reaches, and the module's whole tree is the release check's job.")+
			adviceLine("Use: ./le repo changed scope print both, then go build only the packages it names"), "\n")
	}
	if goBuildBin.MatchString(command) {
		return goBuildAllowed, ""
	}
	if goBuildSession.MatchString(command) {
		return goBuildAllowed, ""
	}
	return goBuildBinary, strings.TrimSuffix(adviceLine("Use: go build -o bin/<name> ./cmd/<name>"), "\n")
}
