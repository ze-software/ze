package trafficvpp_test

import (
	"errors"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/traffic"
	_ "codeberg.org/thomas-mangin/ze/internal/plugins/traffic/vpp"
)

// sentinelFactory is a non-nil factory used as the duplicate-registration
// probe. If our probe ever gets accepted (i.e., the backend was NOT
// registered at package init), this factory replaces the real one and
// the global state is poisoned. Use a non-nil sentinel so that a
// post-poisoning LoadBackend returns a clear error rather than panicking
// with a nil-function call.
func sentinelFactory() (traffic.Backend, error) {
	return nil, errors.New("trafficvpp test sentinel: real backend was NOT registered at init")
}

func TestBackendRegistered(t *testing.T) {
	// init() in the vpp package registers "vpp" at import time. The blank
	// import above pulls that side-effect in. RegisterBackend rejects
	// duplicates, so attempting to re-register returns an error. If we
	// reach here without the init failing, registration succeeded.
	err := traffic.RegisterBackend("vpp", sentinelFactory)
	if err == nil {
		t.Fatal("expected duplicate-registration error, got nil (backend not registered at init?)")
	}
}
