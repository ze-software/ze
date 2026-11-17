// Design: docs/architecture/api/commands.md -- policy inspection and dry-run handlers

package policy

import (
	"encoding/hex"
	"errors"
	"net/netip"
	"strconv"
	"strings"

	selectorpkg "github.com/ze-software/ze/internal/core/selector"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

type policyFilterRef struct {
	Name      string `json:"name"`
	Canonical string `json:"canonical"`
}

// policyChain is one direction of `show policy chain`. Final and FinalSource
// carry plugin.PolicyInfo's effective final unchanged, so the chain view and peer
// detail never disagree on a default reject (RFC 8212) or an undetermined final.
type policyChain struct {
	Filters     []policyFilterRef `json:"filters"`
	Final       string            `json:"final,omitempty"`
	FinalSource string            `json:"final-source,omitempty"`
}

func toPolicyChain(policy *plugin.PolicyInfo) *policyChain {
	if policy == nil {
		return nil
	}
	return &policyChain{Filters: toFilterRefs(policy.Filters), Final: policy.Final, FinalSource: policy.FinalSource}
}

const (
	bgpMinMessageLen = 19
	bgpMaxMessageLen = 65535
	policyDirImport  = "import"
	policyDirExport  = "export"
)

func handleShowPolicyChain(ctx *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	args := validated.Tokens()
	if ctx.Reactor() == nil {
		return &plugin.Response{Status: plugin.StatusError, Error: "reactor not available"}, nil
	}

	selector := ""
	if ctx != nil {
		selector = ctx.Selector("selector")
	}
	if selector == "" && len(args) > 0 && args[0] != policyDirImport && args[0] != policyDirExport {
		selector = args[0]
		args = args[1:]
	}
	if selector == "" {
		selector = ctx.PeerSelector()
	}

	allPeers := ctx.Reactor().Peers()
	matched := filterPeersByPolicySelector(allPeers, selector)

	var tb textbuf.Buffer
	if len(matched) == 0 {
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  tb.Str("peer not found: ").Str(selector).String(),
		}, nil
	}

	direction := ""
	if len(args) > 0 {
		direction = args[0]
		if direction != policyDirImport && direction != policyDirExport {
			return &plugin.Response{
				Status: plugin.StatusError,
				Error:  tb.Reset().Str("invalid direction ").Str(strconv.Quote(direction)).Str(" (expected import or export)").String(),
			}, nil
		}
	}

	type peerChain struct {
		Peer   string       `json:"peer"`
		Name   string       `json:"name,omitempty"`
		Import *policyChain `json:"import,omitempty"`
		Export *policyChain `json:"export,omitempty"`
	}

	chains := make([]peerChain, 0, len(matched))
	for i := range matched {
		p := &matched[i]
		entry := peerChain{
			Peer: p.Address.String(),
			Name: p.Name,
		}
		if direction == "" || direction == "import" {
			entry.Import = toPolicyChain(p.ImportPolicy)
		}
		if direction == "" || direction == "export" {
			entry.Export = toPolicyChain(p.ExportPolicy)
		}
		chains = append(chains, entry)
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
			"chains": chains,
		},
	}, nil
}

func isPolicyTestKeyword(tok string) bool {
	switch strings.ToLower(tok) {
	case policyDirImport, policyDirExport, "filter", "update", "source-asn4", "route-kind":
		return true
	default:
		return false
	}
}

type policyTestError string

func (e policyTestError) Error() string { return string(e) }

var (
	errMissingDirection  = policyTestError("missing direction (import or export)")
	errMissingFilterName = policyTestError("missing filter name after 'filter' keyword")
	errMissingUpdateHex  = policyTestError("missing UPDATE hex bytes after the 'update' keyword")
	errMissingASN4Value  = policyTestError("missing value after 'source-asn4' (true or false)")
	errInvalidASN4Value  = policyTestError("invalid source-asn4 value (expected true or false)")
	errMissingRouteKind  = policyTestError("missing value after 'route-kind' (learned or local)")
	errInvalidRouteKind  = policyTestError("invalid route-kind value (expected learned or local)")
	errImportRouteKind   = policyTestError("route-kind is only available for export; import has one policy answer")
	errRepeatedRouteKind = policyTestError("route-kind is accepted only once (learned or local)")
)

type policyTestArgs struct {
	direction     string
	filter        string
	hexPayload    string
	simulatedKind string
	asn4          bool
}

func parsePolicyTestArgs(args []string) (policyTestArgs, error) {
	parsed := policyTestArgs{asn4: true}

	i := 0
	for i < len(args) {
		switch strings.ToLower(args[i]) {
		case policyDirImport, policyDirExport:
			parsed.direction = strings.ToLower(args[i])
			i++
		case "filter":
			i++
			if i >= len(args) {
				return policyTestArgs{}, errMissingFilterName
			}
			parsed.filter = args[i]
			i++
		case "update":
			i++
			if i >= len(args) {
				return policyTestArgs{}, errMissingUpdateHex
			}
			if strings.EqualFold(args[i], "hex") {
				i++
				if i >= len(args) {
					return policyTestArgs{}, errMissingUpdateHex
				}
			}
			parsed.hexPayload = args[i]
			i++
		case "source-asn4":
			i++
			if i >= len(args) {
				return policyTestArgs{}, errMissingASN4Value
			}
			switch strings.ToLower(args[i]) {
			case "true":
				parsed.asn4 = true
			case "false":
				parsed.asn4 = false
			default:
				return policyTestArgs{}, errInvalidASN4Value
			}
			i++
		case "route-kind":
			if parsed.simulatedKind != "" {
				return policyTestArgs{}, errRepeatedRouteKind
			}
			i++
			if i >= len(args) {
				return policyTestArgs{}, errMissingRouteKind
			}
			switch args[i] {
			case "learned", "local":
				parsed.simulatedKind = args[i]
			default:
				return policyTestArgs{}, errInvalidRouteKind
			}
			i++
		default:
			var tb textbuf.Buffer
			return policyTestArgs{}, policyTestError(tb.Str("unexpected token ").Str(strconv.Quote(args[i])).Str(" (expected import, export, filter, update, source-asn4, or route-kind)").String())
		}
	}

	if parsed.direction == "" {
		return policyTestArgs{}, errMissingDirection
	}
	if parsed.direction == policyDirImport && parsed.simulatedKind != "" {
		return policyTestArgs{}, errImportRouteKind
	}
	if parsed.hexPayload == "" {
		return policyTestArgs{}, errMissingUpdateHex
	}

	return parsed, nil
}

