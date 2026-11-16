// Design: docs/architecture/config/syntax.md -- physical TCP interface aliases
// Related: transport_scope.go -- immutable interface-table comparison

package reactor

import (
	"errors"
	"net"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/diagnostic"
)

// TestTransportScopeNamePrecedesNumericIndex uses immutable interface values
// to prove Go's physical name namespace, including numeric interface names.
// No host interface is created, renamed, or needed for this branch.
func TestTransportScopeNamePrecedesNumericIndex(t *testing.T) {
	interfaces := []net.Interface{
		{Name: "port", Index: 3},
		{Name: "3", Index: 7},
		{Name: "0", Index: 9},
		{Name: "01", Index: 10},
	}
	for _, test := range []struct {
		name string
		a    string
		b    string
		want bool
	}{
		{name: "named interface and padded index", a: "port", b: "03", want: true},
		{name: "numeric name shadows index", a: "port", b: "3", want: false},
		{name: "numeric name resolves its own index", a: "3", b: "7", want: true},
		{name: "zero is an interface name", a: "0", b: "9", want: true},
		{name: "padded numeric name wins", a: "01", b: "10", want: true},
		{name: "numeric prefix follows Go socket conversion", a: "port", b: "3suffix", want: true},
		{name: "wrong interface", a: "port", b: "7", want: false},
		{name: "unknown names do not alias", a: "future-a", b: "future-b", want: false},
		{name: "one unknown name keeps its identity", a: "future-a", b: "future-a", want: true},
		{name: "default scope is not every interface", a: "", b: "port", want: false},
		{name: "numeric default is not every interface", a: "00", b: "port", want: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			a := netip.MustParseAddr(scopeAliasPeerIP).WithZone(test.a)
			b := netip.MustParseAddr(scopeAliasPeerIP).WithZone(test.b)
			assert.Equal(t, test.want, sameTransportAddressOn(a, b, interfaces))
			assert.Equal(t, test.want, sameTransportAddressOn(b, a, interfaces), "physical identity is symmetric")
		})
	}
	index, known := transportZoneIndex("999999999999suffix", interfaces)
	assert.True(t, known)
	assert.Equal(t, transportZoneIndexMax, index, "match Go dtoi's saturated decimal fallback")
}

// TestTransportScopeListenerMapRetainsBoundKey proves that lookup under an
// interface alias returns the original entry for reuse and later removal.
func TestTransportScopeListenerMapRetainsBoundKey(t *testing.T) {
	zones := scopeAliasInterfaces(t)
	for _, aliases := range zones.aliases() {
		t.Run(aliases.name, func(t *testing.T) {
			r := New(&Config{LocalAS: 65000})
			address := netip.MustParseAddr(scopeAliasLocalIP).WithZone(aliases.configured)
			key := netip.AddrPortFrom(address, attrTestPort).String()
			listener := NewListener(key)
			listener.listener = scopeAliasBoundListener{address: &net.TCPAddr{
				IP: address.AsSlice(), Zone: aliases.configured, Port: attrTestPort,
			}}
			r.listeners[key] = listener
			gotKey, got, found := r.listenerForAddressPort(address.WithZone(aliases.observed), attrTestPort)
			require.True(t, found)
			assert.Same(t, listener, got)
			assert.Equal(t, key, gotKey, "remove/restart must use the stored binding's original key")
			_, _, found = r.listenerForAddressPort(address.WithZone(zones.wrong), attrTestPort)
			assert.False(t, found)
			_, _, found = r.listenerForAddressPort(address, DefaultBGPPort)
			assert.False(t, found)
		})
	}
}

// TestTransportScopeReadFailureIsVisible proves the unreadable-table diagnostic
// names its cause and next command under a registered doctor/event code.
func TestTransportScopeReadFailureIsVisible(t *testing.T) {
	failure := transportScopeDiagnostic(errors.New("fixture: interface listing refused"))
	assert.Equal(t, codeTransportScopeUnknown, failure.Code)
	assert.Equal(t, diagnostic.SeverityWarning, failure.Severity)
	assert.Contains(t, failure.Message, "interface listing refused")
	assert.Contains(t, failure.Message, "cannot be matched")
	assert.Equal(t, transportScopeFix, failure.Fix)
	require.NotNil(t, diagnostic.Lookup(failure.Code))
	declared := false
	for _, check := range diagnostic.DoctorChecks() {
		if check.Name != "bgp-transport-scopes" {
			continue
		}
		assert.Contains(t, check.Codes, failure.Code)
		declared = true
	}
	assert.True(t, declared, "the runtime dependency must be reported by doctor")
}

type scopeAliasBoundListener struct {
	net.Listener
	address *net.TCPAddr
}

func (l scopeAliasBoundListener) Addr() net.Addr { return l.address }
