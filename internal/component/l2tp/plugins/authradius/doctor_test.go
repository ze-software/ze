// Design: docs/features/ai-first.md -- RADIUS server reachability doctor check tests

package l2tpauthradius

import (
	"context"
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
	udpReachable = func(string, []byte, net.IP, string, time.Duration) bool { return false }
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
	udpReachable = func(string, []byte, net.IP, string, time.Duration) bool { return false }
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
		respAuth := radius.ResponseAuthenticator(resp.Code, resp.Identifier, uint16(respLen), reqAuth, wire[radius.HeaderLen:respLen], secret)
		copy(wire[4:4+radius.AuthenticatorLen], respAuth[:])
		if _, writeErr := pc.WriteTo(wire[:respLen], addr); writeErr != nil {
			return
		}
	}()

	assert.True(t, udpServerReachable(addr, secret, nil, "ze-doctor", time.Second))
	require.NoError(t, pc.Close())
	<-done
	assert.False(t, udpServerReachable(addr, secret, nil, "ze-doctor", 10*time.Millisecond))
}
