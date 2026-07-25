// VALIDATES: the traffic, anomaly, and ddos config subsystems are nested one
// level (traffic/{control,usage}, anomaly/{detect,shape},
// ddos/{detect,flowtriq,observe,local,flowspec}) in the composed YANG schema,
// with the augmented sibling subtrees resolving under the parent container owned
// by another module.
// PREVENTS: a regression that flattens these back to top-level containers or
// breaks the cross-module augment (wrong import prefix or augment target path),
// which would silently route config to the wrong plugin section.
package cli

import (
	"testing"

	"github.com/ze-software/ze/internal/component/config"
)

func TestNestedTrafficAnomalySchemaPaths(t *testing.T) {
	schema, err := config.YANGSchema()
	if err != nil {
		t.Fatalf("YANGSchema: %v", err)
	}
	// control is owned by the traffic component; usage is augmented in by the
	// traffic-usage plugin. detect is owned by anomaly-detect; shape is augmented
	// in by anomaly-shape. All four must resolve under their shared parent.
	paths := []string{
		"traffic/control", "traffic/control/backend",
		"traffic/usage", "traffic/usage/enabled",
		"anomaly/detect", "anomaly/detect/enabled",
		"anomaly/shape", "anomaly/shape/mode",
		// ddos: detect owns the parent; flowtriq/observe/local/flowspec augment it.
		"ddos/detect", "ddos/detect/enabled",
		"ddos/flowtriq", "ddos/flowtriq/enabled",
		"ddos/observe", "ddos/observe/incident-ring-size",
		"ddos/local", "ddos/local/response-level",
		"ddos/flowspec", "ddos/flowspec/action",
	}
	for _, p := range paths {
		if _, err := schema.Lookup(p); err != nil {
			t.Errorf("schema.Lookup(%q): %v", p, err)
		}
	}
	// The old flat top-level containers must be gone (hard rename).
	for _, old := range []string{
		"traffic-control", "traffic-usage", "anomaly-detect", "anomaly-shape",
		"ddos-detect", "ddos-flowtriq", "ddos-observe", "ddos-local", "ddos-flowspec",
	} {
		if _, err := schema.Lookup(old); err == nil {
			t.Errorf("schema.Lookup(%q) resolved; expected the flat container to be removed", old)
		}
	}
}
