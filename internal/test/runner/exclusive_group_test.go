// VALIDATES: option=exclusive:group= parsing, and the invariant that every ddos
//            functional test carries the group so none of them can run
//            concurrently with a sibling.
// PREVENTS:  a new ddos test being added without isolation and silently
//            corrupting its siblings' victim attribution (QEMU pass 4: test 155
//            resolved 127.0.0.4, which belongs to test 157, and test 158 resolved
//            no victim at all).

package runner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func parseExclusiveLine(t *testing.T, line string) (*Record, error) {
	t.Helper()
	et := &EncodingTests{}
	r := newRecord("exclusive-test")
	return r, et.parseLine(r, "test/plugin/fake.ci", line)
}

func TestExclusiveGroupParsesName(t *testing.T) {
	r, err := parseExclusiveLine(t, "option=exclusive:group=ddos-flood")
	if err != nil {
		t.Fatalf("parseLine: %v", err)
	}
	if r.ExclusiveGroup != "ddos-flood" {
		t.Errorf("ExclusiveGroup = %q, want ddos-flood", r.ExclusiveGroup)
	}
}

// A missing group= must fail at parse time rather than default to "", which the
// scheduler reads as "no group" -- the test would run unserialized and the .ci
// author would have no signal that their isolation request was dropped.
func TestExclusiveGroupWithoutNameIsRejected(t *testing.T) {
	if _, err := parseExclusiveLine(t, "option=exclusive"); err == nil {
		t.Fatal("option=exclusive without group= was accepted; the test would silently run unserialized")
	}
}

func TestRecordWithoutExclusiveOptionHasNoGroup(t *testing.T) {
	r, err := parseExclusiveLine(t, "option=needs-linux")
	if err != nil {
		t.Fatalf("parseLine: %v", err)
	}
	if r.ExclusiveGroup != "" {
		t.Errorf("ExclusiveGroup = %q, want empty for a test that did not ask for it", r.ExclusiveGroup)
	}
}

// TestContendingFunctionalTestsDeclareExclusiveGroup is the ratchet.
//
// Both clusters below share one kernel-global resource in the QEMU VM's single
// root namespace, and unique names inside the config cannot partition it:
//
//   - ddos: every test floods a victim and its detector ranks destinations by
//     bytes over the interface those counters belong to -- the same loopback for
//     all of them. Unique victim addresses only stop the bind collision.
//   - cos: every test configures the same VLAN (vlan-id 100) on the VM's real
//     eth0, so concurrent daemons create and reconcile one device, eth0.100.
//   - ipsec: `ip xfrm state` and `ip xfrm policy` are node-wide, so a test that
//     reads them cannot tell its own SPIs and selectors from a sibling's. The two
//     rekey tests read the SPI set to watch a make-before-break replacement
//     arrive, and ipsec-teardown-leaves-nothing asserts both tables are EMPTY,
//     which any concurrent tunnel falsifies. Unique prefixes partition the POLICY
//     reads and nothing partitions the STATE reads: an SPI is random.
//   - bfd: BFD listens on the ports RFC 5881 / RFC 5883 FIX (3784/3785). Every
//     test's daemon binds the same wildcard tuple, co-existing only because
//     ze.bfd.test-parallel turns on SO_REUSEPORT -- and the kernel then hashes
//     each inbound datagram to ONE socket in that group
//     (internal/component/bfd/transport/udp_linux.go applySocketOptions), so a
//     control reply or a reflected echo meant for one daemon is delivered to a
//     sibling's. A port number an RFC fixes is the one address unique config
//     cannot partition.
//
// Non-overlap is the only property that fixes any of them, so a member without
// the group is a latent corruption of every sibling.
//
// Asserts over the file sets rather than hardcoded lists, so a newly added
// member is covered the moment it lands.
func TestContendingFunctionalTestsDeclareExclusiveGroup(t *testing.T) {
	// selector is the line a file must contain to BE a member of the cluster.
	// ddos/cos contend only inside the QEMU VM, and only needs-linux tests run
	// there -- everything else skips, so it can neither corrupt nor be corrupted.
	// Requiring the option on an offline sibling (ddos-flowspec-announce asserts
	// BGP flowspec encoding and starts no flood) would buy nothing and make the
	// rule look arbitrary to the next author. BFD is the opposite case: its
	// contention is two processes on one host, so it bites on every platform, and
	// membership is exactly "this test asked to co-bind the RFC ports".
	clusters := []struct {
		dir       string
		glob      string
		selector  string
		group     string
		shared    string
		minChecks int
	}{
		{"plugin", "ddos-*.ci", "option=needs-linux", "option=exclusive:group=ddos-flood", "the loopback byte counters their detector reads", 5},
		{"plugin", "cos-*.ci", "option=needs-linux", "option=exclusive:group=cos-vlan", "the eth0.100 VLAN device they each configure", 3},
		{"plugin", "*.ci", "ze.bfd.test-parallel", "option=exclusive:group=bfd-ports", "the RFC-fixed BFD ports 3784/3785 they all co-bind", 10},
		{"ipsec", "*.ci", "option=needs-linux:caps=net-admin", "option=exclusive:group=ipsec-xfrm", "the node-wide XFRM state and policy tables they each program", 3},
	}

	for _, c := range clusters {
		matches, err := filepath.Glob(filepath.Join("..", "..", "..", "test", c.dir, c.glob))
		if err != nil {
			t.Fatalf("glob %s/%s: %v", c.dir, c.glob, err)
		}

		checked := 0
		for _, path := range matches {
			data, err := os.ReadFile(path) //nolint:gosec // test fixture path from a repo-relative glob
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}
			body := string(data)
			if !strings.Contains(body, c.selector) {
				continue
			}
			checked++
			if !strings.Contains(body, c.group) {
				t.Errorf("%s does not declare %q; it will run concurrently with its siblings and corrupt %s",
					filepath.Base(path), c.group, c.shared)
			}
		}

		// Fail closed: a glob or filter that matches nothing would make the loop
		// above vacuous and the ratchet would silently stop ratcheting.
		if checked < c.minChecks {
			t.Fatalf("checked %d %s/%s files matching %q, want at least %d; the assertion above ran on nothing",
				checked, c.dir, c.glob, c.selector, c.minChecks)
		}
	}
}
