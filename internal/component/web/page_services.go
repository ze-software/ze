// Design: docs/architecture/web-workbench-pages.md -- Services section pages
// Related: workbench_form.go -- Form component
// Related: page_ip_dns.go -- DNS form page (pattern reference)
// Related: page_system.go -- System pages (sibling)

package web

import (
	"html/template"

	"github.com/ze-software/ze/internal/component/config"
)

// --- Helper: read config values ---

// getConfigValue reads a leaf value from the config tree at the given
// slash-separated path. Returns empty string if the path does not exist.
func getConfigValue(tree *config.Tree, path string) string {
	if tree == nil {
		return ""
	}

	parts := splitConfigPath(path)
	current := tree
	for i, part := range parts {
		if i == len(parts)-1 {
			// Last segment: read as leaf
			if v, ok := current.Get(part); ok {
				return v
			}
			return ""
		}
		child := current.GetContainer(part)
		if child == nil {
			return ""
		}
		current = child
	}
	return ""
}

// configValueOrDefault reads a leaf like getConfigValue but substitutes def
// when the leaf is absent. The config tree holds only what the operator wrote,
// so a leaf whose YANG default is not false needs its default supplied here.
// Without it a toggle renders unchecked and the next save writes the opposite
// of the running behavior, because the toggle always posts a value.
func configValueOrDefault(tree *config.Tree, path, def string) string {
	if v := getConfigValue(tree, path); v != "" {
		return v
	}
	return def
}

// splitConfigPath splits a slash-separated config path into segments.
func splitConfigPath(path string) []string {
	var parts []string
	start := 0
	for i := range path {
		if path[i] == '/' {
			if i > start {
				parts = append(parts, path[start:i])
			}
			start = i + 1
		}
	}
	if start < len(path) {
		parts = append(parts, path[start:])
	}
	return parts
}

// getConfigListItems returns server[] list entry names from a config tree path.
func getConfigListItems(tree *config.Tree, containerPath, listName string) []string {
	if tree == nil {
		return nil
	}
	parts := splitConfigPath(containerPath)
	current := tree
	for _, part := range parts {
		child := current.GetContainer(part)
		if child == nil {
			return nil
		}
		current = child
	}
	entries := current.GetListOrdered(listName)
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Key)
	}
	return names
}

// --- Services > SSH ---

// buildSSHFormData constructs a WorkbenchFormData for the SSH service config.
// Fields match environment/ssh in ze-ssh-conf.yang.
func buildSSHFormData(tree *config.Tree) WorkbenchFormData {
	return WorkbenchFormData{
		Title: "SSH Configuration",
		Fields: []WorkbenchFormField{
			{
				Name:        "enabled",
				Label:       labelEnabled,
				Type:        "toggle",
				Value:       getConfigValue(tree, "environment/ssh/enabled"),
				Description: "Enable SSH server",
			},
			{
				Name:        "servers",
				Label:       labelListenEndpoints,
				Type:        "list",
				Items:       getConfigListItems(tree, "environment/ssh", "server"),
				Description: "SSH server listen endpoints",
			},
			{
				Name:        "host-key",
				Label:       "Host Key Path",
				Type:        "text",
				Value:       getConfigValue(tree, "environment/ssh/host-key"),
				Description: "Path to SSH host key file (auto-generated if missing)",
			},
			{
				Name:        "idle-timeout",
				Label:       "Idle Timeout (seconds)",
				Type:        "number",
				Value:       getConfigValue(tree, "environment/ssh/idle-timeout"),
				Description: "Idle timeout in seconds (default 600)",
			},
			{
				Name:        "max-sessions",
				Label:       "Max Sessions",
				Type:        "number",
				Value:       getConfigValue(tree, "environment/ssh/max-sessions"),
				Description: "Maximum concurrent SSH sessions",
			},
		},
		SaveURL:    "/config/form/environment/ssh/",
		DiscardURL: "/show/ssh/",
	}
}

