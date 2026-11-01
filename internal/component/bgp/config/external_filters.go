// Design: docs/architecture/api/process-protocol.md -- Filter Declaration (Stage 1)
// Related: peers.go -- validateChainNames, which hands external references here

package bgpconfig

import (
	"errors"
	"strings"
	"sync"

	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// externalChainRef is a chain member naming an external plugin's filter,
// with where the chain sits: the policy attachment and the config context its
// refusal names.
type externalChainRef struct {
	attachment string // "bgp import" or "bgp export"
	context    string // the config path the refusal names
	reference  string // the member as the config spells it, <plugin>:<filter>
	filter     string // the part after the colon
}

// pendingExternal holds the references whose plugin had not declared when the
// config was loaded. The daemon loads its config before external plugins
// reach Stage 1, so those references are judged by checkFilterDeclarations
// when the plugin declares. Keyed by plugin, then by reference; a reference
// loaded twice is held once, so the set is bounded by the distinct references
// configs have named.
//
//nolint:gochecknoglobals // Package-level hand-off between config load and Stage 1.
var pendingExternal struct {
	mu   sync.Mutex
	refs map[string]map[externalChainRef]struct{}
}

// checkExternalChainRef judges a chain member that names an external
// plugin's filter against the filters that plugin declared at Stage 1. When
// the plugin has not declared yet, the reference waits for its Stage 1.
func checkExternalChainRef(reference, attachment, context string) error {
	pluginName, filter, qualified := strings.Cut(reference, ":")
	if !qualified {
		// refuse-by-default: guards validateChainNames -- Registry.ValidateName refused an unqualified unknown name
		return nil
	}
	ref := externalChainRef{attachment: attachment, context: context, reference: reference, filter: filter}
	declared, ok := registry.RuntimeFilterDeclarations(pluginName)
	if ok {
		return judgeExternalChainRef(pluginName, declared, ref)
	}
	pendingExternal.mu.Lock()
	defer pendingExternal.mu.Unlock()
	if pendingExternal.refs == nil {
		pendingExternal.refs = make(map[string]map[externalChainRef]struct{})
	}
	if pendingExternal.refs[pluginName] == nil {
		pendingExternal.refs[pluginName] = make(map[externalChainRef]struct{})
	}
	pendingExternal.refs[pluginName][ref] = struct{}{}
	return nil
}

// judgeExternalChainRef refuses a reference no declared name or pattern
// covers, and one whose declared reads or sets the attachment does not fit.
func judgeExternalChainRef(pluginName string, declared []registry.DeclaredFilter, ref externalChainRef) error {
	d, ok := registry.FindDeclaredFilter(declared, ref.filter)
	if !ok {
		offers := registry.LabelDeclaredFilters(declared)
		if offers == "" {
			offers = "no filter"
		}
		var tb textbuf.Buffer
		tb.Str(ref.context).Str(": chain names ").Quoted(ref.reference).Str(", and plugin ").Quoted(pluginName).
			Str(" declares no filter of that name; it declares ").Str(offers)
		return errors.New(tb.String())
	}
	if err := policy.CheckDeclaredFilter(ref.attachment, ref.reference, fieldReads(d.Reads), d.Sets); err != nil {
		return errors.New(ref.context + ": " + err.Error())
	}
	return nil
}

// checkFilterDeclarations is the Stage 1 judge BGP installs
// (registry.SetFilterDeclarationCheck): every declared read and set is a
// field of the policy vocabulary, the raw read is a read only, and each
// reference a loaded config made to this plugin fits what it declared.
func checkFilterDeclarations(pluginName string, declared []registry.DeclaredFilter) error {
	for i := range declared {
		d := &declared[i]
		var who textbuf.Buffer
		who.Str("plugin ").Quoted(pluginName).Str(" filter ").Str(d.Label())
		declarer := who.String()
		reads := fieldReads(d.Reads)
		if err := policy.CheckFieldNames(declarer, reads); err != nil {
			return err
		}
		// Every field read travels in the filter-update message, so a BGP
		// field this binary cannot write is refused here rather than sent
		// empty (spec-filter-wire-0-umbrella R-10, AC-29). A field no BGP
		// route carries is left to the attachment check at config load
		// (judgeExternalChainRef, checkChain), which names the attachment.
		for _, read := range reads {
			err := policyview.CheckFieldWriter(read)
			if err == nil {
				continue
			}
			if errors.Is(err, policyview.ErrNotBGPField) {
				continue
			}
			return errors.New(declarer + " reads a field Ze cannot send it: " + err.Error())
		}
		for _, set := range d.Sets {
			if set == rpc.FilterFieldRaw {
				return errors.New(declarer + ` declares the raw read as a set; raw is read only, and a filter ` +
					`that rewrites the whole UPDATE answers the raw modify (docs/plugin-development/protocol.md)`)
			}
		}
		if err := policy.CheckFieldNames(declarer, d.Sets); err != nil {
			return err
		}
	}
	pendingExternal.mu.Lock()
	refs := pendingExternal.refs[pluginName]
	delete(pendingExternal.refs, pluginName)
	pendingExternal.mu.Unlock()
	for ref := range refs {
		if err := judgeExternalChainRef(pluginName, declared, ref); err != nil {
			return err
		}
	}
	return nil
}

// fieldReads answers the declared reads that are route fields: every read
// but the raw UPDATE body.
func fieldReads(reads []string) []string {
	fields := make([]string, 0, len(reads))
	for _, read := range reads {
		if read != rpc.FilterFieldRaw {
			fields = append(fields, read)
		}
	}
	return fields
}
