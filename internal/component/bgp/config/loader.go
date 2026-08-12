// Design: docs/architecture/config/syntax.md — config file loading and reactor creation
// Detail: loader_routes.go — BGP route type conversion
// Detail: loader_prefix.go — prefix expansion for route splitting
// Detail: loader_create.go — reactor creation from config tree
// Detail: plugins.go — plugin extraction from config tree

package bgpconfig

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ze-software/ze/internal/chaos"
	"github.com/ze-software/ze/internal/component/bgp/grmarker"
	"github.com/ze-software/ze/internal/component/bgp/reactor"
	"github.com/ze-software/ze/internal/component/config"
	_ "github.com/ze-software/ze/internal/component/config/migration" // init() registers migration function
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/cliio"
	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/internal/core/network"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// configLogger is the config subsystem logger (lazy initialization).
// Controlled by ze.log.bgp.config environment variable.
// Uses LazyLogger to pick up config file settings applied after init().
var configLogger = slogutil.LazyLogger("bgp.config")

// LoadReactor parses config and creates a configured Reactor.
func LoadReactor(input string) (*reactor.Reactor, error) {
	result, err := config.LoadConfig(input, "", nil)
	if err != nil {
		return nil, err
	}
	return CreateReactorFromTree(result.Tree, "", "", result.Plugins, nil, false)
}

// loadContext stores the config.LoadConfigResult and Storage for in-process BGP plugin use.
// Set by the hub after LoadConfig. Retrieved by BGP's RunEngine to create the reactor
// without re-parsing the config.
var loadContext struct {
	result     *config.LoadConfigResult
	configPath string
	store      any // storage.Storage (any to avoid import cycle)
}

// StoreLoadContext saves the LoadConfigResult and storage for retrieval by
// the BGP plugin's RunEngine. Must be called after LoadConfig.
func StoreLoadContext(result *config.LoadConfigResult, configPath string, store any) {
	loadContext.result = result
	loadContext.configPath = configPath
	loadContext.store = store
}

// GetLoadContext returns the stored LoadConfigResult, config path, and storage.
// Returns nil result if StoreLoadContext was not called.
func GetLoadContext() (*config.LoadConfigResult, string, any) {
	return loadContext.result, loadContext.configPath, loadContext.store
}

// chaosConfig stores chaos testing parameters for BGP plugin injection.
var chaosConfig struct {
	seed int64
	rate float64
}

// StoreLoadChaos saves chaos config for retrieval by the BGP plugin's RunEngine.
func StoreLoadChaos(seed int64, rate float64) {
	chaosConfig.seed = seed
	chaosConfig.rate = rate
}

// GetLoadChaos returns the stored chaos config (seed, rate). Zero seed means disabled.
func GetLoadChaos() (int64, float64) {
	return chaosConfig.seed, chaosConfig.rate
}

// CreateReactor creates a Reactor from a config.LoadConfigResult. standalone
// selects self-hosting mode (see reactor.Config.Standalone); production callers
// pass false so the reactor borrows the hub-owned plugin server.
func CreateReactor(cfg *config.LoadConfigResult, configPath string, store storage.Storage, standalone bool) (*reactor.Reactor, error) {
	r, err := CreateReactorFromTree(cfg.Tree, cfg.ConfigDir, configPath, cfg.Plugins, store, standalone)
	if err != nil {
		return nil, err
	}

	if configPath != "" && configPath != "-" {
		r.SetConfigPath(configPath)
		r.SetReloadFunc(createReloadFunc(store, r))
	}

	return r, nil
}

// LoadReactorWithPlugins parses config with CLI plugins and creates a borrow-mode
// (production) Reactor that expects a hub-injected plugin server before start.
func LoadReactorWithPlugins(store storage.Storage, input, configPath string, cliPlugins []string) (*reactor.Reactor, error) {
	cfg, err := config.LoadConfig(input, configPath, cliPlugins)
	if err != nil {
		return nil, err
	}
	return CreateReactor(cfg, configPath, store, false)
}

