//go:build ze_telemetry

// Package exporter is the compile-out-able Prometheus HTTP exporter for Ze.
//
// It is gated behind the ze_telemetry build tag. The metric COLLECTION API in
// internal/core/metrics (Registry, PrometheusRegistry, the NopRegistry dummy,
// Gauge, Counter) stays always-on; only this HTTP EXPOSURE is optional. The
// hub's register_telemetry.go (//go:build ze_telemetry) wires Start into the
// always-on metrics.StartExporter seam so the standalone hub path and the bgp
// reactor path can serve metrics without importing this package. Start is the
// only exported symbol: every other type/func here is package-internal.
//
// Design: docs/architecture/core-design.md (Prometheus HTTP server).
package exporter

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/ze-software/ze/internal/core/health"
	"github.com/ze-software/ze/internal/core/metrics"
)

// endpoint is one parsed "address:port" listen entry for the metrics server.
type endpoint struct {
	Host string
	Port int
}

// joinHostPort returns the endpoint as a Go net.JoinHostPort string.
func (e endpoint) joinHostPort() string {
	return net.JoinHostPort(e.Host, strconv.Itoa(e.Port))
}

// telemetryConfig holds the parsed telemetry.prometheus block.
// Endpoints is guaranteed non-empty when Enabled is true.
// Entries are returned in YANG list key order (sorted alphabetically when
// the configuration tree does not preserve order, e.g. after ToMap()).
type telemetryConfig struct {
	Enabled           bool
	Endpoints         []endpoint
	Path              string
	BasicAuth         basicAuthConfig
	Netdata           netdataConfig
	DeprecatedAliases []string
}

// basicAuthConfig holds optional HTTP Basic Authentication settings for the
// Prometheus HTTP service.
type basicAuthConfig struct {
	Enabled    bool
	Realm      string
	Username   string
	BcryptHash string
}

// netdataConfig holds Netdata-compatible OS collector settings. These settings
// do not affect Ze-native Prometheus metrics such as ze_bgp_* or ze_bfd_*.
type netdataConfig struct {
	Enabled    bool
	Prefix     string
	Interval   int
	Collectors map[string]collectorConfig
}

// collectorConfig holds per-collector overrides from the YANG list.
type collectorConfig struct {
	Enabled  bool
	Interval int
}

const (
	defaultTelemetryHost  = "127.0.0.1"
	defaultPrometheusPath = "/metrics"
	defaultBasicAuthRealm = "ze prometheus"
	defaultNetdataPrefix  = "netdata"
	defaultNetdataSeconds = 1
)

// server serves Prometheus metrics over HTTP on one or more listeners.
// start binds every entry in the supplied endpoint slice; close closes all of
// them because they are registered with the same *http.Server.
type server struct {
	httpServer *http.Server
}

// start binds every endpoint in cfg and begins serving Prometheus metrics
// from the given registry at cfg.Path.
//
// Bind is all-or-nothing: if ANY listener fails to bind, the already-bound
// listeners are closed and start returns the bind error without entering
// the serve loop.
func (s *server) start(registry *metrics.PrometheusRegistry, cfg telemetryConfig) error {
	if len(cfg.Endpoints) == 0 {
		return errors.New("metrics server: at least one endpoint is required")
	}
	if err := cfg.BasicAuth.validate(); err != nil {
		return err
	}
	path := cfg.Path
	if path == "" {
		path = defaultPrometheusPath
	}

	mux := http.NewServeMux()
	mux.Handle(path, registry.Handler())
	if path != "/health" {
		mux.Handle("/health", health.DefaultRegistry.Handler())
	}
	handler := http.Handler(mux)
	if cfg.BasicAuth.Enabled {
		handler = basicAuthMiddleware(cfg.BasicAuth, handler)
	}

	s.httpServer = &http.Server{
		// Addr is informational; multi-listener serving uses Serve(ln).
		Addr:              cfg.Endpoints[0].joinHostPort(),
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	var lc net.ListenConfig
	listeners := make([]net.Listener, 0, len(cfg.Endpoints))
	for _, ep := range cfg.Endpoints {
		ln, err := lc.Listen(context.Background(), "tcp", ep.joinHostPort())
		if err != nil {
			for _, prev := range listeners {
				if closeErr := prev.Close(); closeErr != nil {
					// Best-effort rollback; carry on closing the rest.
					_ = closeErr
				}
			}
			s.httpServer = nil
			return fmt.Errorf("metrics server listen %s: %w", ep.joinHostPort(), err)
		}
		listeners = append(listeners, ln)
	}

	// Lifecycle goroutine per listener. Every listener is registered with
	// the same *http.Server so close() closes all of them.
	var wg sync.WaitGroup
	for _, ln := range listeners {
		wg.Add(1)
		go func(ln net.Listener) {
			defer wg.Done()
			if serveErr := s.httpServer.Serve(ln); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
				// Serve failures on metrics are not fatal; swallow and let
				// the other listeners keep running. close() ultimately
				// stops the server.
				_ = serveErr
			}
		}(ln)
	}

	return nil
}

