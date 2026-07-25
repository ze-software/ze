// Design: docs/architecture/api/commands.md — traffic namespace ownership
//
// Register the `traffic` root command with the importable command registry and
// route its closed set of members. `traffic` is a real object namespace — the
// YANG container `traffic { control { ... } }`
// (internal/component/traffic/yang/ze-traffic-control-conf.yang) — so the
// offline tc/VPP policer tool is the member `control`, reached as
// `ze traffic control`, not a hyphenated `traffic-control` compound (R9,
// ai/rules/cli-grammar.md). This is the owner package: the tool lives with
// internal/component/traffic, not under cmd/ze.
//
// codegen:skip -- the `traffic` root is wired into the ze / ze-appliance CLI by
// cmd/ze/ze_core_dispatch.go's direct blank import (the same way firewall/iface/
// l2tp cli are), so it must NOT also be discovered into plugin/all. The ze-test
// binary imports plugin/all (for editor tests) and separately registers a
// `ze-test traffic` SUITE root; a second `traffic` tool root pulled in through
// plugin/all would panic on duplicate registration. Keeping the tool out of
// plugin/all leaves exactly one `traffic` root per binary.
package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/ze-software/ze/internal/component/command/registry"
)

func init() {
	registry.MustRegisterRootHandler("traffic", dispatchTraffic, registry.Meta{
		Description: "Linux tc / VPP policer helpers",
		Mode:        "offline",
		Section:     registry.SectionConfiguration,
		Subs:        "control",
	})
}

// trafficMembers is the closed set of `ze traffic` sub-tokens. Matching args[0]
// against it before doing anything with the token keeps an unknown sub-command
// from ever reaching the tool's arg parser (ai/rules/cli-grammar.md, closed
// keyword set).
var trafficMembers = map[string]bool{"control": true}

// dispatchTraffic routes `ze traffic <member> ...` to the owning tool. A bare
// `ze traffic` or a help flag lists the members so the namespace is discoverable
// (R-6); an unknown member errors with the member list rather than falling
// through to Run.
func dispatchTraffic(_ *registry.RuntimeContext, args []string) int {
	if len(args) == 0 {
		trafficUsage(os.Stderr)
		return 1
	}
	if isTrafficHelpArg(args[0]) {
		trafficUsage(os.Stdout)
		return 0
	}
	if !trafficMembers[args[0]] {
		fmt.Fprintf(os.Stderr, "unknown traffic command: %s\n", args[0])
		trafficUsage(os.Stderr)
		return 1
	}
	// control is the only member: the tc/VPP policer tool.
	return Run(args[1:])
}

func isTrafficHelpArg(a string) bool { return a == "-h" || a == "--help" || a == "help" }

func trafficUsage(w io.Writer) {
	fmt.Fprintln(w, "usage: ze traffic <command>")                //nolint:errcheck // CLI usage output
	fmt.Fprintln(w, "commands:")                                  //nolint:errcheck // CLI usage output
	fmt.Fprintln(w, "  control   Linux tc / VPP policer helpers") //nolint:errcheck // CLI usage output
}
