// Design: docs/architecture/api/process-protocol.md -- original accepted redistribution lookup.
// Related: bridge.go -- delivery lifetime fencing for typed callbacks.
package rpc

import "context"

// OriginAdmissionLookupHandler reads one consistent published accepted-record
// snapshot. It MUST honor ctx and must not call the engine or perform I/O.
// Input is borrowed until return. Output ownership passes to the caller; the
// handler MUST NOT subsequently modify its output or any referenced bytes.
type OriginAdmissionLookupHandler func(context.Context, OriginAdmissionLookupInput) (*OriginAdmissionLookupOutput, error)

// SetOriginAdmissionLookup MUST install the independent typed callback before
// SetReady publishes it. The worker owner MUST stop and join it at shutdown.
func (b *DirectBridge) SetOriginAdmissionLookup(handler OriginAdmissionLookupHandler) {
	b.originAdmissionLookup = handler
	b.hasOriginAdmissionLookup.Store(handler != nil)
}

// HasOriginAdmissionLookup reports handler support, never a peer permission grant.
func (b *DirectBridge) HasOriginAdmissionLookup() bool {
	return b.ready.Load() && b.hasOriginAdmissionLookup.Load()
}

// OriginAdmissionLookup bypasses the ordinary callback queue. The SDK slot enqueues
// on its independent worker; a blocked execute-command cannot prevent lookup.
func (b *DirectBridge) OriginAdmissionLookup(ctx context.Context, input OriginAdmissionLookupInput) (*OriginAdmissionLookupOutput, error) {
	return bridgeReadSnapshot(b, ctx, input, OriginAdmissionLookupCallContext,
		&b.hasOriginAdmissionLookup, &b.originAdmissionLookup,
		"origin-admission-lookup handler is not installed; register OnOriginAdmissionLookup before Run",
		(*OriginAdmissionLookupOutput).Validate)
}