// close shuts down the HTTP server. Safe to call without start and idempotent
// (http.Server.Close is idempotent in the Go stdlib).
func (s *server) close() error {
	if s.httpServer == nil {
		return nil
	}
	return s.httpServer.Close()
}

func (cfg basicAuthConfig) validate() error {
	if !cfg.Enabled {
		return nil
	}
	if cfg.Username == "" {
		return errors.New("metrics server basic-auth: username is required")
	}
	if cfg.BcryptHash == "" {
		return errors.New("metrics server basic-auth: password is required")
	}
	if _, err := bcrypt.Cost([]byte(cfg.BcryptHash)); err != nil {
		return fmt.Errorf("metrics server basic-auth: password must be a bcrypt hash: %w", err)
	}
	return nil
}

func basicAuthMiddleware(auth basicAuthConfig, next http.Handler) http.Handler {
	realm := auth.Realm
	if realm == "" {
		realm = defaultBasicAuthRealm
	}
	challenge := basicAuthChallenge(realm)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		if ok && basicAuthAccepted(auth, username, password) {
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Set("WWW-Authenticate", challenge)
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
	})
}

func basicAuthAccepted(auth basicAuthConfig, username, password string) bool {
	usernameOK := subtle.ConstantTimeCompare([]byte(username), []byte(auth.Username)) == 1
	passwordOK := bcrypt.CompareHashAndPassword([]byte(auth.BcryptHash), []byte(password)) == nil
	return usernameOK && passwordOK
}

func basicAuthChallenge(realm string) string {
	escaped := strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(realm)
	return `Basic realm="` + escaped + `"`
}

