// Related: tunnel.go -- tunnelKindNames, the list this test reads
// Related: gokrazy/kernel/runtime.config, runtime.require -- the kernel side
//
// VALIDATES: every tunnel kind ze models is requested as =y in the runtime
//            kernel fragment and pinned in the require manifest.
// PREVENTS:  a kind that YANG accepts and the kernel cannot build. Adding a
//            kind to tunnelKindNames and forgetting the kernel leaves an
//            operator with a config ze validates and the daemon refuses at
//            apply time with `operation not supported`.

package iface

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// kindKernelSymbol maps each modeled tunnel kind to the kernel symbol that
// builds it. Nine kinds reach the kernel through six symbols: ip_gre.c
// registers both "gre" and "gretap", ip6_gre.c registers both "ip6gre" and
// "ip6gretap", and ipip6 is ip6tnl with the inner protocol set to IPIP.
//
// The map lives in the test because it pins two lists that have no other
// contact: tunnelKindNames in Go, and the kernel fragment in gokrazy/kernel.
// A kind added to tunnelKindNames and not to this map fails the test, which is
// the drift this test exists to catch.
var kindKernelSymbol = map[TunnelKind]string{
	TunnelKindGRE:       "CONFIG_NET_IPGRE",
	TunnelKindGRETap:    "CONFIG_NET_IPGRE",
	TunnelKindIP6GRE:    "CONFIG_IPV6_GRE",
	TunnelKindIP6GRETap: "CONFIG_IPV6_GRE",
	TunnelKindIPIP:      "CONFIG_NET_IPIP",
	TunnelKindSIT:       "CONFIG_IPV6_SIT",
	TunnelKindIP6Tnl:    "CONFIG_IPV6_TUNNEL",
	TunnelKindIPIP6:     "CONFIG_IPV6_TUNNEL",
	TunnelKindVxlan:     "CONFIG_VXLAN",
}

// TestTunnelKindsAllHaveKernelSupport asserts that every kind ze models is
// requested in the runtime kernel fragment and pinned in the require manifest.
//
// A kind with no symbol is a config ze validates and the kernel refuses:
// CreateTunnel gets `operation not supported` from RTM_NEWLINK. That was the
// state of eight of the nine kinds until 2026-08-11.
//
// The require manifest is the half that matters most. enforce_required_symbols
// (tools/kernel-builder/build.py) accepts =y alone, so a symbol listed there
// cannot silently resolve to =m after a kernel bump.
func TestTunnelKindsAllHaveKernelSupport(t *testing.T) {
	root := repoRoot(t)
	required := requiredSymbols(t, filepath.Join(root, "gokrazy", "kernel", "runtime.require"))
	requested := requestedSymbols(t, filepath.Join(root, "gokrazy", "kernel", "runtime.config"))

	for kind, name := range tunnelKindNames {
		symbol, ok := kindKernelSymbol[kind]
		if !ok {
			t.Errorf("tunnel kind %q has no kernel symbol in kindKernelSymbol: "+
				"add the symbol to gokrazy/kernel/runtime.config, to "+
				"runtime.require, and to the map in this file", name)
			continue
		}
		if !requested[symbol] {
			t.Errorf("tunnel kind %q needs %s, which gokrazy/kernel/runtime.config "+
				"does not request as =y", name, symbol)
		}
		if !required[symbol] {
			t.Errorf("tunnel kind %q needs %s, which gokrazy/kernel/runtime.require "+
				"does not pin: a kernel bump can answer =m and nothing fails", name, symbol)
		}
	}
}

// repoRoot returns the repository root relative to this package directory.
func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	return root
}

// requiredSymbols reads a kernel require manifest. Each line is a bare
// CONFIG_SYMBOL or CONFIG_SYMBOL=y; blank lines and comments carry nothing.
func requiredSymbols(t *testing.T, path string) map[string]bool {
	t.Helper()
	symbols := map[string]bool{}
	for _, line := range readLines(t, path) {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		symbols[strings.TrimSuffix(line, "=y")] = true
	}
	return symbols
}

// requestedSymbols reads a kernel config fragment and returns the symbols it
// asks to build in. A `=m` or `=n` line is not a request for a built-in, and a
// module of this kernel cannot load in a QEMU run, so only `=y` counts here.
func requestedSymbols(t *testing.T, path string) map[string]bool {
	t.Helper()
	symbols := map[string]bool{}
	for _, line := range readLines(t, path) {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		name, value, found := strings.Cut(line, "=")
		if !found || value != "y" {
			continue
		}
		symbols[name] = true
	}
	return symbols
}

func readLines(t *testing.T, path string) []string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer func() { _ = f.Close() }()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return lines
}
