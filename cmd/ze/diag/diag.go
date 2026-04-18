// Design: docs/guide/command-catalogue.md -- diagnostics (ping, traceroute, wireguard keygen)
// Related: ../main.go -- registers RunPing / RunTraceroute / RunWgKeypair as local commands
//
// Package diag is the offline home for network diagnostic commands that
// wrap OS tools with validated argv (no shell). The daemon is not
// required: all three subcommands are local shell-outs from the `ze`
// binary. Per rules/cli-patterns.md each subcommand uses its own
// flag.NewFlagSet with a custom Usage printer.

package diag

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
	maxPingCount        = 100000 // per-invocation echo-request ceiling
	maxTracerouteProbes = 10     // per-hop probes ceiling
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

// diagSpec describes one diagnostic tool (ping, traceroute) with a
// single integer knob. runDiag handles flag parsing, target validation,
// integer bound checks, and exec.
type diagSpec struct {
	name       string // "ping" / "traceroute"
	countName  string // "count" / "probes"
	countShort string // "c" / "q" (short flag)
	countDesc  string // description for --count/--probes
	toolFlag   string // "-c" / "-q" (flag passed to the OS tool)
	ifaceFlag  string // "-I" / "-i"
	maxCount   int    // upper bound on the integer knob
	usageTail  string // trailing help text ("Send ICMP echo-request...")
}

var pingSpec = diagSpec{
	name:       "ping",
	countName:  "count",
	countShort: "c",
	countDesc:  "number of echo requests (1..100000; 0 = tool default)",
	toolFlag:   "-c",
	ifaceFlag:  "-I",
	maxCount:   maxPingCount,
	usageTail:  "Send ICMP echo-request to <target>. Arguments are validated before\nexec; no shell is involved.",
}

var tracerouteSpec = diagSpec{
	name:       "traceroute",
	countName:  "probes",
	countShort: "q",
	countDesc:  "probes per hop (1..10; 0 = tool default)",
	toolFlag:   "-q",
	ifaceFlag:  "-i",
	maxCount:   maxTracerouteProbes,
	usageTail:  "Trace the path to <target> by sending probes per hop. Arguments\nare validated before exec; no shell is involved.",
}

// RunPing invokes the OS `ping` utility against target with optional
// count/interface flags (usage: `ze ping <target> [--count N]
// [--interface IF]`). Returns the exit code from `ping`.
func RunPing(args []string) int { return runDiag(pingSpec, args) }

// RunTraceroute invokes the OS `traceroute` utility (usage: `ze
// traceroute <target> [--probes N] [--interface IF]`). --probes is
// ze's name for traceroute's per-hop query count (the OS tool's -q);
// we rename it because operators usually expect --probes to mean
// per-hop queries rather than total-packets.
func RunTraceroute(args []string) int { return runDiag(tracerouteSpec, args) }

// runDiag is the shared validation+exec path for ping and traceroute.
func runDiag(spec diagSpec, args []string) int {
	fs := flag.NewFlagSet(spec.name, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() {
		if _, err := fmt.Fprintf(fs.Output(), "Usage: ze %s <target> [--%s N] [--interface IF]\n\n%s\n\n", spec.name, spec.countName, spec.usageTail); err != nil {
			return // writing to stderr; nothing to recover
		}
		fs.PrintDefaults()
	}
	var (
		count int
		iface string
	)
	fs.IntVar(&count, spec.countName, 0, spec.countDesc)
	fs.IntVar(&count, spec.countShort, 0, "short form of --"+spec.countName)
	fs.StringVar(&iface, "interface", "", "source interface")
	fs.StringVar(&iface, "i", "", "short form of --interface")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}
	target, rc := extractTarget(fs, spec.name)
	if rc != 0 {
		return rc
	}
	if count < 0 || count > spec.maxCount {
		fmt.Fprintf(os.Stderr, "%s: --%s must be in 0..%d (0 = tool default)\n", spec.name, spec.countName, spec.maxCount)
		return 1
	}
	if err := validateInterfaceName(iface); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", spec.name, err)
		return 1
	}
	argv := []string{}
	if count > 0 {
		argv = append(argv, spec.toolFlag, strconv.Itoa(count))
	}
	if iface != "" {
		argv = append(argv, spec.ifaceFlag, iface)
	}
	argv = append(argv, target)
	return runExec(spec.name, argv)
}

// RunWgKeypair generates a WireGuard keypair by invoking `wg genkey`
// and `wg pubkey`. Prints two lines to stdout:
//
//	private: <base64>
//	public:  <base64>
//
// Usage: ze generate wireguard keypair
//
// Returns 1 if `wg` is not installed. No arguments accepted.
func RunWgKeypair(args []string) int {
	fs := flag.NewFlagSet("generate wireguard keypair", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() {
		if _, err := fmt.Fprintln(fs.Output(), "Usage: ze generate wireguard keypair\n\nGenerate a WireGuard keypair by invoking `wg genkey` and `wg pubkey`.\nThe system must have the `wg` binary installed."); err != nil {
			return // writing to stderr; nothing to recover
		}
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}
	if fs.NArg() > 0 {
		fmt.Fprintln(os.Stderr, "generate wireguard keypair: no arguments accepted")
		fs.Usage()
		return 1
	}
	ctx := context.Background()
	priv, err := exec.CommandContext(ctx, "wg", "genkey").Output() //nolint:gosec // no user input
	if err != nil {
		fmt.Fprintf(os.Stderr, "generate wireguard keypair: wg genkey failed: %v\n", err)
		return 1
	}
	privStr := strings.TrimSpace(string(priv))
	pubCmd := exec.CommandContext(ctx, "wg", "pubkey") //nolint:gosec // no user input
	pubCmd.Stdin = strings.NewReader(privStr + "\n")
	pub, err := pubCmd.Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "generate wireguard keypair: wg pubkey failed: %v\n", err)
		return 1
	}
	fmt.Printf("private: %s\n", privStr)
	fmt.Printf("public:  %s\n", strings.TrimSpace(string(pub)))
	return 0
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
// resolve `ping` and `traceroute` to the expected binaries. Running ze
// under a hardened PATH (or with explicit tool paths in a future config
// option) is the caller's responsibility.
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
