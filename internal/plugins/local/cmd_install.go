// Design: docs/architecture/cli/plugin-modes.md — ze local install: binary copy + config scaffold

package local

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/helpfmt"
	"github.com/ze-software/ze/internal/core/paths"
)

var prefixChoices = []struct {
	path string
	desc string
}{
	{"/usr/local", "recommended"},
	{"/usr", "system"},
	{"/opt/ze", "self-contained"},
}

func cmdInstall(args []string) int {
	fs := flag.NewFlagSet("local install", flag.ContinueOnError)

	prefix := fs.String("prefix", "", "Installation prefix (e.g. /usr/local)")
	dryRun := fs.Bool("dry-run", false, "Print what would be done without making changes")

	fs.Usage = func() { installUsage() }

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}
		return exitError
	}

	if *prefix != "" && strings.ContainsAny(*prefix, " \t\n") {
		fmt.Fprintf(os.Stderr, "error: --prefix must not contain whitespace\n")
		return exitError
	}

	selectedPrefix := *prefix
	if selectedPrefix == "" {
		p, err := promptPrefix(os.Stdin, os.Stderr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return exitError
		}
		selectedPrefix = p
	}

	binDir := filepath.Join(selectedPrefix, "bin")
	binPath := filepath.Join(binDir, "ze")

	self, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: cannot find own binary: %v\n", err)
		return exitError
	}

	resolved, err := filepath.EvalSymlinks(self)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: resolving binary path: %v\n", err)
		return exitError
	}

	configDir := paths.ConfigDirFromBinary(binPath)

	if *dryRun {
		return dryRunInstall(resolved, binPath, configDir)
	}

	if err := os.MkdirAll(binDir, 0o755); err != nil { // #nosec G301 - standard system bin directory
		fmt.Fprintf(os.Stderr, "error: creating %s: %v\n", binDir, err)
		return exitError
	}

	_, existErr := os.Stat(binPath)
	replacing := existErr == nil

	if err := copyFile(resolved, binPath); err != nil {
		fmt.Fprintf(os.Stderr, "error: copying binary to %s: %v\n", binPath, err)
		return exitError
	}
	if replacing {
		fmt.Fprintf(os.Stderr, "replaced %s\n", binPath)
	} else {
		fmt.Fprintf(os.Stderr, "installed %s\n", binPath)
	}

	if configDir != "" {
		dbPath := filepath.Join(configDir, "database.zefs")
		_, statErr := os.Stat(dbPath)
		switch {
		case statErr == nil:
			fmt.Fprintf(os.Stderr, "config directory %s already exists, skipping\n", configDir)
		case os.IsNotExist(statErr):
			if mkErr := os.MkdirAll(configDir, 0o755); mkErr != nil { // #nosec G301 - standard config directory
				fmt.Fprintf(os.Stderr, "error: creating %s: %v\n", configDir, mkErr)
				return exitError
			}
			fmt.Fprintf(os.Stderr, "created %s\n", configDir)
		default:
			fmt.Fprintf(os.Stderr, "error: checking %s: %v\n", dbPath, statErr)
			return exitError
		}
	}

	fmt.Fprintf(os.Stderr, "\ninstallation complete. run 'ze init' to bootstrap the database.\n")
	fmt.Fprintf(os.Stderr, "hint: run 'ze systemd install' to set up systemd service management\n")
	return exitOK
}

func promptPrefix(r io.Reader, w io.Writer) (string, error) {
	var b strings.Builder
	b.WriteString("Select installation prefix:\n")
	for i, c := range prefixChoices {
		b.WriteString("  ")
		b.WriteString(strconv.Itoa(i + 1))
		b.WriteString(") ")
		b.WriteString(c.path)
		b.WriteString("  (")
		b.WriteString(c.desc)
		b.WriteString(")\n")
	}
	b.WriteString("Choice [1]: ")
	fmt.Fprint(w, b.String()) //nolint:errcheck // terminal prompt

	scanner := bufio.NewScanner(r)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return "", err
		}
		return "", errors.New("no input")
	}

	text := strings.TrimSpace(scanner.Text())
	if text == "" {
		return prefixChoices[0].path, nil
	}

	n, err := strconv.Atoi(text)
	if err != nil || n < 1 || n > len(prefixChoices) {
		return "", errors.New("invalid choice: " + text)
	}
	return prefixChoices[n-1].path, nil
}

