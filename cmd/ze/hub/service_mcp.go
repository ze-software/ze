// Design: ai/rules/plugins.md -- compile-out-able services (feature-gate)
// Related: main.go -- resolves MCP listen/token/config to plain values and feeds them via serviceDeps.MCP
//
// MCP (Model Context Protocol) service: built through the construction registry
// and compiled in ONLY under //go:build ze_mcp. This file (with register_mcp.go)
// is the ONLY place always-on-buildable code reaches the internal/component/mcp
// package. With ze_mcp off the factory is not registered, the hub builds no MCP
// service, and the mcp package is linked nowhere -- so the linker drops it
// (smaller binary, smaller attack surface).
//
// The MCP server handle is already Reconfigurable + Shutdown (live listener
// migration), so MCP fits the listener-service registry like web/lg. Only the
// construction helpers that name zemcp types moved here; the neutral command
// metadata source (command_meta.go) and the MCP listen/token resolution
// (main.go) stay always-on. The mcp YANG schema is gated separately by the
// generator (all_ze_mcp.go). See plan/spec-feature-gate-5-mcp.md.

//go:build ze_mcp

package hub

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"slices"
	"sync"
	"time"

	zeconfig "github.com/ze-software/ze/internal/component/config"
	zemcp "github.com/ze-software/ze/internal/component/mcp"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/privilege"
)

// mcpService adapts *mcpServerHandle to the Service interface. The handle
// already satisfies Reconfigurable and Shutdown, so this type adds only Name.
type mcpService struct {
	*mcpServerHandle
}

func (mcpService) Name() string { return "mcp" }

// buildMCPService builds and starts the MCP HTTP server from deps. It returns a
// nil Service (not an error) when MCP is not configured or fails to start --
// preserving the prior best-effort, non-fatal behavior of the inline
// startMCPServer call in main.go.
func buildMCPService(deps *serviceDeps) (Service, error) {
	m := deps.MCP
	if m == nil || len(m.Addrs) == 0 || m.Dispatch == nil {
		// Not configured: a skip, not a failure.
		return nil, nil //nolint:nilnil // not-configured is an intentional skip
	}

	mcpStreamCfg := zemcp.StreamableConfig{Token: m.Token, AuditRecorder: m.Recorder}
	var tlsCert, tlsKey string
	if m.ConfigOK {
		mcpStreamCfg = mcpConfigToStreamable(m.Config, mcpStreamCfg)
		tlsCert = m.Config.TLS.Cert
		tlsKey = m.Config.TLS.Key
	}

	handle := startMCPServer(m.Addrs, m.Dispatch, mcpCommandLister(m.Commands), mcpStreamCfg, tlsCert, tlsKey)
	if handle == nil {
		// startMCPServer already logged the reason; non-fatal skip.
		return nil, nil //nolint:nilnil // start failure is a logged, non-fatal skip
	}
	return mcpService{handle}, nil
}

// mcpCommandLister wraps the neutral always-on command metadata source as a
// zemcp.CommandLister, converting each commandMeta into a zemcp.CommandInfo.
// This is the ONLY conversion from the neutral type to zemcp types; it lives in
// the gated file so always-on API code can adapt the same source without
// pinning the mcp package into the binary.
func mcpCommandLister(src func() ([]commandMeta, error)) zemcp.CommandLister {
	return func() ([]zemcp.CommandInfo, error) {
		metas, err := src()
		if err != nil {
			return nil, err
		}
		if metas == nil {
			return nil, nil
		}
		infos := make([]zemcp.CommandInfo, len(metas))
		for i, m := range metas {
			infos[i] = zemcp.CommandInfo{
				Name:          m.Name,
				ShortHelp:     m.ShortHelp,
				Description:   m.Description,
				ReadOnly:      m.ReadOnly,
				TaskSupport:   parseTaskSupportLevel(m.TaskSupport),
				TakesSelector: m.TakesSelector,
			}
			if len(m.Params) > 0 {
				params := make([]zemcp.ParamInfo, len(m.Params))
				for j, p := range m.Params {
					params[j] = zemcp.ParamInfo{
						Name:        p.Name,
						Type:        p.Type,
						ShortHelp:   p.ShortHelp,
						Description: p.Description,
						Required:    p.Required,
						Anchor:      p.Anchor,
						Flag:        p.Flag,
					}
				}
				infos[i].Params = params
			}
			if m.UIResource != nil {
				infos[i].UIResource = &zemcp.UIResourceInfo{
					Path:        m.UIResource.Path,
					Permissions: m.UIResource.Permissions,
					CSP:         m.UIResource.CSP,
				}
			}
		}
		return infos, nil
	}
}

