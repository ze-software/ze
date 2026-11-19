//go:build integration && linux

// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- actual registry native proof
// Related: pppoe_admission.go -- same existing map/mutex and worker outcomes

package iface

import "errors"

var ErrNativePPPoEObservationBusy = errors.New("native PPPoE registry observation is busy")

// NativePPPoERegistryToken and NativePPPoEClientToken are comparable diagnostic
// identities only. Their zero values mean unobserved. They contain no mutation,
// lookup, file-close or release methods and grant no native authority.
type NativePPPoERegistryToken struct{ registry *pppoeClientSet }
type NativePPPoEClientToken struct{ client *PPPoEClient }

// Stopped observes only the already captured client's stop signal. A zero token
// is unobserved and refuses the question; no registry/native lookup occurs.
func (token NativePPPoEClientToken) Stopped() (bool, error) {
	if token.client == nil {
		return false, errors.New("native PPPoE observation has no client")
	}
	return token.client.stopped(), nil
}

type NativePPPoEObservation struct {
	Registry       NativePPPoERegistryToken
	Client         NativePPPoEClientToken
	Attempts       int
	NativeState    string
	CleanupPending bool
}

// ObserveNativePPPoERegistryForTest reads the actual registry without callbacks
// or I/O. A false result means that client is not published in this map; it says
// nothing about kernel resources or permission to release them. This read-only
// port is compiled only for native integration tests, never a shipped binary.
// A busy error is an unknown observation, never absence; the caller MUST retain
// that distinction. Tokens are diagnostic identity comparisons only.
func ObserveNativePPPoERegistryForTest(name string) (NativePPPoEObservation, bool, error) {
	set := globalPPPoEClients.Load()
	if set == nil {
		return NativePPPoEObservation{}, false, nil
	}
	if !set.mu.TryLock() {
		return NativePPPoEObservation{}, false, ErrNativePPPoEObservationBusy
	}
	defer set.mu.Unlock()
	client, found := set.active[name]
	if !found {
		return NativePPPoEObservation{Registry: NativePPPoERegistryToken{set}}, false, nil
	}
	client.mu.Lock()
	attempts := client.attempts
	pending := errors.Is(client.cleanupErr, ErrNativePPPPending)
	client.mu.Unlock()
	var state string
	switch client.nativeAdmission {
	case pppoeNativeIdle:
		state = "idle"
	case pppoeNativeDialing:
		state = "dialing"
	case pppoeNativeWriting:
		state = "writing"
	case pppoeNativeCleaning:
		state = "cleaning"
	case pppoeNativePending:
		state = "pending"
	case pppoeNativeAdmissionUnspecified:
		panic("BUG: actual published PPPoE client has no admission state")
	default:
		panic("BUG: unknown actual PPPoE admission state")
	}
	return NativePPPoEObservation{Registry: NativePPPoERegistryToken{set}, Client: NativePPPoEClientToken{client},
		Attempts: attempts, NativeState: state, CleanupPending: pending}, true, nil
}
