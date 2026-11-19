// Design: docs/research/vpp-deployment-reference.md -- registered startup requirements

package vpp

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// A selected contribution reaches the actual startup renderer without adding
// a feature-specific field or branch there. Registration owns its input slices.
func TestStartupFeatureRegistrationOwnsDeclaration(t *testing.T) {
	var registrations startupFeatureRegistry
	flags := []string{"control-punt"}
	roots := []string{"probe"}
	var reads []string
	err := registrations.register("probe", StartupFeature{Plugin: "probe_plugin.so", Section: "probe",
		Flags: flags, ConfigReads: roots, Enabled: func(tree *config.Tree) (bool, error) { return tree.GetContainer("probe") != nil, nil }},
		func(root string) error { reads = append(reads, root); return nil })
	if err != nil {
		t.Fatal(err)
	}
	flags[0] = "changed"
	roots[0] = "changed"
	if !reflect.DeepEqual(reads, []string{"probe"}) {
		t.Fatalf("read roots=%v", reads)
	}
	settings, err := parseStartupSections([]sdk.ConfigSection{
		{Root: "vpp", Data: `{"vpp":{"enabled":"true"}}`},
		{Root: "probe", Data: `{"probe":{}}`},
	}, registrations.snapshot())
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := GenerateStartupConf(&out, settings, testHostInventory()); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"plugin probe_plugin.so {\n    enable\n  }", "probe {\n  control-punt\n}"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("missing %q:\n%s", want, out.String())
		}
	}
	if strings.Contains(out.String(), "changed") {
		t.Fatal("registered declaration retained a mutable caller slice")
	}
}

// An inactive registration does not load a plugin or section accidentally.
func TestStartupFeatureUnselectedDoesNotRender(t *testing.T) {
	var registrations startupFeatureRegistry
	err := registrations.register("probe", StartupFeature{Plugin: "probe_plugin.so", Section: "probe",
		Enabled: func(*config.Tree) (bool, error) { return false, nil }}, func(string) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	settings, err := parseStartupSections([]sdk.ConfigSection{{Root: "vpp", Data: `{"vpp":{"enabled":"true"}}`}}, registrations.snapshot())
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := GenerateStartupConf(&out, settings, testHostInventory()); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "probe") {
		t.Fatal("unselected startup feature was rendered")
	}
}

// A predicate's refusal must survive parsing; no partially selected settings
// are published after a failed prerequisite.
func TestStartupFeaturePredicateFailureRefusesSettings(t *testing.T) {
	want := errors.New("required control plane is disabled")
	feature, err := newStartupFeature("probe", StartupFeature{Plugin: "probe_plugin.so", Section: "probe",
		Enabled: func(*config.Tree) (bool, error) { return false, want }})
	if err != nil {
		t.Fatal(err)
	}
	settings, err := parseStartupSections(nil, []startupFeature{feature})
	if !errors.Is(err, want) {
		t.Fatalf("refusal=%v", err)
	}
	if settings != nil {
		t.Fatal("failed prerequisite published settings")
	}
}

// Raw declarations cannot inject VPP syntax or publish a missing callback.
func TestStartupFeatureRejectsMalformedRegistration(t *testing.T) {
	valid := func() StartupFeature {
		return StartupFeature{Plugin: "probe_plugin.so", Section: "probe", Enabled: func(*config.Tree) (bool, error) { return true, nil }}
	}
	for _, point := range []string{"name", "plugin", "section", "flag", "root", "owned root", "predicate"} {
		t.Run(point, func(t *testing.T) {
			input := valid()
			name := "probe"
			switch point {
			case "name":
				name = "bad name"
			case "plugin":
				input.Plugin = "bad.so { enable }"
			case "section":
				input.Section = "bad section"
			case "flag":
				input.Flags = []string{"flag\nother"}
			case "root":
				input.ConfigReads = []string{"bad root"}
			case "owned root":
				input.ConfigReads = []string{"vpp"}
			case "predicate":
				input.Enabled = nil
			default:
				panic("BUG: unknown registration test case")
			}
			if _, err := newStartupFeature(name, input); err == nil {
				t.Fatal("malformed registration accepted")
			}
		})
	}
}

// One plugin can supply independent registered sections; it is enabled once,
// while a second declaration of a core section refuses the generated config.
func TestStartupFeatureOutputOwnership(t *testing.T) {
	for _, collision := range []bool{false, true} {
		t.Run(fmt.Sprint(collision), func(t *testing.T) {
			section := "second"
			if collision {
				section = "unix"
			}
			features := make([]startupFeature, 0, 2)
			for index, name := range []string{"first", section} {
				feature, err := newStartupFeature(fmt.Sprintf("probe%d", index), StartupFeature{
					Plugin: "probe_plugin.so", Section: name, Enabled: func(*config.Tree) (bool, error) { return true, nil },
				})
				if err != nil {
					t.Fatal(err)
				}
				features = append(features, feature)
			}
			settings, err := parseStartupSections([]sdk.ConfigSection{{Root: "vpp", Data: `{"vpp":{"enabled":"true"}}`}}, features)
			if err != nil {
				t.Fatal(err)
			}
			var out bytes.Buffer
			err = GenerateStartupConf(&out, settings, testHostInventory())
			if collision {
				if err == nil {
					t.Fatal("duplicate core section accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if count := strings.Count(out.String(), "plugin probe_plugin.so"); count != 1 {
				t.Fatalf("plugin blocks=%d", count)
			}
			for _, want := range []string{"first {", "second {"} {
				if !strings.Contains(out.String(), want) {
					t.Fatalf("missing %q", want)
				}
			}
		})
	}
}

// A failed delivery registration is not visible, and a successful name cannot
// be overwritten by a second declaration.
func TestStartupFeatureRegistrationFailureDoesNotPublish(t *testing.T) {
	var registrations startupFeatureRegistry
	input := StartupFeature{Plugin: "probe_plugin.so", Section: "probe", ConfigReads: []string{"probe"}, Enabled: func(*config.Tree) (bool, error) { return true, nil }}
	want := errors.New("reader was sealed")
	if err := registrations.register("probe", input, func(string) error { return want }); !errors.Is(err, want) {
		t.Fatalf("registration=%v", err)
	}
	if len(registrations.snapshot()) != 0 {
		t.Fatal("failed read-root registration published the feature")
	}
	if err := registrations.register("probe", input, func(string) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if err := registrations.register("probe", input, func(string) error { t.Fatal("duplicate reached read registration"); return nil }); err == nil {
		t.Fatal("duplicate registration accepted")
	}
}

// Two delivered sections cannot silently replace one another's root.
func TestStartupFeatureDuplicateConfigRootRefused(t *testing.T) {
	settings, err := parseStartupSections([]sdk.ConfigSection{
		{Root: "vpp", Data: `{"vpp":{"enabled":"true"}}`},
		{Root: "vpp", Data: `{"vpp":{"enabled":"false"}}`},
	}, nil)
	if err == nil {
		t.Fatal("duplicate root accepted")
	}
	if settings != nil {
		t.Fatal("duplicate root published settings")
	}
}