// LoadReactorWithPluginsStandalone is LoadReactorWithPlugins for callers that own
// the reactor lifecycle and self-host the plugin server (the ze-chaos in-process
// simulation). The reactor creates its own server, signal handler, and starts
// peers inline instead of borrowing a hub-owned server.
func LoadReactorWithPluginsStandalone(store storage.Storage, input, configPath string, cliPlugins []string) (*reactor.Reactor, error) {
	cfg, err := config.LoadConfig(input, configPath, cliPlugins)
	if err != nil {
		return nil, err
	}
	return CreateReactor(cfg, configPath, store, true)
}

// LoadReactorFile loads config from file and creates Reactor.
func LoadReactorFile(store storage.Storage, path string) (*reactor.Reactor, error) {
	return LoadReactorFileWithPlugins(store, path, nil)
}

// LoadReactorFileWithPlugins loads config from file and creates a borrow-mode
// (production) Reactor.
func LoadReactorFileWithPlugins(store storage.Storage, path string, cliPlugins []string) (*reactor.Reactor, error) {
	return loadReactorFile(store, path, cliPlugins)
}

// loadReactorFile reads the config at path and creates a borrow-mode (production)
// reactor. A standalone reactor comes from LoadReactorWithPluginsStandalone, whose
// callers hold the config text rather than a path.
func loadReactorFile(store storage.Storage, path string, cliPlugins []string) (*reactor.Reactor, error) {
	// "-" reads stdin (claiming it once); a real path goes through the storage
	// abstraction, which may be a blob store where path is a key, not a file.
	var data []byte
	var err error

	if cliio.IsStdin(path) {
		data, err = cliio.ReadFile(path)
	} else {
		data, err = store.ReadFile(path)
	}
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	pluginYANG := plugin.CollectPluginYANG(cliPlugins)

	if v, vErr := config.YANGValidatorWithPlugins(pluginYANG); vErr == nil && v != nil {
		plugin.SetYANGValidator(v)
	}

	cfg, err := config.LoadConfig(string(data), path, cliPlugins)
	if err != nil {
		return nil, err
	}

	return CreateReactor(cfg, path, store, false)
}

// injectChaos wraps the reactor's clock, dialer, and listener with chaos fault injection
// when the coordinator has non-zero chaos seed stored by the hub.
func injectChaos(r *reactor.Reactor, coord registry.CoordinatorAccessor) {
	bs := coord.Bootstrap()
	seed := bs.ChaosSeed
	if seed == 0 {
		return
	}
	rate := bs.ChaosRate
	if rate < 0 {
		rate = 0.1
	}
	resolvedSeed := chaos.ResolveSeed(seed)
	chaosLogger := slogutil.Logger("chaos")
	cfg := chaos.ChaosConfig{Seed: resolvedSeed, Rate: rate, Logger: chaosLogger}
	c, d, l := chaos.NewChaosWrappers(clock.RealClock{}, &network.RealDialer{}, network.RealListenerFactory{}, cfg)
	r.SetClock(c)
	r.SetDialer(d)
	r.SetListenerFactory(l)
	chaosLogger.Info("chaos self-test mode enabled", "seed", resolvedSeed, "rate", rate)
}

// readGRMarker reads and removes the Graceful Restart marker from storage.
// RFC 4724 Section 4.1: reactor uses the expiry to set R bit in OPEN capabilities.
func readGRMarker(r *reactor.Reactor, store storage.Storage) {
	if store == nil {
		return
	}
	if expiry, ok := grmarker.Read(store); ok {
		r.SetRestartUntil(expiry)
		slogutil.Logger("bgp.gr").Info("GR restart marker found", "expires", expiry)
	}
	if err := grmarker.Remove(store); err != nil {
		slogutil.Logger("bgp.gr").Warn("failed to remove GR marker", "error", err)
	}
}

// formatResponseData converts a command response Data value to a human-readable string.
// Strings pass through directly. Maps and other complex types are JSON-encoded with indentation.
func formatResponseData(data any) string {
	if data == nil {
		return ""
	}
	if s, ok := data.(string); ok {
		return s
	}
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Sprintf("%v", data)
	}
	return string(b)
}

