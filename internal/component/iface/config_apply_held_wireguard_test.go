package iface

import (
	"errors"
	"os"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/report"
)

// The WireGuard writes the unprivileged backend refuses, as the kernel refuses
// an RTM_NEWLINK and a WG_CMD_SET_DEVICE without CAP_NET_ADMIN.
func (b *unprivilegedBackend) CreateWireguardDevice(name string) error {
	return refused("create wireguard", name)
}

func (b *unprivilegedBackend) ConfigureWireguardDevice(spec WireguardSpec) error {
	return refused("configure wireguard", spec.Name)
}

// wireguardTestSpec is the configured wg0: every field the write sets holds a
// value, one peer carries a preshared key, an endpoint, a keepalive and two
// allowed-IP families, and one carries only an allowed IP.
func wireguardTestSpec() WireguardSpec {
	return WireguardSpec{
		Name:          "wg0",
		PrivateKey:    wireguardTestKey(1),
		ListenPort:    51820,
		ListenPortSet: true,
		FirewallMark:  7,
		Peers: []WireguardPeerSpec{
			{
				Name:                "site2",
				PublicKey:           wireguardTestKey(2),
				PresharedKey:        wireguardTestKey(9),
				HasPresharedKey:     true,
				EndpointIP:          "192.0.2.2",
				EndpointPort:        51820,
				AllowedIPs:          []string{"10.0.0.0/24", "2001:db8::/64"},
				PersistentKeepalive: 25,
			},
			{Name: "site3", PublicKey: wireguardTestKey(3), AllowedIPs: []string{"10.0.1.0/24"}},
		},
	}
}

// wireguardTestConfig asks for wireguardTestSpec on wg0.
func wireguardTestConfig() *ifaceConfig {
	return &ifaceConfig{Wireguard: []wireguardEntry{{Name: "wg0", Spec: wireguardTestSpec()}}}
}

// wireguardHostBackend is a host whose wg0 already holds wireguardTestSpec and
// is administratively up, as ze left it before a restart. Each peer is named
// by its public key, as the netlink read names it.
func wireguardHostBackend() *fakeBackend {
	held := wireguardTestSpec()
	for i := range held.Peers {
		held.Peers[i].Name = held.Peers[i].PublicKey.String()
	}
	return &fakeBackend{
		ifaces: map[string]fakeIface{"wg0": {name: "wg0", linkType: zeTypeWireguard, index: 7, state: LinkStateUp}},
		wgHeld: map[string]WireguardSpec{"wg0": held},
	}
}

// TestApplyOverMatchingWireguardDeviceWritesNothing proves the first apply
// after start issues no WireGuard write for a device that already holds the
// configured keys and peers. Method: every write fails with EPERM, so any
// write the apply issues fails it. On the kernel backend the read of a
// WireGuard device needs CAP_NET_ADMIN as well (the kernel's GET_DEVICE is
// admin-only), so an unprivileged kernel ze still cannot apply such a config:
// TestApplyWireguardUnreadWithoutPrivilegeStillRefused covers that case.
//
// VALIDATES: a device that holds the config gets no ConfigureWireguardDevice,
// and the refused create of a device that existed is ignored, as before.
// PREVENTS: every start replacing the peer set of a device that already holds
// it, which drops each peer's session.
func TestApplyOverMatchingWireguardDeviceWritesNothing(t *testing.T) {
	resetAddressOwners(t)
	errs := applyConfig(wireguardTestConfig(), nil, &unprivilegedBackend{wireguardHostBackend()})
	require.Empty(t, errs, "nothing differs, so nothing is written")
}

