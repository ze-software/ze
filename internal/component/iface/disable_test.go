// Design: docs/features/interfaces.md -- Interface reconciliation and application
// Related: config_apply.go -- forEachEnabledUnit, desiredState
// Related: register.go -- reconcileDHCP, dhcpDeclared, writtenRoutePriorities
// Related: reconcile_ra.go -- reconcileRA

package iface

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/ze"
)

// stoppableDHCPClient is a DHCP client that only remembers whether reconcile
// stopped it. Stopping is the whole product behavior under test here: the real
// client's worker removes the leased address and the lease's default route on
// its way out (runV4, internal/plugins/iface/dhcp/dhcp_v4_linux.go).
type stoppableDHCPClient struct {
	stopped bool
}

func (c *stoppableDHCPClient) Stop() { c.stopped = true }

// dhcpClientRecorder installs a DHCP client factory that records every client
// reconcile asks for, keyed "interface/unit", and restores the previous
// factory when the test ends.
type dhcpClientRecorder struct {
	created []string
	clients map[string]*stoppableDHCPClient
}

func newDHCPClientRecorder(t *testing.T) *dhcpClientRecorder {
	t.Helper()
	r := &dhcpClientRecorder{clients: make(map[string]*stoppableDHCPClient)}
	previous := dhcpClientFactory
	t.Cleanup(func() { dhcpClientFactory = previous })
	SetDHCPClientFactory(func(ifaceName, unit string, _ ze.EventBus, _, _ bool, _, _ string, _ int, _, _ string, _ bool, _ int) (DHCPStopper, error) {
		key := ifaceName + "/" + unit
		client := &stoppableDHCPClient{}
		r.created = append(r.created, key)
		r.clients[key] = client
		return client, nil
	})
	return r
}

// dhcpEth0Config builds the operator's config: one ethernet interface with one
// unit that asks for DHCPv4. entryBody and unitBody carry the `disable` leaf at
// the interface level and at the unit level, so the enabled control and the
// disabled case differ by that leaf alone.
func dhcpEth0Config(t *testing.T, entryBody, unitBody string) *ifaceConfig {
	t.Helper()
	return mustParseIfaceJSON(t, `{
		"interface": {
			"ethernet": {
				"eth0": {`+entryBody+`
					"unit": {
						"0": {`+unitBody+`
							"ipv4": {"dhcp": {"enabled": "true"}}
						}
					}
				}
			}
		}
	}`)
}

// TestDisableStopsTheDHCPClientAsWellAsTheConfiguredAddresses drives the
// operator's config through the parser into the reconcile that owns DHCP
// clients, which is where `disable` has to be read for the leaf to mean what it
// says.
//
// VALIDATES: `disable` stops every address an interface can take, not only the
// ones it declares. desiredState already programs none of a disabled
// interface's configured addresses; these cases hold the other half, the
// address a lease brings.
// PREVENTS: the 2026-09-03 finding in
// plan/journal/guard-added-to-one-half-of-a-pair.md. reconcileDHCP keyed only
// on dhcp.Enabled, so a disabled interface carrying `dhcp { enabled true }`
// started a client, completed a lease, and installed the leased address on a
// link the operator had taken out of service.
func TestDisableStopsTheDHCPClientAsWellAsTheConfiguredAddresses(t *testing.T) {
	log := slog.New(slog.DiscardHandler)

	t.Run("an enabled interface still gets its client", func(t *testing.T) {
		recorder := newDHCPClientRecorder(t)
		active := map[dhcpUnitKey]dhcpEntry{}

		reconcileDHCP(dhcpEth0Config(t, "", ""), nil, active, log)

		assert.Equal(t, []string{"eth0/0"}, recorder.created)
		assert.Len(t, active, 1)
	})

	t.Run("a disabled interface gets none", func(t *testing.T) {
		recorder := newDHCPClientRecorder(t)
		active := map[dhcpUnitKey]dhcpEntry{}

		reconcileDHCP(dhcpEth0Config(t, `"disable": [null],`, ""), nil, active, log)

		assert.Empty(t, recorder.created, "a disabled interface must start no DHCP client")
		assert.Empty(t, active)
	})

	t.Run("a disabled unit gets none", func(t *testing.T) {
		recorder := newDHCPClientRecorder(t)
		active := map[dhcpUnitKey]dhcpEntry{}

		reconcileDHCP(dhcpEth0Config(t, "", `"disable": [null],`), nil, active, log)

		assert.Empty(t, recorder.created, "a disabled unit must start no DHCP client")
		assert.Empty(t, active)
	})
}

// TestDisablingAnInterfaceStopsItsRunningDHCPClient covers the half that
// decides what happens on a live box: the operator disables an interface whose
// client already holds a lease.
//
// VALIDATES: the client stops at the first reconcile after the commit. Its
// worker then removes the address and the default route the lease installed
// (runV4, internal/plugins/iface/dhcp/dhcp_v4_linux.go, where every
// sleepOrStop returns false once Stop closes the stop channel), so the
// interface keeps neither.
// PREVENTS: a lease that outlives the instruction to stop using the interface,
// renewing for as long as the daemon runs.
func TestDisablingAnInterfaceStopsItsRunningDHCPClient(t *testing.T) {
	log := slog.New(slog.DiscardHandler)
	recorder := newDHCPClientRecorder(t)
	active := map[dhcpUnitKey]dhcpEntry{}

	reconcileDHCP(dhcpEth0Config(t, "", ""), nil, active, log)
	require.Len(t, active, 1, "the client has to be running before disabling it can mean anything")
	client := recorder.clients["eth0/0"]
	require.NotNil(t, client)

	reconcileDHCP(dhcpEth0Config(t, `"disable": [null],`, ""), nil, active, log)

	assert.True(t, client.stopped, "disabling the interface must stop the client that holds its lease")
	assert.Empty(t, active, "a stopped client must leave the tracking map")
}

