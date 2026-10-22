package bgp

import (
	"net/netip"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// ospfSRAnycastCheckerBranches drives each predicate of the SR anycast next-hop checker
// in both polarities, each false case written against one stated wrong reading.
// VALIDATES: the pop toward the SPF next hop, the Originator-before-next-hop order and
// the per-row Full read each accept only the state the scenario names.
// PREVENTS: a swap (the pre-fix Originator-only reading), a lab whose next hop became the
// Originator, or another neighbor's Full passing the checker.
func ospfSRAnycastCheckerBranches(t *testing.T) {
	nextHop := netip.MustParseAddr("172.30.0.3")

	t.Run("pop toward the next hop", func(t *testing.T) {
		const table = "15000 via inet 172.30.0.3 dev eth0\n16100 via inet 172.30.0.3 dev eth0\n"
		if err := ospfSRAnycastPopVerdict(table, ospfSRAnycastInLabel, nextHop); err != nil {
			t.Fatalf("a pop toward the next hop was refused: %v", err)
		}
	})
	t.Run("swap is the pre-fix behaviour", func(t *testing.T) {
		const table = "16100 as to 17100 via inet 172.30.0.3 dev eth0\n"
		if err := ospfSRAnycastPopVerdict(table, ospfSRAnycastInLabel, nextHop); err == nil {
			t.Fatal("a swap toward the next hop, the Originator-only reading, was accepted")
		}
	})
	t.Run("pop toward the other advertiser", func(t *testing.T) {
		const table = "16100 via inet 172.30.0.15 dev eth0\n"
		if err := ospfSRAnycastPopVerdict(table, ospfSRAnycastInLabel, nextHop); err == nil {
			t.Fatal("a pop toward the Originator, which is not the SPF next hop, was accepted")
		}
	})
	t.Run("another label popped", func(t *testing.T) {
		const table = "116100 via inet 172.30.0.3 dev eth0\n15000 via inet 172.30.0.3 dev eth0\n"
		if err := ospfSRAnycastPopVerdict(table, ospfSRAnycastInLabel, nextHop); err == nil {
			t.Fatal("a table without in-label 16100 was accepted on a label that only contains it")
		}
	})
	t.Run("empty table", func(t *testing.T) {
		if err := ospfSRAnycastPopVerdict("", ospfSRAnycastInLabel, nextHop); err == nil {
			t.Fatal("an empty label table was accepted")
		}
	})

	const originatorFirst = `[{"area-opaque":[]},{"extended-prefix":[
		{"advertising-router":"192.0.2.2","opaque-id":1,"prefixes":[{"prefix":"192.0.2.2/32","sub-tlvs":[{"type":2}]}]},
		{"advertising-router":"192.0.2.1","opaque-id":1,"prefixes":[{"prefix":"10.255.255.100/32","sub-tlvs":[{"type":2}]}]},
		{"advertising-router":"192.0.2.3","opaque-id":1,"prefixes":[{"prefix":"10.255.255.100/32","sub-tlvs":[{"type":2}]}]}]}]`
	t.Run("originator before next hop", func(t *testing.T) {
		if err := ospfSRAnycastOriginatorVerdict(originatorFirst); err != nil {
			t.Fatalf("the condition the scenario tests was refused: %v", err)
		}
	})
	t.Run("next hop is the originator", func(t *testing.T) {
		const nextHopFirst = `[{"extended-prefix":[
			{"advertising-router":"192.0.2.3","opaque-id":1,"prefixes":[{"prefix":"10.255.255.100/32","sub-tlvs":[{"type":2}]}]},
			{"advertising-router":"192.0.2.1","opaque-id":2,"prefixes":[{"prefix":"10.255.255.100/32","sub-tlvs":[{"type":2}]}]}]}]`
		if err := ospfSRAnycastOriginatorVerdict(nextHopFirst); err == nil {
			t.Fatal("a lab whose next hop is the Originator, which passes either way, was accepted")
		}
	})
	t.Run("prefix without a Prefix-SID", func(t *testing.T) {
		const noSID = `[{"extended-prefix":[
			{"advertising-router":"192.0.2.1","opaque-id":1,"prefixes":[{"prefix":"10.255.255.100/32","sub-tlvs":[{"type":2}]}]},
			{"advertising-router":"192.0.2.3","opaque-id":1,"prefixes":[{"prefix":"10.255.255.100/32"}]}]}]`
		if err := ospfSRAnycastOriginatorVerdict(noSID); err == nil {
			t.Fatal("a next hop that advertised the prefix with no Prefix-SID was counted as advertising the SID")
		}
	})
	t.Run("one router twice", func(t *testing.T) {
		const twice = `[{"extended-prefix":[
			{"advertising-router":"192.0.2.1","opaque-id":1,"prefixes":[{"prefix":"10.255.255.100/32","sub-tlvs":[{"type":2}]}]},
			{"advertising-router":"192.0.2.1","opaque-id":2,"prefixes":[{"prefix":"10.255.255.100/32","sub-tlvs":[{"type":2}]}]}]}]`
		if err := ospfSRAnycastOriginatorVerdict(twice); err == nil {
			t.Fatal("one router advertising the SID twice, which RFC 8665 Section 5 ignores, was accepted")
		}
	})
	t.Run("undecodable answer", func(t *testing.T) {
		if err := ospfSRAnycastOriginatorVerdict("error: not connected"); err == nil {
			t.Fatal("a reply that is not JSON was accepted")
		}
	})

	zeRouter := ospfSRAnycastZeRouter
	t.Run("full with Ze", func(t *testing.T) {
		const neighbors = "Neighbor ID     Pri State           Up Time         Dead Time Address         Interface\n" +
			"192.0.2.2       100 Full/DR         10.1s             35.0s 172.30.0.2      eth0:172.30.0.3\n"
		if !ospfSRAnycastFullWith(neighbors, zeRouter) {
			t.Fatal("a Full row naming Ze was refused")
		}
	})
	t.Run("full with another router only", func(t *testing.T) {
		const neighbors = "192.0.2.2       100 ExStart/DR      1.1s              35.0s 172.30.0.2      eth0:172.30.0.3\n" +
			"192.0.2.1         1 Full/Backup     10.1s             35.0s 172.30.0.15     eth0:172.30.0.3\n"
		if ospfSRAnycastFullWith(neighbors, zeRouter) {
			t.Fatal("Full on another neighbor's row was read as Full with Ze")
		}
	})
}

// TestZeSysctlArguments covers both polarities of the ze-sysctls reader: a net.* assignment
// becomes a --sysctl argument, and a token outside net.*, without '=' or without a value is
// refused rather than handed to docker.
// VALIDATES: ze-sysctls becomes --sysctl arguments on Ze's container.
// PREVENTS: a host sysctl outside the network namespace, or a malformed token, reaching docker.
func TestZeSysctlArguments(t *testing.T) {
	write := func(t *testing.T, content string) string {
		t.Helper()
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, zeSysctlsFile), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		return dir
	}
	t.Run("absent file", func(t *testing.T) {
		arguments, err := zeSysctlArguments(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		if len(arguments) != 0 {
			t.Fatalf("arguments = %v, want none", arguments)
		}
	})
	t.Run("net assignments", func(t *testing.T) {
		arguments, err := zeSysctlArguments(write(t, "net.mpls.platform_labels=1048575\nnet.ipv4.ip_forward=1\n"))
		if err != nil {
			t.Fatal(err)
		}
		want := []string{"--sysctl", "net.mpls.platform_labels=1048575", "--sysctl", "net.ipv4.ip_forward=1"}
		if !slices.Equal(arguments, want) {
			t.Fatalf("arguments = %v, want %v", arguments, want)
		}
	})
	for name, content := range map[string]string{
		"outside net":  "kernel.pid_max=4096\n",
		"no equals":    "net.mpls.platform_labels\n",
		"empty value":  "net.mpls.platform_labels=\n",
		"lookalike":    "netx.foo=1\n",
		"second wrong": "net.ipv4.ip_forward=1 vm.swappiness=10\n",
	} {
		t.Run(name, func(t *testing.T) {
			if arguments, err := zeSysctlArguments(write(t, content)); err == nil {
				t.Fatalf("%q was accepted as %v", content, arguments)
			}
		})
	}
}
