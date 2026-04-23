package system_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/component/config/system"
)

// TestExpandEnvValue verifies $VAR expansion resolves from environment.
//
// VALIDATES: Values starting with $ are resolved from OS environment.
// PREVENTS: Literal $VAR being passed through as hostname.
func TestExpandEnvValue(t *testing.T) {
	t.Setenv("TEST_ZE_HOST", "router1")
	result := system.ExpandEnvValue("$TEST_ZE_HOST")
	assert.Equal(t, "router1", result)
}

// TestExpandEnvValue_NoPrefix verifies non-$ values are returned as-is.
//
// VALIDATES: Plain string values are not modified.
// PREVENTS: Accidental environment lookup on normal values.
func TestExpandEnvValue_NoPrefix(t *testing.T) {
	result := system.ExpandEnvValue("router1")
	assert.Equal(t, "router1", result)
}

// TestExpandEnvValue_EmptyEnv verifies empty env var returns literal $VAR.
//
// VALIDATES: Unset or empty env var keeps the literal $VAR string.
// PREVENTS: Empty string hostname when env var is not set.
func TestExpandEnvValue_EmptyEnv(t *testing.T) {
	t.Setenv("TEST_ZE_EMPTY", "")
	result := system.ExpandEnvValue("$TEST_ZE_EMPTY")
	assert.Equal(t, "$TEST_ZE_EMPTY", result)

	// Also test completely unset var
	result2 := system.ExpandEnvValue("$TEST_ZE_DOES_NOT_EXIST_XYZ")
	assert.Equal(t, "$TEST_ZE_DOES_NOT_EXIST_XYZ", result2)
}

// TestExtractSystemConfig verifies basic system config extraction.
//
// VALIDATES: system { host X; domain Y; } values are extracted from tree.
// PREVENTS: System identity config being inaccessible at runtime.
func TestExtractSystemConfig(t *testing.T) {
	tree := config.NewTree()
	sys := tree.GetOrCreateContainer("system")
	sys.Set("host", "router1")
	sys.Set("domain", "dc1.example.com")

	sc := system.ExtractSystemConfig(tree)
	assert.Equal(t, "router1", sc.Host)
	assert.Equal(t, "dc1.example.com", sc.Domain)
}

// TestExtractSystemConfig_EnvExpansion verifies $ENV expansion in host/domain.
//
// VALIDATES: $ENV values in system config are resolved from OS environment.
// PREVENTS: Literal $HOSTNAME being used as the system identity.
func TestExtractSystemConfig_EnvExpansion(t *testing.T) {
	t.Setenv("TEST_ZE_HOSTNAME", "myrouter")
	t.Setenv("TEST_ZE_DOMAIN", "lab.net")

	tree := config.NewTree()
	sys := tree.GetOrCreateContainer("system")
	sys.Set("host", "$TEST_ZE_HOSTNAME")
	sys.Set("domain", "$TEST_ZE_DOMAIN")

	sc := system.ExtractSystemConfig(tree)
	assert.Equal(t, "myrouter", sc.Host)
	assert.Equal(t, "lab.net", sc.Domain)
}

// TestExtractSystemConfig_Missing verifies defaults when no system block exists.
//
// VALIDATES: Missing system block produces default values (host="unknown", domain="").
// PREVENTS: Nil pointer or panic when system block is absent.
func TestExtractSystemConfig_Missing(t *testing.T) {
	tree := config.NewTree()

	sc := system.ExtractSystemConfig(tree)
	assert.Equal(t, "unknown", sc.Host)
	assert.Equal(t, "", sc.Domain)
	assert.Equal(t, "https://www.peeringdb.com", sc.PeeringDBURL)
	assert.Equal(t, uint8(10), sc.PeeringDBMargin)
}

// TestExtractSystemConfig_PeeringDB verifies PeeringDB config extraction.
//
// VALIDATES: AC-11 -- custom PeeringDB URL is read from config.
// VALIDATES: AC-12 -- custom margin is read from config.
// PREVENTS: PeeringDB settings being ignored.
func TestExtractSystemConfig_PeeringDB(t *testing.T) {
	tree := config.NewTree()
	sys := tree.GetOrCreateContainer("system")
	pdb := sys.GetOrCreateContainer("peeringdb")
	pdb.Set("url", "https://peeringdb.example.com")
	pdb.Set("margin", "20")

	sc := system.ExtractSystemConfig(tree)
	assert.Equal(t, "https://peeringdb.example.com", sc.PeeringDBURL)
	assert.Equal(t, uint8(20), sc.PeeringDBMargin)
}

// TestExtractSystemConfig_PeeringDB_Defaults verifies PeeringDB defaults
// when peeringdb block exists but has no overrides.
//
// VALIDATES: Default PeeringDB URL and margin are applied.
// PREVENTS: Zero margin or empty URL when peeringdb block is present but empty.
func TestExtractSystemConfig_PeeringDB_Defaults(t *testing.T) {
	tree := config.NewTree()
	sys := tree.GetOrCreateContainer("system")
	sys.GetOrCreateContainer("peeringdb") // empty peeringdb block

	sc := system.ExtractSystemConfig(tree)
	assert.Equal(t, "https://www.peeringdb.com", sc.PeeringDBURL)
	assert.Equal(t, uint8(10), sc.PeeringDBMargin)
}

// TestExtractSystemConfig_NameServers verifies leaf-list extraction.
//
// VALIDATES: AC-1 -- system { name-server [8.8.8.8 1.1.1.1]; } extracts both servers.
// PREVENTS: Name servers being silently ignored.
func TestExtractSystemConfig_NameServers(t *testing.T) {
	tree := config.NewTree()
	sys := tree.GetOrCreateContainer("system")
	sys.SetSlice("name-server", []string{"8.8.8.8", "1.1.1.1"})

	sc := system.ExtractSystemConfig(tree)
	assert.Equal(t, []string{"8.8.8.8", "1.1.1.1"}, sc.NameServers)
}

