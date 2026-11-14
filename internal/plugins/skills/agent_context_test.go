// Design: docs/features/ai-first.md — what an agent session is told about this Ze
// Related: agent_context.go — the command under test

package skills

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/version"
)

// isolateConfigDirectory points HOME, XDG_CONFIG_HOME and ze.config.dir at
// directories of the test's own, restores ze.config.dir when the test ends,
// and answers the config directory.
func isolateConfigDirectory(t *testing.T) string {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	configDirectory := t.TempDir()
	previous := env.Get("ze.config.dir")
	require.NoError(t, env.Set("ze.config.dir", configDirectory))
	t.Cleanup(func() {
		if err := env.Set("ze.config.dir", previous); err != nil {
			t.Errorf("restore ze.config.dir: %v", err)
		}
	})
	return configDirectory
}

// fixedAddress answers the daemon address the test chose.
func fixedAddress(address string) func() (string, string, error) {
	return func() (string, string, error) {
		return net.SplitHostPort(address)
	}
}

// closedAddress answers a loopback address nothing listens on: a port the
// kernel handed out and the test released at once.
func closedAddress(t *testing.T) string {
	t.Helper()
	var listenConfig net.ListenConfig
	listener, err := listenConfig.Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	address := listener.Addr().String()
	require.NoError(t, listener.Close())
	return address
}

// printContext runs the shell route and answers its exit code and output.
func printContext(t *testing.T, routes contextRoutes) (int, string) {
	t.Helper()
	var code int
	out := captureOutput(t, func() { code = routes.printLocal(context.Background()) })
	return code, out
}

// TestAgentContextNamesTheVersionTheDaemonAndTheConfigDirectory checks the
// shell route on a machine whose daemon answers (AC-22). Method: a loopback
// listener stands in for the daemon, the route dials it through the real
// dialer, and the printed lines must name the version, the address that
// answered, the config directory and the guide, with exit status 0.
func TestAgentContextNamesTheVersionTheDaemonAndTheConfigDirectory(t *testing.T) {
	configDirectory := isolateConfigDirectory(t)
	var listenConfig net.ListenConfig
	listener, err := listenConfig.Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { listener.Close() }) //nolint:errcheck // test teardown
	address := listener.Addr().String()

	var dialer net.Dialer
	code, out := printContext(t, contextRoutes{address: fixedAddress(address), dial: dialer.DialContext})

	assert.Equal(t, 0, code)
	assert.Contains(t, out, version.Short()+"\n")
	assert.Contains(t, out, "daemon: answering at "+address+"\n")
	assert.Contains(t, out, "config directory: "+configDirectory+"\n")
	assert.Contains(t, out, "guide: the ze skill, read it with ze show skill ze\n")
}

// TestAgentContextWithNoDaemonAnswersWithinItsBoundAndSucceeds checks the
// shell route with no daemon listening (AC-23). Method: the route dials a
// loopback port nothing listens on through the real dialer; it must say the
// daemon is not answering at that address, still name the version, exit 0,
// and return well under two seconds.
func TestAgentContextWithNoDaemonAnswersWithinItsBoundAndSucceeds(t *testing.T) {
	isolateConfigDirectory(t)
	address := closedAddress(t)

	var dialer net.Dialer
	start := time.Now()
	code, out := printContext(t, contextRoutes{address: fixedAddress(address), dial: dialer.DialContext})
	elapsed := time.Since(start)

	assert.Equal(t, 0, code)
	assert.Contains(t, out, version.Short()+"\n")
	assert.Contains(t, out, "daemon: not answering at "+address)
	assert.Less(t, elapsed, 2*time.Second)
}

// TestAgentContextBoundsADialThatNeverEnds checks that the dial carries the
// one-second bound (AC-23, R-4). Method: a dial that waits for its context to
// end stands in for a firewall that drops the packet. Without the bound the
// call never returns and the test times out; with it the route answers not
// answering, exits 0, and returns well under two seconds.
func TestAgentContextBoundsADialThatNeverEnds(t *testing.T) {
	isolateConfigDirectory(t)
	address := closedAddress(t)
	hang := func(ctx context.Context, _, _ string) (net.Conn, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}

	start := time.Now()
	code, out := printContext(t, contextRoutes{address: fixedAddress(address), dial: hang})
	elapsed := time.Since(start)

	assert.Equal(t, 0, code)
	assert.Contains(t, out, "daemon: not answering at "+address)
	assert.Less(t, elapsed, 2*time.Second)
}

// TestAgentContextAnswersAnUnreadableAddressAsNotAnswering checks R-14: an
// error resolving the daemon's address is the daemon not answering, with the
// reason, and the command still exits 0. Method: the resolver fails and the
// dial fails the test if it is called.
func TestAgentContextAnswersAnUnreadableAddressAsNotAnswering(t *testing.T) {
	isolateConfigDirectory(t)
	routes := contextRoutes{
		address: func() (string, string, error) { return "", "", errors.New("the store is damaged") },
		dial: func(context.Context, string, string) (net.Conn, error) {
			t.Error("the route dialed an address it could not resolve")
			return nil, errors.New("refused by the test")
		},
	}

	code, out := printContext(t, routes)

	assert.Equal(t, 0, code)
	assert.Contains(t, out, "daemon: not answering: its address could not be read: the store is damaged\n")
}

// TestAgentContextThroughTheDaemonCarriesNoPath checks the daemon command
// (AC-24, R-9). Method: the routes hold a resolver and a dial that each fail
// the test when called, and ze.config.dir names a directory of the test's
// own. The answer must carry exactly the version, the daemon line, the skill
// and the guide command, none of them the directory, and its drawing must
// show no config directory line.
func TestAgentContextThroughTheDaemonCarriesNoPath(t *testing.T) {
	configDirectory := isolateConfigDirectory(t)
	routes := contextRoutes{
		address: func() (string, string, error) {
			t.Error("the daemon command resolved the daemon's address")
			return "", "", errors.New("refused by the test")
		},
		dial: func(context.Context, string, string) (net.Conn, error) {
			t.Error("the daemon command dialed")
			return nil, errors.New("refused by the test")
		},
	}

	response := routes.respond()
	require.Equal(t, plugin.StatusDone, response.Status)
	encoded, err := json.Marshal(response.Data)
	require.NoError(t, err)

	var envelope map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(encoded, &envelope))
	raw, found := envelope[keyAgentContext]
	require.True(t, found, "answer: %s", encoded)

	var fields map[string]string
	require.NoError(t, json.Unmarshal(raw, &fields))
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	assert.Equal(t, []string{"daemon", "guide-command", "skill", "version"}, keys)
	assert.Equal(t, version.Short(), fields["version"])
	assert.NotContains(t, string(encoded), configDirectory)

	drawn, err := drawAgentContext(raw)
	require.NoError(t, err)
	assert.Contains(t, drawn, version.Short()+"\n")
	assert.Contains(t, drawn, "daemon: reached")
	assert.NotContains(t, drawn, "config directory")
}