// handleSSHPage renders the SSH service configuration form.
func handleSSHPage(renderer *Renderer, viewTree *config.Tree) template.HTML {
	formData := buildSSHFormData(viewTree)
	return renderer.renderComponent("workbench_form", workbenchForm(formData))
}

// --- Services > Web ---

// buildWebFormData constructs a WorkbenchFormData for the Web service config.
// Fields match environment/web in ze-web-conf.yang.
func buildWebFormData(tree *config.Tree) WorkbenchFormData {
	return WorkbenchFormData{
		Title: "Web Configuration",
		Fields: []WorkbenchFormField{
			{
				Name:        "enabled",
				Label:       labelEnabled,
				Type:        "toggle",
				Value:       getConfigValue(tree, "environment/web/enabled"),
				Description: "Enable web interface",
			},
			{
				Name:        "servers",
				Label:       labelListenEndpoints,
				Type:        "list",
				Items:       getConfigListItems(tree, "environment/web", "server"),
				Description: "Web server listen endpoints",
			},
			{
				Name:        "insecure",
				Label:       "Insecure Mode",
				Type:        "toggle",
				Value:       getConfigValue(tree, "environment/web/insecure"),
				Description: "Disable authentication (forces host to 127.0.0.1)",
			},
		},
		SaveURL:    "/config/form/environment/web/",
		DiscardURL: "/show/web/",
	}
}

// handleWebServicePage renders the Web service configuration form.
func handleWebServicePage(renderer *Renderer, viewTree *config.Tree) template.HTML {
	formData := buildWebFormData(viewTree)
	return renderer.renderComponent("workbench_form", workbenchForm(formData))
}

// --- Services > Telemetry ---

// buildTelemetryFormData constructs a WorkbenchFormData for the Telemetry config.
// Fields match telemetry/prometheus in ze-telemetry-conf.yang.
func buildTelemetryFormData(tree *config.Tree) WorkbenchFormData {
	return WorkbenchFormData{
		Title: "Telemetry Configuration",
		Fields: []WorkbenchFormField{
			{
				Name:        "enabled",
				Label:       labelEnabled,
				Type:        "toggle",
				Value:       getConfigValue(tree, "telemetry/prometheus/enabled"),
				Description: "Enable Prometheus metrics endpoint",
			},
			{
				Name:        "servers",
				Label:       labelListenEndpoints,
				Type:        "list",
				Items:       getConfigListItems(tree, "telemetry/prometheus", "server"),
				Description: "Prometheus listen endpoints",
			},
			{
				Name:        "path",
				Label:       "Metrics Path",
				Type:        "text",
				Value:       getConfigValue(tree, "telemetry/prometheus/path"),
				Description: "HTTP path for metrics endpoint (default /metrics)",
			},
			{
				Name:        "basic-auth-enabled",
				Label:       "Basic Auth Enabled",
				Type:        "toggle",
				Value:       getConfigValue(tree, "telemetry/prometheus/basic-auth/enabled"),
				Description: "Require HTTP Basic Authentication for metrics and health endpoints",
			},
			{
				Name:        "basic-auth-realm",
				Label:       "Basic Auth Realm",
				Type:        "text",
				Value:       getConfigValue(tree, "telemetry/prometheus/basic-auth/realm"),
				Description: "HTTP Basic Authentication realm (default ze prometheus)",
			},
			{
				Name:        "basic-auth-username",
				Label:       "Basic Auth Username",
				Type:        "text",
				Value:       getConfigValue(tree, "telemetry/prometheus/basic-auth/username"),
				Description: "HTTP Basic Authentication username",
			},
			{
				Name:        "basic-auth-password",
				Label:       "Basic Auth Password Hash",
				Type:        "password",
				Value:       getConfigValue(tree, "telemetry/prometheus/basic-auth/password"),
				Description: "Bcrypt-hashed password. Use plaintext-password to set a new value.",
			},
			{
				Name:        "basic-auth-plaintext-password",
				Label:       "Basic Auth New Password",
				Type:        "password",
				Value:       getConfigValue(tree, "telemetry/prometheus/basic-auth/plaintext-password"),
				Description: "Write-only password that is hashed on commit and not persisted",
			},
			{
				Name:        "netdata-enabled",
				Label:       "Netdata OS Collectors",
				Type:        "toggle",
				Value:       getConfigValue(tree, "telemetry/prometheus/netdata/enabled"),
				Description: "Enable Netdata-compatible OS collector metrics",
			},
			{
				Name:        "netdata-prefix",
				Label:       "Netdata Metric Prefix",
				Type:        "text",
				Value:       getConfigValue(tree, "telemetry/prometheus/netdata/prefix"),
				Description: "Metric prefix for Netdata-compatible OS collectors only",
			},
			{
				Name:        "netdata-interval",
				Label:       "Netdata Sampling Interval",
				Type:        "number",
				Value:       getConfigValue(tree, "telemetry/prometheus/netdata/interval"),
				Description: "Netdata-compatible OS collector sampling interval in seconds (1-60)",
			},
			{
				Name:        "netdata-collectors",
				Label:       "Netdata Collector Overrides",
				Type:        "list",
				Items:       getConfigListItems(tree, "telemetry/prometheus/netdata", "collector"),
				Description: "Per-Netdata-collector enable and interval overrides",
			},
		},
		SaveURL:    "/config/form/telemetry/prometheus/",
		DiscardURL: "/show/telemetry/",
	}
}

