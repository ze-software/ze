// Design: docs/features/ai-first.md -- plugin doctor check registration
// VALIDATES: DoctorChecks field in Registration is validated and queryable.
// PREVENTS: silent registration of malformed doctor checks by plugins.

package registry

import (
	"testing"

	"github.com/ze-software/ze/pkg/plugin/rpc"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validDoctorCheck() DoctorCheckDef {
	return DoctorCheckDef{
		Name:         "test-check",
		Phase:        rpc.DoctorPhasePostConfig,
		Order:        100,
		Dependencies: []string{"config-loaded"},
		Platforms:    []string{"any"},
		Codes:        []string{"doctor-test-fail"},
		Check: func(DoctorCheckContext) []rpc.DoctorCheckDiagnostic {
			return nil
		},
	}
}

func TestRegisterWithDoctorChecks(t *testing.T) {
	t.Cleanup(func() { Restore(Snapshot()) })
	Reset()

	dc := validDoctorCheck()
	err := Register(Registration{
		Name:         "doc-plugin",
		Description:  "test doctor",
		RunEngine:    dummyEngine,
		CLIHandler:   dummyCLI,
		DoctorChecks: []DoctorCheckDef{dc},
	})
	require.NoError(t, err)

	checks := PluginDoctorChecks()
	require.Len(t, checks, 1)
	assert.Equal(t, "doc-plugin", checks[0].PluginName)
	assert.Equal(t, "test-check", checks[0].Name)
	assert.Equal(t, rpc.DoctorPhasePostConfig, checks[0].Phase)
	assert.Equal(t, 100, checks[0].Order)
}

func TestRegisterDoctorCheckValidation(t *testing.T) {
	t.Cleanup(func() { Restore(Snapshot()) })

	tests := []struct {
		name    string
		check   DoctorCheckDef
		wantErr string
	}{
		{
			name:    "empty name",
			check:   func() DoctorCheckDef { dc := validDoctorCheck(); dc.Name = ""; return dc }(),
			wantErr: "invalid name",
		},
		{
			name:    "bad kebab",
			check:   func() DoctorCheckDef { dc := validDoctorCheck(); dc.Name = "Bad_Name"; return dc }(),
			wantErr: "invalid name",
		},
		{
			name:    "invalid phase",
			check:   func() DoctorCheckDef { dc := validDoctorCheck(); dc.Phase = "bogus"; return dc }(),
			wantErr: "invalid phase",
		},
		{
			name:    "nil check",
			check:   func() DoctorCheckDef { dc := validDoctorCheck(); dc.Check = nil; return dc }(),
			wantErr: "nil check function",
		},
		{
			name:    "missing dependencies",
			check:   func() DoctorCheckDef { dc := validDoctorCheck(); dc.Dependencies = nil; return dc }(),
			wantErr: "missing dependencies",
		},
		{
			name:    "missing platforms",
			check:   func() DoctorCheckDef { dc := validDoctorCheck(); dc.Platforms = nil; return dc }(),
			wantErr: "missing platforms",
		},
		{
			name:    "missing codes",
			check:   func() DoctorCheckDef { dc := validDoctorCheck(); dc.Codes = nil; return dc }(),
			wantErr: "missing codes",
		},
		{
			name:    "code without doctor prefix",
			check:   func() DoctorCheckDef { dc := validDoctorCheck(); dc.Codes = []string{"bad-code"}; return dc }(),
			wantErr: "doctor- prefix",
		},
		{
			name:    "invalid platform",
			check:   func() DoctorCheckDef { dc := validDoctorCheck(); dc.Platforms = []string{"windows"}; return dc }(),
			wantErr: "invalid platform",
		},
		{
			name:    "duplicate platform",
			check:   func() DoctorCheckDef { dc := validDoctorCheck(); dc.Platforms = []string{"any", "any"}; return dc }(),
			wantErr: "duplicate platform",
		},
		{
			name: "duplicate code",
			check: func() DoctorCheckDef {
				dc := validDoctorCheck()
				dc.Codes = []string{"doctor-a", "doctor-a"}
				return dc
			}(),
			wantErr: "duplicate code",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Reset()
			err := Register(Registration{
				Name:         "val-plugin",
				Description:  "test",
				RunEngine:    dummyEngine,
				CLIHandler:   dummyCLI,
				DoctorChecks: []DoctorCheckDef{tt.check},
			})
			require.Error(t, err)
			assert.ErrorIs(t, err, ErrInvalidDoctorCheck)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestPluginDoctorChecksEmpty(t *testing.T) {
	t.Cleanup(func() { Restore(Snapshot()) })
	Reset()

	err := Register(Registration{
		Name:        "no-doctor",
		Description: "test",
		RunEngine:   dummyEngine,
		CLIHandler:  dummyCLI,
	})
	require.NoError(t, err)
	assert.Empty(t, PluginDoctorChecks())
}

func TestPluginDoctorChecksMultiplePlugins(t *testing.T) {
	t.Cleanup(func() { Restore(Snapshot()) })
	Reset()

	dc1 := validDoctorCheck()
	dc1.Name = "check-alpha"
	dc1.Codes = []string{"doctor-alpha"}

	dc2 := validDoctorCheck()
	dc2.Name = "check-beta"
	dc2.Codes = []string{"doctor-beta"}

	require.NoError(t, Register(Registration{
		Name:         "plugin-a",
		Description:  "a",
		RunEngine:    dummyEngine,
		CLIHandler:   dummyCLI,
		DoctorChecks: []DoctorCheckDef{dc1},
	}))
	require.NoError(t, Register(Registration{
		Name:         "plugin-b",
		Description:  "b",
		RunEngine:    dummyEngine,
		CLIHandler:   dummyCLI,
		DoctorChecks: []DoctorCheckDef{dc2},
	}))

	checks := PluginDoctorChecks()
	require.Len(t, checks, 2)

	names := map[string]string{}
	for _, c := range checks {
		names[c.Name] = c.PluginName
	}
	assert.Equal(t, "plugin-a", names["check-alpha"])
	assert.Equal(t, "plugin-b", names["check-beta"])
}

func TestDoctorCheckFuncReceivesContext(t *testing.T) {
	t.Cleanup(func() { Restore(Snapshot()) })
	Reset()

	var received DoctorCheckContext
	dc := validDoctorCheck()
	dc.Check = func(ctx DoctorCheckContext) []rpc.DoctorCheckDiagnostic {
		received = ctx
		return []rpc.DoctorCheckDiagnostic{{
			Code:     "doctor-test-fail",
			Severity: "warning",
			Message:  "test failure",
		}}
	}

	require.NoError(t, Register(Registration{
		Name:         "ctx-plugin",
		Description:  "test",
		RunEngine:    dummyEngine,
		CLIHandler:   dummyCLI,
		DoctorChecks: []DoctorCheckDef{dc},
	}))

	checks := PluginDoctorChecks()
	require.Len(t, checks, 1)

	ctx := DoctorCheckContext{
		Tree:      "fake-tree",
		ConfigDir: "/etc/ze",
		Platform:  "fake-platform",
	}
	diags := checks[0].Check(ctx)
	require.Len(t, diags, 1)
	assert.Equal(t, "doctor-test-fail", diags[0].Code)
	assert.Equal(t, "warning", diags[0].Severity)
	assert.Equal(t, "fake-tree", received.Tree)
	assert.Equal(t, "/etc/ze", received.ConfigDir)
}

func TestIsKebabCase(t *testing.T) {
	assert.True(t, isKebabCase("hello"))
	assert.True(t, isKebabCase("hello-world"))
	assert.True(t, isKebabCase("a1-b2"))
	assert.False(t, isKebabCase(""))
	assert.False(t, isKebabCase("-start"))
	assert.False(t, isKebabCase("end-"))
	assert.False(t, isKebabCase("double--dash"))
	assert.False(t, isKebabCase("Upper"))
	assert.False(t, isKebabCase("under_score"))
}
