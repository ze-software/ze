// Design: docs/architecture/ike/ipsec-10-cli-diag.md -- show vpn ipsec handler tests

package cmd

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/ike/crypto"
	"github.com/ze-software/ze/internal/component/ike/engine"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

func TestShowIPsecSA_RegisteredWireMethods(t *testing.T) {
	wanted := map[string]bool{
		"ze-show:vpn-ipsec-sa":     false,
		"ze-show:vpn-ipsec-status": false,
		"ze-show:vpn-ipsec-peer":   false,
	}
	for _, r := range pluginserver.AllBuiltinRPCs() {
		if _, ok := wanted[r.WireMethod]; ok {
			require.NotNil(t, r.Handler, "%s handler must not be nil", r.WireMethod)
			wanted[r.WireMethod] = true
		}
	}
	for wm, seen := range wanted {
		require.True(t, seen, "%s not registered via pluginserver.RegisterRPCs", wm)
	}
}

func TestShowIPsecSA_NoEngine(t *testing.T) {
	engine.SetActiveTableForTest(nil)
	resp, err := handleShowVPNIPsecSA(nil, nil)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusDone, resp.Status)
	m, ok := resp.Data.(plugin.Map)
	require.True(t, ok)
	rows, _ := m["peers"].([]map[string]any)
	assert.Empty(t, rows)
}

func TestShowIPsecSA_WithSAs(t *testing.T) {
	table := engine.NewSATable()
	sa := &engine.SA{
		PeerName:      "test-peer",
		State:         engine.StateEstablished,
		IsInitiator:   true,
		CreatedAt:     time.Now().Add(-10 * time.Minute),
		EstablishedAt: time.Now().Add(-5 * time.Minute),
		Proposal: crypto.IKEProposal{
			Encryption: crypto.EncryptionTransform{ID: crypto.ENCR_AES_CBC},
			Integrity:  crypto.IntegrityTransform{ID: crypto.AUTH_HMAC_SHA2_256_128},
			DHGroup:    crypto.DHGroupTransform{ID: crypto.DH_ECP_256},
		},
	}
	sa.InitiatorSPI = [8]byte{1, 2, 3, 4, 5, 6, 7, 8}
	sa.ResponderSPI = [8]byte{8, 7, 6, 5, 4, 3, 2, 1}
	table.Insert(sa)
	engine.SetActiveTableForTest(table)
	engine.SetActivePeersForTest(map[string]*engine.PeerSession{})
	defer func() {
		engine.SetActiveTableForTest(nil)
		engine.SetActivePeersForTest(nil)
	}()

	resp, err := handleShowVPNIPsecSA(nil, nil)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	m, ok := resp.Data.(plugin.Map)
	require.True(t, ok)
	rows, _ := m["peers"].([]map[string]any)
	require.Len(t, rows, 1)

	row := rows[0]
	assert.Equal(t, "test-peer", row["peer-name"])
	assert.Equal(t, "established", row["state"])
	assert.Equal(t, "0102030405060708", row["initiator-spi"])
	assert.Equal(t, "0807060504030201", row["responder-spi"])
	assert.Equal(t, true, row["is-initiator"])
	assert.Equal(t, "aes-cbc", row["encryption"])
	assert.Equal(t, "sha256", row["integrity"])
	assert.Equal(t, "ecp256", row["dh-group"])
	assert.Equal(t, false, row["nat-detected"])

	// No transport-mode selector set was read on this SA, so the two stored
	// pre-substitution addresses are absent. They answer null rather than "", because an
	// empty string reads as an address nobody holds.
	assert.Nil(t, row["original-tsi"])
	assert.Nil(t, row["original-tsr"])

	uptime, ok := row["uptime-seconds"].(float64)
	require.True(t, ok)
	assert.Greater(t, uptime, float64(0))
}

