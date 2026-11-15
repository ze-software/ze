// Design: docs/architecture/firewall/firewall-irr.md -- doctor checks for the IRR lists the firewall enforces
// Related: irr.go -- verifyStored, the commit-time guard checkIRRDataFreshness mirrors
// Related: stored.go -- the reader that records what answerDoctorCheck reports
// Related: register.go -- registerIRRDoctor() registers the check and its codes at init
//
// Two checks, split by what each reads (docs/architecture/doctor-and-health-checks.md,
// "The phase decides what a check may read"). firewall-irr-data-freshness runs
// after config and reads the irr plugin's stored copies through the daemon's
// store: a reference with nothing stored, a stored list too long for a set, a
// stale list. firewall-irr-stored-copies is a runtime check answered by the
// plugin's doctor callback, because the plugin may run forked and only it
// knows its live state: a stored value it could not use, a generation it has
// not applied, an interest key it could not save, tables it could not program.

package irr

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"slices"
	"time"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/resolve/irr/store"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/resolve"
	"github.com/ze-software/ze/internal/core/textbuf"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

const (
	// codeIRRStaleData fires when a configured IRR reference is enforcing
	// last-known-good prefixes: its most recent refresh learned nothing.
	codeIRRStaleData = "doctor-firewall-irr-stale-data"
	// codeIRRNoData fires when the irr plugin stores nothing for a configured
	// IRR reference, so it filters nothing.
	codeIRRNoData = "doctor-firewall-irr-no-data"
	// codeIRROversizedData fires when a configured IRR reference holds more
	// prefixes in one family than a firewall set takes, so the apply is
	// refused and the rules naming it are not in the kernel.
	codeIRROversizedData = "doctor-firewall-irr-oversized-data"
	// codeIRRStoredCopyUnreadable fires when a stored list, its pointer or an
	// AS number's binding exists and the plugin could not use it, so the
	// firewall enforces the list it read before, or none.
	codeIRRStoredCopyUnreadable = "doctor-firewall-irr-stored-copy-unreadable"
	// codeIRRStoredCopyBehind fires when the irr plugin raised a generation the
	// plugin has not applied: a notice was not delivered, so the firewall may
	// enforce an older copy than the one stored.
	codeIRRStoredCopyBehind = "doctor-firewall-irr-stored-copy-behind"
	// codeIRRInterestUnsaved fires when the plugin's interest key was not
	// written, so the irr plugin may not refresh what the firewall enforces.
	codeIRRInterestUnsaved = "doctor-firewall-irr-interest-unsaved"
	// codeIRRNotProgrammed fires when the firewall refused the tables naming
	// IRR sets, so the rules naming them are not in the kernel.
	codeIRRNotProgrammed = "doctor-firewall-irr-not-programmed"
	// cmdDoctorJSON is the first example every code here carries: the command
	// that reports the finding in the first place.
	cmdDoctorJSON = "ze doctor --json"
	// cmdUpdateIRRAll is the fix every stored-copy finding carries: the irr plugin
	// fetches every set again, and the firewall reads and applies what it stores.
	cmdUpdateIRRAll = "update irr all"
)

// storedCopyCheckName names the runtime check in the Stage 1 registration and
// in the doctor callback's dispatch.
const storedCopyCheckName = "firewall-irr-stored-copies"

// storedCopyCheckDecl declares the runtime check. Codes is a literal, so
// `./le arch doctor-coverage check` reads what the check declares.
var storedCopyCheckDecl = sdk.DoctorCheckDecl{
	Name:  storedCopyCheckName,
	Phase: sdk.DoctorPhaseRuntime,
	Codes: []string{
		codeIRRStoredCopyUnreadable,
		codeIRRStoredCopyBehind,
		codeIRRInterestUnsaved,
		codeIRRNotProgrammed,
	},
}

// doctorReadTimeout bounds the store reads one doctor answer makes.
const doctorReadTimeout = 5 * time.Second

