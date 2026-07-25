package web

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
)

func TestSplitConfigPath(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"environment/ssh/enabled", []string{"environment", "ssh", "enabled"}},
		{"system/host", []string{"system", "host"}},
		{"", nil},
		{"single", []string{"single"}},
		{"/leading/slash", []string{"leading", "slash"}},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := splitConfigPath(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetConfigValue_Present(t *testing.T) {
	tree := config.NewTree()
	env := tree.GetOrCreateContainer("environment")
	ssh := env.GetOrCreateContainer("ssh")
	ssh.Set("enabled", "true")

	got := getConfigValue(tree, "environment/ssh/enabled")
	assert.Equal(t, "true", got)
}

func TestGetConfigValue_Missing(t *testing.T) {
	tree := config.NewTree()
	got := getConfigValue(tree, "environment/ssh/enabled")
	assert.Empty(t, got)
}

func TestGetConfigValue_NilTree(t *testing.T) {
	got := getConfigValue(nil, "environment/ssh/enabled")
	assert.Empty(t, got)
}

func TestGetConfigListItems(t *testing.T) {
	tree := config.NewTree()
	env := tree.GetOrCreateContainer("environment")
	ssh := env.GetOrCreateContainer("ssh")
	ssh.AddListEntry("server", "0.0.0.0:22", config.NewTree())
	ssh.AddListEntry("server", "[::]:22", config.NewTree())

	items := getConfigListItems(tree, "environment/ssh", "server")
	require.Len(t, items, 2)
}

func TestGetConfigListItems_NilTree(t *testing.T) {
	items := getConfigListItems(nil, "environment/ssh", "server")
	assert.Nil(t, items)
}

func TestGetConfigListItems_MissingPath(t *testing.T) {
	tree := config.NewTree()
	items := getConfigListItems(tree, "environment/ssh", "server")
	assert.Nil(t, items)
}

func TestBuildSSHFormData(t *testing.T) {
	tree := config.NewTree()
	env := tree.GetOrCreateContainer("environment")
	ssh := env.GetOrCreateContainer("ssh")
	ssh.Set("enabled", "true")
	ssh.Set("host-key", "/etc/ze/host_key")
	ssh.Set("idle-timeout", "300")
	ssh.Set("max-sessions", "10")

	form := BuildSSHFormData(tree)
	assert.Equal(t, "SSH Configuration", form.Title)
	require.Len(t, form.Fields, 5)
	assert.Equal(t, "true", form.Fields[0].Value)
	assert.Equal(t, "toggle", form.Fields[0].Type)
	assert.Equal(t, "/etc/ze/host_key", form.Fields[2].Value)
	assert.Equal(t, "300", form.Fields[3].Value)
}

func TestBuildSSHFormData_NilTree(t *testing.T) {
	form := BuildSSHFormData(nil)
	assert.Equal(t, "SSH Configuration", form.Title)
	require.Len(t, form.Fields, 5)
	assert.Empty(t, form.Fields[0].Value)
}

func TestBuildWebFormData(t *testing.T) {
	tree := config.NewTree()
	env := tree.GetOrCreateContainer("environment")
	web := env.GetOrCreateContainer("web")
	web.Set("enabled", "true")
	web.Set("insecure", "false")

	form := BuildWebFormData(tree)
	assert.Equal(t, "Web Configuration", form.Title)
	require.Len(t, form.Fields, 3)
	assert.Equal(t, "true", form.Fields[0].Value)
	assert.Equal(t, "false", form.Fields[2].Value)
}

func TestBuildTelemetryFormData(t *testing.T) {
	tree := config.NewTree()
	tel := tree.GetOrCreateContainer("telemetry")
	prom := tel.GetOrCreateContainer("prometheus")
	prom.Set("enabled", "true")
	prom.Set("path", "/metrics")

	form := BuildTelemetryFormData(tree)
	assert.Equal(t, "Telemetry Configuration", form.Title)
	require.Len(t, form.Fields, 12)
	assert.Equal(t, "true", form.Fields[0].Value)
	assert.Equal(t, "/metrics", form.Fields[2].Value)
}

func TestBuildTelemetryFormData_NilTree(t *testing.T) {
	form := BuildTelemetryFormData(nil)
	assert.Equal(t, "Telemetry Configuration", form.Title)
	require.Len(t, form.Fields, 12)
	assert.Empty(t, form.Fields[0].Value)
}

func TestBuildTACACSFormData(t *testing.T) {
	tree := config.NewTree()
	sys := tree.GetOrCreateContainer("system")
	auth := sys.GetOrCreateContainer("authentication")
	tacacs := auth.GetOrCreateContainer("tacacs")
	tacacs.Set("timeout", "10")
	tacacs.Set("source-address", "10.0.0.1")
	tacacs.Set("authorization", "true")
	tacacs.Set("accounting", "true")

	form := BuildTACACSFormData(tree)
	assert.Equal(t, "TACACS+ Configuration", form.Title)
	require.Len(t, form.Fields, 5)
	assert.Equal(t, "10", form.Fields[1].Value)
	assert.Equal(t, "10.0.0.1", form.Fields[2].Value)
}

func TestBuildMCPFormData(t *testing.T) {
	tree := config.NewTree()
	env := tree.GetOrCreateContainer("environment")
	mcp := env.GetOrCreateContainer("mcp")
	mcp.Set("enabled", "true")
	mcp.Set("auth-mode", "bearer")

	form := BuildMCPFormData(tree)
	assert.Equal(t, "MCP Configuration", form.Title)
	require.Len(t, form.Fields, 10)
	assert.Equal(t, "true", form.Fields[0].Value)
	assert.Equal(t, "bearer", form.Fields[2].Value)
	assert.Equal(t, "dropdown", form.Fields[2].Type)
	assert.Equal(t, []string{"none", "bearer", "bearer-list", "oauth"}, form.Fields[2].Options)
}

func TestBuildMCPFormData_SensitiveFields(t *testing.T) {
	form := BuildMCPFormData(nil)
	tokenField := form.Fields[3]
	assert.Equal(t, "password", tokenField.Type)
	tlsKeyField := form.Fields[9]
	assert.Equal(t, "password", tlsKeyField.Type)
}

func TestBuildLookingGlassFormData(t *testing.T) {
	tree := config.NewTree()
	env := tree.GetOrCreateContainer("environment")
	lg := env.GetOrCreateContainer("looking-glass")
	lg.Set("enabled", "true")
	lg.Set("tls", "false")

	form := BuildLookingGlassFormData(tree)
	assert.Equal(t, "Looking Glass Configuration", form.Title)
	require.Len(t, form.Fields, 3)
	assert.Equal(t, "true", form.Fields[0].Value)
}

func TestBuildAPIFormData(t *testing.T) {
	tree := config.NewTree()
	env := tree.GetOrCreateContainer("environment")
	api := env.GetOrCreateContainer("api-server")
	rest := api.GetOrCreateContainer("rest")
	rest.Set("enabled", "true")
	rest.Set("cors-origin", "*")

	form := BuildAPIFormData(tree)
	assert.Equal(t, "API Configuration", form.Title)
	require.Len(t, form.Fields, 8)
	assert.Equal(t, "password", form.Fields[0].Type)
	assert.Equal(t, "true", form.Fields[1].Value)
	assert.Equal(t, "*", form.Fields[3].Value)
}

func TestBuildAPIFormData_SensitiveFields(t *testing.T) {
	form := BuildAPIFormData(nil)
	assert.Equal(t, "password", form.Fields[0].Type)
	assert.Equal(t, "password", form.Fields[7].Type)
}

func TestRenderServicePageContent_KnownServices(t *testing.T) {
	unknown := []string{"unknown", "dns", "dhcp", "radius", ""}
	for _, svc := range unknown {
		t.Run(svc+"_not_dispatched", func(t *testing.T) {
			_, ok := renderServicePageContent(nil, svc, nil)
			assert.False(t, ok)
		})
	}
}

func TestRenderServicePageContent_Unknown(t *testing.T) {
	_, ok := renderServicePageContent(nil, "unknown", nil)
	assert.False(t, ok)
}

func TestRenderL2TPPageContent_Dispatch(t *testing.T) {
	// Only test paths that do not require a renderer (unknown falls through).
	_, ok := renderL2TPPageContent(nil, []string{"unknown"}, nil)
	assert.False(t, ok)
}