// mcpConfigToStreamable converts the YANG-derived MCPListenConfig into the
// StreamableConfig that NewStreamable consumes. Fields already populated on
// base (from env vars or CLI flags) are preserved; config-file values only
// fill in blanks.
//
// YANG auth-mode strings are parsed via zemcp.ParseAuthMode; unknown values
// are caught by MCPListenConfig.Validate at `ze config validate` time so
// this path trusts the enum has been pre-validated.
func mcpConfigToStreamable(cfg zeconfig.MCPListenConfig, base zemcp.StreamableConfig) zemcp.StreamableConfig {
	if base.AuthMode == zemcp.AuthUnspecified {
		mode, _ := zemcp.ParseAuthMode(cfg.AuthMode)
		base.AuthMode = mode
	}
	if base.Token == "" {
		base.Token = cfg.Token
	}
	if len(base.BearerList) == 0 && len(cfg.Identities) > 0 {
		entries := make([]zemcp.BearerListEntry, len(cfg.Identities))
		for i, id := range cfg.Identities {
			entries[i] = zemcp.BearerListEntry{Name: id.Name, Token: id.Token, Scopes: id.Scopes}
		}
		base.BearerList = entries
	}
	if base.OAuth.AuthorizationServer == "" {
		base.OAuth.AuthorizationServer = cfg.OAuth.AuthorizationServer
	}
	if base.OAuth.Audience == "" {
		base.OAuth.Audience = cfg.OAuth.Audience
	}
	if len(base.OAuth.RequiredScopes) == 0 {
		base.OAuth.RequiredScopes = cfg.OAuth.RequiredScopes
	}
	return base
}

// parseTaskSupportLevel converts a YANG ze:task-support string to the typed enum.
func parseTaskSupportLevel(s string) zemcp.TaskSupportLevel {
	switch s {
	case "required":
		return zemcp.TaskSupportRequired
	case "forbidden":
		return zemcp.TaskSupportForbidden
	default:
		return zemcp.TaskSupportOptional
	}
}

// mcpServerHandle bundles the running HTTP server with the Streamable handler,
// so the shutdown path can close both. http.Server.Shutdown drains the TCP
// listener. And handler.Close drains the task registry's GC goroutine.
type mcpServerHandle struct {
	Server    *http.Server
	Handler   *zemcp.Streamable
	useTLS    bool
	authMode  zemcp.AuthMode
	listeners map[string]net.Listener
	bound     []string
	mu        sync.RWMutex
	stopped   bool
}

// effectiveMCPAuthMode mirrors NewStreamable's mode inference so startup and
// listener reconfiguration enforce TLS against the mode the handler actually
// runs.
func effectiveMCPAuthMode(cfg zemcp.StreamableConfig) zemcp.AuthMode {
	if cfg.AuthMode != zemcp.AuthUnspecified {
		return cfg.AuthMode
	}
	if cfg.Token != "" {
		return zemcp.AuthBearer
	}
	return zemcp.AuthNone
}

// mcpModeAuthenticates reports whether the typed mode the handler runs gates
// every request, the typed twin of zeconfig.MCPAuthModeAuthenticates.
func mcpModeAuthenticates(mode zemcp.AuthMode) bool {
	switch mode {
	case zemcp.AuthBearer, zemcp.AuthBearerList, zemcp.AuthOAuth:
		return true
	case zemcp.AuthNone, zemcp.AuthUnspecified:
		return false
	default:
		// Open caller-supplied mode: NewStreamable rejects unknown values.
		return false
	}
}

// checkMCPMigrationExposure applies the management-listener exposure rule
// (zeconfig.MCPExposure, the one the boot guard refuses start on) to the
// addresses a listener migration would bind, and answers every breach joined,
// or nil.
func checkMCPMigrationExposure(mode zemcp.AuthMode, useTLS bool, addrs []string) error {
	refusals := zeconfig.MCPExposure(addrs, mcpModeAuthenticates(mode), useTLS).Refusals()
	if len(refusals) == 0 {
		return nil
	}
	errs := make([]error, 0, len(refusals))
	for _, r := range refusals {
		errs = append(errs, r)
	}
	return errors.Join(errs...)
}

