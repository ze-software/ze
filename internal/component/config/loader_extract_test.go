package config

import (
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	configyang "github.com/ze-software/ze/internal/component/config/yang"

	// Blank imports trigger init() registration of service YANG modules so
	// YANGSchema() picks up environment/{api-server,web,mcp,looking-glass}
	// used by these tests.
	_ "github.com/ze-software/ze/internal/component/api/grpc/yang"
	_ "github.com/ze-software/ze/internal/component/api/rest/yang"
	_ "github.com/ze-software/ze/internal/component/api/yang"
	_ "github.com/ze-software/ze/internal/component/gnmi/yang"
	_ "github.com/ze-software/ze/internal/component/lg/yang"
	_ "github.com/ze-software/ze/internal/component/mcp/yang"
	_ "github.com/ze-software/ze/internal/component/web/yang"
)

const labelTestSecret = "a-managed-client-secret-of-32-chars"

func hubLabelConfig(labels string) string {
	return "bus { hub { server central { client edge-01 { secret " + labelTestSecret + "; " + labels + " } } } }"
}

// VALIDATES: A loaded device retains its secret and each label, including the
// longest key and value.
// PREVENTS: Accepting labels in the schema and dropping them before the registry.
func TestExtractHubConfigLabels(t *testing.T) {
	key := "a" + strings.Repeat("b", 62)
	value := strings.Repeat("x", 128)
	result, err := LoadConfig(hubLabelConfig("label "+key+" { value "+value+"; } label site { value \"London North\"; }"), filepath.Join(t.TempDir(), "hub.conf"), nil)
	require.NoError(t, err)
	hub, err := ExtractHubConfig(result.Tree)
	require.NoError(t, err)
	require.Len(t, hub.Servers, 1)
	assert.Equal(t, map[string]string{key: value, "site": "London North"}, hub.Servers[0].ClientLabels["edge-01"])
	assert.Equal(t, labelTestSecret, hub.Servers[0].Clients["edge-01"])
}

// VALIDATES: A file with 32 labels loads, while the upper bounds, key syntax
// and required value are enforced by the normal loader.
// PREVENTS: A schema restriction on the wrong list or leaf passing unnoticed.
func TestExtractHubConfigLabelConstraints(t *testing.T) {
	var labels strings.Builder
	for i := range 32 {
		labels.WriteString("label k" + strconv.Itoa(i) + " { value v; } ")
	}
	path := filepath.Join(t.TempDir(), "hub.conf")
	_, err := LoadConfig(hubLabelConfig(labels.String()), path, nil)
	require.NoError(t, err)
	for _, test := range []struct{ name, config string }{
		{"33 entries", labels.String() + "label excess { value v; }"},
		{"empty key", `label "" { value v; }`},
		{"key too long", "label " + strings.Repeat("a", 64) + " { value v; }"},
		{"invalid key", "label 3site { value v; }"},
		{"missing value", "label site { }"},
		{"empty value", `label site { value ""; }`},
		{"value too long", "label site { value " + strings.Repeat("x", 129) + "; }"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, loadErr := LoadConfig(hubLabelConfig(test.config), path, nil)
			require.Error(t, loadErr)
		})
	}
}

// VALIDATES: Printable Unicode and spaces reach the device, while a tab and
// an invisible formatting character are refused with a repair command.
// PREVENTS: Accepting control characters or excluding printable non-ASCII text.
func TestExtractHubConfigLabelPrintable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hub.conf")
	for _, test := range []struct {
		value string
		valid bool
	}{
		{"Île de Ré", true},
		{"London\tNorth", false},
		{"London\u200bNorth", false},
	} {
		t.Run(strconv.Quote(test.value), func(t *testing.T) {
			result, err := LoadConfig(hubLabelConfig("label site { value "+QuoteIfNeeded(test.value)+"; }"), path, nil)
			if test.valid {
				require.NoError(t, err)
				hub, err := ExtractHubConfig(result.Tree)
				require.NoError(t, err)
				assert.Equal(t, test.value, hub.Servers[0].ClientLabels["edge-01"]["site"])
				return
			}
			require.ErrorContains(t, err, "printable")
			assert.Contains(t, err.Error(), "set bus hub server central client edge-01 label site value")
		})
	}
}

// VALIDATES: Two server blocks may declare one device with the same labels.
// A missing, extra or changed label identifies both blocks and its repair.
// PREVENTS: The device's labels depending on which listener it connects to.
func TestExtractHubConfigRefusesDisagreeingLabels(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hub.conf")
	block := func(name, labels string) string {
		return "server " + name + " { client edge-01 { secret " + labelTestSecret + "; " + labels + " } } "
	}
	site := "label site { value lon; }"
	for _, test := range []struct{ name, first, second, fix string }{
		{"same", site, site, ""},
		{"different value", site, "label site { value par; }", "set bus hub server south client edge-01 label site value lon"},
		{"missing key", site, "", "set bus hub server south client edge-01 label site value lon"},
		{"extra key", "", site, "delete bus hub server south client edge-01 label site"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := LoadConfig("bus { hub { "+block("north", test.first)+block("south", test.second)+"} }", path, nil)
			if test.fix == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			for _, word := range []string{"north", "south", "edge-01", "site", test.fix} {
				assert.Contains(t, err.Error(), word)
			}
		})
	}
}

// VALIDATES: Extraction and commit verification bound the distinct device
// names across all blocks and allow a second declaration at the limit.
// PREVENTS: Counting duplicates twice or enforcing the limit per listener.
func TestExtractHubConfigRefusesTooManyClients(t *testing.T) {
	tree, bus, hub := NewTree(), NewTree(), NewTree()
	tree.SetContainer("bus", bus)
	bus.SetContainer("hub", hub)
	central, backup, client := NewTree(), NewTree(), NewTree()
	hub.AddListEntry("server", "central", central)
	hub.AddListEntry("server", "backup", backup)
	client.Set("secret", labelTestSecret)
	for i := range 4096 {
		central.AddListEntry("client", "edge-"+strconv.Itoa(i), client)
	}
	backup.AddListEntry("client", "edge-0", client)
	_, err := ExtractHubConfig(tree)
	require.NoError(t, err)
	backup.AddListEntry("client", "edge-extra", client)
	_, verifyErr := ValidateCustomSections(tree)
	require.ErrorContains(t, verifyErr, "4097")
	_, err = ExtractHubConfig(tree)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "4097")
	assert.Contains(t, err.Error(), "4096")
	assert.Contains(t, err.Error(), "delete bus hub server backup client edge-extra")
}

