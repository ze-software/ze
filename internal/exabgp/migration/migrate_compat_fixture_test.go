// End-to-end tests over the ported ExaBGP compatibility configs in
// test/exabgp-compat/etc. They drive the same three calls `ze exabgp migrate`
// makes (ParseExaBGPConfig, MigrateFromExaBGP, SerializeTree), so a config the
// suite ships is proven to migrate here rather than only in a hand-written
// string.

package migration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// compatFixture reads one config from the ported ExaBGP compatibility suite.
func compatFixture(t *testing.T, name string) string {
	t.Helper()

	path := filepath.Join("..", "..", "..", "test", "exabgp-compat", "etc", name+".conf")
	data, err := os.ReadFile(path) //nolint:gosec // Test data path.
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return string(data)
}

// TestMigrateCompatFixtures proves each ported config migrates, and pins the
// part of the ze config that the ExaBGP config asked for.
//
// VALIDATES: the five config shapes the migration mishandled until now -- a
// named api block, two api blocks in one template, the manual-eor leaf, a
// processes-match pattern, and a flow route with a scope block.
// PREVENTS: `ze exabgp migrate` failing on a config the compatibility suite
// runs, which leaves the ported test with nothing to drive ze with.
func TestMigrateCompatFixtures(t *testing.T) {
	cases := []struct {
		fixture string
		want    []string
	}{
		{
			// `api connection { ... }` inside a template, named.
			fixture: "api-check",
			want: []string{
				"attach process exabgp-bridge",
				"run ./run/api-check.run",
				"ipv4/unicast add 127.0.0.1/32",
			},
		},
		{
			// Two named api blocks, `api speaking` and `api listening`.
			fixture: "api-api",
			want: []string{
				"attach process exabgp-bridge",
				"run ./run/api-api.receive.run",
			},
		},
		{
			// The neighbor leaf that says the script sends End-of-RIB.
			fixture: "api-manual-eor",
			want: []string{
				"manual-eor true",
				"attach process exabgp-bridge",
			},
		},
		{
			// `processes-match [ "^add" ]` in place of a literal process name.
			fixture: "api-announce-processes-match",
			want: []string{
				"attach process exabgp-bridge",
				"run ./run/api-announce.run",
			},
		},
		{
			// Two flow routes, each with a scope block of interface sets.
			fixture: "api-flow-merge",
			want: []string{
				"extended-community [0x4702caffee014001 rate-limit:0]",
				"extended-community [0x4702caffee014001 0x0702000000fe80fe rate-limit:0]",
				"ipv4/flow add source-ipv4 202.255.238.1/32",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.fixture, func(t *testing.T) {
			tree, err := ParseExaBGPConfig(compatFixture(t, tc.fixture))
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			result, err := MigrateFromExaBGP(tree)
			if err != nil {
				t.Fatalf("migrate: %v", err)
			}

			output := SerializeTree(result.Tree)
			for _, want := range tc.want {
				if !strings.Contains(output, want) {
					t.Errorf("migrated config is missing %q:\n%s", want, output)
				}
			}
		})
	}
}

// TestMigrateCompatFixturesRefusingMultiSession pins the one ExaBGP capability
// in the ported suite that ze has no implementation of.
//
// VALIDATES: the migration stops and names the capability.
// PREVENTS: the refusal being softened into a warning, which would hand the
// operator a config whose sessions negotiate less than the ExaBGP config asked
// for. ze sends no multi-session capability: capability.go declares codes 1, 2,
// 5, 6, 9, 64, 65, 69, 70, 73 and 76, and 68 is not among them. Implementing it
// is what flips this test, and the fixtures then join the table above.
func TestMigrateCompatFixturesRefusingMultiSession(t *testing.T) {
	for _, fixture := range []string{"api-multisession", "api-open"} {
		t.Run(fixture, func(t *testing.T) {
			tree, err := ParseExaBGPConfig(compatFixture(t, fixture))
			if err != nil {
				t.Fatalf("parse: %v", err)
			}

			_, err = MigrateFromExaBGP(tree)
			if err == nil {
				t.Fatal("the migration accepted a capability ze does not implement")
			}
			if !strings.Contains(err.Error(), "multi-session") {
				t.Errorf("the refusal does not name the capability: %v", err)
			}
		})
	}
}
