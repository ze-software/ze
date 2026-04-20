package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	// Blank imports trigger init() registration of service YANG modules so
	// YANGSchema() picks up environment/{api-server,web,mcp,looking-glass}
	// used by these tests.
	_ "codeberg.org/thomas-mangin/ze/internal/component/api/schema"
	_ "codeberg.org/thomas-mangin/ze/internal/component/lg/schema"
	_ "codeberg.org/thomas-mangin/ze/internal/component/mcp/schema"
	_ "codeberg.org/thomas-mangin/ze/internal/component/web/schema"
)

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
		tls true;
		server v4 {
			ip 0.0.0.0;
			port 8443;
		}
		server v6 {
			ip ::1;
			port 8444;
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
	assert.True(t, cfg.TLS)
	require.Len(t, cfg.Servers, 2)
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
}

// -----------------------------------------------------------------------------
// MCPListenConfig.Validate -- Phase 2 (AC-6, AC-6a, AC-6b, plus extended checks)
// -----------------------------------------------------------------------------

// TestMCPConfigValidate exercises the exact-or-reject gates applied at
// `ze config verify` time.
//
// VALIDATES: Phase 2 AC-6/6a/6b + bearer-list / oauth sanity.
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
			name: "AC-6b oauth on remote without TLS rejects",
			cfg: MCPListenConfig{
				Servers:    remote,
				BindRemote: true,
				AuthMode:   "oauth",
				OAuth: MCPOAuthConfig{
					AuthorizationServer: "https://as.example/",
					Audience:            "https://mcp.example/",
				},
			},
			wantErr: "requires tls.cert and tls.key on non-loopback listeners",
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
