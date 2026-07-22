// Design: docs/architecture/hub-architecture.md -- infrastructure setup hook
// Related: ssh.go -- SSH server config extraction feeding HookParams.SSHConfig
// Related: authz.go -- authorization extraction feeding HookParams.AuthzStore

// Package infra holds the always-on daemon-startup contract between the hub and
// whichever routing engine constructs a reactor.
//
// It exists because both sides must survive the other being compiled out. The
// hub builds AAA/authorization/accounting and (when ze_ssh is on) the SSH server
// from a parsed config tree; the BGP engine, once it has built its reactor,
// calls back through Hook so that infrastructure is wired into the reactor's
// post-start. Neither the extraction nor the hook contract is BGP-specific, so
// keeping them here lets internal/component/bgp compile out entirely
// (//go:build ze_bgp) while `ze config validate`, the SSH-only startup path, and
// the AAA stack keep working.
//
// This is a CHILD of internal/component/config on purpose: the extractors need
// internal/component/authz and internal/component/aaa, and both of those already
// import internal/component/config, so the code cannot live in config itself.
package infra

import (
	"codeberg.org/thomas-mangin/ze/internal/component/authz"
	"codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/component/config/storage"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

// SSHExtractedConfig holds SSH server configuration extracted from the config
// tree as plain data. The caller converts this to ssh.Config and creates the
// server. This avoids the extractor importing the ssh package.
type SSHExtractedConfig struct {
	Listen       string
	ListenAddrs  []string
	HostKeyPath  string
	HostCertPath string
	IdleTimeout  uint32
	MaxSessions  int
	Users        []authz.UserConfig
	HasConfig    bool // true if SSH block was present in config
}

// LoginWarning holds a warning message and optional command for the SSH login
// banner. Mirrors cli.LoginWarning to avoid this package importing the cli
// package.
type LoginWarning struct {
	Message string
	Command string
}

// ReactorHandle is the always-on view of a running protocol reactor: the three
// operations daemon-startup infrastructure needs once the engine exists. The
// BGP *reactor.Reactor satisfies it; always-on code never names the concrete
// type, which is what lets internal/component/bgp be compiled out
// (ai/rules/feature-gate-registration.md, "no feature type in an always-on
// signature").
type ReactorHandle interface {
	// SetPostStartFunc registers a callback the reactor runs once it is up.
	SetPostStartFunc(func())
	// Dispatcher returns the command dispatcher to attach authorization and
	// accounting to. May be nil before post-start.
	Dispatcher() *pluginserver.Dispatcher
	// Stop signals the reactor to shut down (shutdown / restart / reboot).
	Stop()
}

// HookParams holds the data passed to the infrastructure setup hook.
type HookParams struct {
	Reactor    ReactorHandle
	SSHConfig  SSHExtractedConfig
	ConfigTree *config.Tree // full config tree for component-specific extraction
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

// Hook sets up infrastructure servers (SSH, auth) after reactor creation.
// Provided by the hub, which imports ssh/cli/web packages.
// Set via SetHook before the engine starts.
type Hook func(params HookParams)

// hook is the package-level hook for infrastructure setup.
// Set by the hub before engine start. Called by the engine once its reactor
// exists (BGP: CreateReactorFromTree).
var hook Hook

// SetHook registers the infrastructure setup hook.
// MUST be called before the engine starts.
func SetHook(h Hook) {
	hook = h
}

// Run invokes the registered infrastructure setup hook, if any. The engine
// calls this after building its reactor. A nil hook (no hub, e.g. an offline
// CLI load) is a no-op, so the engine never needs its own nil check.
func Run(params HookParams) {
	if hook != nil {
		hook(params)
	}
}
