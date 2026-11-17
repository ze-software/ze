//go:build ze_ssh && ze_bgp

package hub

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/ze-software/ze/internal/component/authz"
	"github.com/ze-software/ze/internal/component/bgp/reactor"
	"github.com/ze-software/ze/internal/component/config/infra"
	zessh "github.com/ze-software/ze/internal/component/ssh"
	"github.com/ze-software/ze/internal/core/env"
)

// TestInfraSetupWiresSessionModelFactory verifies that infraSetup sets the SSH
// session model factory unconditionally, not only in ephemeral mode.
//
// Regression for ab982c3ba: SetSessionModelFactory was accidentally nested
// inside the ephemeral-file check, so appliance SSH sessions with a bgp {}
// config got a nil factory and the TUI crashed on connect.
//
// VALIDATES: SSH TUI works on appliance (bgp + ssh config, no ze.ssh.ephemeral).
// PREVENTS: SetSessionModelFactory accidentally gated on ephemeral mode.
func TestInfraSetupWiresSessionModelFactory(t *testing.T) {
	resetAAABundleForTest(t)
	require.NoError(t, env.Set("ze.ssh.ephemeral", ""))
	defer func() { _ = env.Set("ze.ssh.ephemeral", "") }()

	r := reactor.New(&reactor.Config{})

	params := infra.HookParams{
		Reactor: r,
		SSHConfig: infra.SSHExtractedConfig{
			Listen:      "127.0.0.1:0",
			HostKeyPath: t.TempDir() + "/test_host_key",
			HasConfig:   true,
		},
	}

	sshSrv := infraSetup(params, nil, &configMutationOwner{}, nil)
	require.NotNil(t, sshSrv, "infraSetup should return a running SSH server")
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		require.NoError(t, sshSrv.Stop(ctx))
	})
	srv, ok := sshSrv.(*zessh.Server)
	require.True(t, ok, "infraSetup should return a *zessh.Server when ssh is compiled in")
	assert.True(t, srv.HasSessionModelFactoryForTest(),
		"session model factory must be set for interactive SSH sessions to work")
}
