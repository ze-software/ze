package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestApplyDefaults_LeafInserted verifies that missing leaf defaults are inserted.
//
// VALIDATES: YANG defaults are applied to config maps.
// PREVENTS: Missing RFC defaults when user doesn't configure a value.
func TestApplyDefaults_LeafInserted(t *testing.T) {
	schema := NewSchema()
	schema.Define("hold-time", LeafWithDefault(TypeUint16, "90"))
	schema.Define("port", LeafWithDefault(TypeUint16, "179"))
	schema.Define("name", Leaf(TypeString)) // no default

	m := map[string]any{
		"hold-time": "45", // explicitly set, should not be overwritten
	}

	ApplyDefaults(m, schema.root)

	assert.Equal(t, "45", m["hold-time"], "explicit value must not be overwritten")
	assert.Equal(t, "179", m["port"], "missing leaf with default must be inserted")
	_, hasName := m["name"]
	assert.False(t, hasName, "leaf without default must not be inserted")
}

// TestApplyDefaults_NonPresenceContainer verifies that non-presence containers
// are created to hold child defaults.
//
// VALIDATES: Timer container with RFC defaults is created even when not configured.
// PREVENTS: Missing hold-time/connect-retry defaults.
func TestApplyDefaults_NonPresenceContainer(t *testing.T) {
	timer := Container(
		Field("receive-hold-time", LeafWithDefault(TypeUint16, "90")),
		Field("connect-retry", LeafWithDefault(TypeUint16, "120")),
	)
	schema := NewSchema()
	schema.Define("timer", timer)

	m := map[string]any{}

	ApplyDefaults(m, schema.root)

	timerMap, ok := m["timer"].(map[string]any)
	require.True(t, ok, "timer container must be created")
	assert.Equal(t, "90", timerMap["receive-hold-time"])
	assert.Equal(t, "120", timerMap["connect-retry"])
}

// TestApplyDefaults_PresenceContainerSkipped verifies that presence containers
// are not created when absent.
//
// VALIDATES: Presence containers (route-refresh, add-path) only get defaults if configured.
// PREVENTS: Accidentally enabling features the user didn't configure.
func TestApplyDefaults_PresenceContainerSkipped(t *testing.T) {
	addPath := Container(
		Field("send", LeafWithDefault(TypeBool, "false")),
		Field("receive", LeafWithDefault(TypeBool, "false")),
	)
	addPath.Presence = true

	schema := NewSchema()
	schema.Define("add-path", addPath)

	m := map[string]any{}

	ApplyDefaults(m, schema.root)

	_, hasAddPath := m["add-path"]
	assert.False(t, hasAddPath, "presence container must not be created when absent")
}

// TestApplyDefaults_PresenceContainerExisting verifies that defaults are applied
// inside an existing presence container.
//
// VALIDATES: Defaults fill in missing leaves when the container is present.
// PREVENTS: Missing default values inside explicitly configured presence containers.
func TestApplyDefaults_PresenceContainerExisting(t *testing.T) {
	addPath := Container(
		Field("send", LeafWithDefault(TypeBool, "false")),
		Field("receive", LeafWithDefault(TypeBool, "false")),
	)
	addPath.Presence = true

	schema := NewSchema()
	schema.Define("add-path", addPath)

	m := map[string]any{
		"add-path": map[string]any{
			"send": "true", // explicitly set
		},
	}

	ApplyDefaults(m, schema.root)

	ap, ok := m["add-path"].(map[string]any)
	require.True(t, ok, "add-path must be a map")
	assert.Equal(t, "true", ap["send"], "explicit value must not be overwritten")
	assert.Equal(t, "false", ap["receive"], "missing default must be inserted")
}

// TestApplyDefaults_NestedContainer verifies defaults in nested non-presence containers.
//
// VALIDATES: Deeply nested defaults are applied correctly.
// PREVENTS: Defaults only applied at the top level.
func TestApplyDefaults_NestedContainer(t *testing.T) {
	schema := NewSchema()
	schema.Define("capability", Container(
		Field("asn4", LeafWithDefault(TypeBool, "true")),
	))

	m := map[string]any{}

	ApplyDefaults(m, schema.root)

	cap, ok := m["capability"].(map[string]any)
	require.True(t, ok, "capability container must be created")
	assert.Equal(t, "true", cap["asn4"])
}

// TestSchemaDefault_Lookup verifies SchemaDefault reads from a real path.
//
// VALIDATES: SchemaDefault navigates dot-separated paths correctly.
// PREVENTS: Wrong path navigation returning empty defaults.
func TestSchemaDefault_Lookup(t *testing.T) {
	schema := NewSchema()
	schema.Define("timer", Container(
		Field("hold-time", LeafWithDefault(TypeUint16, "90")),
	))

	assert.Equal(t, "90", SchemaDefault(schema, "timer.hold-time"))
	assert.Equal(t, "", SchemaDefault(schema, "timer.nonexistent"))
	assert.Equal(t, "", SchemaDefault(schema, "nonexistent"))
}

