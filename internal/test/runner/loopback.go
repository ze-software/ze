// Design: docs/architecture/testing/ci-format.md -- loopback addresses a test needs
//
// A fixture binds addresses the host must already carry. IPv4 loopback is cheap:
// Linux routes all of 127.0.0.0/8 to lo, and on BSD the kernel can be asked for
// an alias. IPv6 has exactly ONE loopback address, ::1, so a fixture whose two
// session ends must differ (RFC 4271 Section 5.1.3 forbids a peer its own
// address as NEXT_HOP) needs a second address really added to the interface.
//
// The runner does not add it, and this is measured rather than assumed:
// SIOCAIFADDR_IN6 returns EPERM to an unprivileged process on darwin, and the
// Linux equivalent needs CAP_NET_ADMIN, while `make ze-precommit-verify` runs as an
// ordinary user on both (the merge gate runs it bare on ubuntu-latest,
// .github/workflows/verify.yml). So the privilege moved to environment-setup
// time, where the IPv4 aliases already live: `make ze-dev-setup` adds the addresses
// (scripts/dev/dev-setup.py, loopback_status/apply_loopback_fix). This file only
// REPORTS, and its error carries the exact command an operator can run instead.

package runner

import (
	"fmt"
	"maps"
	"net"
	"slices"
	"strings"
)

// ensureBindAddresses makes every address a fixture binds usable, or returns the
// error naming what an operator must run.
//
// A `.ci` names such an address in two places, and both are read here:
//
//   - `ze-peer --bind <ip>` on a command line. ze-peer is the one binary in a
//     fixture that takes its own source address from its arguments.
//   - `connection { local { ip <addr> } }` inside the config the fixture embeds.
//     Ze binds that address twice over: the dialer sends from it
//     (internal/component/bgp/reactor/session.go, dialer.LocalAddr) and the
//     listener opens on it when `accept` is true
//     (internal/component/bgp/reactor/reactor_peers.go,
//     startListenerForAddressPort). A host without it fails the same way a
//     missing --bind address does.
//
// IPv4 on Linux is a no-op (127.0.0.0/8 routes to lo automatically); on
// macOS/FreeBSD an IPv4 alias is added through SIOCAIFADDR. An IPv6 address is
// probed and never added, because adding one needs a privilege the runner does
// not have (see the file comment above).
func ensureBindAddresses(rec *Record) error {
	if err := ensurePeerBindAddresses(rec.RunCommands); err != nil {
		return err
	}
	for _, config := range embeddedConfigs(rec) {
		for _, ip := range configLocalAddresses(config) {
			// A config-declared local address is not always one this host is
			// meant to carry, so the candidate filter decides before the probe.
			if !loopbackCandidate(ip) {
				continue
			}
			if err := ensureLoopbackAlias(ip); err != nil {
				return err
			}
		}
	}
	return nil
}

// ensurePeerBindAddresses makes every `ze-peer --bind <ip>` address usable.
func ensurePeerBindAddresses(cmds []RunCommand) error {
	for _, cmd := range cmds {
		if !strings.Contains(cmd.Exec, "ze-peer") {
			continue
		}
		parts := strings.Fields(cmd.Exec)
		for i, p := range parts {
			if p != "--bind" || i+1 >= len(parts) {
				continue
			}
			ip := net.ParseIP(parts[i+1])
			if ip == nil {
				continue
			}
			// The wildcard (0.0.0.0, ::) names no address, so there is nothing
			// to put on an interface: it binds every address the host has, and
			// always succeeds. test/reload/test-config-apply-ordering-swap.ci
			// and its rotation sibling use it.
			if ip.IsUnspecified() {
				continue
			}
			if err := ensureLoopbackAlias(ip); err != nil {
				return err
			}
		}
	}
	return nil
}

// embeddedConfigs returns every text a `.ci` feeds to Ze as configuration.
//
// A fixture launches the daemon as `ze -` with a stdin block, and a reload
// fixture rewrites that config from an embedded `.conf` (action=rewrite:
// source=config2.conf), so both sources carry `connection { local { ip ... } }`.
// A stdin block that is not a config contributes nothing: the ze-peer block
// holds expect/action directives, which declare no block at all.
//
// The order is stable so a host missing two addresses always names the same one
// first, whatever order the maps iterate in.
func embeddedConfigs(rec *Record) []string {
	configs := make([]string, 0, len(rec.StdinBlocks)+len(rec.TmpfsFiles))
	for _, name := range slices.Sorted(maps.Keys(rec.StdinBlocks)) {
		configs = append(configs, string(rec.StdinBlocks[name]))
	}
	for _, path := range slices.Sorted(maps.Keys(rec.TmpfsFiles)) {
		if strings.HasSuffix(path, ".conf") {
			configs = append(configs, string(rec.TmpfsFiles[path]))
		}
	}
	return configs
}

