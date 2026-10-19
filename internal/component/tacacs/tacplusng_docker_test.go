//go:build docker

// Design: docs/architecture/aaa-tacacs.md -- the TACACS+ client against a server Ze did not write
// Related: authenticator.go -- the PAP login and session authorization this drives.
// Related: authorizer.go -- the per-command authorization this drives.
// Related: client.go -- the wire client every exchange here goes through.

package tacacs

import (
	"bytes"
	"context"
	"log/slog"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/le/interoplab"
	"github.com/ze-software/ze/internal/le/interoplab/labtest"
)

const (
	// tacPlusNGImage is tac_plus-ng from Marc Huber's event-driven-servers,
	// packaged on Alpine by christian-becker/tac_plus-ng (MIT). Pinned by
	// digest, so the server under the test cannot change between two runs.
	tacPlusNGImage = "docker.io/christianbecker/tac_plus-ng" +
		"@sha256:f17972cf0471ec32d4947a4152c8bd6a383b205f0dd0b48602fa6fa243fa393c"
	// tacPlusNGConfigPath is where the container's shell writes the config.
	tacPlusNGConfigPath = "/tmp/ze-interop.cfg"
	// tacPlusNGKey is the shared secret the server's device block holds.
	tacPlusNGKey = "ze-interop-shared-secret-0123456789"
	// tacPlusNGUser and tacPlusNGPassword are the one account the server
	// knows, with a cleartext PAP password.
	tacPlusNGUser     = "alice"
	tacPlusNGPassword = "alice-correct-password"
	// tacPlusNGPrivLvl is the level the server's operator profile grants a
	// shell session; Ze maps it to tacPlusNGProfile.
	tacPlusNGPrivLvl = 15
	tacPlusNGProfile = "operator-profile"
	// tacPlusNGRemote is the remote address Ze names in every request.
	tacPlusNGRemote = "192.0.2.10"
)

// tacPlusNGConfig grants alice a shell session at level 15, permits commands
// starting with "show" and denies every other command. It holds no single
// quote, so the container's shell can write it inside one.
const tacPlusNGConfig = `id = spawnd {
	background = no
	listen { port = 49 }
	spawn { instances min = 1 instances max = 4 }
}
id = tac_plus-ng {
	log stderrlog { destination = ">&2" }
	access log = stderrlog
	authorization log = stderrlog
	accounting log = stderrlog
	device everywhere {
		address = 0.0.0.0/0
		address = ::/0
		key = "` + tacPlusNGKey + `"
	}
	profile operator {
		script {
			if (service == shell) {
				if (cmd == "") {
					set priv-lvl = 15
					permit
				}
				if (cmd =~ /^show( |$)/) permit
				deny
			}
			deny
		}
	}
	user ` + tacPlusNGUser + ` {
		password { pap = clear "` + tacPlusNGPassword + `" }
	}
	ruleset {
		rule everyone {
			script {
				if (user == ` + tacPlusNGUser + `) {
					profile = operator
					permit
				}
			}
		}
	}
}
`

// fixedAuthorizer is a local fallback that answers one way for every command,
// so a test can tell the server's verdict from the fallback's.
type fixedAuthorizer bool

func (f fixedAuthorizer) Authorize(string, string, string, bool) bool { return bool(f) }

// startTacPlusNG starts tac_plus-ng and answers the host address it serves
// TACACS+ at. Readiness is a TACACS+ answer, not an accepted TCP connection:
// Docker's published port accepts before the daemon listens.
func startTacPlusNG(t *testing.T) netip.AddrPort {
	t.Helper()
	script := "printf '%s' '" + tacPlusNGConfig + "' > " + tacPlusNGConfigPath +
		" && exec /usr/local/sbin/tac_plus-ng " + tacPlusNGConfigPath
	return labtest.Start(t, labtest.Spec{
		Name:         "tacacs-tac-plus-ng",
		Image:        tacPlusNGImage,
		Command:      []string{"/bin/sh", "-c", script},
		Port:         interoplab.TCPPort(49),
		Ready:        tacPlusNGReady,
		ReadyTimeout: 60 * time.Second,
		LogLines:     200,
	})
}

// tacPlusNGReady answers nil once the server replies to a PAP START with any
// status, which only a listening daemon holding the shared secret can do.
func tacPlusNGReady(_ context.Context, address netip.AddrPort) error {
	client := newTacPlusNGClient(address, tacPlusNGKey, nil)
	defer client.Close()
	_, err := client.Authenticate("ze-ready-probe", "ze-ready-probe", defaultService, tacPlusNGRemote)
	return err
}

// newTacPlusNGClient builds Ze's client for the one server; a nil logger is
// the default logger.
func newTacPlusNGClient(address netip.AddrPort, key string, logger *slog.Logger) *TacacsClient {
	return NewTacacsClient(TacacsClientConfig{
		Servers: []TacacsServer{{Address: address.String(), Key: []byte(key)}},
		Timeout: 5 * time.Second,
		Logger:  logger,
	})
}