func handleShowPolicyTest(ctx *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	args := validated.Tokens()
	peerSelector := ""
	if ctx != nil {
		peerSelector = ctx.Selector("selector")
	}
	if peerSelector == "" && len(args) > 0 && !isPolicyTestKeyword(args[0]) {
		peerSelector = args[0]
		args = args[1:]
	}

	parsed, err := parsePolicyTestArgs(args)
	if err != nil {
		return plugin.OperatorErrorResponse(opererr.Wrap(err,
			"policy test arguments are invalid", "The UPDATE was not tested.",
			"ze help show policy test peer")), nil
	}

	hexPayload := strings.TrimPrefix(parsed.hexPayload, "0x")
	hexPayload = strings.TrimPrefix(hexPayload, "0X")
	if len(hexPayload) > bgpMaxMessageLen*2 {
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  "BGP message too long (maximum 65535 bytes)",
		}, nil
	}
	raw, err := hex.DecodeString(hexPayload)
	if err != nil {
		return &plugin.Response{Status: plugin.StatusError, Error: "invalid hex: " + err.Error()}, nil //nolint:nilerr // operational error in Response
	}

	if len(raw) < bgpMinMessageLen {
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  "BGP message too short (minimum 19 bytes)",
		}, nil
	}

	if len(raw) > bgpMaxMessageLen {
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  "BGP message too long (maximum 65535 bytes)",
		}, nil
	}

	if raw[18] != 2 {
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  "not a BGP UPDATE message (type byte is not 2)",
		}, nil
	}

	updateBody := raw[bgpMinMessageLen:]

	if ctx == nil || ctx.Reactor() == nil {
		return &plugin.Response{Status: plugin.StatusError, Error: "reactor not available"}, nil
	}

	if peerSelector == "" {
		peerSelector = ctx.PeerSelector()
	}
	allPeers := ctx.Reactor().Peers()
	matched := filterPeersByPolicySelector(allPeers, peerSelector)

	var tb2 textbuf.Buffer
	if len(matched) == 0 {
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  tb2.Str("peer not found: ").Str(peerSelector).String(),
		}, nil
	}

	if len(matched) > 1 {
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  tb2.Reset().Str("selector matches multiple peers, narrow to one: ").Str(peerSelector).String(),
		}, nil
	}

	peer := matched[0]

	dryRunner, ok := ctx.Reactor().(plugin.PolicyDryRunner)
	if !ok {
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  "reactor does not support policy dry-run",
		}, nil
	}

	result, err := dryRunner.PolicyDryRun(peer.PeerOwner, parsed.direction, parsed.filter, updateBody, parsed.asn4, parsed.simulatedKind)
	if err != nil {
		if refusal, ok := errors.AsType[*opererr.Error](err); ok {
			return plugin.OperatorErrorResponse(refusal), nil
		}
		//nolint:nilerr // operational error in Response
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  err.Error(),
		}, nil
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   result,
	}, nil
}

func filterPeersByPolicySelector(peers []plugin.PeerInfo, selector string) []plugin.PeerInfo {
	if selector == "*" {
		return peers
	}

	if filterIP, err := netip.ParseAddr(selector); err == nil {
		for i := range peers {
			if peers[i].Address == filterIP {
				return []plugin.PeerInfo{peers[i]}
			}
		}
		return nil
	}

	for i := range peers {
		if peers[i].Name == selector {
			return []plugin.PeerInfo{peers[i]}
		}
	}

	// selectorpkg.ParseASNSelector is the one declaration of the `as<N>` form,
	// shared with the selector every other peer command parses. It reads all
	// three RFC 5396 spellings, so `AS1.10` selects the peers `AS65546` does.
	if number, ok := selectorpkg.ParseASNSelector(selector); ok {
		var matched []plugin.PeerInfo
		for i := range peers {
			// A peer whose AS is not yet negotiated matches no AS selector.
			if peerAS, known := peers[i].PeerAS.ASN(); known && peerAS == number {
				matched = append(matched, peers[i])
			}
		}
		return matched
	}

	return nil
}

func toFilterRefs(canonicals []string) []policyFilterRef {
	if len(canonicals) == 0 {
		return nil
	}
	refs := make([]policyFilterRef, len(canonicals))
	for i, c := range canonicals {
		inactive := strings.TrimPrefix(c, "inactive:")
		name := inactive
		if _, after, ok := strings.Cut(inactive, ":"); ok {
			name = after
		}
		if c != inactive {
			var tb3 textbuf.Buffer
			name = tb3.Str("inactive:").Str(name).String()
		}
		refs[i] = policyFilterRef{Name: name, Canonical: c}
	}
	return refs
}
