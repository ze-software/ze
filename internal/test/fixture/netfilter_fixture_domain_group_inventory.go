// Design: docs/architecture/firewall/firewall-domain-group.md -- prove kernel table removal with an answered inventory.
// Related: netfilter_fixture_domain_group_restart.go -- the table removal barrier between daemon starts.
// Related: netfilter_fixture_domain_group.go -- the clear observer's table removal barrier.

package fixture

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// domainGroupTableAbsent asks nftables(8) for the kernel's table inventory in its
// text form. It does not ask for JSON: the QEMU guest's nft binary is built
// --without-json (internal/le/setup/docker/host/static.go), so `nft --json`
// exits 1 there with "JSON support not compiled-in" and no inventory. A failed
// query leaves absence unknown, so callers MUST check the error before using
// the returned absence flag.
func domainGroupTableAbsent(ctx context.Context, table string) (bool, error) {
	output, err := netfilterCommandOutput(ctx, "nft", "list", "tables")
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && len(exitErr.Stderr) > 0 {
			return false, fmt.Errorf("read nftables table inventory: %w: %s", err, strings.TrimSpace(string(exitErr.Stderr)))
		}
		return false, fmt.Errorf("read nftables table inventory: %w", err)
	}
	return domainGroupTableInventoryAbsent(output, table)
}

// domainGroupTableInventoryAbsent reads what `nft list tables` printed after
// it exited 0: one `table <family> <name>` line per table, and nothing when
// the kernel holds none. A line of any other shape, a cut-short one included,
// cannot prove absence. Only the exact inet table counts: another family or a
// name sharing its prefix is another table. The line count is bounded by the
// received output.
func domainGroupTableInventoryAbsent(output, name string) (bool, error) {
	found := false
	for line := range strings.Lines(output) {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if len(fields) != 3 {
			return false, fmt.Errorf("nftables table inventory line %q is not `table <family> <name>`", strings.TrimSpace(line))
		}
		if fields[0] != nftTable {
			return false, fmt.Errorf("nftables table inventory line %q does not name a table", strings.TrimSpace(line))
		}
		if fields[1] != nftFamilyInet {
			continue
		}
		if fields[2] == name {
			found = true
		}
	}
	return !found, nil
}
