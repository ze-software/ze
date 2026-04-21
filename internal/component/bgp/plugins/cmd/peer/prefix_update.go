// Design: plan/spec-prefix-data.md -- PeeringDB prefix update command
// Overview: peer.go -- BGP peer lifecycle and introspection handlers

package peer

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/cli"
	"codeberg.org/thomas-mangin/ze/internal/component/config/system"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
	"codeberg.org/thomas-mangin/ze/internal/component/resolve/peeringdb"
)

const (
	peeringdbRateLimit = time.Second // 1 request per second

	statusUpdated = "updated"
	statusSkipped = "skipped"
	statusError   = "error"
)

type prefixLookupClient interface {
	LookupASN(ctx context.Context, asn uint32) (peeringdb.PrefixCounts, error)
}

var newPrefixLookupClient = func(baseURL string) prefixLookupClient {
	return peeringdb.NewPeeringDB(baseURL)
}

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

	requestCtx := ctx.Context()
	client := newPrefixLookupClient(sc.PeeringDBURL)
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

		counts, lookupErr := lookupPrefixCounts(requestCtx, client, p.PeerAS, i > 0)
		if errors.Is(lookupErr, context.Canceled) || errors.Is(lookupErr, context.DeadlineExceeded) {
			return &plugin.Response{
				Status: plugin.StatusError,
				Data:   lookupErr.Error(),
			}, lookupErr
		}
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

func lookupPrefixCounts(ctx context.Context, client prefixLookupClient, asn uint32, waitBefore bool) (peeringdb.PrefixCounts, error) {
	if waitBefore {
		if err := waitForPeeringDBRateLimit(ctx, peeringdbRateLimit); err != nil {
			return peeringdb.PrefixCounts{}, err
		}
	}
	return client.LookupASN(ctx, asn)
}

func waitForPeeringDBRateLimit(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
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
	// After YANG reorg, family/prefix are inside the session container.
	basePath := []string{"bgp", "peer", peerKey}

	if counts.IPv4 > 0 {
		newMax := peeringdb.ApplyMargin(counts.IPv4, margin)
		familyPath := append(basePath, "session", "family", "ipv4/unicast", "prefix") //nolint:gocritic // append to copy is intentional
		if setErr := ed.SetValue(familyPath, "maximum", fmt.Sprintf("%d", newMax)); setErr != nil {
			result.Status = statusError
			result.Error = fmt.Sprintf("set ipv4 maximum: %v", setErr)
			return false
		}
		// Set updated timestamp per-family (YANG: prefix.updated is inside family).
		if setErr := ed.SetValue(familyPath, "updated", today); setErr != nil {
			result.Status = statusError
			result.Error = fmt.Sprintf("set ipv4 updated timestamp: %v", setErr)
			return false
		}
		changes["ipv4/unicast"] = newMax
	}

	if counts.IPv6 > 0 {
		newMax := peeringdb.ApplyMargin(counts.IPv6, margin)
		familyPath := append(basePath, "session", "family", "ipv6/unicast", "prefix") //nolint:gocritic // append to copy is intentional
		if setErr := ed.SetValue(familyPath, "maximum", fmt.Sprintf("%d", newMax)); setErr != nil {
			result.Status = statusError
			result.Error = fmt.Sprintf("set ipv6 maximum: %v", setErr)
			return false
		}
		if setErr := ed.SetValue(familyPath, "updated", today); setErr != nil {
			result.Status = statusError
			result.Error = fmt.Sprintf("set ipv6 updated timestamp: %v", setErr)
			return false
		}
		changes["ipv6/unicast"] = newMax
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