// startMCPServer creates and starts an MCP HTTP server bound to every entry
// in addrs. Returns the handle on success, nil on failure (logged,
// non-fatal). Shutdown on the returned server closes every listener; the
// caller MUST also call handler.Close() so the task registry GC goroutine
// exits.
//
// Bind is all-or-nothing: if ANY listener fails to bind, the already-bound
// listeners are closed and the function returns nil.
//
// When tlsCert or tlsKey is non-empty, the server serves HTTPS with the
// PEM-encoded certificate and key file paths supplied, or does not start.
// Setting one without the other is a config error that
// MCPListenConfig.Validate rejects at verify time; loadMCPTLSConfig refuses
// it here too, so a half-named pair never serves plain HTTP. Exposure (a
// non-loopback address without authentication or TLS) is not judged here: the
// boot guard (filterMgmtListener over mcpMgmtListener) filters it out and
// reports it before any service is built, and Reconfigure applies the same
// rule to a listener migration.
//
// Speaks the MCP 2026-07-28 Streamable HTTP profile: POST-only and stateless,
// with per-request metadata, headers and authentication, and no handshake,
// session or server-to-client stream.
func startMCPServer(addrs []string, dispatch zemcp.CommandDispatcher, commands zemcp.CommandLister, mcpCfg zemcp.StreamableConfig, tlsCert, tlsKey string) *mcpServerHandle {
	if len(addrs) == 0 {
		fmt.Fprintln(os.Stderr, "warning: MCP server disabled: no listen addresses")
		return nil
	}

	useTLS := tlsCert != "" || tlsKey != ""
	mode := effectiveMCPAuthMode(mcpCfg)

	// Caller populates auth + token fields; we fill the dispatcher + command
	// lister because they come from the reactor, not from YANG.
	mcpCfg.Dispatch = dispatch
	mcpCfg.Commands = commands
	handler, err := zemcp.NewStreamable(mcpCfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: MCP server disabled: %v\n", err)
		return nil
	}

	srv := &http.Server{
		// Addr is informational; multi-listener serving uses Serve(ln).
		Addr:              addrs[0],
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	if useTLS {
		tlsConf, tlsErr := loadMCPTLSConfig(tlsCert, tlsKey)
		if tlsErr != nil {
			fmt.Fprintf(os.Stderr, "warning: MCP server disabled: TLS: %v\n", tlsErr)
			handler.Close()
			return nil
		}
		srv.TLSConfig = tlsConf
	}

	var lc net.ListenConfig
	lnSlice := make([]net.Listener, 0, len(addrs))
	lnMap := make(map[string]net.Listener, len(addrs))
	bound := make([]string, 0, len(addrs))
	for _, addr := range addrs {
		ln, err := lc.Listen(context.Background(), "tcp", addr)
		if err != nil {
			for _, prev := range lnSlice {
				if closeErr := prev.Close(); closeErr != nil {
					fmt.Fprintf(os.Stderr, "warning: MCP server: close partial listener: %v\n", closeErr)
				}
			}
			fmt.Fprintf(os.Stderr, "warning: MCP server disabled: bind %s: %v\n", addr, err)
			handler.Close()
			return nil
		}
		resolvedAddr := ln.Addr().String()
		lnSlice = append(lnSlice, ln)
		lnMap[resolvedAddr] = ln
		bound = append(bound, resolvedAddr)
	}

	scheme := "http"
	if useTLS {
		scheme = "https"
	}
	for _, ln := range lnSlice {
		go serveMCP(srv, ln, useTLS)
		fmt.Fprintf(os.Stderr, "MCP server listening on %s://%s/\n", scheme, ln.Addr().String())
	}

	return &mcpServerHandle{
		Server:    srv,
		Handler:   handler,
		useTLS:    useTLS,
		authMode:  mode,
		listeners: lnMap,
		bound:     bound,
	}
}

// loadMCPTLSConfig parses the cert + key file pair and returns a tls.Config
// with TLS 1.2 minimum. Cert chains are supported (stdlib X509KeyPair).
//
// Before loading, checkKeyFilePermissions (mcp_keyfile.go) resolves the key
// path one entry at a time and refuses it, for every user including root,
// unless OpenSSH's secure-path rule holds: every directory on the path and
// every symlink followed is owned by root or the user ze runs as, no such
// directory is group- or other-writable unless sticky, and the resolved key
// is a regular file with no group or other permission bit. A symlink that
// passes is followed, so Kubernetes and systemd secret mounts work.
func loadMCPTLSConfig(certFile, keyFile string) (*tls.Config, error) {
	if certFile == "" {
		return nil, opererr.New("MCP TLS key file "+keyFile+" is set without a certificate file",
			"MCP serves HTTPS only with both files, and never falls back to plain HTTP",
			"set environment mcp tls cert to the certificate of that key, then restart ze: "+privilege.RestartRemedy())
	}
	if keyFile == "" {
		return nil, opererr.New("MCP TLS certificate file "+certFile+" is set without a key file",
			"MCP serves HTTPS only with both files, and never falls back to plain HTTP",
			"set environment mcp tls key to the private key of that certificate, then restart ze: "+privilege.RestartRemedy())
	}
	if err := checkKeyFilePermissions(keyFile); err != nil {
		return nil, err
	}
	pair, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("load keypair: %w", err)
	}
	return &tls.Config{
		Certificates: []tls.Certificate{pair},
		MinVersion:   tls.VersionTLS12,
	}, nil
}

