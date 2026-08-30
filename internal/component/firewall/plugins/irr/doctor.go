// Design: docs/architecture/firewall/firewall-irr.md -- doctor check for IRR data freshness
// Related: irr.go -- verifyRefs, the commit-time guard this check mirrors at runtime
// Related: register.go -- registerIRRDoctor() registers the check and its codes at init
//
// A refresh that learns nothing keeps the prefixes it already has, so a firewall
// filter can enforce data no IRR server has confirmed for days without anything
// failing. `show firewall irr` reports it per entry; this check reports it for
// the whole node, which is where an operator looks when nothing is obviously
// broken.

package irr

import (
	"path/filepath"
	"time"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/resolve/irr/store"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	// codeIRRStaleData fires when a configured IRR reference is enforcing
	// last-known-good prefixes: its most recent refresh learned nothing.
	codeIRRStaleData = "doctor-firewall-irr-stale-data"
	// codeIRRNoData fires when a configured IRR reference has no cached
	// prefixes at all, so it filters nothing.
	codeIRRNoData = "doctor-firewall-irr-no-data"
)

// irrDiagnosticCodes is the explanation metadata for the codes this plugin owns,
// so `ze explain <code>` can describe them. They live with the plugin, so
// removing the plugin removes the codes (ai/rules/plugins.md).
var irrDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:        codeIRRStaleData,
		Title:       "Firewall IRR filter is enforcing stale data",
		Description: "A firewall rule or interface binding references an ASN or AS-SET whose most recent IRR refresh returned no prefixes. Ze keeps the prefixes it learned before, because replacing them with an empty list would drop every packet the filter was written to accept. The filter is enforcing data the IRR has stopped confirming. Check that the IRR server is reachable and that the AS-SET still exists, then run 'update firewall irr all'. When the AS-SET is gone upstream for good, run 'clear firewall irr as-set <name>' to remove its prefixes.",
		Examples:    []string{"ze doctor --json", cmdShowIRR, "ze explain doctor-firewall-irr-stale-data"},
	},
	{
		Code:        codeIRRNoData,
		Title:       "Firewall IRR filter has no prefixes",
		Description: "A firewall rule or interface binding references an ASN or AS-SET with no cached prefixes. The reference filters nothing. Run 'update firewall irr asn <asn>' or 'update firewall irr as-set <name>' to fetch the prefix list.",
		Examples:    []string{"ze doctor --json", "ze explain doctor-firewall-irr-no-data"},
	},
}

// checkIRRDataFreshness reports every configured IRR reference that is
// enforcing stale prefixes or has none. It is a no-op on a node with no
// firewall IRR references, so a node that does not use IRR filtering sees
// nothing.
func checkIRRDataFreshness(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	tree, ok := ctx.Tree.(*config.Tree)
	if !ok || tree == nil {
		return nil
	}
	fw := tree.GetContainer(configRoot)
	if fw == nil {
		return nil
	}
	root := map[string]any{configRoot: fw.ToMap()}
	refs := extractRefsFromConfig(root)
	refs = append(refs, extractIfaceRefs(root)...)
	if len(refs) == 0 {
		return nil
	}

	ps := store.New(nil, nil, doctorCachePath(ctx.ConfigDir))
	if err := ps.Open(); err != nil {
		return nil // the cache file is the store's own problem to report
	}

	now := time.Now()
	seen := make(map[string]bool, len(refs))
	var diags []diagnostic.Diagnostic
	for _, ref := range refs {
		if seen[ref.Name] {
			continue
		}
		seen[ref.Name] = true

		entry := ps.Get(ref.Name)
		if entry == nil || entry.PrefixList().Empty() {
			var tb textbuf.Buffer
			tb.Str("firewall irr: ").Str(ref.Name).Str(" has no cached prefixes and filters nothing")
			diags = append(diags, diagnostic.Diagnostic{
				Code:     codeIRRNoData,
				Severity: diagnostic.SeverityError,
				Message:  tb.String(),
				Help:     updateHelp(ref),
			})
			continue
		}
		if !entry.Stale() {
			continue
		}
		var tb textbuf.Buffer
		tb.Str("firewall irr: ").Str(ref.Name).Str(" is enforcing prefixes learned ")
		tb.Str(entry.RefreshedAt.Format(time.RFC3339))
		tb.Str("; every refresh since ").Str(entry.StaleSince.Format(time.RFC3339)).Str(" learned nothing")
		diags = append(diags, diagnostic.Diagnostic{
			Code:     codeIRRStaleData,
			Severity: diagnostic.SeverityWarning,
			Message:  tb.String(),
			Actual:   int(now.Sub(entry.RefreshedAt).Seconds()),
			Help:     updateHelp(ref),
		})
	}
	return diags
}

// updateHelp names the command that fetches this reference's prefix list.
func updateHelp(ref irrRef) string {
	var tb textbuf.Buffer
	tb.Str("run 'update firewall irr ")
	if ref.IsASSet {
		tb.Str("as-set ").Str(ref.Name)
	} else {
		tb.Str("asn ").Str(bareASN(ref.Name))
	}
	tb.Byte('\'')
	return tb.String()
}

// doctorCachePath is the zefs file holding the persisted prefix cache. Doctor
// may be pointed at a config directory other than the default, so its own
// directory wins when it has one.
func doctorCachePath(configDir string) string {
	if configDir == "" {
		return cacheStorePath()
	}
	return filepath.Join(configDir, "database.zefs")
}
