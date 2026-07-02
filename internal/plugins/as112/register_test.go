package as112

import (
	"net"
	"slices"
	"testing"
)

// VALIDATES: AC-1 -- the four canonical host addresses (as /32,/128 CIDRs)
// are the complete set with no address-family restriction.
func TestHostAddresses_Both(t *testing.T) {
	got := hostAddresses(addressFamilyBoth)
	want := []string{anycastV4DirectDelegationHost, anycastV4DNAMERedirectionHost, anycastV6DirectDelegationHost, anycastV6DNAMERedirectionHost}
	if len(got) != len(want) {
		t.Fatalf("hostAddresses(both) = %v, want %v", got, want)
	}
	for _, w := range want {
		if !slices.Contains(got, w) {
			t.Errorf("hostAddresses(both) missing %q", w)
		}
	}
}

// VALIDATES: AC-10 -- ipv4-only registers only the two IPv4 addresses.
func TestHostAddresses_IPv4Only(t *testing.T) {
	got := hostAddresses(addressFamilyIPv4Only)
	if len(got) != 2 || !slices.Contains(got, anycastV4DirectDelegationHost) || !slices.Contains(got, anycastV4DNAMERedirectionHost) {
		t.Fatalf("hostAddresses(ipv4-only) = %v, want exactly the 2 IPv4 addresses", got)
	}
}

// VALIDATES: AC-10 -- ipv6-only registers only the two IPv6 addresses.
func TestHostAddresses_IPv6Only(t *testing.T) {
	got := hostAddresses(addressFamilyIPv6Only)
	if len(got) != 2 || !slices.Contains(got, anycastV6DirectDelegationHost) || !slices.Contains(got, anycastV6DNAMERedirectionHost) {
		t.Fatalf("hostAddresses(ipv6-only) = %v, want exactly the 2 IPv6 addresses", got)
	}
}

// VALIDATES: AC-1/AC-9 -- OnConfigure registers on enable and unregisters on
// disable, symmetrically, using spec-as112-1's registry API (via the
// injectable registerFn/unregisterFn seam so this test does not depend on
// the real iface package's global state).
func TestOnConfigure_RegistersAddressesOnEnable(t *testing.T) {
	var registered []string
	var registeredOwner, unregisteredOwner string
	registerFn := func(ifaceName, owner string, addrs []string) error {
		registeredOwner = owner
		registered = addrs
		_ = ifaceName
		return nil
	}
	unregisterFn := func(owner string) { unregisteredOwner = owner }

	if err := applyAddressRegistration(as112Config{Enabled: true, AddressFamily: addressFamilyBoth}, registerFn, unregisterFn); err != nil {
		t.Fatalf("applyAddressRegistration(enabled): unexpected error: %v", err)
	}
	if registeredOwner != as112Owner {
		t.Fatalf("registered owner = %q, want %q", registeredOwner, as112Owner)
	}
	if len(registered) != 4 {
		t.Fatalf("registered addrs = %v, want 4", registered)
	}

	if err := applyAddressRegistration(as112Config{Enabled: false}, registerFn, unregisterFn); err != nil {
		t.Fatalf("applyAddressRegistration(disabled): unexpected error: %v", err)
	}
	if unregisteredOwner != as112Owner {
		t.Fatalf("unregistered owner = %q, want %q", unregisteredOwner, as112Owner)
	}
}

// VALIDATES: runAS112Plugin refuses to start (nonzero exit) when conn is not
// a same-process (DirectBridge-carrying) connection -- applyAddressRegistration
// calls iface.RegisterOwnedAddresses as a plain Go function, which would
// silently no-op against a forked subprocess's own copy of iface's package
// state instead of erroring. A plain net.Pipe() end here matches exactly what
// an external plugin's non-bridged conn looks like from the SDK's
// perspective (see sdk.Plugin.IsInternal).
// PREVENTS: as112 configured `plugin { external as112 { ... } }` silently
// accepting every config commit while never landing its anycast addresses on
// any real kernel interface.
func TestRunAS112Plugin_RefusesExternalProcess(t *testing.T) {
	pluginEnd, engineEnd := net.Pipe()
	t.Cleanup(func() {
		pluginEnd.Close() //nolint:errcheck // test cleanup
		engineEnd.Close() //nolint:errcheck // test cleanup
	})

	if code := runAS112Plugin(pluginEnd); code != 1 {
		t.Fatalf("runAS112Plugin(external conn) = %d, want 1", code)
	}
}
