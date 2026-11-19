// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- original admission fence
// Related: pppoe_native_write.go, pppoe_cleanup.go -- actual operation publication

package iface

import (
	"errors"
	"sync"
)

// pppoeNativeAdmission is guarded by the client's existing registry mutex, or
// mu for a private client never published in a registry. Unspecified is invalid;
// NewPPPoEClient initializes Idle before publication. No state grants a native
// capability: it only orders admission against operations on existing owners.
type pppoeNativeAdmission uint8

const (
	pppoeNativeAdmissionUnspecified pppoeNativeAdmission = iota
	pppoeNativeIdle
	pppoeNativeDialing
	pppoeNativeWriting
	pppoeNativeCleaning
	pppoeNativePending
)

func (c *PPPoEClient) admissionMutex() *sync.Mutex {
	if c.registry != nil {
		return c.registry.mu
	}
	return &c.mu
}

// pppoeRegistryFor returns the actual process registry when this is its mutex.
// Standalone fixtures retain a view of their SAME supplied map/mutex; no extra
// map or retired-owner ledger is created. The pointer is set before Start.
func pppoeRegistryFor(active map[string]*PPPoEClient, mu *sync.Mutex) *pppoeClientSet {
	registry := globalPPPoEClients.Load()
	if registry != nil {
		if registry.mu == mu {
			return registry
		}
	}
	return &pppoeClientSet{mu: mu, active: active}
}

// pppoeRegistryBlockedLocked MUST hold the same registry mutex that guards
// active. This is a fresh observation at actual Dial, never a stale snapshot
// carried past native outcome publication. Resource-free config publication
// does not grant native admission and does not run I/O here.
func pppoeRegistryBlockedLocked(active map[string]*PPPoEClient, candidate *PPPoEClient) bool {
	for _, client := range active {
		if client == candidate {
			continue
		}
		switch client.nativeAdmission {
		case pppoeNativeIdle:
		case pppoeNativeDialing, pppoeNativeWriting, pppoeNativeCleaning, pppoeNativePending:
			return true
		case pppoeNativeAdmissionUnspecified:
			panic("BUG: published PPPoE client has no admission state")
		default:
			panic("BUG: unknown PPPoE native admission state")
		}
		// Private fixtures may install a callback/result before their first
		// worker. Production publishes this result under this same mutex.
		client.mu.Lock()
		pending := errors.Is(client.cleanupErr, ErrNativePPPPending)
		client.mu.Unlock()
		if pending {
			return true
		}
	}
	return false
}

// beginNativeDial MUST precede the actual Dial callback. On success the caller
// MUST call finishNativeDial after that callback returns, before inspecting or
// publishing its result. The mutex is released before the callback starts.
func (c *PPPoEClient) beginNativeDial() error {
	mu := c.admissionMutex()
	mu.Lock()
	defer mu.Unlock()
	if c.stopped() {
		return errStopped
	}
	if err := c.nativeAdmissionResultLocked(); err != nil {
		return err
	}
	if c.registry != nil {
		if pppoeRegistryBlockedLocked(c.registry.active, c) {
			return errors.Join(ErrNativePPPPending, errors.New("pppoe-client: an original operation blocks new admission"))
		}
	}
	c.nativeAdmission = pppoeNativeDialing
	return nil
}

// finishNativeDial MUST follow a successful beginNativeDial and the returned
// actual Dial result. A partial owner remains blocked until its cleanup runs.
func (c *PPPoEClient) finishNativeDial(cleanupPresent bool, err error) {
	mu := c.admissionMutex()
	mu.Lock()
	defer mu.Unlock()
	c.nativeAdmission = pppoeNativeIdle
	if err == nil {
		return
	}
	if cleanupPresent {
		// Partial custody is blocked until the same callback actually runs.
		c.nativeAdmission = pppoeNativeCleaning
		return
	}
	if errors.Is(err, ErrNativePPPPending) {
		c.nativeAdmission = pppoeNativePending
	}
}

// beginNativeWrite MUST precede a native callback. Success MUST be paired with
// finishNativeWrite on every returned or recovered result, with no mutex held
// across the callback or native ACK.
func (c *PPPoEClient) beginNativeWrite() error {
	mu := c.admissionMutex()
	mu.Lock()
	defer mu.Unlock()
	switch c.nativeAdmission {
	case pppoeNativeIdle:
		c.nativeAdmission = pppoeNativeWriting
		return nil
	case pppoeNativeDialing, pppoeNativeWriting, pppoeNativeCleaning, pppoeNativePending:
		return errors.Join(ErrNativePPPPending, errors.New("pppoe-client: original operation is still owned"))
	case pppoeNativeAdmissionUnspecified:
		panic("BUG: PPPoE native write has no admission state")
	default:
		panic("BUG: unknown PPPoE native admission state")
	}
}

// finishNativeWrite MUST follow a successful beginNativeWrite. Only explicit
// provider Pending/Unavailable retains the fence; settled route NACKs do not.
func (c *PPPoEClient) finishNativeWrite(err error) {
	mu := c.admissionMutex()
	mu.Lock()
	defer mu.Unlock()
	c.nativeAdmission = pppoeNativeIdle
	if pppoeNativeRefused(err) {
		c.nativeAdmission = pppoeNativePending
	}
}

// beginNativeCleanup MUST precede the retained callback. The caller MUST invoke
// publishNativeCleanup after its returned or recovered result. No mutex may be
// held across the callback, protocol joins, counter sample or native ACK.
func (c *PPPoEClient) beginNativeCleanup() {
	mu := c.admissionMutex()
	mu.Lock()
	c.nativeAdmission = pppoeNativeCleaning
	mu.Unlock()
}

// publishNativeCleanup closes the operation/publication gap. The existing slot
// and current custody outcome change under the same boundary as new admission.
// A terminal historical error remains reported while its cleared slot admits.
// It MUST follow beginNativeCleanup and the actual callback result.
// Its caller MUST then report that result through reportCleanupFailure after
// these publication locks have been released.
func (c *PPPoEClient) publishNativeCleanup(err error) {
	mu := c.admissionMutex()
	mu.Lock()
	defer mu.Unlock()
	if c.registry != nil {
		c.mu.Lock()
		defer c.mu.Unlock()
	}
	c.cleanupErr = err
	c.nativeAdmission = pppoeNativePending
	if errors.Is(err, ErrNativePPPPending) {
		return
	}
	c.nativeAdmission = pppoeNativeIdle
	c.pendingCleanup = nil
	c.native = nil
}

func (c *PPPoEClient) nativeAdmissionResult() error {
	mu := c.admissionMutex()
	mu.Lock()
	defer mu.Unlock()
	return c.nativeAdmissionResultLocked()
}

// nativeAdmissionResultLocked MUST hold admissionMutex. Absence of a callback
// cannot clear a previously explicit Pending outcome or an operation in flight.
func (c *PPPoEClient) nativeAdmissionResultLocked() error {
	switch c.nativeAdmission {
	case pppoeNativeIdle:
		return nil
	case pppoeNativeDialing, pppoeNativeWriting, pppoeNativeCleaning, pppoeNativePending:
		return ErrNativePPPPending
	case pppoeNativeAdmissionUnspecified:
		panic("BUG: PPPoE admission has no initialized state")
	default:
		panic("BUG: unknown PPPoE native admission state")
	}
}
