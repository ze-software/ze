// Design: docs/architecture/web-workbench-pages.md -- IP DNS form page
// Related: workbench_form.go -- Reusable form component
// Related: page_ip_routes.go -- IP Routes page (sibling)

package web

import (
	"fmt"
	"html/template"
	"strconv"

	"github.com/ze-software/ze/internal/component/config"
)

// dNSFormData holds the DNS resolver configuration for the form.
type dNSFormData struct {
	Servers        []string
	ResolvConfPath string
	Timeout        string
	CacheSize      uint32
	CacheTTL       string
}

// buildDNSFormData reads DNS resolver configuration from the config tree.
// Returns sensible defaults when the tree has no DNS section.
func buildDNSFormData(tree *config.Tree) dNSFormData {
	data := dNSFormData{
		CacheSize: 10000, // must match system DNS YANG default
	}

	if tree == nil {
		return data
	}
	if systemTree := tree.GetContainer("system"); systemTree != nil {
		data.Servers = append(data.Servers, systemTree.GetSlice("name-server")...)
		if dnsTree := systemTree.GetContainer("dns"); dnsTree != nil {
			if resolvConfPath, ok := dnsTree.Get("resolv-conf-path"); ok {
				data.ResolvConfPath = resolvConfPath
			}
			if timeout, ok := dnsTree.Get("timeout"); ok {
				data.Timeout = timeout
			}
			if cacheSize, ok := dnsTree.Get("cache-size"); ok && cacheSize != "" {
				var size uint32
				if _, err := fmt.Sscanf(cacheSize, "%d", &size); err == nil {
					data.CacheSize = size
				}
			}
			if cacheTTL, ok := dnsTree.Get("cache-ttl"); ok {
				data.CacheTTL = cacheTTL
			}
		}
	}

	return data
}

// buildDNSWorkbenchForm constructs a WorkbenchFormData for DNS configuration.
func buildDNSWorkbenchForm(data dNSFormData) WorkbenchFormData {
	fields := []WorkbenchFormField{
		{
			Name:        "resolv-conf-path",
			Path:        "system/dns/resolv-conf-path",
			Label:       "resolv.conf Path",
			Type:        "text",
			Value:       data.ResolvConfPath,
			Description: "Path where Ze writes resolver configuration",
		},
		{
			Name:        "timeout",
			Path:        "system/dns/timeout",
			Label:       "Query Timeout",
			Type:        "number",
			Value:       data.Timeout,
			Description: "DNS query timeout in seconds",
		},
		{
			Name:        "cache-size",
			Path:        "system/dns/cache-size",
			Label:       "Cache Size",
			Type:        "number",
			Value:       strconv.Itoa(int(data.CacheSize)),
			Description: "Maximum number of cached DNS entries (0 disables cache)",
		},
		{
			Name:        "cache-ttl",
			Path:        "system/dns/cache-ttl",
			Label:       "Cache TTL",
			Type:        "number",
			Value:       data.CacheTTL,
			Description: "Maximum cache TTL in seconds",
		},
		{
			Name:        "servers",
			Label:       "Static Name Servers",
			Type:        "list",
			Items:       data.Servers,
			Description: "Static DNS name servers",
		},
	}

	return WorkbenchFormData{
		Title:      "DNS Configuration",
		Fields:     fields,
		SaveURL:    "/config/form/",
		DiscardURL: "/show/ip/dns/",
	}
}

// handleDNSPage renders the DNS configuration form content for the workbench.
func handleDNSPage(renderer *Renderer, tree *config.Tree) template.HTML {
	data := buildDNSFormData(tree)
	formData := buildDNSWorkbenchForm(data)
	return renderer.RenderFragment("workbench_form", formData)
}
