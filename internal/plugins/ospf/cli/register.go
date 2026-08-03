// Design: plan/learned/1169-cli-root-namespace-grammar.md -- ospf root namespace (decode member)
//
// Owner package: the offline OSPFv2 packet decode tool lives with the
// internal/plugins/ospf codec, not under cmd/ze. It registers the `ospf` root
// namespace and routes its single member `decode`, so the command is
// `ze ospf decode`. `ospf` is a real object namespace: the YANG container `ospf`
// (internal/plugins/ospf/yang/ze-ospf-conf.yang, ze-ospf-cmd.yang) and the
// `show ospf` command tree live under it, so a hyphenated `ospf-decode` root was
// a namespace member masquerading as an indivisible compound (R9,
// ai/rules/cli.md).
//
// codegen:skip -- the `ospf` root is wired into the ze CLI by
// cmd/ze/dispatch_ospf.go's direct blank import (ze_core && ze_ospf), so it must
// NOT also be discovered into plugin/all. internal/test/cli imports plugin/all
// (for editor tests) and separately registers a `ze-test ospf` SUITE root; a
// second `ospf` tool root pulled in through plugin/all would panic on duplicate
// registration under the ze_ospf tag. One `ospf` root per binary.

package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/ze-software/ze/internal/component/command/registry"
)

func init() {
	registry.MustRegisterRootHandler("ospf", dispatchOSPF, registry.Meta{
		Description: "Offline OSPF wire tools",
		Mode:        "offline",
		Section:     registry.SectionConfiguration,
		Subs:        "decode [--pretty]",
	})
}

// ospfMembers is the closed set of `ze ospf` sub-tokens. Matching args[0] against
// it before doing anything with the token keeps an unknown sub-command from ever
// reaching the codec (ai/rules/cli.md, closed keyword set).
var ospfMembers = map[string]bool{"decode": true}

// dispatchOSPF routes `ze ospf <member> ...` to the owning tool. A bare
// `ze ospf` or a help flag lists the members so the namespace is discoverable
// (R-6); an unknown member errors with the member list rather than falling
// through to Run.
func dispatchOSPF(_ *registry.RuntimeContext, args []string) int {
	if len(args) == 0 {
		ospfUsage(os.Stderr)
		return 1
	}
	if isHelpArg(args[0]) {
		ospfUsage(os.Stdout)
		return 0
	}
	if !ospfMembers[args[0]] {
		fmt.Fprintf(os.Stderr, "unknown ospf command: %s\n", args[0])
		ospfUsage(os.Stderr)
		return 1
	}
	// decode is the only member: the offline OSPFv2 packet decoder.
	return Run(args[1:])
}

func ospfUsage(w io.Writer) {
	fmt.Fprintln(w, "usage: ze ospf <command>")                                            //nolint:errcheck // CLI usage output
	fmt.Fprintln(w, "commands:")                                                           //nolint:errcheck // CLI usage output
	fmt.Fprintln(w, "  decode [--pretty]   Decode a hex OSPFv2 packet from stdin to JSON") //nolint:errcheck // CLI usage output
}
