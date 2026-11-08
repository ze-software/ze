// Design: docs/features/dns-resolver.md -- the host's resolv.conf as a received record
// Related: resolv.go -- the registry that holds the record

package system

import (
	"errors"
	"net/netip"
	"os"

	"github.com/ze-software/ze/internal/core/dnssource"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// readHost answers the nameserver addresses of the host's resolv.conf, in
// file order, for the host origin's record. It answers none when no host
// path is set, when the host file is Ze's own resolv.conf (directly or
// through a symlink: Ze would read back what it wrote), or when the file
// cannot be read. A failure other than an absent file, and a nameserver that
// is not an IP address, are logged at WARN. The caller MUST hold r.mu.
func (r *Resolvers) readHost() []netip.Addr {
	if r.hostPath == "" {
		return nil
	}
	if sameFile(r.hostPath, r.path) {
		return nil
	}
	servers, rejected, err := dnssource.ReadResolvConf(r.hostPath)
	if errors.Is(err, dnssource.ErrNoResolvConf) {
		return nil
	}
	if err != nil {
		slogutil.Logger("resolv").Warn("host resolv.conf not read", "path", r.hostPath, "err", err)
	}
	for _, text := range rejected {
		slogutil.Logger("resolv").Warn("resolver server dropped: not an IP address",
			"origin", hostResolverOrigin.name, "path", r.hostPath, "server", text)
	}
	return servers
}

// sameFile reports whether host and own name one file. An own path that is
// empty or does not exist yet is not the host file: when the host path is a
// symlink to it, the read that follows fails on the dangling link.
func sameFile(host, own string) bool {
	if own == "" {
		return false
	}
	hostInfo, err := os.Stat(host)
	if err != nil {
		return false
	}
	ownInfo, err := os.Stat(own)
	if err != nil {
		return false
	}
	return os.SameFile(hostInfo, ownInfo)
}
