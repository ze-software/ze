package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	// Blank import triggers init() registration of ze-api-conf.yang so
	// YANGSchema() picks up environment/api-server used by these tests.
	_ "codeberg.org/thomas-mangin/ze/internal/component/api/schema"
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
