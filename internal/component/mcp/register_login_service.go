// Design: ai/patterns/registration.md -- MCP login-service registration
// Related: streamable.go -- enforces LoginServiceGate under loginServiceMCP

package mcp

import "github.com/ze-software/ze/internal/component/config/loginservice"

// loginServiceMCP is the registered login-service name for MCP Streamable HTTP
// named-principal authentication (bearer-list and oauth).
const loginServiceMCP = "mcp"

func init() {
	loginservice.MustRegister(loginservice.Entry{
		Name:        loginServiceMCP,
		Owner:       "internal/component/mcp",
		Description: "MCP Streamable HTTP named-principal authentication",
	})
}
