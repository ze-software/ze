// Design: docs/architecture/container/runtime.md -- the container config and its one rule set

package container

import (
	"errors"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// containerSections is one delivery that carries the container root with
// data as its body, as the engine delivers it: wrapped in the root's name.
func containerSections(data string) []sdk.ConfigSection {
	return []sdk.ConfigSection{{Root: configRoot, Data: data}}
}

const (
	sectionNoContainer  = `{"container":{}}`
	sectionRemovedRoot  = `{}`
	sectionDisabledWeb  = `{"container":{"instance":{"web":{"enabled":"false"}}}}`
	sectionEnabledWeb   = `{"container":{"instance":{"web":{"enabled":"true"}}}}`
	sectionDefaultWeb   = `{"container":{"instance":{"web":{}}}}`
	sectionMalformed    = `{`
	sectionBadName      = `{"container":{"instance":{"Bad_Name":{"enabled":"false"}}}}`
	sectionEnabledMaybe = `{"container":{"instance":{"web":{"enabled":"maybe"}}}}`
)

// errorText answers the text of err, and the empty string for nil, so two
// verdicts compare as one value.
func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// TestOneRuleSetServesValidateVerifyAndConfigure proves that `ze config
// validate`, a reload and a boot cannot disagree about one config.
//
// Method: run the registry's static verifier, the config transaction's
// reload verify and its boot configure over the same deliveries. All three
// accept, or all three refuse with the same text.
func TestOneRuleSetServesValidateVerifyAndConfigure(t *testing.T) {
	reg := registry.Lookup(pluginName)
	if reg == nil {
		t.Fatal("the container plugin is not registered")
	}
	if reg.InProcessConfigVerifier == nil {
		t.Fatal("the container plugin registers no static config verifier")
	}
	cases := []struct {
		name    string
		data    string
		refused bool
	}{
		{"no container", sectionNoContainer, false},
		{"removed root", sectionRemovedRoot, false},
		{"disabled container", sectionDisabledWeb, false},
		{"malformed delivery", sectionMalformed, true},
		{"name outside the rule", sectionBadName, true},
		{"enabled neither true nor false", sectionEnabledMaybe, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sections := containerSections(tc.data)
			newTx := func() *sdk.ConfigTx[containerConfig] {
				sup := newSupervisor(slogutil.DiscardLogger(), nil, nil, nil)
				return sdk.NewConfigTx(containerConfig{}, parseContainerSections, sup.install)
			}

			static := reg.InProcessConfigVerifier(sections)
			reload := newTx().Verify(sections)
			boot := newTx().Configure(sections)

			if (static != nil) != tc.refused {
				t.Fatalf("the static verifier answered %v, want refused=%v", static, tc.refused)
			}
			if errorText(reload) != errorText(static) {
				t.Errorf("the reload verify answered %q, the static verifier %q", errorText(reload), errorText(static))
			}
			if errorText(boot) != errorText(static) {
				t.Errorf("the boot configure answered %q, the static verifier %q", errorText(boot), errorText(static))
			}
		})
	}
}

// TestRemovedRootStopsEveryContainer proves a removed container root is read
// as "no container", never as "no change": the delivery is present, it holds
// no instance, and applying it empties the supervisor's desired set.
//
// MUTATION: make parseContainerSections answer present false for an empty
// body. The reload then installs nothing and the container stays configured.
func TestRemovedRootStopsEveryContainer(t *testing.T) {
	sup := newSupervisor(slogutil.DiscardLogger(), nil, nil, nil)
	tx := sdk.NewConfigTx(containerConfig{}, parseContainerSections, sup.install)
	if err := tx.Configure(containerSections(sectionDisabledWeb)); err != nil {
		t.Fatalf("the boot configure refused a disabled container: %v", err)
	}
	if _, configured := sup.instance("web"); !configured {
		t.Fatal("the supervisor does not hold the container the boot configured")
	}

	cfg, present, err := parseContainerSections(containerSections(sectionRemovedRoot))
	if err != nil {
		t.Fatalf("a removed root was refused: %v", err)
	}
	if !present {
		t.Fatal("a removed root parsed as an absent section, which a reload reads as no change")
	}
	if len(cfg.instances) != 0 {
		t.Fatalf("a removed root parsed to %d containers, want 0", len(cfg.instances))
	}

	if err := tx.Verify(containerSections(sectionRemovedRoot)); err != nil {
		t.Fatalf("the reload verify refused a removed root: %v", err)
	}
	if err := tx.Apply(); err != nil {
		t.Fatalf("the reload apply of a removed root failed: %v", err)
	}
	if _, configured := sup.instance("web"); configured {
		t.Fatal("the container is still configured after its root was removed")
	}
}

