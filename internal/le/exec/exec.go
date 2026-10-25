// Design: docs/guide/developer-setup.md -- le exec: the launcher the tool shims run
// Related: register.go -- the registration of `le exec`

// Package leexec is `le exec <path> [args...]`: it points a program's caches at
// the one per-user cache and then becomes that program.
//
// gopls, golangci-lint and staticcheck each keep a cache of their own under
// the platform's default folder, ~/Library/Caches on macOS. A shim that
// `le setup` writes in its shim directory runs `le exec` on the real tool instead, so the
// tool inherits GOCACHE, GOPLSCACHE, GOLANGCI_LINT_CACHE and STATICCHECK_CACHE
// under the per-user target (scratch.CacheTarget), the same tree every le
// action builds into. le changes its own environment and replaces itself with
// the program (crashlog.Exec), so no le process stays behind and the
// program's exit status is the caller's.
package leexec

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/internal/core/crashlog"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/go/stdpatch/gen"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	"github.com/ze-software/ze/internal/le/scratch"
)

// ShimMarker is the line every tool shim `le setup` writes carries, within its
// first shimMarkerWindow bytes. `le exec` refuses a file holding it: a shim
// handed back to `le exec` would start `le exec` again, forever.
const ShimMarker = "# ze-le-exec-shim: written by ./le setup; le exec refuses to run this file"

// shimMarkerWindow bounds how much of a candidate file is read for the marker.
// A shim is a few lines long and carries the marker on its second line, while
// the real tools are binaries of tens of megabytes, so the check reads one page
// and never the whole program.
const shimMarkerWindow = 4096

// cacheVariables maps each cache variable `le exec` sets to the directory
// under the per-user target that holds it.
var cacheVariables = [...]struct {
	name      string
	directory string
}{
	{name: "GOCACHE", directory: "go-cache"},
	{name: "GOPLSCACHE", directory: "gopls"},
	{name: "GOLANGCI_LINT_CACHE", directory: "golangci-lint-cache"},
	{name: "STATICCHECK_CACHE", directory: "staticcheck-matrix"},
}

// usage is the grammar `le exec` prints for a help request or a bare call.
const usage = "usage: le exec <path> [args...]\n" +
	"  Runs <path> with every argument after it untouched, with GOCACHE, GOPLSCACHE,\n" +
	"  GOLANGCI_LINT_CACHE and STATICCHECK_CACHE under the per-user cache. A bare\n" +
	"  name is looked up on PATH, skipping the shims ./le setup writes. Inside a Ze\n" +
	"  checkout the program also gets Ze's patched GOROOT; anywhere else it keeps\n" +
	"  the caller's.\n"

// run is answer with its streams named, so a test reaches every refusal.
func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		write(stderr, usage)
		return 2
	}
	first := args[0]
	if first == "--help" {
		write(stdout, usage)
		return 0
	}
	if first == "-h" {
		write(stdout, usage)
		return 0
	}
	// ai/rules/cli.md: a dash-leading token where a value is expected is an
	// option nobody declared, so it is refused by name and never run.
	if strings.HasPrefix(first, "-") {
		var message textbuf.Buffer
		message.Str("le exec: ").Str(first).Str(" is not an option of le exec, and a program path never begins with a dash.\n").Str(usage)
		write(stderr, message.String())
		return 2
	}
	cwd, err := os.Getwd()
	if err != nil {
		var message textbuf.Buffer
		message.Str("le exec: cannot read the working directory: ").Str(err.Error()).
			Str("\nle exec decides from it whether the program gets Ze's patched GOROOT.\ncd to an existing directory and run the command again.\n")
		write(stderr, message.String())
		return 1
	}
	program, err := prepare(cwd, args, os.Environ(), stderr)
	if err != nil {
		return 1
	}
	err = replace(program)
	var message textbuf.Buffer
	message.Str("le exec: ").Str(program.program).Str(": the system refused to run it: ").Str(err.Error()).
		Str("\nThe file exists and is executable, so the kernel rejected its format or its interpreter.\nRun it directly to see the system's own error, then reinstall the tool and run ./le setup.\n")
	write(stderr, message.String())
	return 1
}

// write puts text on a stream le has nothing behind to report a failure on.
func write(stream io.Writer, text string) {
	_, _ = io.WriteString(stream, text) //nolint:errcheck // the operator's stream is the last channel
}

// launch is one program ready to replace le: the resolved file, its argv with
// the name the caller typed as argv[0], and its whole environment.
type launch struct {
	program string
	argv    []string
	environ []string
}

// errRefused says the refusal was already written for the operator.
var errRefused = errors.New("le exec refused")

// prepare resolves the program args name and builds its environment, for a
// caller working in cwd. environ is this le's environment, which
// activatePatchedGOROOT already changed. A refusal is written to stderr and
// answered as errRefused.
func prepare(cwd string, args, environ []string, stderr io.Writer) (launch, error) {
	environ, err := programEnvironment(cwd, environ)
	if err != nil {
		return launch{}, refuse(stderr, "le exec: cannot build the program's environment: "+err.Error()+".",
			"le exec starts nothing it cannot give its caches and the right GOROOT.",
			"Fix the cause named above, then run the command again; ./le setup check reports the host's state.")
	}
	program, err := resolve(args[0], cwd, lookup(environ, "PATH"), stderr)
	if err != nil {
		return launch{}, err
	}
	return launch{program: program, argv: args, environ: environ}, nil
}

