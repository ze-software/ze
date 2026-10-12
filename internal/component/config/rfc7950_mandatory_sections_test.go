// Design: docs/architecture/config/yang-config-design.md -- mandatory nodes
// Related: validate_sections.go -- mandatoryFailures, the walk under test
// RFC: rfc/short/rfc7950.md

package config

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

// bfdAuthConfig writes a bfd profile whose auth presence container carries the
// given body.
func bfdAuthConfig(auth string) string {
	return "bfd {\n\tprofile p {\n\t\tauth {\n" + auth + "\t\t}\n\t}\n}\n"
}

// TestRFC7950MandatoryOutsideValidatedSections proves the mandatory check runs
// in a section the full ze:validate walk does not cover.
//
// VALIDATES: bfd is outside validatedSections, and LoadConfig refuses a bfd
// profile whose auth container exists without its mandatory type and secret,
// naming both; the same profile with every mandatory leaf loads.
// PREVENTS: the check reaching only validatedSections and the bgp global
// level, which let a bfd, firewall, static or traffic config load with a
// mandatory leaf missing.
//
// Method: LoadConfig over two configs, after checking the section set the walk
// derives from the schema holds bfd and validatedSections does not.
//
// RFC requirement: RFC7950-7.6.5-3 negative -- LoadConfig refuses a bfd profile whose auth presence container exists without its mandatory type and secret leaves, naming both paths.
// RFC requirement: RFC7950-7.6.5-3 positive -- LoadConfig accepts the same profile once auth carries type, key-id and secret.
func TestRFC7950MandatoryOutsideValidatedSections(t *testing.T) {
	validator, err := YANGValidatorWithPlugins(nil)
	if err != nil {
		t.Fatalf("build validator: %v", err)
	}
	if !slices.Contains(validator.Sections(), "bfd") {
		t.Fatalf("schema sections %v do not hold bfd", validator.Sections())
	}
	if slices.Contains(validatedSections, "bfd") {
		t.Fatal("bfd is in validatedSections, so this test no longer proves a section outside it")
	}

	_, err = LoadConfig(bfdAuthConfig("\t\t\tkey-id 1\n"), "test.conf", nil)
	if err == nil {
		t.Fatal("LoadConfig accepted a bfd auth container missing type and secret")
	}
	if !errors.Is(err, ErrCustomValidation) {
		t.Fatalf("refusal is not a validation refusal: %v", err)
	}
	for _, want := range []string{"bfd/profile[p]/auth/type", "bfd/profile[p]/auth/secret"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal %q does not name %q", err, want)
		}
	}

	complete := bfdAuthConfig("\t\t\ttype keyed-sha1\n\t\t\tkey-id 1\n\t\t\tsecret s3cret\n")
	if _, err := LoadConfig(complete, "test.conf", nil); err != nil {
		t.Errorf("LoadConfig refused a bfd auth container carrying every mandatory leaf: %v", err)
	}
}