// TestExtractAPIConfig_RESTSingleServer verifies a single named list entry
// is read into cfg.REST[0] and the transport is marked enabled.
//
// VALIDATES: ExtractAPIConfig returns one REST endpoint when one list entry
// is present. The YANG container->list conversion preserves single-entry
// parsing.
// PREVENTS: Regression if the YANG shape or extraction walker changes.
func TestExtractAPIConfig_RESTSingleServer(t *testing.T) {
	input := `
environment {
	api-server {
		rest {
			enabled true;
			server main {
				ip 127.0.0.1;
				port 8091;
			}
		}
	}
}
`
	schema, err := YANGSchema()
	require.NoError(t, err)
	p := NewParser(schema)
	tree, err := p.Parse(input)
	require.NoError(t, err)

	cfg, ok := ExtractAPIConfig(tree)
	require.True(t, ok, "ExtractAPIConfig returned not-ok with REST enabled")
	require.True(t, cfg.RESTOn, "RESTOn should be true")
	require.False(t, cfg.GRPCOn, "GRPCOn should be false")
	require.Len(t, cfg.REST, 1)
	assert.Equal(t, "127.0.0.1", cfg.REST[0].Host)
	assert.Equal(t, "8091", cfg.REST[0].Port)
	assert.Equal(t, "127.0.0.1:8091", cfg.REST[0].Listen())
}

// TestExtractAPIConfig_RESTMultipleServers verifies every server list entry
// is returned in insertion order.
//
// VALIDATES: AC-5 (Chunk 2 slice of REST endpoints).
// PREVENTS: Silent drop of extra list entries (the "first entry only" bug
// that this spec exists to kill).
func TestExtractAPIConfig_RESTMultipleServers(t *testing.T) {
	input := `
environment {
	api-server {
		rest {
			enabled true;
			server primary {
				ip 0.0.0.0;
				port 8081;
			}
			server admin {
				ip 127.0.0.1;
				port 18081;
			}
		}
	}
}
`
	schema, err := YANGSchema()
	require.NoError(t, err)
	p := NewParser(schema)
	tree, err := p.Parse(input)
	require.NoError(t, err)

	cfg, ok := ExtractAPIConfig(tree)
	require.True(t, ok)
	require.True(t, cfg.RESTOn)
	require.Len(t, cfg.REST, 2)
	assert.Equal(t, "0.0.0.0", cfg.REST[0].Host)
	assert.Equal(t, "8081", cfg.REST[0].Port)
	assert.Equal(t, "127.0.0.1", cfg.REST[1].Host)
	assert.Equal(t, "18081", cfg.REST[1].Port)
}

// TestExtractAPIConfig_GRPCMultipleServers verifies gRPC transport reads
// the server list independently of REST.
//
// VALIDATES: AC-6.
// PREVENTS: Cross-talk between REST and gRPC server lists.
func TestExtractAPIConfig_GRPCMultipleServers(t *testing.T) {
	input := `
environment {
	api-server {
		grpc {
			enabled true;
			server v4 {
				ip 0.0.0.0;
				port 50051;
			}
			server v6 {
				ip ::1;
				port 50052;
			}
			tls-cert /etc/ze/cert.pem;
			tls-key /etc/ze/key.pem;
		}
	}
}
`
	schema, err := YANGSchema()
	require.NoError(t, err)
	p := NewParser(schema)
	tree, err := p.Parse(input)
	require.NoError(t, err)

	cfg, ok := ExtractAPIConfig(tree)
	require.True(t, ok)
	require.False(t, cfg.RESTOn)
	require.True(t, cfg.GRPCOn)
	require.Len(t, cfg.GRPC, 2)
	assert.Equal(t, "0.0.0.0", cfg.GRPC[0].Host)
	assert.Equal(t, "50051", cfg.GRPC[0].Port)
	assert.Equal(t, "::1", cfg.GRPC[1].Host)
	assert.Equal(t, "50052", cfg.GRPC[1].Port)
	assert.Equal(t, "/etc/ze/cert.pem", cfg.GRPCTLSCert)
	assert.Equal(t, "/etc/ze/key.pem", cfg.GRPCTLSKey)
}

// TestExtractAPIConfig_RESTEmptyListUsesDefaults verifies that enabling the
// transport without naming any list entry still produces one endpoint using
// the YANG refine defaults.
//
// VALIDATES: Extraction synthesizes a default entry when the list is empty,
// matching the legacy "container server { }" shape so existing configs
// upgrade cleanly.
// PREVENTS: Enabled transport with zero entries landing in the binder and
// silently doing nothing.
func TestExtractAPIConfig_RESTEmptyListUsesDefaults(t *testing.T) {
	input := `
environment {
	api-server {
		rest {
			enabled true;
		}
	}
}
`
	schema, err := YANGSchema()
	require.NoError(t, err)
	p := NewParser(schema)
	tree, err := p.Parse(input)
	require.NoError(t, err)

	cfg, ok := ExtractAPIConfig(tree)
	require.True(t, ok)
	require.True(t, cfg.RESTOn)
	require.Len(t, cfg.REST, 1)
	assert.Equal(t, "0.0.0.0", cfg.REST[0].Host)
	assert.Equal(t, "8081", cfg.REST[0].Port)
}

// TestExtractAPIConfig_GRPCEmptyListUsesDefaults mirrors the REST empty-list
// case for gRPC with its own refine defaults.
func TestExtractAPIConfig_GRPCEmptyListUsesDefaults(t *testing.T) {
	input := `
environment {
	api-server {
		grpc {
			enabled true;
		}
	}
}
`
	schema, err := YANGSchema()
	require.NoError(t, err)
	p := NewParser(schema)
	tree, err := p.Parse(input)
	require.NoError(t, err)

	cfg, ok := ExtractAPIConfig(tree)
	require.True(t, ok)
	require.True(t, cfg.GRPCOn)
	require.Len(t, cfg.GRPC, 1)
	assert.Equal(t, "0.0.0.0", cfg.GRPC[0].Host)
	assert.Equal(t, "50051", cfg.GRPC[0].Port)
}

