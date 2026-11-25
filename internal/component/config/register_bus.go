// Design: docs/architecture/api/architecture.md -- bus config registration seams and external declaration store
// Related: bus_validate.go -- ValidateBusConfig and participant discovery
// Related: bus_validate_rules.go -- commit refusals 1-5 and 9-11
// Related: declared_schemas.go -- offline query mode feeding RecordExternalBusDecl

package config

import (
	"maps"
	"slices"
	"strings"
	"sync"

	"github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// ExternalBusDecl holds the bus participant, send, and receive declarations
// learned from one external endpoint (through offline query mode or daemon
// Stage 1).
type ExternalBusDecl struct {
	Endpoint         string
	ParticipantNodes []rpc.ParticipantNodeDecl
	Sends            []rpc.MessageSendDecl
	Receives         []rpc.MessageReceiveDecl
}

var (
	externalBusDeclsMu sync.RWMutex
	externalBusDecls   = map[string]ExternalBusDecl{}
)

// RecordExternalBusDecl records the bus declarations reported by one external
// endpoint so schema augmentation and commit validation see its participant
// nodes, sends, and receives. Safe for concurrent use.
func RecordExternalBusDecl(decl ExternalBusDecl) {
	if decl.Endpoint == "" {
		return
	}
	cloned := cloneExternalBusDecl(decl)
	externalBusDeclsMu.Lock()
	externalBusDecls[decl.Endpoint] = cloned
	externalBusDeclsMu.Unlock()
}

// ExternalBusDeclarations returns a deterministic, defensive copy of all
// recorded external endpoint bus declarations. Safe for concurrent use.
func ExternalBusDeclarations() []ExternalBusDecl {
	externalBusDeclsMu.RLock()
	defer externalBusDeclsMu.RUnlock()
	if len(externalBusDecls) == 0 {
		return nil
	}
	out := make([]ExternalBusDecl, 0, len(externalBusDecls))
	for _, key := range slices.Sorted(maps.Keys(externalBusDecls)) {
		out = append(out, cloneExternalBusDecl(externalBusDecls[key]))
	}
	return out
}

func lookupExternalBusDecl(endpoint string) (ExternalBusDecl, bool) {
	externalBusDeclsMu.RLock()
	defer externalBusDeclsMu.RUnlock()
	decl, ok := externalBusDecls[endpoint]
	if !ok {
		return ExternalBusDecl{}, false
	}
	return cloneExternalBusDecl(decl), true
}

// ResetExternalBusDeclsForTest clears recorded external bus declarations and
// returns a restore function. MUST be paired with t.Cleanup in tests.
func ResetExternalBusDeclsForTest() func() {
	externalBusDeclsMu.Lock()
	saved := make(map[string]ExternalBusDecl, len(externalBusDecls))
	for k, v := range externalBusDecls {
		saved[k] = cloneExternalBusDecl(v)
	}
	externalBusDecls = map[string]ExternalBusDecl{}
	externalBusDeclsMu.Unlock()
	return func() {
		externalBusDeclsMu.Lock()
		externalBusDecls = saved
		externalBusDeclsMu.Unlock()
	}
}

func cloneExternalBusDecl(d ExternalBusDecl) ExternalBusDecl {
	out := ExternalBusDecl{
		Endpoint:         d.Endpoint,
		ParticipantNodes: slices.Clone(d.ParticipantNodes),
	}
	if len(d.Sends) > 0 {
		out.Sends = make([]rpc.MessageSendDecl, len(d.Sends))
		for i, s := range d.Sends {
			out.Sends[i] = rpc.MessageSendDecl{
				Message:             s.Message,
				Tags:                cloneMessageTags(s.Tags),
				DefaultDestinations: slices.Clone(s.DefaultDestinations),
			}
		}
	}
	if len(d.Receives) > 0 {
		out.Receives = make([]rpc.MessageReceiveDecl, len(d.Receives))
		for i, r := range d.Receives {
			out.Receives[i] = rpc.MessageReceiveDecl{
				Message: r.Message,
				From:    slices.Clone(r.From),
				Tags:    cloneMessageTags(r.Tags),
			}
		}
	}
	return out
}

func cloneMessageTags(in []rpc.MessageTagDecl) []rpc.MessageTagDecl {
	if len(in) == 0 {
		return nil
	}
	out := make([]rpc.MessageTagDecl, len(in))
	for i, t := range in {
		out[i] = rpc.MessageTagDecl{
			Name:   t.Name,
			Values: slices.Clone(t.Values),
		}
	}
	return out
}

// BusSecondaryPathResolver resolves a subsystem-registered secondary address
// lookup prefix against a parsed configuration tree. Subsystems register their
// secondary prefixes from init() so config validation stays decoupled from any
// individual protocol.
type BusSecondaryPathResolver struct {
	Prefix  string
	Resolve func(tree *Tree, path string) (canonicalPaths []string, dormant bool, validDesc string, err error)
}

var (
	busSecondaryResolversMu sync.RWMutex
	busSecondaryResolvers   []BusSecondaryPathResolver
)

// RegisterBusSecondaryPathResolver registers a secondary lookup path resolver
// for a prefix such as "/proto/ip/". Callers MUST register from init().
// Safe for concurrent use.
func RegisterBusSecondaryPathResolver(r BusSecondaryPathResolver) {
	if r.Prefix == "" {
		panic("BUG: config: RegisterBusSecondaryPathResolver requires a non-empty Prefix")
	}
	if r.Resolve == nil {
		panic("BUG: config: RegisterBusSecondaryPathResolver requires a non-nil Resolve function")
	}
	busSecondaryResolversMu.Lock()
	defer busSecondaryResolversMu.Unlock()
	for i := range busSecondaryResolvers {
		if busSecondaryResolvers[i].Prefix == r.Prefix {
			busSecondaryResolvers[i] = r
			return
		}
	}
	busSecondaryResolvers = append(busSecondaryResolvers, r)
}

func matchingSecondaryResolver(path string) (BusSecondaryPathResolver, bool) {
	busSecondaryResolversMu.RLock()
	defer busSecondaryResolversMu.RUnlock()
	for _, r := range busSecondaryResolvers {
		if strings.HasPrefix(path, r.Prefix) {
			return r, true
		}
	}
	return BusSecondaryPathResolver{}, false
}

// ResetBusSecondaryPathResolversForTest snapshots and restores the registered
// secondary path resolvers for tests.
func ResetBusSecondaryPathResolversForTest() func() {
	busSecondaryResolversMu.Lock()
	saved := slices.Clone(busSecondaryResolvers)
	busSecondaryResolversMu.Unlock()
	return func() {
		busSecondaryResolversMu.Lock()
		busSecondaryResolvers = saved
		busSecondaryResolversMu.Unlock()
	}
}

// BusValidatedParticipant describes one configured participant discovered
// during bus validation, along with its declared sends/receives and merged
// filter block.
type BusValidatedParticipant struct {
	Path           string
	Kind           string
	HasDeclaration bool
	Sends          []rpc.MessageSendDecl
	Receives       []rpc.MessageReceiveDecl
	Block          plugin.BusBlock
	Source         string
	Ancestors      []string
}

// BusValidationContext provides the resolved bus participants and endpoints to
// registered feature-requirement validators (commit refusal 7).
type BusValidationContext struct {
	Participants map[string]*BusValidatedParticipant
	Endpoints    []plugin.PluginConfig
}

// BusRequirementValidator checks whether configured bus filters satisfy a
// feature's declared bus relationships (commit refusal 7).
type BusRequirementValidator func(tree *Tree, ctx *BusValidationContext) []error

type namedBusRequirementValidator struct {
	name string
	fn   BusRequirementValidator
}

var (
	busRequirementValidatorsMu sync.RWMutex
	busRequirementValidators   []namedBusRequirementValidator
)

// RegisterBusRequirementValidator registers a feature-requirement validator
// invoked by ValidateBusConfig (commit refusal 7). Safe for concurrent use.
func RegisterBusRequirementValidator(name string, fn BusRequirementValidator) {
	if name == "" {
		panic("BUG: config: RegisterBusRequirementValidator requires a non-empty name")
	}
	if fn == nil {
		panic("BUG: config: RegisterBusRequirementValidator requires a non-nil function")
	}
	busRequirementValidatorsMu.Lock()
	defer busRequirementValidatorsMu.Unlock()
	for i := range busRequirementValidators {
		if busRequirementValidators[i].name == name {
			busRequirementValidators[i].fn = fn
			return
		}
	}
	busRequirementValidators = append(busRequirementValidators, namedBusRequirementValidator{name: name, fn: fn})
}

// ResetBusRequirementValidatorsForTest snapshots and restores the registered
// feature-requirement validators for tests.
func ResetBusRequirementValidatorsForTest() func() {
	busRequirementValidatorsMu.Lock()
	saved := slices.Clone(busRequirementValidators)
	busRequirementValidatorsMu.Unlock()
	return func() {
		busRequirementValidatorsMu.Lock()
		busRequirementValidators = saved
		busRequirementValidatorsMu.Unlock()
	}
}

func init() {
	yang.SetBusParticipantTargetsProvider(collectBusParticipantTargets)
	if err := RegisterTreeValidator("bus-config", ValidateBusConfig); err != nil {
		panic("BUG: config: bus-config validator registration failed: " + err.Error())
	}
	if err := RegisterTreeValidator("managed-client-labels", func(tree *Tree) error {
		_, err := readManagedClientLabels(tree.GetContainerPath("bus/hub"))
		return err
	}); err != nil {
		panic("BUG: config: managed-client-labels validator registration failed: " + err.Error())
	}
}

// collectBusParticipantTargets returns all YANG schema paths registered as bus
// participant nodes by compiled-in plugins and active external declarations,
// normalized to unprefixed short paths for synthetic augmentation.
func collectBusParticipantTargets() []string {
	seen := make(map[string]bool)
	var out []string
	add := func(schemaPath string) {
		segs := stripSchemaPrefixes(schemaPath)
		if len(segs) == 0 {
			return
		}
		shortPath := "/" + textbuf.Join(segs, "/")
		if seen[shortPath] {
			return
		}
		seen[shortPath] = true
		out = append(out, shortPath)
	}
	for _, reg := range registry.All() {
		for _, node := range reg.ParticipantNodes {
			add(node.SchemaPath)
		}
	}
	for _, ext := range ExternalBusDeclarations() {
		for _, node := range ext.ParticipantNodes {
			add(node.SchemaPath)
		}
	}
	slices.Sort(out)
	return out
}
