// Design: plan/spec-install-0-umbrella.md — ze install local: binary copy + systemd + config scaffold

package install

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/helpfmt"
	"codeberg.org/thomas-mangin/ze/internal/core/paths"
)

var prefixChoices = []struct {
	path string
	desc string
}{
	{"/usr/local", "recommended"},
	{"/usr", "system"},
	{"/opt/ze", "self-contained"},
}

const systemdUnitPrefix = `[Unit]
Description=Ze Network OS
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=`

const systemdUnitSuffix = ` start
Restart=on-failure
RestartSec=5
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
`

const systemdUnitPath = "/etc/systemd/system/ze.service"

func runLocal(args []string) int {
	fs := flag.NewFlagSet("install local", flag.ContinueOnError)

	prefix := fs.String("prefix", "", "Installation prefix (e.g. /usr/local)")
	noSystemd := fs.Bool("no-systemd", false, "Skip systemd service setup")
	forceSystemd := fs.Bool("systemd", false, "Force systemd setup even if auto-detection fails")
	dryRun := fs.Bool("dry-run", false, "Print what would be done without making changes")

	fs.Usage = func() { localUsage() }

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}

	if *noSystemd && *forceSystemd {
		fmt.Fprintf(os.Stderr, "error: --systemd and --no-systemd are mutually exclusive\n")
		return 1
	}

	if *prefix != "" && strings.ContainsAny(*prefix, " \t\n") {
		fmt.Fprintf(os.Stderr, "error: --prefix must not contain whitespace\n")
		return 1
	}

	selectedPrefix := *prefix
	if selectedPrefix == "" {
		p, err := promptPrefix(os.Stdin, os.Stderr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
		selectedPrefix = p
	}

	binDir := filepath.Join(selectedPrefix, "bin")
	binPath := filepath.Join(binDir, "ze")

	self, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: cannot find own binary: %v\n", err)
		return 1
	}

	resolved, err := filepath.EvalSymlinks(self)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: resolving binary path: %v\n", err)
		return 1
	}

	configDir := paths.ConfigDirFromBinary(binPath)

	wantSystemd := !*noSystemd
	if !*forceSystemd && !*noSystemd {
		wantSystemd = hasSystemd()
	}
	if *forceSystemd {
		wantSystemd = true
	}

	if *dryRun {
		return dryRunLocal(resolved, binPath, configDir, wantSystemd)
	}

	if err := os.MkdirAll(binDir, 0o755); err != nil { // #nosec G301 - standard system bin directory
		fmt.Fprintf(os.Stderr, "error: creating %s: %v\n", binDir, err)
		return 1
	}

	_, existErr := os.Stat(binPath)
	replacing := existErr == nil

	if err := copyFile(resolved, binPath, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "error: copying binary to %s: %v\n", binPath, err)
		return 1
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
				return 1
			}
			fmt.Fprintf(os.Stderr, "created %s\n", configDir)
		default:
			fmt.Fprintf(os.Stderr, "error: checking %s: %v\n", dbPath, statErr)
			return 1
		}
	}

	if wantSystemd {
		if err := installSystemdUnit(binPath); err != nil {
			fmt.Fprintf(os.Stderr, "error: systemd setup: %v\n", err)
			return 1
		}
		fmt.Fprintf(os.Stderr, "systemd unit installed and enabled\n")
	}

	fmt.Fprintf(os.Stderr, "\ninstallation complete. run 'ze init' to bootstrap the database.\n")
	return 0
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

func hasSystemd() bool {
	_, err := exec.LookPath("systemctl")
	return err == nil
}

func copyFile(src, dst string, perm os.FileMode) error {
	in, err := os.Open(src) // #nosec G304 - src is our own resolved binary path
	if err != nil {
		return err
	}
	defer in.Close() //nolint:errcheck // read-only source

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm) // #nosec G304 - dst is derived from user-selected prefix
	if err != nil {
		return err
	}

	if _, err := io.Copy(out, in); err != nil {
		out.Close() //nolint:errcheck // already returning copy error
		return err
	}
	return out.Close()
}

func buildSystemdUnit(binPath string) string {
	return systemdUnitPrefix + binPath + systemdUnitSuffix
}

func installSystemdUnit(binPath string) error {
	content := buildSystemdUnit(binPath)
	if err := os.WriteFile(systemdUnitPath, []byte(content), 0o644); err != nil { // #nosec G306 - systemd units must be world-readable
		return fmt.Errorf("writing %s: %w", systemdUnitPath, err)
	}

	if err := runSystemctl("daemon-reload"); err != nil {
		return err
	}
	return runSystemctl("enable", "ze")
}

func runSystemctl(args ...string) error {
	cmd := exec.CommandContext(context.Background(), "systemctl", args...) // #nosec G204 - args are hardcoded string literals
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("systemctl %s: %w", strings.Join(args, " "), err)
	}
	return nil
}

func dryRunLocal(src, binPath, configDir string, wantSystemd bool) int {
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
			return 1
		}
	}
	if wantSystemd {
		fmt.Fprintf(os.Stderr, "would install %s\n", systemdUnitPath)
		fmt.Fprintf(os.Stderr, "would run: systemctl daemon-reload\n")
		fmt.Fprintf(os.Stderr, "would run: systemctl enable ze\n")
	}
	return 0
}

func localUsage() {
	p := helpfmt.Page{
		Command: "ze install local",
		Summary: "Install ze binary, systemd unit, and config directory on this machine",
		Usage:   []string{"ze install local [options]"},
		Sections: []helpfmt.HelpSection{
			{Title: "Options", Entries: []helpfmt.HelpEntry{
				{Name: "--prefix <path>", Desc: "Installation prefix (default: interactive selection)"},
				{Name: "--systemd", Desc: "Force systemd service setup"},
				{Name: "--no-systemd", Desc: "Skip systemd service setup"},
				{Name: "--dry-run", Desc: "Print what would be done without making changes"},
			}},
			{Title: "Installation paths", Entries: []helpfmt.HelpEntry{
				{Name: "/usr/local", Desc: "Binary in /usr/local/bin, config in /etc/ze (recommended)"},
				{Name: "/usr", Desc: "Binary in /usr/bin, config in /etc/ze"},
				{Name: "/opt/ze", Desc: "Binary in /opt/ze/bin, config in /opt/ze/etc/ze"},
			}},
		},
		Examples: []string{
			"ze install local                   Interactive prefix selection",
			"ze install local --prefix /usr/local",
			"ze install local --prefix /opt/ze --no-systemd",
			"ze install local --dry-run",
		},
	}
	p.Write()
}
