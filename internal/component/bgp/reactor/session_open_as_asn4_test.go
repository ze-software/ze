// Design: docs/architecture/behavior/fsm-open-sent.md — OPEN acceptance
// Overview: session_open_as.go — validateOpenPeerAS and judgedPeerAS, under test here, over capability.OpenPeerAS
// Related: reactor_dynamic.go — resolveDynamicPeerSettings, the dynamic peer's AS at establishment
// Related: config.go — parsePeerSettings, which refuses an asn4-disabled four-octet remote
// RFC: rfc/short/rfc6793.md — the Four-octet AS capability replaces My AS only between NEW speakers

package reactor

import (
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	routepolicy "github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/bgp/capability"
)

// asn4TestPeerAS is the four-octet AS of the peer every test below talks to: one with no
// two-octet AS, so its OPEN carries My AS = AS_TRANS and the capability carries this.
const asn4TestPeerAS uint32 = 4200000000

// receivedASN4Open is the OPEN a NEW speaker in asn4TestPeerAS sends, decoded by
// UnpackOpen the way the session read path decodes it, so Open.ASN4 is left unset as it
// is in production.
func receivedASN4Open(t *testing.T) *message.Open {
	t.Helper()

	open, err := message.UnpackOpen(openBodyWithASN4Capability(asn4TestPeerAS))
	require.NoError(t, err)
	require.Zero(t, open.ASN4, "UnpackOpen leaves ASN4 unset; a fixture that sets it hides the defect")
	return open
}

// TestOpenAdvertisedASFollowsLocalASN4 pins the localASN4 flag at the two shared readers.
//
// VALIDATES: RFC 6793 Section 4.1 reads the capability "in lieu of" My AS only between NEW
// speakers. openAdvertisedAS answers the capability value only when ze advertises the
// capability too, My AS when it does not, and My AS for a peer that sends no capability
// whatever ze advertises. sessionPeerAS keeps a configured remote AS and otherwise
// answers what openAdvertisedAS does.
// PREVENTS: a reader that ignores the flag, which judged an asn4-disabled peer by a
// four-octet AS the session never negotiated.
func TestOpenAdvertisedASFollowsLocalASN4(t *testing.T) {
	withAS4 := receivedASN4Open(t)
	plain := &message.Open{Version: 4, MyAS: 65001, HoldTime: 90, BGPIdentifier: 0x01020304}

	assert.Equal(t, asn4TestPeerAS, openAdvertisedAS(withAS4, true), "NEW to NEW: the capability value")
	assert.Equal(t, uint32(message.AS_TRANS), openAdvertisedAS(withAS4, false), "ze is OLD: My AS")
	assert.Equal(t, uint32(65001), openAdvertisedAS(plain, true), "the peer is OLD: My AS")
	assert.Equal(t, uint32(65001), openAdvertisedAS(plain, false), "both OLD: My AS")

	assert.Equal(t, asn4TestPeerAS, sessionPeerAS(0, true, withAS4), "dynamic peer, NEW to NEW")
	assert.Equal(t, uint32(message.AS_TRANS), sessionPeerAS(0, false, withAS4), "dynamic peer, ze is OLD")
	assert.Equal(t, uint32(65001), sessionPeerAS(0, true, plain), "dynamic peer that sends no capability")
	assert.Equal(t, uint32(65010), sessionPeerAS(65010, true, withAS4), "a configured remote AS wins")
	assert.Zero(t, sessionPeerAS(0, true, nil), "no OPEN yet: unknown, never an AS")
}

