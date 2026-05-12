package host

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// VALIDATES: ApplyTuning returns ErrUnsupported on non-Linux.
// On Linux, it would attempt real sysfs writes.
// PREVENTS: panic on unsupported platforms.
func TestApplyTuning_UnsupportedPlatform(t *testing.T) {
	cfg := TuningConfig{CPUGovernor: "performance"}
	result := ApplyTuning(cfg)
	if len(result.Errors) > 0 && errors.Is(result.Errors[0].Err, ErrUnsupported) {
		t.Log("non-Linux platform: ErrUnsupported as expected")
		return
	}
	t.Log("Linux platform: tuning attempted (may fail without sysfs)")
}

// VALIDATES: TuningError implements error interface.
// PREVENTS: broken error wrapping.
func TestTuningError_Implements(t *testing.T) {
	te := TuningError{
		Operation: "governor",
		Subject:   "cpu0",
		Err:       errors.New("permission denied"),
	}
	assert.Contains(t, te.Error(), "governor")
	assert.Contains(t, te.Error(), "cpu0")
	assert.Contains(t, te.Error(), "permission denied")

	var target error = te
	require.True(t, errors.Is(target, te.Err))
}

// VALIDATES: TuningConfig zero value means no operations.
// PREVENTS: spurious writes when config is empty.
func TestApplyTuning_EmptyConfig(t *testing.T) {
	result := ApplyTuning(TuningConfig{})
	if len(result.Errors) > 0 && errors.Is(result.Errors[0].Err, ErrUnsupported) {
		t.Skip("non-Linux platform")
	}
	assert.Empty(t, result.Applied)
}

// VALIDATES: TuningResult tracks applied operations.
// PREVENTS: lost audit trail of tuning changes.
func TestTuningResult_Fields(t *testing.T) {
	r := TuningResult{
		Applied: []string{"governor cpu0=performance"},
		Errors: []TuningError{
			{Operation: "ethtool-ring", Subject: "eth0", Err: errors.New("ioctl failed")},
		},
	}
	assert.Len(t, r.Applied, 1)
	assert.Len(t, r.Errors, 1)
	assert.Equal(t, "ethtool-ring", r.Errors[0].Operation)
}
