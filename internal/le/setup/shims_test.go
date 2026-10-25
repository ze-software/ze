// Tests for the three cache rows `le setup` adds: the tool shims, the PATH
// directory they go in, and the `go env GOCACHE` setting.
//
// Every case runs against a temporary HOME, a temporary directory of real
// tools and a temporary checkout, so nothing outside the test's own
// directories is read or written. External commands go through the recorder.

package setup

import (
	"context"
	"errors"
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	leexec "github.com/ze-software/ze/internal/le/exec"
	"github.com/ze-software/ze/internal/le/go/stdpatch/gen"
)

// shimHost is one machine for the shim rows: a HOME, a checkout, a directory
// of real tools, and the PATH the run sees.
type shimHost struct {
	home  string
	root  string
	tools string
	env   map[string]string
	rec   *recorder
	setup *Setup
}

// newShimHost builds a host whose real gopls, golangci-lint and staticcheck
// sit in one directory on PATH, with ~/bin absent from PATH.
func newShimHost(t *testing.T) *shimHost {
	t.Helper()
	host := &shimHost{home: t.TempDir(), root: t.TempDir(), tools: t.TempDir(), rec: rootShell("go")}
	writeRealTools(t, host.tools)
	writePatchSet(t, host.root)
	host.env = map[string]string{
		"PATH":           host.tools + ":/usr/bin:/bin",
		"SHELL":          "/bin/zsh",
		"XDG_CACHE_HOME": filepath.Join(host.home, ".cache"),
	}
	host.setup = &Setup{
		Root:  host.root,
		Home:  host.home,
		Shell: host.rec.shell(),
		Env:   func(key string) string { return host.env[key] },
	}
	return host
}

// writeRealTools puts an executable stand-in for each shimmed tool in dir.
func writeRealTools(t *testing.T, dir string) {
	t.Helper()
	for _, tool := range shimTools() {
		path := filepath.Join(dir, tool.Probe[0])
		if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
}

// writePatchSet gives root a one-file stdpatch patch set, so callerPath can
// tell a patched tree from a directory of the caller's the way it does in a
// real checkout.
func writePatchSet(t *testing.T, root string) {
	t.Helper()
	dir := filepath.Join(root, filepath.FromSlash(gen.PatchDir))
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	patch := "--- a/src/example/added.go\n+++ b/src/example/added.go\n"
	if err := os.WriteFile(filepath.Join(dir, "0001-example.patch"), []byte(patch), 0o600); err != nil {
		t.Fatalf("write patch: %v", err)
	}
}

// homeBin answers ~/bin, where the shims go when no PATH entry qualifies.
func (h *shimHost) homeBin() string { return filepath.Join(h.home, "bin") }

// visitShims runs the shim rows and the shim-dir row, the way Run does.
func (h *shimHost) visitShims() (*Report, map[string]Outcome) {
	report := &Report{}
	h.setup.visitShims(report)
	rows := map[string]Outcome{}
	for _, outcome := range report.Outcomes {
		rows[outcome.Name] = outcome
	}
	return report, rows
}

// homeDir creates dir under the host's HOME and answers its path.
func (h *shimHost) homeDir(t *testing.T, parts ...string) string {
	t.Helper()
	dir := filepath.Join(append([]string{h.home}, parts...)...)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	return dir
}

// writeShimFile puts a le-marked shim for name in dir, the shape an earlier
// run left behind.
func writeShimFile(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+leexec.ShimMarker+"\nexit 0\n"), 0o700); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

// shimLine answers the exec line a shim for name must carry.
func (h *shimHost) shimLine(name, toolsDir string) string {
	return "exec " + shellSingleQuote(filepath.Join(h.root, "le")) + " exec " +
		shellSingleQuote(filepath.Join(toolsDir, name)) + ` "$@"`
}

// readShim answers the shim file's text and mode.
func readShim(t *testing.T, path string) (string, os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	data, err := os.ReadFile(path) //nolint:gosec // G304: a path under the test's own temporary HOME
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data), info.Mode().Perm()
}

