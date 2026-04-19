// Design: docs/guide/command-reference.md -- `show kernel-routes` top-level shortcut
// Related: ip.go -- `show ip route` does the same work under the ip/ subtree

package show

import (
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

// defaultKernelRoutesLimit mirrors defaultIPRouteLimit in ip.go but for
// the top-level `show kernel-routes` handler. Kept as a separate const
// so the two entry points can evolve independently if one grows a
// larger cap for programmatic use.
const defaultKernelRoutesLimit = 100_000

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:kernel-routes",
			Handler:    handleShowKernelRoutes,
		},
	)
}

// handleShowKernelRoutes returns the kernel routing table via the
// iface component's active backend. A single positional argument
// restricts the output to one CIDR (use "default" for 0.0.0.0/0 /
// ::/0). The optional `--limit N` caps the response.
//
// Invalid prefixes reject with the usage line. Backends that do not
// own the kernel FIB (VPP today) reject per exact-or-reject. Shares
// the parser + truncation envelope with handleShowIPRoute via
// dumpKernelRoutes.
func handleShowKernelRoutes(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return dumpKernelRoutes(args, "usage: show kernel-routes [<cidr>|default] [--limit N]", defaultKernelRoutesLimit)
}
