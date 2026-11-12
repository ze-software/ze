// Design: docs/architecture/plugin/show-enrichers.md -- proxy enricher for external plugins

package server

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"sync"
	"time"

	plugipc "github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/show"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

const (
	enrichShowTimeout = 2 * time.Second
	maxEnrichers      = 16
	maxKeyLen         = 128
)

func validateEnricherDecls(enrichers []rpc.EnricherDecl) error {
	if len(enrichers) > maxEnrichers {
		return fmt.Errorf("too many enrichers: %d (max %d)", len(enrichers), maxEnrichers)
	}
	type cmdKey struct{ command, key string }
	seen := make(map[cmdKey]struct{}, len(enrichers))
	for _, e := range enrichers {
		if e.Command == "" {
			return fmt.Errorf("enricher command must not be empty")
		}
		if e.Key == "" || len(e.Key) > maxKeyLen {
			return fmt.Errorf("invalid enricher key %q (must be 1-%d chars)", e.Key, maxKeyLen)
		}
		if !isLowerKebab(e.Key) {
			return fmt.Errorf("invalid enricher key %q (must be kebab-case)", e.Key)
		}
		ck := cmdKey{e.Command, e.Key}
		if _, exists := seen[ck]; exists {
			return fmt.Errorf("duplicate enricher: command=%q key=%q", e.Command, e.Key)
		}
		seen[ck] = struct{}{}
	}
	return nil
}

// proxyRegistry holds the enrichers each plugin registered through this
// proxy. unregisterProxyEnrichers, a process cleanup hook registered from
// init() (register.go), removes a plugin's entries when its process ends.
// Guarded by proxyMu, which no caller holds across an enricher call.
var (
	proxyMu       sync.Mutex
	proxyRegistry = map[string][]proxyEntry{}
)

// proxyEntry is one registered enricher and the connection of the process
// that registered it, which its calls go through.
type proxyEntry struct {
	command string
	key     string
	conn    *plugipc.PluginConn
}

func registerProxyEnrichers(pluginName string, enrichers []rpc.EnricherDecl, conn *plugipc.PluginConn) {
	proxyMu.Lock()
	defer proxyMu.Unlock()

	// A new process starts with no enricher failure. The cleanup of the
	// process it replaces forgot the plugin's failures, but a call that
	// process's stop cut can record its failure after that: Process.Stop
	// cancels the context the cleanup waits on before it closes the bridge
	// the call waits on. Forgotten here, under proxyMu, the record cannot
	// outlive the registration, and recordProxyCallFailure refuses one that
	// arrives after it.
	forgetEnricherFailures(pluginName)
	for _, decl := range enrichers {
		detail := makeProxyCall(pluginName, decl.Command, decl.Key, "detail", conn)
		brief := makeProxyCall(pluginName, decl.Command, decl.Key, "brief", conn)
		if err := show.Register(decl.Command, decl.Key, show.Enricher{Detail: detail, Brief: brief}); err != nil {
			logger().Warn("proxy enricher registration failed",
				"plugin", pluginName, "command", decl.Command, "key", decl.Key, "error", err,
				"code", diagnostic.CodeDoctorPluginEnricherUnavailable)
			recordEnricherFailure(pluginName, EnricherFailure{
				Command: decl.Command, Key: decl.Key, Registration: true, Error: err.Error(),
			})
			continue
		}
		proxyRegistry[pluginName] = append(proxyRegistry[pluginName], proxyEntry{
			command: decl.Command,
			key:     decl.Key,
			conn:    conn,
		})
		logger().Debug("proxy enricher registered",
			"plugin", pluginName, "command", decl.Command, "key", decl.Key)
	}
}

// replacedProxyConn reports whether the plugin's enrichers are registered by
// a process other than conn's: a later process replaced it. The caller MUST
// hold proxyMu.
//
// A plugin with no registered enricher is not replaced: its process ended and
// its cleanup ran, and the next registration forgets what is recorded
// meanwhile. makeProxyCall built without a registration (a test) records as
// before.
func replacedProxyConn(pluginName string, conn *plugipc.PluginConn) bool {
	entries := proxyRegistry[pluginName]
	return len(entries) > 0 && !slices.ContainsFunc(entries, func(e proxyEntry) bool { return e.conn == conn })
}

// recordProxyCallFailure records a failed call through conn, unless conn's
// process was replaced. That failure belongs to the old process (a call its
// stop cut, or one a `show` reached through the enricher it read before the
// restart), whose cleanup forgot its failures, and recording it would blame
// the replacement.
func recordProxyCallFailure(pluginName string, conn *plugipc.PluginConn, failure EnricherFailure) {
	proxyMu.Lock()
	defer proxyMu.Unlock()
	if replacedProxyConn(pluginName, conn) {
		return
	}
	recordEnricherFailure(pluginName, failure)
}

// clearProxyCallFailure clears the call failure of one enricher once a call
// through conn succeeded, unless conn's process was replaced: the old
// process answering says nothing of the replacement's enricher.
func clearProxyCallFailure(pluginName string, conn *plugipc.PluginConn, command, key string) {
	proxyMu.Lock()
	defer proxyMu.Unlock()
	if replacedProxyConn(pluginName, conn) {
		return
	}
	clearEnricherCallFailure(pluginName, command, key)
}

func makeProxyCall(pluginName, command, key, mode string, conn *plugipc.PluginConn) func(base map[string]any) {
	return func(base map[string]any) {
		ctx, cancel := context.WithTimeout(context.Background(), enrichShowTimeout)
		defer cancel()

		input := &rpc.EnrichShowInput{
			Command: command,
			Key:     key,
			Mode:    mode,
			Base:    base,
		}

		out, err := conn.SendEnrichShow(ctx, input)
		if err != nil {
			logger().Warn("proxy enricher call failed",
				"plugin", pluginName, "command", command, "key", key, "error", err,
				"code", diagnostic.CodeDoctorPluginEnricherUnavailable)
			recordProxyCallFailure(pluginName, conn, EnricherFailure{Command: command, Key: key, Error: err.Error()})
			return
		}
		clearProxyCallFailure(pluginName, conn, command, key)
		maps.Copy(base, out.Data)
	}
}

// unregisterProxyEnrichers runs for every process that ends: one that
// registered no enricher has no entry and no failure, so it does nothing.
func unregisterProxyEnrichers(processName string) {
	proxyMu.Lock()
	entries, ok := proxyRegistry[processName]
	if ok {
		delete(proxyRegistry, processName)
	}
	proxyMu.Unlock()
	forgetEnricherFailures(processName)

	if !ok {
		return
	}
	for _, e := range entries {
		show.Unregister(e.command, e.key)
		logger().Debug("proxy enricher unregistered",
			"plugin", processName, "command", e.command, "key", e.key)
	}
}