// TestExtractSystemConfig_NameServers_Empty verifies no servers returns nil.
//
// VALIDATES: AC-8 -- no name-servers configured returns nil slice.
// PREVENTS: Non-nil empty slice causing unexpected behavior.
func TestExtractSystemConfig_NameServers_Empty(t *testing.T) {
	tree := config.NewTree()
	tree.GetOrCreateContainer("system")

	sc := system.ExtractSystemConfig(tree)
	assert.Nil(t, sc.NameServers)
}

// TestExtractSystemConfig_DNS verifies dns tuning extraction.
//
// VALIDATES: AC-4 -- dns timeout, cache-size, cache-ttl extracted from config.
// PREVENTS: DNS tuning values being silently ignored.
func TestExtractSystemConfig_DNS(t *testing.T) {
	tree := config.NewTree()
	sys := tree.GetOrCreateContainer("system")
	dns := sys.GetOrCreateContainer("dns")
	dns.Set("timeout", "10")
	dns.Set("cache-size", "5000")
	dns.Set("cache-ttl", "3600")

	sc := system.ExtractSystemConfig(tree)
	assert.Equal(t, uint16(10), sc.DNSTimeout)
	assert.Equal(t, uint32(5000), sc.DNSCacheSize)
	assert.Equal(t, uint32(3600), sc.DNSCacheTTL)
}

// TestExtractSystemConfig_DNS_Defaults verifies default values when no dns block.
//
// VALIDATES: AC-10 -- default timeout 5, cache-size 10000, cache-ttl 86400, resolv-conf-path /tmp/resolv.conf.
// PREVENTS: Zero-value defaults breaking resolver or resolv.conf writer.
func TestExtractSystemConfig_DNS_Defaults(t *testing.T) {
	tree := config.NewTree()

	sc := system.ExtractSystemConfig(tree)
	assert.Equal(t, uint16(5), sc.DNSTimeout)
	assert.Equal(t, uint32(10000), sc.DNSCacheSize)
	assert.Equal(t, uint32(86400), sc.DNSCacheTTL)
	assert.Equal(t, "/tmp/resolv.conf", sc.ResolvConfPath)
}

// TestExtractSystemConfig_ResolvConfPath verifies custom resolv-conf-path extraction.
//
// VALIDATES: AC-5 -- resolv-conf-path extracted from system { dns {} }.
// PREVENTS: Custom path being ignored.
func TestExtractSystemConfig_ResolvConfPath(t *testing.T) {
	tree := config.NewTree()
	sys := tree.GetOrCreateContainer("system")
	dns := sys.GetOrCreateContainer("dns")
	dns.Set("resolv-conf-path", "/etc/resolv.conf")

	sc := system.ExtractSystemConfig(tree)
	assert.Equal(t, "/etc/resolv.conf", sc.ResolvConfPath)
}

// TestExtractSystemConfig_ResolvConfPath_Relative verifies relative paths are rejected.
//
// VALIDATES: Relative resolv-conf-path silently disabled (returns default).
// PREVENTS: Path traversal via relative resolv-conf-path.
func TestExtractSystemConfig_ResolvConfPath_Relative(t *testing.T) {
	tree := config.NewTree()
	sys := tree.GetOrCreateContainer("system")
	dns := sys.GetOrCreateContainer("dns")
	dns.Set("resolv-conf-path", "../etc/resolv.conf")

	sc := system.ExtractSystemConfig(tree)
	assert.Equal(t, "", sc.ResolvConfPath)
}

// TestExtractSystemConfig_ResolvConfPath_Traversal verifies path traversal is rejected.
//
// VALIDATES: Path containing /../ silently disabled.
// PREVENTS: Writing to unintended locations.
func TestExtractSystemConfig_ResolvConfPath_Traversal(t *testing.T) {
	tree := config.NewTree()
	sys := tree.GetOrCreateContainer("system")
	dns := sys.GetOrCreateContainer("dns")
	dns.Set("resolv-conf-path", "/tmp/../etc/resolv.conf")

	sc := system.ExtractSystemConfig(tree)
	assert.Equal(t, "", sc.ResolvConfPath)
}

// TestExtractSystemConfig_ResolvConfPath_Empty verifies empty disables writing.
//
// VALIDATES: Empty resolv-conf-path disables resolv.conf writing.
// PREVENTS: Writing to empty path.
func TestExtractSystemConfig_ResolvConfPath_Empty(t *testing.T) {
	tree := config.NewTree()
	sys := tree.GetOrCreateContainer("system")
	dns := sys.GetOrCreateContainer("dns")
	dns.Set("resolv-conf-path", "")

	sc := system.ExtractSystemConfig(tree)
	assert.Equal(t, "", sc.ResolvConfPath)
}

// TestExtractSystemConfig_PeeringDB_InvalidMargin verifies invalid margin is ignored.
//
// VALIDATES: Invalid margin value keeps the default.
// PREVENTS: Parsing error from crashing or setting margin to 0.
func TestExtractSystemConfig_PeeringDB_InvalidMargin(t *testing.T) {
	tree := config.NewTree()
	sys := tree.GetOrCreateContainer("system")
	pdb := sys.GetOrCreateContainer("peeringdb")
	pdb.Set("margin", "not-a-number")

	sc := system.ExtractSystemConfig(tree)
	assert.Equal(t, uint8(10), sc.PeeringDBMargin)
}
