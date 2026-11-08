// Design: docs/features/dns-resolver.md -- where a local `ze resolve` command finds its servers
// Related: cmd_dns.go -- ze resolve dns
// Related: cmd_cymru.go -- ze resolve cymru

package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/ze-software/ze/internal/core/dnssource"
)

// hostResolvConfPath is where a standard Linux host keeps its resolvers.
const hostResolvConfPath = "/etc/resolv.conf"

// localServers answers the servers a local resolve command asks: the one
// --server names, else the host's resolv.conf, in file order. The local
// command runs in the caller's process with no daemon, so the host's file is
// its only source; with none, the resolver fails closed with "no DNS server
// configured". A file that exists and cannot be read is reported on stderr.
func localServers(flagServer string) []string {
	if flagServer != "" {
		return []string{flagServer}
	}
	servers, rejected, err := dnssource.ReadResolvConf(hostResolvConfPath)
	if err != nil && !errors.Is(err, dnssource.ErrNoResolvConf) {
		fmt.Fprintln(os.Stderr, "warning: "+hostResolvConfPath+" not read: "+err.Error()+"; name a server with --server <address>")
	}
	for _, text := range rejected {
		fmt.Fprintln(os.Stderr, "warning: "+hostResolvConfPath+" names "+text+", which is not an IP address; it is skipped")
	}
	out := make([]string, len(servers))
	for i, addr := range servers {
		out[i] = addr.String()
	}
	return out
}
