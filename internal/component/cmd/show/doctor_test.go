package show

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	"codeberg.org/thomas-mangin/ze/internal/core/diagnostic"
)

func TestHandleShowDoctor_NoProvider(t *testing.T) {
	diagnostic.RegisterDoctorProvider(nil)
	resp, err := handleShowDoctor(nil, nil)
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	result, ok := resp.Data.(diagnostic.DoctorResult)
	require.True(t, ok)
	assert.True(t, result.Ready)
	assert.Empty(t, result.Diagnostics)
}

func TestHandleShowDoctor_WithProvider(t *testing.T) {
	diagnostic.RegisterDoctorProvider(func(_ string) []diagnostic.Diagnostic {
		return []diagnostic.Diagnostic{{
			Code:     "doctor-test-diag",
			Severity: diagnostic.SeverityWarning,
			Message:  "test warning",
		}}
	})
	defer diagnostic.RegisterDoctorProvider(nil)

	resp, err := handleShowDoctor(nil, nil)
	require.NoError(t, err)

	result, ok := resp.Data.(diagnostic.DoctorResult)
	require.True(t, ok)
	assert.True(t, result.Ready, "warnings should not prevent readiness")
	require.Len(t, result.Diagnostics, 1)
	assert.Equal(t, "doctor-test-diag", result.Diagnostics[0].Code)
}

func TestHandleShowDoctor_ErrorMeansNotReady(t *testing.T) {
	diagnostic.RegisterDoctorProvider(func(_ string) []diagnostic.Diagnostic {
		return []diagnostic.Diagnostic{{
			Code:     "doctor-test-error",
			Severity: diagnostic.SeverityError,
			Message:  "test error",
		}}
	})
	defer diagnostic.RegisterDoctorProvider(nil)

	resp, err := handleShowDoctor(nil, nil)
	require.NoError(t, err)

	result, ok := resp.Data.(diagnostic.DoctorResult)
	require.True(t, ok)
	assert.False(t, result.Ready, "errors should prevent readiness")
}
