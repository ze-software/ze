// Design: docs/architecture/api/commands.md -- show policy test dry-run handler
// Related: show_policy.go -- show policy list/chain handlers
// Spec: plan/spec-pol-4-explain.md

package show

import (
	"encoding/hex"
	"strconv"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{
			WireMethod:       "ze-show:policy-test",
			Handler:          handleShowPolicyTest,
			RequiresSelector: true,
		},
	)
}

// parsePolicyTestArgs extracts direction, filter, hex payload, and source-asn4
// from the argument list. The peer selector is stripped by the dispatcher
// before these args arrive, so the canonical forms seen here are:
//
//	export update HEXSTR
//	import filter NAME update HEXSTR
//	export update HEXSTR source-asn4 false
//
// An optional legacy "hex" sub-keyword (update hex HEXSTR) is still tolerated.
func parsePolicyTestArgs(args []string) (direction, filter, hexPayload string, asn4 bool, err error) {
	asn4 = true // default

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
			// Reject unknown tokens rather than skipping them. Silently ignoring
			// a typo (e.g. "source-asn" for "source-asn4") would leave a flag at
			// its default and hand the operator a result that quietly differs
			// from intent.
			return "", "", "", false, policyTestError("unexpected token " + strconv.Quote(args[i]) + " (expected import, export, filter, update, or source-asn4)")
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

var (
	errMissingDirection  = policyTestError("missing direction (import or export)")
	errMissingFilterName = policyTestError("missing filter name after 'filter' keyword")
	errMissingUpdateHex  = policyTestError("missing UPDATE hex bytes after the 'update' keyword")
	errMissingASN4Value  = policyTestError("missing value after 'source-asn4' (true or false)")
	errInvalidASN4Value  = policyTestError("invalid source-asn4 value (expected true or false)")
)

type policyTestError string

func (e policyTestError) Error() string { return string(e) }

const (
	bgpMinMessageLen = 19
	bgpMaxMessageLen = 65535
	policyDirImport  = "import"
	policyDirExport  = "export"
)

func handleShowPolicyTest(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	direction, filter, hexPayload, asn4, err := parsePolicyTestArgs(args)
	if err != nil {
		return &plugin.Response{Status: plugin.StatusError, Error: err.Error()}, nil //nolint:nilerr // operational error in Response
	}

	// Decode hex.
	hexPayload = strings.TrimPrefix(hexPayload, "0x")
	hexPayload = strings.TrimPrefix(hexPayload, "0X")
	// Bound the decoded size before allocating: each byte is two hex chars, so
	// reject anything that could exceed the BGP maximum rather than letting
	// hex.DecodeString allocate an oversized buffer first.
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

	// Validate it looks like a BGP UPDATE (type byte at offset 18 == 2).
	// The 16-byte marker (offsets 0-15) is intentionally not validated: this is
	// a paste-the-hex diagnostic tool and operators routinely capture UPDATEs
	// without reconstructing the all-ones marker. Length and type suffice.
	if raw[18] != 2 {
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  "not a BGP UPDATE message (type byte is not 2)",
		}, nil
	}

	// Extract UPDATE body (after 19-byte header).
	updateBody := raw[bgpMinMessageLen:]

	// Reactor check after input validation so bad input is rejected early.
	if ctx == nil || ctx.Reactor() == nil {
		return &plugin.Response{Status: plugin.StatusError, Error: "reactor not available"}, nil
	}

	// Resolve peer.
	allPeers := ctx.Reactor().Peers()
	selector := ctx.PeerSelector()
	matched := filterPeersByPolicySelector(allPeers, selector)

	if len(matched) == 0 {
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  "peer not found: " + selector,
		}, nil
	}

	if len(matched) > 1 {
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  "selector matches multiple peers, narrow to one: " + selector,
		}, nil
	}

	peer := matched[0]

	// Type-assert to PolicyDryRunner.
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