// TestSetupWritesShimsAndReportsPath: an install run writes one executable
// shim per shimmed tool row into ~/bin, each starting `<checkout>/le exec` on
// the real tool with every argument, and reports ~/bin's PATH position as a
// step only the developer can finish.
// VALIDATES: AC-10 (shims written, executable, rewritten on rerun), AC-11
// (PENDING with the zsh and NuShell lines, no shell file written), R-2 (a shim
// whose le is gone says so and names ./le setup), and that a non-shim file in
// ~/bin is never overwritten.
// PREVENTS: a gopls started by an editor writing its cache under
// ~/Library/Caches because no shim stood in front of it.
func TestSetupWritesShimsAndReportsPath(t *testing.T) {
	t.Run("an install run writes the shims through Run", func(t *testing.T) {
		host := newShimHost(t)
		host.setup.GOOS = osDarwin
		host.setup.Gopls = func() Result { return Result{Out: "Now Function 20:6-20:9\n"} }
		host.setup.Bindable = func(string) bool { return true }
		host.setup.LoopbackPrefixes = func() ([]netip.Prefix, error) { return nil, nil }
		host.setup.Shell.Look = func(name string) (string, bool) {
			if name == nuBin {
				return "", false
			}
			return "/test/bin/" + name, true
		}

		report, _ := host.setup.Run()
		text := report.Text()
		for _, tool := range shimTools() {
			path := filepath.Join(host.homeBin(), tool.Probe[0])
			content, mode := readShim(t, path)
			if mode != shimMode {
				t.Errorf("%s has mode %v, want %v", path, mode, shimMode)
			}
			lines := strings.Split(content, "\n")
			if len(lines) < 2 || lines[1] != leexec.ShimMarker {
				t.Errorf("%s does not carry the le exec marker on its second line:\n%s", path, content)
			}
			if !strings.Contains(content, host.shimLine(tool.Probe[0], host.tools)) {
				t.Errorf("%s does not run le exec on the real tool:\n%s", path, content)
			}
		}
		for _, name := range []string{toolGopls + shimRowSuffix, shimDirRow} {
			if !slices.ContainsFunc(report.Outcomes, func(o Outcome) bool { return o.Name == name }) {
				t.Errorf("the run has no %s row:\n%s", name, text)
			}
		}
		if !strings.Contains(text, "export PATH="+shellSingleQuote(host.homeBin())+`:"$PATH"`) {
			t.Errorf("the run does not print the PATH line for ~/bin:\n%s", text)
		}
		if strings.Contains(text, "NuShell") {
			t.Errorf("the run printed a NuShell line with no nu on PATH:\n%s", text)
		}
	})

	t.Run("a shim starts le exec with every argument", func(t *testing.T) {
		host := newShimHost(t)
		host.visitShims()
		lePath := filepath.Join(host.root, "le")
		if err := os.WriteFile(lePath, []byte("#!/bin/sh\nprintf '%s\\n' \"$@\"\n"), 0o700); err != nil {
			t.Fatalf("write le: %v", err)
		}
		shim := filepath.Join(host.homeBin(), toolGopls)
		out, err := exec.CommandContext(t.Context(), shim, "a b", "c").Output() //nolint:gosec // G204: the shim this test wrote
		if err != nil {
			t.Fatalf("the shim failed: %v", err)
		}
		want := "exec\n" + filepath.Join(host.tools, toolGopls) + "\na b\nc\n"
		if string(out) != want {
			t.Errorf("le was started with\n%s\nwant\n%s", out, want)
		}
	})

	t.Run("a shim whose le is gone names the path and ./le setup", func(t *testing.T) {
		host := newShimHost(t)
		host.visitShims()
		shim := filepath.Join(host.homeBin(), toolGopls)
		command := exec.CommandContext(t.Context(), shim) //nolint:gosec // G204: the shim this test wrote
		var stderr strings.Builder
		command.Stderr = &stderr
		err := command.Run()
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 1 {
			t.Fatalf("the shim answered %v, want exit 1", err)
		}
		message := stderr.String()
		if !strings.Contains(message, filepath.Join(host.root, "le")) || !strings.Contains(message, "./le setup") {
			t.Errorf("the refusal does not name the missing le and ./le setup:\n%s", message)
		}
	})

	t.Run("a rerun rewrites the shims with the current tool path", func(t *testing.T) {
		host := newShimHost(t)
		host.visitShims()
		moved := t.TempDir()
		writeRealTools(t, moved)
		host.env["PATH"] = moved + ":/usr/bin"
		_, rows := host.visitShims()
		content, _ := readShim(t, filepath.Join(host.homeBin(), toolGopls))
		if !strings.Contains(content, host.shimLine(toolGopls, moved)) {
			t.Errorf("the rerun kept the old tool path:\n%s", content)
		}
		if got := rows[toolGopls+shimRowSuffix].State; got != StateInstalled {
			t.Errorf("the rewritten shim is %s, want %s", got, StateInstalled)
		}
		_, rows = host.visitShims()
		if got := rows[toolGopls+shimRowSuffix].State; got != StatePresent {
			t.Errorf("an up-to-date shim is %s, want %s", got, StatePresent)
		}
	})

	t.Run("a file in ~/bin that is not a shim is left alone", func(t *testing.T) {
		host := newShimHost(t)
		if err := os.MkdirAll(host.homeBin(), 0o750); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		mine := filepath.Join(host.homeBin(), toolGopls)
		if err := os.WriteFile(mine, []byte("the owner's own gopls"), 0o700); err != nil {
			t.Fatalf("write: %v", err)
		}
		report, rows := host.visitShims()
		if got := rows[toolGopls+shimRowSuffix].State; got != StatePending {
			t.Errorf("the conflict is %s, want %s", got, StatePending)
		}
		if content, _ := readShim(t, mine); content != "the owner's own gopls" {
			t.Errorf("setup overwrote a file it did not write: %q", content)
		}
		if !strings.Contains(report.Text(), "mv "+shellSingleQuote(mine)) {
			t.Errorf("the conflict does not name the command that fixes it:\n%s", report.Text())
		}
	})

	t.Run("a check run writes nothing", func(t *testing.T) {
		host := newShimHost(t)
		host.setup.Check = true
		_, rows := host.visitShims()
		if got := rows[toolGopls+shimRowSuffix].State; got != StateMissing {
			t.Errorf("an unwritten shim in a check run is %s, want %s", got, StateMissing)
		}
		if _, err := os.Stat(host.homeBin()); !os.IsNotExist(err) {
			t.Errorf("a check run created %s: %v", host.homeBin(), err)
		}
	})

	t.Run("a tool with no real copy outside ~/bin is missing", func(t *testing.T) {
		host := newShimHost(t)
		host.visitShims()
		// Only the shims remain on PATH, and a shim is not the tool.
		host.env["PATH"] = host.homeBin() + ":/nonexistent"
		_, rows := host.visitShims()
		row := rows[toolGopls+shimRowSuffix]
		if row.State != StateMissing || !strings.Contains(row.Detail, toolGopls+" row") {
			t.Errorf("a shim with no real tool is %s %q, want %s pointing at the tool row", row.State, row.Detail, StateMissing)
		}
		// PATH finds the shim first, the way a shell would.
		host.setup.Shell.Look = func(name string) (string, bool) {
			path := filepath.Join(host.homeBin(), name)
			if _, err := os.Stat(path); err != nil {
				return "", false
			}
			return path, true
		}
		if host.setup.Probe(goplsRow(t)) {
			t.Error("the gopls probe took the shim for the real tool")
		}
	})

	t.Run("the PATH lines name zsh and NuShell", func(t *testing.T) {
		host := newShimHost(t)
		envFile := filepath.Join(host.home, ".config", "nushell", "env.nu")
		host.rec.present[nuBin] = true
		host.rec.answers[nuBin+" -c $nu.env-path"] = Result{Out: envFile + "\n"}
		report, _ := host.visitShims()
		text := report.Text()
		if !strings.Contains(text, "~/.zshenv") {
			t.Errorf("the PATH note does not name ~/.zshenv:\n%s", text)
		}
		nuLine := "$env.PATH = ($env.PATH | split row (char esep) | prepend r#'" + host.homeBin() + "'# | uniq)"
		if !strings.Contains(text, envFile) || !strings.Contains(text, nuLine) {
			t.Errorf("the PATH note does not give the NuShell line for %s:\n%s", envFile, text)
		}
		if strings.Count(text, nuLine) != 1 {
			t.Errorf("the NuShell line is printed more than once:\n%s", text)
		}
		if _, err := os.Stat(envFile); !os.IsNotExist(err) {
			t.Errorf("setup wrote %s: %v", envFile, err)
		}
	})
}

