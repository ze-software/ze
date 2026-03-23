// Design: plan/spec-prefix-data.md -- PeeringDB prefix update command
// Overview: peer.go -- BGP peer lifecycle and introspection handlers

package peer

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/peeringdb"
	"codeberg.org/thomas-mangin/ze/internal/component/cli"
	"codeberg.org/thomas-mangin/ze/internal/component/config/system"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

const (
	peeringdbRateLimit = time.Second // 1 request per second

	statusUpdated = "updated"
	statusSkipped = "skipped"
	statusError   = "error"
)

// peerResult holds the outcome of a prefix update attempt for one peer.
type peerResult struct {
	Peer    string         `json:"peer"`
	ASN     uint32         `json:"asn"`
	Status  string         `json:"status"`
	Changes map[string]any `json:"changes,omitempty"`
	Error   string         `json:"error,omitempty"`
}

// HandleBgpPeerPrefixUpdate handles "update bgp peer <selector> prefix".
// Queries PeeringDB for each peer's ASN and proposes new prefix maximums.
// Updates config but does NOT commit -- operator must run "ze config commit".
func HandleBgpPeerPrefixUpdate(ctx *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	_, errResp, err := pluginserver.RequireReactor(ctx)
	if err != nil {
		return errResp, err
	}

	configPath := ctx.Server.ConfigPath()
	if configPath == "" {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   "config path not available",
		}, fmt.Errorf("config path not set")
	}

	peers, errResp, err := filterPeersBySelector(ctx)
	if errResp != nil {
		return errResp, err
	}
	if len(peers) == 0 {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   "no peers found matching selector",
		}, fmt.Errorf("no peers matched")
	}

	// Open config to read PeeringDB settings and update values.
	ed, err := cli.NewEditor(configPath)
	if err != nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   fmt.Sprintf("cannot open config: %v", err),
		}, fmt.Errorf("open config: %w", err)
	}
	defer func() { _ = ed.Close() }()

	// Read PeeringDB settings from system config (single source of truth).
	sc := system.ExtractSystemConfig(ed.Tree())

	if err := validatePeeringDBURL(sc.PeeringDBURL); err != nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   fmt.Sprintf("invalid peeringdb url: %v", err),
		}, err
	}

	client := peeringdb.NewPeeringDB(sc.PeeringDBURL)
	today := time.Now().Format(time.DateOnly)

	var results []peerResult
	var updated int

	for i := range peers {
		p := &peers[i]
		result := peerResult{Peer: p.Address.String(), ASN: p.PeerAS}

		if p.PeerAS == 0 {
			result.Status = statusSkipped
			result.Error = "no remote ASN configured"
			results = append(results, result)
			continue
		}

		// Rate limit: sleep between PeeringDB requests.
		if i > 0 {
			time.Sleep(peeringdbRateLimit)
		}

		// TODO: CommandContext does not carry context.Context -- use background.
		// Rate limiting (sleep above) provides the only cancellation point.
		counts, lookupErr := client.LookupASN(context.TODO(), p.PeerAS)
		if lookupErr != nil {
			result.Status = statusError
			result.Error = lookupErr.Error()
			results = append(results, result)
			continue
		}

		if counts.Suspicious() {
			result.Status = statusSkipped
			result.Error = "PeeringDB returned zero prefixes (suspicious)"
			results = append(results, result)
			continue
		}

		if ok := updatePeerPrefixConfig(ed, p, counts, sc.PeeringDBMargin, today, &result); !ok {
			results = append(results, result)
			continue
		}

		result.Status = statusUpdated
		results = append(results, result)
		updated++
	}

	// Save config (creates backup, writes atomically).
	if updated > 0 {
		if err := ed.Save(); err != nil {
			return &plugin.Response{
				Status: plugin.StatusError,
				Data:   fmt.Sprintf("failed to save config: %v", err),
			}, fmt.Errorf("save config: %w", err)
		}
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"results": results,
			"updated": updated,
			"total":   len(peers),
			"message": fmt.Sprintf("updated %d of %d peer(s) -- run 'ze config commit' to apply", updated, len(peers)),
		},
	}, nil
}

// updatePeerPrefixConfig applies PeeringDB counts to a single peer's config.
// Returns true on success, false on error (result.Status/Error set).
func updatePeerPrefixConfig(ed *cli.Editor, p *plugin.PeerInfo, counts peeringdb.PrefixCounts, margin uint8, today string, result *peerResult) bool {
	changes := make(map[string]any)
	peerKey := p.Name
	if peerKey == "" {
		peerKey = p.Address.String()
	}

	// Build config path. Grouped peers use group path; standalone use peer path.
	// This matches the save.go convention (always writes to standalone peer path).
	basePath := []string{"bgp", "peer", peerKey}

	if counts.IPv4 > 0 {
		newMax := peeringdb.ApplyMargin(counts.IPv4, margin)
		familyPath := append(basePath, "family", "ipv4/unicast", "prefix") //nolint:gocritic // append to copy is intentional
		if setErr := ed.SetValue(familyPath, "maximum", fmt.Sprintf("%d", newMax)); setErr != nil {
			result.Status = statusError
			result.Error = fmt.Sprintf("set ipv4 maximum: %v", setErr)
			return false
		}
		changes["ipv4/unicast"] = newMax
	}

	if counts.IPv6 > 0 {
		newMax := peeringdb.ApplyMargin(counts.IPv6, margin)
		familyPath := append(basePath, "family", "ipv6/unicast", "prefix") //nolint:gocritic // append to copy is intentional
		if setErr := ed.SetValue(familyPath, "maximum", fmt.Sprintf("%d", newMax)); setErr != nil {
			result.Status = statusError
			result.Error = fmt.Sprintf("set ipv6 maximum: %v", setErr)
			return false
		}
		changes["ipv6/unicast"] = newMax
	}

	// Set updated timestamp (hidden leaf).
	prefixPath := append(basePath, "prefix") //nolint:gocritic // append to copy is intentional
	if setErr := ed.SetValue(prefixPath, "updated", today); setErr != nil {
		result.Status = statusError
		result.Error = fmt.Sprintf("set updated timestamp: %v", setErr)
		return false
	}

	result.Changes = changes
	return true
}

// validatePeeringDBURL checks that a PeeringDB URL uses http or https scheme.
func validatePeeringDBURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("peeringdb: invalid url %q: %w", rawURL, err)
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("peeringdb: url scheme must be http or https, got %q", scheme)
	}
	return nil
}
