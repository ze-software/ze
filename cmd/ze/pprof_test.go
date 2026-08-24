// Design: (none -- pprof build-tag gate for TinyGo compatibility)

//go:build !tinygo && ze_core

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestIsLocalhostPprof verifies pprof address localhost validation.
//
// VALIDATES: Only loopback addresses are accepted for pprof.
// PREVENTS: Exposing pprof on public interfaces (CWE-200).
//
// It carries pprof.go's own build constraint. It lived in main_test.go, which
// carries `//go:build ze_core` and not `!tinygo`, so `-tags 'ze_core tinygo'`
// selected the test and dropped the function it calls: isLocalhostPprof is
// defined in pprof.go alone, which that build excludes. cmd/ze then failed to
// type-check, and no lint or vet pass could read pprof_tinygo.go at all.
func TestIsLocalhostPprof(t *testing.T) {
	tests := []struct {
		name string
		addr string
		want bool
	}{
		{"localhost_ipv4", "127.0.0.1:6060", true},
		{"localhost_ipv6", "[::1]:6060", true},
		{"localhost_name", "localhost:6060", true},
		{"all_interfaces", "0.0.0.0:6060", false},
		{"empty_host", ":6060", false},
		{"public_ip", "192.168.1.1:6060", false},
		{"ipv6_all", "[::]:6060", false},
		{"no_port", "127.0.0.1", false},
		{"garbage", "not-an-address", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isLocalhostPprof(tt.addr)
			assert.Equal(t, tt.want, got, "isLocalhostPprof(%q)", tt.addr)
		})
	}
}
