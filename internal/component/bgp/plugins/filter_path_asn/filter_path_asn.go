// Design: docs/architecture/bgp/filter-path-asn.md -- the reject-asn filter plugin
// Detail: config.go -- the position vocabulary and the config parse
// Detail: match.go -- the AS_PATH scan and the position judgement
// Related: internal/component/bgp/policyview/aspath_text.go -- ASPathText, the subject the scan reads
//
// Package filter_path_asn implements the bgp-filter-path-asn plugin.
//
// The plugin loads named reject-asn definitions from
// policy { reject-asn NAME { indirect [ N N ]; regex [ "P" ]; } }
// at Stage 2 and on every reload. A BGP chain reaches a list only through a
// match instance whose reject-asn condition names it (condition.go), judged in
// process over the route view, so the plugin answers no filter-update. The
// plugin process serves the `show ... reject-asn` commands from the installed
// config.
//
// A list is an unordered reject set with two arms. A route is rejected when a
// position leaf-list holds an ASN the AS_PATH carries at that position, or when
// a pattern of the regex leaf-list matches the whole path. There is no ordering
// between the arms and no first-match-wins inside either, which is what
// separates the type from as-path-list, whose entries are ordered and carry
// their own accept-or-reject action.
//
// RFC 7454 Section 9: "This loose policy could be combined with filters for
// specific 2-byte or 4-byte AS paths that must not be accepted if advertised by
// the customer, such as upstream transit providers or peer ASNs."
//
// policy.BuildRegistry (internal/component/policy) discovers each list
// instance from the ze:filter marker on the YANG list under the top-level
// policy root.
package filter_path_asn

import (
	"errors"
	"net"
	"sync/atomic"

	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/slogutil"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

var errInvalidBGPConfigJSON = errors.New("filter-path-asn: invalid bgp config JSON")

var logger = slogutil.LazyLogger("bgp.filter.path.asn")

const (
	configRootBGP = "bgp"

	// directionImport is the direction label of an import chain, the one
	// direction whose session names the peer that SENT the route
	// (policyview.Session.SenderKnown, condition.go).
	directionImport = "import"
)

// rejectASNConfig is one installed config: the reject-asn lists keyed by the
// list name an operator wrote, and the attachment counts of the same delivery.
// They live in one value so a reader that loaded the lists reads the counts
// of the same config, never the counts of the next one.
type rejectASNConfig struct {
	lists       map[string]*rejectList
	attachments map[string]attachment
}

// installedRejectASN holds the installed config. It is replaced whole by
// installRejectASN (Stage 2 and each reload's apply and rollback) so the hot
// path reads it without a lock.
//
// A nil pointer means no delivery has arrived yet, which a show command
// answers as such: it is a state, not an empty list.
//
// Safe for concurrent use. The value it points at MUST NOT be written after
// the Store, because every show command reads it without a lock.
var installedRejectASN atomic.Pointer[rejectASNConfig]

// runFilterPathASN runs the reject-asn filter plugin over the SDK RPC protocol.
// This is the in-process entry point called through InternalPluginRunner.
func runFilterPathASN(conn net.Conn) int {
	p := sdk.NewWithConn("bgp-filter-path-asn", conn)
	defer p.Close() //nolint:errcheck // best-effort cleanup

	newRejectASNConfigTx().Register(p)

	p.OnExecuteCommand(func(_, command string, args []string, _ string) (string, any, error) {
		return handleCommand(command, args)
	})

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	if err := p.Run(ctx, &sdk.Registration{
		Commands:    commandDecls(),
		WantsConfig: []string{configRootBGP},
	}); err != nil {
		logger().Error("filter-path-asn plugin failed", "error", err)
		return 1
	}
	return 0
}

// newRejectASNConfigTx is the filter's side of the config transaction:
// Stage 2 installs the parsed config, a reload's verify parses the candidate,
// apply installs it and keeps the replaced one, and rollback installs that
// back. A delivery that does not parse is refused, so a config an operator
// believes is filtering never reaches a session half-applied.
func newRejectASNConfigTx() *sdk.ConfigTx[*rejectASNConfig] {
	parse := policy.SectionsParser("filter-path-asn", errInvalidBGPConfigJSON, configRootBGP, parseRejectASNConfig)
	return sdk.NewConfigTx[*rejectASNConfig](nil, parse, installRejectASN)
}

// parseRejectASNConfig reads the lists from the policy root and, only when
// they parse, the attachment counts from the bgp root, so
// `show bgp reject-asn` can never report a peer count against a set of lists
// the plugin refused.
func parseRejectASNConfig(policyCfg, bgpCfg map[string]any) (*rejectASNConfig, error) {
	lists, err := parseRejectASNLists(policyCfg)
	if err != nil {
		return nil, err
	}
	return &rejectASNConfig{lists: lists, attachments: countAttachments(policyCfg, bgpCfg)}, nil
}

// installRejectASN makes cfg what every later show reads.
// A rollback of the first apply installs nil, the "no delivery" state. It
// cannot fail.
func installRejectASN(cfg *rejectASNConfig) error {
	installedRejectASN.Store(cfg)
	if cfg != nil {
		logger().Debug("configured", "reject-asn-lists", len(cfg.lists))
	}
	return nil
}

// commandDecls names the commands this plugin serves and states what each
// answer holds (pkg/plugin/rpc/types.go, CommandDecl).
//
// It has two readers and MUST stay one function. init() puts it on the
// registry.Registration, which anything linking the composition root reads
// without starting an engine, and the runner sends it in the Stage 1
// registration message, which a running daemon reads.
func commandDecls() []sdk.CommandDecl {
	return []sdk.CommandDecl{
		{Name: cmdShowRejectASN, ShortHelp: "Show every reject-asn list with its ASNs, positions and attached peers"},
		{Name: cmdShowRejectASNName, ShortHelp: "Show one reject-asn list by name", Args: []string{"<name>"}},
		// The description says "well-known" rather than naming the table
		// this command reads: TestCuratedTableDecidesNothing asserts that
		// no file deciding anything mentions it, and this file decides.
		{Name: cmdShowRejectASNTransitFree, ShortHelp: "Print the well-known transit-free ASNs as a config block"},
	}
}