// goCacheHost is a host whose `go env GOCACHE` answers current, with every
// command's environment recorded.
func goCacheHost(t *testing.T, current string) (*shimHost, *[][]string) {
	t.Helper()
	host := newShimHost(t)
	host.rec.answers["go env GOCACHE"] = Result{Out: current + "\n"}
	answer := host.setup.Shell.Exec
	envs := &[][]string{}
	host.setup.Shell.Exec = func(ctx context.Context, cmd Cmd) Result {
		*envs = append(*envs, cmd.Env)
		return answer(ctx, cmd)
	}
	return host, envs
}

// goCacheWant answers the setting the host's cache target asks for.
func goCacheWant(h *shimHost) string {
	return filepath.Join(h.home, ".cache", "ze", goCacheDir)
}

// TestSetupRepairsGoCacheSetting: a `go env GOCACHE` that is not the per-user
// target's go-cache is rewritten with `go env -w`, and the row names the old
// and the new value.
// VALIDATES: AC-12, R-5 (a failed `go env -w` names the exact command), and
// that a check run only reports.
// PREVENTS: plain `go build` outside le writing a second cache under
// ~/Library/Caches/go-build.
func TestSetupRepairsGoCacheSetting(t *testing.T) {
	const old = "/Users/someone/Library/Caches/go-build"

	t.Run("a wrong setting is rewritten", func(t *testing.T) {
		t.Setenv(goCacheKey, "/somewhere/else")
		host, envs := goCacheHost(t, old)
		want := goCacheWant(host)
		report := &Report{}
		row := host.setup.visitGoCacheSetting(report)
		if row.State != StateInstalled {
			t.Fatalf("the repaired setting is %s, want %s", row.State, StateInstalled)
		}
		if !strings.Contains(row.Detail, old) || !strings.Contains(row.Detail, want) {
			t.Errorf("the row %q does not name the old and the new value", row.Detail)
		}
		if !slices.Contains(host.rec.ran(), "go env -w GOCACHE="+want) {
			t.Errorf("go env -w was not run: %v", host.rec.ran())
		}
		for _, env := range *envs {
			for _, entry := range env {
				if strings.HasPrefix(entry, goCacheKey+"=") {
					t.Errorf("go env ran with %s, which hides the setting it reads", entry)
				}
			}
		}
	})

	t.Run("the target reached through a link is already right", func(t *testing.T) {
		host, _ := goCacheHost(t, "")
		want := goCacheWant(host)
		if err := os.MkdirAll(want, 0o750); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		link := filepath.Join(host.root, "cache")
		if err := os.Symlink(filepath.Dir(want), link); err != nil {
			t.Fatalf("symlink: %v", err)
		}
		host.rec.answers["go env GOCACHE"] = Result{Out: filepath.Join(link, goCacheDir) + "\n"}
		row := host.setup.visitGoCacheSetting(&Report{})
		if row.State != StatePresent {
			t.Errorf("the linked target is %s %q, want %s", row.State, row.Detail, StatePresent)
		}
		if slices.ContainsFunc(host.rec.ran(), func(line string) bool { return strings.HasPrefix(line, "go env -w") }) {
			t.Errorf("a right setting was rewritten: %v", host.rec.ran())
		}
	})

	t.Run("a failed go env -w names the command", func(t *testing.T) {
		host, _ := goCacheHost(t, old)
		want := goCacheWant(host)
		host.rec.answers["go env -w GOCACHE="+want] = Result{Code: 1, Err: "go: GOENV=off\n"}
		report := &Report{}
		row := host.setup.visitGoCacheSetting(report)
		if row.State != StateMissing || !strings.Contains(row.Detail, "GOENV=off") {
			t.Errorf("the failure is %s %q, want %s with the reason", row.State, row.Detail, StateMissing)
		}
		if !strings.Contains(report.Text(), "go env -w GOCACHE="+shellSingleQuote(want)) {
			t.Errorf("the failure does not print the command to run:\n%s", report.Text())
		}
	})

	t.Run("a check run only reports", func(t *testing.T) {
		host, _ := goCacheHost(t, old)
		host.setup.Check = true
		row := host.setup.visitGoCacheSetting(&Report{})
		if row.State != StateMissing || !strings.Contains(row.Detail, old) {
			t.Errorf("a wrong setting in a check run is %s %q, want %s naming it", row.State, row.Detail, StateMissing)
		}
		if slices.ContainsFunc(host.rec.ran(), func(line string) bool { return strings.HasPrefix(line, "go env -w") }) {
			t.Errorf("a check run rewrote the setting: %v", host.rec.ran())
		}
	})
}