// TestASN4DisabledSessionReadersJudgeMyAS drives every reader of a received OPEN's peer
// AS through its own caller, on a session with no configured remote AS (the dynamic
// peer's state before establishment), once with asn4 disabled and once enabled.
//
// VALIDATES: the collision tie-break (collisionPeerAS), the RFC 6286 identifier scope
// (validateOpenIdentifier), the negotiated identity (negotiateWith), the reload
// comparison's identity (judgedPeerAS, which negotiationOutcomeUnchanged calls) and the
// peer's identifier claim (claimPeerAS) all read My AS on the two-octet session and the
// capability on the four-octet one.
// PREVENTS: any one caller stating the local advertisement as a constant: with true at
// that caller the disabled arm reads 4200000000 and fails.
func TestASN4DisabledSessionReadersJudgeMyAS(t *testing.T) {
	tests := []struct {
		name      string
		advertise bool
		want      uint32
	}{
		{"asn4 disabled reads My AS", false, uint32(message.AS_TRANS)},
		{"asn4 enabled reads the capability", true, asn4TestPeerAS},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session, _ := openSentSessionASN4(t, 0, tt.advertise)
			open := receivedASN4Open(t)
			session.mu.Lock()
			session.peerOpen = open
			session.mu.Unlock()

			assert.Equal(t, tt.want, session.judgedPeerAS(open), "judgedPeerAS")
			assert.Equal(t, tt.want, session.collisionPeerAS(), "collisionPeerAS")

			session.mu.RLock()
			localOpen := session.localOpen
			session.mu.RUnlock()
			require.NotNil(t, localOpen, "Accept sent ze's OPEN")
			localCaps, err := capability.ParseFromOptionalParams(localOpen.OptionalParams, localOpen.ExtendedParams)
			require.NoError(t, err)
			peerCaps, err := capability.ParseFromOptionalParams(open.OptionalParams, open.ExtendedParams)
			require.NoError(t, err)
			session.negotiateWith(localCaps, peerCaps)
			require.NotNil(t, session.negotiated)
			assert.Equal(t, tt.want, session.negotiated.PeerASN, "negotiateWith identity")

			peer := NewPeer(session.settings)
			assert.Equal(t, tt.want, peer.claimPeerAS(open), "claimPeerAS")
		})
	}
}

// TestValidateOpenIdentifierScopesByJudgedAS drives validateOpenIdentifier where the AS
// it reads decides the verdict: ze is in asn4TestPeerAS with the identifier the peer
// presents, so the peer is internal only if it is judged to be in asn4TestPeerAS.
//
// VALIDATES: on an asn4-disabled session the peer is judged AS_TRANS, an external peer,
// so RFC 6286 Section 2.2's internal-peer identifier rule does not apply and the OPEN is
// accepted; on an asn4-enabled one it is internal and refused with Bad BGP Identifier.
// PREVENTS: validateOpenIdentifier reading the capability on a session that never
// negotiated it.
func TestValidateOpenIdentifierScopesByJudgedAS(t *testing.T) {
	const peerIdentifier uint32 = 0x0A000001 // openBodyWithASN4Capability's identifier

	for _, advertise := range []bool{false, true} {
		session, _ := openSentSessionASN4(t, 0, advertise)
		session.settings.LocalAS = asn4TestPeerAS
		session.settings.RouterID = peerIdentifier

		err := session.validateOpenIdentifier(receivedASN4Open(t))

		if advertise {
			require.Error(t, err, "an internal peer presenting ze's own identifier is refused")
			continue
		}
		require.NoError(t, err, "an external peer may share ze's identifier")
	}
}

// TestResolveDynamicPeerSettingsReadsASN4Capability drives a dynamic peer's establishment
// resolution over the OPEN a four-octet speaker sends.
//
// VALIDATES: resolveDynamicPeerSettings publishes the AS the session judged: the
// capability value when ze advertises asn4, My AS (AS_TRANS) when it does not, and My AS
// for a peer that sends no capability. $remote_as in the filter chain resolves to the
// same AS.
// PREVENTS: the dead `open.ASN4 > 0` read, which UnpackOpen never satisfies, so every
// four-octet dynamic peer was published as AS 23456 while the OPEN checks and the
// negotiated identity judged it 4200000000.
func TestResolveDynamicPeerSettingsReadsASN4Capability(t *testing.T) {
	plain := &message.Open{Version: 4, MyAS: 65001, HoldTime: 90, BGPIdentifier: 0x01020304}

	tests := []struct {
		name      string
		advertise bool
		open      *message.Open
		want      uint32
		wantRef   string
	}{
		{"asn4 enabled reads the capability", true, receivedASN4Open(t), asn4TestPeerAS, "in-4200000000"},
		{"asn4 disabled reads My AS", false, receivedASN4Open(t), uint32(message.AS_TRANS), "in-23456"},
		{"a peer with no capability is its My AS", true, plain, 65001, "in-65001"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings := NewPeerSettings(netip.MustParseAddr("185.1.69.10"), 65000, 0, 0x01020304)
			settings.IsDynamic = true
			settings.DisableASN4 = !tt.advertise
			settings.ImportPolicy = mustPolicy(t, []routepolicy.Ref{{Name: "in-$remote_as"}}, routepolicy.FinalAccept)

			peer := NewPeer(settings)
			session := NewSession(settings)
			session.mu.Lock()
			session.peerOpen = tt.open
			session.mu.Unlock()

			peer.resolveDynamicPeerSettings(session)

			assert.Equal(t, tt.want, peer.PeerAS(), "the published PeerAS")
			filters := peer.ImportPolicy().Filters()
			require.Len(t, filters, 1)
			assert.Equal(t, tt.wantRef, filters[0].Name, "$remote_as resolves to the judged AS")
		})
	}
}