// programEnvironment is environ with the four cache variables pointed under
// the per-user target, each directory created, and GOROOT, GOTOOLCHAIN and
// PATH given back to the caller unless cwd lies in a Ze checkout.
//
// O-5 of spec-le-one-go-cache-per-host: the patched GOROOT is Ze's toolchain
// and belongs to Ze's code alone. Inside the checkout gopls must analyze the
// crypto/tls Ze compiles; anywhere else it must analyze the caller's own Go.
// The test is the working directory, never lepath.Root, which falls back to
// the location of le itself and so always finds a checkout.
func programEnvironment(cwd string, environ []string) ([]string, error) {
	if _, inside := lepath.CheckoutContaining(cwd); !inside {
		root, err := lepath.Root()
		if err == nil {
			environ, err = gen.CallerEnvironment(root, environ)
			if err != nil {
				return nil, fmt.Errorf("restore the caller's GOROOT: %w", err)
			}
		}
		// lepath.Root failing means activatePatchedGOROOT found no checkout and
		// changed nothing, so environ is the caller's already.
	}
	target, err := scratch.CacheTarget(environ)
	if err != nil {
		return nil, fmt.Errorf("derive the per-user cache: %w", err)
	}
	for _, variable := range cacheVariables {
		directory := filepath.Join(target, variable.directory)
		if err := os.MkdirAll(directory, 0o750); err != nil {
			return nil, fmt.Errorf("create %s for %s: %w", directory, variable.name, err)
		}
		environ = append(withoutName(environ, variable.name), variable.name+"="+directory)
	}
	return environ, nil
}

// resolve answers the file name names: the path itself when it holds a
// separator, made absolute against cwd, else the first PATH entry holding an
// executable file of that name that is not a shim. Anything else is refused.
func resolve(name, cwd, searchPath string, stderr io.Writer) (string, error) {
	if strings.ContainsRune(name, filepath.Separator) {
		return resolvePath(name, cwd, stderr)
	}
	for _, directory := range filepath.SplitList(searchPath) {
		if directory == "" {
			continue // an empty entry means the working directory, which le never searches
		}
		candidate := filepath.Join(directory, name)
		if executableFile(candidate) != nil {
			continue
		}
		shim, err := IsShim(candidate)
		if err != nil {
			continue
		}
		if shim {
			continue
		}
		return candidate, nil
	}
	return "", refuse(stderr, "le exec: "+name+": no executable of that name on PATH, shims skipped.",
		"A shim runs le exec on the real tool, so the tool itself has to be installed outside the shim directory.",
		"Install the tool, then run ./le setup in the Ze checkout to rewrite the shims with its path.")
}

// resolvePath judges a program the caller named by path.
func resolvePath(name, cwd string, stderr io.Writer) (string, error) {
	program := name
	if !filepath.IsAbs(program) {
		program = filepath.Join(cwd, program)
	}
	if err := executableFile(program); err != nil {
		return "", refuse(stderr, "le exec: "+name+": "+err.Error()+".",
			"The tool was moved or removed since ./le setup recorded its path in the shim.",
			"Run ./le setup in the Ze checkout to rewrite the shims with the current tool paths.")
	}
	shim, err := IsShim(program)
	if err != nil {
		return "", refuse(stderr, "le exec: "+name+": cannot read it: "+err.Error()+".",
			"le exec reads the start of the file to make sure it is not one of its own shims.",
			"Run ./le setup in the Ze checkout to rewrite the shims with the current tool paths.")
	}
	if shim {
		return "", refuse(stderr, "le exec: "+name+" is a shim ./le setup wrote, not the real tool.",
			"Running it would start le exec on itself again, in a loop.",
			"Run ./le setup in the Ze checkout to rewrite the shims with the real tool paths.")
	}
	return program, nil
}

// executableFile answers nil when path is a regular file with an execute bit,
// and otherwise the reason it is not, in words for the operator.
func executableFile(path string) error {
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return errors.New("no such file")
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return errors.New("not a regular file")
	}
	if info.Mode().Perm()&0o111 == 0 {
		return errors.New("not executable")
	}
	return nil
}

// IsShim answers whether the first shimMarkerWindow bytes of path hold
// ShimMarker. `le setup` asks it too, so it never overwrites a file that is
// not one of its shims, and never takes a shim for the real tool.
func IsShim(path string) (bool, error) {
	file, err := os.Open(path) //nolint:gosec // G304: the operator names the program; this reads its first page
	if err != nil {
		return false, err
	}
	defer file.Close() //nolint:errcheck // read-only; the read error is the one that matters

	head := make([]byte, shimMarkerWindow)
	count, err := io.ReadFull(file, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return false, err
	}
	return bytes.Contains(head[:count], []byte(ShimMarker)), nil
}

// refuse writes the three-line refusal (what is wrong, why, the fix) and
// answers errRefused.
func refuse(stderr io.Writer, what, why, fix string) error {
	var message textbuf.Buffer
	message.Str(what).Byte('\n').Str(why).Byte('\n').Str(fix).Byte('\n')
	_, _ = stderr.Write(message.Bytes()) //nolint:errcheck // stderr is the last channel; nothing remains to report a failure on
	return errRefused
}

// replace becomes the program. It returns only when the execve was refused.
func replace(program launch) error {
	return crashlog.Exec(program.program, program.argv, program.environ)
}

// lookup answers the last value environ gives name, the one an executed
// process sees.
func lookup(environ []string, name string) string {
	value := ""
	for _, entry := range environ {
		key, rest, found := strings.Cut(entry, "=")
		if found && key == name {
			value = rest
		}
	}
	return value
}

// withoutName answers environ with every assignment to name removed.
func withoutName(environ []string, name string) []string {
	kept := make([]string, 0, len(environ))
	for _, entry := range environ {
		key, _, _ := strings.Cut(entry, "=")
		if key != name {
			kept = append(kept, entry)
		}
	}
	return kept
}