// TestShowIPsecSAReportsThePreSubstitutionSelectorAddresses drives the stored originals
// of RFC 7296 Section 2.23.1 through the show handler, which is the only entry point an
// operator has to them.
//
// VALIDATES: an SA carrying OriginalTSiAddr and OriginalTSrAddr reports both in the
// payload of `show vpn ipsec sa`, as text, beside the NAT verdict fields.
// PREVENTS: the two fields being written by the substitution and read by nothing, which
// leaves the operator with the programmed pair alone and no way to see what the peer put
// on the wire. Behind a NAT those are different addresses, which is the whole point of
// the section.
func TestShowIPsecSAReportsThePreSubstitutionSelectorAddresses(t *testing.T) {
	table := engine.NewSATable()
	sa := &engine.SA{
		PeerName:      "nat-peer",
		State:         engine.StateEstablished,
		IsInitiator:   true,
		CreatedAt:     time.Now().Add(-time.Minute),
		EstablishedAt: time.Now().Add(-time.Minute),
		NATDetected:   true,
		BehindNAT:     true,
		PeerBehindNAT: true,
		// The pair the peer answered with, in ITS address space. Neither address exists
		// on this node, which is why the substitution replaced them.
		OriginalTSiAddr: net.ParseIP("172.28.0.6"),
		OriginalTSrAddr: net.ParseIP("172.28.0.3"),
	}
	table.Insert(sa)
	engine.SetActiveTableForTest(table)
	engine.SetActivePeersForTest(map[string]*engine.PeerSession{})
	defer func() {
		engine.SetActiveTableForTest(nil)
		engine.SetActivePeersForTest(nil)
	}()

	resp, err := handleShowVPNIPsecSA(nil, nil)
	require.NoError(t, err)
	require.NotNil(t, resp)
	m, ok := resp.Data.(plugin.Map)
	require.True(t, ok)
	rows, _ := m["peers"].([]map[string]any)
	require.Len(t, rows, 1)

	row := rows[0]
	assert.Equal(t, true, row["behind-nat"])
	assert.Equal(t, true, row["peer-behind-nat"])
	assert.Equal(t, "172.28.0.6", row["original-tsi"])
	assert.Equal(t, "172.28.0.3", row["original-tsr"])
}

func TestShowIPsecStatus_WithSAs(t *testing.T) {
	table := engine.NewSATable()
	sa := &engine.SA{PeerName: "peer-a", State: engine.StateEstablished}
	sa.InitiatorSPI = [8]byte{1, 2, 3, 4, 5, 6, 7, 8}
	table.Insert(sa)
	engine.SetActiveTableForTest(table)
	engine.SetActivePeersForTest(map[string]*engine.PeerSession{})
	defer func() {
		engine.SetActiveTableForTest(nil)
		engine.SetActivePeersForTest(nil)
	}()

	resp, err := handleShowVPNIPsecStatus(nil, nil)
	require.NoError(t, err)
	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok)
	assert.Equal(t, true, data["engine-running"])
	assert.Equal(t, 1, data["active-ike-sas"])
	assert.Equal(t, 1, data["established-sas"])
}

func TestShowIPsecStatus_NoEngine(t *testing.T) {
	engine.SetActiveTableForTest(nil)
	engine.SetActivePeersForTest(nil)
	resp, err := handleShowVPNIPsecStatus(nil, nil)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusDone, resp.Status)
	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok)
	assert.Equal(t, false, data["engine-running"])
}

func TestShowIPsecPeer_Found(t *testing.T) {
	table := engine.NewSATable()
	sa := &engine.SA{PeerName: "mgmt-peer", State: engine.StateEstablished}
	sa.InitiatorSPI = [8]byte{0xaa, 0xbb, 0xcc, 0xdd, 0, 0, 0, 1}
	table.Insert(sa)
	engine.SetActiveTableForTest(table)
	engine.SetActivePeersForTest(map[string]*engine.PeerSession{})
	defer func() {
		engine.SetActiveTableForTest(nil)
		engine.SetActivePeersForTest(nil)
	}()

	resp, err := handleShowVPNIPsecPeer(nil, []string{"mgmt-peer"})
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)
	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok)
	assert.Equal(t, "mgmt-peer", data["peer-name"])
	sas, ok := data["ike-sas"].([]map[string]any)
	require.True(t, ok)
	require.Len(t, sas, 1)
}

func TestShowIPsecPeer_MissingArg(t *testing.T) {
	resp, err := handleShowVPNIPsecPeer(nil, nil)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusError, resp.Status)
	msg := resp.Error
	assert.Contains(t, msg, "usage")
}

func TestShowIPsecPeer_NoEngine(t *testing.T) {
	engine.SetActiveTableForTest(nil)
	resp, err := handleShowVPNIPsecPeer(nil, []string{"test-peer"})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusError, resp.Status)
}

func TestShowIPsecPeer_InvalidName(t *testing.T) {
	resp, err := handleShowVPNIPsecPeer(nil, []string{""})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusError, resp.Status)
}

func TestShowIPsecPeer_NotFound(t *testing.T) {
	table := engine.NewSATable()
	engine.SetActiveTableForTest(table)
	defer engine.SetActiveTableForTest(nil)

	resp, err := handleShowVPNIPsecPeer(nil, []string{"nonexistent"})
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusError, resp.Status)
	msg := resp.Error
	assert.Contains(t, msg, "peer not found")
}
