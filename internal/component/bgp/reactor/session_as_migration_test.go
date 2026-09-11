// Design: session_as_migration.go — RFC 7705 Section 4.2 Internal BGP AS Migration
// Related: session_open_as_test.go — the Bad Peer AS rail these cases ride
// RFC: rfc/short/rfc7705.md — one iBGP session under either of two ASNs

package reactor

import (
	"net"
	"net/netip"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/capability"
)

// The AS numbers every case below is written in, named once so a reader can follow which
// of the two a given OPEN is speaking under. They are the RFC 5398 documentation range.
const (
	migrationLocalAS    = 64500 // the permanently retained ASN, this speaker's LocalAS
	migrationLegacyAS   = 64510 // the ASN being retired, configured as MigrationAS
	migrationStrangerAS = 64496 // neither of the two, so no session may establish under it
)

// migrationSession builds a session in OpenSent whose settings carry the RFC 7705
// Section 4.2 pair, and returns the messages ze writes.
//
// migrationAS of 0 configures no migration, which is the control every negative arm below
// runs against.
func migrationSession(t *testing.T, localAS, peerAS, migrationAS uint32) (*Session, chan []byte) {
	t.Helper()

	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), localAS, peerAS, 0x0A000002)
	settings.Connection = ConnectionPassive
	settings.ReceiveHoldTime = 90 * time.Second
	settings.Capabilities = []capability.Capability{
		&capability.ASN4{ASN: localAS},
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
	}
	require.NoError(t, setMigrationAS(settings, migrationAS))

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

// TestMigrationAcceptsEitherASN drives handleOpen, the rail every non-colliding connection
// takes, with a peer presenting each of the two AS numbers a migrating iBGP session runs
// under, and with a third AS that is neither.
//
// RFC requirement: RFC7705-4.2-2 positive -- a peer configured with the mechanism whose
// OPEN carries the globally configured ASN, and the same peer whose OPEN carries the
// locally configured ASN, are each accepted and the session advances to OpenConfirm.
// RFC requirement: RFC7705-4.2-2 negative -- an OPEN carrying a third AS is refused with
// NOTIFICATION 2/2 Bad Peer AS, and the same third AS is refused on a session that
// configures no migration AS. Without this arm the positive arm would pass against code
// that accepted every OPEN, which is what ze did before validateOpenPeerAS compared the
// advertised AS to the configuration at all.
//
// VALIDATES: AC-4, AC-5, AC-1.
// PREVENTS: "accept either ASN" being satisfied by accepting ANY ASN, which is how the
// requirement read against the no-check baseline.
func TestMigrationAcceptsEitherASN(t *testing.T) {
	tests := []struct {
		name        string
		migrationAS uint32
		advertised  uint16
		wantReject  bool
	}{
		{"globally configured asn", migrationLegacyAS, migrationLocalAS, false},
		{"locally configured asn", migrationLegacyAS, migrationLegacyAS, false},
		{"a third asn is still refused", migrationLegacyAS, migrationStrangerAS, true},
		{"no migration configured, the configured asn is accepted", 0, migrationLocalAS, false},
		{"no migration configured, the legacy asn is refused", 0, migrationLegacyAS, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// PeerAS is the retained ASN: this is an iBGP session, and the peer is the
			// one still to be renumbered.
			session, written := migrationSession(t, migrationLocalAS, migrationLocalAS, tt.migrationAS)

			err := session.handleOpen(openBodyWithIdentifier(tt.advertised, 0x0A000001))

			if !tt.wantReject {
				require.NoError(t, err, "an AS this session is configured for must be accepted")
				assert.Equal(t, fsm.StateOpenConfirm, session.State(), "the session advances")
				code, subcode, found := notificationFrom(t, written)
				assert.False(t, found,
					"an accepted OPEN must draw no NOTIFICATION, got %d/%d", code, subcode)
				return
			}

			require.Error(t, err)
			assert.ErrorIs(t, err, ErrPeerASMismatch)
			assert.NotEqual(t, fsm.StateOpenConfirm, session.State(),
				"a refused OPEN must not advance the FSM")

			code, subcode, found := notificationFrom(t, written)
			require.True(t, found, "RFC 4271 Section 6.2 requires a NOTIFICATION")
			assert.Equal(t, message.NotifyOpenMessage, message.NotifyErrorCode(code),
				"error code must be OPEN Message Error")
			assert.Equal(t, message.NotifyOpenBadPeerAS, subcode, "subcode must be Bad Peer AS")
		})
	}
}

