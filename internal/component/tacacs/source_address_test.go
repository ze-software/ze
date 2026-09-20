// Design: (none -- new TACACS+ component)
// Related: client.go -- dial, which binds the outbound source address
// Related: register.go -- tacacsBackend.Build, which refuses a bad one at load

// Source-address binding for outbound TACACS+ TCP.
package tacacs

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/component/config"
)

// tacacsSourceTree builds the config tree shape ExtractConfig reads: one keyed
// server and, when it is not empty, a source-address.
func tacacsSourceTree(address, sourceAddress string) *config.Tree {
	tree := config.NewTree()
	sys := config.NewTree()
	auth := config.NewTree()
	tac := config.NewTree()

	srv := config.NewTree()
	srv.Set("key", "secret")
	tac.AddListEntry("server", address, srv)
	if sourceAddress != "" {
		tac.Set("source-address", sourceAddress)
	}

	auth.SetContainer("tacacs", tac)
	sys.SetContainer("authentication", auth)
	tree.SetContainer("system", sys)
	return tree
}

// VALIDATES: tacacsBackend.Build refuses a source-address it cannot parse, and
// the error names the address the operator typed.
// METHOD: build the AAA contribution from a config tree carrying "10.0.0.256".
// PREVENTS: the daemon starting on a mistyped source-address, where every login
// then fails behind a warning that reads as an unreachable server.
func TestTacacsBuildRefusesUnparseableSourceAddress(t *testing.T) {
	_, err := tacacsBackend{}.Build(aaa.BuildParams{
		Ctx:        context.Background(),
		ConfigTree: tacacsSourceTree("10.0.0.1", "10.0.0.256"),
		Logger:     slog.Default(),
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "10.0.0.256")
}

// VALIDATES: a source-address that parses still builds the backend.
// METHOD: build the same contribution with "127.0.0.1".
// PREVENTS: the refusal above rejecting a valid configuration.
func TestTacacsBuildAcceptsParseableSourceAddress(t *testing.T) {
	contrib, err := tacacsBackend{}.Build(aaa.BuildParams{
		Ctx:        context.Background(),
		ConfigTree: tacacsSourceTree("10.0.0.1", "127.0.0.1"),
		Logger:     slog.Default(),
	})

	require.NoError(t, err)
	assert.NotNil(t, contrib.Authenticator)
}

// VALIDATES: Authenticate opens no connection when the configured
// source-address cannot be parsed.
// METHOD: point a client at a live listener that accepts anything, then watch
// whether the listener is reached at all.
// PREVENTS: the wildcard bind. Its tell is not a failed login but a TACACS+
// connection that arrives from whichever address the route picked, so the
// assertion is that the server is never reached.
func TestTacacsAuthenticateRefusesUnparseableSourceAddress(t *testing.T) {
	listener := listenTCP(t)
	defer closeIgnore(listener)

	accepted := make(chan struct{}, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return // listener closed
		}
		closeIgnore(conn)
		accepted <- struct{}{}
	}()

	client := NewTacacsClient(TacacsClientConfig{
		Servers:       []TacacsServer{{Address: listener.Addr().String(), Key: []byte("secret")}},
		Timeout:       200 * time.Millisecond,
		SourceAddress: "10.0.0.256",
	})
	defer client.Close()

	reply, err := client.Authenticate("operator", "password", portSSH, "127.0.0.1")
	require.Error(t, err)
	assert.Nil(t, reply)

	select {
	case <-accepted:
		t.Fatal("the client reached the server with an unparseable source-address: the bind fell back to the wildcard")
	case <-time.After(500 * time.Millisecond):
	}
}
