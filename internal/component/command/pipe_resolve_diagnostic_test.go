// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic registration for pipe resolve failures.
// Related: pipe_resolve.go -- CodeDoctorResolvePTRFailed; ../resolve/cmd/register_doctor_codes.go -- registration.

package command_test

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	_ "github.com/ze-software/ze/internal/component/resolve/cmd"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// TestReverseLookupDoctorCodeRegistered verifies that linking resolve/cmd
// registers CodeDoctorResolvePTRFailed in both diagnostic.Lookup and
// diagnostic.EventCodes.
//
// VALIDATES: AC-2 -- doctor-resolve-ptr-failed is described and declared as a doctor event code.
// PREVENTS: ReverseLookup emitting an unregistered or undescribed doctor code.
func TestReverseLookupDoctorCodeRegistered(t *testing.T) {
	meta := diagnostic.Lookup(command.CodeDoctorResolvePTRFailed)
	require.NotNil(t, meta)
	assert.Equal(t, "doctor-resolve-ptr-failed", meta.Code)
	assert.NotEmpty(t, meta.Title)
	assert.NotEmpty(t, meta.Description)
	assert.NotEmpty(t, meta.Examples)
	require.True(t, slices.Contains(diagnostic.EventCodes(), command.CodeDoctorResolvePTRFailed))
}