// TestConfigRefusesASN4DisableWithFourOctetRemote drives the config parser over the
// `asn4 disable` shapes.
//
// VALIDATES: a peer configured `capability { asn4 disable; }` with a four-octet remote AS
// is refused at load with ErrASN4DisabledFourOctetRemote, because the session judges the
// peer by the two-octet My AS and so can never establish; remote 23456 with asn4 disable,
// and the four-octet remote with asn4 enabled, are accepted; so is a dynamic group, which
// accepts any AS.
// PREVENTS: a config that loads cleanly and then refuses every OPEN with Bad Peer AS.
func TestConfigRefusesASN4DisableWithFourOctetRemote(t *testing.T) {
	peerTree := func(remote, asn4 string) map[string]any {
		return map[string]any{
			"connection": map[string]any{"remote": map[string]any{"ip": "10.0.0.1"}, "local": map[string]any{"ip": "auto"}},
			"session": map[string]any{
				"asn":        map[string]any{"remote": remote},
				"capability": map[string]any{"asn4": asn4},
			},
		}
	}

	_, err := parsePeerFromTree("peer1", peerTree("4200000000", "disable"), 65000, 0)
	require.ErrorIs(t, err, ErrASN4DisabledFourOctetRemote)
	assert.Contains(t, err.Error(), "peer peer1", "the refusal names the peer")

	ps, err := parsePeerFromTree("peer1", peerTree("23456", "disable"), 65000, 0)
	require.NoError(t, err, "remote AS_TRANS is what a two-octet session judges a four-octet peer by")
	assert.True(t, ps.DisableASN4)

	_, err = parsePeerFromTree("peer1", peerTree("4200000000", "enable"), 65000, 0)
	require.NoError(t, err, "with the capability the four-octet AS is negotiated")

	group := map[string]any{
		"connection": map[string]any{"local": map[string]any{"ip": "auto"}},
		"session":    map[string]any{"capability": map[string]any{"asn4": "disable"}},
	}
	_, err = ParseDynamicGroupTemplate("group1", group, 65000, 0x01020304)
	require.NoError(t, err, "a dynamic group configures no remote AS")
}

// TestCheckASN4DisabledReachableFollowsPeerASAccepted holds the refusal to the AS set
// peerASAccepted accepts, including the RFC 7705 Section 4.2 pair.
//
// VALIDATES: a migrating session is refused only when neither of its two ASNs fits two
// octets, and accepted when one does, because a peer may present either.
// PREVENTS: the refusal drifting from the OPEN rule and refusing a session that could
// establish.
func TestCheckASN4DisabledReachableFollowsPeerASAccepted(t *testing.T) {
	tests := []struct {
		name        string
		localAS     uint32
		peerAS      uint32
		migrationAS uint32
		wantErr     bool
	}{
		{"four-octet remote", 65000, asn4TestPeerAS, 0, true},
		{"two-octet remote", 65000, 65001, 0, false},
		{"dynamic", 65000, 0, 0, false},
		{"migration pair both four-octet", asn4TestPeerAS, asn4TestPeerAS, 4200000001, true},
		{"migration pair with a two-octet legacy AS", asn4TestPeerAS, asn4TestPeerAS, 65010, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), tt.localAS, tt.peerAS, 0x0A000002)
			settings.DisableASN4 = true
			settings.MigrationAS = tt.migrationAS

			err := checkASN4DisabledReachable(settings)
			if tt.wantErr {
				require.ErrorIs(t, err, ErrASN4DisabledFourOctetRemote)
				settings.DisableASN4 = false
				require.NoError(t, checkASN4DisabledReachable(settings), "asn4 enabled is never refused")
				return
			}
			require.NoError(t, err)
		})
	}
}

