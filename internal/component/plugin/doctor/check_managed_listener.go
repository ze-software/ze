// Design: docs/architecture/fleet-config.md -- one address cannot carry both the plugin acceptor and the managed listener
// Related: ../acceptor.go -- hubAcceptorServer binds Servers[0], which is the address this check compares against
// Related: ../server/managed_serve.go -- ManagedServer.Start skips a block whose address is already taken

package doctor

import (
	"net"

	"github.com/ze-software/ze/internal/component/config"
	zeplugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// codeHubManagedCollision is the diagnostic code this check publishes and raises.
const codeHubManagedCollision = "doctor-hub-managed-collision"

// checkManagedListener reads the parsed config tree and reports a hub server
// block whose managed client listener cannot bind.
//
// It is a STATELESS config-tree check, like the managed component's
// hub-unreachable check. `ze doctor` runs in the operator's own process.
// DoctorCheckContext therefore carries the parsed config and no runtime state.
// The verdict is read off the two producers that choose an address,
// hubAcceptorServer and startManagedServer. It holds before the daemon runs.
func checkManagedListener(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	tree, ok := ctx.Tree.(*config.Tree)
	if !ok || tree == nil {
		return nil
	}
	hub, err := config.ExtractHubConfig(tree)
	if err != nil {
		return nil // A malformed hub block is config validation's verdict, not this one.
	}
	return diagnoseManagedListener(hub)
}

// diagnoseManagedListener reports each server block that declares managed
// clients and cannot serve them, because the plugin acceptor already holds its
// address.
//
// Two producers pick an address from the same list, and neither knows about the
// other. The plugin acceptor takes Servers[0] and nothing else
// (hubAcceptorServer, ../acceptor.go). The managed listener takes every block
// that declares client entries (startManagedServer, cmd/ze/hub/managed_server.go).
// A block that both of them pick is bound by the acceptor first.
// ManagedServer.Start then logs the failed bind and skips the block. Every
// managed client that dials it is refused by an acceptor that routes only to a
// WaitForPlugin waiter.
//
// A block that carries a plugin secret AND client entries is NOT the condition.
// The central-hub configuration in docs/architecture/fleet-config.md carries
// both on its `central` block, and it works. The acceptor binds the `local`
// block ahead of it. The address is what collides.
func diagnoseManagedListener(hub zeplugin.HubConfig) []diagnostic.Diagnostic {
	if len(hub.Servers) == 0 {
		return nil // No block, so no acceptor listener and no managed listener.
	}
	acceptor := hub.Servers[0]

	var diags []diagnostic.Diagnostic
	for _, block := range hub.Servers {
		if len(block.Clients) == 0 {
			continue // Not a managed-client-serving block.
		}
		if !hubAddressesCollide(acceptor, block) {
			continue
		}
		var message textbuf.Buffer
		message.Str("hub server block ").Quoted(block.Name).
			Str(" declares managed clients on ").Str(block.Address()).
			Str(", which the plugin acceptor already binds (block ").Quoted(acceptor.Name).
			Str(" on ").Str(acceptor.Address()).
			Str("). Move the client entries to a server block with its own port")
		diags = append(diags, diagnostic.Diagnostic{
			Code:     codeHubManagedCollision,
			Severity: diagnostic.SeverityError,
			Message:  message.String(),
		})
	}
	return diags
}

// hubAddressesCollide reports whether two hub server blocks ask the kernel for
// the same listening socket.
//
// The port has to match, and the hosts have to overlap. Two named hosts overlap
// when they are equal. An unspecified host (0.0.0.0, ::, or no ip leaf at all)
// covers every address on the machine. It therefore overlaps any host on that
// port, and tls.Listen refuses the second bind with EADDRINUSE either way round.
//
// Port 0 is the one case a shared block does not collide. The kernel picks a
// free port for each listener. Such a hub still cannot be reached by a remote
// managed client. No client config can name a port the kernel has not chosen
// yet. That is a different defect and this check does not claim it.
func hubAddressesCollide(first, second zeplugin.HubServerConfig) bool {
	if first.Port != second.Port {
		return false
	}
	if first.Port == 0 {
		return false
	}
	if first.Host == second.Host {
		return true
	}
	return hubHostIsUnspecified(first.Host) || hubHostIsUnspecified(second.Host)
}

// hubHostIsUnspecified reports whether a host names every address on the
// machine. An empty ip leaf reaches tls.Listen as ":port", which binds the
// wildcard, so it counts as unspecified too.
func hubHostIsUnspecified(host string) bool {
	if host == "" {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false // A name, not an address. Resolution is the kernel's job at bind time.
	}
	return ip.IsUnspecified()
}
