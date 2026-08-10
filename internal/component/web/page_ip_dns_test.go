package web

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
)

// TestDNSFormData_Build verifies DNS form data construction from config tree.
func TestDNSFormData_Build(t *testing.T) {
	tree := config.NewTree()
	system := tree.GetOrCreateContainer("system")
	system.SetSlice("name-server", []string{"8.8.8.8"})
	dns := system.GetOrCreateContainer("dns")
	dns.Set("resolv-conf-path", "/etc/resolv.conf")
	dns.Set("timeout", "3")
	dns.Set("cache-size", "5000")
	dns.Set("cache-ttl", "600")

	data := buildDNSFormData(tree)
	require.Len(t, data.Servers, 1)
	assert.Equal(t, "8.8.8.8", data.Servers[0])
	assert.Equal(t, "/etc/resolv.conf", data.ResolvConfPath)
	assert.Equal(t, "3", data.Timeout)
	assert.Equal(t, uint32(5000), data.CacheSize)
	assert.Equal(t, "600", data.CacheTTL)
}

// TestDNSFormData_Defaults verifies default values when tree is empty.
func TestDNSFormData_Defaults(t *testing.T) {
	data := buildDNSFormData(nil)
	assert.Empty(t, data.Servers)
	assert.Equal(t, uint32(10000), data.CacheSize)
	assert.Empty(t, data.ResolvConfPath)
}

// TestDNSFormData_EmptyTree verifies defaults when tree has no DNS section.
func TestDNSFormData_EmptyTree(t *testing.T) {
	tree := config.NewTree()
	data := buildDNSFormData(tree)
	assert.Empty(t, data.Servers)
	assert.Equal(t, uint32(10000), data.CacheSize)
}

// TestDNSWorkbenchForm_Fields verifies the form field construction.
func TestDNSWorkbenchForm_Fields(t *testing.T) {
	data := dNSFormData{
		Servers:        []string{"8.8.8.8", "1.1.1.1"},
		ResolvConfPath: "/etc/resolv.conf",
		Timeout:        "3",
		CacheSize:      2000,
		CacheTTL:       "600",
	}

	form := buildDNSWorkbenchForm(data)
	assert.Equal(t, "DNS Configuration", form.Title)
	assert.Equal(t, "/config/form/", form.SaveURL)
	require.Len(t, form.Fields, 5)

	assert.Equal(t, "resolv-conf-path", form.Fields[0].Name)
	assert.Equal(t, "system/dns/resolv-conf-path", form.Fields[0].Path)
	assert.Equal(t, "text", form.Fields[0].Type)
	assert.Equal(t, "/etc/resolv.conf", form.Fields[0].Value)

	assert.Equal(t, "timeout", form.Fields[1].Name)
	assert.Equal(t, "number", form.Fields[1].Type)
	assert.Equal(t, "3", form.Fields[1].Value)

	assert.Equal(t, "cache-size", form.Fields[2].Name)
	assert.Equal(t, "number", form.Fields[2].Type)
	assert.Equal(t, "2000", form.Fields[2].Value)

	assert.Equal(t, "cache-ttl", form.Fields[3].Name)
	assert.Equal(t, "number", form.Fields[3].Type)
	assert.Equal(t, "600", form.Fields[3].Value)

	assert.Equal(t, "servers", form.Fields[4].Name)
	assert.Equal(t, "list", form.Fields[4].Type)
	assert.Equal(t, []string{"8.8.8.8", "1.1.1.1"}, form.Fields[4].Items)
}
