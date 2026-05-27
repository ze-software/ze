package scenario

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseTargetValid(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  Target
	}{
		{"ze", TargetZe},
		{"frr", TargetFRR},
		{"bird", TargetBIRD},
	} {
		got, err := ParseTarget(tc.input)
		require.NoError(t, err)
		assert.Equal(t, tc.want, got)
	}
}

func TestParseTargetInvalid(t *testing.T) {
	for _, input := range []string{"", "gobgp", "ZE", "FRR", "BIRD"} {
		_, err := ParseTarget(input)
		assert.Error(t, err, "input %q should fail", input)
	}
}

func TestTargetDefaultBinary(t *testing.T) {
	assert.Equal(t, "ze", TargetZe.DefaultBinary())
	assert.Equal(t, "bgpd", TargetFRR.DefaultBinary())
	assert.Equal(t, "bird", TargetBIRD.DefaultBinary())
}

func TestTargetSinglePort(t *testing.T) {
	assert.False(t, TargetZe.SinglePort())
	assert.True(t, TargetFRR.SinglePort())
	assert.True(t, TargetBIRD.SinglePort())
}
