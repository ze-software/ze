// Design: docs/architecture/cli/root-namespace-grammar.md -- isis root namespace (decode member)
//
// Owner package: the offline IS-IS PDU decode tool lives with the
// internal/plugins/isis codec, not under cmd/ze. It registers the `isis` root
// namespace and routes its single member `decode`, so the command is
// `ze isis decode`. `isis` is a real object namespace: the YANG container `isis`
// (internal/plugins/isis/yang/ze-isis-conf.yang, ze-isis-cmd.yang) and the
// `show isis` / `clear isis` command tree both live under it, so a hyphenated
// `isis-decode` root was a namespace member masquerading as an indivisible
// compound (R9, ai/rules/cli.md). No collision is avoided by the hyphen:
// the `isis` config root sits under the set/delete verbs and the command tree
// under show/clear; the bare root token `isis` was simply unregistered until
// now. The sibling that supposedly collided is exactly what makes `isis` a
// namespace.
//
// codegen:skip -- the `isis` root is wired into the ze CLI by
// cmd/ze/dispatch_isis.go's direct blank import (ze_core && ze_isis), so it must
// NOT also be discovered into plugin/all. internal/test/cli imports plugin/all
// (for editor tests) and separately registers a `ze-test isis` SUITE root; a
// second `isis` tool root pulled in through plugin/all would panic on duplicate
// registration under the ze_isis tag. One `isis` root per binary.

package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/ze-software/ze/internal/component/command/registry"
)

func init() {
	registry.MustRegisterRootHandler("isis", dispatchISIS, registry.Meta{
		Description: "Offline IS-IS wire tools",
		Mode:        "offline",
		Section:     registry.SectionConfiguration,
		Subs:        "decode [--pretty]",
	})
}

// isisMembers is the closed set of `ze isis` sub-tokens. Matching args[0] against
// it before doing anything with the token keeps an unknown sub-command from ever
// reaching the codec (ai/rules/cli.md, closed keyword set).
var isisMembers = map[string]bool{"decode": true}

// dispatchISIS routes `ze isis <member> ...` to the owning tool. A bare
// `ze isis` or a help flag lists the members so the namespace is discoverable
// (R-6); an unknown member errors with the member list rather than falling
// through to Run.
func dispatchISIS(_ *registry.RuntimeContext, args []string) int {
	if len(args) == 0 {
		isisUsage(os.Stderr)
		return 1
	}
	if isHelpArg(args[0]) {
		isisUsage(os.Stdout)
		return 0
	}
	if !isisMembers[args[0]] {
		fmt.Fprintf(os.Stderr, "unknown isis command: %s\n", args[0])
		isisUsage(os.Stderr)
		return 1
	}
	// decode is the only member: the offline IS-IS PDU decoder.
	return Run(args[1:])
}

func isisUsage(w io.Writer) {
	fmt.Fprintln(w, "usage: ze isis <command>")                                        //nolint:errcheck // CLI usage output
	fmt.Fprintln(w, "commands:")                                                       //nolint:errcheck // CLI usage output
	fmt.Fprintln(w, "  decode [--pretty]   Decode a hex IS-IS PDU from stdin to JSON") //nolint:errcheck // CLI usage output
}
