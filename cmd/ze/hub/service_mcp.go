// Design: ai/rules/feature-gate-registration.md -- compile-out-able services (feature-gate)
// Related: main.go -- resolves MCP listen/token/config to plain values and feeds them via ServiceDeps.MCP
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
)

// mcpService adapts *MCPServerHandle to the Service interface (the handle
// already satisfies Reconfigurable + Shutdown; only Name is added).
type mcpService struct {
	*MCPServerHandle
}

func (mcpService) Name() string { return "mcp" }

// buildMCPService builds and starts the MCP HTTP server from deps. It returns a
// nil Service (not an error) when MCP is not configured or fails to start --
// preserving the prior best-effort, non-fatal behavior of the inline
// startMCPServer call in main.go.
func buildMCPService(deps ServiceDeps) (Service, error) {
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
func mcpCommandLister(src func() []commandMeta) zemcp.CommandLister {
	return func() []zemcp.CommandInfo {
		metas := src()
		if metas == nil {
			return nil
		}
		infos := make([]zemcp.CommandInfo, len(metas))
		for i, m := range metas {
			infos[i] = zemcp.CommandInfo{
				Name:          m.Name,
				Help:          m.Help,
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
						Description: p.Description,
						Required:    p.Required,
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
		return infos
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

// MCPServerHandle bundles the running HTTP server with the Streamable handler
// so the shutdown path can close both: http.Server.Shutdown drains the TCP
// listener, handler.Close drains the session registry's GC goroutine.
type MCPServerHandle struct {
	Server    *http.Server
	Handler   *zemcp.Streamable
	useTLS    bool
	listeners map[string]net.Listener
	bound     []string
	mu        sync.RWMutex
	stopped   bool
}

// startMCPServer creates and starts an MCP HTTP server bound to every entry
// in addrs. Returns the handle on success, nil on failure (logged,
// non-fatal). Shutdown on the returned server closes every listener; the
// caller MUST also call handler.Close() so the session registry GC goroutine
// exits.
//
// Bind is all-or-nothing: if ANY listener fails to bind, the already-bound
// listeners are closed and the function returns nil.
//
// When tlsCert + tlsKey are non-empty, the server serves HTTPS with the
// PEM-encoded certificate and key file paths supplied. Setting one without
// the other is a config error; MCPListenConfig.Validate rejects that at
// verify time, so we trust the pair is complete here.
//
// Speaks the MCP 2025-06-18 Streamable HTTP profile (sessions, SSE, GET/DELETE).
func startMCPServer(addrs []string, dispatch zemcp.CommandDispatcher, commands zemcp.CommandLister, mcpCfg zemcp.StreamableConfig, tlsCert, tlsKey string) *MCPServerHandle {
	if len(addrs) == 0 {
		fmt.Fprintln(os.Stderr, "warning: MCP server disabled: no listen addresses")
		return nil
	}

	// Caller populates auth + token fields; we fill the dispatcher + command
	// lister because they come from the reactor, not from YANG.
	mcpCfg.Dispatch = dispatch
	mcpCfg.Commands = commands
	handler, err := zemcp.NewStreamable(mcpCfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: MCP server disabled: %v\n", err)
		return nil
	}

	useTLS := tlsCert != "" && tlsKey != ""
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

	return &MCPServerHandle{
		Server:    srv,
		Handler:   handler,
		useTLS:    useTLS,
		listeners: lnMap,
		bound:     bound,
	}
}

// loadMCPTLSConfig parses the cert + key file pair and returns a tls.Config
// with TLS 1.2 minimum. Cert chains are supported (stdlib X509KeyPair).
//
// Refuses key files whose permissions grant any access to group or other
// (mode bits & 0o077 != 0) when ze is running as non-root. Defense in depth:
// a world-readable private key on a shared host is a misconfiguration the
// daemon should catch at startup, not a runtime quirk to document.
func loadMCPTLSConfig(certFile, keyFile string) (*tls.Config, error) {
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

// checkKeyFilePermissions rejects private-key files that are readable by
// group or other (mode bits & 0o077 != 0) or that are symlinks. Only
// enforced when running as a non-root user -- root can already read any
// file and the check is cosmetic in that case.
//
// Symlinks are rejected outright: an attacker who can write to the
// key-file's parent directory could otherwise swap a strict-perm symlink
// pointing at a world-readable target, and Stat's follow-through would pass
// the 0o077 mask. Refusing symlinks forces the operator to store the key
// as a regular file in a root-only directory.
func checkKeyFilePermissions(keyFile string) error {
	if os.Geteuid() == 0 {
		return nil
	}
	info, err := os.Lstat(keyFile)
	if err != nil {
		return fmt.Errorf("stat key file %q: %w", keyFile, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("key file %q: symlinks are not permitted", keyFile)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("key file %q: must be a regular file", keyFile)
	}
	perm := info.Mode().Perm()
	if perm&0o077 != 0 {
		return fmt.Errorf(
			"key file %q has mode %04o; group/other permissions must be cleared (chmod 600)",
			keyFile, perm,
		)
	}
	return nil
}

// Addresses returns every bound listen address.
func (h *MCPServerHandle) Addresses() []string {
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
// session registry. Idempotent through http.Server.Shutdown and
// Streamable.Close. MUST be called before process exit.
func (h *MCPServerHandle) Shutdown(ctx context.Context) error {
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
func (h *MCPServerHandle) Reconfigure(ctx context.Context, newAddrs []string) error {
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