// irrDiagnosticCodes is the explanation metadata for the codes this plugin owns,
// so `ze show diagnostic <code>` can describe them. They live with the plugin, so
// removing the plugin removes the codes (ai/rules/plugins.md).
var irrDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:        codeIRRStaleData,
		Title:       "Firewall IRR filter is enforcing stale data",
		Description: "A firewall rule or interface binding references an ASN or AS-SET whose most recent IRR refresh returned no prefixes. The irr plugin keeps the prefixes it stored before, because replacing them with an empty list would drop every packet the filter was written to accept. The filter is enforcing data the IRR has stopped confirming. Check that the IRR server is reachable and that the AS-SET still exists, then run 'update irr all'. When the AS-SET is gone upstream for good, run 'clear irr as-set <name>' to remove its prefixes.",
		Examples:    []string{cmdDoctorJSON, cmdShowIRR, "ze show diagnostic doctor-firewall-irr-stale-data"},
	},
	{
		Code:        codeIRRNoData,
		Title:       "Firewall IRR filter has no prefixes",
		Description: "A firewall rule or interface binding references an ASN or AS-SET for which the irr plugin stores no prefix list. The reference filters nothing, and the table naming it is held back. Run 'update irr asn <asn>' or 'update irr as-set <name>' to fetch and store the prefix list; the firewall programs it on the irr plugin's notice.",
		Examples:    []string{cmdDoctorJSON, "ze show irr", "ze show diagnostic doctor-firewall-irr-no-data"},
	},
	{
		Code:        codeIRROversizedData,
		Title:       "Firewall IRR reference is too large for a firewall set",
		Description: "A firewall rule or interface binding references an ASN or AS-SET whose IPv4 or IPv6 prefix list is longer than one firewall set holds. Ze does not program a part of it, because a set that holds part of a prefix list drops traffic an accept rule was written to pass, and passes traffic a drop rule was written to block. The rules naming the reference are not in the kernel. Narrow the reference to the members you filter on, or run 'clear irr as-set <name>' to remove it.",
		Examples:    []string{cmdDoctorJSON, cmdShowIRR, "ze show diagnostic doctor-firewall-irr-oversized-data"},
	},
	{
		Code:        codeIRRStoredCopyUnreadable,
		Title:       "firewall-irr cannot read a stored IRR list",
		Description: "The irr plugin stored a list, its pointer or an AS number's binding for a reference the firewall enforces, and firewall-irr could not read it, or it failed its digest. The firewall keeps the list it read before; with none, the table naming it is held back. Run 'update irr as-set <name>' to fetch and store the set again; the firewall reads it on the notice.",
		Examples:    []string{cmdDoctorJSON, cmdShowIRR, "ze show diagnostic " + codeIRRStoredCopyUnreadable},
	},
	{
		Code:        codeIRRStoredCopyBehind,
		Title:       "firewall-irr missed a stored IRR list",
		Description: "The irr plugin stored a newer list than firewall-irr has read: a notice did not reach the plugin. It catches up at the next notice. Run 'update irr all' to store every set again, which sends the notices.",
		Examples:    []string{cmdDoctorJSON, cmdShowIRR, "ze show diagnostic " + codeIRRStoredCopyBehind},
	},
	{
		Code:        codeIRRInterestUnsaved,
		Title:       "firewall-irr could not save the AS-SETs it enforces",
		Description: "The store refused the key through which firewall-irr tells the irr plugin which AS-SETs and AS numbers it enforces, so the irr plugin may not refresh them. The plugin writes it again every minute. Fix the store error in the message, or run 'update irr all' to fetch every referenced set now.",
		Examples:    []string{cmdDoctorJSON, "ze show irr", "ze show diagnostic " + codeIRRInterestUnsaved},
	},
	{
		Code:        codeIRRNotProgrammed,
		Title:       "firewall-irr could not program its IRR sets",
		Description: "The firewall refused the sets and tables firewall-irr built from the stored IRR lists, so the rules naming them are not in the kernel. The plugin applies them again every minute and after every stored list it reads. Fix the error in the message, then run 'update irr all' to send the notices that apply them now.",
		Examples:    []string{cmdDoctorJSON, cmdShowIRR, "ze show diagnostic " + codeIRRNotProgrammed},
	},
}

