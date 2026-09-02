package config

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config/yang"
)

// TestLoadConfigRefusesAnInvalidCustomValidatorValue is AC-1: LoadConfig applies
// the registered ze:validate custom validators and refuses a value one rejects.
//
// The input passes the YANG schema on its own. `plugin/internal/use` is
// `type string { length "1..64" }` in
// internal/component/plugin/yang/ze-plugin-conf.yang, so this name is well typed
// and only InternalPluginNameValidator (internal/component/config/validators.go)
// can refuse it. Revert LoadConfig's refuseInvalidCustomSections call and this
// test goes red, which is what makes it evidence rather than decoration.
func TestLoadConfigRefusesAnInvalidCustomValidatorValue(t *testing.T) {
	const src = `plugin {
	internal p1 {
		use no-such-plugin-here
	}
}
`
	_, err := LoadConfig(src, "test.conf", nil)
	if err == nil {
		t.Fatal("LoadConfig accepted a config the internal-plugin-name validator refuses")
	}
	msg := err.Error()
	for _, want := range []string{
		"config validation failed",
		"plugin",
		"no-such-plugin-here",
		"is not a registered internal plugin",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("error does not name %q: %s", want, msg)
		}
	}
}

// TestLoadConfigAcceptsAValidConfigUnchanged is AC-4: a config whose values every
// registered validator accepts still loads, and loads to the tree it always did.
// The interface section is in validatedSections and carries a mac-address
// ze:validate binding, so this input goes through the new walk, not around it.
func TestLoadConfigAcceptsAValidConfigUnchanged(t *testing.T) {
	const src = `interface {
	backend netlink;
	ethernet eth0 {
		mac {
			address 02:00:00:00:00:02;
		}
	}
}
`
	result, err := LoadConfig(src, "test.conf", nil)
	if err != nil {
		t.Fatalf("LoadConfig refused a valid config: %v", err)
	}
	if result == nil || result.Tree == nil {
		t.Fatal("LoadConfig returned no tree")
	}
	if c := result.Tree.GetContainer("interface"); c == nil {
		t.Error("the interface section did not survive the load")
	}
}

// TestValidateCustomSectionsGradesMissingAsNonBlocking pins the severity rule
// both callers share. A missing mandatory field is reported and does not refuse:
// `ze config validate` has always graded it a warning, and a LoadConfig that
// graded it an error would refuse configs the operator was told were fine.
func TestValidateCustomSectionsGradesMissingAsNonBlocking(t *testing.T) {
	missing := SectionValidationError{
		Section: "isis",
		Err: yang.ValidationError{
			Path:    "instance.core",
			Type:    yang.ErrTypeMissing,
			Message: `mandatory field "net" is missing`,
		},
	}
	if missing.Blocking() {
		t.Error("a missing mandatory field must not refuse the config")
	}
	refused := SectionValidationError{
		Section: "plugin",
		Err: yang.ValidationError{
			Path:    "internal.p1.use",
			Type:    yang.ErrTypeType,
			Message: "not a registered internal plugin",
		},
	}
	if !refused.Blocking() {
		t.Error("a custom validator refusal must refuse the config")
	}
}

// TestSectionValidationErrorRedactsASensitiveLeaf proves a refusal never prints
// the value of a leaf the schema marks sensitive. The message reaches stderr and
// the startup log, so a validator bound to a secret must not leak it there.
func TestSectionValidationErrorRedactsASensitiveLeaf(t *testing.T) {
	e := SectionValidationError{
		Section:   "isis",
		Sensitive: true,
		Err: yang.ValidationError{
			Path:    "isis/authentication/key-chain[kc1]/key[1]/secret",
			Type:    yang.ErrTypeType,
			Message: `"hunter2" is not acceptable`,
		},
	}
	msg := e.Message()
	if strings.Contains(msg, "hunter2") {
		t.Errorf("a sensitive value reached the message: %s", msg)
	}
	if !strings.Contains(msg, "sensitive value redacted") {
		t.Errorf("message does not say the value was redacted: %s", msg)
	}
	if !strings.Contains(msg, "isis") || !strings.Contains(msg, "key[1]/secret") {
		t.Errorf("message does not name the section and the leaf: %s", msg)
	}
}

// TestIsSensitiveLeafReadsTheProducersPathSeparator pins the redaction lookup to
// the shape the walk really emits. walkTree
// (internal/component/config/yang/validator.go) joins each child onto its parent
// with Byte('/'), and SensitiveKeys (schema.go) keys the map on the bare leaf
// name. The copy this function replaced split the path on ".", so it matched
// nothing the walk produces: every sensitive leaf that failed printed its value
// into stderr and the startup log while Message() promised it would not.
func TestIsSensitiveLeafReadsTheProducersPathSeparator(t *testing.T) {
	keys := map[string]bool{"secret": true}
	cases := []struct {
		path string
		want bool
	}{
		{"isis/authentication/key-chain[kc1]/key[1]/secret", true},
		{"l2tp/tunnel[t1]/secret", true},
		{"secret", true},
		{"isis/hostname", false},
		{"isis/authentication/key-chain[kc1]/key[1]/algorithm", false},
	}
	for _, c := range cases {
		if got := isSensitiveLeaf(c.path, keys); got != c.want {
			t.Errorf("isSensitiveLeaf(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}

// TestValidatedSectionsExcludesBGP records why the list stops where it does. BGP
// is re-checked at startup by PeersFromConfigTree
// (internal/component/bgp/config/peers.go), which startup reaches directly from
// CreateReactorFromTree (internal/component/bgp/config/loader_create.go).
// infra.ValidateBGPPeers is the OFFLINE door onto the same walk, and neither of
// its two non-test callers is the startup path. Adding bgp here would arm
// AddressFamilyValidator over registry.FamilyMap(), which omits the builtin
// families (plan/journal/gate-excludes-part-of-its-population.md).
func TestValidatedSectionsExcludesBGP(t *testing.T) {
	for _, s := range validatedSections {
		if s == "bgp" || s == "redistribute" {
			t.Errorf("%q is in validatedSections; widening to it is gated on the "+
				"AddressFamilyValidator defect, not on this spec", s)
		}
	}
}

// TestValidatedSectionsIncludesService pins the 2026-09-02 widening. The goal
// is that a ze:validate annotation under `service` actually runs; the method is
// to read the list the walk iterates, because that membership is the whole
// mechanism. Before the widening, `ze config validate` accepted a DHCP
// `default-router` of 2001:db8::1 against `ze:validate "ipv4-address"`, and the
// annotation still passed CheckAllValidatorsRegistered, so nothing anywhere
// reported the gap. Drop "service" from validatedSections and this goes red.
//
// `service` is safe to walk where `bgp` and `redistribute` are not: the only
// validator names under it are ipv4-address and ipv4-prefix, both pure form
// checks that read no registry and depend on no startup order.
func TestValidatedSectionsIncludesService(t *testing.T) {
	for _, s := range validatedSections {
		if s == "service" {
			return
		}
	}
	t.Error("\"service\" is absent from validatedSections, so every ze:validate under it is inert")
}
