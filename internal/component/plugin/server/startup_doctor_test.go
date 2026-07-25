package server

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func TestRegistrationFromRPCDoctorChecks(t *testing.T) {
	t.Parallel()

	input := &rpc.DeclareRegistrationInput{
		DoctorChecks: []rpc.DoctorCheckDecl{
			{
				Name:         "rpki-cache-reachable",
				Phase:        rpc.DoctorPhasePostConfig,
				Order:        100,
				Dependencies: []string{"config-loaded"},
				Platforms:    []string{"any"},
				Codes:        []string{"doctor-rpki-cache-unreachable"},
			},
		},
	}

	reg := registrationFromRPC(input)
	require.Len(t, reg.DoctorChecks, 1)

	dc := reg.DoctorChecks[0]
	assert.Equal(t, "rpki-cache-reachable", dc.Name)
	assert.Equal(t, rpc.DoctorPhasePostConfig, dc.Phase)
	assert.Equal(t, 100, dc.Order)
	assert.Equal(t, []string{"config-loaded"}, dc.Dependencies)
	assert.Equal(t, []string{"any"}, dc.Platforms)
	assert.Equal(t, []string{"doctor-rpki-cache-unreachable"}, dc.Codes)
}

func TestRegistrationFromRPCDoctorChecksEmpty(t *testing.T) {
	t.Parallel()

	input := &rpc.DeclareRegistrationInput{
		Commands: []rpc.CommandDecl{{Name: "test-cmd"}},
	}

	reg := registrationFromRPC(input)
	assert.Empty(t, reg.DoctorChecks)
	assert.Len(t, reg.Commands, 1)
}

func TestRegistrationFromRPCDoctorChecksDefaultPlatform(t *testing.T) {
	t.Parallel()

	input := &rpc.DeclareRegistrationInput{
		DoctorChecks: []rpc.DoctorCheckDecl{
			{
				Name:  "my-check",
				Phase: rpc.DoctorPhasePreConfig,
				Codes: []string{"doctor-my-fail"},
			},
		},
	}

	reg := registrationFromRPC(input)
	require.Len(t, reg.DoctorChecks, 1)
	assert.Equal(t, []string{"any"}, reg.DoctorChecks[0].Platforms)
}

func TestDoctorCheckDeclValidation(t *testing.T) {
	t.Parallel()

	valid := []rpc.DoctorCheckDecl{
		{
			Name:  "valid-check",
			Phase: rpc.DoctorPhasePostConfig,
			Order: 100,
			Codes: []string{"doctor-valid-fail"},
		},
	}
	require.NoError(t, validateDoctorCheckDecls(valid))

	tests := []struct {
		name   string
		checks []rpc.DoctorCheckDecl
		errMsg string
	}{
		{
			name: "empty name",
			checks: []rpc.DoctorCheckDecl{
				{Name: "", Phase: rpc.DoctorPhasePostConfig, Codes: []string{"doctor-x"}},
			},
			errMsg: "invalid doctor check name",
		},
		{
			name: "non-kebab name",
			checks: []rpc.DoctorCheckDecl{
				{Name: "BadName", Phase: rpc.DoctorPhasePostConfig, Codes: []string{"doctor-x"}},
			},
			errMsg: "must be kebab-case",
		},
		{
			name: "invalid phase",
			checks: []rpc.DoctorCheckDecl{
				{Name: "my-check", Phase: "invalid-phase", Codes: []string{"doctor-x"}},
			},
			errMsg: "invalid doctor check phase",
		},
		{
			name: "order too high",
			checks: []rpc.DoctorCheckDecl{
				{Name: "my-check", Phase: rpc.DoctorPhasePostConfig, Order: 10000, Codes: []string{"doctor-x"}},
			},
			errMsg: "invalid doctor check order",
		},
		{
			name: "no codes",
			checks: []rpc.DoctorCheckDecl{
				{Name: "my-check", Phase: rpc.DoctorPhasePostConfig},
			},
			errMsg: "invalid doctor check codes count",
		},
		{
			name: "code without prefix",
			checks: []rpc.DoctorCheckDecl{
				{Name: "my-check", Phase: rpc.DoctorPhasePostConfig, Codes: []string{"bad-code"}},
			},
			errMsg: "must start with \"doctor-\"",
		},
		{
			name: "duplicate name",
			checks: []rpc.DoctorCheckDecl{
				{Name: "dup", Phase: rpc.DoctorPhasePostConfig, Codes: []string{"doctor-a"}},
				{Name: "dup", Phase: rpc.DoctorPhasePostConfig, Codes: []string{"doctor-b"}},
			},
			errMsg: "duplicate doctor check name",
		},
		{
			name: "too many checks",
			checks: func() []rpc.DoctorCheckDecl {
				checks := make([]rpc.DoctorCheckDecl, 17)
				for i := range checks {
					checks[i] = rpc.DoctorCheckDecl{
						Name:  "check-" + string(rune('a'+i)),
						Phase: rpc.DoctorPhasePostConfig,
						Codes: []string{"doctor-x"},
					}
				}
				return checks
			}(),
			errMsg: "too many doctor checks",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validateDoctorCheckDecls(tt.checks)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errMsg)
		})
	}
}