// TestMigrationOpenCarriesResolvedASN reads the OPEN ze BUILDS, in both fields that can
// carry an AS, for each state of the Section 4.2 fallback.
//
// RFC requirement: RFC7705-4.2-3 positive -- a router configured with the mechanism sends
// its OPEN under the globally configured ASN before any rejection, and under the locally
// configured ASN once the peer has answered Bad Peer AS. The header's My Autonomous System
// and the Four-octet AS capability name the same AS in both states.
// RFC requirement: RFC7705-4.2-3 negative -- a session that configures no migration AS
// sends the local AS whatever the fallback flag holds, so the mechanism changes no OPEN it
// was not configured on. Without this arm the positive arm would pass against code that
// sent the migration AS unconditionally.
//
// VALIDATES: AC-6, AC-9, AC-11, R-4.
// PREVENTS: the header AS and the capability AS disagreeing, which is what two separate
// resolutions produce.
func TestMigrationOpenCarriesResolvedASN(t *testing.T) {
	const fourOctetLocalAS = 196608
	const fourOctetLegacyAS = 196609

	tests := []struct {
		name        string
		localAS     uint32
		migrationAS uint32
		fallback    bool
		wantAS      uint32
		wantMyAS    uint16
	}{
		{"migration configured, no rejection yet", migrationLocalAS, migrationLegacyAS, false, migrationLocalAS, migrationLocalAS},
		{"migration configured, peer answered bad peer as", migrationLocalAS, migrationLegacyAS, true, migrationLegacyAS, migrationLegacyAS},
		{"no migration, flag set anyway", migrationLocalAS, 0, true, migrationLocalAS, migrationLocalAS},
		{"no migration, flag clear", migrationLocalAS, 0, false, migrationLocalAS, migrationLocalAS},
		// RFC 6793 Section 3: a four-octet AS rides in the capability and My AS carries
		// AS_TRANS. The migration AS narrows on exactly the same terms as the local AS.
		{"four-octet local as", fourOctetLocalAS, fourOctetLegacyAS, false, fourOctetLocalAS, 23456},
		{"four-octet migration as", fourOctetLocalAS, fourOctetLegacyAS, true, fourOctetLegacyAS, 23456},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), tt.localAS, tt.localAS, 0x0A000002)
			settings.ReceiveHoldTime = 90 * time.Second
			require.NoError(t, setMigrationAS(settings, tt.migrationAS))

			session := NewSession(settings)
			var flag atomic.Bool
			flag.Store(tt.fallback)
			session.asMigrationFallback = &flag

			open, err := session.buildOpen(settings, nil)
			require.NoError(t, err)

			assert.Equal(t, tt.wantMyAS, open.MyAS, "My Autonomous System")
			assert.Equal(t, tt.wantAS, open.ASN4, "the ASN4 field the encoder reads")

			caps, err := capability.ParseFromOptionalParams(open.OptionalParams, open.ExtendedParams)
			require.NoError(t, err)
			var capAS uint32
			for _, c := range caps {
				if as4, ok := c.(*capability.ASN4); ok {
					capAS = as4.ASN
				}
			}
			require.NotZero(t, capAS, "the OPEN must carry a Four-octet AS capability")
			assert.Equal(t, tt.wantAS, capAS,
				"the Four-octet AS capability must name the same AS as the header")
		})
	}
}

// TestMigrationOpenLocalASWithoutAPeer pins the one state openLocalAS reads a nil flag in.
//
// VALIDATES: a Session no Peer owns opens under the local AS, which is the state Section
// 4.2 asks for before any peer has answered Bad Peer AS.
// PREVENTS: reading the nil as "unknown" and inventing a different AS for it.
func TestMigrationOpenLocalASWithoutAPeer(t *testing.T) {
	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), migrationLocalAS, migrationLocalAS, 0x0A000002)
	require.NoError(t, setMigrationAS(settings, migrationLegacyAS))

	assert.Equal(t, uint32(migrationLocalAS), openLocalAS(settings, nil))
}

