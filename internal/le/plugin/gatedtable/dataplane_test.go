// Design: docs/architecture/plugin/feature-gates.md -- generated absent-writer identity.

package gatedtable

import (
	"encoding/json"
	"strings"
	"testing"

	pluginimports "github.com/ze-software/ze/internal/le/plugin/imports"
	repofeaturetags "github.com/ze-software/ze/internal/le/repo/featuretags"
	repoinventory "github.com/ze-software/ze/internal/le/repo/inventory"
)

// TestRenderedGatesRetainDataPlaneWriter proves a backend is mapped to its
// declared plugin name, without assuming that names share a prefix.
func TestRenderedGatesRetainDataPlaneWriter(t *testing.T) {
	var rows []Row
	if err := json.Unmarshal([]byte(`[{"name":"forwarder","tags":["ze_fabric"],"data-plane":"fabric"}]`), &rows); err != nil {
		t.Fatal(err)
	}
	got, err := render(rows)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), `"fabric": "forwarder"`) {
		t.Fatalf("generated gates lost the declared writer: %s", got)
	}
}

// TestGateRowsRetainDeclaredDataPlane exercises the inventory-to-gate seam,
// including the manifest requirements for a writer with an unrelated name.
func TestGateRowsRetainDeclaredDataPlane(t *testing.T) {
	var plugins []repoinventory.Plugin
	if err := json.Unmarshal([]byte(`[{"name":"forwarder","source-dir":"internal/plugins/fabric","data-plane":"fabric"}]`), &plugins); err != nil {
		t.Fatal(err)
	}
	gates := pluginimports.GateIndexFrom([]repofeaturetags.Gate{{Tag: "ze_fabric", Package: "internal/plugins/fabric"}})
	got, err := render(rowsOf(gates, plugins))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), `"fabric": "forwarder"`) {
		t.Fatalf("derived gate rows lost the writer identity: %s", got)
	}
}

// TestGatedDataPlaneRefusesAmbiguousWriters proves generation cannot silently
// choose a build tag when two declarations claim the same forwarding backend.
func TestGatedDataPlaneRefusesAmbiguousWriters(t *testing.T) {
	var rows []Row
	if err := json.Unmarshal([]byte(`[{"name":"first","tags":["ze_first"],"data-plane":"fabric"},{"name":"second","tags":["ze_second"],"data-plane":"fabric"}]`), &rows); err != nil {
		t.Fatal(err)
	}
	if _, err := render(rows); err == nil {
		t.Fatal("ambiguous data-plane writer rendered as a valid gate table")
	}
}
