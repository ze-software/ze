// Design: docs/features/ai-first.md -- RADIUS server reachability doctor check tests

package l2tpauthradius

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/component/radius"
)

func TestCheckRADIUSServers(t *testing.T) {
	// VALIDATES: AC-7 unreachable RADIUS servers return doctor-radius-unreachable.
	// PREVENTS: L2TP authentication failures surfacing only on subscriber login.
	oldProbe := udpReachable
	udpReachable = func(radius.Server, net.IP, string, time.Duration) error { return errors.New("probe: no answer") }
	t.Cleanup(func() { udpReachable = oldProbe })

	tree := config.NewTree()
	l2tpBlock := tree.GetOrCreateContainer("l2tp")
	auth := l2tpBlock.GetOrCreateContainer("auth")
	radiusBlock := auth.GetOrCreateContainer("radius")
	server := config.NewTree()
	server.Set("address", "radius.example.invalid")
	server.Set("port", "1812")
	server.Set("shared-key", "testing123")
	radiusBlock.AddListEntry("server", "primary", server)

	diags := checkRADIUSServers(registry.DoctorCheckContext{Tree: tree})
	require.Len(t, diags, 1)
	assert.Equal(t, "doctor-radius-unreachable", diags[0].Code)
	assert.Equal(t, "warning", diags[0].Severity)
	assert.Equal(t, "none of the configured RADIUS servers are reachable", diags[0].Message)
}

func TestCheckRADIUSServersAbsentConfig(t *testing.T) {
	// VALIDATES: the check fires only when the l2tp.auth.radius block is present.
	// PREVENTS: doctor warning about RADIUS on boxes that do not use it.
	oldProbe := udpReachable
	udpReachable = func(radius.Server, net.IP, string, time.Duration) error { return errors.New("probe: no answer") }
	t.Cleanup(func() { udpReachable = oldProbe })

	assert.Empty(t, checkRADIUSServers(registry.DoctorCheckContext{Tree: config.NewTree()}))
	assert.Empty(t, checkRADIUSServers(registry.DoctorCheckContext{Tree: nil}))
}

func TestRADIUSDoctorCheckRegistered(t *testing.T) {
	// VALIDATES: this plugin registers the l2tp-auth-radius-servers doctor check
	// so `ze doctor` runs it through the plugin registry, not a central call list.
	// PREVENTS: the removal test failing: deleting this plugin must delete the check.
	checks := registry.PluginDoctorChecks()
	found := false
	for _, c := range checks {
		if c.Name == "l2tp-auth-radius-servers" {
			found = true
			break
		}
	}
	assert.True(t, found,
		"doctor check l2tp-auth-radius-servers not registered via Registration.DoctorChecks")
}

func TestUDPServerReachableRequiresResponse(t *testing.T) {
	// VALIDATES: The RADIUS readiness probe requires an authenticated response instead of accepting Dial success.
	// PREVENTS: Unbound UDP ports or bad shared keys being reported as reachable.
	secret := []byte("testing123")
	var lc net.ListenConfig
	pc, err := lc.ListenPacket(context.Background(), "udp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := pc.LocalAddr().String()

	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 64)
		n, addr, readErr := pc.ReadFrom(buf)
		if readErr != nil || n < 20 {
			return
		}
		var reqAuth [radius.AuthenticatorLen]byte
		copy(reqAuth[:], buf[4:4+radius.AuthenticatorLen])
		resp := &radius.Packet{Code: radius.CodeAccessReject, Identifier: buf[1], Authenticator: reqAuth}
		wire := make([]byte, radius.MaxPacketLen)
		respLen, encodeErr := resp.EncodeTo(wire, 0)
		if encodeErr != nil {
			return
		}
		// The probe's server row requires the Message-Authenticator by
		// default, so the answer is signed as a BlastRADIUS-updated server signs.
		signed := signAccessAnswer(wire[:respLen], reqAuth, secret)
		respAuth := radius.ResponseAuthenticator(resp.Code, resp.Identifier, uint16(len(signed)), reqAuth, signed[radius.HeaderLen:], secret)
		copy(signed[4:4+radius.AuthenticatorLen], respAuth[:])
		if _, writeErr := pc.WriteTo(signed, addr); writeErr != nil {
			return
		}
	}()

	assert.NoError(t, udpServerReachable(radius.Server{Address: addr, SharedKey: secret}, nil, "ze-doctor", time.Second))
	require.NoError(t, pc.Close())
	<-done
	assert.Error(t, udpServerReachable(radius.Server{Address: addr, SharedKey: secret}, nil, "ze-doctor", 10*time.Millisecond))
}

// TestDoctorRadiusUnsignedResponse proves AC-15 for the l2tp tree.
//
// VALIDATES: a server that answers only unsigned yields
// doctor-radius-unsigned-response with the exact set command.
// PREVENTS: an operator reading "unreachable" for a server that answers.
func TestDoctorRadiusUnsignedResponse(t *testing.T) {
	oldProbe := udpReachable
	udpReachable = func(srv radius.Server, _ net.IP, _ string, _ time.Duration) error {
		if srv.AllowUnsignedResponse {
			return errors.New("probe: the leaf did not reach the probe")
		}
		return fmt.Errorf("radius: server %s: %w", srv.Address, radius.ErrUnsignedResponse)
	}
	t.Cleanup(func() { udpReachable = oldProbe })

	tree := config.NewTree()
	radiusBlock := tree.GetOrCreateContainer("l2tp").GetOrCreateContainer("auth").GetOrCreateContainer("radius")
	server := config.NewTree()
	server.Set("address", "192.0.2.1")
	server.Set("shared-key", "testing123")
	radiusBlock.AddListEntry("server", "primary", server)

	diags := checkRADIUSServers(registry.DoctorCheckContext{Tree: tree})
	require.Len(t, diags, 1)
	assert.Equal(t, "doctor-radius-unsigned-response", diags[0].Code)
	assert.Contains(t, diags[0].Message, "'set l2tp auth radius server primary require-message-authenticator false'")
}

// TestDoctorRadiusInvalidLeaf asks the l2tp doctor about a server row whose
// require-message-authenticator is neither true nor false.
//
// VALIDATES: the row is reported as doctor-radius-invalid, naming the value and
// the exact set command, and the server is never probed.
// PREVENTS: an operator reading "unreachable" and debugging the network for a
// config value.
func TestDoctorRadiusInvalidLeaf(t *testing.T) {
	oldProbe := udpReachable
	udpReachable = func(radius.Server, net.IP, string, time.Duration) error {
		t.Error("a server row with an invalid leaf was probed")
		return errors.New("probe: should not run")
	}
	t.Cleanup(func() { udpReachable = oldProbe })

	tree := config.NewTree()
	radiusBlock := tree.GetOrCreateContainer("l2tp").GetOrCreateContainer("auth").GetOrCreateContainer("radius")
	server := config.NewTree()
	server.Set("address", "192.0.2.1")
	server.Set("shared-key", "testing123")
	server.Set("require-message-authenticator", "maybe")
	radiusBlock.AddListEntry("server", "primary", server)

	diags := checkRADIUSServers(registry.DoctorCheckContext{Tree: tree})
	require.Len(t, diags, 1)
	assert.Equal(t, "doctor-radius-invalid", diags[0].Code)
	assert.Equal(t, "warning", diags[0].Severity)
	assert.Equal(t,
		`RADIUS server primary was not probed: require-message-authenticator is "maybe"; it takes true or false; `+
			`set it with 'set l2tp auth radius server primary require-message-authenticator true', `+
			`or false only for a server that cannot sign its answers`,
		diags[0].Message)
}
