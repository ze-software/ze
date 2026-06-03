// Design: docs/guide/command-catalogue.md -- offline ping diagnostic
// Related: ping.go -- daemon ICMP ping; this is the offline OS-ping root
//
// offline.go is the offline home for `ze ping <target>`: it wraps the OS ping
// tool with validated argv (no shell). The daemon is not required -- the `ze`
// binary shells out directly. Per ai/rules/cli-patterns.md the command uses its
// own flag.NewFlagSet with a custom Usage printer.

package cmd

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// hostnameRE matches RFC 1123 hostnames, IP literals, and interface
// names. Accepts: letters, digits, dot, underscore, colon (IPv6),
// hyphen. No shell meta-characters.
var hostnameRE = regexp.MustCompile(`^[A-Za-z0-9._:-]+$`)

const (
	maxOSPingCount      = 100000 // per-invocation echo-request ceiling
	maxTargetLen        = 253    // RFC 1035 hostname ceiling
	maxInterfaceNameLen = 15     // Linux IFNAMSIZ minus NUL
)

// validateTarget accepts a hostname or IP literal. Caller must have
// already stripped flags.
func validateTarget(t string) error {
	if t == "" {
		return errors.New("target is required")
	}
	if len(t) > maxTargetLen {
		return fmt.Errorf("target too long (%d > %d chars)", len(t), maxTargetLen)
	}
	if !hostnameRE.MatchString(t) {
		return fmt.Errorf("invalid target %q: only letters, digits, dot, underscore, colon, hyphen allowed", t)
	}
	// If it looks like a numeric IP, verify it parses.
	if strings.ContainsAny(t, ":") || (strings.Count(t, ".") == 3 && strings.IndexFunc(t, func(r rune) bool {
		return (r < '0' || r > '9') && r != '.'
	}) == -1) {
		if net.ParseIP(t) == nil {
			return fmt.Errorf("invalid IP literal %q", t)
		}
	}
	return nil
}

// validateInterfaceName rejects interface names outside IFNAMSIZ or
// containing shell meta-characters. Empty string is allowed and means
// do not pass --interface to the tool.
func validateInterfaceName(name string) error {
	if name == "" {
		return nil
	}
	if len(name) > maxInterfaceNameLen {
		return fmt.Errorf("interface name too long (%d > %d chars)", len(name), maxInterfaceNameLen)
	}
	if !hostnameRE.MatchString(name) {
		return fmt.Errorf("invalid interface name %q", name)
	}
	return nil
}

// RunPing invokes the OS `ping` utility against target with optional
// count/interface flags (usage: `ze ping <target> [--count N]
// [--interface IF]`). Returns the exit code from `ping`.
func RunPing(args []string) int {
	const name = "ping"
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() {
		usageTail := "Send ICMP echo-request to <target>. Arguments are validated before\nexec; no shell is involved."
		if _, err := fmt.Fprintf(os.Stderr, "Usage: ze %s <target> [--count N] [--interface IF]\n\n%s\n\n", name, usageTail); err != nil {
			return // writing to stderr; nothing to recover
		}
		fs.PrintDefaults()
	}
	var (
		count int
		iface string
	)
	fs.IntVar(&count, "count", 0, "number of echo requests (1..100000; 0 = tool default)")
	fs.IntVar(&count, "c", 0, "short form of --count")
	fs.StringVar(&iface, "interface", "", "source interface")
	fs.StringVar(&iface, "i", "", "short form of --interface")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}
	target, rc := extractTarget(fs, name)
	if rc != 0 {
		return rc
	}
	if count < 0 || count > maxOSPingCount {
		fmt.Fprintf(os.Stderr, "%s: --count must be in 0..%d (0 = tool default)\n", name, maxOSPingCount)
		return 1
	}
	if err := validateInterfaceName(iface); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", name, err)
		return 1
	}
	argv := []string{}
	if count > 0 {
		argv = append(argv, "-c", strconv.Itoa(count))
	}
	if iface != "" {
		argv = append(argv, "-I", iface)
	}
	argv = append(argv, target)
	return runExec(name, argv)
}

// extractTarget pulls the single target positional argument from fs
// and validates it. On error, it prints to stderr and returns a
// non-zero exit code so the caller can bail immediately.
func extractTarget(fs *flag.FlagSet, name string) (string, int) {
	rest := fs.Args()
	if len(rest) == 0 {
		fmt.Fprintf(os.Stderr, "%s: target is required\n", name)
		fs.Usage()
		return "", 1
	}
	if len(rest) > 1 {
		fmt.Fprintf(os.Stderr, "%s: multiple targets not allowed\n", name)
		return "", 1
	}
	target := rest[0]
	if err := validateTarget(target); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", name, err)
		return "", 1
	}
	return target, 0
}

// runExec invokes tool with args, streaming stdout/stderr through.
// The tool path is looked up via exec.LookPath; missing binary returns 1.
//
// exec.LookPath honors PATH, so ze trusts its environment's PATH to
// resolve `ping` to the expected binary. Running ze under a hardened PATH
// (or with explicit tool paths in a future config option) is the caller's
// responsibility.
func runExec(tool string, args []string) int {
	path, err := exec.LookPath(tool)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: not installed: %v\n", tool, err)
		return 1
	}
	cmd := exec.CommandContext(context.Background(), path, args...) //nolint:gosec // args are validated above
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode()
		}
		return 1
	}
	return 0
}