// TestSetupChoosesShimDir: the shims go in the earliest PATH entry that is an
// existing directory under HOME this user can write, outside a ze checkout and
// ahead of every directory holding a real shimmed tool, and nowhere else.
// Each case builds a PATH, runs the shim rows, and reads which directory got
// the shims and what the shim-dir row says.
// VALIDATES: AC-10a (shims in the chosen directory), AC-11a (a qualifying
// entry prints no PATH line; none prints the ~/bin lines), O-8.
// PREVENTS: shims written into ~/bin on a PATH that never reaches ~/bin.
func TestSetupChoosesShimDir(t *testing.T) {
	type layout struct {
		path   string
		chosen string
	}
	for name, build := range map[string]func(t *testing.T, h *shimHost) layout{
		// The owner's PATH: a directory outside HOME first, then ~/Unix/bin,
		// then the Go bin directory with the real tools, then ~/Unix/bin again.
		"the first entry under HOME ahead of the real tools": func(t *testing.T, h *shimHost) layout {
			outside := t.TempDir()
			unixBin := h.homeDir(t, "Unix", "bin")
			// The owner's is ~/Bin/go/bin; ~/Bin is ~/bin on a Mac's
			// case-insensitive disk, so another name keeps ~/bin absent.
			goBin := h.homeDir(t, "Tools", "go", "bin")
			writeRealTools(t, goBin)
			return layout{strings.Join([]string{outside, unixBin, goBin, "/usr/bin", unixBin}, ":"), unixBin}
		},
		"a directory inside a ze checkout is skipped": func(t *testing.T, h *shimHost) layout {
			checkout := h.homeDir(t, "ze")
			for _, marker := range []string{"go.mod", "feature-gates.txt"} {
				if err := os.WriteFile(filepath.Join(checkout, marker), nil, 0o600); err != nil {
					t.Fatalf("write marker: %v", err)
				}
			}
			unixBin := h.homeDir(t, "Unix", "bin")
			return layout{strings.Join([]string{h.homeDir(t, "ze", "bin"), unixBin, h.tools}, ":"), unixBin}
		},
		"a directory this user cannot write is skipped": func(t *testing.T, h *shimHost) layout {
			if os.Geteuid() == 0 {
				t.Skip("root can write a read-only directory")
			}
			readOnly := h.homeDir(t, "ro")
			if err := os.Chmod(readOnly, 0o555); err != nil { //nolint:gosec // G302: the test's own directory, made read-only
				t.Fatalf("chmod: %v", err)
			}
			t.Cleanup(func() { _ = os.Chmod(readOnly, 0o750) }) //nolint:gosec // G302: restored so TempDir can remove it
			unixBin := h.homeDir(t, "Unix", "bin")
			return layout{strings.Join([]string{readOnly, unixBin, h.tools}, ":"), unixBin}
		},
		"a missing, a relative and a file entry are skipped": func(t *testing.T, h *shimHost) layout {
			file := filepath.Join(h.homeDir(t, "files"), "plain")
			if err := os.WriteFile(file, nil, 0o600); err != nil {
				t.Fatalf("write: %v", err)
			}
			unixBin := h.homeDir(t, "Unix", "bin")
			missing := filepath.Join(h.home, "missing")
			return layout{strings.Join([]string{missing, "bin", file, unixBin, h.tools}, ":"), unixBin}
		},
	} {
		t.Run(name, func(t *testing.T) {
			host := newShimHost(t)
			want := build(t, host)
			host.env["PATH"] = want.path
			report, rows := host.visitShims()
			text := report.Text()
			if row := rows[shimDirRow]; row.State != StatePresent || !strings.Contains(row.Detail, want.chosen) {
				t.Errorf("the shim-dir row is %s %q, want %s naming %s", row.State, row.Detail, StatePresent, want.chosen)
			}
			for _, tool := range shimTools() {
				if row := rows[tool.Name+shimRowSuffix]; row.State != StateInstalled {
					t.Fatalf("the %s row is %s %q, want %s:\n%s", tool.Name, row.State, row.Detail, StateInstalled, text)
				}
				content, _ := readShim(t, filepath.Join(want.chosen, tool.Probe[0]))
				if !strings.Contains(content, leexec.ShimMarker) {
					t.Errorf("%s holds no shim for %s:\n%s", want.chosen, tool.Probe[0], content)
				}
			}
			if strings.Contains(text, "export PATH=") {
				t.Errorf("a qualifying PATH entry still printed a PATH line:\n%s", text)
			}
			if _, err := os.Stat(host.homeBin()); !os.IsNotExist(err) {
				t.Errorf("the run created %s although %s qualified: %v", host.homeBin(), want.chosen, err)
			}
		})
	}

	t.Run("the pending row names each entry under HOME it passed over", func(t *testing.T) {
		host := newShimHost(t)
		checkout := host.homeDir(t, "ze")
		for _, marker := range []string{"go.mod", "feature-gates.txt"} {
			if err := os.WriteFile(filepath.Join(checkout, marker), nil, 0o600); err != nil {
				t.Fatalf("write marker: %v", err)
			}
		}
		checkoutBin := host.homeDir(t, "ze", "bin")
		missing := filepath.Join(host.home, "missing")
		host.env["PATH"] = strings.Join([]string{missing, checkoutBin, host.tools}, ":")
		_, rows := host.visitShims()
		row := rows[shimDirRow]
		for _, want := range []string{missing + ": does not exist", checkoutBin + ": is inside the ze checkout " + checkout} {
			if !strings.Contains(row.Detail, want) {
				t.Errorf("the shim-dir row %q does not say %q", row.Detail, want)
			}
		}
	})

	t.Run("a directory after the real tools does not qualify", func(t *testing.T) {
		host := newShimHost(t)
		late := host.homeDir(t, "late")
		host.env["PATH"] = host.tools + ":" + late
		report, rows := host.visitShims()
		row := rows[shimDirRow]
		if row.State != StatePending || !strings.Contains(row.Detail, host.tools) {
			t.Errorf("the shim-dir row is %s %q, want %s naming %s", row.State, row.Detail, StatePending, host.tools)
		}
		if !strings.Contains(report.Text(), "export PATH="+shellSingleQuote(host.homeBin())+`:"$PATH"`) {
			t.Errorf("no qualifying entry, yet the ~/bin PATH line is missing:\n%s", report.Text())
		}
		if _, err := os.Stat(filepath.Join(host.homeBin(), toolGopls)); err != nil {
			t.Errorf("the fallback shim is not in ~/bin: %v", err)
		}
		if _, err := os.Stat(filepath.Join(late, toolGopls)); !os.IsNotExist(err) {
			t.Errorf("a shim went into %s, which PATH reaches after the real tool: %v", late, err)
		}
	})
}

