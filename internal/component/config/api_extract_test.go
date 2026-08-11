package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// parseAPITree parses a config source through the real YANG schema, the same
// way the daemon reaches these extractors.
func parseAPITree(t *testing.T, input string) *Tree {
	t.Helper()
	schema, err := YANGSchema()
	require.NoError(t, err)
	tree, err := NewParser(schema).Parse(input)
	require.NoError(t, err)
	return tree
}

// VALIDATES: an environment.api-server block whose transports do NOT say
// `enabled true` still yields its token, its listen addresses and the gRPC TLS
// pair. ze.api-server.rest.enabled and ze.api-server.grpc.enabled start a
// transport without that leaf, so gating the settings on it threw the
// operator's instruction away.
// PREVENTS: three failures at once. The dropped token made the daemon refuse to
// boot naming the very token the operator had written; the dropped address bound
// the 0.0.0.0 default over a loopback address; the dropped TLS pair served
// management gRPC in clear while that token crossed the wire. Same defect as
// environment.mcp, environment.looking-glass and environment.gnmi
// (plan/journal/enabled-gate-discards-settings.md).
func TestExtractAPISettingsSurviveDisabledTransports(t *testing.T) {
	tree := parseAPITree(t, `
environment {
	api-server {
		token api-s3cret;
		rest {
			server main {
				ip 127.0.0.1;
				port 18095;
			}
		}
		grpc {
			tls-cert /etc/ze/grpc.pem;
			tls-key /etc/ze/grpc.key;
			server main {
				ip 127.0.0.1;
				port 50052;
			}
		}
	}
}
`)

	// The listener question still answers "no": config must not start either
	// transport.
	_, ok := ExtractAPIConfig(tree)
	assert.False(t, ok, "a block with no enabled leaf must not ask for a listener")

	// The settings question answers "here they are".
	cfg, ok := ExtractAPISettings(tree)
	require.True(t, ok, "settings must be available for any api-server block")
	assert.Equal(t, "api-s3cret", cfg.Token, "the token must survive the enable gate")
	assert.False(t, cfg.RESTOn, "settings must not turn a dormant transport into a listener")
	assert.False(t, cfg.GRPCOn)
	require.Len(t, cfg.REST, 1)
	assert.Equal(t, "127.0.0.1", cfg.REST[0].Host)
	assert.Equal(t, "18095", cfg.REST[0].Port)
	require.Len(t, cfg.GRPC, 1)
	assert.Equal(t, "127.0.0.1", cfg.GRPC[0].Host)
	assert.Equal(t, "/etc/ze/grpc.pem", cfg.GRPCTLSCert, "the gRPC TLS pair must survive the enable gate")
	assert.Equal(t, "/etc/ze/grpc.key", cfg.GRPCTLSKey)
}

// VALIDATES: no block means no settings, and a caller must not read a
// zero-value config as an operator instruction.
func TestExtractAPISettingsAbsentBlock(t *testing.T) {
	_, ok := ExtractAPISettings(NewTree())
	assert.False(t, ok, "a tree with no api-server block has no settings")
	_, ok = ExtractAPISettings(nil)
	assert.False(t, ok, "a nil tree has no settings")
}

// VALIDATES: the split changes only WHICH question each function answers.
// ExtractAPIConfig keeps meaning "config asks for an API listener", and the
// per-transport RESTOn/GRPCOn flags stay the only answer to which one binds.
func TestExtractAPIConfigStillGatesOnEnabled(t *testing.T) {
	tree := parseAPITree(t, `
environment {
	api-server {
		rest {
			enabled true;
			server main {
				ip 127.0.0.1;
				port 18095;
			}
		}
		grpc {
			server main {
				ip 0.0.0.0;
				port 50052;
			}
		}
	}
}
`)

	cfg, ok := ExtractAPIConfig(tree)
	require.True(t, ok, "one enabled transport asks for a listener")
	assert.True(t, cfg.RESTOn)
	assert.False(t, cfg.GRPCOn, "a transport with no enabled leaf must not bind")
	require.Len(t, cfg.REST, 1)
	assert.Equal(t, "127.0.0.1:18095", cfg.REST[0].Listen())
}
