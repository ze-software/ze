// Design: docs/architecture/config/syntax.md -- infrastructure setup hook
// Related: loader_create.go -- CreateReactorFromTree calls the hook

package bgpconfig

import (
	"codeberg.org/thomas-mangin/ze/internal/component/authz"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/reactor"
	"codeberg.org/thomas-mangin/ze/internal/component/config/storage"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

// SSHExtractedConfig holds SSH server configuration extracted from the config
// tree as plain data. The caller converts this to ssh.Config and creates the
// server. This avoids bgpconfig importing the ssh package.
type SSHExtractedConfig struct {
	Listen      string
	ListenAddrs []string
	HostKeyPath string
	IdleTimeout uint32
	MaxSessions int
	Users       []authz.UserConfig
	HasConfig   bool // true if SSH block was present in config
}

// LoginWarning holds a warning message and optional command for the SSH login banner.
// Mirrors cli.LoginWarning to avoid bgpconfig importing the cli package.
type LoginWarning struct {
	Message string
	Command string
}

// InfraHookParams holds the data passed to the infrastructure setup hook.
type InfraHookParams struct {
	Reactor    *reactor.Reactor
	SSHConfig  SSHExtractedConfig
	AuthzStore *authz.Store
	ConfigDir  string
	ConfigPath string
	Store      storage.Storage

	// CollectLoginWarnings returns prefix warnings for the SSH login banner.
	// Called lazily on each SSH login, not at startup.
	CollectLoginWarnings func(rl plugin.ReactorIntrospector) []LoginWarning

	// FormatResponseData formats command response data for SSH display.
	FormatResponseData func(data any) string

	// APIServer returns the reactor's API server (available after post-start).
	APIServer func() *pluginserver.Server
}

// InfraHook sets up infrastructure servers (SSH, auth) after reactor creation.
// Provided by the hub, which imports ssh/cli/web packages.
// Set via SetInfraHook before the engine starts.
type InfraHook func(params InfraHookParams)

// infraHook is the package-level hook for infrastructure setup.
// Set by hub before engine start. Called by CreateReactorFromTree.
var infraHook InfraHook

// SetInfraHook registers the infrastructure setup hook.
// MUST be called before the engine starts.
func SetInfraHook(h InfraHook) {
	infraHook = h
}
