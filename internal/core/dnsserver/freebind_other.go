// Design: docs/architecture/dns/server-harness.md -- non-Linux Freebind fallback
//
// IP_FREEBIND is Linux-specific; on other platforms Freebind is accepted but
// has no effect (matches a bare net.ListenConfig, which never sets it either).

//go:build !linux

package dnsserver

import "net"

func listenConfig(_ bool) net.ListenConfig {
	return net.ListenConfig{}
}