// TestApplyWireguardWritesOnlyWhatDiffers proves the WireGuard compare in both
// directions, one field the write sets per case. Method: start from a device
// that holds the config, change one held or configured value, apply as the
// first apply after start, and count the configure writes.
//
// VALIDATES: private key, fwmark, listen port, the peer set (extra, missing,
// disabled and repeated peers), preshared key, keepalive, endpoint address
// and port, and the allowed-IP set are each written when they differ, and a
// spelling, an order or a kernel-picked port that the write would not change
// is not.
// PREVENTS: a compare that skips a needed write, and one that writes on every
// start.
func TestApplyWireguardWritesOnlyWhatDiffers(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(held, want *WireguardSpec)
		writes int
	}{
		{"all held", func(_, _ *WireguardSpec) {}, 0},
		{"listen port not configured, the kernel picked one", func(held, want *WireguardSpec) {
			want.ListenPortSet = false
			want.ListenPort = 0
			held.ListenPort = 40000
		}, 0},
		{"allowed ips in another order and with host bits", func(_, want *WireguardSpec) {
			want.Peers[0].AllowedIPs = []string{"2001:db8::1/64", "10.0.0.9/24"}
		}, 0},
		{"endpoint spelled as an ipv4-mapped ipv6 address", func(_, want *WireguardSpec) {
			want.Peers[0].EndpointIP = "::ffff:192.0.2.2"
		}, 0},
		{"peers reported in another order", func(held, _ *WireguardSpec) {
			slices.Reverse(held.Peers)
		}, 0},
		{"disabled peer absent from the device", func(_, want *WireguardSpec) {
			want.Peers = append(want.Peers, WireguardPeerSpec{Name: "old", PublicKey: wireguardTestKey(4), Disable: true})
		}, 0},
		{"private key", func(held, _ *WireguardSpec) { held.PrivateKey = wireguardTestKey(5) }, 1},
		{"fwmark", func(held, _ *WireguardSpec) { held.FirewallMark = 8 }, 1},
		{"fwmark on the device, none configured", func(_, want *WireguardSpec) { want.FirewallMark = 0 }, 1},
		{"listen port", func(held, _ *WireguardSpec) { held.ListenPort = 51821 }, 1},
		{"preshared key", func(held, _ *WireguardSpec) { held.Peers[0].PresharedKey = wireguardTestKey(8) }, 1},
		{"preshared key on the device, none configured", func(held, _ *WireguardSpec) {
			held.Peers[1].PresharedKey = wireguardTestKey(8)
			held.Peers[1].HasPresharedKey = true
		}, 1},
		{"keepalive", func(held, _ *WireguardSpec) { held.Peers[0].PersistentKeepalive = 0 }, 1},
		{"endpoint address", func(held, _ *WireguardSpec) { held.Peers[0].EndpointIP = "192.0.2.3" }, 1},
		{"endpoint port", func(held, _ *WireguardSpec) { held.Peers[0].EndpointPort = 51821 }, 1},
		{"endpoint learned, none configured", func(held, _ *WireguardSpec) {
			held.Peers[1].EndpointIP = "198.51.100.1"
			held.Peers[1].EndpointPort = 4000
		}, 1},
		{"allowed ip missing from the device", func(held, _ *WireguardSpec) {
			held.Peers[0].AllowedIPs = []string{"10.0.0.0/24"}
		}, 1},
		{"extra allowed ip on the device", func(held, _ *WireguardSpec) {
			held.Peers[1].AllowedIPs = append(held.Peers[1].AllowedIPs, "10.0.2.0/24")
		}, 1},
		{"allowed ip of another length", func(held, _ *WireguardSpec) {
			held.Peers[1].AllowedIPs = []string{"10.0.1.0/25"}
		}, 1},
		{"extra peer on the device", func(held, _ *WireguardSpec) {
			held.Peers = append(held.Peers, WireguardPeerSpec{PublicKey: wireguardTestKey(6)})
		}, 1},
		{"peer missing from the device", func(held, _ *WireguardSpec) { held.Peers = held.Peers[:1] }, 1},
		{"disabled peer still on the device", func(held, want *WireguardSpec) {
			want.Peers = append(want.Peers, WireguardPeerSpec{Name: "old", PublicKey: wireguardTestKey(4), Disable: true})
			held.Peers = append(held.Peers, WireguardPeerSpec{PublicKey: wireguardTestKey(4)})
		}, 1},
		{"one peer reported twice, another missing", func(held, _ *WireguardSpec) {
			held.Peers[1] = held.Peers[0]
		}, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetAddressOwners(t)
			b := wireguardHostBackend()
			cfg := wireguardTestConfig()
			held := b.wgHeld["wg0"]
			tt.mutate(&held, &cfg.Wireguard[0].Spec)
			b.wgHeld["wg0"] = held

			require.Empty(t, applyConfig(cfg, nil, b))
			assert.Equal(t, tt.writes, b.wgConfigCt["wg0"])
			if tt.writes > 0 {
				assert.Equal(t, cfg.Wireguard[0].Spec, b.wgConfigs["wg0"], "the write carries the configured spec")
			}
		})
	}
}

