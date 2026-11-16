//go:build ze_bgp

// Design: docs/guide/config-reload.md -- unchanged configuration preserves BGP identity.
// Related: reload_globals.go -- unchanged global defaults require no apply.
package reactor

import (
	"bufio"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
)

// TestRFC6286DefaultIdentifierSurvivesUnchangedReload reapplies the same native
// configuration through ApplyConfigDiff. It retains both Peer objects and the
// startup global identifier, then observes the OPENs their settings produce.
// An explicitly changed identifier is separately preserved by
// TestReloadAppliesGlobalRouterID and reload-global-router-id.ci.
//
// RFC requirement: RFC6286-2.1-1 positive -- without identifier overrides or an
// operator-requested identifier change, an unchanged configuration reload keeps
// the startup global identifier and both configured peers; a fresh Session for
// either peer transmits that same identifier in its OPEN.
func TestRFC6286DefaultIdentifierSurvivesUnchangedReload(t *testing.T) {
	const identifier uint32 = 0x01020304
	r := New(&Config{ListenAddr: "127.0.0.1:0", Standalone: true, RouterID: identifier, LocalAS: 65000})
	r.SetTreeReloadFunc(testTreeReload(t))
	require.NoError(t, r.Start())
	defer r.Stop()
	adapter := &reactorAPIAdapter{r: r}
	require.NoError(t, adapter.ApplyConfigDiff(rfc6286UnchangedReloadTree(t)))
	require.Equal(t, identifier, r.Stats().RouterID)
	first := peerByAddr(t, r, "192.0.2.1")
	second := peerByAddr(t, r, "192.0.2.2")

	require.NoError(t, adapter.ApplyConfigDiff(rfc6286UnchangedReloadTree(t)))
	require.Equal(t, identifier, r.Stats().RouterID)
	require.Len(t, r.Peers(), 2)
	require.Same(t, first, peerByAddr(t, r, "192.0.2.1"))
	require.Same(t, second, peerByAddr(t, r, "192.0.2.2"))
	for _, peer := range []*Peer{first, second} {
		session := NewSession(peer.settingsSnapshot())
		t.Cleanup(func() { require.NoError(t, session.Stop()) })
		conn := &recordingConn{}
		session.bufWriter = bufio.NewWriter(conn)
		require.NoError(t, session.sendOpen(conn))
		wire := conn.written()
		require.GreaterOrEqual(t, len(wire), message.HeaderLen)
		header, err := message.ParseHeader(wire[:message.HeaderLen])
		require.NoError(t, err)
		require.Equal(t, msgtype.TypeOPEN, header.Type)
		require.Equal(t, int(header.Length), len(wire))
		open, err := message.UnpackOpen(wire[message.HeaderLen:])
		require.NoError(t, err)
		require.Equal(t, identifier, open.BGPIdentifier)
	}
}

// rfc6286UnchangedReloadTree uses the side-effect-free startup loader. Both
// peers are passive so this test owns no outbound connection attempt.
func rfc6286UnchangedReloadTree(t *testing.T) map[string]any {
	t.Helper()
	loaded, err := config.ReadConfig(`bgp {
    router-id 1.2.3.4;
    session { asn { local 65000; } }
    peer first {
        connection { local { ip auto; } remote { ip 192.0.2.1; connect false; } }
        session { asn { remote 65001; } }
    }
    peer second {
        connection { local { ip auto; } remote { ip 192.0.2.2; connect false; } }
        session { asn { remote 65002; } }
    }
}`, "", nil)
	require.NoError(t, err)
	return loaded.Tree.ToMap()
}