// Addresses returns every bound listen address.
func (h *mcpServerHandle) Addresses() []string {
	if h == nil {
		return nil
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make([]string, len(h.bound))
	copy(out, h.bound)
	return out
}

// Shutdown gracefully stops the HTTP server and drains the Streamable's
// task registry. Idempotent through http.Server.Shutdown and
// Streamable.Close. MUST be called before process exit.
func (h *mcpServerHandle) Shutdown(ctx context.Context) error {
	if h == nil {
		return nil
	}
	h.mu.Lock()
	h.stopped = true
	h.mu.Unlock()
	var httpErr error
	if h.Server != nil {
		httpErr = h.Server.Shutdown(ctx)
	}
	if h.Handler != nil {
		h.Handler.Close()
	}
	return httpErr
}

// Reconfigure migrates MCP listeners to a new set of addresses.
func (h *mcpServerHandle) Reconfigure(ctx context.Context, newAddrs []string) error {
	if h == nil {
		return errors.New("MCP server not running")
	}
	if len(newAddrs) == 0 {
		return errors.New("MCP: at least one listen address is required")
	}
	if slices.Contains(newAddrs, "") {
		return errors.New("MCP: listen address must not be empty")
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	if h.stopped {
		return errors.New("MCP server has been shut down")
	}

	if err := checkMCPMigrationExposure(h.authMode, h.useTLS, newAddrs); err != nil {
		return fmt.Errorf("MCP reconfigure: %w", err)
	}

	_, toAdd, toRemove := mcpListenerDiff(h.bound, newAddrs)
	if len(toAdd) == 0 && len(toRemove) == 0 {
		return nil
	}

	var lc net.ListenConfig
	newLns := make([]net.Listener, 0, len(toAdd))
	resolved := make(map[string]string, len(toAdd))
	for _, addr := range toAdd {
		ln, err := lc.Listen(ctx, "tcp", addr)
		if err != nil {
			for _, prev := range newLns {
				if closeErr := prev.Close(); closeErr != nil {
					fmt.Fprintf(os.Stderr, "MCP: close partial listener: %v\n", closeErr)
				}
			}
			return fmt.Errorf("MCP reconfigure bind %s: %w", addr, err)
		}
		newLns = append(newLns, ln)
		resolved[addr] = ln.Addr().String()
	}

	for _, ln := range newLns {
		resolvedAddr := ln.Addr().String()
		h.listeners[resolvedAddr] = ln
		go serveMCP(h.Server, ln, h.useTLS)
	}

	for _, addr := range toRemove {
		if ln, ok := h.listeners[addr]; ok {
			if closeErr := ln.Close(); closeErr != nil {
				fmt.Fprintf(os.Stderr, "MCP close listener %s: %v\n", addr, closeErr)
			}
			delete(h.listeners, addr)
		}
	}

	bound := make([]string, 0, len(newAddrs))
	for _, a := range newAddrs {
		if r, ok := resolved[a]; ok {
			bound = append(bound, r)
		} else if _, ok := h.listeners[a]; ok {
			bound = append(bound, a)
		}
	}
	h.bound = bound
	return nil
}

func mcpListenerDiff(oldAddrs, newAddrs []string) (keep, add, remove []string) {
	oldSet := make(map[string]struct{}, len(oldAddrs))
	for _, a := range oldAddrs {
		oldSet[a] = struct{}{}
	}
	newSet := make(map[string]struct{}, len(newAddrs))
	for _, a := range newAddrs {
		newSet[a] = struct{}{}
	}
	for _, a := range newAddrs {
		if _, exists := oldSet[a]; exists {
			keep = append(keep, a)
		} else {
			add = append(add, a)
		}
	}
	for _, a := range oldAddrs {
		if _, exists := newSet[a]; !exists {
			remove = append(remove, a)
		}
	}
	return keep, add, remove
}

// serveMCP runs the MCP HTTP server on one listener. Started once as a
// lifecycle goroutine per configured address. When useTLS is true,
// srv.TLSConfig is already populated (loadMCPTLSConfig ran in
// startMCPServer) and tls.NewListener wraps ln.
func serveMCP(srv *http.Server, ln net.Listener, useTLS bool) {
	var err error
	if useTLS {
		err = srv.Serve(tls.NewListener(ln, srv.TLSConfig))
	} else {
		err = srv.Serve(ln)
	}
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		fmt.Fprintf(os.Stderr, "warning: MCP server: %v\n", err)
	}
}
