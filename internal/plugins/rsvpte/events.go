// Design: docs/architecture/rsvpte/mpls-rsvp-te.md -- RSVP-TE event bus types
// Related: wire.go -- message types used in events
package rsvpte

import "github.com/ze-software/ze/internal/core/events"

const Namespace = "rsvp-te"

const (
	EventLSPUp   = "lsp-up"
	EventLSPDown = "lsp-down"
	EventPathErr = "path-err"
)

// lSPEvent carries RSVP-TE LSP lifecycle information on the event bus.
type lSPEvent struct {
	TunnelEndpoint string  `json:"tunnel-endpoint"`
	TunnelID       uint16  `json:"tunnel-id"`
	LSPID          uint16  `json:"lsp-id"`
	Bandwidth      float32 `json:"bandwidth"`
	State          string  `json:"state"`
}

// pathErrEvent carries RSVP-TE PathErr information on the event bus.
type pathErrEvent struct {
	TunnelEndpoint string `json:"tunnel-endpoint"`
	TunnelID       uint16 `json:"tunnel-id"`
	ErrorCode      uint8  `json:"error-code"`
	ErrorValue     uint16 `json:"error-value"`
	ErrorNode      string `json:"error-node"`
}

var (
	LSPUp   = events.Register[*lSPEvent](Namespace, EventLSPUp)
	LSPDown = events.Register[*lSPEvent](Namespace, EventLSPDown)
	PathErr = events.Register[*pathErrEvent](Namespace, EventPathErr)
)