// TestApplyWritesAWireguardDeviceItCannotRead proves an unread WireGuard state
// is written, never taken for one that matches, and that the failed read
// reaches show doctor without key material. Method: wg0 holds the config but
// its WireGuard read fails; its link read answers in the same apply. Then the
// read answers and a fresh start applies again.
//
// VALIDATES: the write is issued; one doctor-iface-state-unread warning with
// subject "wg0 wireguard", the backend's answer and the show command, and no
// key in it; the link read of wg0 does not clear it; a read that answers
// clears it and finds nothing to write.
// PREVENTS: a failed read taken as "already holds", a warning a link read
// erases in the same apply, and a key in the report.
func TestApplyWritesAWireguardDeviceItCannotRead(t *testing.T) {
	resetAddressOwners(t)
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)

	b := wireguardHostBackend()
	b.getWireguardErr = map[string]error{"wg0": errors.New("wireguard read failed")}
	cfg := wireguardTestConfig()
	require.Empty(t, applyConfig(cfg, nil, b))
	assert.Equal(t, 1, b.wgConfigCt["wg0"], "an unread device is written")

	warnings := stateUnreadWarnings()
	require.Len(t, warnings, 1)
	assert.Equal(t, "wg0 wireguard", warnings[0].Subject)
	assert.Equal(t, "ze show interface name wg0 detail", warnings[0].Fix)
	assert.Contains(t, warnings[0].Message, "wireguard read failed", "the backend's answer is the cause")
	spec := wireguardTestSpec()
	for _, key := range []WireguardKey{spec.PrivateKey, spec.Peers[0].PresharedKey} {
		assert.NotContains(t, warnings[0].Message, key.String(), "no secret key in the report")
	}

	b.getWireguardErr = nil
	require.Empty(t, applyConfig(cfg, nil, b))
	assert.Equal(t, 1, b.wgConfigCt["wg0"], "the device holds the config: no second write")
	assert.Empty(t, stateUnreadWarnings(), "a read that answers clears the warning")
}

// TestApplyWireguardUnreadWithoutPrivilegeStillRefused proves the kernel
// case without CAP_NET_ADMIN: the read is refused as the write is, so the
// write follows and its refusal fails the apply with the capability named.
//
// VALIDATES: a refused read does not stop the write, and the write's EPERM
// keeps the CAP_NET_ADMIN remedy.
// PREVENTS: a refused read hiding a change ze could not make.
func TestApplyWireguardUnreadWithoutPrivilegeStillRefused(t *testing.T) {
	resetAddressOwners(t)
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)

	b := wireguardHostBackend()
	b.getWireguardErr = map[string]error{"wg0": refused("get wireguard", "wg0")}
	errs := applyConfig(wireguardTestConfig(), nil, &unprivilegedBackend{b})
	require.NotEmpty(t, errs, "an unread device is written, and the write needs CAP_NET_ADMIN")
	require.True(t, errors.Is(errors.Join(errs...), os.ErrPermission))
	require.ErrorContains(t, joinApplyErrors("interface apply", errs), "CAP_NET_ADMIN")
}

// TestApplyDoesNotReadAWireguardDeviceItCreates proves a device made by this
// apply is written without a read. Method: wg0 does not exist and its read
// would fail.
//
// VALIDATES: the device is created and written, and no warning is raised.
// PREVENTS: a warning on every start of the vpp backend, whose wireguard
// interface exists only once the configure write made it.
func TestApplyDoesNotReadAWireguardDeviceItCreates(t *testing.T) {
	resetAddressOwners(t)
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)

	b := &fakeBackend{
		ifaces:          map[string]fakeIface{},
		getWireguardErr: map[string]error{"wg0": errors.New("no such device")},
	}
	require.Empty(t, applyConfig(wireguardTestConfig(), nil, b))
	assert.True(t, b.created["wg0"])
	assert.Equal(t, 1, b.wgConfigCt["wg0"])
	assert.Empty(t, stateUnreadWarnings(), "a device created just now is not read")
}

// TestRollbackLeavesASkippedWireguardStepAlone proves a skipped WireGuard step
// records no undo. Method: a reload whose spec changed (fwmark 8 to 7) over a
// device that already holds the new spec, plus an address on wg0 that fails
// so the apply rolls back. A recorded undo would write the previous spec.
//
// VALIDATES: neither the apply nor its rollback writes the device.
// PREVENTS: a failed commit writing over a device it did not change.
func TestRollbackLeavesASkippedWireguardStepAlone(t *testing.T) {
	resetAddressOwners(t)
	b := wireguardHostBackend()
	b.addAddressErr = map[string]error{addressErrKey("wg0", "10.9.0.1/24"): errors.New("address refused")}
	previous := wireguardTestConfig()
	previous.Wireguard[0].Spec.FirewallMark = 8
	cfg := wireguardTestConfig()
	cfg.Wireguard[0].Units = []unitEntry{{Label: "0", Addresses: []string{"10.9.0.1/24"}}}

	require.NotEmpty(t, applyConfig(cfg, previous, b), "the refused address fails the apply")
	assert.Zero(t, b.wgConfigCt["wg0"], "the device held the new spec, so nothing is written or undone")
}
