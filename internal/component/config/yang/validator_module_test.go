package yang

import (
	"errors"
	"testing"
)

// sectionProbeModule declares a top-level section whose name appears nowhere in
// validator.go. A hand-written prefix-to-module switch cannot resolve it; the
// model can, because the module's own entry tree lists the section it declares.
const sectionProbeModule = `module ze-test-section-conf {
    namespace "urn:ze:test-section:conf";
    prefix tse;

    revision 2026-01-01 { description "Probe module for the section-to-module derivation."; }

    container test-probe-section {
        description "A section no prefix switch ever named.";
        leaf limit {
            type uint16 { range "1..100"; }
            description "A bounded leaf, so a refusal proves the section resolved.";
        }
    }
}`

// probeValidator builds a validator over the registered model plus the probe
// module above.
func probeValidator(t *testing.T) *Validator {
	t.Helper()

	loader := NewLoader()
	if err := loader.LoadEmbedded(); err != nil {
		t.Fatalf("load embedded modules: %v", err)
	}
	if err := loader.LoadRegistered(); err != nil {
		t.Fatalf("load registered modules: %v", err)
	}
	if err := loader.AddModuleFromText("ze-test-section-conf", sectionProbeModule); err != nil {
		t.Fatalf("add the probe module: %v", err)
	}
	if err := loader.Resolve(); err != nil {
		t.Fatalf("resolve the model: %v", err)
	}
	return NewValidator(loader)
}

// TestValidateResolvesASectionValidatorGoDoesNotName is the discrimination test
// for the section-to-module derivation. The probe module declares a section the
// validator names nowhere, and Validate must still resolve its leaf and refuse
// a value outside the declared range.
//
// A prefix switch restored in validator.go makes this RED: the probe section is
// in no case arm, so findSchemaNode resolves no module and Validate returns
// "module not found" instead of the range refusal.
//
// VALIDATES: findSchemaNode asks the compiled model which module declares a
// section, so a module that registers a new section validates with no edit to
// internal/component/config/yang.
// PREVENTS: a section whose leaves silently validate nothing because nobody
// added it to a central switch -- the defect "pppoe" carried until this
// derivation landed (ai/rules/principles.md, a central enumeration).
func TestValidateResolvesASectionValidatorGoDoesNotName(t *testing.T) {
	validator := probeValidator(t)

	if err := validator.Validate("test-probe-section/limit", 50); err != nil {
		t.Errorf("a value inside the declared range was refused: %v", err)
	}

	err := validator.Validate("test-probe-section/limit", 500)
	if err == nil {
		t.Fatal("a value above the declared range was accepted: the section resolved to no schema")
	}
	var validationErr *ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("want a ValidationError naming the range, got %T: %v", err, err)
	}
	if validationErr.Type != ErrTypeRange {
		t.Errorf("want a range refusal, got %v: %v", validationErr.Type, validationErr)
	}
}

// TestModuleDeclaringAnswersOnlyADeclaringModule proves the derivation reads
// the model rather than guessing, in both directions.
//
// VALIDATES: moduleDeclaring answers the module that lists the section, and
// answers nil for a section no module declares.
// PREVENTS: a lookup that returns a plausible module for an unknown section, so
// a typo in a config path validates against the wrong schema instead of being
// refused (ai/rules/principles.md, "a zero value is never an answer").
func TestModuleDeclaringAnswersOnlyADeclaringModule(t *testing.T) {
	validator := probeValidator(t)

	entry := validator.moduleDeclaring("test-probe-section")
	if entry == nil {
		t.Fatal("the probe module declares test-probe-section and moduleDeclaring answered nil")
	}
	if entry.Name != "ze-test-section-conf" {
		t.Errorf("want the declaring module ze-test-section-conf, got %q", entry.Name)
	}

	if unknown := validator.moduleDeclaring("no-module-declares-this"); unknown != nil {
		t.Errorf("want nil for a section no module declares, got module %q", unknown.Name)
	}
}

// TestModuleDeclaringCoversEveryLoadedSection proves the derivation reaches the
// whole model: every top-level section any loaded config module declares
// resolves to a module, so no section is left unvalidatable.
//
// The assertion runs over the loaded modules, not over a fixture list, so a
// module the tree gains widens this test on its own.
//
// VALIDATES: every section the model declares is resolvable by findSchemaNode.
// PREVENTS: the partial coverage a hand-written switch gives, where twelve
// sections resolve and every later one does not.
func TestModuleDeclaringCoversEveryLoadedSection(t *testing.T) {
	validator := probeValidator(t)

	sections := 0
	for _, module := range validator.loader.ConfModuleNames() {
		entry := validator.loader.GetEntry(module)
		if entry == nil || entry.Dir == nil {
			continue
		}
		for section := range entry.Dir {
			sections++
			if validator.moduleDeclaring(section) == nil {
				t.Errorf("module %q declares section %q and moduleDeclaring answered nil", module, section)
			}
		}
	}
	if sections == 0 {
		t.Fatal("no config module declared a section: the model is not loaded")
	}
}
