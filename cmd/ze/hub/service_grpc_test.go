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
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	zepb "github.com/ze-software/ze/api/proto"
	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/env"
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

// VALIDATES: AC-3 and AC-4 through runYANGConfig. A user declared only at
// system.authentication.user reaches ConfigProvider, the accepted identity
// generation, the gRPC builder, and a real authenticated RPC without an SSH
// block.
// PREVENTS: a hand-built user slice proving grpcBuildImpl while the daemon boot
// producer remains disconnected from gRPC.
func TestGRPCBuildAuthenticatesConfigUserWithoutSSH(t *testing.T) {
	resetAAABundleForTest(t)
	clearAPIEnv(t)
	const (
		username = "grpc-config-user"
		password = "grpc-pass"
		command  = "test grpc no ssh auth"
	)
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	require.NoError(t, err)
	certPath, keyPath := writeGRPCTestTLSMaterial(t)
	configText := fmt.Sprintf(`
environment {
	api-server {
		grpc {
			enabled true
			tls-cert %q
			tls-key %q
			server main {
				ip 0.0.0.0
				port 0
			}
		}
	}
}
system {
	authentication {
		user %s {
			password %q
			profile [ grpc-operator ]
		}
	}
	authorization {
		profile grpc-operator {
			run { default-action allow }
			edit { default-action allow }
		}
	}
}
`, certPath, keyPath, username, string(hash))

	type bootServer struct {
		handle   apiServerHandle
		server   *pluginserver.Server
		buildErr error
	}
	started := make(chan bootServer, 1)
	originalBuild := grpcBuild
	t.Cleanup(func() { grpcBuild = originalBuild })
	grpcBuild = func(in *apiBuildInputs, shared *apiShared) (apiServerHandle, error) {
		in.Server.Dispatcher().Register(command, func(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
			return plugin.NewResponse(plugin.StatusDone, plugin.RawJSON(`"ok"`)), nil
		}, command)
		handle, buildErr := grpcBuildImpl(in, shared)
		started <- bootServer{
			handle:   handle,
			server:   in.Server,
			buildErr: buildErr,
		}
		return handle, buildErr
	}

	configDir := t.TempDir()
	originalConfigDir := env.Get("ze.config.dir")
	require.NoError(t, env.Set("ze.config.dir", configDir))
	t.Cleanup(func() { require.NoError(t, env.Set("ze.config.dir", originalConfigDir)) })
	exitResult := make(chan int, 1)
	go func() {
		exitResult <- runYANGConfig(
			storage.NewFilesystem(),
			"-",
			[]byte(configText),
			nil,
			0,
			-1,
			false,
			false,
			"",
			false,
			"",
			"",
			false,
			nil,
		)
	}()

	var booted bootServer
	select {
	case booted = <-started:
	case exit := <-exitResult:
		t.Fatalf("runYANGConfig exited with %d before building gRPC", exit)
	case <-time.After(10 * time.Second):
		t.Fatal("runYANGConfig did not build the configured gRPC server")
	}
	require.NoError(t, booted.buildErr)
	t.Cleanup(booted.server.Stop)
	require.NotNil(t, booted.handle.Server)
	require.Len(t, booted.handle.Server.Addresses(), 1)
	_, port, err := net.SplitHostPort(booted.handle.Server.Addresses()[0])
	require.NoError(t, err)

	conn, err := grpc.NewClient(net.JoinHostPort("127.0.0.1", port), grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{
		InsecureSkipVerify: true, //nolint:gosec // self-signed test certificate
		MinVersion:         tls.VersionTLS12,
	})))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, conn.Close()) })
	client := zepb.NewZeServiceClient(conn)

	goodCtx := metadata.NewOutgoingContext(t.Context(), metadata.Pairs(
		"authorization", "Bearer "+username+":"+password,
	))
	response, err := client.Execute(goodCtx, &zepb.CommandRequest{Command: command})
	require.NoError(t, err)
	assert.Equal(t, "done", response.GetStatus())

	wrongCtx := metadata.NewOutgoingContext(t.Context(), metadata.Pairs(
		"authorization", "Bearer "+username+":wrong",
	))
	_, err = client.Execute(wrongCtx, &zepb.CommandRequest{Command: command})
	assert.Equal(t, codes.Unauthenticated, status.Code(err))

	booted.server.Stop()
	select {
	case exit := <-exitResult:
		assert.Zero(t, exit)
	case <-time.After(10 * time.Second):
		t.Fatal("runYANGConfig did not exit after its plugin server stopped")
	}
}