func newTacPlusNGAuthenticator(t *testing.T, address netip.AddrPort, key string, logger *slog.Logger) *tacacsAuthenticator {
	t.Helper()
	client := newTacPlusNGClient(address, key, logger)
	t.Cleanup(client.Close)
	privLvlMap := map[int][]string{tacPlusNGPrivLvl: {tacPlusNGProfile}}
	return newTacacsAuthenticator(client, privLvlMap, nil)
}

// TestTacPlusNGInterop drives Ze's TACACS+ client against tac_plus-ng, a
// server Ze did not write: a PAP login with its session authorization, a
// wrong password, a wrong shared secret, per-command authorization both ways,
// and an accounting START. Each case asserts the result Ze's AAA chain acts
// on. One container serves every case.
func TestTacPlusNGInterop(t *testing.T) {
	address := startTacPlusNG(t)

	// Correct secret and password: PASS, then the shell session authorization
	// returns priv-lvl=15, which Ze maps to its configured profile.
	t.Run("login accepted", func(t *testing.T) {
		auth := newTacPlusNGAuthenticator(t, address, tacPlusNGKey, nil)
		result, err := auth.Authenticate(aaa.AuthRequest{
			Username: tacPlusNGUser, Password: tacPlusNGPassword,
			RemoteAddr: tacPlusNGRemote, Service: "ssh",
		})
		require.NoError(t, err)
		assert.Equal(t, aaa.AuthResult{
			Authenticated: true, Profiles: []string{tacPlusNGProfile}, Source: backendName,
		}, result)
	})

	// Wrong password: the server's FAIL is a terminal rejection, so the chain
	// does not try the local backend.
	t.Run("wrong password rejected", func(t *testing.T) {
		auth := newTacPlusNGAuthenticator(t, address, tacPlusNGKey, nil)
		result, err := auth.Authenticate(aaa.AuthRequest{
			Username: tacPlusNGUser, Password: "alice-wrong-password",
			RemoteAddr: tacPlusNGRemote, Service: "ssh",
		})
		require.ErrorIs(t, err, aaa.ErrAuthRejected)
		assert.Equal(t, aaa.AuthResult{Source: backendName}, result)
	})

	// Wrong shared secret, correct password: the reply does not de-obfuscate,
	// which RFC 8907 Section 4.5 makes an ERROR ("If the sum is not identical
	// to the cleartext datalength value from the header, the packet MUST be
	// discarded and an ERROR signaled"), and Section 4.4 has the client behave
	// as if the server could not be connected to. So no login succeeds, and
	// the answer is an unreachable server rather than a rejection: the chain
	// goes on to the next backend. The client's log names the bad secret,
	// which is how the test knows the server answered and was not merely
	// absent.
	t.Run("shared secret mismatch", func(t *testing.T) {
		var clientLog bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&clientLog, nil))
		auth := newTacPlusNGAuthenticator(t, address, "not-the-servers-shared-secret-0123", logger)
		result, err := auth.Authenticate(aaa.AuthRequest{
			Username: tacPlusNGUser, Password: tacPlusNGPassword,
			RemoteAddr: tacPlusNGRemote, Service: "ssh",
		})
		require.EqualError(t, err, "tacacs: all TACACS+ servers unreachable for authentication")
		assert.Equal(t, aaa.AuthResult{}, result)
		assert.Contains(t, clientLog.String(), ErrBadSecret.Error())
	})

	// A command the server's profile permits is allowed while the local
	// fallback denies everything, so the allow is the server's.
	t.Run("command permitted", func(t *testing.T) {
		client := newTacPlusNGClient(address, tacPlusNGKey, nil)
		t.Cleanup(client.Close)
		authorizer := newTacacsAuthorizer(client, fixedAuthorizer(false))
		assert.True(t, authorizer.Authorize(tacPlusNGUser, tacPlusNGRemote, "show version", true))
	})

	// A command the server's profile denies is refused while the local
	// fallback permits everything, so the refusal is the server's.
	t.Run("command denied", func(t *testing.T) {
		client := newTacPlusNGClient(address, tacPlusNGKey, nil)
		t.Cleanup(client.Close)
		authorizer := newTacacsAuthorizer(client, fixedAuthorizer(true))
		assert.False(t, authorizer.Authorize(tacPlusNGUser, tacPlusNGRemote, "configure terminal", false))
	})

	// An accounting START in the form TacacsAccountant sends is recorded.
	t.Run("accounting start recorded", func(t *testing.T) {
		client := newTacPlusNGClient(address, tacPlusNGKey, nil)
		t.Cleanup(client.Close)
		reply, err := client.SendAccounting(&AcctRequest{
			Flags: AcctFlagStart, AuthenMethod: AuthenMethodTACACS, PrivLvl: 1,
			AuthenType: 0x01, AuthenService: authenServiceLogin,
			User: tacPlusNGUser, Port: portSSH, RemAddr: tacPlusNGRemote,
			Args: accountingArguments("1", "start_time=1760000000", splitTacacsTokens(strings.Fields("show version"))),
		})
		require.NoError(t, err)
		assert.Equal(t, uint8(AcctStatusSuccess), reply.Status)
	})
}
