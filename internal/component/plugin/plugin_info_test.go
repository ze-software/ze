// Design: docs/guide/command-reference.md — show plugin list
// Related: resolve.go — pluginInfo, the row each registration becomes

package plugin

import (
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// TestPluginInfoCopiesEveryDescriptiveField proves a `show plugin list` row
// carries each registration field it names, so the runtime answer that
// `le`'s registered check compares with the declared source is complete.
//
// VALIDATES: pluginInfo copies the name, description, RFCs, families,
// capability codes, config roots, dependencies, optional dependencies,
// start-after names and features, and answers yang, nlri-decoder and
// nlri-encoder from whether the registration carries a schema, a decoder and
// an encoder; an empty registration answers none of them.
// PREVENTS: a field the row drops reading as "the plugin declares none".
func TestPluginInfoCopiesEveryDescriptiveField(t *testing.T) {
	reg := &registry.Registration{
		Name:                 "probe",
		Description:          "Probe",
		RFCs:                 []string{"4271"},
		Families:             []string{"ipv4/unicast"},
		CapabilityCodes:      []uint8{1, 65},
		ConfigRoots:          []string{"bgp"},
		Dependencies:         []string{"bgp-rib"},
		OptionalDependencies: []string{"bgp-adj-rib-in"},
		StartAfter:           []string{"bgp-gr"},
		Features:             "nlri yang",
		YANG:                 "module probe {}",
		InProcessNLRIDecoder: func(string, string, bool, bool) (any, error) {
			return map[string]any{}, nil
		},
	}
	info := pluginInfo(reg)
	if info.Name != "probe" {
		t.Errorf("name = %q, want probe", info.Name)
	}
	if info.Description != "Probe" {
		t.Errorf("description = %q, want Probe", info.Description)
	}
	for _, field := range []struct {
		label     string
		got, want []string
	}{
		{"rfcs", info.RFCs, reg.RFCs},
		{"families", info.Families, reg.Families},
		{"config-roots", info.ConfigRoots, reg.ConfigRoots},
		{"dependencies", info.Dependencies, reg.Dependencies},
		{"optional-dependencies", info.OptionalDependencies, reg.OptionalDependencies},
		{"start-after", info.StartAfter, reg.StartAfter},
	} {
		if !slices.Equal(field.got, field.want) {
			t.Errorf("%s = %v, want %v", field.label, field.got, field.want)
		}
	}
	if !slices.Equal(info.Capabilities, []int{1, 65}) {
		t.Errorf("capabilities = %v, want [1 65]", info.Capabilities)
	}
	if info.Features != "nlri yang" {
		t.Errorf("features = %q, want %q", info.Features, "nlri yang")
	}
	if !info.YANG {
		t.Error("yang = false for a registration that carries a schema")
	}
	if !info.NLRIDecoder {
		t.Error("nlri-decoder = false for a registration that carries a decoder")
	}
	if info.NLRIEncoder {
		t.Error("nlri-encoder = true for a registration that carries no encoder")
	}

	empty := pluginInfo(&registry.Registration{Name: "bare"})
	if empty.YANG {
		t.Error("yang = true for a registration with no schema")
	}
	if empty.NLRIDecoder {
		t.Error("nlri-decoder = true for a registration with no decoder")
	}
	if len(empty.Dependencies)+len(empty.ConfigRoots)+len(empty.StartAfter) != 0 {
		t.Errorf("bare registration row = %+v, want no lists", empty)
	}
}
