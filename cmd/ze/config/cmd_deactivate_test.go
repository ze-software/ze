// Design: docs/architecture/config/syntax.md — config deactivate tests

package config

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/config/storage"
)

const deactivateTestConfig = `bgp {
	session {
		asn {
			local 65000;
		}
	}
	router-id 1.2.3.4;
	peer peer1 {
		connection {
			remote {
				ip 1.1.1.1;
			}
		}
		session {
			asn {
				remote 65001;
			}
		}
	}
	filter {
		import [ no-self-as reject-bogons ];
	}
}
`

// TestCmdDeactivateLeaf verifies the one-shot CLI marks a leaf inactive
// and the file on disk gets the `inactive: ` prefix.
//
// VALIDATES: AC-1 -- end-to-end CLI flow for the leaf case.
//
// PREVENTS: regression in path resolution / dispatch when leaf paths
// are passed to the new verb.
func TestCmdDeactivateLeaf(t *testing.T) {
	configPath := writeTestConfig(t, deactivateTestConfig)

	rc := cmdDeactivateImpl(storage.NewFilesystem(), []string{
		"--no-reload",
		configPath,
		"bgp", "router-id",
	})
	if rc != exitOK {
		t.Fatalf("exit code = %d, want %d", rc, exitOK)
	}

	content, err := os.ReadFile(configPath) //nolint:gosec // test path from t.TempDir()
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if !strings.Contains(string(content), "inactive: router-id") {
		t.Fatalf("file missing inactive prefix on leaf:\n%s", content)
	}
}

// TestCmdDeactivateContainer verifies the same dispatch for a container.
// AC-2: byte-equivalent observable behavior to the existing TUI deactivate.
func TestCmdDeactivateContainer(t *testing.T) {
	configPath := writeTestConfig(t, deactivateTestConfig)

	rc := cmdDeactivateImpl(storage.NewFilesystem(), []string{
		"--no-reload",
		configPath,
		"bgp", "peer", "peer1",
	})
	if rc != exitOK {
		t.Fatalf("exit code = %d", rc)
	}

	content, err := os.ReadFile(configPath) //nolint:gosec // test path
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if !strings.Contains(string(content), "inactive: peer") {
		t.Fatalf("file missing inactive prefix on list entry:\n%s", content)
	}
}

// TestCmdDeactivateLeafListValue verifies AC-3.
func TestCmdDeactivateLeafListValue(t *testing.T) {
	configPath := writeTestConfig(t, deactivateTestConfig)

	rc := cmdDeactivateImpl(storage.NewFilesystem(), []string{
		"--no-reload",
		configPath,
		"bgp", "filter", "import", "no-self-as",
	})
	if rc != exitOK {
		t.Fatalf("exit code = %d", rc)
	}

	content, err := os.ReadFile(configPath) //nolint:gosec // test path
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if !strings.Contains(string(content), "inactive:no-self-as") {
		t.Fatalf("leaf-list value not deactivated:\n%s", content)
	}
}

// TestCmdActivateRoundTrip verifies AC-4: activate after deactivate
// returns the file to its original observable state.
func TestCmdActivateRoundTrip(t *testing.T) {
	configPath := writeTestConfig(t, deactivateTestConfig)

	if rc := cmdDeactivateImpl(storage.NewFilesystem(), []string{
		"--no-reload", configPath, "bgp", "router-id",
	}); rc != exitOK {
		t.Fatalf("deactivate rc = %d", rc)
	}
	if rc := cmdActivateImpl(storage.NewFilesystem(), []string{
		"--no-reload", configPath, "bgp", "router-id",
	}); rc != exitOK {
		t.Fatalf("activate rc = %d", rc)
	}

	content, err := os.ReadFile(configPath) //nolint:gosec // test path
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if strings.Contains(string(content), "inactive: router-id") {
		t.Fatalf("activate did not clear leaf prefix:\n%s", content)
	}
	if !strings.Contains(string(content), "router-id 1.2.3.4") {
		t.Fatalf("value lost after round-trip:\n%s", content)
	}
}

// TestCmdDeactivateBadPath verifies AC-7: a non-existent path exits
// non-zero and leaves the file untouched.
func TestCmdDeactivateBadPath(t *testing.T) {
	configPath := writeTestConfig(t, deactivateTestConfig)
	before, err := os.ReadFile(configPath) //nolint:gosec // test path
	if err != nil {
		t.Fatalf("read before: %v", err)
	}

	rc := cmdDeactivateImpl(storage.NewFilesystem(), []string{
		"--no-reload", configPath, "no", "such", "path",
	})
	if rc == exitOK {
		t.Fatalf("expected non-zero exit on bad path")
	}

	after, err := os.ReadFile(configPath) //nolint:gosec // test path
	if err != nil {
		t.Fatalf("read after: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Fatalf("file modified despite error\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

// TestCmdDeactivateAlreadyInactive verifies AC-8: re-running deactivate
// on an already-inactive leaf is idempotent -- exits 0 with a "no
// change" status. The Editor primitive errors so callers can distinguish;
// the CLI verb swallows the sentinel for the script-friendly path.
func TestCmdDeactivateAlreadyInactive(t *testing.T) {
	configPath := writeTestConfig(t, deactivateTestConfig)

	if rc := cmdDeactivateImpl(storage.NewFilesystem(), []string{
		"--no-reload", configPath, "bgp", "router-id",
	}); rc != exitOK {
		t.Fatalf("first deactivate rc = %d", rc)
	}

	rc := cmdDeactivateImpl(storage.NewFilesystem(), []string{
		"--no-reload", configPath, "bgp", "router-id",
	})
	if rc != exitOK {
		t.Fatalf("second deactivate on already-inactive leaf must be idempotent (rc=%d)", rc)
	}
}

// TestCmdDeactivateMissingArgs verifies the usage check.
func TestCmdDeactivateMissingArgs(t *testing.T) {
	rc := cmdDeactivateImpl(storage.NewFilesystem(), []string{"--no-reload", "only-one-arg"})
	if rc == exitOK {
		t.Fatalf("expected non-zero exit when args insufficient")
	}
}

// TestCmdDeactivatePositionalListEntry verifies AC-12: a positional list
// entry (nlri, nexthop, add-path -- lists with all-leaf children whose
// schema does not carry an injected inactive leaf) is rejected with a
// message pointing at the parent container.
//
// PREVENTS: silent acceptance that would write an unknown `inactive`
// leaf into the entry tree, producing a config that does not round-trip.
func TestCmdDeactivatePositionalListEntry(t *testing.T) {
	configWithNexthop := `bgp {
	session {
		asn {
			local 65000;
		}
	}
	router-id 1.2.3.4;
	peer peer1 {
		connection {
			remote {
				ip 1.1.1.1;
			}
		}
		session {
			asn {
				remote 65001;
			}
			capability {
				nexthop ipv4/unicast {
					nhafi ipv6;
					mode enable;
				}
			}
		}
	}
}
`
	configPath := writeTestConfig(t, configWithNexthop)
	before, err := os.ReadFile(configPath) //nolint:gosec // test path
	if err != nil {
		t.Fatalf("read before: %v", err)
	}

	rc := cmdDeactivateImpl(storage.NewFilesystem(), []string{
		"--no-reload", configPath,
		"bgp", "peer", "peer1", "session", "capability", "nexthop", "ipv4/unicast",
	})
	if rc == exitOK {
		t.Fatalf("expected non-zero exit on positional list entry")
	}

	after, err := os.ReadFile(configPath) //nolint:gosec // test path
	if err != nil {
		t.Fatalf("read after: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Fatalf("file modified despite rejection")
	}
}
