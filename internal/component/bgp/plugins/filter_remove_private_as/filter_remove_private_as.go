// Design: docs/architecture/core-design.md -- remove-private-as policy action filter
// Detail: config.go -- bgp/policy/remove-private-as config parsing
// Detail: private_as.go -- Private Use ASN detection and text rewrite

package filter_remove_private_as

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"sync/atomic"

	"github.com/ze-software/ze/internal/component/bgp/configjson"
	"github.com/ze-software/ze/internal/core/slogutil"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

var errRemovePrivateASInvalidBgpConfigJSON = errors.New("filter-remove-private-as: invalid bgp config JSON")

var logger = slogutil.LazyLogger("bgp.filter.remove-private-as")

var defsByName atomic.Pointer[map[string]*removePrivateASDef]

func runFilterRemovePrivateAS(conn net.Conn) int {
	p := sdk.NewWithConn("bgp-filter-remove-private-as", conn)
	defer p.Close() //nolint:errcheck // best-effort cleanup

	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		for _, section := range sections {
			if section.Root != "bgp" {
				continue
			}
			bgpCfg, ok := configjson.ParseBGPSubtree(section.Data)
			if !ok {
				return errRemovePrivateASInvalidBgpConfigJSON
			}
			defs, err := parseRemovePrivateASDefs(bgpCfg)
			if err != nil {
				return fmt.Errorf("filter-remove-private-as: %w", err)
			}
			defsByName.Store(&defs)
			logger().Debug("configured", "remove-private-as", len(defs))
		}
		return nil
	})

	p.OnFilterUpdate(func(in *sdk.FilterUpdateInput) (*sdk.FilterUpdateOutput, error) {
		return handleFilterUpdate(in), nil
	})

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	if err := p.Run(ctx, sdk.Registration{
		WantsConfig: []string{"bgp"},
		Filters: []sdk.FilterDecl{{
			Name:       "*",
			Direction:  sdk.FilterBoth,
			Attributes: []string{"as-path", removePrivateASDirective},
			Raw:        true,
			OnError:    sdk.OnErrorReject,
		}},
	}); err != nil {
		logger().Error("filter-remove-private-as plugin failed", "error", err)
		return 1
	}
	return 0
}

func handleFilterUpdate(in *sdk.FilterUpdateInput) *sdk.FilterUpdateOutput {
	defsP := defsByName.Load()
	if defsP == nil {
		logger().Warn("filter-update before configure", "filter", in.Filter, "peer", in.Peer)
		return &sdk.FilterUpdateOutput{Action: sdk.FilterReject}
	}
	defs := *defsP
	def, ok := defs[in.Filter]
	if !ok {
		logger().Warn("unknown remove-private-as", "filter", in.Filter, "peer", in.Peer)
		return &sdk.FilterUpdateOutput{Action: sdk.FilterReject}
	}

	asPath := extractASPathField(in.Update)
	rewritten, asPathChanged := rewriteASPathText(asPath, def.mode, in.PeerAS)
	as4Changed := hasPrivateAS4PathPayload(in.Raw)
	if !asPathChanged && !as4Changed {
		return &sdk.FilterUpdateOutput{Action: sdk.FilterAccept}
	}

	delta := buildDirectiveDelta(def.mode, rewritten, asPathChanged)
	logger().Info("remove-private-as modify", "filter", in.Filter, "peer", in.Peer, "mode", def.mode.String())
	return &sdk.FilterUpdateOutput{Action: sdk.FilterModify, Update: delta}
}

func extractASPathField(updateText string) string {
	_, rest, ok := strings.Cut(updateText, "as-path ")
	if !ok {
		return ""
	}
	return extractASPathValue(rest)
}

func extractASPathValue(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if s[0] == '[' {
		for i := 1; i < len(s); i++ {
			if s[i] == ']' {
				return s[:i+1]
			}
		}
		return s
	}
	before, _, found := strings.Cut(s, " ")
	if !found {
		return s
	}
	return before
}
