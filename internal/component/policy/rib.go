// Design: plan/post-release/features/spec-route-policy-0-umbrella.md -- Key Design Decisions, the Loc-RIB route evaluator
// Related: fields.go -- the field check a RIB chain passes beside the evaluator check
// Related: rib_compile.go -- the compiler that turns the attachments' config into a snapshot
//
// The Loc-RIB import gate runs on every insert in the engine, so a filter type
// usable at a Loc-RIB attachment registers an in-process route evaluator here,
// compiled from its instance config when the config applies. The gate never
// calls a plugin process.

package policy

import (
	"sync"

	"github.com/ze-software/ze/internal/core/redistevents"
	ribpolicy "github.com/ze-software/ze/internal/core/rib/policy"
)

// RouteResult is one filter's answer for one Loc-RIB route. Ze chain
// semantics: a filter that does not reject passes the route on to the next
// filter, carrying any sets it made; a reject stops the chain; the chain's
// final decides after the last filter.
type RouteResult struct {
	Reject bool
	Sets   ribpolicy.Sets
}

// RouteEvaluator judges one route. It MUST be safe for concurrent use, MUST
// NOT block and SHOULD NOT allocate: the gate calls it on every insert, from
// several shards at once and sometimes under a shard lock (ribpolicy.Chain).
type RouteEvaluator func(r ribpolicy.Route) RouteResult

// RouteEvaluatorCompiler compiles one filter instance's config subtree (the
// entry of its ze:filter list under the policy root) into a RouteEvaluator.
// It MUST NOT change any state: a refused verify and a CLI validate run it.
// An instance whose config cannot be judged at a Loc-RIB attachment (a BGP
// session action, say) is refused with an error naming the fix.
type RouteEvaluatorCompiler func(name string, instance map[string]any) (RouteEvaluator, error)

// RIBScope says which chain of the Loc-RIB snapshot an attachment fills.
// Zero names no scope, so an unset scope is refused at registration.
type RIBScope uint8

const (
	RIBScopeUnspecified RIBScope = iota
	// RIBScopeSource is one protocol's chain: it judges that protocol's paths only.
	RIBScopeSource
	// RIBScopeCentral is the chain every source's paths pass after their own.
	RIBScopeCentral
)

// RIBImport declares one Loc-RIB import attachment: where its chain is
// configured and which chain of the snapshot it fills.
type RIBImport struct {
	// Attachment is the attachment name MustRegisterAttachment registered.
	Attachment string
	// Root is the config root holding `filter { import { ... } }`.
	Root string
	// Scope is the snapshot chain the attachment fills.
	Scope RIBScope
	// Protocol is the redistevents protocol whose paths a source chain
	// judges; empty for the central chain.
	Protocol string
}

// ribImport is a registered RIBImport with its protocol resolved.
type ribImport struct {
	RIBImport
	protocolID redistevents.ProtocolID
}

// ribRegistry holds the route evaluators and the Loc-RIB attachments. Written
// from init(), read at config verify and apply. Safe for concurrent use.
type ribRegistry struct {
	mu         sync.RWMutex
	evaluators map[string]RouteEvaluatorCompiler
	imports    []ribImport
}

var ribReg = &ribRegistry{evaluators: make(map[string]RouteEvaluatorCompiler)}

// MustRegisterRouteEvaluator registers the route evaluator of a filter type,
// from the filter type's own package. A filter type with none is refused at a
// Loc-RIB attachment at load. A second registration is a Ze defect and panics.
func MustRegisterRouteEvaluator(filterType string, compile RouteEvaluatorCompiler) {
	if filterType == "" {
		panic("BUG: policy: route evaluator registered with an empty filter type")
	}
	if compile == nil {
		panic("BUG: policy: route evaluator for " + filterType + " registered with no compiler")
	}
	ribReg.mu.Lock()
	defer ribReg.mu.Unlock()
	if _, dup := ribReg.evaluators[filterType]; dup {
		panic("BUG: policy: route evaluator for " + filterType + " registered twice")
	}
	ribReg.evaluators[filterType] = compile
}

// MustRegisterRIBImport declares a Loc-RIB import attachment, from the init()
// of the plugin whose config holds it, after its MustRegisterAttachment. The
// policy plugin then reads the attachment's root (registry ConfigReads) and
// compiles its chain into the snapshot the gate reads.
func MustRegisterRIBImport(imp RIBImport) {
	if imp.Attachment == "" {
		panic("BUG: policy: Loc-RIB import registered with no attachment")
	}
	if imp.Root == "" {
		panic("BUG: policy: Loc-RIB import " + imp.Attachment + " registered with no config root")
	}
	resolved := ribImport{RIBImport: imp}
	switch imp.Scope {
	case RIBScopeSource:
		if imp.Protocol == "" {
			panic("BUG: policy: Loc-RIB source import " + imp.Attachment + " names no protocol")
		}
		resolved.protocolID = redistevents.RegisterProtocol(imp.Protocol)
	case RIBScopeCentral:
		if imp.Protocol != "" {
			panic("BUG: policy: Loc-RIB central import " + imp.Attachment + " names a protocol")
		}
	case RIBScopeUnspecified:
		panic("BUG: policy: Loc-RIB import " + imp.Attachment + " registered with no scope")
	default:
		panic("BUG: policy: Loc-RIB import " + imp.Attachment + " registered with an unknown scope")
	}
	vocab.mu.RLock()
	_, known := vocab.attachments[imp.Attachment]
	vocab.mu.RUnlock()
	if !known {
		panic("BUG: policy: Loc-RIB import " + imp.Attachment + " registered before its attachment")
	}

	ribReg.mu.Lock()
	defer ribReg.mu.Unlock()
	for _, existing := range ribReg.imports {
		if existing.Attachment == imp.Attachment {
			panic("BUG: policy: Loc-RIB import " + imp.Attachment + " registered twice")
		}
		if imp.Scope == RIBScopeCentral && existing.Scope == RIBScopeCentral {
			panic("BUG: policy: a second central Loc-RIB import " + imp.Attachment + " registered")
		}
	}
	ribReg.imports = append(ribReg.imports, resolved)
	addPolicyConfigRead(imp.Root)
}

// RIBImportRoots answers the config roots of the registered Loc-RIB import
// attachments, in registration order.
func RIBImportRoots() []string {
	ribReg.mu.RLock()
	defer ribReg.mu.RUnlock()
	roots := make([]string, 0, len(ribReg.imports))
	for _, imp := range ribReg.imports {
		roots = append(roots, imp.Root)
	}
	return roots
}

func (r *ribRegistry) compiler(filterType string) (RouteEvaluatorCompiler, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	compile, ok := r.evaluators[filterType]
	return compile, ok
}

func (r *ribRegistry) importList() []ribImport {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]ribImport(nil), r.imports...)
}
