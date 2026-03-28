// Package schema provides the YANG schema for MCP server configuration.
package schema

import _ "embed"

//go:embed ze-mcp-conf.yang
var ZeMCPConfYANG string