// handleTelemetryPage renders the Telemetry service configuration form.
func handleTelemetryPage(renderer *Renderer, viewTree *config.Tree) template.HTML {
	formData := buildTelemetryFormData(viewTree)
	return renderer.renderComponent("workbench_form", workbenchForm(formData))
}

// --- Services > TACACS ---

// buildTACACSFormData constructs a WorkbenchFormData for the TACACS config.
// Fields match system/authentication/tacacs in ze-tacacs-conf.yang.
func buildTACACSFormData(tree *config.Tree) WorkbenchFormData {
	return WorkbenchFormData{
		Title: "TACACS+ Configuration",
		Fields: []WorkbenchFormField{
			{
				Name:        "servers",
				Label:       "TACACS+ Servers",
				Type:        "list",
				Items:       getConfigListItems(tree, "system/authentication/tacacs", "server"),
				Description: "TACACS+ servers, tried in configured order",
			},
			{
				Name:        "timeout",
				Label:       "Timeout (seconds)",
				Type:        "number",
				Value:       getConfigValue(tree, "system/authentication/tacacs/timeout"),
				Description: "Per-server connection timeout in seconds (1-300, default 5)",
			},
			{
				Name:        "source-address",
				Label:       "Source Address",
				Type:        "ip",
				Value:       getConfigValue(tree, "system/authentication/tacacs/source-address"),
				Description: "Source IP for outbound TACACS+ connections",
			},
			{
				Name:        "authorization",
				Label:       "Authorization",
				Type:        "toggle",
				Value:       getConfigValue(tree, "system/authentication/tacacs/authorization"),
				Description: "Enable per-command TACACS+ authorization",
			},
			{
				Name:        "accounting",
				Label:       "Accounting",
				Type:        "toggle",
				Value:       getConfigValue(tree, "system/authentication/tacacs/accounting"),
				Description: "Enable command execution accounting",
			},
		},
		SaveURL:    "/config/form/system/authentication/tacacs/",
		DiscardURL: "/show/tacacs/",
	}
}

// handleTACACSPage renders the TACACS+ service configuration form.
func handleTACACSPage(renderer *Renderer, viewTree *config.Tree) template.HTML {
	formData := buildTACACSFormData(viewTree)
	return renderer.renderComponent("workbench_form", workbenchForm(formData))
}

// --- Services > MCP ---

