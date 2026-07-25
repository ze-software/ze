package firewallvpp_test

import (
	"errors"
	"testing"

	"github.com/ze-software/ze/internal/component/firewall"
	_ "github.com/ze-software/ze/internal/plugins/firewall/vpp"
)

func sentinelFactory() (firewall.Backend, error) {
	return nil, errors.New("firewallvpp test sentinel: real backend was NOT registered at init")
}

func TestBackendRegistered(t *testing.T) {
	err := firewall.RegisterBackend("vpp", sentinelFactory)
	if err == nil {
		t.Fatal("expected duplicate-registration error, got nil (backend not registered at init?)")
	}
}

func TestVerifierRegistered(t *testing.T) {
	err := firewall.RegisterVerifier("vpp", func(_ []firewall.Table) error { return nil })
	if err == nil {
		t.Fatal("expected duplicate-registration error, got nil (verifier not registered at init?)")
	}
}
