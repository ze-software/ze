// Design: docs/architecture/hub-architecture.md -- MCP server startup
// Overview: main.go -- hub CLI entry point

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
	"strings"
	"sync"
	"time"

	zeconfig "codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/component/config/yang"
	zemcp "codeberg.org/thomas-mangin/ze/internal/component/mcp"
)

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

// buildParamMap extracts all RPC metadata from the YANG loader and builds
// a map from CLI command path to input parameters.
func buildParamMap(loader *yang.Loader) map[string][]zemcp.ParamInfo {
	if loader == nil {
		return nil
	}

	// Build reverse map: CLI path -> wire method.
	wireToPath := yang.WireMethodToPath(loader)
	pathToWire := make(map[string]string, len(wireToPath))
	for wire, path := range wireToPath {
		pathToWire[path] = wire
	}

	// Extract RPC input params for each command path.
	result := make(map[string][]zemcp.ParamInfo)
	for path, wire := range pathToWire {
		// Wire method format: "module:rpc-name". Extract module, add "-api" suffix.
		module := wireModule(wire)
		rpcName := wireRPC(wire)
		if module == "" || rpcName == "" {
			continue
		}

		rpcs := yang.ExtractRPCs(loader, module+"-api")
		if rpcs == nil {
			// Try without -api suffix (some modules use -cmd).
			rpcs = yang.ExtractRPCs(loader, module+"-cmd")
		}
		for _, rpc := range rpcs {
			if rpc.Name != rpcName {
				continue
			}
			if len(rpc.Input) == 0 {
				break
			}
			params := make([]zemcp.ParamInfo, len(rpc.Input))
			for i, leaf := range rpc.Input {
				params[i] = zemcp.ParamInfo{
					Name:        leaf.Name,
					Type:        leaf.Type,
					Description: leaf.Description,
					Required:    leaf.Mandatory,
				}
			}
			result[path] = params
			break
		}
	}

	return result
}

// buildTaskSupportMap extracts ze:task-support values from the YANG loader.
func buildTaskSupportMap(loader *yang.Loader) map[string]string {
	if loader == nil {
		return nil
	}
	return yang.PathToTaskSupport(loader)
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

// lookupUIResource checks if a command path or any of its parent paths has
// a ze:ui-resource annotation. Commands like "peer list" inherit the UI
// resource from the "peer" grouping container.
func lookupUIResource(cmdPath string, m map[string]yang.UIResourceEntry) (yang.UIResourceEntry, bool) {
	if m == nil {
		return yang.UIResourceEntry{}, false
	}
	if info, ok := m[cmdPath]; ok {
		return info, true
	}
	for {
		idx := strings.LastIndex(cmdPath, " ")
		if idx < 0 {
			break
		}
		cmdPath = cmdPath[:idx]
		if info, ok := m[cmdPath]; ok {
			return info, true
		}
	}
	return yang.UIResourceEntry{}, false
}

// wireModule extracts the module prefix from a wire method (e.g. "ze-bgp:peer-list" -> "ze-bgp").
func wireModule(wire string) string {
	mod, _, ok := strings.Cut(wire, ":")
	if !ok {
		return ""
	}
	return mod
}

// wireRPC extracts the RPC name from a wire method (e.g. "ze-bgp:peer-list" -> "peer-list").
func wireRPC(wire string) string {
	_, rpc, ok := strings.Cut(wire, ":")
	if !ok {
		return ""
	}
	return rpc
}

// MCPServerHandle bundles the running HTTP server with the Streamable handler
// so the shutdown path can close both: http.Server.Shutdown drains the TCP
// listener, handler.Close drains the session registry's GC goroutine. Phase
// 2 resolution of the Phase 1 deferral (`plan/deferrals.md` row 226).
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