// copyFile copies src to dst via a temp file in dst's own directory, then
// renames it into place. os.Rename installs a NEW inode, so a process already
// executing dst keeps its old mapping intact and the swap is atomic: a reader
// sees either the whole old binary or the whole new one, never a partial copy.
//
// Writing onto dst in place (O_WRONLY|O_CREATE|O_TRUNC) is the hazard fixed in
// pkg/zefs/store.go: the kernel maps a running executable, so truncating the
// same inode invalidates its text pages and the running process takes SIGBUS.
// Linux usually refuses with ETXTBSY instead; macOS does not protect it at all.
// Same filesystem is guaranteed because the temp file is created in filepath.Dir(dst).
// installedBinaryMode is the mode an installed `ze` must end up with. Applied
// explicitly because os.CreateTemp makes the staging file 0600.
const installedBinaryMode = 0o755

func copyFile(src, dst string) error {
	in, err := os.Open(src) // #nosec G304 - src is our own resolved binary path
	if err != nil {
		return err
	}
	defer in.Close() //nolint:errcheck // read-only source

	tmp, err := os.CreateTemp(filepath.Dir(dst), ".ze-install-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	committed := false
	defer func() {
		if !committed {
			os.Remove(tmpName) //nolint:errcheck // best-effort cleanup of temp file
		}
	}()

	if _, err := io.Copy(tmp, in); err != nil {
		tmp.Close() //nolint:errcheck // already returning copy error
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close() //nolint:errcheck // already failing on sync path
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	// os.CreateTemp creates the file 0600; an installed binary must be executable.
	if err := os.Chmod(tmpName, installedBinaryMode); err != nil { // #nosec G302 - an installed binary must be executable
		return err
	}

	if err := os.Rename(tmpName, dst); err != nil {
		return err
	}
	committed = true
	return nil
}

func dryRunInstall(src, binPath, configDir string) int {
	fmt.Fprintf(os.Stderr, "would copy %s -> %s\n", src, binPath)
	if configDir != "" {
		dbPath := filepath.Join(configDir, "database.zefs")
		_, statErr := os.Stat(dbPath)
		switch {
		case statErr == nil:
			fmt.Fprintf(os.Stderr, "would skip %s (already exists)\n", configDir)
		case os.IsNotExist(statErr):
			fmt.Fprintf(os.Stderr, "would create %s\n", configDir)
		default:
			fmt.Fprintf(os.Stderr, "error: checking %s: %v\n", dbPath, statErr)
			return exitError
		}
	}
	return exitOK
}

func installUsage() {
	p := helpfmt.Page{
		Command: "ze local install",
		Summary: "Copy ze binary and create config directory on this machine",
		Usage:   []string{"ze local install [options]"},
		Sections: []helpfmt.HelpSection{
			{Title: "Options", Entries: []helpfmt.HelpEntry{
				{Name: "--prefix <path>", Desc: "Installation prefix (default: interactive selection)"},
				{Name: "--dry-run", Desc: "Print what would be done without making changes"},
			}},
			{Title: "Installation paths", Entries: []helpfmt.HelpEntry{
				{Name: "/usr/local", Desc: "Binary in /usr/local/bin, config in /etc/ze (recommended)"},
				{Name: "/usr", Desc: "Binary in /usr/bin, config in /etc/ze"},
				{Name: "/opt/ze", Desc: "Binary in /opt/ze/bin, config in /opt/ze/etc/ze"},
			}},
		},
		Examples: []string{
			"ze local install                   Interactive prefix selection",
			"ze local install --prefix /usr/local",
			"ze local install --prefix /opt/ze",
			"ze local install --dry-run",
		},
	}
	p.WriteErr()
}