// TestSetupRemovesStrayShims: once the shims are in the chosen directory, a le
// shim for a shimmed tool in ~/bin or any other PATH entry is removed, and a
// file without the marker, or a marked file under another name, is left. A
// check run removes nothing and reports the old shims.
// VALIDATES: AC-10a (a le-marked shim left in another directory, ~/bin
// included, is removed by the same run).
// PREVENTS: a shim in ~/bin from an earlier run outliving the move.
func TestSetupRemovesStrayShims(t *testing.T) {
	host := newShimHost(t)
	unixBin := host.homeDir(t, "Unix", "bin")
	bin := host.homeDir(t, "bin")
	other := t.TempDir()
	host.env["PATH"] = strings.Join([]string{unixBin, host.tools, other}, ":")

	var strays []string
	for _, tool := range shimTools() {
		if tool.Probe[0] == toolStaticcheck {
			continue
		}
		strays = append(strays, writeShimFile(t, bin, tool.Probe[0]))
	}
	strays = append(strays, writeShimFile(t, other, toolGopls))
	foreign := filepath.Join(bin, toolStaticcheck)
	if err := os.WriteFile(foreign, []byte("the owner's own staticcheck"), 0o700); err != nil {
		t.Fatalf("write: %v", err)
	}
	unrelated := writeShimFile(t, bin, "not-a-shimmed-tool")

	host.setup.Check = true
	_, rows := host.visitShims()
	if row := rows[toolGopls+shimRowSuffix]; row.State != StateMissing || !strings.Contains(row.Detail, filepath.Join(bin, toolGopls)) {
		t.Errorf("a check run reports the gopls row %s %q, want %s naming the old shim", row.State, row.Detail, StateMissing)
	}
	for _, stray := range strays {
		if _, err := os.Stat(stray); err != nil {
			t.Errorf("a check run removed %s: %v", stray, err)
		}
	}

	host.setup.Check = false
	_, rows = host.visitShims()
	for _, stray := range strays {
		if _, err := os.Stat(stray); !os.IsNotExist(err) {
			t.Errorf("the old shim %s is still there: %v", stray, err)
		}
	}
	if row := rows[toolGopls+shimRowSuffix]; !strings.Contains(row.Detail, "removed old shims") ||
		!strings.Contains(row.Detail, filepath.Join(other, toolGopls)) {
		t.Errorf("the gopls row does not report what it removed: %q", row.Detail)
	}
	// A second spelling of the chosen directory names the same shims, which
	// are kept: a link stands in for the other case of a case-insensitive disk.
	alias := filepath.Join(host.home, "alias")
	if err := os.Symlink(unixBin, alias); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	host.env["PATH"] = strings.Join([]string{unixBin, host.tools, alias}, ":")
	_, rows = host.visitShims()
	if row := rows[toolGopls+shimRowSuffix]; row.State != StatePresent {
		t.Errorf("with a second spelling of %s on PATH the gopls row is %s %q, want %s", unixBin, row.State, row.Detail, StatePresent)
	}
	if content, _ := readShim(t, foreign); content != "the owner's own staticcheck" {
		t.Errorf("setup touched a file it did not write: %q", content)
	}
	if _, err := os.Stat(unrelated); err != nil {
		t.Errorf("setup removed a marked file that is not a shimmed tool: %v", err)
	}
	for _, tool := range shimTools() {
		if _, err := os.Stat(filepath.Join(unixBin, tool.Probe[0])); err != nil {
			t.Errorf("the shim for %s is not in %s: %v", tool.Probe[0], unixBin, err)
		}
	}
}