// buildMCPFormData constructs a WorkbenchFormData for the MCP config.
// Fields match environment/mcp in ze-mcp-conf.yang. Sensitive fields
// (token, TLS key) use the password type for masking.
func buildMCPFormData(tree *config.Tree) WorkbenchFormData {
	return WorkbenchFormData{
		Title: "MCP Configuration",
		Fields: []WorkbenchFormField{
			{
				Name:        "enabled",
				Label:       labelEnabled,
				Type:        "toggle",
				Value:       getConfigValue(tree, "environment/mcp/enabled"),
				Description: "Enable MCP server",
			},
			{
				Name:        "bind-remote",
				Label:       "Bind Remote",
				Type:        "toggle",
				Value:       getConfigValue(tree, "environment/mcp/bind-remote"),
				Description: "Allow binding to non-loopback addresses",
			},
			{
				Name:        "auth-mode",
				Label:       "Auth Mode",
				Type:        "dropdown",
				Value:       getConfigValue(tree, "environment/mcp/auth-mode"),
				Options:     []string{"none", "bearer", "bearer-list", "oauth"},
				Description: "Authentication strategy",
			},
			{
				Name:        "token",
				Label:       labelBearerToken,
				Type:        "password",
				Value:       getConfigValue(tree, "environment/mcp/token"),
				Description: "Bearer token for auth-mode=bearer (sensitive)",
			},
			{
				Name:        "servers",
				Label:       labelListenEndpoints,
				Type:        "list",
				Items:       getConfigListItems(tree, "environment/mcp", "server"),
				Description: "MCP server listen endpoints",
			},
			{
				Name:        "identities",
				Label:       "Identities",
				Type:        "list",
				Items:       getConfigListItems(tree, "environment/mcp", "identity"),
				Description: "Per-identity bearer entries (auth-mode=bearer-list)",
			},
			{
				Name:        "oauth-authorization-server",
				Label:       "OAuth Authorization Server",
				Type:        "text",
				Value:       getConfigValue(tree, "environment/mcp/oauth/authorization-server"),
				Description: "HTTPS URL of the authorization server",
			},
			{
				Name:        "oauth-audience",
				Label:       "OAuth Audience",
				Type:        "text",
				Value:       getConfigValue(tree, "environment/mcp/oauth/audience"),
				Description: "Canonical URL identifying this MCP endpoint",
			},
			{
				Name:        "tls-cert",
				Label:       "TLS Certificate Path",
				Type:        "text",
				Value:       getConfigValue(tree, "environment/mcp/tls/cert"),
				Description: "Path to PEM-encoded certificate file",
			},
			{
				Name:        "tls-key",
				Label:       "TLS Key Path",
				Type:        "password",
				Value:       getConfigValue(tree, "environment/mcp/tls/key"),
				Description: "Path to PEM-encoded private key file (sensitive)",
			},
		},
		SaveURL:    "/config/form/environment/mcp/",
		DiscardURL: "/show/mcp/",
	}
}

// handleMCPPage renders the MCP service configuration form.
func handleMCPPage(renderer *Renderer, viewTree *config.Tree) template.HTML {
	formData := buildMCPFormData(viewTree)
	return renderer.renderComponent("workbench_form", workbenchForm(formData))
}

// --- Services > Looking Glass ---

// buildLookingGlassFormData constructs a WorkbenchFormData for the LG config.
// Fields match environment/looking-glass in ze-lg-conf.yang.
func buildLookingGlassFormData(tree *config.Tree) WorkbenchFormData {
	return WorkbenchFormData{
		Title: "Looking Glass Configuration",
		Fields: []WorkbenchFormField{
			{
				Name:        "enabled",
				Label:       labelEnabled,
				Type:        "toggle",
				Value:       getConfigValue(tree, "environment/looking-glass/enabled"),
				Description: "Enable looking glass",
			},
			{
				Name:        "servers",
				Label:       labelListenEndpoints,
				Type:        "list",
				Items:       getConfigListItems(tree, "environment/looking-glass", "server"),
				Description: "Looking glass listen endpoints",
			},
			{
				Name: "tls",
				// getConfigValue reads the RAW tree, which carries no YANG
				// defaults, so an absent leaf reads "". The looking glass
				// defaults TLS ON, and a toggle rendered from "" shows OFF and
				// posts `tls false` on the next save of ANY field on this form.
				// That would silently downgrade a TLS looking glass to plaintext
				// (ai/rules/protocol.md), so the default is applied here
				// exactly as ExtractLGConfig applies it.
				Label:       "TLS",
				Type:        "toggle",
				Value:       configValueOrDefault(tree, "environment/looking-glass/tls", "true"),
				Description: "Serve HTTPS (needs blob storage for certificates). Default on; turn off to serve plaintext",
			},
			{
				Name:        "token",
				Label:       labelBearerToken,
				Type:        "password",
				Value:       getConfigValue(tree, "environment/looking-glass/token"),
				Description: "Bearer token gating every route (sensitive). Empty leaves the looking glass open",
			},
		},
		SaveURL:    "/config/form/environment/looking-glass/",
		DiscardURL: "/show/lg/",
	}
}