// TestDeliveryWithoutTheRootChangesNothing proves the other reading stays
// available: a delivery that carries no container section at all is absent,
// which a reload means as "unchanged".
func TestDeliveryWithoutTheRootChangesNothing(t *testing.T) {
	_, present, err := parseContainerSections([]sdk.ConfigSection{{Root: "interface", Data: `{}`}})
	if err != nil {
		t.Fatalf("a delivery with no container section was refused: %v", err)
	}
	if present {
		t.Fatal("a delivery with no container section parsed as present")
	}
}

// TestDisabledContainerTakesTheDisabledState proves the one state the config
// alone decides: a container whose enabled leaf is false is held in the
// disabled state.
func TestDisabledContainerTakesTheDisabledState(t *testing.T) {
	cfg, _, err := parseContainerSections(containerSections(sectionDisabledWeb))
	if err != nil {
		t.Fatalf("a disabled container was refused: %v", err)
	}
	sup := newSupervisor(slogutil.DiscardLogger(), nil, nil, nil)
	if err := sup.install(cfg); err != nil {
		t.Fatalf("install failed: %v", err)
	}

	state, known := sup.state("web")

	if !known {
		t.Fatal("a disabled container holds no state")
	}
	if state != StateDisabled {
		t.Fatalf("a disabled container is in state %s, want %s", state, StateDisabled)
	}
}

// TestRuntimeNotBuiltRefusesAnEnabledContainer pins the refusal of the wiring
// stage: until a container can start, a config that enables one is refused
// with the statement that disables it, whether enabled is written or left to
// its default of true. This test goes with refuseUnbuiltRuntime, when the
// start sequence is built.
func TestRuntimeNotBuiltRefusesAnEnabledContainer(t *testing.T) {
	for name, data := range map[string]string{
		"enabled true":     sectionEnabledWeb,
		"enabled defaults": sectionDefaultWeb,
	} {
		t.Run(name, func(t *testing.T) {
			_, _, err := parseContainerSections(containerSections(data))

			var refusal *opererr.Error
			if !errors.As(err, &refusal) {
				t.Fatalf("an enabled container answered %v, want an operator refusal", err)
			}
			if !strings.Contains(refusal.Problem(), "container web cannot run") {
				t.Errorf("the refusal's problem is %q, want it to name the container", refusal.Problem())
			}
			if refusal.Fix() != "set container instance web enabled false" {
				t.Errorf("the refusal's fix is %q, want the statement that disables the container", refusal.Fix())
			}
		})
	}
}

// TestInstanceNameRule covers both sides of each bound of the name rule: the
// length, the first and last character, and the character set.
func TestInstanceNameRule(t *testing.T) {
	longest := strings.Repeat("a", instanceNameLengthMax)
	cases := []struct {
		name  string
		valid bool
	}{
		{"a", true},
		{"web-1", true},
		{"0", true},
		{longest, true},
		{"", false},
		{longest + "a", false},
		{"-web", false},
		{"web-", false},
		{"Web", false},
		{"web_1", false},
		{"web.1", false},
		{"web/1", false},
	}
	for _, tc := range cases {
		err := checkInstanceName(tc.name)
		if (err == nil) != tc.valid {
			t.Errorf("checkInstanceName(%q) answered %v, want valid=%v", tc.name, err, tc.valid)
		}
	}
}
