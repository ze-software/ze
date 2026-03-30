// Design: plan/spec-iface-2-manage.md — Interface migration CLI

package iface

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
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

	fmt.Printf("migrate: %s (unit %d) -> %s (unit %d), address %s",
		fromIface, fromUnit, toIface, toUnit, address)
	if createTyp != "" {
		fmt.Printf(", create %s", createTyp)
	}
	fmt.Printf(", timeout %s\n", timeout)

	// MigrateInterface requires a Bus, which is only available when running
	// inside the ze engine. The CLI command validates arguments and prints
	// the plan; actual migration is dispatched via the engine's RPC interface.
	fmt.Fprintf(os.Stderr, "error: migrate requires a running ze engine (use ze rpc or config)\n")
	return 1
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
	fmt.Fprintf(os.Stderr, `Usage: ze interface migrate --from <iface>.<unit> --to <iface>.<unit> --address <cidr> [options]

Perform a make-before-break IP migration between interfaces.

Five phases:
  1. Create new interface (if --create is set)
  2. Add address to new interface unit
  3. Wait for BGP readiness on new address
  4. Remove address from old interface unit
  5. Clean up old interface (if Ze-managed)

Required flags:
  --from <iface>.<unit>     Source interface and unit (e.g., eth0.0)
  --to <iface>.<unit>       Destination interface and unit (e.g., lo1.0)
  --address <cidr>          IP address to migrate (e.g., 10.0.0.1/24)

Optional flags:
  --create <type>           Create new interface: dummy, veth, bridge
  --timeout <duration>      BGP readiness timeout (default: 30s)

Examples:
  ze interface migrate --from eth0.0 --to lo1.0 --address 10.0.0.1/24
  ze interface migrate --from eth0.0 --to lo1.0 --address 10.0.0.1/24 --create dummy
  ze interface migrate --from eth0.100 --to lo2.0 --address fd00::1/64 --timeout 60s
`)
}