// configLocalAddresses returns the addresses a config's
// `connection { local { ip <addr> } }` blocks declare.
//
// The scan is structural rather than textual, because `local` names two
// different things in a Ze config. One is the block above. The other is a leaf
// every fixture carries, `asn { local 65000 }`, and a pattern that reads the
// token after `local` collects an AS number as though it were an address.
//
// Two properties keep them apart. A block is entered only on `{`, so `local
// 65000` opens none. An `ip` key is read only while the two innermost open
// blocks are `connection` then `local`, so the reach of one `local` ends at its
// own closing brace and never extends to the next `ip` in the file.
func configLocalAddresses(config string) []net.IP {
	var (
		found []net.IP
		open  []string // names of the blocks currently open, outermost first
		prev  string   // the last word read, which names a block when `{` follows
	)
	for _, token := range configTokens(config) {
		switch token {
		case "{":
			open = append(open, prev)
			prev = ""
		case "}":
			if len(open) > 0 {
				open = open[:len(open)-1]
			}
			prev = ""
		case ";":
			prev = ""
		default:
			if prev == "ip" && inConnectionLocal(open) {
				if ip := net.ParseIP(token); ip != nil {
					found = append(found, ip)
				}
			}
			prev = token
		}
	}
	return found
}

// inConnectionLocal reports whether the innermost open blocks are a `local`
// block inside a `connection` block.
func inConnectionLocal(open []string) bool {
	n := len(open)
	return n >= 2 && open[n-1] == "local" && open[n-2] == "connection"
}

// configDelimiters pads the three characters that carry a config's structure, so
// `local {` and `ip 192.0.2.1;` split into tokens whatever the spacing is.
var configDelimiters = strings.NewReplacer("{", " { ", "}", " } ", ";", " ; ")

// configTokens splits a config into words and delimiters. A `#` starts a
// comment and ends the line.
func configTokens(config string) []string {
	var stripped strings.Builder
	stripped.Grow(len(config))
	for line := range strings.SplitSeq(config, "\n") {
		if i := strings.IndexByte(line, '#'); i >= 0 {
			line = line[:i]
		}
		stripped.WriteString(line)
		stripped.WriteByte('\n')
	}
	return strings.Fields(configDelimiters.Replace(stripped.String()))
}

// loopbackCandidate reports whether ip is an address this host is meant to
// carry on its loopback interface, and so one whose absence an operator can act
// on with `make ze-dev-setup`.
//
// A `.ci` declares two kinds of local address. One is a loopback address the
// fixture needs the host to carry: 127.0.0.4 for a second IPv4 session end,
// fd00::2 for an IPv6 one (scripts/dev/dev-setup.py provisions both). The other
// is a routable address a config-validation fixture names and Ze never binds --
// test/parse/graceful-restart-llgr.ci declares `local { ip 192.0.2.1 }` and the
// daemon exits after parsing. Probing the second kind would fail a test that
// passes today, with an error naming a fix that does not apply: `make ze-dev-setup`
// adds no address outside 127.0.0.0/8 and fc00::/7.
func loopbackCandidate(ip net.IP) bool {
	// The wildcard names no address (see ensurePeerBindAddresses).
	if ip.IsUnspecified() {
		return false
	}
	if ip4 := ip.To4(); ip4 != nil {
		return ip4[0] == 127
	}
	return ip.IsLoopback() || ip.IsPrivate()
}

// loopbackBindable reports whether a TCP listener opens on ip right now.
//
// This probe is the only authority on "the host carries this address". Reading
// the interface list would answer a different question: an address can be listed
// and still refuse a bind (a tentative IPv6 address under duplicate-address
// detection is the common case), and the bind is what every fixture actually
// needs to succeed.
func loopbackBindable(ip net.IP) bool {
	ln, err := net.Listen("tcp", net.JoinHostPort(ip.String(), "0")) //nolint:noctx // probe-only, no cancellation needed
	if err != nil {
		return false
	}
	ln.Close() //nolint:errcheck // probe listener, result irrelevant
	return true
}

// loopbackMissing is the error returned when ip is not on the loopback
// interface and this process cannot put it there.
//
// The text names both routes deliberately. `make ze-dev-setup` is the supported one
// and adds every address the suite needs; the raw command covers an address the
// setup script does not know about, and tells a reader what setup would do.
func loopbackMissing(ip net.IP) error {
	return fmt.Errorf(
		"ensureLoopbackAlias: %v is not on this host's loopback interface: run `make ze-dev-setup`,"+
			" or add it by hand with `%s`", ip, loopbackIPv6AddCommand(ip))
}