// TestMigrationSessionIsIBGP holds the verdict every AS-scoped decision reads.
//
// RFC requirement: RFC7705-4.2-4 positive -- a session whose configured remote AS is the
// migration AS while the local AS is the other of the pair is INTERNAL, so the eBGP
// AS_PATH prepend does not run and the RFC 7606 iBGP branch is taken.
// RFC requirement: RFC7705-4.2-4 negative -- the same pair of AS numbers with no migration
// AS configured is EXTERNAL. Without this arm the positive arm would pass against code
// that called every session internal.
//
// A remote AS outside the pair has no row here, because it is not a configuration this
// table can build: Section 4.2 widens "ours" to exactly two ASNs, so setMigrationAS
// refuses a third and the settings never reach a verdict. That refusal is asserted by
// TestMigrationConfigRefusesTheTwoShapesSection42CannotDescribe, and the third AS reaches
// the verdict the way a real one does, ADVERTISED, in
// TestMigrationIBGPVerdictAgreesAcrossEverySite.
//
// VALIDATES: AC-7.
// PREVENTS: a migrating session being iBGP for one decision and eBGP for another, which is
// what four copies of the equality produced.
func TestMigrationSessionIsIBGP(t *testing.T) {
	tests := []struct {
		name        string
		peerAS      uint32
		migrationAS uint32
		wantIBGP    bool
	}{
		{"peer on the retained asn", migrationLocalAS, migrationLegacyAS, true},
		{"peer still on the legacy asn", migrationLegacyAS, migrationLegacyAS, true},
		{"peer on the legacy asn with no migration configured", migrationLegacyAS, 0, false},
		{"peer on the retained asn with no migration configured", migrationLocalAS, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), migrationLocalAS, tt.peerAS, 0x0A000002)
			require.NoError(t, setMigrationAS(settings, tt.migrationAS))

			assert.Equal(t, tt.wantIBGP, settings.IsIBGP(), "IsIBGP")
			assert.Equal(t, !tt.wantIBGP, settings.IsEBGP(),
				"IsEBGP must be the negation of IsIBGP and never a second rule")

			// The observable, not the flag: forward facts gate the eBGP AS_PATH prepend
			// on this verdict, so a wrong verdict here is a wrong AS_PATH on the wire.
			assert.Equal(t, !tt.wantIBGP, localASPrependFor(settings).owed() && settings.IsEBGP(),
				"an internal session owes no eBGP prepend")
		})
	}
}

// TestMigrationIBGPVerdictAgreesAcrossEverySite is the assumption A-2 check: the sites that
// each held their own copy of the equality now answer identically for every combination.
//
// VALIDATES: A-2. The Peer method under its lock, the settings method, and the
// advertised-AS form the RFC 6286 rail uses all take one rule.
// PREVENTS: a site keeping the old equality, which makes a migrating session internal to
// the forward path and external to RFC 7606 at the same moment.
//
// A third AS with the mechanism on is not a configuration: setMigrationAS refuses it
// (TestMigrationConfigRefusesTheTwoShapesSection42CannotDescribe), because peerASAccepted
// would then never accept the OPEN the configured remote AS sends. The loop therefore
// asserts the refusal for that pair rather than a verdict, and the third AS reaches the
// rule the way a real one does: ADVERTISED, on a legal migration session, below.
func TestMigrationIBGPVerdictAgreesAcrossEverySite(t *testing.T) {
	for _, peerAS := range []uint32{migrationLocalAS, migrationLegacyAS, migrationStrangerAS} {
		for _, migrationAS := range []uint32{0, migrationLegacyAS} {
			settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), migrationLocalAS, peerAS, 0x0A000002)

			err := setMigrationAS(settings, migrationAS)
			if peerAS == migrationStrangerAS && migrationAS != 0 {
				require.ErrorIs(t, err, ErrMigrationASNotPeerAS,
					"a remote AS outside the pair is refused, not classified")
				continue
			}
			require.NoError(t, err)

			want := settings.IsIBGP()
			assert.Equal(t, want, settings.isIBGPWith(peerAS),
				"the advertised-AS form must agree for peer %d migration %d", peerAS, migrationAS)
			assert.Equal(t, !want, settings.IsEBGP(),
				"IsEBGP must agree for peer %d migration %d", peerAS, migrationAS)
		}
	}

	// The one rule reads an ADVERTISED AS on the RFC 6286 rail, and a peer can put any AS
	// there. A migrating session widens "ours" to exactly two numbers, so a third is
	// external, and the same call answers for every site that consumes the verdict.
	migrating := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), migrationLocalAS, migrationLegacyAS, 0x0A000002)
	require.NoError(t, setMigrationAS(migrating, migrationLegacyAS))
	assert.False(t, migrating.isIBGPWith(migrationStrangerAS),
		"an advertised AS outside the migrating pair is external")
	assert.True(t, migrating.isIBGPWith(migrationLocalAS), "the retained asn is internal")
	assert.True(t, migrating.isIBGPWith(migrationLegacyAS), "the legacy asn is internal")
}

