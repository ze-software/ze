// Design: docs/architecture/config/yang-config-design.md -- Value refusals
// Related: validate_sections.go -- section walk and sensitive refusal rendering

package config

import (
	"errors"
	"slices"
	"strings"
	"testing"

	configyang "github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/core/opererr"
)

// TestSectionValidationErrorRendersRefusal checks the sensitive renderer using
// an actual local YANG tree walk. Its schema stays local to the loader, so the
// probe changes neither the registered module set nor the shared validator.
func TestSectionValidationErrorRendersRefusal(t *testing.T) {
	const module = `module ze-section-refusal-conf {
		namespace "urn:ze:section-refusal:conf";
		prefix tsr;
		import ze-extensions { prefix ze; }
		container probe {
			leaf secret-mode {
				type enumeration {
					enum alpha { ze:help "First mode"; }
					enum beta { ze:help "Second mode"; }
				}
				ze:sensitive;
			}
		}
	}`
	loader := configyang.NewLoader()
	if err := loader.LoadEmbedded(); err != nil {
		t.Fatalf("load embedded YANG: %v", err)
	}
	if err := loader.AddModuleFromText("ze-section-refusal-conf.yang", module); err != nil {
		t.Fatalf("add local secret enum: %v", err)
	}
	resolved, err := loader.Resolve()
	if err != nil {
		t.Fatalf("resolve local secret enum: %v", err)
	}
	validator := configyang.NewValidator(resolved)
	refusals := validator.ValidateTreeAllModules("probe", map[string]any{"secret-mode": "alph"})
	if len(refusals) != 1 {
		t.Fatalf("secret enum walk returned %d refusals, want one: %v", len(refusals), refusals)
	}
	section := SectionValidationError{Section: "probe", Sensitive: true, Err: refusals[0]}
	if !section.Err.Secret() {
		t.Fatal("YANG walk did not mark the sensitive enum refusal secret")
	}
	const body = "probe secret-mode: /* SECRET-DATA */ is not an accepted value\n" +
		"secret-mode accepts only these values:\n" +
		"  alpha  First mode\n" +
		"  beta   Second mode\n" +
		"fix: set probe secret-mode <alpha|beta>"
	message := section.Message()
	if want := "probe: " + body; message != want {
		t.Errorf("sensitive section message:\n%s\nwant:\n%s", message, want)
	}
	for _, forbidden := range []string{"\"alph\"", "did you mean", "sensitive value redacted"} {
		if strings.Contains(message, forbidden) {
			t.Errorf("sensitive section message contains %q: %s", forbidden, message)
		}
	}
	var typed *configyang.ValidationError
	if !errors.As(&section.Err, &typed) {
		t.Fatal("section Err chain lost the YANG ValidationError")
	}
	var operator *opererr.Error
	if !errors.As(&section.Err, &operator) {
		t.Fatal("section Err chain lost the operator error")
	}
	if got := operator.Error(); got != body {
		t.Errorf("operator error:\n%s\nwant:\n%s", got, body)
	}
	if got := typed.Error(); got != body {
		t.Errorf("typed error:\n%s\nwant:\n%s", got, body)
	}
	if fix := "set probe secret-mode <alpha|beta>"; operator.Fix() != fix {
		t.Errorf("operator fix %q, want %q", operator.Fix(), fix)
	}
}

// TestValidateCustomSectionsEnumRefusal bypasses parsing to prove the actual
// section validator finds an invalid stored enum, retains its typed chain and
// renders the shipped schema's complete help. Both declared values still pass.
func TestValidateCustomSectionsEnumRefusal(t *testing.T) {
	entry := NewTree()
	entry.Set("run", "/bin/true")
	entry.Set("encoder", "jsn")
	plugin := NewTree()
	plugin.AddListEntry("external", "p1", entry)
	tree := NewTree()
	tree.SetContainer("plugin", plugin)

	found, err := ValidateCustomSections(tree)
	if err != nil {
		t.Fatalf("run section validation: %v", err)
	}
	if len(found) != 1 {
		t.Fatalf("section validation returned %d refusals, want one: %v", len(found), found)
	}
	section := &found[0]
	if section.Section != "plugin" {
		t.Errorf("section %q, want plugin", section.Section)
	}
	if section.Sensitive {
		t.Fatal("shipped plugin encoder is unexpectedly sensitive")
	}
	var typed *configyang.ValidationError
	if !errors.As(&section.Err, &typed) {
		t.Fatal("section Err chain lost the YANG ValidationError")
	}
	if typed.Type != configyang.ErrTypeEnum {
		t.Errorf("error type %v, want enum", typed.Type)
	}
	if typed.Got != "jsn" {
		t.Errorf("actual %q, want jsn", typed.Got)
	}
	if typed.Expected != "json, text" {
		t.Errorf("expected values %q, want json, text in declared order", typed.Expected)
	}
	if typed.Path != "plugin/external[p1]/encoder" {
		t.Errorf("walk path %q, want plugin/external[p1]/encoder", typed.Path)
	}
	if want := []string{"plugin", "external", "p1", "encoder"}; !slices.Equal(typed.Segments(), want) {
		t.Errorf("operator path %q, want %q", typed.Segments(), want)
	}
	const body = "plugin external p1 encoder: \"jsn\" is not an accepted value (did you mean json?)\n" +
		"encoder accepts only these values:\n" +
		"  json  JSON encoding\n" +
		"  text  Human-readable text encoding\n" +
		"fix: set plugin external p1 encoder json"
	if got := typed.Error(); got != body {
		t.Errorf("typed walk error:\n%s\nwant:\n%s", got, body)
	}
	if got, want := section.Message(), "plugin: "+body; got != want {
		t.Errorf("section message:\n%s\nwant:\n%s", got, want)
	}
	var operator *opererr.Error
	if !errors.As(&section.Err, &operator) {
		t.Fatal("section Err chain lost the operator error")
	}
	if got := operator.Error(); got != body {
		t.Errorf("operator error:\n%s\nwant:\n%s", got, body)
	}
	for _, accepted := range []string{"json", "text"} {
		entry.Set("encoder", accepted)
		acceptedFailures, err := ValidateCustomSections(tree)
		if err != nil {
			t.Fatalf("validate accepted encoder %q: %v", accepted, err)
		}
		if len(acceptedFailures) != 0 {
			t.Errorf("accepted encoder %q was refused: %v", accepted, acceptedFailures)
		}
	}
}
