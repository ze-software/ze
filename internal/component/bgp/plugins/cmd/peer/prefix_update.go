// Design: plan/learned/904-update-bgp-prefix.md -- PeeringDB prefix update command
// Overview: peer.go -- BGP peer lifecycle and introspection handlers

package peer

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/config/system"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/component/resolve/peeringdb"
	"github.com/ze-software/ze/internal/core/textbuf"
)

var (
	errConfigPathNotSet = errors.New("config path not set")
	errNoPeersMatched   = errors.New("no peers matched")
)

const (
	peeringdbRateLimit = time.Second

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

type peerResult struct {
	Peer    string         `json:"peer"`
	ASN     uint32         `json:"asn"`
	Status  string         `json:"status"`
	Changes map[string]any `json:"changes,omitempty"`
	Error   string         `json:"error,omitempty"`
}

func handleBgpPeerPrefixUpdate(ctx *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	_, errResp, err := pluginserver.RequireReactor(ctx)
	if err != nil {
		return errResp, err
	}

	configPath := ctx.Server.ConfigPath()
	if configPath == "" {
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  "config path not available",
		}, errConfigPathNotSet
	}

	peers, errResp, err := filterPeersBySelectorValue(ctx, ctx.PeerSelector())
	if errResp != nil {
		return errResp, err
	}
	if len(peers) == 0 {
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  "no peers found matching selector",
		}, errNoPeersMatched
	}

	ed, err := cli.NewEditor(configPath)
	if err != nil {
		var tb textbuf.Buffer
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  tb.Str("cannot open config: ").Err(err).String(),
		}, fmt.Errorf("open config: %w", err)
	}
	defer func() { _ = ed.Close() }()

	session := cli.NewEditSession("peeringdb", "api")
	ed.SetSession(session)

	sc := system.ExtractSystemConfig(ed.Tree())

	if err := validatePeeringDBURL(sc.PeeringDBURL); err != nil {
		var tb textbuf.Buffer
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  tb.Str("invalid peeringdb url: ").Err(err).String(),
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
				Error:  lookupErr.Error(),
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

	if updated > 0 {
		if err := ed.SaveDraft(); err != nil {
			var tb textbuf.Buffer
			return &plugin.Response{
				Status: plugin.StatusError,
				Error:  tb.Str("failed to save draft: ").Err(err).String(),
			}, fmt.Errorf("save draft: %w", err)
		}
	}

	var b textbuf.Buffer
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
			"results": results,
			"updated": updated,
			"total":   len(peers),
			"message": b.Str("updated ").Int(int64(updated)).Str(" of ").Int(int64(len(peers))).Str(" peer(s) -- run 'config commit' to apply").String(),
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

func updatePeerPrefixConfig(ed *cli.Editor, p *plugin.PeerInfo, counts peeringdb.PrefixCounts, margin uint8, today string, result *peerResult) bool {
	changes := make(map[string]any)
	peerKey := p.Name
	if peerKey == "" {
		peerKey = p.Address.String()
	}

	basePath := []string{"bgp", "peer", peerKey}

	if counts.IPv4 > 0 {
		newMax := peeringdb.ApplyMargin(counts.IPv4, margin)
		familyPath := append(basePath, "session", "family", "ipv4/unicast", "prefix") //nolint:gocritic // append to copy is intentional
		if setErr := ed.SetValue(familyPath, "maximum", textbuf.StringUint32(newMax)); setErr != nil {
			var tb textbuf.Buffer
			result.Status = statusError
			result.Error = tb.Str("set ipv4 maximum: ").Err(setErr).String()
			return false
		}
		if setErr := ed.SetValue(familyPath, "updated", today); setErr != nil {
			var tb textbuf.Buffer
			result.Status = statusError
			result.Error = tb.Str("set ipv4 updated timestamp: ").Err(setErr).String()
			return false
		}
		changes["ipv4/unicast"] = newMax
	}

	if counts.IPv6 > 0 {
		newMax := peeringdb.ApplyMargin(counts.IPv6, margin)
		familyPath := append(basePath, "session", "family", "ipv6/unicast", "prefix") //nolint:gocritic // append to copy is intentional
		if setErr := ed.SetValue(familyPath, "maximum", textbuf.StringUint32(newMax)); setErr != nil {
			var tb textbuf.Buffer
			result.Status = statusError
			result.Error = tb.Str("set ipv6 maximum: ").Err(setErr).String()
			return false
		}
		if setErr := ed.SetValue(familyPath, "updated", today); setErr != nil {
			var tb textbuf.Buffer
			result.Status = statusError
			result.Error = tb.Str("set ipv6 updated timestamp: ").Err(setErr).String()
			return false
		}
		changes["ipv6/unicast"] = newMax
	}

	result.Changes = changes
	return true
}

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
