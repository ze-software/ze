package migration

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
)

// autoFlushNeighbor is an ExaBGP neighbor that states auto-flush and
// group-updates, so the migrated peer carries a behavior container.
const autoFlushNeighbor = "neighbor 10.0.0.1 {\n\tlocal-as 65001\n\tpeer-as 65002\n" +
	"\tgroup-updates false\n\tauto-flush false\n}\n"

// parseZeConfig answers ze's parse of a config, or its refusal.
func parseZeConfig(t *testing.T, input string) error {
	t.Helper()
	schema, err := config.YANGSchema()
	if err != nil {
		t.Fatalf("load ze schema: %v", err)
	}
	_, err = config.NewParser(schema).Parse(input)
	return err
}

// TestMigrateAutoFlushAcceptedAndDropped proves an ExaBGP `auto-flush false`
// migrates cleanly and writes no leaf, because ExaBGP has ignored it since
// 4.0.4 (commit d337e1519 removed its only reader), so the migrated peer
// sends routes at once, as ExaBGP does.
//
// VALIDATES: The migration accepts auto-flush, the output names it nowhere,
// and ze parses the output.
// PREVENTS: A migrated `auto-flush false` landing in a leaf nothing reads, or
// holding routes ExaBGP sends.
func TestMigrateAutoFlushAcceptedAndDropped(t *testing.T) {
	output := migrateNeighborOutput(t, autoFlushNeighbor)
	if strings.Contains(output, "auto-flush") {
		t.Fatalf("migrated config still names auto-flush:\n%s", output)
	}
	requireLine(t, output, "group-updates", "false")
	if err := parseZeConfig(t, output); err != nil {
		t.Fatalf("ze refuses the migrated config: %v\n%s", err, output)
	}
}

// TestZeConfigRefusesRemovedAutoFlush proves a ze config that still carries
// the removed `behavior { auto-flush }` leaf is refused, naming the leaf,
// rather than parsing to a peer that silently ignores it.
//
// VALIDATES: The leaf is gone from ze-bgp-conf.yang and the parser says so.
// PREVENTS: A hand-written or previously migrated config keeping a leaf that
// changes nothing.
func TestZeConfigRefusesRemovedAutoFlush(t *testing.T) {
	output := migrateNeighborOutput(t, autoFlushNeighbor)
	withLeaf := strings.Replace(output, "group-updates false", "group-updates false\nauto-flush false", 1)
	if withLeaf == output {
		t.Fatalf("migrated config holds no group-updates line to place auto-flush beside:\n%s", output)
	}
	err := parseZeConfig(t, withLeaf)
	if err == nil {
		t.Fatalf("ze accepts the removed auto-flush leaf:\n%s", withLeaf)
	}
	t.Logf("refusal: %v", err)
	if !strings.Contains(err.Error(), "auto-flush") {
		t.Errorf("refusal %q does not name auto-flush", err)
	}
}
