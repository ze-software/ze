// Design: docs/research/l2tpv2-ze-integration.md -- native unit release after remote cleanup

package ppp

// NativeReleaseSeal belongs to the policy admitted for this exact transport.
// Sealing MUST atomically prevent future native operations. It succeeds only
// when no earlier operation can still affect the interface. An uncertain
// operation MUST return an error until its exact cleanup is confirmed.
// This capability is local to the transport and MUST NOT cross an RPC boundary.
type NativeReleaseSeal interface {
	// NativeBirthTag is the full opaque alias used by the admitted policy's
	// native fence. The kernel owner MUST preserve it without reconstructing it.
	NativeBirthTag() string
	SealNativeRelease() error
}

// NativeReleaseHandle is the kernel owner's optional release capability. The
// caller MUST bind the admitted seal before its first remote rate operation
// and reserve a bounded, strong cleanup owner before that operation starts.
// Close wakes borrowed readers and retains the native unit while sealing is
// unconfirmed. The cleanup owner retries Close after safe release is confirmed.
// Stream or final descriptor-close errors can remain after confirmed release.
type NativeReleaseHandle interface {
	BindNativeReleaseSeal(NativeReleaseSeal) error
	NativeReleaseBound() bool
	NativeReleasePending() bool
	// NativeReleaseBirthTag returns the immutable receipt admitted at binding,
	// including after release, so cleanup can clear only its own warning.
	NativeReleaseBirthTag() string
}