// TestSchemaDefaultInt_Parsing verifies integer default parsing.
//
// VALIDATES: SchemaDefaultInt returns correct integer values and errors.
// PREVENTS: Incorrect parsing of numeric YANG defaults.
func TestSchemaDefaultInt_Parsing(t *testing.T) {
	schema := NewSchema()
	schema.Define("port", LeafWithDefault(TypeUint16, "179"))
	schema.Define("name", Leaf(TypeString))

	v, err := SchemaDefaultInt(schema, "port")
	require.NoError(t, err)
	assert.Equal(t, 179, v)

	_, err = SchemaDefaultInt(schema, "name")
	assert.Error(t, err, "leaf without default must return error")

	_, err = SchemaDefaultInt(schema, "nonexistent")
	assert.Error(t, err, "missing path must return error")

	_, err = SchemaDefaultInt(nil, "port")
	assert.Error(t, err, "nil schema must return error")
}

// TestEnvironmentSchemaChildren verifies all expected containers exist in environment schema.
//
// VALIDATES: YANG augments for api, chaos, etc. are resolved into the environment container.
// PREVENTS: Missing environment sections due to YANG parse errors.
func TestEnvironmentSchemaChildren(t *testing.T) {
	schema := YANGSchema()
	if schema == nil {
		t.Skip("YANG schema not available")
	}
	env := schema.Get("environment")
	require.NotNil(t, env, "environment node must exist")
	c, ok := env.(*ContainerNode)
	require.True(t, ok, "environment must be ContainerNode")

	children := c.Children()
	t.Logf("environment children: %v", children)

	for _, name := range []string{"daemon", "log", "debug", "tcp", "bgp", "cache", "api", "reactor", "chaos"} {
		assert.NotNil(t, c.Get(name), "environment.%s must exist in schema", name)
	}
}

// TestYANGDefaultsMatchRFC verifies that YANG schema declares correct RFC defaults.
// This is the single point of truth verification -- if this test fails,
// either YANG or the RFC reference needs updating.
//
// VALIDATES: YANG schema has RFC-compliant default values.
// PREVENTS: Drift between YANG declarations and RFC requirements.
func TestYANGDefaultsMatchRFC(t *testing.T) {
	schema := YANGSchema()
	if schema == nil {
		t.Skip("YANG schema not available")
	}

	tests := []struct {
		path string
		want string
		rfc  string
	}{
		// Peer-level (ze-bgp-conf.yang peer-fields grouping)
		{"bgp.peer.timer.receive-hold-time", "90", "RFC 4271 Section 10"},
		{"bgp.peer.timer.connect-retry", "120", "RFC 4271 Section 10"},
		{"bgp.peer.timer.send-hold-time", "0", "RFC 9687 (0 = auto)"},
		{"bgp.peer.capability.asn4", "true", "RFC 6793"},
		{"bgp.peer.remote.accept", "true", "RFC 4271 Section 8.1.1"},
		{"bgp.peer.local.connect", "true", "RFC 4271 Section 8.1.1"},
		{"bgp.peer.prefix.teardown", "true", "RFC 4486"},
		// Environment (ze-hub-conf.yang + ze-bgp-conf.yang augment)
		{"environment.daemon.user", "zeuser", "ze-hub-conf.yang"},
		{"environment.daemon.drop", "true", "ze-hub-conf.yang"},
		{"environment.daemon.umask", "0137", "ze-hub-conf.yang"},
		{"environment.log.enable", "true", "ze-hub-conf.yang"},
		{"environment.log.level", "INFO", "ze-hub-conf.yang"},
		{"environment.log.destination", "stdout", "ze-hub-conf.yang"},
		{"environment.log.short", "true", "ze-hub-conf.yang"},
		{"environment.tcp.port", "179", "RFC 4271"},
		{"environment.bgp.openwait", "120", "ze-bgp-conf.yang"},
		{"environment.cache.attributes", "true", "ze-bgp-conf.yang"},
		{"environment.api.ack", "true", "ze-bgp-conf.yang"},
		{"environment.api.chunk", "1", "ze-bgp-conf.yang"},
		{"environment.api.encoder", "json", "ze-bgp-conf.yang"},
		{"environment.api.respawn", "true", "ze-bgp-conf.yang"},
		{"environment.api.cli", "true", "ze-bgp-conf.yang"},
		{"environment.reactor.speed", "1.0", "ze-bgp-conf.yang"},
		{"environment.reactor.cache-ttl", "60", "ze-bgp-conf.yang"},
		{"environment.reactor.cache-max", "1000000", "ze-bgp-conf.yang"},
		{"environment.chaos.rate", "0.1", "ze-bgp-conf.yang"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := SchemaDefault(schema, tt.path)
			assert.Equal(t, tt.want, got, "YANG default for %s must match %s", tt.path, tt.rfc)
		})
	}
}
