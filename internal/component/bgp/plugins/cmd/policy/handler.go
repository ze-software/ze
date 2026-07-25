// Design: docs/architecture/api/commands.md -- policy inspection and dry-run handlers

package policy

import (
	"encoding/hex"
	"net/netip"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/textbuf"
)

type policyFilterRef struct {
	Name      string `json:"name"`
	Canonical string `json:"canonical"`
}

const (
	bgpMinMessageLen = 19
	bgpMaxMessageLen = 65535
	policyDirImport  = "import"
	policyDirExport  = "export"
)

func handleShowPolicyChain(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
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
		Peer   string            `json:"peer"`
		Name   string            `json:"name,omitempty"`
		Import []policyFilterRef `json:"import,omitempty"`
		Export []policyFilterRef `json:"export,omitempty"`
	}

	chains := make([]peerChain, 0, len(matched))
	for i := range matched {
		p := &matched[i]
		entry := peerChain{
			Peer: p.Address.String(),
			Name: p.Name,
		}
		if direction == "" || direction == "import" {
			entry.Import = toFilterRefs(p.ImportFilters)
		}
		if direction == "" || direction == "export" {
			entry.Export = toFilterRefs(p.ExportFilters)
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
	case policyDirImport, policyDirExport, "filter", "update", "source-asn4":
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
)

func parsePolicyTestArgs(args []string) (direction, filter, hexPayload string, asn4 bool, err error) {
	asn4 = true

	i := 0
	for i < len(args) {
		switch strings.ToLower(args[i]) {
		case policyDirImport, policyDirExport:
			direction = strings.ToLower(args[i])
			i++
		case "filter":
			i++
			if i >= len(args) {
				return "", "", "", false, errMissingFilterName
			}
			filter = args[i]
			i++
		case "update":
			i++
			if i >= len(args) {
				return "", "", "", false, errMissingUpdateHex
			}
			if strings.EqualFold(args[i], "hex") {
				i++
				if i >= len(args) {
					return "", "", "", false, errMissingUpdateHex
				}
			}
			hexPayload = args[i]
			i++
		case "source-asn4":
			i++
			if i >= len(args) {
				return "", "", "", false, errMissingASN4Value
			}
			switch strings.ToLower(args[i]) {
			case "true":
				asn4 = true
			case "false":
				asn4 = false
			default:
				return "", "", "", false, errInvalidASN4Value
			}
			i++
		default:
			var tb textbuf.Buffer
			return "", "", "", false, policyTestError(tb.Str("unexpected token ").Str(strconv.Quote(args[i])).Str(" (expected import, export, filter, update, or source-asn4)").String())
		}
	}

	if direction == "" {
		return "", "", "", false, errMissingDirection
	}
	if hexPayload == "" {
		return "", "", "", false, errMissingUpdateHex
	}

	return direction, filter, hexPayload, asn4, nil
}

func handleShowPolicyTest(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	peerSelector := ""
	if ctx != nil {
		peerSelector = ctx.Selector("selector")
	}
	if peerSelector == "" && len(args) > 0 && !isPolicyTestKeyword(args[0]) {
		peerSelector = args[0]
		args = args[1:]
	}

	direction, filter, hexPayload, asn4, err := parsePolicyTestArgs(args)
	if err != nil {
		return &plugin.Response{Status: plugin.StatusError, Error: err.Error()}, nil //nolint:nilerr // operational error in Response
	}

	hexPayload = strings.TrimPrefix(hexPayload, "0x")
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

	result, err := dryRunner.PolicyDryRun(peer.Address.String(), direction, filter, updateBody, asn4)
	if err != nil {
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

	if len(selector) > 2 && (selector[0] == 'a' || selector[0] == 'A') && (selector[1] == 's' || selector[1] == 'S') {
		if asn, err := strconv.ParseUint(selector[2:], 10, 32); err == nil {
			var matched []plugin.PeerInfo
			for i := range peers {
				if uint64(peers[i].PeerAS) == asn {
					matched = append(matched, peers[i])
				}
			}
			return matched
		}
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