// storeKeyReader is the read half of the daemon's store the post-config check
// opens (storage.OpenReadOnly).
type storeKeyReader interface {
	ReadKey(key string) ([]byte, error)
}

// keyStateReader answers store.StateReader over the daemon's store, so the
// post-config check reads through the one reader every consumer uses.
type keyStateReader struct{ keys storeKeyReader }

// StateGet answers a key the store does not hold as not found, never as an
// error: the reader maps it to ErrNotStored.
func (r keyStateReader) StateGet(_ context.Context, key string) ([]byte, bool, error) {
	value, err := r.keys.ReadKey(key)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return value, true, nil
}

// checkIRRDataFreshness reports every configured IRR reference whose stored
// copy filters nothing, does not fit a firewall set, or is stale. It is a no-op
// on a node with no firewall IRR references, so a node that does not use IRR
// filtering sees nothing.
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

	dir := ctx.ConfigDir
	if dir == "" {
		dir = resolve.StoreDir("")
	}
	persistence, err := storage.OpenReadOnly(dir, dir)
	if err != nil {
		return nil // The storage check reports unavailable or unsafe stores.
	}
	defer func() { _ = persistence.Close() }()
	return storedCopyDiagnostics(keyStateReader{keys: persistence}, refs, time.Now())
}

// storedCopyDiagnostics judges each reference's stored copy. A stored value
// that cannot be read is no finding here: the irr plugin's irr-datasets check
// reports it (doctor-irr-dataset-unreadable), and firewall-irr-stored-copies
// reports the copy the plugin keeps instead.
func storedCopyDiagnostics(state store.StateReader, refs []irrRef, now time.Time) []diagnostic.Diagnostic {
	ctx, cancel := context.WithTimeout(context.Background(), doctorReadTimeout)
	defer cancel()
	seen := make(map[string]bool, len(refs))
	var diags []diagnostic.Diagnostic
	for _, ref := range refs {
		if seen[ref.Name] {
			continue
		}
		seen[ref.Name] = true
		st := newRefState(ref)
		if st.keyErr != "" {
			continue // the commit refused it; it names no key to read
		}
		stored, err := readStoredCopy(ctx, state, st)
		if errors.Is(err, store.ErrNotStored) || (err == nil && stored.entry.PrefixList().Empty()) {
			var tb textbuf.Buffer
			tb.Str("firewall irr: ").Str(ref.Name).Str(" has no stored prefixes and filters nothing")
			diags = append(diags, diagnostic.Diagnostic{
				Code:     codeIRRNoData,
				Severity: diagnostic.SeverityError,
				Message:  tb.String(),
				Fix:      fetchCommand(ref),
			})
			continue
		}
		if err != nil {
			continue
		}
		// An oversized entry is reported instead of a stale one. Both say the
		// filter is not what the config asks for, and this one says the rules
		// are not programmed at all, which is the larger fact.
		entry := stored.entry
		if oversized := oversizedEntryError(ref.Name, entry); oversized != nil {
			diags = append(diags, diagnostic.Diagnostic{
				Code:     codeIRROversizedData,
				Severity: diagnostic.SeverityError,
				Message:  oversized.Error(),
				Actual:   len(entry.IPv4) + len(entry.IPv6),
				Expected: maxPrefixesPerFamily,
				Fix:      clearCommand(ref),
			})
			continue
		}
		if !entry.Stale() {
			continue
		}
		var tb textbuf.Buffer
		tb.Str("firewall irr: ").Str(ref.Name).Str(" is enforcing prefixes learned ")
		tb.Str(entry.RefreshedAt.UTC().Format(time.RFC3339))
		tb.Str("; every refresh since ").Str(entry.StaleSince.UTC().Format(time.RFC3339)).Str(" learned nothing")
		diags = append(diags, diagnostic.Diagnostic{
			Code:     codeIRRStaleData,
			Severity: diagnostic.SeverityWarning,
			Message:  tb.String(),
			Actual:   int(now.Sub(entry.RefreshedAt).Seconds()),
			Fix:      fetchCommand(ref),
		})
	}
	return diags
}

