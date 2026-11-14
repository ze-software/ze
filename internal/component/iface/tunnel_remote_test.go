package iface

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/opererr"
)

// tunnelRemoteConfig builds an interface config with one tunnel t0 of the given
// kind, a local endpoint of the kind's family and the given remote ip, under
// the given backend.
func tunnelRemoteConfig(backend, kind, local, remote string) string {
	extra := ""
	if kind == "vxlan" {
		extra = `,"vni":"100"`
	}
	return `{"interface":{"backend":"` + backend + `","tunnel":{"t0":{"name":"t0","encapsulation":{"` +
		kind + `":{"local":{"ip":"` + local + `"},"remote":{"ip":"` + remote + `"}` + extra + `}}}}}}`
}

// tunnelRemoteKinds lists every tunnel kind with an address of its underlay
// family for the local endpoint, a specific remote, and its unspecified
// address.
var tunnelRemoteKinds = []struct {
	kind        string
	local       string
	remote      string
	unspecified string
}{
	{"gre", "192.0.2.1", "198.51.100.1", "0.0.0.0"},
	{"gretap", "192.0.2.1", "198.51.100.1", "0.0.0.0"},
	{"ipip", "192.0.2.1", "198.51.100.1", "0.0.0.0"},
	{"sit", "192.0.2.1", "198.51.100.1", "0.0.0.0"},
	{"vxlan", "192.0.2.1", "198.51.100.1", "0.0.0.0"},
	{"ip6gre", "2001:db8::1", "2001:db8::2", "::"},
	{"ip6gretap", "2001:db8::1", "2001:db8::2", "::"},
	{"ip6tnl", "2001:db8::1", "2001:db8::2", "::"},
	{"ipip6", "2001:db8::1", "2001:db8::2", "::"},
}

// TestTunnelVerifyRefusesAnUnspecifiedRemote drives the config verify entry
// point (verifyIfaceConfig, what OnConfigVerify and `ze config validate` call)
// with every tunnel kind and its unspecified remote, and checks the refusal is
// an operator error naming the address and the set command that fixes it.
//
// VALIDATES: the refusal happens at verify, for every kind, before a backend
// sees the config, so both backends and offline validation refuse alike.
// PREVENTS: a config with remote ip 0.0.0.0 or :: committing and failing only
// when the netlink backend applies it, or installing a tunnel that
// decapsulates from any source (RFC 4213 Section 3.6).
func TestTunnelVerifyRefusesAnUnspecifiedRemote(t *testing.T) {
	for _, tc := range tunnelRemoteKinds {
		t.Run(tc.kind, func(t *testing.T) {
			err := verifyIfaceConfig(gateSection(tunnelRemoteConfig("netlink", tc.kind, tc.local, tc.unspecified)))
			require.Error(t, err, "remote ip %s must be refused on a %s tunnel", tc.unspecified, tc.kind)
			var oe *opererr.Error
			require.True(t, errors.As(err, &oe), "refusal is not an operator error: %v", err)
			assert.Equal(t, "remote ip "+tc.unspecified+" is the unspecified address, not the address of a remote endpoint", oe.Problem())
			assert.Contains(t, oe.Why(), "accepts encapsulated packets from any source")
			assert.Equal(t, "set interface tunnel t0 encapsulation "+tc.kind+" remote ip <address>", oe.Fix())
			assert.Contains(t, err.Error(), `tunnel "t0"`)
		})
	}
}

// TestTunnelVerifyAcceptsASpecificRemote is the accepted counterpart: every
// kind with a specific remote, and the addresses next to the unspecified one
// (0.0.0.1, ::1), passes verify, so the refusal above cannot pass against a
// verifier that refuses every tunnel.
func TestTunnelVerifyAcceptsASpecificRemote(t *testing.T) {
	for _, tc := range tunnelRemoteKinds {
		t.Run(tc.kind, func(t *testing.T) {
			err := verifyIfaceConfig(gateSection(tunnelRemoteConfig("netlink", tc.kind, tc.local, tc.remote)))
			require.NoError(t, err)
		})
	}
	t.Run("sit_0.0.0.1", func(t *testing.T) {
		err := verifyIfaceConfig(gateSection(tunnelRemoteConfig("netlink", "sit", "192.0.2.1", "0.0.0.1")))
		require.NoError(t, err)
	})
	t.Run("ip6tnl_::1", func(t *testing.T) {
		err := verifyIfaceConfig(gateSection(tunnelRemoteConfig("netlink", "ip6tnl", "2001:db8::1", "::1")))
		require.NoError(t, err)
	})
}

// TestTunnelVerifyRefusesAnUnspecifiedRemoteUnderVPP checks that the refusal
// does not depend on the backend: a gre tunnel the vpp backend could program
// is refused at verify with the same operator error, and the same tunnel with
// a specific remote is accepted.
func TestTunnelVerifyRefusesAnUnspecifiedRemoteUnderVPP(t *testing.T) {
	err := verifyIfaceConfig(gateSection(tunnelRemoteConfig("vpp", "gre", "192.0.2.1", "0.0.0.0")))
	require.Error(t, err)
	var oe *opererr.Error
	require.True(t, errors.As(err, &oe), "refusal is not an operator error: %v", err)
	assert.Equal(t, "set interface tunnel t0 encapsulation gre remote ip <address>", oe.Fix())

	require.NoError(t, verifyIfaceConfig(gateSection(tunnelRemoteConfig("vpp", "gre", "192.0.2.1", "198.51.100.1"))))
}

// TestTunnelSpecCheckRemote covers the shapes of RemoteAddress the backends
// pass to CheckRemote directly: the IPv4-mapped unspecified address, the
// spelled-out IPv6 unspecified address, an address that does not parse, and
// the absent remote, which is left to the YANG mandatory statement and to the
// backends' own missing-remote refusal.
func TestTunnelSpecCheckRemote(t *testing.T) {
	refused := []struct {
		name   string
		remote string
		want   string
	}{
		{"v4-mapped-unspecified", "::ffff:0.0.0.0", "is the unspecified address"},
		{"v6-unspecified-spelled-out", "0:0:0:0:0:0:0:0", "is the unspecified address"},
		{"not-an-address", "not-an-ip", "is not an IP address"},
	}
	for _, tc := range refused {
		t.Run(tc.name, func(t *testing.T) {
			spec := TunnelSpec{Kind: TunnelKindSIT, Name: "t0", LocalAddress: "192.0.2.1", RemoteAddress: tc.remote}
			err := spec.CheckRemote()
			var oe *opererr.Error
			require.True(t, errors.As(err, &oe), "remote %q: want an operator error, got %v", tc.remote, err)
			assert.Contains(t, oe.Problem(), "remote ip "+tc.remote+" "+tc.want)
			assert.Equal(t, "set interface tunnel t0 encapsulation sit remote ip <address>", oe.Fix())
		})
	}
	t.Run("absent", func(t *testing.T) {
		spec := TunnelSpec{Kind: TunnelKindSIT, Name: "t0", LocalAddress: "192.0.2.1"}
		require.NoError(t, spec.CheckRemote())
	})
	t.Run("specific", func(t *testing.T) {
		spec := TunnelSpec{Kind: TunnelKindSIT, Name: "t0", LocalAddress: "192.0.2.1", RemoteAddress: "198.51.100.1"}
		require.NoError(t, spec.CheckRemote())
	})
}