// openSentSessionASN4 is openSentSessionAS with the Four-octet AS advertisement chosen by
// the caller. With advertise false the session is built the way `capability { asn4
// disable; }` builds it (parseCapabilitiesFromTree sets DisableASN4), so the OPEN ze
// sends carries no code-65 capability and ze is an OLD speaker toward this peer.
func openSentSessionASN4(t *testing.T, peerAS uint32, advertise bool) (*Session, chan []byte) {
	t.Helper()

	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, peerAS, 0x0A000002)
	settings.Connection = ConnectionPassive
	settings.ReceiveHoldTime = 90 * time.Second
	settings.DisableASN4 = !advertise
	settings.Capabilities = []capability.Capability{
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
	}

	session := NewSession(settings)
	require.NoError(t, session.Start())

	client, server := net.Pipe()
	written := make(chan []byte, 8)
	go func() {
		for {
			buf := make([]byte, 4096)
			n, err := client.Read(buf)
			if err != nil {
				close(written)
				return
			}
			written <- buf[:n]
		}
	}()

	require.NoError(t, session.Accept(server))
	require.Equal(t, fsm.StateOpenSent, session.State())

	t.Cleanup(func() {
		client.Close() //nolint:errcheck // test cleanup
		server.Close() //nolint:errcheck // test cleanup
	})

	return session, written
}

// TestOpenPeerASIsMyASWhenZeDoesNotAdvertiseASN4 drives handleOpen with the OPEN a NEW
// speaker in AS 4200000000 sends (My AS = AS_TRANS, capability value 4200000000) to a
// session ze holds two-octet.
//
// VALIDATES: RFC 6793 Section 4.1 uses the capability value "in lieu of" My AS only
// "When a NEW BGP speaker processes an OPEN message from another NEW BGP speaker". With
// the capability not advertised ze is an OLD speaker toward the peer, so the peer AS is
// the My Autonomous System field, AS_TRANS, and a session configured for remote 23456
// accepts it.
//
// PREVENTS: validateOpenPeerAS reading the capability whatever ze advertised, which sent
// NOTIFICATION 2/2 Bad Peer AS to BIRD in the interop scenario
// bgp-remove-private-as-as4path-frr.
//
// The other cases hold the gate to the local advertisement: a NEW session still judges by
// the capability, and a two-octet session does not accept the four-octet AS it never
// negotiated.
func TestOpenPeerASIsMyASWhenZeDoesNotAdvertiseASN4(t *testing.T) {
	const peerAS4 uint32 = 4200000000

	tests := []struct {
		name       string
		peerAS     uint32
		advertise  bool
		wantReject bool
	}{
		{"two-octet session accepts AS_TRANS", message.AS_TRANS, false, false},
		{"two-octet session refuses the unnegotiated four-octet AS", peerAS4, false, true},
		{"four-octet session accepts the capability AS", peerAS4, true, false},
		{"four-octet session refuses AS_TRANS", message.AS_TRANS, true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session, written := openSentSessionASN4(t, tt.peerAS, tt.advertise)

			err := session.handleOpen(openBodyWithASN4Capability(peerAS4))

			if !tt.wantReject {
				require.NoError(t, err, "the peer AS this session is configured for must be accepted")
				assert.Equal(t, fsm.StateOpenConfirm, session.State(), "the session advances")
				code, subcode, found := notificationFrom(t, written)
				assert.False(t, found,
					"an accepted OPEN must draw no NOTIFICATION, got %d/%d", code, subcode)
				return
			}

			require.Error(t, err)
			assert.ErrorIs(t, err, ErrPeerASMismatch)
			code, subcode, found := notificationFrom(t, written)
			require.True(t, found, "RFC 4271 Section 6.2 requires a NOTIFICATION")
			assert.Equal(t, message.NotifyOpenMessage, message.NotifyErrorCode(code),
				"error code must be OPEN Message Error")
			assert.Equal(t, message.NotifyOpenBadPeerAS, subcode, "subcode must be Bad Peer AS")
		})
	}
}
