// Design: plan/learned/491-iface-2-manage.md — Interface migration CLI

package iface

import (
	"errors"
	"flag"
	"fmt"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"time"

	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/helpfmt"
	sshclient "codeberg.org/thomas-mangin/ze/cmd/ze/internal/ssh/client"
)

// cmdMigrate handles: ze interface migrate --from <iface>.<unit> --to <iface>.<unit> --address <cidr> [--create <type>] [--timeout <duration>]
// Returns exit code.
func cmdMigrate(args []string) int {
	fs := flag.NewFlagSet("ze interface migrate", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var (
		from      string
		to        string
		address   string
		createTyp string
		timeout   time.Duration
	)

	fs.StringVar(&from, "from", "", "source interface.unit (e.g., eth0.0)")
	fs.StringVar(&to, "to", "", "destination interface.unit (e.g., lo1.0)")
	fs.StringVar(&address, "address", "", "CIDR address to migrate (e.g., 10.0.0.1/24)")
	fs.StringVar(&createTyp, "create", "", "create new interface of type: dummy, veth, bridge")
	fs.DurationVar(&timeout, "timeout", 30*time.Second, "BGP readiness timeout")

	fs.Usage = func() { migrateUsage() }

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}

	if from == "" || to == "" || address == "" {
		fmt.Fprintf(os.Stderr, "error: --from, --to, and --address are required\n")
		migrateUsage()
		return 1
	}

	fromIface, fromUnit, ok := parseIfaceUnit(from)
	if !ok {
		fmt.Fprintf(os.Stderr, "error: invalid --from value %q (expected <name>.<unit>)\n", from)
		return 1
	}

	toIface, toUnit, ok := parseIfaceUnit(to)
	if !ok {
		fmt.Fprintf(os.Stderr, "error: invalid --to value %q (expected <name>.<unit>)\n", to)
		return 1
	}

	// Validate address is a valid CIDR before sending to daemon.
	if _, err := netip.ParsePrefix(address); err != nil {
		fmt.Fprintf(os.Stderr, "error: invalid --address %q: %v\n", address, err)
		return 1
	}

	// Validate --create type against known set.
	if createTyp != "" {
		switch createTyp {
		case "dummy", "veth", "bridge": // valid
		default:
			fmt.Fprintf(os.Stderr, "error: invalid --create type %q (expected dummy, veth, or bridge)\n", createTyp)
			return 1
		}
	}

	// Build the command to dispatch via SSH to the running daemon.
	cmd := fmt.Sprintf("interface migrate --from %s.%d --to %s.%d --address %s",
		fromIface, fromUnit, toIface, toUnit, address)
	if createTyp != "" {
		cmd += " --create " + createTyp
	}
	if timeout != 30*time.Second {
		cmd += " --timeout " + timeout.String()
	}

	creds, err := sshclient.LoadCredentials()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		fmt.Fprintf(os.Stderr, "hint: run 'ze init' first, or start the daemon with 'ze start'\n")
		return 1
	}

	result, err := sshclient.ExecCommand(creds, cmd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	if result != "" {
		fmt.Println(result)
	}
	return 0
}

// parseIfaceUnit splits "<name>.<unit>" into name and unit number.
// Returns false if the format is invalid.
func parseIfaceUnit(s string) (string, int, bool) {
	idx := strings.LastIndex(s, ".")
	if idx <= 0 || idx == len(s)-1 {
		return "", 0, false
	}

	name := s[:idx]
	unitStr := s[idx+1:]

	unit, err := strconv.Atoi(unitStr)
	if err != nil || unit < 0 {
		return "", 0, false
	}

	return name, unit, true
}

func migrateUsage() {
	p := helpfmt.Page{
		Command: "ze interface migrate",
		Summary: "Perform a make-before-break IP migration between interfaces",
		Usage:   []string{"ze interface migrate --from <iface>.<unit> --to <iface>.<unit> --address <cidr> [options]"},
		Sections: []helpfmt.HelpSection{
			{Title: "Five phases", Entries: []helpfmt.HelpEntry{
				{Name: "1.", Desc: "Create new interface (if --create is set)"},
				{Name: "2.", Desc: "Add address to new interface unit"},
				{Name: "3.", Desc: "Wait for BGP readiness on new address"},
				{Name: "4.", Desc: "Remove address from old interface unit"},
				{Name: "5.", Desc: "Clean up old interface (if Ze-managed)"},
			}},
			{Title: "Required flags", Entries: []helpfmt.HelpEntry{
				{Name: "--from <iface>.<unit>", Desc: "Source interface and unit (e.g., eth0.0)"},
				{Name: "--to <iface>.<unit>", Desc: "Destination interface and unit (e.g., lo1.0)"},
				{Name: "--address <cidr>", Desc: "IP address to migrate (e.g., 10.0.0.1/24)"},
			}},
			{Title: "Optional flags", Entries: []helpfmt.HelpEntry{
				{Name: "--create <type>", Desc: "Create new interface: dummy, veth, bridge"},
				{Name: "--timeout <duration>", Desc: "BGP readiness timeout (default: 30s)"},
			}},
		},
		Examples: []string{
			"ze interface migrate --from eth0.0 --to lo1.0 --address 10.0.0.1/24",
			"ze interface migrate --from eth0.0 --to lo1.0 --address 10.0.0.1/24 --create dummy",
			"ze interface migrate --from eth0.100 --to lo2.0 --address fd00::1/64 --timeout 60s",
		},
	}
	p.Write()
}
