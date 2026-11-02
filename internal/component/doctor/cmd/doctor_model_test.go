package cmd

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	_ "github.com/ze-software/ze/internal/component/doctor/yang" // registers ze-doctor-cmd.yang
)

// TestDoctorLeavesKeepTheLocalGrammar proves the leaves ze-doctor-cmd.yang
// declares on doctor and show doctor judge every spelling `ze doctor` has
// always taken, and that the selecting words reach the producer as typed.
//
// Method: validate each local spelling through command.ValidateModelArgs, the
// validator both routes call before the handler, and read the tokens back.
func TestDoctorLeavesKeepTheLocalGrammar(t *testing.T) {
	spellings := [][]string{
		{},
		{"--json"},
		{"config", "/etc/ze/ze.conf"},
		{"--json", "config", "/etc/ze/ze.conf"},
		{"config", "-"},
		{"kernel-capabilities"},
		{"--json", "kernel-capabilities"},
	}
	for _, path := range []string{"doctor", "show doctor"} {
		for _, words := range spellings {
			validated, err := command.ValidateModelArgs(path, words, nil)
			require.NoError(t, err, "%s %q refused by the model", path, words)
			require.Equal(t, words, validated.Tokens(), "%s %q reached the producer changed", path, words)
		}
	}
}
