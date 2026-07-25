// Design: docs/features/ai-first.md — doctor check registration tests

package doctor

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/diagnostic"
)

func TestDoctorCheckRegistryOrdersByPhaseOrderName(t *testing.T) {
	// VALIDATES: AC-2 registered checks are returned sorted by phase, order, then name.
	// PREVENTS: init registration order leaking into ze doctor output order.
	reg := newDoctorCheckRegistry()
	maxInt := int(^uint(0) >> 1)
	checks := []doctorCheck{
		testDoctorCheck("post-zeta", doctorCheckPhasePostConfig, 10, "doctor-shared-fixture"),
		testDoctorCheck("pre-max", doctorCheckPhasePreConfig, maxInt, "doctor-pre-max-fixture"),
		testDoctorCheck("missing", doctorCheckPhaseMissingConfig, 0, "doctor-missing-fixture"),
		testDoctorCheck("post-negative", doctorCheckPhasePostConfig, -1, "doctor-post-negative-fixture"),
		testDoctorCheck("pre-negative", doctorCheckPhasePreConfig, -5, "doctor-shared-fixture"),
		testDoctorCheck("post-alpha", doctorCheckPhasePostConfig, 10, "doctor-post-alpha-fixture"),
	}
	for i := range checks {
		require.NoError(t, reg.register(checks[i]))
	}

	assert.Equal(t, []string{
		"pre-negative",
		"pre-max",
		"missing",
		"post-negative",
		"post-alpha",
		"post-zeta",
	}, doctorCheckNames(reg.checks()))
	assert.Equal(t, []string{"post-negative", "post-alpha", "post-zeta"}, doctorCheckNames(reg.checksForPhase(doctorCheckPhasePostConfig)))
}

func TestDoctorCheckRegistryRejectsDuplicateName(t *testing.T) {
	// VALIDATES: AC-3 duplicate doctor check names are rejected.
	// PREVENTS: one registration silently replacing or shadowing another check.
	reg := newDoctorCheckRegistry()
	require.NoError(t, reg.register(testDoctorCheck("duplicate", doctorCheckPhasePostConfig, 0, "doctor-duplicate-a")))

	err := reg.register(testDoctorCheck("duplicate", doctorCheckPhasePostConfig, 1, "doctor-duplicate-b"))
	require.Error(t, err)
	assert.ErrorContains(t, err, "duplicate")
}

func TestDoctorCheckRegistryRejectsDuplicateCodeWithinCheck(t *testing.T) {
	// VALIDATES: AC-4 a single check cannot declare the same diagnostic code twice.
	// PREVENTS: misleading check metadata that overstates code coverage.
	reg := newDoctorCheckRegistry()
	check := testDoctorCheck("duplicate-code", doctorCheckPhasePostConfig, 0, "doctor-duplicate-code")
	check.Codes = append(check.Codes, "doctor-duplicate-code")

	err := reg.register(check)
	require.Error(t, err)
	assert.ErrorContains(t, err, "duplicate diagnostic code")
}

func TestDoctorCheckRegistryRejectsUnknownPhase(t *testing.T) {
	// VALIDATES: boundary testing for the closed doctor phase set.
	// PREVENTS: checks registered in an unreachable phase from being silently ignored.
	reg := newDoctorCheckRegistry()
	check := testDoctorCheck("unknown-phase", doctorCheckPhase("unknown"), 0, "doctor-unknown-phase")

	err := reg.register(check)
	require.Error(t, err)
	assert.ErrorContains(t, err, "unknown phase")
}

func TestDoctorRegisteredCheckCodesHaveMetadata(t *testing.T) {
	// VALIDATES: AC-5 every registered doctor-* code resolves through diagnostic.Lookup.
	// PREVENTS: ze doctor emitting codes that ze explain cannot describe.
	diagnostic.RegisterBuiltinCodes()
	for _, check := range defaultDoctorCheckRegistry.checks() {
		require.NotEmpty(t, check.Codes, "check %q must declare diagnostic codes", check.Name)
		for _, code := range check.Codes {
			if !strings.HasPrefix(code, "doctor-") {
				t.Fatalf("check %q declares non-doctor diagnostic code %q", check.Name, code)
			}
			if diagnostic.Lookup(code) == nil {
				t.Fatalf("check %q declares %q without diagnostic metadata", check.Name, code)
			}
		}
	}
}

func testDoctorCheck(name string, phase doctorCheckPhase, order int, code string) doctorCheck {
	return doctorCheck{
		Name:         name,
		Phase:        phase,
		Order:        order,
		Component:    "test",
		Dependencies: []string{"test-dependency"},
		Platforms:    []string{doctorCheckPlatformAny},
		Codes:        []string{code},
		Check:        func(doctorCheckContext) []diagnostic.Diagnostic { return nil },
	}
}

func doctorCheckNames(checks []doctorCheck) []string {
	names := make([]string, 0, len(checks))
	for i := range checks {
		names = append(names, checks[i].Name)
	}
	return names
}
