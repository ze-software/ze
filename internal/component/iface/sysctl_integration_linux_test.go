//go:build integration && linux

package iface

import (
	"os"
	"testing"

)
func TestIntegrationSysctlIPv4Forwarding(t *testing.T) {
	// VALIDATES: SetIPv4Forwarding writes to real /proc/sys in a namespace.
	// PREVENTS: Sysctl writes only work with test-overridden sysctlRoot.
	withNetNS(t, func() {
		ensureBackendForIntegration(t)
		createDummyForTest(t, "test0")

		if err := SetIPv4Forwarding("test0", true); err != nil {
			t.Fatalf("SetIPv4Forwarding(true): %v", err)
		}

		data, err := os.ReadFile("/proc/sys/net/ipv4/conf/test0/forwarding")
		if err != nil {
			t.Fatalf("read forwarding: %v", err)
		}
		got := string(data)
		if got != "1" && got != "1\n" {
			t.Errorf("forwarding = %q, want %q", got, "1")
		}
	})
}

func TestIntegrationSysctlSLAAC(t *testing.T) {
	// VALIDATES: EnableSLAAC writes to real /proc/sys/net/ipv6/conf/<iface>/autoconf.
	// PREVENTS: SLAAC control only works in mocked sysctl environment.
	withNetNS(t, func() {
		ensureBackendForIntegration(t)
		createDummyForTest(t, "test0")

		if err := EnableSLAAC("test0"); err != nil {
			t.Fatalf("EnableSLAAC: %v", err)
		}

		data, err := os.ReadFile("/proc/sys/net/ipv6/conf/test0/autoconf")
		if err != nil {
			t.Fatalf("read autoconf: %v", err)
		}
		got := string(data)
		if got != "1" && got != "1\n" {
			t.Errorf("autoconf = %q, want %q", got, "1")
		}
	})
}
