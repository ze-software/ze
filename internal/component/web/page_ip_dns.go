// Design: plan/spec-web-4-interfaces.md -- IP DNS form page
// Related: workbench_form.go -- Reusable form component
// Related: page_ip_routes.go -- IP Routes page (sibling)

package web

import (
	"fmt"
	"html/template"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
)

// DNSFormData holds the DNS resolver configuration for the form.
type DNSFormData struct {
	Servers      []string
	CacheEnabled bool
	CacheSize    uint32
}

// BuildDNSFormData reads DNS resolver configuration from the config tree.
// Returns sensible defaults when the tree has no DNS section.
func BuildDNSFormData(tree *config.Tree) DNSFormData {
	data := DNSFormData{
		CacheSize: 1000, // default
	}

	if tree == nil {
		return data
	}

	// Try to read resolve/dns configuration from the tree.
	resolveTree := tree.GetContainer("resolve")
	if resolveTree == nil {
		return data
	}

	dnsTree := resolveTree.GetContainer("dns")
	if dnsTree == nil {
		return data
	}

	if server, ok := dnsTree.Get("server"); ok && server != "" {
		data.Servers = []string{server}
	}

	if cacheSize, ok := dnsTree.Get("cache-size"); ok && cacheSize != "" {
		var size uint32
		if _, err := fmt.Sscanf(cacheSize, "%d", &size); err == nil {
			data.CacheSize = size
			data.CacheEnabled = size > 0
		}
	}

	return data
}

// BuildDNSWorkbenchForm constructs a WorkbenchFormData for DNS configuration.
func BuildDNSWorkbenchForm(data DNSFormData) WorkbenchFormData {
	cacheValue := "false"
	if data.CacheEnabled {
		cacheValue = htmxRequestTrue // reuse package-level "true" constant
	}

	fields := []WorkbenchFormField{
		{
			Name:        "servers",
			Label:       "Upstream DNS Servers",
			Type:        "list",
			Items:       data.Servers,
			Description: "DNS servers for name resolution (e.g., 8.8.8.8, 1.1.1.1)",
		},
		{
			Name:        "cache-enabled",
			Label:       "Cache Enabled",
			Type:        "toggle",
			Value:       cacheValue,
			Description: "Enable DNS response caching",
		},
		{
			Name:        "cache-size",
			Label:       "Cache Size",
			Type:        "number",
			Value:       fmt.Sprintf("%d", data.CacheSize),
			Description: "Maximum number of cached DNS entries (0 disables cache)",
		},
	}

	return WorkbenchFormData{
		Title:      "DNS Configuration",
		Fields:     fields,
		SaveURL:    "/admin/ip/dns/save",
		DiscardURL: "/show/ip/dns/",
	}
}

// HandleDNSPage renders the DNS configuration form content for the workbench.
func HandleDNSPage(renderer *Renderer, tree *config.Tree) template.HTML {
	data := BuildDNSFormData(tree)
	formData := BuildDNSWorkbenchForm(data)
	return renderer.RenderFragment("workbench_form", formData)
}
