// Design: docs/architecture/ddos/cp-survival-5-detect-0-umbrella.md -- Flowtriq reporter config

package flowtriq

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/ze-software/ze/internal/core/opererr"
)

const (
	Name = "ddos-flowtriq"
	// configRoot is the nested YANG config path (ddos/flowtriq); the plugin
	// augments the shared `ddos` container, so the section is wrapped as
	// {"ddos":{"flowtriq":{...}}}.
	configRoot = "ddos/flowtriq"
)

var loggerPtr atomic.Pointer[slog.Logger]

func setLogger(l *slog.Logger) {
	if l != nil {
		loggerPtr.Store(l)
	}
}

func logger() *slog.Logger {
	if l := loggerPtr.Load(); l != nil {
		return l
	}
	return slog.Default()
}

type Config struct {
	Enabled  bool   `json:"enabled"`
	APIKey   string `json:"api-key"` //nolint:gosec // config value, not hardcoded
	NodeUUID string `json:"node-uuid"`
	APIBase  string `json:"api-base"`
}

// defaultAPIBase is the Flowtriq API the reporter posts to when api-base is
// not set. The YANG default of the api-base leaf states the same URL.
const defaultAPIBase = "https://flowtriq.com/api/v1"

// schemeHTTPS is the only URL scheme an api-base may use: the api-key and the
// incident data travel in every request.
const schemeHTTPS = "https"

func DefaultConfig() *Config {
	return &Config{
		APIBase: defaultAPIBase,
	}
}

func ParseConfig(data string) (*Config, error) {
	cfg := DefaultConfig()
	if strings.TrimSpace(data) == "" {
		return cfg, nil
	}
	var root map[string]any
	if err := json.Unmarshal([]byte(data), &root); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	// Section is wrapped by ExtractConfigSubtree as {"ddos":{"flowtriq":{...}}}.
	ddos, ok := root["ddos"].(map[string]any)
	if !ok {
		return cfg, nil
	}
	m, ok := ddos["flowtriq"].(map[string]any)
	if !ok {
		return cfg, nil
	}
	if v, ok := cfgBool(m["enabled"]); ok {
		cfg.Enabled = v
	}
	if v, ok := m["api-key"].(string); ok {
		cfg.APIKey = v
	}
	if v, ok := m["node-uuid"].(string); ok {
		cfg.NodeUUID = v
	}
	if v, ok := m["api-base"].(string); ok {
		cfg.APIBase = v
	}
	return cfg, nil
}

// cfgBool coerces a config value to bool. The config framework delivers YANG leaf
// values as JSON strings ("true"/"false"), so the native JSON bool and the string
// form are both accepted -- without the string case `enabled` silently stayed false
// and the reporter never ran.
func cfgBool(v any) (bool, bool) {
	switch b := v.(type) {
	case bool:
		return b, true
	case string:
		if pb, err := strconv.ParseBool(strings.TrimSpace(b)); err == nil {
			return pb, true
		}
		return false, false
	default:
		return false, false
	}
}

func (c *Config) Validate() error {
	if c.Enabled {
		if c.APIKey == "" {
			return fmt.Errorf("api-key is required when enabled")
		}
		if c.NodeUUID == "" {
			return fmt.Errorf("node-uuid is required when enabled")
		}
		if c.APIBase == "" {
			return fmt.Errorf("api-base must not be empty")
		}
		if err := validateAPIBase(c.APIBase); err != nil {
			return err
		}
	}
	return nil
}

// apiBaseWhy is why an api-base must be an https URL with a host: the reason
// every refusal of validateAPIBase gives.
const apiBaseWhy = "Ze sends the api-key as a bearer token, and the incident data, in every request, " +
	"so it posts only to an https URL; over http anyone on the path reads the token"

// validateAPIBase refuses an api-base that is not an https URL naming a host.
// No http form is allowed, loopback included: nothing in Ze posts to a local
// endpoint, and the unit tests build their client with newClient against an
// httptest server, never through verify. url.Parse lowercases the scheme, so
// HTTPS:// is accepted.
func validateAPIBase(base string) error {
	parsed, err := url.Parse(base)
	if err != nil {
		// The parse error quotes the raw value, which may carry a password, so
		// the refusal does not show it.
		return opererr.New("ddos flowtriq api-base is not a valid URL", apiBaseWhy,
			"set ddos flowtriq api-base "+defaultAPIBase)
	}
	if parsed.Scheme != schemeHTTPS {
		return opererr.New("ddos flowtriq api-base "+parsed.Redacted()+" does not use https", apiBaseWhy,
			"set ddos flowtriq api-base "+httpsAPIBase(parsed))
	}
	if parsed.Host == "" {
		return opererr.New("ddos flowtriq api-base "+parsed.Redacted()+" names no host", apiBaseWhy,
			"set ddos flowtriq api-base "+defaultAPIBase)
	}
	return nil
}

// httpsAPIBase answers the https form of an api-base that names a host, and
// the default api-base for one that names none, such as a value typed without
// a scheme, which url.Parse reads as a path.
func httpsAPIBase(parsed *url.URL) string {
	if parsed.Host == "" {
		return defaultAPIBase
	}
	secure := *parsed
	secure.Scheme = schemeHTTPS
	return secure.Redacted()
}
