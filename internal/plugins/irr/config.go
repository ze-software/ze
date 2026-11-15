// Design: docs/architecture/irr-owner.md -- system irr and system peeringdb url, as the owner reads them
// Related: irr.go -- runIRR binds the config transaction built here
// Related: owner.go -- the worker that installs a parsed config
//
// The owner reads one root, system, and two containers in it: irr (server,
// source-address, refresh-interval) and peeringdb (url, the endpoint asked for
// the AS-SET of a bare AS number). Every value arrives as a JSON string.

package irr

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/ze-software/ze/internal/component/resolve/irr/store"
	"github.com/ze-software/ze/internal/core/opererr"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// The YANG defaults (ze-irr-conf.yang, ze-system-conf.yang). A delivery that
// omits a leaf means the default, so the parser applies the same values.
const (
	defaultServer          = "whois.radb.net"
	defaultPeeringDBURL    = "https://www.peeringdb.com"
	defaultRefreshInterval = 3600 * time.Second
	// refreshIntervalMax and refreshIntervalMin bound the leaf's non-zero range
	// (range "0 | 60..86400"); the parser refuses what YANG would.
	refreshIntervalMin = 60
	refreshIntervalMax = 86400
)

// ownerConfig is the owner's whole config. It is a comparable value, so the
// config transaction installs nothing when a commit leaves it unchanged (AC-4).
type ownerConfig struct {
	// server is the whois server as configured; store.DatasetServer gives the
	// key form a dataset is stored under.
	server string
	// sourceAddress is the address bound on the whois connection, empty for
	// the kernel's choice.
	sourceAddress string
	// refreshInterval is how long after the server last answered a dataset is
	// fetched again. Zero is manual only, never "due at once".
	refreshInterval time.Duration
	// peeringDBURL is system peeringdb url, asked for a bare AS number's set.
	peeringDBURL string
}

// manual reports whether refresh-interval 0 turned the automatic refresh off.
func (c ownerConfig) manual() bool {
	return c.refreshInterval == 0
}

// defaultOwnerConfig is the config of a box whose system root names no irr or
// peeringdb leaf: the owner still runs, because a consumer depends on it.
func defaultOwnerConfig() ownerConfig {
	return ownerConfig{server: defaultServer, refreshInterval: defaultRefreshInterval, peeringDBURL: defaultPeeringDBURL}
}

// parseOwnerConfig is the config transaction's parser. present is false when
// no system section is delivered, which leaves the defaults running.
func parseOwnerConfig(sections []sdk.ConfigSection) (ownerConfig, bool, error) {
	cfg := defaultOwnerConfig()
	present := false
	for _, section := range sections {
		if section.Root != configRootSystem {
			continue
		}
		present = true
		var root map[string]any
		if err := json.Unmarshal([]byte(section.Data), &root); err != nil {
			return ownerConfig{}, false, opererr.Wrap(err, "the irr plugin could not read the system configuration",
				"The daemon delivered a system section that is not JSON, so the IRR server and refresh period are unknown.",
				"ze config validate")
		}
		system, _ := root[configRootSystem].(map[string]any)
		if err := applyIRRBlock(&cfg, system); err != nil {
			return ownerConfig{}, false, err
		}
		if peeringdb, ok := system["peeringdb"].(map[string]any); ok {
			if url, ok := peeringdb["url"].(string); ok && url != "" {
				cfg.peeringDBURL = url
			}
		}
	}
	return cfg, present, nil
}

// applyIRRBlock reads system irr into cfg. A server no dataset key can name,
// and an interval outside the leaf's range, are refused at verify: the
// commit fails rather than the owner fetching from a server it cannot store.
func applyIRRBlock(cfg *ownerConfig, system map[string]any) error {
	block, ok := system["irr"].(map[string]any)
	if !ok {
		return nil
	}
	if server, ok := block["server"].(string); ok && server != "" {
		if _, err := store.DatasetServer(server); err != nil {
			return opererr.Wrap(err, "system irr server "+server+" cannot name a stored dataset",
				"Each IRR dataset is stored under its server, so a server Ze cannot key is never fetched.",
				"set system irr server whois.radb.net")
		}
		cfg.server = server
	}
	if address, ok := block["source-address"].(string); ok {
		cfg.sourceAddress = address
	}
	if raw, ok := block["refresh-interval"]; ok {
		seconds, err := readSeconds(raw)
		if err != nil {
			return opererr.Wrap(err, "system irr refresh-interval is not a number of seconds",
				"The refresh period decides when every stored IRR dataset is fetched again.",
				"set system irr refresh-interval 3600")
		}
		cfg.refreshInterval = time.Duration(seconds) * time.Second
	}
	return nil
}

// readSeconds reads the refresh-interval leaf, as a string or a JSON number,
// inside its YANG range.
func readSeconds(raw any) (uint64, error) {
	var seconds uint64
	switch value := raw.(type) {
	case string:
		parsed, err := strconv.ParseUint(value, 10, 32)
		if err != nil {
			return 0, err
		}
		seconds = parsed
	case float64:
		if value < 0 {
			return 0, fmt.Errorf("%v is negative", value)
		}
		seconds = uint64(value)
	default:
		return 0, fmt.Errorf("%v is neither a string nor a number", raw)
	}
	if seconds == 0 {
		return 0, nil
	}
	if seconds < refreshIntervalMin {
		return 0, fmt.Errorf("%d is below %d", seconds, refreshIntervalMin)
	}
	if seconds > refreshIntervalMax {
		return 0, fmt.Errorf("%d is over %d", seconds, refreshIntervalMax)
	}
	return seconds, nil
}