// TestDHCPAutoStandsAsideForADisabledInterfacesDHCPBlock holds the dhcp-auto
// half of the same change. The reconcile's desired set no longer counts a
// disabled unit, and dhcp-auto used to read that set to decide whether the
// operator had configured DHCP at all.
//
// VALIDATES: yang/ze-iface-conf.yang, leaf dhcp-auto -- "Ze ignores the leaf
// when any explicit DHCP config exists". Disabling the interface that carries
// the config tells ze to run no client; it does not tell ze to find another
// NIC.
// PREVENTS: disabling an interface starting a DHCP client on a different one.
func TestDHCPAutoStandsAsideForADisabledInterfacesDHCPBlock(t *testing.T) {
	log := slog.New(slog.DiscardHandler)

	autoConfig := func(t *testing.T, body string) *ifaceConfig {
		t.Helper()
		return mustParseIfaceJSON(t, `{
			"interface": {
				"dhcp-auto": "true",
				"ethernet": {`+body+`}
			}
		}`)
	}

	t.Run("dhcp-auto reaches the discovered interface when nothing declares DHCP", func(t *testing.T) {
		fb := setupFakeBackendForTest(t)
		fb.ifaces["eth9"] = fakeIface{name: "eth9", linkType: "device"}
		recorder := newDHCPClientRecorder(t)
		active := map[dhcpUnitKey]dhcpEntry{}

		reconcileDHCP(autoConfig(t, ""), nil, active, log)

		require.Equal(t, []string{"eth9/default"}, recorder.created,
			"the control: without this, the case below proves nothing")
	})

	t.Run("a disabled interface's DHCP block still stands dhcp-auto down", func(t *testing.T) {
		fb := setupFakeBackendForTest(t)
		fb.ifaces["eth9"] = fakeIface{name: "eth9", linkType: "device"}
		recorder := newDHCPClientRecorder(t)
		active := map[dhcpUnitKey]dhcpEntry{}

		reconcileDHCP(autoConfig(t, `"eth0": {
			"disable": [null],
			"unit": {"0": {"ipv4": {"dhcp": {"enabled": "true"}}}}
		}`), nil, active, log)

		assert.Empty(t, recorder.created,
			"ze must run no client at all: not on the disabled interface, and not on a discovered one")
	})
}

// TestDisableStopsTheRouterAdvertisementSender covers the sibling service that
// read the same units through the same walker and made the same omission.
//
// VALIDATES: a disabled interface advertises ze as a router to nobody. RFC 4861
// Section 6.2.1 makes an advertising interface a router for the hosts on its
// link, which is the opposite of what `disable` tells the operator they get.
// PREVENTS: reconcileRA starting a sender for a unit whose interface is
// disabled, which the walker allowed by visiting every unit in the config
// rather than every unit ze must configure.
func TestDisableStopsTheRouterAdvertisementSender(t *testing.T) {
	log := slog.New(slog.DiscardHandler)
	raConfig := func(t *testing.T, entryBody string) *ifaceConfig {
		t.Helper()
		return mustParseIfaceJSON(t, `{
			"interface": {
				"ethernet": {
					"eth0": {`+entryBody+`
						"unit": {
							"0": {
								"ipv6": {
									"router-advertisement": {
										"enabled": "true",
										"prefix": {"2001:db8:1::/64": {}}
									}
								}
							}
						}
					}
				}
			}
		}`)
	}

	t.Run("an enabled interface still advertises", func(t *testing.T) {
		recorder := newRAFactoryRecorder(t)
		active := make(map[raUnitKey]raEntry)

		reconcileRA(raConfig(t, ""), active, log)

		assert.Len(t, recorder.started, 1)
	})

	t.Run("a disabled interface advertises nothing", func(t *testing.T) {
		recorder := newRAFactoryRecorder(t)
		active := make(map[raUnitKey]raEntry)

		reconcileRA(raConfig(t, `"disable": [null],`), active, log)

		assert.Empty(t, recorder.started, "a disabled interface must send no router advertisement")
		assert.Empty(t, active)
	})
}

// TestDisableStopsZeOwningTheInterfacesIPv6DefaultRoutes covers the third
// service that read route-priority off every unit in the config.
//
// VALIDATES: writtenRoutePriorities publishes only the interfaces ze manages.
// The map it returns is what makes suppressRAForConfig write
// accept_ra_defrtr=0 on an interface and what makes handleRouterDiscovered
// install a ::/0 route for it.
// PREVENTS: ze taking over the IPv6 default routes, and the kernel sysctl, of
// an interface the operator disabled.
func TestDisableStopsZeOwningTheInterfacesIPv6DefaultRoutes(t *testing.T) {
	priorityConfig := func(t *testing.T, entryBody, unitBody string) *ifaceConfig {
		t.Helper()
		return mustParseIfaceJSON(t, `{
			"interface": {
				"ethernet": {
					"eth0": {`+entryBody+`
						"unit": {"0": {`+unitBody+`"route-priority": "5"}}
					}
				}
			}
		}`)
	}

	assert.Equal(t, map[string]int{"eth0": 5}, writtenRoutePriorities(priorityConfig(t, "", "")),
		"an enabled interface still publishes the metric the operator wrote")
	assert.Empty(t, writtenRoutePriorities(priorityConfig(t, `"disable": [null],`, "")),
		"a disabled interface must not ask ze to own its default routes")
	assert.Empty(t, writtenRoutePriorities(priorityConfig(t, "", `"disable": [null],`)),
		"a disabled unit must not ask ze to own its default routes")
}
