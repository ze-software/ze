// Design: docs/architecture/core-design.md -- audit component
// Related: streamable.go -- Streamable MCP auth path

package mcp

import (
	"strings"

	"github.com/ze-software/ze/internal/core/audit"
)

func recordMCPAuthFailure(recorder audit.Recorder, authHeader, remoteAddr string) {
	if recorder == nil {
		return
	}
	_ = recorder.Record(audit.Entry{
		Actor:      attemptedMCPBearerActor(authHeader),
		RemoteAddr: remoteAddr,
		Surface:    audit.MCP,
		Action:     audit.ActionAuthFail,
		Outcome:    audit.OutcomeDenied,
	})
}

func attemptedMCPBearerActor(header string) string {
	raw, ok := strings.CutPrefix(header, "Bearer ")
	if !ok {
		return ""
	}
	username, _, ok := strings.Cut(raw, ":")
	if !ok {
		return ""
	}
	return username
}
