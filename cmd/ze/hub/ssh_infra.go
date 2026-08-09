// Design: docs/architecture/hub-architecture.md -- infrastructure server setup
//
// The ssh compile-out seam. Always-on daemon-startup code (infraSetup, the
// no-bgp{} path in main.go) builds AAA/authorization/accounting and then, if an
// ssh implementation is registered, builds and wires the ssh server THROUGH
// this seam -- never importing internal/component/ssh directly. The seam
// carries only generic infra types (infra.HookParams, AAA, audit); the opaque
// sshServer handle hides *zessh.Server. When ze_ssh is compiled out, the seam
// vars stay nil, ssh is never built, and the ssh package is dropped. The
// looking-glass seam was the pilot for this shape and uses the same one.

package hub

import (
	"log/slog"

	"github.com/ze-software/ze/internal/component/config/infra"

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/audit"
)

// sshServer is the always-on view of a built ssh server: enough to log its
// address and hand it back to the wire step. The real (gated) *zessh.Server
// satisfies it. Always-on code never names the concrete type.
type sshServer interface {
	Address() string
}

// sshBuildInputs carries everything the gated ssh builder needs to construct
// and start the server. Generic types only -- no zessh.
type sshBuildInputs struct {
	Config infra.SSHExtractedConfig
	Users  []aaa.UserCredential
	// UsersFunc is the running-config credential source (liveLocalUsers in
	// main_servers.go). Public-key authentication answers from it, so a user a
	// reload removed loses their key access without a daemon restart. It
	// REPLACES Users at the server rather than adding to it.
	UsersFunc     func() ([]aaa.UserCredential, error)
	Authenticator aaa.Authenticator
	Recorder      audit.Recorder
	EphemeralFile string
	Params        infra.HookParams
	ReloadFn      func() error
	Log           *slog.Logger
}

// sshWireInputs carries the inputs for the post-start setter wiring (command
// executors, monitor, shutdown/restart/reboot, login warnings). The gated wire
// step re-derives the dispatcher and api server from the reactor/params.
//
// Reactor is the reactor-free infra.ReactorHandle, not *reactor.Reactor: this
// file is always-on, so naming the BGP type here would pin internal/component/bgp
// into every binary and defeat //go:build ze_bgp.
type sshWireInputs struct {
	Reactor       infra.ReactorHandle
	Params        infra.HookParams
	WriteGRMarker func()
}

// sshStandaloneInputs carries the inputs for the no-bgp{} startup path
// (main.go): an ssh-only daemon (e.g. a gokrazy appliance with just an
// environment{} block) that wires a session model + a simple dispatch executor
// and starts the server. The AAA bundle is built always-on by the caller (it
// may also serve MCP/API); only the resolved authenticator crosses the seam.
type sshStandaloneInputs struct {
	Config infra.SSHExtractedConfig
	Users  []aaa.UserCredential
	// UsersFunc is the running-config credential source, as in sshBuildInputs.
	UsersFunc     func() ([]aaa.UserCredential, error)
	Authenticator aaa.Authenticator
	Recorder      audit.Recorder
	ConfigDir     string
	Storage       storage.Storage
	ConfigPath    string
	EphemeralFile string
	Dispatch      plugin.CommandDispatcher
	ReloadFn      func() error
	Log           *slog.Logger
}

// sshBuild builds and starts the ssh server, returning a handle (or nil if ssh
// is not configured / failed to start). Set by register_ssh.go under
// //go:build ze_ssh; nil when ssh is compiled out. Inputs are passed by pointer
// (the struct embeds the heavy infra.HookParams).
var sshBuild func(in *sshBuildInputs) sshServer

// sshWirePostStart wires the ssh command executors and lifecycle callbacks onto
// a built server, inside the reactor's post-start callback. Set alongside
// sshBuild; nil when ssh is compiled out.
var sshWirePostStart func(srv sshServer, in *sshWireInputs)

// sshBuildStandalone builds + wires + starts ssh for the no-bgp{} path and
// returns a shutdown func (nil if ssh did not start). Set alongside sshBuild;
// nil when ssh is compiled out.
var sshBuildStandalone func(in *sshStandaloneInputs) func()

// setSSHInfra installs the ssh implementations into the seam. Called from
// register_ssh.go's init() (under //go:build ze_ssh in Phase 2); absent that
// file, the seam stays nil and ssh is not built.
func setSSHInfra(
	build func(in *sshBuildInputs) sshServer,
	wire func(srv sshServer, in *sshWireInputs),
	standalone func(in *sshStandaloneInputs) func(),
) {
	sshBuild = build
	sshWirePostStart = wire
	sshBuildStandalone = standalone
}
