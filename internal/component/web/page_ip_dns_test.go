package web

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
)

// TestDNSFormData_Build verifies DNS form data construction from config tree.
func TestDNSFormData_Build(t *testing.T) {
	tree := config.NewTree()
	resolve := tree.GetOrCreateContainer("resolve")
	dns := resolve.GetOrCreateContainer("dns")
	dns.Set("server", "8.8.8.8")
	dns.Set("cache-size", "5000")

	data := BuildDNSFormData(tree)
	require.Len(t, data.Servers, 1)
	assert.Equal(t, "8.8.8.8", data.Servers[0])
	assert.Equal(t, uint32(5000), data.CacheSize)
	assert.True(t, data.CacheEnabled)
}

// TestDNSFormData_Defaults verifies default values when tree is empty.
func TestDNSFormData_Defaults(t *testing.T) {
	data := BuildDNSFormData(nil)
	assert.Empty(t, data.Servers)
	assert.Equal(t, uint32(1000), data.CacheSize)
	assert.False(t, data.CacheEnabled)
}

// TestDNSFormData_EmptyTree verifies defaults when tree has no DNS section.
func TestDNSFormData_EmptyTree(t *testing.T) {
	tree := config.NewTree()
	data := BuildDNSFormData(tree)
	assert.Empty(t, data.Servers)
	assert.Equal(t, uint32(1000), data.CacheSize)
}

// TestDNSWorkbenchForm_Fields verifies the form field construction.
func TestDNSWorkbenchForm_Fields(t *testing.T) {
	data := DNSFormData{
		Servers:      []string{"8.8.8.8", "1.1.1.1"},
		CacheEnabled: true,
		CacheSize:    2000,
	}

	form := BuildDNSWorkbenchForm(data)
	assert.Equal(t, "DNS Configuration", form.Title)
	assert.Equal(t, "/admin/ip/dns/save", form.SaveURL)
	require.Len(t, form.Fields, 3)

	// Servers field is a list type.
	assert.Equal(t, "servers", form.Fields[0].Name)
	assert.Equal(t, "list", form.Fields[0].Type)
	assert.Equal(t, []string{"8.8.8.8", "1.1.1.1"}, form.Fields[0].Items)

	// Cache enabled is a toggle.
	assert.Equal(t, "cache-enabled", form.Fields[1].Name)
	assert.Equal(t, "toggle", form.Fields[1].Type)
	assert.Equal(t, "true", form.Fields[1].Value)

	// Cache size is a number.
	assert.Equal(t, "cache-size", form.Fields[2].Name)
	assert.Equal(t, "number", form.Fields[2].Type)
	assert.Equal(t, "2000", form.Fields[2].Value)
}