// TestExtractAPIConfig_Disabled verifies both transports off returns not-ok.
func TestExtractAPIConfig_Disabled(t *testing.T) {
	input := `
environment {
	api-server {
		rest { enabled false; }
		grpc { enabled false; }
	}
}
`
	schema, err := YANGSchema()
	require.NoError(t, err)
	p := NewParser(schema)
	tree, err := p.Parse(input)
	require.NoError(t, err)

	cfg, ok := ExtractAPIConfig(tree)
	assert.False(t, ok, "expected not-ok when both transports disabled")
	assert.False(t, cfg.RESTOn)
	assert.False(t, cfg.GRPCOn)
}

// TestExtractAPIConfig_Token verifies the shared bearer token is read into
// the top-level APIConfig, not per-transport.
func TestExtractAPIConfig_Token(t *testing.T) {
	input := `
environment {
	api-server {
		token secret-123;
		rest {
			enabled true;
			server main { port 8091; }
		}
	}
}
`
	schema, err := YANGSchema()
	require.NoError(t, err)
	p := NewParser(schema)
	tree, err := p.Parse(input)
	require.NoError(t, err)

	cfg, ok := ExtractAPIConfig(tree)
	require.True(t, ok)
	assert.Equal(t, "secret-123", cfg.Token)
	require.Len(t, cfg.REST, 1)
	assert.Equal(t, "0.0.0.0", cfg.REST[0].Host) // YANG refine default
	assert.Equal(t, "8091", cfg.REST[0].Port)
}

// -----------------------------------------------------------------------------
// ExtractWebConfig -- Chunk 3
// -----------------------------------------------------------------------------

// TestExtractWebConfig_SingleServer verifies a single-entry YANG list maps
// to one ServerEndpoint.
//
// VALIDATES: Pre-Chunk-3 single-entry behavior is preserved.
// PREVENTS: Reshape from scalar to slice silently drops the original entry.
func TestExtractWebConfig_SingleServer(t *testing.T) {
	input := `
environment {
	web {
		enabled true;
		server main {
			ip 0.0.0.0;
			port 3443;
		}
	}
}
`
	schema, err := YANGSchema()
	require.NoError(t, err)
	p := NewParser(schema)
	tree, err := p.Parse(input)
	require.NoError(t, err)

	cfg, ok := ExtractWebConfig(tree)
	require.True(t, ok)
	require.Len(t, cfg.Servers, 1)
	assert.Equal(t, "0.0.0.0", cfg.Servers[0].Host)
	assert.Equal(t, "3443", cfg.Servers[0].Port)
	assert.Equal(t, "0.0.0.0:3443", cfg.Servers[0].Listen())
	assert.False(t, cfg.Insecure)
}

// TestExtractWebConfig_MultipleServers verifies every list entry is returned
// in insertion order.
//
// VALIDATES: AC-1 (web multi-bind) feeds the binder with every configured
// endpoint.
// PREVENTS: "first entry only" regression returning silently.
func TestExtractWebConfig_MultipleServers(t *testing.T) {
	input := `
environment {
	web {
		enabled true;
		server primary {
			ip 0.0.0.0;
			port 3443;
		}
		server admin {
			ip 127.0.0.1;
			port 13443;
		}
	}
}
`
	schema, err := YANGSchema()
	require.NoError(t, err)
	p := NewParser(schema)
	tree, err := p.Parse(input)
	require.NoError(t, err)

	cfg, ok := ExtractWebConfig(tree)
	require.True(t, ok)
	require.Len(t, cfg.Servers, 2)
	assert.Equal(t, "0.0.0.0", cfg.Servers[0].Host)
	assert.Equal(t, "3443", cfg.Servers[0].Port)
	assert.Equal(t, "127.0.0.1", cfg.Servers[1].Host)
	assert.Equal(t, "13443", cfg.Servers[1].Port)
}

// TestExtractWebConfig_EmptyListUsesDefaults verifies that enabling web
// without any server block produces one default entry from YANG refine.
//
// VALIDATES: Legacy "web { enabled true; }" configs keep working without
// requiring users to add an explicit server block.
// PREVENTS: Empty-list crash or silent no-listener.
func TestExtractWebConfig_EmptyListUsesDefaults(t *testing.T) {
	input := `
environment {
	web {
		enabled true;
	}
}
`
	schema, err := YANGSchema()
	require.NoError(t, err)
	p := NewParser(schema)
	tree, err := p.Parse(input)
	require.NoError(t, err)

	cfg, ok := ExtractWebConfig(tree)
	require.True(t, ok)
	require.Len(t, cfg.Servers, 1)
	assert.Equal(t, "0.0.0.0", cfg.Servers[0].Host)
	assert.Equal(t, "3443", cfg.Servers[0].Port)
}

// TestExtractWebConfig_InsecureForcesLoopback verifies insecure rewrites
// every entry's host to 127.0.0.1, not just the first.
//
// VALIDATES: AC-13 (insecure web applies per-entry).
// PREVENTS: Insecure flag bypassed on extra entries in multi-listener mode.
func TestExtractWebConfig_InsecureForcesLoopback(t *testing.T) {
	input := `
environment {
	web {
		enabled true;
		insecure true;
		server a {
			ip 0.0.0.0;
			port 3443;
		}
		server b {
			ip 192.0.2.1;
			port 13443;
		}
	}
}
`
	schema, err := YANGSchema()
	require.NoError(t, err)
	p := NewParser(schema)
	tree, err := p.Parse(input)
	require.NoError(t, err)

	cfg, ok := ExtractWebConfig(tree)
	require.True(t, ok)
	assert.True(t, cfg.Insecure)
	require.Len(t, cfg.Servers, 2)
	for i, s := range cfg.Servers {
		assert.Equal(t, "127.0.0.1", s.Host, "server[%d] host", i)
	}
}

// -----------------------------------------------------------------------------
// ExtractMCPConfig -- Chunk 3
// -----------------------------------------------------------------------------