// extractTelemetryConfig extracts Prometheus telemetry settings from a config
// tree. The service must be explicitly enabled via the enabled leaf (default
// false). Every YANG `list server {}` entry is returned as an endpoint in
// alphabetical key order; when the list is empty, one default entry using
// the YANG refine defaults is synthesized so the binder always sees at least
// one endpoint.
func extractTelemetryConfig(tree map[string]any) telemetryConfig {
	var zero telemetryConfig
	if tree == nil {
		return zero
	}
	telemetry, ok := tree["telemetry"].(map[string]any)
	if !ok {
		return zero
	}
	prom, ok := telemetry["prometheus"].(map[string]any)
	if !ok {
		return zero
	}

	// Service must be explicitly enabled (default false).
	enabled, _ := prom["enabled"].(string)
	if enabled != "true" {
		return zero
	}

	cfg := telemetryConfig{Enabled: true}

	// Extract path (default: /metrics).
	cfg.Path, _ = prom["path"].(string)
	if cfg.Path == "" {
		cfg.Path = defaultPrometheusPath
	}

	cfg.BasicAuth = extractBasicAuthConfig(prom)
	cfg.Netdata, cfg.DeprecatedAliases = extractNetdataConfig(prom)

	// Read every server list entry in alphabetical key order. ToMap() loses
	// the original insertion order, so alphabetical is the best deterministic
	// substitute; users who care about order should name entries accordingly
	// (primary, secondary, ...).
	if serverMap, ok := prom["server"].(map[string]any); ok && len(serverMap) > 0 {
		keys := make([]string, 0, len(serverMap))
		for k := range serverMap {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, key := range keys {
			srv, ok := serverMap[key].(map[string]any)
			if !ok {
				continue
			}
			ep := endpoint{Host: defaultTelemetryHost, Port: 9273}
			if v, ok := srv["ip"].(string); ok && v != "" {
				ep.Host = v
			}
			if portStr, ok := srv["port"].(string); ok {
				if n, err := strconv.Atoi(portStr); err == nil && n >= 1 && n <= 65535 {
					ep.Port = n
				}
			}
			cfg.Endpoints = append(cfg.Endpoints, ep)
		}
	}

	// Synthesize a default entry when no list entries are present.
	if len(cfg.Endpoints) == 0 {
		cfg.Endpoints = []endpoint{{Host: defaultTelemetryHost, Port: 9273}}
	}

	return cfg
}

func extractBasicAuthConfig(prom map[string]any) basicAuthConfig {
	cfg := basicAuthConfig{Realm: defaultBasicAuthRealm}
	authMap, ok := prom["basic-auth"].(map[string]any)
	if !ok {
		return cfg
	}
	if enabled, ok := authMap["enabled"].(string); ok && enabled == "true" {
		cfg.Enabled = true
	}
	if realm, ok := authMap["realm"].(string); ok && realm != "" {
		cfg.Realm = realm
	}
	cfg.Username, _ = authMap["username"].(string)
	cfg.BcryptHash, _ = authMap["password"].(string)
	return cfg
}

func extractNetdataConfig(prom map[string]any) (netdataConfig, []string) {
	cfg := defaultNetdataConfig()
	deprecated := deprecatedNetdataAliases(prom)
	netdata, hasNetdata := prom["netdata"].(map[string]any)

	if hasNetdata {
		if enabled, ok := netdata["enabled"].(string); ok && enabled == "false" {
			cfg.Enabled = false
		}
	}

	if hasNetdata {
		if prefix, ok := netdata["prefix"].(string); ok && prefix != "" {
			cfg.Prefix = prefix
		} else if prefix, ok := prom["prefix"].(string); ok && prefix != "" {
			cfg.Prefix = prefix
		}
	} else if prefix, ok := prom["prefix"].(string); ok && prefix != "" {
		cfg.Prefix = prefix
	}

	if hasNetdata {
		cfg.Interval = parseCollectorInterval(netdata["interval"], cfg.Interval)
		if _, ok := netdata["interval"]; !ok {
			cfg.Interval = parseCollectorInterval(prom["interval"], cfg.Interval)
		}
	} else {
		cfg.Interval = parseCollectorInterval(prom["interval"], cfg.Interval)
	}

	if hasNetdata {
		if collMap, ok := netdata["collector"].(map[string]any); ok {
			cfg.Collectors = parseCollectorConfigs(collMap)
		} else if collMap, ok := prom["collector"].(map[string]any); ok {
			cfg.Collectors = parseCollectorConfigs(collMap)
		}
	} else if collMap, ok := prom["collector"].(map[string]any); ok {
		cfg.Collectors = parseCollectorConfigs(collMap)
	}

	return cfg, deprecated
}

func defaultNetdataConfig() netdataConfig {
	return netdataConfig{
		Enabled:    true,
		Prefix:     defaultNetdataPrefix,
		Interval:   defaultNetdataSeconds,
		Collectors: make(map[string]collectorConfig),
	}
}

func deprecatedNetdataAliases(prom map[string]any) []string {
	var deprecated []string
	if _, ok := prom["prefix"]; ok {
		deprecated = append(deprecated, "telemetry.prometheus.prefix")
	}
	if _, ok := prom["interval"]; ok {
		deprecated = append(deprecated, "telemetry.prometheus.interval")
	}
	if _, ok := prom["collector"]; ok {
		deprecated = append(deprecated, "telemetry.prometheus.collector")
	}
	return deprecated
}

func parseCollectorInterval(value any, fallback int) int {
	intervalStr, ok := value.(string)
	if !ok {
		return fallback
	}
	n, err := strconv.Atoi(intervalStr)
	if err != nil || n < 1 || n > 60 {
		return fallback
	}
	return n
}

func parseCollectorConfigs(collMap map[string]any) map[string]collectorConfig {
	collectors := make(map[string]collectorConfig, len(collMap))
	for name, v := range collMap {
		entry, ok := v.(map[string]any)
		if !ok {
			continue
		}
		cc := collectorConfig{Enabled: true}
		if enabled, ok := entry["enabled"].(string); ok && enabled == "false" {
			cc.Enabled = false
		}
		cc.Interval = parseCollectorInterval(entry["interval"], cc.Interval)
		collectors[name] = cc
	}
	return collectors
}