// handleLookingGlassPage renders the Looking Glass service configuration form.
func handleLookingGlassPage(renderer *Renderer, viewTree *config.Tree) template.HTML {
	formData := buildLookingGlassFormData(viewTree)
	return renderer.renderComponent("workbench_form", workbenchForm(formData))
}

// --- Services > API ---

// buildAPIFormData constructs a WorkbenchFormData for the API config.
// Fields match environment/api-server in ze-api-conf.yang. Sensitive fields
// (token, TLS key) use the password type for masking.
func buildAPIFormData(tree *config.Tree) WorkbenchFormData {
	return WorkbenchFormData{
		Title: "API Configuration",
		Fields: []WorkbenchFormField{
			{
				Name:        "token",
				Label:       labelBearerToken,
				Type:        "password",
				Value:       getConfigValue(tree, "environment/api-server/token"),
				Description: "Bearer token for API authentication (sensitive)",
			},
			{
				Name:        "rest-enabled",
				Label:       "REST Enabled",
				Type:        "toggle",
				Value:       getConfigValue(tree, "environment/api-server/rest/enabled"),
				Description: "Enable REST API server",
			},
			{
				Name:        "rest-servers",
				Label:       "REST Listen Endpoints",
				Type:        "list",
				Items:       getConfigListItems(tree, "environment/api-server/rest", "server"),
				Description: "REST API listen endpoints",
			},
			{
				Name:        "rest-cors-origin",
				Label:       "CORS Allowed Origin",
				Type:        "text",
				Value:       getConfigValue(tree, "environment/api-server/rest/cors-origin"),
				Description: "CORS allowed origin (empty disables CORS headers)",
			},
			{
				Name:        "grpc-enabled",
				Label:       "gRPC Enabled",
				Type:        "toggle",
				Value:       getConfigValue(tree, "environment/api-server/grpc/enabled"),
				Description: "Enable gRPC API server",
			},
			{
				Name:        "grpc-servers",
				Label:       "gRPC Listen Endpoints",
				Type:        "list",
				Items:       getConfigListItems(tree, "environment/api-server/grpc", "server"),
				Description: "gRPC API listen endpoints",
			},
			{
				Name:        "grpc-tls-cert",
				Label:       "gRPC TLS Certificate",
				Type:        "text",
				Value:       getConfigValue(tree, "environment/api-server/grpc/tls-cert"),
				Description: "Path to TLS certificate file for gRPC",
			},
			{
				Name:        "grpc-tls-key",
				Label:       "gRPC TLS Key",
				Type:        "password",
				Value:       getConfigValue(tree, "environment/api-server/grpc/tls-key"),
				Description: "Path to TLS private key file for gRPC (sensitive)",
			},
		},
		SaveURL:    "/config/form/environment/api-server/",
		DiscardURL: "/show/api/",
	}
}

// handleAPIPage renders the API service configuration form.
func handleAPIPage(renderer *Renderer, viewTree *config.Tree) template.HTML {
	formData := buildAPIFormData(viewTree)
	return renderer.renderComponent("workbench_form", workbenchForm(formData))
}