// TestMigrationFallbackOnBadPeerAS drives the deadlock avoidance from the receiving side.
//
// VALIDATES: AC-10. A peer answering Bad Peer AS moves the next OPEN onto the other AS,
// and answering again moves it back, so two speakers that both run the mechanism reach the
// pairing that works rather than sitting on the one that does not.
// PREVENTS: a latching flag, which converts the deadlock Section 4.2 names into a second
// deadlock on the other AS.
func TestMigrationFallbackOnBadPeerAS(t *testing.T) {
	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), migrationLocalAS, migrationLocalAS, 0x0A000002)
	require.NoError(t, setMigrationAS(settings, migrationLegacyAS))

	session := NewSession(settings)
	var flag atomic.Bool
	session.asMigrationFallback = &flag

	badPeerAS := &message.Notification{
		ErrorCode:    message.NotifyOpenMessage,
		ErrorSubcode: message.NotifyOpenBadPeerAS,
	}

	require.Equal(t, uint32(migrationLocalAS), openLocalAS(settings, &flag),
		"before any rejection the globally configured ASN is sent first")

	session.noteASMigrationRejection(badPeerAS)
	assert.Equal(t, uint32(migrationLegacyAS), openLocalAS(settings, &flag),
		"after Bad Peer AS the next OPEN carries the locally configured ASN")

	session.noteASMigrationRejection(badPeerAS)
	assert.Equal(t, uint32(migrationLocalAS), openLocalAS(settings, &flag),
		"a second rejection returns to the other ASN rather than latching")

	// A different NOTIFICATION is not this mechanism's business.
	session.noteASMigrationRejection(&message.Notification{
		ErrorCode:    message.NotifyOpenMessage,
		ErrorSubcode: message.NotifyOpenBadBGPID,
	})
	assert.Equal(t, uint32(migrationLocalAS), openLocalAS(settings, &flag),
		"only Bad Peer AS moves the fallback")
}

// TestMigrationFallbackIgnoredWithoutTheMechanism pins the control: a peer that configures
// no migration AS never changes which AS its OPEN carries, whatever a peer answers.
//
// VALIDATES: AC-11.
// PREVENTS: the fallback reaching a session it was never configured on.
func TestMigrationFallbackIgnoredWithoutTheMechanism(t *testing.T) {
	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), migrationLocalAS, migrationStrangerAS, 0x0A000002)

	session := NewSession(settings)
	var flag atomic.Bool
	session.asMigrationFallback = &flag

	session.noteASMigrationRejection(&message.Notification{
		ErrorCode:    message.NotifyOpenMessage,
		ErrorSubcode: message.NotifyOpenBadPeerAS,
	})

	assert.False(t, flag.Load(), "a session with no migration AS records no fallback")
	assert.Equal(t, uint32(migrationLocalAS), openLocalAS(settings, &flag))
}

// TestOpenCheckSkippedForDynamicPeer holds the exemption A-4 names.
//
// VALIDATES: AC-3. A dynamic peer states no remote AS, so the Bad Peer AS comparison has
// nothing to compare and the session establishes as it did before the check existed.
// PREVENTS: the regression this exemption exists to stop -- every dynamic peer refused at
// OPEN, because 0 is UNKNOWN here and never an AS.
func TestOpenCheckSkippedForDynamicPeer(t *testing.T) {
	// PeerAS 0 is what buildDynamicPeerSettings leaves on the template.
	session, written := migrationSession(t, migrationLocalAS, 0, 0)

	err := session.handleOpen(openBodyWithIdentifier(migrationStrangerAS, 0x0A000001))

	require.NoError(t, err, "a dynamic peer has no configured AS to be checked against")
	assert.Equal(t, fsm.StateOpenConfirm, session.State())
	code, subcode, found := notificationFrom(t, written)
	assert.False(t, found, "no NOTIFICATION, got %d/%d", code, subcode)
}

// TestMigrationConfigRefusesTheTwoShapesSection42CannotDescribe holds the config validator.
//
// VALIDATES: the two refusals setMigrationAS owes, and that a legal pair is accepted.
// PREVENTS: a migration AS equal to the local AS, which reads as the mechanism being on
// and widens nothing; and a remote AS that is neither of the pair, which would make
// peerASAccepted widen past the two values Section 4.2 names.
func TestMigrationConfigRefusesTheTwoShapesSection42CannotDescribe(t *testing.T) {
	tests := []struct {
		name        string
		peerAS      uint32
		migrationAS uint32
		wantErr     error
	}{
		{"legal: remote is the retained asn", migrationLocalAS, migrationLegacyAS, nil},
		{"legal: remote is the legacy asn", migrationLegacyAS, migrationLegacyAS, nil},
		{"legal: a dynamic group states no remote asn", 0, migrationLegacyAS, nil},
		{"migration equals local", migrationLocalAS, migrationLocalAS, ErrMigrationASEqualsLocal},
		{"remote is neither of the pair", migrationStrangerAS, migrationLegacyAS, ErrMigrationASNotPeerAS},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), migrationLocalAS, tt.peerAS, 0x0A000002)

			err := setMigrationAS(settings, tt.migrationAS)

			if tt.wantErr == nil {
				require.NoError(t, err)
				assert.Equal(t, tt.migrationAS, settings.MigrationAS, "the leaf reaches the settings")
				return
			}
			require.Error(t, err)
			assert.ErrorIs(t, err, tt.wantErr)
			assert.Zero(t, settings.MigrationAS,
				"a refused config must leave the mechanism off rather than half on")
		})
	}
}
