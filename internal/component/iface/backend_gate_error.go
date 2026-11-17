// Design: docs/architecture/core-design.md — typed backend capability refusals
// Related: register.go — validateBackendGate renders the complete refusal

package iface

// backendGateRefusal preserves the operator's formatted aggregate and every
// original cause, including host verdicts and concrete JSON parser failures.
// validateBackendGate constructs it only after a refusal. Its zero value has
// empty text and no causes; a nil receiver is unsupported. No fields change
// after construction, so concurrent Error and Unwrap calls are safe.
type backendGateRefusal struct {
	text   string
	causes []error
}

func (refusal *backendGateRefusal) Error() string { return refusal.text }

// Unwrap keeps every cause reachable through errors.Is and errors.As, in the
// same order as the rendered messages. Callers MUST NOT modify the slice.
func (refusal *backendGateRefusal) Unwrap() []error { return refusal.causes }
