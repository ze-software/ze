package flowexport

import (
	"testing"

	"github.com/ze-software/ze/internal/component/config/configneedtest"
)

// TestConfigNeedPathsExistInSchema checks that every config need this plugin
// declares names a leaf its own YANG holds, reached through containers only
// (`flow-export enrichment bgp`). Method: walk each need path against the loaded schema. A
// renamed leaf or a path through a list would make the need hold for no
// config, so the provider would never be pulled and its absence never refused.
func TestConfigNeedPathsExistInSchema(t *testing.T) {
	configneedtest.RequirePathsInSchemaForTest(t, "flow-export")
}
