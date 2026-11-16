//go:build ze_bgp

// Design: docs/guide/configuration.md -- one global BGP Identifier by default and explicit overrides.
// Related: ../config/resolve.go -- global, group and peer inheritance.
// Related: session_negotiate.go -- the configured identifier in the transmitted OPEN.
package reactor_test

import (
	"bytes"
	"fmt"
	"net"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	bgpconfig "github.com/ze-software/ze/internal/component/bgp/config"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/reactor"
	"github.com/ze-software/ze/internal/component/config"
	_ "github.com/ze-software/ze/internal/component/plugin/all"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
)

// TestRFC6286DefaultIdentifierSharedOnWire parses native configuration and
// observes the OPEN each resulting Session sends. Distinct IPv4 and IPv6
// transport addresses cannot select different identifiers, and a new Session
// with unchanged startup configuration must send that same identifier again.
//
// RFC requirement: RFC6286-2.1-1 positive -- without session router-id overrides,
// the configured global non-zero 32-bit identifier is determined from startup
// configuration and appears unchanged in every peer's transmitted OPEN across
// distinct local addresses and repeated Session construction.
func TestRFC6286DefaultIdentifierSharedOnWire(t *testing.T) {
	for _, identifier := range []struct {
		text string
		want uint32
	}{
		{"0.0.0.1", 1},
		{"128.0.0.0", 0x80000000},
		{"255.255.255.255", 0xffffffff},
	} {
		t.Run(identifier.text, func(t *testing.T) {
			settings := rfc6286ConfiguredPeers(t, fmt.Sprintf(`bgp {
    router-id %s;
    session { asn { local 65000; } }
    peer first {
        connection { local { ip 192.0.2.10; } remote { ip 192.0.2.1; } }
        session { asn { remote 65001; } }
    }
    group inherited {
        peer second {
            connection { local { ip 198.51.100.10; } remote { ip 198.51.100.1; } }
            session { asn { remote 65001; } }
        }
        peer ipv6 {
            connection { local { ip 2001:db8::10; } remote { ip 2001:db8::1; } }
            session { asn { remote 65001; } }
        }
    }
}`, identifier.text))
			require.Len(t, settings, 3)
			for _, peer := range settings {
				for range 2 {
					open := rfc6286TransmittedOpen(t, peer)
					require.Equal(t, identifier.want, open.BGPIdentifier, peer.Name)
				}
			}
		})
	}
}

// TestRFC6286ExplicitRouterIDOverridesOnWire preserves operator control beyond
// the default RFC path: a group overrides the global value, a peer overrides
// its group, and another peer continues to inherit the global value.
func TestRFC6286ExplicitRouterIDOverridesOnWire(t *testing.T) {
	settings := rfc6286ConfiguredPeers(t, `bgp {
    router-id 1.2.3.4;
    session { asn { local 65000; } }
    peer global {
        connection { local { ip 192.0.2.10; } remote { ip 192.0.2.1; } }
        session { asn { remote 65001; } }
    }
    group overridden {
        session { router-id 5.6.7.8; }
        peer group {
            connection { local { ip 192.0.2.10; } remote { ip 192.0.2.2; } }
            session { asn { remote 65001; } }
        }
        peer own {
            connection { local { ip 192.0.2.10; } remote { ip 192.0.2.3; } }
            session { asn { remote 65001; } router-id 9.10.11.12; }
        }
    }
}`)
	want := map[string]uint32{"global": 0x01020304, "group": 0x05060708, "own": 0x090a0b0c}
	require.Len(t, settings, len(want))
	for _, peer := range settings {
		identifier, exists := want[peer.Name]
		require.True(t, exists, "unexpected peer %q", peer.Name)
		require.Equal(t, identifier, rfc6286TransmittedOpen(t, peer).BGPIdentifier, peer.Name)
		delete(want, peer.Name)
	}
	require.Empty(t, want, "every configured override must reach the wire")
}

// TestRFC6286IdentifiersWithoutGlobalOnWire preserves configurations that
// explicitly supply every identifier through peers or groups, with no global
// router-id. Config validation and the resolver must keep these usable.
func TestRFC6286IdentifiersWithoutGlobalOnWire(t *testing.T) {
	settings := rfc6286ConfiguredPeers(t, `bgp {
    session { asn { local 65000; } }
    peer own {
        connection { local { ip 192.0.2.10; } remote { ip 192.0.2.1; } }
        session { asn { remote 65001; } router-id 0.0.0.1; }
    }
    group explicit {
        session { router-id 255.255.255.255; }
        peer inherited {
            connection { local { ip 192.0.2.10; } remote { ip 192.0.2.2; } }
            session { asn { remote 65001; } }
        }
    }
}`)
	want := map[string]uint32{"own": 1, "inherited": 0xffffffff}
	require.Len(t, settings, len(want))
	for _, peer := range settings {
		identifier, exists := want[peer.Name]
		require.True(t, exists, "unexpected peer %q", peer.Name)
		require.Equal(t, identifier, rfc6286TransmittedOpen(t, peer).BGPIdentifier, peer.Name)
		delete(want, peer.Name)
	}
	require.Empty(t, want, "every explicitly supplied identifier must reach the wire")
}

// rfc6286ConfiguredPeers follows the side-effect-free startup loader and group
// inheritance entry points. It never constructs the expected OPEN.
func rfc6286ConfiguredPeers(t *testing.T, input string) []*reactor.PeerSettings {
	t.Helper()
	loaded, err := config.ReadConfig(input, "", nil)
	require.NoError(t, err)
	resolved, err := bgpconfig.ResolveBGPTree(loaded.Tree)
	require.NoError(t, err)
	settings, err := reactor.PeersFromTree(resolved)
	require.NoError(t, err)
	return settings
}

// rfc6286TransmittedOpen owns one synchronous Session and its recording socket.
// The Session MUST be stopped before its underlying pipe ends; deferred cleanup
// preserves that order. No read loop or test-owned goroutine is started.
func rfc6286TransmittedOpen(t *testing.T, settings *reactor.PeerSettings) *message.Open {
	t.Helper()
	local, remote := net.Pipe()
	defer func() { require.NoError(t, local.Close()) }()
	defer func() { require.NoError(t, remote.Close()) }()
	conn := &rfc6286OpenConn{Conn: local}
	session := reactor.NewSession(settings)
	require.NoError(t, session.Start())
	defer func() { require.NoError(t, session.Stop()) }()
	require.NoError(t, session.Accept(conn))
	wire := conn.written()
	require.GreaterOrEqual(t, len(wire), message.HeaderLen)
	header, err := message.ParseHeader(wire[:message.HeaderLen])
	require.NoError(t, err)
	require.Equal(t, msgtype.TypeOPEN, header.Type)
	require.Equal(t, int(header.Length), len(wire))
	open, err := message.UnpackOpen(wire[message.HeaderLen:])
	require.NoError(t, err)
	return open
}

// rfc6286OpenConn records the bounded startup exchange without a concurrent
// reader. The embedded pipe supplies lifecycle and deadline behavior. Safe for
// concurrent writes and observation; written returns an owned snapshot.
type rfc6286OpenConn struct {
	net.Conn
	mu     sync.Mutex
	output bytes.Buffer
}

func (c *rfc6286OpenConn) Write(wire []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.output.Write(wire)
}

func (c *rfc6286OpenConn) written() []byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	return bytes.Clone(c.output.Bytes())
}
