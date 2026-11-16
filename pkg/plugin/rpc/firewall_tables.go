// Design: docs/architecture/api/process-protocol.md -- firewall table contribution
// Related: owned_devices.go -- the owned interface device requests

package rpc

import "encoding/json"

// MethodFirewallTablesReplace replaces the caller's complete set of firewall
// tables. The firewall component registers the handler from its own package,
// and accepts only table names the calling plugin reserved from its init().
const MethodFirewallTablesReplace = "ze-firewall:tables-replace"

// FirewallTablesReplaceInput carries the caller's complete set of tables as a
// firewall config section, the wrapped form the operator's config takes:
// {"firewall":{"table":{...}}}. The section form already has a schema, a
// parser, a validator and a backend verifier, so a plugin in any language can
// produce it. An empty table map withdraws every table the caller contributed.
type FirewallTablesReplaceInput struct {
	Section json.RawMessage `json:"section"`
}

// FirewallTablesReplaceOutput names the kernel tables the caller contributes
// after the replacement.
type FirewallTablesReplaceOutput struct {
	Tables []string `json:"tables"`
}