// TestExtractMCPConfig_MultipleServers verifies MCP returns every list entry
// and forces each to 127.0.0.1.
//
// VALIDATES: AC-3 (MCP multi-bind with loopback enforcement on every entry).
// PREVENTS: Non-loopback entries silently retained on the secondary listeners.
func TestExtractMCPConfig_MultipleServers(t *testing.T) {
	input := `
environment {
	mcp {
		enabled true;
		token abc-123;
		server a {
			ip 127.0.0.1;
			port 8080;
		}
		server b {
			ip 192.0.2.1;
			port 18080;
		}
	}
}
`
	schema, err := YANGSchema()
	require.NoError(t, err)
	p := NewParser(schema)
	tree, err := p.Parse(input)
	require.NoError(t, err)

	cfg, ok := ExtractMCPConfig(tree)
	require.True(t, ok)
	assert.Equal(t, "abc-123", cfg.Token)
	require.Len(t, cfg.Servers, 2)
	assert.Equal(t, "127.0.0.1", cfg.Servers[0].Host)
	assert.Equal(t, "8080", cfg.Servers[0].Port)
	// Non-loopback entry is rewritten to 127.0.0.1.
	assert.Equal(t, "127.0.0.1", cfg.Servers[1].Host)
	assert.Equal(t, "18080", cfg.Servers[1].Port)
}

// -----------------------------------------------------------------------------
// ExtractLGConfig -- Chunk 3
// -----------------------------------------------------------------------------

// TestExtractLGConfig_MultipleServers verifies LG returns every list entry.
//
// VALIDATES: AC-2 (LG multi-bind).
// PREVENTS: "first entry only" regression.
func TestExtractLGConfig_MultipleServers(t *testing.T) {
	input := `
environment {
	looking-glass {
		enabled true;
		server v4 {
			ip 0.0.0.0;
			port 8443;
		}
		server v6 {
			ip ::1;
			port 8444;
			tls false;
		}
	}
}
`
	schema, err := YANGSchema()
	require.NoError(t, err)
	p := NewParser(schema)
	tree, err := p.Parse(input)
	require.NoError(t, err)

	cfg, ok := ExtractLGConfig(tree)
	require.True(t, ok)
	require.Len(t, cfg.Servers, 2)
	assert.Equal(t, LGServerTLSInherited, cfg.Servers[0].TLS, "an entry with no tls leaf inherits the default")
	assert.Equal(t, LGServerTLSOff, cfg.Servers[1].TLS, "tls false is read on the entry that wrote it")
	assert.Equal(t, "0.0.0.0", cfg.Servers[0].Host)
	assert.Equal(t, "8443", cfg.Servers[0].Port)
	assert.Equal(t, "::1", cfg.Servers[1].Host)
	assert.Equal(t, "8444", cfg.Servers[1].Port)
}

// TestExtractLGConfig_EmptyListUsesDefaults mirrors the web empty-list case.
func TestExtractLGConfig_EmptyListUsesDefaults(t *testing.T) {
	input := `
environment {
	looking-glass {
		enabled true;
	}
}
`
	schema, err := YANGSchema()
	require.NoError(t, err)
	p := NewParser(schema)
	tree, err := p.Parse(input)
	require.NoError(t, err)

	cfg, ok := ExtractLGConfig(tree)
	require.True(t, ok)
	require.Len(t, cfg.Servers, 1)
	assert.Equal(t, "0.0.0.0", cfg.Servers[0].Host)
	assert.Equal(t, "8443", cfg.Servers[0].Port)
	assert.Equal(t, LGServerTLSInherited, cfg.Servers[0].TLS, "the default endpoint has no tls leaf")
}

// -----------------------------------------------------------------------------
// MCPListenConfig.Validate -- Phase 2 (AC-6, AC-6a, AC-6b, plus extended checks)
// -----------------------------------------------------------------------------

