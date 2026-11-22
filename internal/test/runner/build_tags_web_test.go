// Design: docs/architecture/testing/ci-format.md -- conditions of a suite that admits its own cases

package runner

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"slices"
	"strings"
	"testing"

	repofeaturetags "github.com/ze-software/ze/internal/le/repo/featuretags"
)

const webSampleCondition = "needs-build-tag:value=ze_fleet_ui_sample:state=present"

// webAdmission answers an admission over a DUT built with tags.
func webAdmission(tags string) *DUTAdmission {
	return &DUTAdmission{dut: newDUTBuildTags("ze", func(string) (*debug.BuildInfo, error) {
		return &debug.BuildInfo{Settings: []debug.BuildSetting{{Key: "-tags", Value: tags}}}, nil
	})}
}

// parsedWebCondition answers the record of a case declaring the sample tag.
func parsedWebCondition(t *testing.T) *Record {
	t.Helper()

	record := &Record{}
	handled, err := ParseDUTOption(record, webSampleCondition)
	if !handled || err != nil {
		t.Fatalf("ParseDUTOption(%q) = %v, %v; want handled with no error", webSampleCondition, handled, err)
	}
	return record
}

// TestParseDUTOptionAcceptsADevelopmentTag pins the vocabulary of a suite
// that admits its own cases: a tag outside the manifest that a review build
// names is a condition, not an unknown tag.
func TestParseDUTOptionAcceptsADevelopmentTag(t *testing.T) {
	record := parsedWebCondition(t)

	want := []buildTagRequirement{{name: "ze_fleet_ui_sample", state: buildTagPresent}}
	if !slices.Equal(record.buildTags, want) {
		t.Fatalf("buildTags = %v, want %v", record.buildTags, want)
	}
}

// TestParseDUTOptionRefusesAnUnknownTag pins that a misspelled tag is a parse
// error, never a case skipped forever.
func TestParseDUTOptionRefusesAnUnknownTag(t *testing.T) {
	_, err := ParseDUTOption(&Record{}, "needs-build-tag:value=ze_fleet_ui_sampel:state=present")
	if err == nil || !strings.Contains(err.Error(), "unknown feature build tag") {
		t.Fatalf("err = %v, want the unknown-tag refusal", err)
	}
}

// TestParseResourceOptionStillRefusesADUTCondition pins the adapters with no
// selected DUT: they keep refusing the condition they could not check.
func TestParseResourceOptionStillRefusesADUTCondition(t *testing.T) {
	handled, err := ParseResourceOption(&Record{}, webSampleCondition)
	if !handled || err == nil {
		t.Fatalf("ParseResourceOption = %v, %v; want handled with the selected-DUT refusal", handled, err)
	}
}

// TestDUTAdmissionRunsACaseWhoseTagIsPresent pins the present path: a DUT
// built with the tag through ze.tags runs the case.
func TestDUTAdmissionRunsACaseWhoseTagIsPresent(t *testing.T) {
	reason, err := webAdmission("ze_core,ze_web,ze_fleet_ui_sample").Admit(parsedWebCondition(t))
	if err != nil || reason != "" {
		t.Fatalf("Admit = %q, %v; want the case admitted", reason, err)
	}
}

// TestDUTAdmissionSkipsACaseWhoseTagIsAbsent pins the absent path: the case
// is skipped, and the reason names the tag and how to build with it.
func TestDUTAdmissionSkipsACaseWhoseTagIsAbsent(t *testing.T) {
	reason, err := webAdmission("ze_core,ze_web").Admit(parsedWebCondition(t))
	if err != nil {
		t.Fatalf("Admit error = %v, want a skip reason", err)
	}
	for _, text := range []string{"value=ze_fleet_ui_sample:state=present", "env ze.tags=ze_fleet_ui_sample"} {
		if !strings.Contains(reason, text) {
			t.Fatalf("skip reason %q does not hold %q", reason, text)
		}
	}
}

// TestDevelopmentBuildTagsStayOutsideTheManifest compares each development
// tag with its declaration: the file it names constrains its build by the
// tag, and the manifest, which every shipped build sets, does not hold it.
func TestDevelopmentBuildTagsStayOutsideTheManifest(t *testing.T) {
	root, err := repoRootFrom("")
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := repofeaturetags.DaemonTags(root)
	if err != nil {
		t.Fatal(err)
	}

	for _, tag := range developmentBuildTags {
		if slices.Contains(manifest, tag.name) {
			t.Errorf("%s is in %s, so every shipped build would set it", tag.name, featureGatesFile)
		}
		source, readErr := os.ReadFile(filepath.Join(root, tag.declaredIn)) //nolint:gosec // a path the table declares, inside the checkout
		if readErr != nil {
			t.Errorf("%s: %v", tag.name, readErr)
			continue
		}
		constraint := regexp.MustCompile(`(?m)^//go:build .*\b` + regexp.QuoteMeta(tag.name) + `\b`)
		if !constraint.Match(source) {
			t.Errorf("%s does not constrain its build by %s", tag.declaredIn, tag.name)
		}
	}
}