// collectPrefixWarnings gathers BGP-sourced prefix warnings for the login banner.
// Reads from the report bus instead of per-peer state. Two kinds: stale prefix
// data (raised at peer add via report.RaiseWarning) and active threshold
// exceeded (raised by Session.applyPrefixCheck on the upward edge).
//
// If exactly one warning exists, the specific detail is shown.
// If more than one, a count is shown with the command to investigate.
//
// Malformed prefix-threshold subjects (missing the composite "<addr>/<family>"
// form) are skipped with a debug log rather than producing visually broken
// banner lines. Producers in this codebase always use the composite form;
// the skip handles future producers and protects the operator UI.
func collectPrefixWarnings(rl plugin.ReactorIntrospector) []LoginWarning {
	peerNames := buildPeerNameLookup(rl)
	issues := report.Warnings()

	var warnings []LoginWarning
	for i := range issues {
		issue := &issues[i]
		if issue.Source != "bgp" {
			continue
		}
		switch issue.Code {
		case "prefix-stale":
			label := peerLabelFromSubject(issue.Subject, peerNames)
			var tb textbuf.Buffer
			warnings = append(warnings, LoginWarning{
				Message: tb.Str(label).Str(" has stale prefix data (updated ").Str(detailString(issue.Detail, "updated")).Byte(')').String(),
				Command: tb.Reset().Str("set bgp peer ").Str(issue.Subject).Str(" prefix").String(),
			})
		case "prefix-threshold":
			peerAddr, fam, ok := splitThresholdSubject(issue.Subject)
			if !ok {
				configLogger().Debug("skipping malformed prefix-threshold subject in banner",
					"subject", issue.Subject)
				continue
			}
			label := peerLabelFromSubject(peerAddr, peerNames)
			var tb textbuf.Buffer
			warnings = append(warnings, LoginWarning{
				Message: tb.Str(label).Byte(' ').Str(fam).Str(" prefix count exceeds warning threshold").String(),
			})
		}
	}

	if len(warnings) == 0 {
		return nil
	}
	if len(warnings) == 1 {
		return warnings
	}
	return []LoginWarning{{
		Message: textbuf.IntStr(int64(len(warnings)), " warnings"),
		Command: "show warnings",
	}}
}

// buildPeerNameLookup walks the reactor peers once to build a peer-address-to-
// peer map, used to enrich bus warnings (which only carry the peer address)
// with the human-readable peer name from config. Stores PeerInfo by value
// (not pointer) so the map's lifetime does not depend on the lifetime of the
// slice returned by rl.Peers().
func buildPeerNameLookup(rl plugin.ReactorIntrospector) map[string]plugin.PeerInfo {
	if rl == nil {
		return nil
	}
	peers := rl.Peers()
	out := make(map[string]plugin.PeerInfo, len(peers))
	for i := range peers {
		out[peers[i].Address.String()] = peers[i]
	}
	return out
}

// peerLabelFromSubject returns a human-readable peer label given the peer
// address from a bus subject and the name lookup map. Falls back to the
// raw address when the peer is not found in the lookup (e.g., already removed).
func peerLabelFromSubject(addr string, lookup map[string]plugin.PeerInfo) string {
	if p, ok := lookup[addr]; ok {
		return peerLabel(&p)
	}
	var tb textbuf.Buffer
	return tb.Str("peer ").Str(addr).String()
}

// splitThresholdSubject parses the composite subject "<addr>/<afi>/<safi>"
// into peer address and family string. Returns ok=false when the format
// does not match (no "/", or "/" at start or end). Callers must check ok
// before using the returned values; malformed subjects should be skipped
// rather than producing broken UI text.
func splitThresholdSubject(subject string) (peerAddr, family string, ok bool) {
	idx := strings.Index(subject, "/")
	if idx <= 0 || idx == len(subject)-1 {
		return subject, "", false
	}
	return subject[:idx], subject[idx+1:], true
}

// detailString returns the string value of detail[key], or "" if missing or
// not a string. Used for safe extraction of bus-detail fields in user-facing text.
func detailString(detail map[string]any, key string) string {
	if detail == nil {
		return ""
	}
	v, ok := detail[key]
	if !ok {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// peerLabel returns a human-readable label for a peer (name or IP + AS).
func peerLabel(p *plugin.PeerInfo) string {
	var b textbuf.Buffer
	if p.Name != "" {
		return b.Reset().Str("peer ").Str(p.Name).Str(" (AS").Int(int64(p.PeerAS)).Byte(')').String()
	}
	return b.Str("peer ").Addr(p.Address).Str(" (AS").Int(int64(p.PeerAS)).Byte(')').String()
}
