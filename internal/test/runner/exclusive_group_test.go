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
//
// Non-overlap is the only property that fixes either, so a member without the
// group is a latent corruption of every sibling.
//
// Asserts over the file sets rather than hardcoded lists, so a newly added
// ddos-*.ci or cos-*.ci is covered the moment it lands.
func TestContendingFunctionalTestsDeclareExclusiveGroup(t *testing.T) {
	clusters := []struct {
		glob      string
		group     string
		shared    string
		minChecks int
	}{
		{"ddos-*.ci", "option=exclusive:group=ddos-flood", "the loopback byte counters their detector reads", 5},
		{"cos-*.ci", "option=exclusive:group=cos-vlan", "the eth0.100 VLAN device they each configure", 3},
	}

	for _, c := range clusters {
		matches, err := filepath.Glob(filepath.Join("..", "..", "..", "test", "plugin", c.glob))
		if err != nil {
			t.Fatalf("glob %s: %v", c.glob, err)
		}

		checked := 0
		for _, path := range matches {
			data, err := os.ReadFile(path) //nolint:gosec // test fixture path from a repo-relative glob
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}
			body := string(data)
			// Scoped to option=needs-linux: the contention exists only in the VM,
			// and only needs-linux tests execute there -- everything else skips, so
			// it can neither corrupt nor be corrupted. Requiring the option on an
			// offline sibling (ddos-flowspec-announce asserts BGP flowspec encoding
			// and starts no flood) would buy nothing and make the rule look
			// arbitrary to the next author.
			if !strings.Contains(body, "option=needs-linux") {
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
			t.Fatalf("checked %d needs-linux %s files, want at least %d; the assertion above ran on nothing",
				checked, c.glob, c.minChecks)
		}
	}
}