// TestMCPConfigValidate exercises the exact-or-reject gates applied at
// `ze config verify` time.
//
// VALIDATES: Phase 2 AC-6/6a + bearer-list / oauth sanity. AC-6b (oauth on a
// remote listener without TLS) is an exposure, answered by Refusals and
// asserted in TestMCPListenConfigRefusalsRequireTLSOffLoopback.
// PREVENTS: Silent acceptance of internally inconsistent configs.
func TestMCPConfigValidate(t *testing.T) {
	loopback := []ServerEndpoint{{Host: "127.0.0.1", Port: "8080"}}
	remote := []ServerEndpoint{{Host: "192.0.2.1", Port: "8080"}}

	cases := []struct {
		name    string
		cfg     MCPListenConfig
		wantErr string // empty = expect nil error; non-empty = substring match
	}{
		{
			name:    "empty config ok",
			cfg:     MCPListenConfig{Servers: loopback},
			wantErr: "",
		},
		{
			name:    "auth-mode none ok on loopback",
			cfg:     MCPListenConfig{Servers: loopback, AuthMode: "none"},
			wantErr: "",
		},
		{
			name:    "AC-6a bind-remote without auth rejects",
			cfg:     MCPListenConfig{Servers: remote, BindRemote: true, AuthMode: "none"},
			wantErr: "bind-remote requires auth-mode != none",
		},
		{
			name:    "AC-6a bind-remote with empty auth-mode rejects",
			cfg:     MCPListenConfig{Servers: remote, BindRemote: true},
			wantErr: "bind-remote requires auth-mode != none",
		},
		{
			name:    "AC-6 oauth without authorization-server rejects",
			cfg:     MCPListenConfig{Servers: loopback, AuthMode: "oauth"},
			wantErr: "requires oauth.authorization-server",
		},
		{
			name: "AC-6 oauth without audience rejects",
			cfg: MCPListenConfig{
				Servers:  loopback,
				AuthMode: "oauth",
				OAuth:    MCPOAuthConfig{AuthorizationServer: "https://as.example/"},
			},
			wantErr: "requires oauth.audience",
		},
		{
			name: "oauth on loopback without TLS ok",
			cfg: MCPListenConfig{
				Servers:  loopback,
				AuthMode: "oauth",
				OAuth: MCPOAuthConfig{
					AuthorizationServer: "https://as.example/",
					Audience:            "https://mcp.example/",
				},
			},
			wantErr: "",
		},
		{
			name: "oauth remote with TLS ok",
			cfg: MCPListenConfig{
				Servers:    remote,
				BindRemote: true,
				AuthMode:   "oauth",
				OAuth: MCPOAuthConfig{
					AuthorizationServer: "https://as.example/",
					Audience:            "https://mcp.example/",
				},
				TLS: MCPTLSConfig{Cert: "/etc/ze/cert.pem", Key: "/etc/ze/key.pem"},
			},
			wantErr: "",
		},
		{
			name: "tls cert without key rejects",
			cfg: MCPListenConfig{
				Servers:  loopback,
				AuthMode: "oauth",
				OAuth: MCPOAuthConfig{
					AuthorizationServer: "https://as.example/",
					Audience:            "https://mcp.example/",
				},
				TLS: MCPTLSConfig{Cert: "/etc/ze/cert.pem"},
			},
			wantErr: "cert set without key",
		},
		{
			name:    "bearer without token rejects",
			cfg:     MCPListenConfig{Servers: loopback, AuthMode: "bearer"},
			wantErr: "auth-mode=bearer requires token",
		},
		{
			name:    "bearer with token ok",
			cfg:     MCPListenConfig{Servers: loopback, AuthMode: "bearer", Token: "secret"},
			wantErr: "",
		},
		{
			name:    "bearer-list empty rejects",
			cfg:     MCPListenConfig{Servers: loopback, AuthMode: "bearer-list"},
			wantErr: "requires at least one identity",
		},
		{
			name: "bearer-list missing token rejects",
			cfg: MCPListenConfig{
				Servers:    loopback,
				AuthMode:   "bearer-list",
				Identities: []MCPIdentity{{Name: "alice"}},
			},
			wantErr: "token is required",
		},
		{
			name: "bearer-list duplicate name rejects",
			cfg: MCPListenConfig{
				Servers:  loopback,
				AuthMode: "bearer-list",
				Identities: []MCPIdentity{
					{Name: "alice", Token: "t1"},
					{Name: "alice", Token: "t2"},
				},
			},
			wantErr: "duplicate name",
		},
		{
			name: "bearer-list duplicate token rejects",
			cfg: MCPListenConfig{
				Servers:  loopback,
				AuthMode: "bearer-list",
				Identities: []MCPIdentity{
					{Name: "alice", Token: "shared"},
					{Name: "bob", Token: "shared"},
				},
			},
			wantErr: "shared with another identity",
		},
		{
			name: "bearer-list ok",
			cfg: MCPListenConfig{
				Servers:  loopback,
				AuthMode: "bearer-list",
				Identities: []MCPIdentity{
					{Name: "alice", Token: "t1"},
					{Name: "bob", Token: "t2"},
				},
			},
			wantErr: "",
		},
		{
			name:    "unknown auth-mode rejects",
			cfg:     MCPListenConfig{Servers: loopback, AuthMode: "bogus"},
			wantErr: "unknown value",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if tc.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

// The offline validator must reject identifiers discovery cannot use, while a
// query remains valid on the protected resource identifier.
func TestMCPOAuthIdentifierValidation(t *testing.T) {
	for _, tc := range []struct {
		name     string
		issuer   string
		audience string
		valid    bool
	}{
		{"HTTPS", "https://as.example/realm", "https://mcp.example/mcp", true},
		{"resource query", "HTTPS://AS.EXAMPLE/", "https://mcp.example/mcp?tenant=A", true},
		{"HTTP issuer", "http://as.example/", "https://mcp.example/mcp", false},
		{"issuer query", "https://as.example/?tenant=A", "https://mcp.example/mcp", false},
		{"empty issuer query", "https://as.example/?", "https://mcp.example/mcp", false},
		{"issuer userinfo", "https://secret:password@as.example/", "https://mcp.example/mcp", false},
		{"issuer fragment", "https://as.example/#", "https://mcp.example/mcp", false},
		{"issuer missing host", "https:///realm", "https://mcp.example/mcp", false},
		{"HTTP resource", "https://as.example/", "http://mcp.example/mcp", false},
		{"resource userinfo", "https://as.example/", "https://secret:password@mcp.example/mcp", false},
		{"resource fragment", "https://as.example/", "https://mcp.example/mcp#", false},
		{"malformed resource", "https://as.example/", "https://mcp.example/%secret", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := MCPListenConfig{
				Servers:  []ServerEndpoint{{Host: "127.0.0.1", Port: "8080"}},
				AuthMode: "oauth",
				OAuth: MCPOAuthConfig{
					AuthorizationServer: tc.issuer,
					Audience:            tc.audience,
				},
			}
			err := cfg.Validate()
			if tc.valid {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.NotContains(t, err.Error(), "secret")
			assert.NotContains(t, err.Error(), "password")
		})
	}
}

// A partial certificate pair must never silently select plaintext for a
// non-OAuth listener.
func TestMCPTLSPairValidationAllModes(t *testing.T) {
	for _, mode := range []string{"none", "bearer", "bearer-list"} {
		t.Run(mode, func(t *testing.T) {
			cfg := MCPListenConfig{
				Servers:    []ServerEndpoint{{Host: "127.0.0.1", Port: "8080"}},
				AuthMode:   mode,
				Token:      "token",
				Identities: []MCPIdentity{{Name: "alice", Token: "alice-token"}},
			}
			cfg.TLS = MCPTLSConfig{Cert: "cert.pem"}
			require.ErrorContains(t, cfg.Validate(), "cert set without key")
			cfg.TLS = MCPTLSConfig{Key: "key.pem"}
			require.ErrorContains(t, cfg.Validate(), "key set without cert")
			cfg.TLS = MCPTLSConfig{Cert: "cert.pem", Key: "key.pem"}
			require.NoError(t, cfg.Validate())
		})
	}
}

// TestMCPConfigValidate_RemoteBearerRequiresTLS verifies that Validate refuses
// a remote bearer, bearer-list or oauth listener that names only one of its two
// TLS files, while loopback listeners remain valid without TLS and remote
// listeners with both TLS files are accepted. A remote listener with no TLS at
// all is an exposure, answered by Refusals and asserted in
// TestMCPListenConfigRefusalsRequireTLSOffLoopback.
//
// VALIDATES: a half-named TLS pair never passes offline validation, so static
// credentials are never served over a listener that only appears to have TLS.
// PREVENTS: bearer / bearer-list / oauth remote configs with a half TLS pair
// passing offline validation.
func TestMCPConfigValidate_RemoteBearerRequiresTLS(t *testing.T) {
	loopback := []ServerEndpoint{{Host: "127.0.0.1", Port: "8080"}}
	remote := []ServerEndpoint{{Host: "192.0.2.1", Port: "8443"}}
	tlsPair := MCPTLSConfig{Cert: "/etc/ze/mcp.pem", Key: "/etc/ze/mcp.key"}
	identities := []MCPIdentity{{Name: "alice", Token: "alice-secret"}}

	cases := []struct {
		name    string
		cfg     MCPListenConfig
		wantErr string
	}{
		{
			name:    "bearer remote with cert only rejects",
			cfg:     MCPListenConfig{Servers: remote, BindRemote: true, AuthMode: "bearer", Token: "secret", TLS: MCPTLSConfig{Cert: "/etc/ze/mcp.pem"}},
			wantErr: "cert set without key",
		},
		{
			name:    "bearer remote with key only rejects",
			cfg:     MCPListenConfig{Servers: remote, BindRemote: true, AuthMode: "bearer", Token: "secret", TLS: MCPTLSConfig{Key: "/etc/ze/mcp.key"}},
			wantErr: "key set without cert",
		},
		{
			name:    "bearer loopback without TLS ok",
			cfg:     MCPListenConfig{Servers: loopback, AuthMode: "bearer", Token: "secret"},
			wantErr: "",
		},
		{
			name:    "bearer remote with TLS ok",
			cfg:     MCPListenConfig{Servers: remote, BindRemote: true, AuthMode: "bearer", Token: "secret", TLS: tlsPair},
			wantErr: "",
		},
		{
			name:    "bearer-list remote with cert only rejects",
			cfg:     MCPListenConfig{Servers: remote, BindRemote: true, AuthMode: "bearer-list", Identities: identities, TLS: MCPTLSConfig{Cert: "/etc/ze/mcp.pem"}},
			wantErr: "cert set without key",
		},
		{
			name:    "bearer-list remote with key only rejects",
			cfg:     MCPListenConfig{Servers: remote, BindRemote: true, AuthMode: "bearer-list", Identities: identities, TLS: MCPTLSConfig{Key: "/etc/ze/mcp.key"}},
			wantErr: "key set without cert",
		},
		{
			name:    "bearer-list loopback without TLS ok",
			cfg:     MCPListenConfig{Servers: loopback, AuthMode: "bearer-list", Identities: identities},
			wantErr: "",
		},
		{
			name:    "bearer-list remote with TLS ok",
			cfg:     MCPListenConfig{Servers: remote, BindRemote: true, AuthMode: "bearer-list", Identities: identities, TLS: tlsPair},
			wantErr: "",
		},
		{
			name: "oauth remote with cert only rejects",
			cfg: MCPListenConfig{
				Servers:    remote,
				BindRemote: true,
				AuthMode:   "oauth",
				OAuth: MCPOAuthConfig{
					AuthorizationServer: "https://as.example/",
					Audience:            "https://mcp.example/",
				},
				TLS: MCPTLSConfig{Cert: "/etc/ze/mcp.pem"},
			},
			wantErr: "cert set without key",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if tc.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

// TestMCPAuthModesMatchTheModel ties the auth modes Validate enforces to the
// enumeration the model offers an operator.
//
// The Go side is the declaration here, and deliberately so: each mode carries
// its own requirement (bearer needs a token, bearer-list needs identities,
// oauth needs an authorization server), so a mode Validate merely accepted
// would be an authenticated listener with nothing to check. Deriving the
// membership test from the model would accept exactly that. The model is the
// copy, and this test is what keeps the copy honest.
//
// VALIDATES: the set the model declares at environment/mcp/auth-mode and the
// set Validate recognizes are the same set.
// PREVENTS: a mode added to the YANG that Validate refuses as unknown, and a
// mode added here that no operator can reach.
func TestMCPAuthModesMatchTheModel(t *testing.T) {
	declared, err := configyang.EnumValues("environment/mcp/auth-mode")
	require.NoError(t, err)
	require.NotEmpty(t, declared, "the model declares no MCP auth mode")

	// The constants, never their spellings: the test reads what the code
	// enforces rather than stating it a second time.
	enforced := []string{mcpAuthNone, mcpAuthBearer, mcpAuthBearerList, mcpAuthOAuth}

	loopback := []ServerEndpoint{{Host: "127.0.0.1", Port: "8080"}}
	for _, mode := range declared {
		cfg := MCPListenConfig{Servers: loopback, AuthMode: mode}
		err := cfg.Validate()
		if err != nil {
			assert.NotContains(t, err.Error(), "unknown value",
				"the model declares auth-mode %q, so Validate must know it", mode)
		}
		assert.Contains(t, enforced, mode,
			"the model declares auth-mode %q, which carries no requirement of its own here", mode)
	}

	for _, mode := range enforced {
		assert.Contains(t, declared, mode,
			"auth-mode %q is enforced here and no operator can reach it: the model does not declare it", mode)
	}
}

// TestExtractMCPConfig_BindRemotePreservesHost verifies the loopback clamp is
// suppressed when bind-remote is true.
//
// VALIDATES: Phase 2 extract path (bind-remote lifts the Phase-1 clamp).
// PREVENTS: Silent override of the operator's configured ip.
func TestExtractMCPConfig_BindRemotePreservesHost(t *testing.T) {
	input := `
environment {
	mcp {
		enabled true;
		bind-remote true;
		auth-mode bearer;
		token secret-1234;
		server remote {
			ip 192.0.2.1;
			port 8443;
		}
	}
}
`
	schema, err := YANGSchema()
	require.NoError(t, err)
	p := NewParser(schema)
	tree, err := p.Parse(input)
	require.NoError(t, err)

	cfg, ok := ExtractMCPConfig(tree)
	require.True(t, ok)
	assert.True(t, cfg.BindRemote)
	assert.Equal(t, "bearer", cfg.AuthMode)
	assert.Equal(t, "secret-1234", cfg.Token)
	require.Len(t, cfg.Servers, 1)
	assert.Equal(t, "192.0.2.1", cfg.Servers[0].Host)
	assert.Equal(t, "8443", cfg.Servers[0].Port)
}

// TestExtractMCPConfig_TokenInfersBearer verifies a legacy config with token
// set but no auth-mode still works: AuthMode is inferred as "bearer" so the
// runtime dispatcher routes through the single-token path.
//
// VALIDATES: Existing (pre-Phase-2) configs do not break on upgrade.
// PREVENTS: Silent auth downgrade when auth-mode is absent but token is set.
func TestExtractMCPConfig_TokenInfersBearer(t *testing.T) {
	input := `
environment {
	mcp {
		enabled true;
		token secret-legacy;
		server main {
			ip 127.0.0.1;
			port 8080;
		}
	}
}
`
	schema, err := YANGSchema()
	require.NoError(t, err)
	p := NewParser(schema)
	tree, err := p.Parse(input)
	require.NoError(t, err)

	cfg, ok := ExtractMCPConfig(tree)
	require.True(t, ok)
	assert.Equal(t, "bearer", cfg.AuthMode, "token set without auth-mode should infer bearer")
	assert.Equal(t, "secret-legacy", cfg.Token)
}

// TestExtractMCPConfig_IdentityList verifies the identity[] list is extracted
// with name + token + scope leaf-list.
//
// VALIDATES: Phase 2 bearer-list extraction (drives AC-10/AC-11 later).
// PREVENTS: Silent loss of scopes or identity entries.
func TestExtractMCPConfig_IdentityList(t *testing.T) {
	input := `
environment {
	mcp {
		enabled true;
		auth-mode bearer-list;
		identity alice {
			token alice-secret;
			scope [ mcp.read mcp.write ];
		}
		identity bob {
			token bob-secret;
		}
		server main {
			ip 127.0.0.1;
			port 8080;
		}
	}
}
`
	schema, err := YANGSchema()
	require.NoError(t, err)
	p := NewParser(schema)
	tree, err := p.Parse(input)
	require.NoError(t, err)

	cfg, ok := ExtractMCPConfig(tree)
	require.True(t, ok)
	require.Len(t, cfg.Identities, 2)
	assert.Equal(t, "alice", cfg.Identities[0].Name)
	assert.Equal(t, "alice-secret", cfg.Identities[0].Token)
	assert.Equal(t, []string{"mcp.read", "mcp.write"}, cfg.Identities[0].Scopes)
	assert.Equal(t, "bob", cfg.Identities[1].Name)
	assert.Equal(t, "bob-secret", cfg.Identities[1].Token)
	assert.Empty(t, cfg.Identities[1].Scopes)
}

// TestExtractMCPConfig_OAuthAndTLS verifies the oauth and tls containers are
// extracted into the config struct.
//
// VALIDATES: Phase 2 oauth / tls extraction.
// PREVENTS: Silent loss of required-scopes leaf-list entries.
func TestExtractMCPConfig_OAuthAndTLS(t *testing.T) {
	input := `
environment {
	mcp {
		enabled true;
		bind-remote true;
		auth-mode oauth;
		oauth {
			authorization-server https://as.example/;
			audience https://mcp.example/;
			required-scopes [ mcp.admin mcp.read ];
		}
		tls {
			cert /etc/ze/mcp.pem;
			key /etc/ze/mcp.key;
		}
		server main {
			ip 192.0.2.1;
			port 443;
		}
	}
}
`
	schema, err := YANGSchema()
	require.NoError(t, err)
	p := NewParser(schema)
	tree, err := p.Parse(input)
	require.NoError(t, err)

	cfg, ok := ExtractMCPConfig(tree)
	require.True(t, ok)
	assert.Equal(t, "oauth", cfg.AuthMode)
	assert.True(t, cfg.BindRemote)
	assert.Equal(t, "https://as.example/", cfg.OAuth.AuthorizationServer)
	assert.Equal(t, "https://mcp.example/", cfg.OAuth.Audience)
	assert.Equal(t, []string{"mcp.admin", "mcp.read"}, cfg.OAuth.RequiredScopes)
	assert.Equal(t, "/etc/ze/mcp.pem", cfg.TLS.Cert)
	assert.Equal(t, "/etc/ze/mcp.key", cfg.TLS.Key)
	require.NoError(t, cfg.Validate())
}

// TestExtractMCPConfig_AnyListenerNonLoopback exercises the helper used by
// Validate to decide whether TLS is required for oauth.
func TestExtractMCPConfig_AnyListenerNonLoopback(t *testing.T) {
	cases := []struct {
		name string
		cfg  MCPListenConfig
		want bool
	}{
		{"only loopback", MCPListenConfig{Servers: []ServerEndpoint{{Host: "127.0.0.1", Port: "1"}}}, false},
		{"only ipv6 loopback", MCPListenConfig{Servers: []ServerEndpoint{{Host: "::1", Port: "1"}}}, false},
		{"mixed", MCPListenConfig{Servers: []ServerEndpoint{{Host: "127.0.0.1", Port: "1"}, {Host: "192.0.2.1", Port: "1"}}}, true},
		{"only remote", MCPListenConfig{Servers: []ServerEndpoint{{Host: "0.0.0.0", Port: "1"}}}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.cfg.AnyListenerNonLoopback())
		})
	}
}

func TestExtractGNMIConfig_FullConfig(t *testing.T) {
	input := `
environment {
	gnmi {
		enabled true;
		token secret-token;
		tls {
			cert /etc/ze/gnmi.crt;
			key /etc/ze/gnmi.key;
		}
		server default {
			ip 0.0.0.0;
			port 9339;
		}
	}
}
`
	schema, err := YANGSchema()
	require.NoError(t, err)
	p := NewParser(schema)
	tree, err := p.Parse(input)
	require.NoError(t, err)

	cfg, ok := ExtractGNMIConfig(tree)
	require.True(t, ok)
	assert.Equal(t, "secret-token", cfg.Token)
	assert.Equal(t, "/etc/ze/gnmi.crt", cfg.TLS.Cert)
	assert.Equal(t, "/etc/ze/gnmi.key", cfg.TLS.Key)
	require.Len(t, cfg.Servers, 1)
	assert.Equal(t, "0.0.0.0", cfg.Servers[0].Host)
	assert.Equal(t, "9339", cfg.Servers[0].Port)
}

func TestExtractGNMIConfig_Disabled(t *testing.T) {
	input := `
environment {
	gnmi {
		token secret-token;
	}
}
`
	schema, err := YANGSchema()
	require.NoError(t, err)
	p := NewParser(schema)
	tree, err := p.Parse(input)
	require.NoError(t, err)

	_, ok := ExtractGNMIConfig(tree)
	assert.False(t, ok)
}

func TestExtractGNMIConfig_EmptyListUsesDefaults(t *testing.T) {
	input := `
environment {
	gnmi {
		enabled true;
	}
}
`
	schema, err := YANGSchema()
	require.NoError(t, err)
	p := NewParser(schema)
	tree, err := p.Parse(input)
	require.NoError(t, err)

	cfg, ok := ExtractGNMIConfig(tree)
	require.True(t, ok)
	require.Len(t, cfg.Servers, 1)
	assert.Equal(t, "0.0.0.0", cfg.Servers[0].Host)
	assert.Equal(t, "9339", cfg.Servers[0].Port)
}

func TestExtractGNMIConfig_NilTree(t *testing.T) {
	_, ok := ExtractGNMIConfig(nil)
	assert.False(t, ok)
}

// -----------------------------------------------------------------------------
// ExtractMCPSettings -- auth settings apply regardless of `enabled`
// -----------------------------------------------------------------------------

// parseTreeForTest parses config text into a Tree, failing the test on error.
func parseTreeForTest(t *testing.T, input string) *Tree {
	t.Helper()
	schema, err := YANGSchema()
	require.NoError(t, err)
	tree, err := NewParser(schema).Parse(input)
	require.NoError(t, err)
	return tree
}

// VALIDATES: ExtractMCPSettings returns the operator's auth settings for a
// block that neither sets `enabled true` nor names a port, while
// ExtractMCPConfig still reports ok=false for the same block.
// PREVENTS: the fail-open where `ze --mcp <port>` with `auth-mode bearer`
// configured produced an ACCEPT-ALL listener, because ExtractMCPConfig returned
// ok=false and every caller discarded auth-mode, token, identities and oauth
// with it. `enabled` answers "does config start a listener". It must not also
// decide whether the operator's authentication instruction applies.
func TestExtractMCPSettings_AppliesWithoutEnabledOrPort(t *testing.T) {
	tree := parseTreeForTest(t, `
environment {
	mcp {
		auth-mode bearer;
		token secret-1234;
	}
}
`)
	// Config does not ask for a listener: no `enabled`, no server/port.
	_, listenOK := ExtractMCPConfig(tree)
	assert.False(t, listenOK, "ExtractMCPConfig must still refuse to start a listener")

	// But the settings are the operator's instruction and must survive.
	cfg, ok := ExtractMCPSettings(tree)
	require.True(t, ok, "settings must be returned whenever the mcp block exists")
	assert.Equal(t, "bearer", cfg.AuthMode)
	assert.Equal(t, "secret-1234", cfg.Token)
}

// VALIDATES: the same for bearer-list, including the identity table, and for
// oauth, so no auth mode is silently downgraded to accept-all.
func TestExtractMCPSettings_AllAuthModesSurvive(t *testing.T) {
	t.Run("bearer-list", func(t *testing.T) {
		tree := parseTreeForTest(t, `
environment {
	mcp {
		auth-mode bearer-list;
		identity alice {
			token alice-token;
			scope read;
		}
		identity bob {
			token bob-token;
		}
	}
}
`)
		cfg, ok := ExtractMCPSettings(tree)
		require.True(t, ok)
		assert.Equal(t, "bearer-list", cfg.AuthMode)
		require.Len(t, cfg.Identities, 2, "the identity table must survive; without it bearer-list authenticates nobody")
	})

	t.Run("oauth", func(t *testing.T) {
		tree := parseTreeForTest(t, `
environment {
	mcp {
		auth-mode oauth;
		oauth {
			authorization-server https://as.example/;
			audience https://mcp.example/;
		}
	}
}
`)
		cfg, ok := ExtractMCPSettings(tree)
		require.True(t, ok)
		assert.Equal(t, "oauth", cfg.AuthMode)
		assert.Equal(t, "https://as.example/", cfg.OAuth.AuthorizationServer)
		assert.Equal(t, "https://mcp.example/", cfg.OAuth.Audience)
	})
}

// VALIDATES: an absent mcp block yields ok=false from both extractors, so the
// split did not turn "no config at all" into a zero-value config that reads as
// an instruction (ai/rules/evidence.md).
func TestExtractMCPSettings_AbsentBlockIsNotAnInstruction(t *testing.T) {
	tree := parseTreeForTest(t, "environment {\n\tlog {\n\t\tlevel warn;\n\t}\n}\n")
	_, ok := ExtractMCPSettings(tree)
	assert.False(t, ok)
	_, listenOK := ExtractMCPConfig(tree)
	assert.False(t, listenOK)
}

// VALIDATES: the normal path is unchanged -- a fully enabled block with a port
// still returns ok=true from ExtractMCPConfig with its servers and settings.
// PREVENTS: the split regressing the case that already worked.
func TestExtractMCPConfig_EnabledPathUnchanged(t *testing.T) {
	tree := parseTreeForTest(t, `
environment {
	mcp {
		enabled true;
		auth-mode bearer;
		token secret-1234;
		server a {
			ip 127.0.0.1;
			port 8080;
		}
	}
}
`)
	cfg, ok := ExtractMCPConfig(tree)
	require.True(t, ok)
	assert.Equal(t, "bearer", cfg.AuthMode)
	assert.Equal(t, "secret-1234", cfg.Token)
	require.Len(t, cfg.Servers, 1)
	assert.Equal(t, "8080", cfg.Servers[0].Port)
}