// answerDoctorCheck is the plugin's doctor callback. It answers the one
// runtime check the plugin declares and refuses a name it did not declare.
func (plug *irrPlugin) answerDoctorCheck(name string) ([]sdk.DoctorCheckDiagnostic, error) {
	if name != storedCopyCheckName {
		return nil, fmt.Errorf("firewall-irr: unknown doctor check %s", name)
	}
	findings := plug.unreadableFindings()
	if failure := plug.interestErr.Load(); failure != nil {
		var tb textbuf.Buffer
		tb.Str("firewall-irr: the AS-SETs it enforces were not saved since ").Str(failure.since.UTC().Format(time.RFC3339))
		tb.Str(" (").Str(failure.err).Str(")")
		findings = append(findings, sdk.DoctorCheckDiagnostic{
			Code: codeIRRInterestUnsaved, Severity: string(diagnostic.SeverityWarning),
			Message: tb.String(), Fix: cmdUpdateIRRAll,
		})
	}
	if failure := plug.applyErr.Load(); failure != nil {
		var tb textbuf.Buffer
		tb.Str("firewall-irr: the tables naming IRR sets were not programmed since ").Str(failure.since.UTC().Format(time.RFC3339))
		tb.Str(" (").Str(failure.err).Str(")")
		findings = append(findings, sdk.DoctorCheckDiagnostic{
			Code: codeIRRNotProgrammed, Severity: string(diagnostic.SeverityError),
			Message: tb.String(), Fix: cmdUpdateIRRAll,
		})
	}
	if behind, ok := plug.behindFinding(); ok {
		findings = append(findings, behind)
	}
	return findings, nil
}

// unreadableFindings is one finding per reference whose stored value the
// reader could not use, sorted by name.
func (plug *irrPlugin) unreadableFindings() []sdk.DoctorCheckDiagnostic {
	plug.mu.RLock()
	defer plug.mu.RUnlock()
	names := make([]string, 0, len(plug.refs))
	for name, st := range plug.refs {
		if st.readErr != "" {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	out := make([]sdk.DoctorCheckDiagnostic, 0, len(names))
	for _, name := range names {
		st := plug.refs[name]
		var tb textbuf.Buffer
		tb.Str("firewall-irr: the stored list for ").Str(name).Str(" cannot be used (").Str(st.readErr).Str(")")
		if st.copy != nil {
			tb.Str("; the list read before stays enforced")
		}
		out = append(out, sdk.DoctorCheckDiagnostic{
			Code: codeIRRStoredCopyUnreadable, Severity: string(diagnostic.SeverityWarning),
			Message: tb.String(), Fix: fetchCommand(st.ref),
		})
	}
	return out
}

// behindFinding compares the generation the irr plugin last raised with the
// one the reader applied. A generation that cannot be read is no finding here:
// the irr plugin's own check reports an unreadable store.
func (plug *irrPlugin) behindFinding() (sdk.DoctorCheckDiagnostic, bool) {
	if plug.reader == nil {
		return sdk.DoctorCheckDiagnostic{}, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), doctorReadTimeout)
	defer cancel()
	stored, err := store.ReadGeneration(ctx, plug.reader.state)
	if err != nil {
		return sdk.DoctorCheckDiagnostic{}, false
	}
	applied := plug.reader.seen.Load()
	if stored <= applied {
		return sdk.DoctorCheckDiagnostic{}, false
	}
	var tb textbuf.Buffer
	tb.Str("firewall-irr: applied IRR generation ").Uint32(applied).Str(", the irr plugin stored ").Uint32(stored)
	return sdk.DoctorCheckDiagnostic{
		Code: codeIRRStoredCopyBehind, Severity: string(diagnostic.SeverityWarning),
		Message: tb.String(), Fix: cmdUpdateIRRAll,
	}, true
}
