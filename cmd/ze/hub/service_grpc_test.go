// Design: ai/rules/plugins.md -- ze_grpc-gated gRPC seam test
//
//go:build ze_grpc

package hub

// VALIDATES: grpcBuildImpl treats a config with gRPC disabled as a clean skip
// (zero handle, no error) without touching the shared engine/sessions. The full
// gRPC bind path is covered by the test/parse api-grpc-*.ci functional tests
// under the ze_grpc build.
// PREVENTS: a regression where the gRPC seam errors or builds a server when gRPC
// is not enabled in config.

import (
	"crypto/tls"
	"encoding/base64"
	"encoding/pem"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	zepb "github.com/ze-software/ze/api/proto"
	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/component/authz"
	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

func TestGRPCBuildNotEnabled(t *testing.T) {
	// GRPCOn defaults false: the builder returns a zero handle before using the
	// shared state, so an empty apiShared is sufficient.
	h, err := grpcBuildImpl(&apiBuildInputs{Config: zeconfig.APIConfig{}}, &apiShared{})
	require.NoError(t, err)
	assert.Nil(t, h.Server, "gRPC disabled must yield a zero handle (nil Server)")
	assert.Nil(t, h.Shutdown, "gRPC disabled must yield no shutdown")
}

func writeGRPCTestTLSMaterial(t *testing.T) (string, string) {
	t.Helper()
	_, certB64, keyB64 := caSignedB64(t, "grpc no ssh")
	certDER, err := base64.StdEncoding.DecodeString(certB64)
	require.NoError(t, err)
	keyDER, err := base64.StdEncoding.DecodeString(keyB64)
	require.NoError(t, err)
	dir := t.TempDir()
	certPath := filepath.Join(dir, "grpc.pem")
	keyPath := filepath.Join(dir, "grpc.key")
	require.NoError(t, os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER}), 0o600))
	require.NoError(t, os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}), 0o600))
	return certPath, keyPath
}

// VALIDATES: AC-3 and AC-4, grpcBuildImpl serves a real authenticated RPC from
// the shared no-SSH user source on a non-loopback TLS listener.
// PREVENTS: the gRPC builder receiving a nil authenticator because boot users
// were classified through SSH, or accepting credentials without publishing the
// user's resolved profiles.
func TestGRPCBuildAuthenticatesConfigUserWithoutSSH(t *testing.T) {
	const username = "grpc-config-user"
	aaa.ForgetLoginProfilesForTest(username)
	t.Cleanup(func() { aaa.ForgetLoginProfilesForTest(username) })

	hash, err := bcrypt.GenerateFromPassword([]byte("grpc-pass"), bcrypt.MinCost)
	require.NoError(t, err)
	users := []authz.UserConfig{{
		Name:     username,
		Hash:     string(hash),
		Profiles: []string{"grpc-operator"},
	}}
	server, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
	require.NoError(t, err)
	t.Cleanup(server.Stop)
	const command = "test grpc no ssh auth"
	server.Dispatcher().Register(command, func(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
		return plugin.NewResponse(plugin.StatusDone, plugin.RawJSON(`"ok"`)), nil
	}, command)

	certPath, keyPath := writeGRPCTestTLSMaterial(t)
	cfg := zeconfig.APIConfig{
		GRPCOn:      true,
		GRPC:        []zeconfig.APIListenConfig{{Host: "0.0.0.0", Port: "0"}},
		GRPCTLSCert: certPath,
		GRPCTLSKey:  keyPath,
	}
	in := &apiBuildInputs{
		Config: cfg,
		Server: server,
		Users:  users,
		UsersLive: func() ([]authz.UserConfig, error) {
			return users, nil
		},
	}
	shared := buildAPIShared(in)
	require.NotNil(t, shared.Authenticator)
	assert.False(t, checkMgmtListeners([]mgmtListener{{
		service:       "API gRPC",
		addrs:         apiGuardAddrs(cfg),
		authenticated: shared.Authenticator != nil,
	}}), "real per-user authentication must satisfy the generic non-loopback guard")

	handle, err := grpcBuildImpl(in, shared)
	require.NoError(t, err)
	require.NotNil(t, handle.Server)
	require.NotNil(t, handle.Shutdown)
	t.Cleanup(func() { handle.Shutdown(t.Context()) })
	require.Len(t, handle.Server.Addresses(), 1)
	_, port, err := net.SplitHostPort(handle.Server.Addresses()[0])
	require.NoError(t, err)

	conn, err := grpc.NewClient(net.JoinHostPort("127.0.0.1", port), grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{
		InsecureSkipVerify: true, //nolint:gosec // self-signed test certificate
		MinVersion:         tls.VersionTLS12,
	})))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, conn.Close()) })
	client := zepb.NewZeServiceClient(conn)

	goodCtx := metadata.NewOutgoingContext(t.Context(), metadata.Pairs(
		"authorization", "Bearer "+username+":grpc-pass",
	))
	response, err := client.Execute(goodCtx, &zepb.CommandRequest{Command: command})
	require.NoError(t, err)
	assert.Equal(t, "done", response.GetStatus())

	wrongCtx := metadata.NewOutgoingContext(t.Context(), metadata.Pairs(
		"authorization", "Bearer "+username+":wrong",
	))
	_, err = client.Execute(wrongCtx, &zepb.CommandRequest{Command: command})
	assert.Equal(t, codes.Unauthenticated, status.Code(err))

	profiles, ok := aaa.LoginProfiles(username)
	require.True(t, ok, "the actual gRPC login must publish resolved profiles")
	assert.Equal(t, []string{"grpc-operator"}, profiles)
}
