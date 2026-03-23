// Design: plan/spec-prefix-data.md -- PeeringDB prefix update command
// Overview: peer.go -- BGP peer lifecycle and introspection handlers

package peer

import (
	"context"
	"fmt"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/peeringdb"
	"codeberg.org/thomas-mangin/ze/internal/component/cli"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

const (
	defaultPeeringDBURL    = "https://www.peeringdb.com"
	defaultPeeringDBMargin = 10
	peeringdbRateLimit     = time.Second // 1 request per second

	statusUpdated = "updated"
	statusSkipped = "skipped"
	statusError   = "error"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{
			WireMethod:       "ze-bgp:peer-prefix-update",
			Handler:          handleBgpPeerPrefixUpdate,
			Help:             "Update prefix maximums from PeeringDB for matched peer(s)",
			RequiresSelector: true,
		},
	)
}

// handleBgpPeerPrefixUpdate handles "bgp peer <selector> prefix update".
// Queries PeeringDB for each peer's ASN and proposes new prefix maximums.
// Updates config but does NOT commit -- operator must run "ze config commit".
func handleBgpPeerPrefixUpdate(ctx *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
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

	// Get peers matching selector.
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

	// Read PeeringDB settings from system config.
	pdbURL, margin := readPeeringDBSettings(ed)

	client := peeringdb.NewClient(pdbURL)
	today := time.Now().Format(time.DateOnly)

	type peerResult struct {
		Peer    string         `json:"peer"`
		ASN     uint32         `json:"asn"`
		Status  string         `json:"status"`
		Changes map[string]any `json:"changes,omitempty"`
		Error   string         `json:"error,omitempty"`
	}

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

		// Rate limit: sleep between requests.
		if i > 0 {
			time.Sleep(peeringdbRateLimit)
		}

		counts, err := client.LookupASN(context.Background(), p.PeerAS)
		if err != nil {
			result.Status = statusError
			result.Error = err.Error()
			results = append(results, result)
			continue
		}

		if counts.Suspicious() {
			result.Status = statusSkipped
			result.Error = "PeeringDB returned zero prefixes (suspicious)"
			results = append(results, result)
			continue
		}

		// Apply margin and update config for ipv4/unicast and ipv6/unicast.
		changes := make(map[string]any)
		peerKey := p.Name
		if peerKey == "" {
			peerKey = p.Address.String()
		}

		if counts.IPv4 > 0 {
			newMax := peeringdb.ApplyMargin(counts.IPv4, margin)
			familyPath := []string{"bgp", "peer", peerKey, "family", "ipv4/unicast", "prefix"}
			if setErr := ed.SetValue(familyPath, "maximum", fmt.Sprintf("%d", newMax)); setErr != nil {
				result.Status = statusError
				result.Error = fmt.Sprintf("set ipv4 maximum: %v", setErr)
				results = append(results, result)
				continue
			}
			changes["ipv4/unicast"] = newMax
		}

		if counts.IPv6 > 0 {
			newMax := peeringdb.ApplyMargin(counts.IPv6, margin)
			familyPath := []string{"bgp", "peer", peerKey, "family", "ipv6/unicast", "prefix"}
			if setErr := ed.SetValue(familyPath, "maximum", fmt.Sprintf("%d", newMax)); setErr != nil {
				result.Status = statusError
				result.Error = fmt.Sprintf("set ipv6 maximum: %v", setErr)
				results = append(results, result)
				continue
			}
			changes["ipv6/unicast"] = newMax
		}

		// Set updated timestamp (hidden leaf).
		prefixPath := []string{"bgp", "peer", peerKey, "prefix"}
		if setErr := ed.SetValue(prefixPath, "updated", today); setErr != nil {
			result.Status = statusError
			result.Error = fmt.Sprintf("set updated timestamp: %v", setErr)
			results = append(results, result)
			continue
		}

		result.Status = statusUpdated
		result.Changes = changes
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

// readPeeringDBSettings reads PeeringDB URL and margin from the config tree.
// Returns defaults if the peeringdb block is not present.
func readPeeringDBSettings(ed *cli.Editor) (string, uint8) {
	pdbURL := defaultPeeringDBURL
	margin := uint8(defaultPeeringDBMargin)

	tree := ed.Tree()
	if tree == nil {
		return pdbURL, margin
	}

	sys := tree.GetContainer("system")
	if sys == nil {
		return pdbURL, margin
	}

	pdb := sys.GetContainer("peeringdb")
	if pdb == nil {
		return pdbURL, margin
	}

	if url, ok := pdb.Get("url"); ok && url != "" {
		pdbURL = url
	}

	if marginStr, ok := pdb.Get("margin"); ok {
		var v int
		if _, scanErr := fmt.Sscanf(marginStr, "%d", &v); scanErr == nil && v >= 0 && v <= 100 {
			margin = uint8(v) //nolint:gosec // Bounded by range check above
		}
	}

	return pdbURL, margin
}
