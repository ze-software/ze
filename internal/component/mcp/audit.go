// Design: docs/architecture/core-design.md -- audit component
// Related: streamable.go -- Streamable MCP auth path

package mcp

import "github.com/ze-software/ze/internal/core/audit"

// A rejected opaque bearer token has no trustworthy actor name. No part of
// that credential may enter the audit record.
func recordMCPAuthFailure(recorder audit.Recorder, remoteAddr string) {
	if recorder == nil {
		return
	}
	_ = recorder.Record(audit.Entry{
		RemoteAddr: remoteAddr,
		Surface:    audit.MCP,
		Action:     audit.ActionAuthFail,
		Outcome:    audit.OutcomeDenied,
	})
}