// TestSetupJudgesTheCallersPath: le's own PATH starts with the patched
// GOROOT's bin, which lives under the per-user cache and so under HOME, and
// the run still chooses the first qualifying entry of the caller's PATH, as
// gen.CallerEnvironment reconstructs it. The shims an earlier run wrote into
// the tree's bin are removed, and the gopls-answers row names the shim the
// caller's shell runs.
// VALIDATES: AC-10a (the shim directory is judged on the developer's PATH,
// and a le shim in a directory only le's PATH holds is removed).
// PREVENTS: the shims landing in the patched tree's bin, where no shell of the
// developer's looks, as they did on the owner's Mac.
func TestSetupJudgesTheCallersPath(t *testing.T) {
	host := newShimHost(t)
	cache := host.homeDir(t, ".cache", "ze")
	if err := os.Symlink(cache, filepath.Join(host.root, "cache")); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	treeBin := host.homeDir(t, ".cache", "ze", "stdpatch", "0123abcd", "goroot", "bin")
	var strays []string
	for _, tool := range shimTools() {
		strays = append(strays, writeShimFile(t, treeBin, tool.Probe[0]))
	}
	outside := t.TempDir()
	unixBin := host.homeDir(t, "Unix", "bin")
	goBin := host.homeDir(t, "Tools", "go", "bin")
	writeRealTools(t, goBin)
	host.env["PATH"] = strings.Join([]string{treeBin, outside, unixBin, goBin, "/usr/bin"}, ":")

	report, rows := host.visitShims()
	if row := rows[shimDirRow]; row.State != StatePresent || !strings.Contains(row.Detail, unixBin) {
		t.Fatalf("the shim-dir row is %s %q, want %s naming %s:\n%s", row.State, row.Detail, StatePresent, unixBin, report.Text())
	}
	for _, tool := range shimTools() {
		content, _ := readShim(t, filepath.Join(unixBin, tool.Probe[0]))
		if !strings.Contains(content, host.shimLine(tool.Probe[0], goBin)) {
			t.Errorf("the %s shim in %s does not start the real tool in %s:\n%s", tool.Probe[0], unixBin, goBin, content)
		}
	}
	for _, stray := range strays {
		if _, err := os.Stat(stray); !os.IsNotExist(err) {
			t.Errorf("the shim %s in the patched tree's bin is still there: %v", stray, err)
		}
	}

	answerer := host.setup.goplsAnswerer()
	if want := "answered by " + filepath.Join(unixBin, toolGopls) + ", the le exec shim"; answerer != want {
		t.Errorf("the gopls-answers row says %q, want %q", answerer, want)
	}
}
