package migration

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
)

// addPathNeighbor is an ExaBGP neighbor with the given capability and
// neighbor-level add-path statements.
func addPathNeighbor(capabilityAddPath, familyAddPath string) string {
	return "neighbor 192.0.2.1 {\n\tlocal-as 65001;\n\tpeer-as 65002;\n" +
		"\tcapability {\n\t\t" + capabilityAddPath + "\n\t}\n" +
		"\tfamily {\n\t\tipv4 unicast;\n\t\tipv6 unicast;\n\t}\n" +
		"\tadd-path {\n" + familyAddPath + "\t}\n}\n"
}

// migratedAddPath migrates one ExaBGP config and answers the peer's
// `session capability add-path` container, failing when it is absent.
func migratedAddPath(t *testing.T, input string) *config.Tree {
	t.Helper()
	tree, err := ParseExaBGPConfig(input)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := MigrateFromExaBGP(tree)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	peer := findPeerByRemoteIP(t, result.Tree, "192.0.2.1")
	if peer == nil {
		t.Fatalf("migrated config has no peer 192.0.2.1:\n%s", SerializeTree(result.Tree))
	}
	session := peer.GetContainer("session")
	if session == nil {
		t.Fatalf("migrated peer has no session container:\n%s", SerializeTree(result.Tree))
	}
	capability := session.GetContainer("capability")
	if capability == nil {
		t.Fatalf("migrated peer has no capability container:\n%s", SerializeTree(result.Tree))
	}
	addPath := capability.GetContainer("add-path")
	if addPath == nil {
		t.Fatalf("migrated peer has no add-path container:\n%s", SerializeTree(result.Tree))
	}
	return addPath
}

// migratedAddPathFamily answers the family entry of a migrated add-path
// container, failing when the family is absent.
func migratedAddPathFamily(t *testing.T, addPath *config.Tree, family string) *config.Tree {
	t.Helper()
	families := addPath.GetContainer("family")
	if families == nil {
		t.Fatalf("migrated add-path has no family block:\n%s", SerializeTree(addPath))
	}
	entry := families.GetContainer(family)
	if entry == nil {
		t.Fatalf("migrated add-path has no family %s:\n%s", family, SerializeTree(addPath))
	}
	return entry
}

// requireReceiveLimit fails unless the family entry carries
// `limit { receive <want>; }` and no `send` leaf beside it.
func requireReceiveLimit(t *testing.T, entry *config.Tree, want string) {
	t.Helper()
	if scalar, ok := entry.Get("limit"); ok {
		t.Fatalf("family carries the scalar `limit %s;`, want `limit { receive %s; }`", scalar, want)
	}
	limit := entry.GetContainer("limit")
	if limit == nil {
		t.Fatalf("family carries no limit container:\n%s", SerializeTree(entry))
	}
	got, ok := limit.Get("receive")
	if !ok {
		t.Fatalf("limit container carries no receive leaf:\n%s", SerializeTree(entry))
	}
	if got != want {
		t.Fatalf("limit receive = %q, want %q", got, want)
	}
	if send, ok := limit.Get("send"); ok {
		t.Fatalf("limit container carries `send %s;`; ExaBGP has no send-side limit", send)
	}
}

// TestMigrateAddPathLimitBecomesReceiveContainer proves an ExaBGP per-family
// ADD-PATH limit migrates to the receive half of ze's limit container.
//
// VALIDATES: AC-12. `add-path { ipv4 unicast limit 10; }` becomes
// `family { ipv4/unicast { limit { receive 10; } } }`, with no send leaf.
// ExaBGP's only limit is the receive request
// (`ParseAddPath._parse_addpath_family`, the spec's Q-2).
// PREVENTS: the migration writing the deleted scalar `limit 10;`, which ze
// refuses, or writing the value into the send direction.
func TestMigrateAddPathLimitBecomesReceiveContainer(t *testing.T) {
	addPath := migratedAddPath(t, addPathNeighbor("add-path send/receive;", "\t\tipv4 unicast limit 10;\n"))

	requireReceiveLimit(t, migratedAddPathFamily(t, addPath, "ipv4/unicast"), "10")
}

// TestMigrateAddPathWithoutLimitEmitsNoContainer proves a family the ExaBGP
// config names without a limit migrates with no limit at all.
//
// VALIDATES: AC-13. `add-path { ipv4 unicast; }` writes the family entry and
// neither a limit container nor a scalar limit, and a limit on one family is
// not copied onto its sibling.
// PREVENTS: an empty `limit { }` or an invented receive value that asks the
// peer for something the ExaBGP config never asked for.
func TestMigrateAddPathWithoutLimitEmitsNoContainer(t *testing.T) {
	addPath := migratedAddPath(t, addPathNeighbor("add-path send/receive;",
		"\t\tipv4 unicast;\n\t\tipv6 unicast limit 4;\n"))

	bare := migratedAddPathFamily(t, addPath, "ipv4/unicast")
	if limit := bare.GetContainer("limit"); limit != nil {
		t.Fatalf("family with no ExaBGP limit carries a limit container:\n%s", SerializeTree(bare))
	}
	if scalar, ok := bare.Get("limit"); ok {
		t.Fatalf("family with no ExaBGP limit carries `limit %s;`", scalar)
	}
	requireReceiveLimit(t, migratedAddPathFamily(t, addPath, "ipv6/unicast"), "4")
}

// TestMigrateAddPathDirectionAndLimitTogether proves the capability-level
// direction and the neighbor-level limit both survive one migration.
//
// VALIDATES: `capability { add-path receive; }` with
// `add-path { ipv4 unicast limit 10; }` writes `direction receive` on the
// add-path container and `limit { receive 10; }` on the family.
// PREVENTS: the limit rewrite dropping the direction, or the default
// send/receive replacing the direction the ExaBGP config stated.
func TestMigrateAddPathDirectionAndLimitTogether(t *testing.T) {
	addPath := migratedAddPath(t, addPathNeighbor("add-path receive;", "\t\tipv4 unicast limit 10;\n"))

	direction, ok := addPath.Get("direction")
	if !ok {
		t.Fatalf("migrated add-path carries no direction:\n%s", SerializeTree(addPath))
	}
	if direction != "receive" {
		t.Fatalf("direction = %q, want %q", direction, "receive")
	}
	requireReceiveLimit(t, migratedAddPathFamily(t, addPath, "ipv4/unicast"), "10")
}

// TestMigrateAddPathLimitLoadsInZe proves ze's own parser accepts the
// migrated limit container, so `ze exabgp migrate` hands the operator a
// config that loads.
//
// VALIDATES: the migrated `limit { receive 10; }` matches ze's YANG schema.
// PREVENTS: the migration and the schema disagreeing about the limit
// spelling, which fails the converted config at load.
func TestMigrateAddPathLimitLoadsInZe(t *testing.T) {
	output := migrateNeighborOutput(t, addPathNeighbor("add-path send/receive;", "\t\tipv4 unicast limit 10;\n"))
	if !strings.Contains(output, "receive 10") {
		t.Fatalf("migrated config carries no `receive 10`:\n%s", output)
	}
	if err := parseZeConfig(t, output); err != nil {
		t.Fatalf("ze refuses the migrated config: %v\n%s", err, output)
	}
}
