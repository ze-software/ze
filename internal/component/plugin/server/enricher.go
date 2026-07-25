// Design: plan/learned/895-show-enricher-v2.md -- proxy enricher for external plugins

package server

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"sync"
	"time"

	plugipc "github.com/ze-software/ze/internal/component/plugin/ipc"
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

var (
	proxyMu       sync.Mutex
	proxyRegistry = map[string][]proxyEntry{}
	proxyInitOnce sync.Once
)

type proxyEntry struct {
	command string
	key     string
}

func registerProxyEnrichers(pluginName string, enrichers []rpc.EnricherDecl, conn *plugipc.PluginConn) {
	proxyInitOnce.Do(func() {
		RegisterProcessCleanup(unregisterProxyEnrichers)
	})

	proxyMu.Lock()
	defer proxyMu.Unlock()

	for _, decl := range enrichers {
		detail := makeProxyCall(pluginName, decl.Command, decl.Key, "detail", conn)
		brief := makeProxyCall(pluginName, decl.Command, decl.Key, "brief", conn)
		if err := show.Register(decl.Command, decl.Key, show.Enricher{Detail: detail, Brief: brief}); err != nil {
			logger().Warn("proxy enricher registration failed",
				"plugin", pluginName, "command", decl.Command, "key", decl.Key, "error", err)
			continue
		}
		proxyRegistry[pluginName] = append(proxyRegistry[pluginName], proxyEntry{
			command: decl.Command,
			key:     decl.Key,
		})
		logger().Debug("proxy enricher registered",
			"plugin", pluginName, "command", decl.Command, "key", decl.Key)
	}
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
			slog.Warn("proxy enricher call failed",
				"plugin", pluginName, "command", command, "key", key, "error", err)
			return
		}
		maps.Copy(base, out.Data)
	}
}

func unregisterProxyEnrichers(processName string) {
	proxyMu.Lock()
	entries, ok := proxyRegistry[processName]
	if ok {
		delete(proxyRegistry, processName)
	}
	proxyMu.Unlock()

	if !ok {
		return
	}
	for _, e := range entries {
		show.Unregister(e.command, e.key)
		logger().Debug("proxy enricher unregistered",
			"plugin", processName, "command", e.command, "key", e.key)
	}
}
