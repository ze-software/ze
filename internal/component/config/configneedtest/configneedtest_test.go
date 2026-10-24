package configneedtest

import (
	"errors"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// TestPathFaultRefusesEveryPathANeedCannotHoldOn checks the walk each owner's
// TestConfigNeedPathsExistInSchema relies on. Method: one synthetic schema
// with a container, a leaf under it and a list beside it; a path to the leaf,
// and a path through the list followed by registry.AnyEntry, pass; each other
// path fails naming the element that broke it, with the two list mistakes
// wrapping their own sentinel.
//
// VALIDATES: PathFault accepts only a leaf reached through containers, and
// through lists each followed by the entry marker.
// PREVENTS: a need path that names a list as a container, or a drifted path,
// passing the per-owner schema test.
func TestPathFaultRefusesEveryPathANeedCannotHoldOn(t *testing.T) {
	schema := config.NewSchema()
	schema.Define("traffic", config.Container(
		config.Field("control", config.Container(
			config.Field("backend", config.Leaf(config.TypeString)),
			config.Field("interface", config.List(config.TypeString,
				config.Field("rate", config.Leaf(config.TypeString)),
				config.Field("queue", config.Container(
					config.Field("depth", config.Leaf(config.TypeString)),
				)),
			)),
		)),
	))

	accepted := [][]string{
		{"traffic", "control", "backend"},
		{"traffic", "control", "interface", registry.AnyEntry, "rate"},
		{"traffic", "control", "interface", registry.AnyEntry, "queue", "depth"},
	}
	for _, path := range accepted {
		if fault := PathFault(schema, path); fault != nil {
			t.Errorf("path %q was refused: %v", path, fault)
		}
	}

	cases := []struct {
		name     string
		path     []string
		names    string
		sentinel error
	}{
		{name: "absent root", path: []string{"qos", "control", "backend"}, names: `"qos"`},
		{name: "absent leaf", path: []string{"traffic", "control", "backend-name"}, names: `"backend-name"`},
		{name: "list read as a container", path: []string{"traffic", "control", "interface", "rate"}, names: `"interface"`, sentinel: errListWithoutEntry},
		{name: "entry marker after a container", path: []string{"traffic", registry.AnyEntry, "control", "backend"}, names: `"traffic"`, sentinel: errEntryWithoutList},
		{name: "absent leaf inside a list entry", path: []string{"traffic", "control", "interface", registry.AnyEntry, "burst"}, names: `"burst"`},
		{name: "ends at a list entry", path: []string{"traffic", "control", "interface", registry.AnyEntry}, names: "list entry"},
		{name: "ends at a container inside a list entry", path: []string{"traffic", "control", "interface", registry.AnyEntry, "queue"}, names: `"queue"`},
		{name: "ends at a container", path: []string{"traffic", "control"}, names: `"control"`},
		{name: "continues past a leaf", path: []string{"traffic", "control", "backend", "name"}, names: `"backend"`},
		{name: "empty", path: nil, names: "empty"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fault := PathFault(schema, tc.path)
			if fault == nil {
				t.Fatalf("path %q was accepted", tc.path)
			}
			if !strings.Contains(fault.Error(), tc.names) {
				t.Errorf("fault %q does not name %s", fault, tc.names)
			}
			if tc.sentinel != nil && !errors.Is(fault, tc.sentinel) {
				t.Errorf("fault %q does not wrap %v", fault, tc.sentinel)
			}
		})
	}
}
