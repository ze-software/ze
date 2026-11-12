package yang

import (
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
)

// constraintProbeModule declares one restriction of each kind RFC 7950 gives
// an error-message: a range reached through a typedef, a pattern on the leaf
// itself, a range on a union member, and a range with no message.
const constraintProbeModule = `module ze-test-constraint-conf {
    namespace "urn:ze:test-constraint:conf";
    prefix tcm;

    revision 2026-01-01 { description "Probe module for constraint error-message strings."; }

    typedef probe-vlan {
        type uint16 {
            range "1..4094" {
                error-message "probe range message.";
            }
        }
    }

    container test-constraint-probe {
        description "Leaves whose restrictions declare an error-message.";
        leaf vlan {
            type probe-vlan;
            description "A range through a typedef.";
        }
        leaf code {
            type string {
                pattern "[a-z]{3}" {
                    error-message "probe pattern message.";
                }
            }
            description "A pattern on the leaf.";
        }
        leaf either {
            type union {
                type uint8 {
                    range "1..5" {
                        error-message "probe member message.";
                    }
                }
                type boolean;
            }
            description "A range on a union member.";
        }
        leaf plain {
            type uint16 { range "1..10"; }
            description "A range with no message.";
        }
    }
}`

// constraintProbe resolves the registered model plus the probe module.
func constraintProbe(t *testing.T) *Resolved {
	t.Helper()

	loader := NewLoader()
	if err := loader.LoadEmbedded(); err != nil {
		t.Fatalf("load embedded modules: %v", err)
	}
	if err := loader.LoadRegistered(); err != nil {
		t.Fatalf("load registered modules: %v", err)
	}
	if err := loader.AddModuleFromText("ze-test-constraint-conf", constraintProbeModule); err != nil {
		t.Fatalf("add the probe module: %v", err)
	}
	schema, err := loader.Resolve()
	if err != nil {
		t.Fatalf("resolve the model: %v", err)
	}
	return schema
}

// TestYANGRangeErrorMessageReachesRefusal proves that the error-message of a
// range or a pattern reaches the refusal an operator reads, on the tree
// validator and on the command argument definition.
//
// VALIDATES: RFC 7950 Section 7.5.4.1, the error-message string travels with
// the refusal of its constraint: through a typedef chain, on the leaf's own
// type, and on a union member. A range with no message keeps its generic text.
// PREVENTS: goyang's resolved YangType dropping the statement, so a module
// author's message is parsed and never spoken (A-1 of
// spec-operator-errors-name-the-fix). Removing speakConstraintMessage, the
// RangeMessage carry in yangTypeToArgDef, or typeConstraintMessages makes this
// red.
func TestYANGRangeErrorMessageReachesRefusal(t *testing.T) {
	schema := constraintProbe(t)
	validator := NewValidator(schema)

	t.Run("tree validator range through a typedef", func(t *testing.T) {
		for _, accepted := range []int{1, 4094} {
			if err := validator.Validate("test-constraint-probe/vlan", accepted); err != nil {
				t.Errorf("vlan %d refused: %v", accepted, err)
			}
		}
		for _, refused := range []int{0, 4095} {
			err := validator.Validate("test-constraint-probe/vlan", refused)
			var refusal *ValidationError
			if !errors.As(err, &refusal) {
				t.Fatalf("vlan %d: want a ValidationError, got %T: %v", refused, err, err)
			}
			if !strings.HasSuffix(refusal.Message, " is outside range: probe range message.") {
				t.Errorf("vlan %d: the refusal does not speak the range's error-message: %q", refused, refusal.Message)
			}
		}
	})

	t.Run("tree validator pattern", func(t *testing.T) {
		err := validator.Validate("test-constraint-probe/code", "ABC")
		var refusal *ValidationError
		if !errors.As(err, &refusal) {
			t.Fatalf("want a ValidationError, got %T: %v", err, err)
		}
		if refusal.Message != `invalid value "ABC": probe pattern message.` {
			t.Errorf("the pattern refusal does not speak its error-message: %q", refusal.Message)
		}
	})

	t.Run("tree validator range with no message", func(t *testing.T) {
		err := validator.Validate("test-constraint-probe/plain", 11)
		var refusal *ValidationError
		if !errors.As(err, &refusal) {
			t.Fatalf("want a ValidationError, got %T: %v", err, err)
		}
		if refusal.Message != "value 11 is outside range" {
			t.Errorf("a range with no error-message changed its refusal: %q", refusal.Message)
		}
	})

	probe := schema.GetEntry("ze-test-constraint-conf").Dir["test-constraint-probe"]
	if probe == nil {
		t.Fatal("the probe container did not resolve")
	}

	t.Run("union member", func(t *testing.T) {
		either := probe.Dir["either"]
		if got := LeafConstraintMessages(either, either.Type.Type[0]).Range(); got != "probe member message." {
			t.Errorf("the union member's range message is %q", got)
		}
		if got := LeafConstraintMessages(either, either.Type).Range(); got != "" {
			t.Errorf("the union itself declares no range, got message %q", got)
		}
	})

	t.Run("command argument range", func(t *testing.T) {
		def, ok := compiledPatterns{}.argDefFor(probe.Dir["vlan"], "vlan")
		if !ok {
			t.Fatal("vlan has no argument definition")
		}
		for _, accepted := range []string{"1", "4094"} {
			if err := command.ValidateArgString(accepted, &def); err != nil {
				t.Errorf("vlan %s refused: %v", accepted, err)
			}
		}
		for _, refused := range []string{"0", "4095"} {
			err := command.ValidateArgString(refused, &def)
			// The declared error-message is the why line (RFC 7950 Section 7.5.4.1).
			want := `vlan: "` + refused + `" is outside the accepted range` + "\nprobe range message."
			if err == nil || err.Error() != want {
				t.Errorf("vlan %s: got %v, want %q", refused, err, want)
			}
		}
	})

	t.Run("command argument pattern", func(t *testing.T) {
		patterns := compiledPatterns{"[a-z]{3}": regexp.MustCompile(`^(?:[a-z]{3})$`)}
		def, ok := patterns.argDefFor(probe.Dir["code"], "code")
		if !ok {
			t.Fatal("code has no argument definition")
		}
		err := command.ValidateArgString("ABC", &def)
		want := `code: "ABC" is not in the accepted format` + "\nprobe pattern message."
		if err == nil || err.Error() != want {
			t.Errorf("got %v, want %q", err, want)
		}
	})
}
