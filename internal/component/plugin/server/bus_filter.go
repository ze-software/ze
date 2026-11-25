// Design: docs/architecture/api/architecture.md -- compiled bus filters, origin tracking, zero-alloc delivery index, and sender stamping
// Related: bus_tag_intern.go -- interned tag IDs and zero-alloc BusTagMatcher
// Related: delivery_graph.go -- runtime delivery index consumer
// Related: ../bus_path.go -- short bus path syntax and segment prefix matching
// Related: ../bus_config.go -- parsed BusBlock AST and equality helpers

package server

import (
	"maps"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

const (
	maxParentInheritanceDepth = 16

	busDirSend              = "send"
	busDirReceive           = "receive"
	busKeyAllow             = "allow"
	busKeyTo                = "to"
	busKeyFrom              = "from"
	busSepArrow             = " > "
	busLabelMsg             = "message "
	busLabelTag             = "tag "
	busParticipantSrcPrefix = "bus > participant "
	busCLIPrefix            = events.NamespaceCLI + ":"
)

// BusEdgeOrigin states why a compiled filter rule or delivery edge exists.
type BusEdgeOrigin uint8

const (
	// BusOriginRegistered marks a default edge or rule derived from code registration.
	BusOriginRegistered BusEdgeOrigin = iota
	// BusOriginWritten marks a rule or edge produced by explicit operator configuration.
	BusOriginWritten
	// BusOriginInherited marks a rule or edge inherited from a parent container.
	BusOriginInherited
	// BusOriginRequired marks an edge required by an active configured feature.
	BusOriginRequired
	// BusOriginDormant marks a declared reception with no active matching sender or empty runtime tag overlap.
	BusOriginDormant
)

// String returns the canonical human-readable origin name.
func (o BusEdgeOrigin) String() string {
	switch o {
	case BusOriginRegistered:
		return "registered"
	case BusOriginWritten:
		return "written"
	case BusOriginInherited:
		return "inherited"
	case BusOriginRequired:
		return "required"
	case BusOriginDormant:
		return "dormant"
	default:
		panic("BUG: server: invalid BusEdgeOrigin value")
	}
}

// BusSendRefusalReason states why MaySend or MaySendWithTags refused a send.
type BusSendRefusalReason uint8

const (
	// SendAllowed indicates the send is permitted.
	SendAllowed BusSendRefusalReason = iota
	// SendRefusedUnknownSender indicates the sender path is not registered in the compiled bus.
	SendRefusedUnknownSender
	// SendRefusedUndeclared indicates the sender's kind did not declare sending the message.
	SendRefusedUndeclared
	// SendRefusedAllowList indicates the sender's send allow-list excludes the message.
	SendRefusedAllowList
	// SendRefusedDestination indicates the destination is outside the sender's to set or refuses the sender.
	SendRefusedDestination
	// SendRefusedOutboundTag indicates the message's tags fail the sender's outbound tag filter.
	SendRefusedOutboundTag
)

// String returns the canonical human-readable refusal reason name.
func (r BusSendRefusalReason) String() string {
	switch r {
	case SendAllowed:
		return "allowed"
	case SendRefusedUnknownSender:
		return "unknown-sender"
	case SendRefusedUndeclared:
		return "undeclared"
	case SendRefusedAllowList:
		return "allow-list"
	case SendRefusedDestination:
		return "destination"
	case SendRefusedOutboundTag:
		return "outbound-tag"
	default:
		panic("BUG: server: invalid BusSendRefusalReason value")
	}
}

// BusParticipantInput describes one participant's declarations and configured
// bus blocks supplied to CompileBus.
type BusParticipantInput struct {
	// Path is the canonical short bus path of this participant.
	Path string
	// EndpointName is the endpoint or process name used for delivery when this
	// participant is an endpoint; empty for non-endpoint participants.
	EndpointName string
	// OwnerEndpoint names the authenticated endpoint permitted to stamp
	// messages as this participant; empty when owned by EndpointName itself.
	OwnerEndpoint string
	// SecondaryPaths holds optional alias paths resolving to this exact
	// participant (e.g. an address-keyed lookup path).
	SecondaryPaths []string
	// ParentPath holds the optional parent container path for dynamic or nested
	// participants and inheritance origin reporting.
	ParentPath string
	// Sends declares the bus messages this participant's kind may send.
	Sends []rpc.MessageSendDecl
	// Receives declares the bus messages this participant's kind is willing to
	// receive.
	Receives []rpc.MessageReceiveDecl
	// OwnBlock is the participant's own bus block (or pre-inherited block when
	// OwnOrigin is BusOriginInherited).
	OwnBlock plugin.BusBlock
	// OwnOrigin states whether OwnBlock came from BusOriginWritten or
	// BusOriginInherited. Zero defaults to BusOriginWritten when OwnBlock is
	// non-empty.
	OwnOrigin BusEdgeOrigin
	// OwnSource is the human-readable config location of OwnBlock.
	OwnSource string
	// BusSideBlock is the central bus > endpoint or bus > participant block at
	// this participant's own rank.
	BusSideBlock plugin.BusBlock
	// BusSideSource is the human-readable config location of BusSideBlock.
	BusSideSource string
	// ParentBusSideBlock holds an optional central bus > participant block
	// written at ParentPath's rank (overridden item-by-item by OwnBlock).
	ParentBusSideBlock plugin.BusBlock
	// ParentBusSideSource is the human-readable config location of
	// ParentBusSideBlock.
	ParentBusSideSource string
	// RuntimeReceiveTags optionally supplies runtime-narrowed tag filters
	// (message -> tag -> allowed values) declared after commit so empty
	// intersections surface as dormant receptions instead of commit errors.
	RuntimeReceiveTags map[string]map[string][]string
	// Required marks edges touching this participant as required by a feature.
	Required bool
	// PureContainer marks a parent container participant (such as a group) that
	// carries inheritable bus blocks but does not send or receive on its own.
	PureContainer bool
}

// BusRuleOrigin records why one compiled rule holds and which config source
// line(s) produced it.
type BusRuleOrigin struct {
	Origin  BusEdgeOrigin `json:"origin"`
	Sources []string      `json:"sources,omitempty"`
}

// BusCompiledSendRule holds the compiled effective send policy for one
// declared message of a participant.
type BusCompiledSendRule struct {
	Message           string                   `json:"message"`
	Allowed           bool                     `json:"allowed"`
	AllowOrigin       BusRuleOrigin            `json:"allow-origin"`
	ToNone            bool                     `json:"to-none,omitempty"`
	ToAll             bool                     `json:"to-all,omitempty"`
	ToPaths           []string                 `json:"to-paths,omitempty"`
	ToOrigin          BusRuleOrigin            `json:"to-origin"`
	ReplacedDefaults  []string                 `json:"replaced-defaults,omitempty"`
	ResolvedPaths     []string                 `json:"resolved-paths,omitempty"`
	ResolvedEndpoints []string                 `json:"resolved-endpoints,omitempty"`
	OutboundTagMap    map[string][]string      `json:"outbound-tags,omitempty"`
	OutboundTags      *BusTagMatcher           `json:"-"`
	TagOrigins        map[string]BusRuleOrigin `json:"tag-origins,omitempty"`
}

// BusCompiledReceiveRule holds the compiled effective receive policy for one
// declared message of a participant.
type BusCompiledReceiveRule struct {
	Message          string                   `json:"message"`
	Allowed          bool                     `json:"allowed"`
	AllowOrigin      BusRuleOrigin            `json:"allow-origin"`
	FromNone         bool                     `json:"from-none,omitempty"`
	FromAll          bool                     `json:"from-all,omitempty"`
	FromPaths        []string                 `json:"from-paths,omitempty"`
	FromOrigin       BusRuleOrigin            `json:"from-origin"`
	ReplacedDefaults []string                 `json:"replaced-defaults,omitempty"`
	ResolvedPaths    []string                 `json:"resolved-paths,omitempty"`
	Direction        events.Direction         `json:"-"`
	InboundTagMap    map[string][]string      `json:"inbound-tags,omitempty"`
	InboundTags      *BusTagMatcher           `json:"-"`
	TagOrigins       map[string]BusRuleOrigin `json:"tag-origins,omitempty"`
	Dormant          bool                     `json:"dormant,omitempty"`
}

// BusDormantReception records a declared reception that currently produces no
// active delivery edge because no matching sender is present or because a
// runtime tag filter shares no value with the effective filter.
type BusDormantReception struct {
	ParticipantPath string `json:"participant-path"`
	Message         string `json:"message"`
	Reason          string `json:"reason"`
}

// CompiledParticipantFilter holds the single resolved send and receive filter
// compiled from registration, participant-side config, inherited parent config,
// and central bus-side config for one participant.
type CompiledParticipantFilter struct {
	Path           string   `json:"path"`
	EndpointName   string   `json:"endpoint-name,omitempty"`
	OwnerEndpoint  string   `json:"owner-endpoint,omitempty"`
	SecondaryPaths []string `json:"secondary-paths,omitempty"`
	ParentPath     string   `json:"parent-path,omitempty"`

	SendAllowNone        bool                            `json:"send-allow-none,omitempty"`
	SendAllowAllDeclared bool                            `json:"send-allow-all-declared,omitempty"`
	AllowedSendMessages  []string                        `json:"allowed-send-messages,omitempty"`
	AllowedCLIPrefixes   []string                        `json:"allowed-cli-prefixes,omitempty"`
	SendAllowOrigin      BusRuleOrigin                   `json:"send-allow-origin"`
	SendToNone           bool                            `json:"send-to-none,omitempty"`
	SendToAll            bool                            `json:"send-to-all,omitempty"`
	SendToPaths          []string                        `json:"send-to-paths,omitempty"`
	SendToOrigin         BusRuleOrigin                   `json:"send-to-origin"`
	SendRules            map[string]*BusCompiledSendRule `json:"send-rules,omitempty"`

	ReceiveAllowNone        bool                               `json:"receive-allow-none,omitempty"`
	ReceiveAllowAllDeclared bool                               `json:"receive-allow-all-declared,omitempty"`
	AllowedReceiveMessages  []string                           `json:"allowed-receive-messages,omitempty"`
	ReceiveAllowOrigin      BusRuleOrigin                      `json:"receive-allow-origin"`
	ReceiveFromNone         bool                               `json:"receive-from-none,omitempty"`
	ReceiveFromAll          bool                               `json:"receive-from-all,omitempty"`
	ReceiveFromPaths        []string                           `json:"receive-from-paths,omitempty"`
	ReceiveFromOrigin       BusRuleOrigin                      `json:"receive-from-origin"`
	ReceiveRules            map[string]*BusCompiledReceiveRule `json:"receive-rules,omitempty"`

	DormantReceptions []BusDormantReception `json:"dormant-receptions,omitempty"`
}

// BusCompiledEdge describes one effective (or dormant) delivery edge between
// participants for inspection surfaces.
type BusCompiledEdge struct {
	SenderPath       string        `json:"sender-path"`
	ReceiverPath     string        `json:"receiver-path"`
	ReceiverEndpoint string        `json:"receiver-endpoint,omitempty"`
	Message          string        `json:"message"`
	Origin           BusEdgeOrigin `json:"origin"`
	TagSummary       string        `json:"tag-summary,omitempty"`
	ParentPath       string        `json:"parent-path,omitempty"`
}

// BusResolvedTarget carries one pre-resolved destination participant's
// canonical path, optional endpoint name, and secondary lookup paths.
type BusResolvedTarget struct {
	Path           string   `json:"path"`
	EndpointName   string   `json:"endpoint-name,omitempty"`
	SecondaryPaths []string `json:"secondary-paths,omitempty"`
}

// BusSenderStampResult reports the canonical sender path stamped by the bus
// from an authenticated connection and whether a caller-supplied sender path
// was overridden.
type BusSenderStampResult struct {
	StampedPath string
	Spoofed     bool
}

type busDeliveryKey struct {
	senderIdx uint32
	ns        events.NamespaceID
	et        events.EventTypeID
	dir       events.Direction
}

type compiledReceiver struct {
	EndpointName string
	Path         string
	TagFilter    *BusTagMatcher
	Origin       BusEdgeOrigin
}

type busFilterTag struct {
	name string
	id   TagID
}

type busMessageKey struct {
	ns events.NamespaceID
	et events.EventTypeID
}

type busEndpointMsgKey struct {
	endpoint string
	ns       events.NamespaceID
	et       events.EventTypeID
}

type busDeliveryEntry struct {
	receivers    []compiledReceiver
	endpointOnly []string       // non-nil when no receiver carries a runtime TagFilter
	filterTags   []busFilterTag // deduplicated tags tested by receivers when endpointOnly == nil
}

type busSendKey struct {
	senderIdx uint32
	message   string
}

type compiledSendEntry struct {
	status         BusSendRefusalReason
	isCLI          bool
	toNone         bool
	allowedDests   map[uint32]struct{}
	allowedPaths   []string
	allowedTargets []BusResolvedTarget
	outboundTags   *BusTagMatcher
}

type compiledCLISend struct {
	prefix string
	entry  *compiledSendEntry
}

// CompiledBus is an immutable snapshot of all compiled participant filters,
// pre-resolved zero-alloc delivery tables, send permission tables, and
// inspection edges. Safe for concurrent use after CompileBus returns.
type CompiledBus struct {
	interner           *TagInterner
	pathToIdx          map[string]uint32
	endpointToIdx      map[string]uint32
	order              []string
	filters            []*CompiledParticipantFilter
	deliveryIndex      map[busDeliveryKey]busDeliveryEntry
	inboundTagIndex    map[busMessageKey][]busFilterTag
	inboundTagMatchers map[busEndpointMsgKey]*BusTagMatcher
	sendIndex          map[busSendKey]*compiledSendEntry
	cliSends           map[uint32][]compiledCLISend
	edges              []BusCompiledEdge
}

// CompileBus compiles every participant's registration declarations, own bus
// block, inherited parent bus blocks, and central bus-side block into one
// immutable CompiledBus snapshot.
func CompileBus(interner *TagInterner, participants []BusParticipantInput) (*CompiledBus, error) {
	if interner == nil {
		interner = NewTagInterner()
	}
	bus := &CompiledBus{
		interner:      interner,
		pathToIdx:     make(map[string]uint32, len(participants)*2),
		endpointToIdx: make(map[string]uint32, len(participants)),
		order:         make([]string, 0, len(participants)),
		filters:       make([]*CompiledParticipantFilter, len(participants)),
		deliveryIndex: make(map[busDeliveryKey]busDeliveryEntry),
		sendIndex:     make(map[busSendKey]*compiledSendEntry),
		cliSends:      make(map[uint32][]compiledCLISend),
	}
	if len(participants) == 0 {
		return bus, nil
	}

	sorted := make([]BusParticipantInput, len(participants))
	copy(sorted, participants)
	slices.SortFunc(sorted, func(a, b BusParticipantInput) int {
		return strings.Compare(a.Path, b.Path)
	})

	parentContainers := make(map[string]bool)
	for i := range sorted {
		p := &sorted[i]
		if err := plugin.ValidateBusPathSyntax(p.Path); err != nil {
			return nil, err
		}
		if _, dup := bus.pathToIdx[p.Path]; dup {
			var tb textbuf.Buffer
			return nil, opererr.New(
				tb.Str("duplicate bus participant path ").Quoted(p.Path).String(),
				"each bus participant must have a unique canonical path",
				"keep one participant entry for "+p.Path)
		}
		idx := uint32(i)
		bus.pathToIdx[p.Path] = idx
		bus.order = append(bus.order, p.Path)
		if p.EndpointName != "" {
			bus.endpointToIdx[p.EndpointName] = idx
		} else if epName, isEP := strings.CutPrefix(p.Path, plugin.BusEndpointPathPrefix); isEP && epName != "" {
			bus.endpointToIdx[epName] = idx
		}

		for _, sp := range p.SecondaryPaths {
			if err := plugin.ValidateBusPathSyntax(sp); err != nil {
				return nil, err
			}
			if existing, dup := bus.pathToIdx[sp]; dup && existing != idx {
				var tb textbuf.Buffer
				return nil, opererr.New(
					tb.Str("secondary bus path ").Quoted(sp).Str(" collides across ").
						Quoted(sorted[existing].Path).Str(" and ").Quoted(p.Path).String(),
					"a secondary path must identify exactly one participant",
					"ensure each participant registers a unique secondary path")
			}
			bus.pathToIdx[sp] = idx
		}
		if p.ParentPath != "" {
			parentContainers[p.ParentPath] = true
		}
	}

	canonicalPaths := make([]string, len(sorted))
	for i := range sorted {
		canonicalPaths[i] = sorted[i].Path
	}

	// Build message tag catalog and sender existence set from events registry
	// and all participants' declared sends ("Sends before receptions").
	msgTagDefs, hasSenderForMsg := buildMessageTagCatalog(sorted)

	// Step 1: Resolve same-rank blocks for every participant (Refusal 6).
	selfLayers := make([][]busRankLayer, len(sorted))
	for i := range sorted {
		layers, err := buildSelfRankLayers(&sorted[i], bus.pathToIdx, canonicalPaths)
		if err != nil {
			return nil, err
		}
		selfLayers[i] = layers
	}

	// Step 2: Assemble full inheritance layer stacks (self -> parent bus-side
	// -> ancestor chain) and compile each participant's single filter.
	states := make([]compiledParticipantState, len(sorted))
	for i := range sorted {
		p := &sorted[i]
		layers := buildFullLayerStack(p, sorted, selfLayers, i, bus.pathToIdx)
		isPureContainer := (p.PureContainer || parentContainers[p.Path]) && len(p.Sends) == 0 && len(p.Receives) == 0
		st, err := compileParticipantFilter(uint32(i), p, layers, isPureContainer, interner, msgTagDefs)
		if err != nil {
			return nil, err
		}
		states[i] = st
		bus.filters[i] = st.filter
	}

	// Step 3: Populate ReplacedDefaults for overridden sends, then resolve
	// Conditions 1..7 into deliveryIndex, sendIndex, cliSends, and edges.
	populateReplacedSendDefaults(states, sorted, bus.pathToIdx)
	compileDeliveryAndSendIndices(bus, states, sorted, hasSenderForMsg)

	return bus, nil
}

// Interner returns the TagInterner used by b.
func (b *CompiledBus) Interner() *TagInterner {
	if b == nil {
		return nil
	}
	return b.interner
}

// Len returns the number of canonical participants in b.
func (b *CompiledBus) Len() int {
	if b == nil {
		return 0
	}
	return len(b.order)
}

// Participants returns the sorted canonical paths of every participant in b.
// The returned slice is read-only and must not be mutated.
func (b *CompiledBus) Participants() []string {
	if b == nil {
		return nil
	}
	return b.order
}

// Filter returns the single compiled filter for participantPath (accepting
// either a canonical path or a registered secondary path), or nil if unknown.
func (b *CompiledBus) Filter(participantPath string) *CompiledParticipantFilter {
	if b == nil {
		return nil
	}
	idx, ok := b.pathToIdx[participantPath]
	// refuse-by-default: guards CompiledParticipantFilter -- unknown path has no compiled filter and callers branch on nil
	if !ok {
		return nil
	}
	return b.filters[idx]
}

// Edges returns the deduplicated, deterministically sorted list of compiled
// delivery edges (including dormant receptions) for inspection surfaces.
// The returned slice is read-only and must not be mutated.
func (b *CompiledBus) Edges() []BusCompiledEdge {
	if b == nil {
		return nil
	}
	return b.edges
}

// Receivers returns the receiver endpoint names (or canonical paths for
// non-endpoint participants) that admit an event from senderPath matching
// (ns, et, dir, msgTags).
//
// Zero-allocation contract:
//   - When no matching receiver carries a runtime TagFilter and dst is nil,
//     Receivers returns a stored read-only slice directly with 0 allocations
//     and 0 tag comparisons. Callers MUST NOT mutate the returned slice.
//   - When dst is a caller-owned pre-sized buffer (e.g. buf[:0]), Receivers
//     appends matches into dst with 0 allocations even when tag filters run.
func (b *CompiledBus) Receivers(
	senderPath string,
	ns events.NamespaceID,
	et events.EventTypeID,
	dir events.Direction,
	msgTags []BusTagValue,
	dst []string,
) []string {
	if b == nil {
		return dst
	}
	senderIdx, ok := b.pathToIdx[senderPath]
	if !ok {
		return dst
	}
	entry, ok := b.deliveryIndex[busDeliveryKey{
		senderIdx: senderIdx,
		ns:        ns,
		et:        et,
		dir:       dir,
	}]
	if !ok {
		return dst
	}
	if entry.endpointOnly != nil {
		if dst == nil {
			return entry.endpointOnly
		}
		return append(dst, entry.endpointOnly...)
	}
	for i := range entry.receivers {
		r := &entry.receivers[i]
		if r.TagFilter != nil && !r.TagFilter.Matches(msgTags) {
			continue
		}
		if r.EndpointName != "" {
			dst = append(dst, r.EndpointName)
		} else {
			dst = append(dst, r.Path)
		}
	}
	return dst
}

// MaySend reports whether senderPath may send message to destPath (or on the
// bus in general when destPath is empty) with zero heap allocations and no
// path parsing on the hot path.
func (b *CompiledBus) MaySend(senderPath, message, destPath string) (bool, BusSendRefusalReason) {
	return b.MaySendWithTags(senderPath, message, destPath, nil)
}

// MaySendWithTags reports whether senderPath may send message carrying msgTags
// to destPath (or on the bus in general when destPath is empty). Allocates
// nothing on the hot path.
func (b *CompiledBus) MaySendWithTags(
	senderPath, message, destPath string,
	msgTags []BusTagValue,
) (bool, BusSendRefusalReason) {
	if b == nil {
		return false, SendRefusedUnknownSender
	}
	senderIdx, ok := b.pathToIdx[senderPath]
	if !ok {
		return false, SendRefusedUnknownSender
	}
	entry := b.lookupSendEntry(senderIdx, message)
	if entry == nil {
		return false, SendRefusedUndeclared
	}
	if entry.status != SendAllowed {
		return false, entry.status
	}
	if entry.outboundTags != nil && !entry.outboundTags.Matches(msgTags) {
		return false, SendRefusedOutboundTag
	}
	if destPath == "" {
		if entry.isCLI {
			return true, SendAllowed
		}
		if entry.toNone {
			return false, SendRefusedDestination
		}
		return true, SendAllowed
	}
	if entry.toNone {
		return false, SendRefusedDestination
	}
	destIdx, ok := b.pathToIdx[destPath]
	if !ok {
		return false, SendRefusedDestination
	}
	if _, permitted := entry.allowedDests[destIdx]; !permitted {
		return false, SendRefusedDestination
	}
	return true, SendAllowed
}

// AllowedDestinations returns the pre-resolved sorted canonical destination
// paths permitted for senderPath sending message. Allocates nothing; the
// returned slice is read-only.
func (b *CompiledBus) AllowedDestinations(senderPath, message string) []string {
	if b == nil {
		return nil
	}
	senderIdx, ok := b.pathToIdx[senderPath]
	// refuse-by-default: guards CompiledBus -- an unknown sender has no permitted destinations (fail-closed)
	if !ok {
		return nil
	}
	entry := b.lookupSendEntry(senderIdx, message)
	if entry == nil {
		return nil
	}
	if entry.status != SendAllowed {
		return nil
	}
	return entry.allowedPaths
}

// AllowedDestinationTargets returns the pre-resolved destination targets
// (canonical path, optional endpoint name, and secondary paths) permitted for
// senderPath sending message. Allocates nothing; the returned slice is
// read-only.
func (b *CompiledBus) AllowedDestinationTargets(senderPath, message string) []BusResolvedTarget {
	if b == nil {
		return nil
	}
	senderIdx, ok := b.pathToIdx[senderPath]
	// refuse-by-default: guards CompiledBus -- an unknown sender has no permitted destination targets (fail-closed)
	if !ok {
		return nil
	}
	entry := b.lookupSendEntry(senderIdx, message)
	if entry == nil {
		return nil
	}
	if entry.status != SendAllowed {
		return nil
	}
	return entry.allowedTargets
}

// ResolveAuthenticatedSender stamps the canonical bus sender path for a
// message arriving from authenticatedEndpoint.
//
// When requestedPath is empty or equals /bus/endpoint/<authenticatedEndpoint>,
// it returns that endpoint path with Spoofed=false. When requestedPath resolves
// (canonically or via a secondary path) to a participant whose OwnerEndpoint or
// EndpointName equals authenticatedEndpoint, it returns that participant's
// canonical path with Spoofed=false. Any other requestedPath is rejected and
// stamped back to /bus/endpoint/<authenticatedEndpoint> with Spoofed=true.
func (b *CompiledBus) ResolveAuthenticatedSender(authenticatedEndpoint, requestedPath string) (BusSenderStampResult, error) {
	if authenticatedEndpoint == "" {
		return BusSenderStampResult{}, opererr.New(
			"cannot stamp bus sender without an authenticated endpoint",
			"every bus message must carry a sender derived from an authenticated connection",
			"authenticate the endpoint connection before emitting messages")
	}
	ownPath := plugin.EndpointPath(authenticatedEndpoint)
	if requestedPath == "" {
		return BusSenderStampResult{StampedPath: ownPath}, nil
	}
	if requestedPath == ownPath {
		return BusSenderStampResult{StampedPath: ownPath}, nil
	}
	if b != nil {
		if idx, ok := b.pathToIdx[requestedPath]; ok {
			f := b.filters[idx]
			if f.OwnerEndpoint == authenticatedEndpoint {
				return BusSenderStampResult{StampedPath: f.Path}, nil
			}
			if f.EndpointName == authenticatedEndpoint {
				return BusSenderStampResult{StampedPath: f.Path}, nil
			}
		}
	}
	return BusSenderStampResult{
		StampedPath: ownPath,
		Spoofed:     true,
	}, nil
}

func (b *CompiledBus) lookupSendEntry(senderIdx uint32, message string) *compiledSendEntry {
	if entry, ok := b.sendIndex[busSendKey{senderIdx: senderIdx, message: message}]; ok {
		return entry
	}
	if !strings.HasPrefix(message, busCLIPrefix) {
		return nil
	}
	cliEntries := b.cliSends[senderIdx]
	for i := range cliEntries {
		if cliPrefixMatchesFold(cliEntries[i].prefix, message) {
			return cliEntries[i].entry
		}
	}
	return nil
}

// cliPrefixMatches reports whether prefix ("cli:show route") covers full
// ("cli:show route" or "cli:show route detail") at a word boundary. Allocates
// nothing.
func cliPrefixMatches(prefix, full string) bool {
	if prefix == full {
		return true
	}
	if len(full) <= len(prefix) {
		return false
	}
	if !strings.HasPrefix(full, prefix) {
		return false
	}
	return full[len(prefix)] == ' '
}

// --- Internal compilation helpers ---

type busDirRankLayer struct {
	block         plugin.BusDirectionBlock
	allowSources  []string
	targetSources []string
	msgTargetSrcs map[string][]string
	msgTagSrcs    map[string]map[string][]string
}

type busRankLayer struct {
	origin  BusEdgeOrigin
	source  string
	send    busDirRankLayer
	receive busDirRankLayer
}

type compiledParticipantState struct {
	idx         uint32
	input       *BusParticipantInput
	filter      *CompiledParticipantFilter
	sendDecls   map[string]*rpc.MessageSendDecl
	recvDecls   map[string]*rpc.MessageReceiveDecl
	sendAllow   resolvedAllowSet
	recvAllow   resolvedAllowSet
	sendRules   map[string]*BusCompiledSendRule
	recvRules   map[string]*BusCompiledReceiveRule
	dormantMsgs map[string]string // message -> reason when empty runtime tag overlap
}

type resolvedAllowSet struct {
	present  bool
	none     bool
	allowed  []string
	origin   BusRuleOrigin
	layerIdx int
}

type resolvedTargetSet struct {
	present bool
	none    bool
	paths   []string
	origin  BusRuleOrigin
}

type resolvedTagSet struct {
	values  map[string][]string
	origins map[string]BusRuleOrigin
}

func buildMessageTagCatalog(participants []BusParticipantInput) (map[string]map[string][]string, map[string]bool) {
	tagDefs := make(map[string]map[string][]string)
	hasSender := make(map[string]bool)
	// Engine-emitted lifecycle signals have no participant SendDecl, yet are
	// actively produced by the daemon (e.g. bus:participant-exit for bgp-watchdog).
	hasSender[events.NamespaceBus+":"+events.EventParticipantExit] = true

	for _, def := range events.AllMessages() {
		full := def.FullName()
		m := make(map[string][]string, len(def.Tags))
		for _, t := range def.Tags {
			m[t.Name] = slices.Clone(t.Values)
		}
		tagDefs[full] = m
	}

	for i := range participants {
		p := &participants[i]
		for j := range p.Sends {
			send := &p.Sends[j]
			hasSender[send.Message] = true
			m, ok := tagDefs[send.Message]
			if !ok {
				m = make(map[string][]string, len(send.Tags))
				tagDefs[send.Message] = m
			}
			for _, t := range send.Tags {
				if _, exists := m[t.Name]; !exists {
					m[t.Name] = slices.Clone(t.Values)
				}
			}
		}
	}
	return tagDefs, hasSender
}

func buildSelfRankLayers(
	p *BusParticipantInput,
	pathToIdx map[string]uint32,
	canonicalPaths []string,
) ([]busRankLayer, error) {
	ownSrc := p.OwnSource
	if ownSrc == "" {
		ownSrc = p.Path + " > bus"
	}
	busSrc := p.BusSideSource
	if busSrc == "" {
		busSrc = busParticipantSrcPrefix + p.Path
	}

	ownEmpty := p.OwnBlock.IsEmpty()
	busEmpty := p.BusSideBlock.IsEmpty()
	if ownEmpty && busEmpty {
		return nil, nil
	}

	if p.OwnOrigin == BusOriginInherited {
		// Bus-side block is written at this participant's rank and overrides an
		// inherited OwnBlock item-by-item without triggering Refusal 6.
		var layers []busRankLayer
		if !busEmpty {
			layers = append(layers, makeSingleRankLayer(BusOriginWritten, busSrc, &p.BusSideBlock))
		}
		if !ownEmpty {
			layers = append(layers, makeSingleRankLayer(BusOriginInherited, ownSrc, &p.OwnBlock))
		}
		return layers, nil
	}

	if ownEmpty {
		return []busRankLayer{makeSingleRankLayer(BusOriginWritten, busSrc, &p.BusSideBlock)}, nil
	}
	if busEmpty {
		return []busRankLayer{makeSingleRankLayer(BusOriginWritten, ownSrc, &p.OwnBlock)}, nil
	}

	// Same rank: check Refusal 6 between OwnBlock and BusSideBlock, using
	// canonicalized target equivalence so canonical and secondary spellings of
	// the same participant agree.
	if err := checkSameRankBlockConflict(ownSrc, &p.OwnBlock, busSrc, &p.BusSideBlock, pathToIdx, canonicalPaths); err != nil {
		return nil, err
	}
	merged := mergeSameRankLayer(ownSrc, &p.OwnBlock, busSrc, &p.BusSideBlock)
	return []busRankLayer{merged}, nil
}

func makeSingleRankLayer(origin BusEdgeOrigin, source string, block *plugin.BusBlock) busRankLayer {
	return busRankLayer{
		origin:  origin,
		source:  source,
		send:    makeDirRankLayer(source, &block.Send),
		receive: makeDirRankLayer(source, &block.Receive),
	}
}

func makeDirRankLayer(source string, dir *plugin.BusDirectionBlock) busDirRankLayer {
	out := busDirRankLayer{
		block: dir.Clone(),
	}
	if dir.AllowPresent {
		out.allowSources = []string{source}
	}
	if dir.TargetsPresent {
		out.targetSources = []string{source}
	}
	if len(dir.Messages) > 0 {
		out.msgTargetSrcs = make(map[string][]string, len(dir.Messages))
		out.msgTagSrcs = make(map[string]map[string][]string, len(dir.Messages))
		for msg, rule := range dir.Messages {
			if rule == nil {
				continue
			}
			if rule.TargetsPresent {
				out.msgTargetSrcs[msg] = []string{source}
			}
			if len(rule.Tags) > 0 {
				tagSrcs := make(map[string][]string, len(rule.Tags))
				for tagName := range rule.Tags {
					tagSrcs[tagName] = []string{source}
				}
				out.msgTagSrcs[msg] = tagSrcs
			}
		}
	}
	return out
}

func mergeSameRankLayer(srcA string, a *plugin.BusBlock, srcB string, b *plugin.BusBlock) busRankLayer {
	return busRankLayer{
		origin:  BusOriginWritten,
		source:  srcA,
		send:    mergeSameRankDirLayer(srcA, &a.Send, srcB, &b.Send),
		receive: mergeSameRankDirLayer(srcA, &a.Receive, srcB, &b.Receive),
	}
}

func mergeSameRankDirLayer(srcA string, a *plugin.BusDirectionBlock, srcB string, b *plugin.BusDirectionBlock) busDirRankLayer {
	out := makeDirRankLayer(srcA, a)
	if b.AllowPresent {
		if out.block.AllowPresent {
			out.allowSources = appendUniqueSource(out.allowSources, srcB)
		} else {
			out.block.AllowPresent = true
			out.block.AllowNone = b.AllowNone
			out.block.Allow = slices.Clone(b.Allow)
			out.allowSources = []string{srcB}
		}
	}
	if b.TargetsPresent {
		if out.block.TargetsPresent {
			out.targetSources = appendUniqueSource(out.targetSources, srcB)
		} else {
			out.block.TargetsPresent = true
			out.block.TargetsNone = b.TargetsNone
			out.block.Targets = slices.Clone(b.Targets)
			out.targetSources = []string{srcB}
		}
	}
	for msg, rb := range b.Messages {
		if rb == nil {
			continue
		}
		if out.block.Messages == nil {
			out.block.Messages = make(map[string]*plugin.BusMessageRule)
		}
		if out.msgTargetSrcs == nil {
			out.msgTargetSrcs = make(map[string][]string)
		}
		if out.msgTagSrcs == nil {
			out.msgTagSrcs = make(map[string]map[string][]string)
		}
		ra := out.block.Messages[msg]
		if ra == nil {
			out.block.Messages[msg] = rb.Clone()
			if rb.TargetsPresent {
				out.msgTargetSrcs[msg] = []string{srcB}
			}
			if len(rb.Tags) > 0 {
				tagSrcs := make(map[string][]string, len(rb.Tags))
				for tagName := range rb.Tags {
					tagSrcs[tagName] = []string{srcB}
				}
				out.msgTagSrcs[msg] = tagSrcs
			}
			continue
		}
		if rb.TargetsPresent {
			if ra.TargetsPresent {
				out.msgTargetSrcs[msg] = appendUniqueSource(out.msgTargetSrcs[msg], srcB)
			} else {
				ra.TargetsPresent = true
				ra.TargetsNone = rb.TargetsNone
				ra.Targets = slices.Clone(rb.Targets)
				out.msgTargetSrcs[msg] = []string{srcB}
			}
		}
		for tagName, vals := range rb.Tags {
			if ra.Tags == nil {
				ra.Tags = make(map[string][]string)
			}
			if out.msgTagSrcs[msg] == nil {
				out.msgTagSrcs[msg] = make(map[string][]string)
			}
			if _, exists := ra.Tags[tagName]; exists {
				out.msgTagSrcs[msg][tagName] = appendUniqueSource(out.msgTagSrcs[msg][tagName], srcB)
			} else {
				ra.Tags[tagName] = slices.Clone(vals)
				out.msgTagSrcs[msg][tagName] = []string{srcB}
			}
		}
	}
	return out
}

func appendUniqueSource(srcs []string, src string) []string {
	if src == "" {
		return srcs
	}
	if slices.Contains(srcs, src) {
		return srcs
	}
	return append(srcs, src)
}

func buildFullLayerStack(
	p *BusParticipantInput,
	participants []BusParticipantInput,
	selfLayers [][]busRankLayer,
	idx int,
	pathToIdx map[string]uint32,
) []busRankLayer {
	var stack []busRankLayer
	stack = append(stack, selfLayers[idx]...)
	if !p.ParentBusSideBlock.IsEmpty() {
		src := p.ParentBusSideSource
		if src == "" {
			src = busParticipantSrcPrefix + p.ParentPath
		}
		stack = append(stack, makeSingleRankLayer(BusOriginInherited, src, &p.ParentBusSideBlock))
	}

	seen := make(map[int]bool, 4)
	seen[idx] = true
	curParent := p.ParentPath
	for range maxParentInheritanceDepth {
		if curParent == "" {
			break
		}
		pIdx, ok := pathToIdx[curParent]
		if !ok {
			break
		}
		parentIndex := int(pIdx)
		if seen[parentIndex] {
			break
		}
		seen[parentIndex] = true
		for _, layer := range selfLayers[parentIndex] {
			inherited := layer
			inherited.origin = BusOriginInherited
			stack = append(stack, inherited)
		}
		curParent = participants[parentIndex].ParentPath
	}
	return stack
}

func checkSameRankBlockConflict(
	srcA string,
	a *plugin.BusBlock,
	srcB string,
	b *plugin.BusBlock,
	pathToIdx map[string]uint32,
	canonicalPaths []string,
) error {
	if err := checkSameRankDirConflict(srcA, busDirReceive, busKeyFrom, &a.Receive, srcB, &b.Receive, pathToIdx, canonicalPaths); err != nil {
		return err
	}
	return checkSameRankDirConflict(srcA, busDirSend, busKeyTo, &a.Send, srcB, &b.Send, pathToIdx, canonicalPaths)
}

func checkSameRankDirConflict(
	srcA, dirName, targetKey string,
	a *plugin.BusDirectionBlock,
	srcB string,
	b *plugin.BusDirectionBlock,
	pathToIdx map[string]uint32,
	canonicalPaths []string,
) error {
	if a.AllowPresent && b.AllowPresent && !plugin.BusAllowEqual(*a, *b) {
		lineA := srcA + busSepArrow + dirName + busSepArrow + busKeyAllow
		lineB := srcB + busSepArrow + dirName + busSepArrow + busKeyAllow
		return busRefusal6Error(lineA, formatBusListValue(a.AllowNone, a.Allow), lineB, formatBusListValue(b.AllowNone, b.Allow))
	}
	if a.TargetsPresent && b.TargetsPresent && !canonicalTargetsEqual(a.TargetsNone, a.Targets, b.TargetsNone, b.Targets, pathToIdx, canonicalPaths) {
		lineA := srcA + busSepArrow + dirName + busSepArrow + targetKey
		lineB := srcB + busSepArrow + dirName + busSepArrow + targetKey
		return busRefusal6Error(lineA, formatBusListValue(a.TargetsNone, a.Targets), lineB, formatBusListValue(b.TargetsNone, b.Targets))
	}
	for _, msg := range slices.Sorted(maps.Keys(a.Messages)) {
		ra := a.Messages[msg]
		rb, ok := b.Messages[msg]
		if !ok {
			continue
		}
		if ra == nil {
			continue
		}
		if rb == nil {
			continue
		}
		if ra.TargetsPresent && rb.TargetsPresent && !canonicalTargetsEqual(ra.TargetsNone, ra.Targets, rb.TargetsNone, rb.Targets, pathToIdx, canonicalPaths) {
			lineA := srcA + busSepArrow + dirName + busSepArrow + busLabelMsg + msg + busSepArrow + targetKey
			lineB := srcB + busSepArrow + dirName + busSepArrow + busLabelMsg + msg + busSepArrow + targetKey
			return busRefusal6Error(lineA, formatBusListValue(ra.TargetsNone, ra.Targets), lineB, formatBusListValue(rb.TargetsNone, rb.Targets))
		}
		for _, tagName := range slices.Sorted(maps.Keys(ra.Tags)) {
			va := ra.Tags[tagName]
			vb, hasB := rb.Tags[tagName]
			if !hasB {
				continue
			}
			if !plugin.StringSetEqual(va, vb) {
				lineA := srcA + busSepArrow + dirName + busSepArrow + busLabelMsg + msg + busSepArrow + busLabelTag + tagName
				lineB := srcB + busSepArrow + dirName + busSepArrow + busLabelMsg + msg + busSepArrow + busLabelTag + tagName
				return busRefusal6Error(lineA, formatBusListValue(false, va), lineB, formatBusListValue(false, vb))
			}
		}
	}
	return nil
}

func canonicalTargetsEqual(
	aNone bool,
	aPaths []string,
	bNone bool,
	bPaths []string,
	pathToIdx map[string]uint32,
	canonicalPaths []string,
) bool {
	if aNone != bNone {
		return false
	}
	ca := canonicalizePathSlice(aPaths, pathToIdx, canonicalPaths)
	cb := canonicalizePathSlice(bPaths, pathToIdx, canonicalPaths)
	return plugin.StringSetEqual(ca, cb)
}

func canonicalizePathSlice(paths []string, pathToIdx map[string]uint32, canonicalPaths []string) []string {
	if len(paths) == 0 {
		return nil
	}
	out := make([]string, len(paths))
	for i, p := range paths {
		if idx, ok := pathToIdx[p]; ok {
			out[i] = canonicalPaths[idx]
		} else {
			out[i] = p
		}
	}
	return out
}

func busRefusal6Error(lineA, valA, lineB, valB string) *opererr.Error {
	var problem, fix textbuf.Buffer
	problem.Str("conflicting bus rules for the same participant: ").
		Str(lineA).Str(" sets ").Str(valA).Str(", while ").
		Str(lineB).Str(" sets ").Str(valB)
	fix.Str("keep one value and remove or match the conflicting entry in ").
		Str(lineA).Str(" or ").Str(lineB)
	return opererr.New(
		problem.String(),
		"the same field for the same participant cannot be written in two places with different values",
		fix.String())
}

func formatBusListValue(isNone bool, vals []string) string {
	if isNone {
		return plugin.BusKeywordNone
	}
	var tb textbuf.Buffer
	return tb.Str("[ ").Join(vals, " ").Str(" ]").String()
}

func compileParticipantFilter(
	idx uint32,
	p *BusParticipantInput,
	layers []busRankLayer,
	isPureContainer bool,
	interner *TagInterner,
	msgTagDefs map[string]map[string][]string,
) (compiledParticipantState, error) {
	ownerEP := p.OwnerEndpoint
	if ownerEP == "" {
		ownerEP = p.EndpointName
	}

	filter := &CompiledParticipantFilter{
		Path:           p.Path,
		EndpointName:   p.EndpointName,
		OwnerEndpoint:  ownerEP,
		SecondaryPaths: slices.Clone(p.SecondaryPaths),
		ParentPath:     p.ParentPath,
		SendRules:      make(map[string]*BusCompiledSendRule, len(p.Sends)),
		ReceiveRules:   make(map[string]*BusCompiledReceiveRule, len(p.Receives)),
	}

	sendDecls := make(map[string]*rpc.MessageSendDecl, len(p.Sends))
	declaredSendNames := make([]string, 0, len(p.Sends))
	for i := range p.Sends {
		msg := p.Sends[i].Message
		if _, exists := sendDecls[msg]; !exists {
			sendDecls[msg] = &p.Sends[i]
			declaredSendNames = append(declaredSendNames, msg)
		}
	}
	slices.Sort(declaredSendNames)

	recvDecls := make(map[string]*rpc.MessageReceiveDecl, len(p.Receives))
	declaredRecvNames := make([]string, 0, len(p.Receives))
	for i := range p.Receives {
		msg := p.Receives[i].Message
		if _, exists := recvDecls[msg]; !exists {
			recvDecls[msg] = &p.Receives[i]
			declaredRecvNames = append(declaredRecvNames, msg)
		}
	}
	slices.Sort(declaredRecvNames)

	sendAllow := resolveAllowFromLayers(layers, true)
	recvAllow := resolveAllowFromLayers(layers, false)

	if !isPureContainer {
		if err := validateDirectionAllowAndMessages(p.Path, busDirSend, busKeyTo, sendAllow, layers, true, declaredSendNames, msgTagDefs); err != nil {
			return compiledParticipantState{}, err
		}
		if err := validateDirectionAllowAndMessages(p.Path, busDirReceive, busKeyFrom, recvAllow, layers, false, declaredRecvNames, msgTagDefs); err != nil {
			return compiledParticipantState{}, err
		}
	}

	// Summarize send allow-list and block-level send targets.
	filter.SendAllowOrigin = sendAllow.origin
	if sendAllow.present && sendAllow.none {
		filter.SendAllowNone = true
	} else if !sendAllow.present {
		filter.SendAllowAllDeclared = true
	}
	blockSendTo := resolveBlockTargetsFromLayers(layers, true)
	filter.SendToOrigin = blockSendTo.origin
	if blockSendTo.present {
		filter.SendToNone = blockSendTo.none
		filter.SendToPaths = slices.Clone(blockSendTo.paths)
	} else {
		filter.SendToAll = true
	}

	// Compile per-message send rules.
	var allowedSends []string
	var allowedCLI []string
	for _, msg := range declaredSendNames {
		decl := sendDecls[msg]
		allowed := sendMessageAdmitted(msg, sendAllow)
		if allowed {
			allowedSends = append(allowedSends, msg)
			if strings.HasPrefix(msg, busCLIPrefix) {
				allowedCLI = append(allowedCLI, strings.TrimPrefix(msg, busCLIPrefix))
			}
		}

		// Precedence: member per-message -> member block -> parent per-message
		// -> parent block -> registered default.
		toSet := resolveMessageTargetsFromLayers(layers, true, msg)
		rule := &BusCompiledSendRule{
			Message:     msg,
			Allowed:     allowed,
			AllowOrigin: sendAllow.origin,
		}
		if toSet.present {
			rule.ToNone = toSet.none
			rule.ToPaths = slices.Clone(toSet.paths)
			rule.ToOrigin = toSet.origin
		} else {
			rule.ToOrigin = BusRuleOrigin{Origin: BusOriginRegistered}
			if len(decl.DefaultDestinations) == 0 {
				rule.ToAll = true
			} else {
				rule.ToPaths = slices.Clone(decl.DefaultDestinations)
			}
		}

		outTags := resolveMessageTagsFromLayers(layers, true, msg)
		if len(outTags.values) > 0 {
			rule.OutboundTagMap = cloneTagValueMap(outTags.values)
			rule.TagOrigins = outTags.origins
			rule.OutboundTags = NewBusTagMatcher(interner, outTags.values)
		}
		filter.SendRules[msg] = rule
	}

	// If send > allow narrowed a declared cli: prefix to a more specific sub-prefix,
	// record the allowed sub-prefixes in AllowedSendMessages and AllowedCLIPrefixes.
	if sendAllow.present && !sendAllow.none {
		for _, entry := range sendAllow.allowed {
			if !strings.HasPrefix(entry, busCLIPrefix) {
				continue
			}
			if !slices.Contains(allowedSends, entry) {
				allowedSends = append(allowedSends, entry)
			}
			trimmed := strings.TrimPrefix(entry, busCLIPrefix)
			if !slices.Contains(allowedCLI, trimmed) {
				allowedCLI = append(allowedCLI, trimmed)
			}
		}
	}
	slices.Sort(allowedSends)
	slices.Sort(allowedCLI)
	filter.AllowedSendMessages = allowedSends
	filter.AllowedCLIPrefixes = allowedCLI

	// Summarize receive allow-list and block-level receive sources.
	filter.ReceiveAllowOrigin = recvAllow.origin
	if recvAllow.present && recvAllow.none {
		filter.ReceiveAllowNone = true
	} else if !recvAllow.present {
		filter.ReceiveAllowAllDeclared = true
	}
	blockRecvFrom := resolveBlockTargetsFromLayers(layers, false)
	filter.ReceiveFromOrigin = blockRecvFrom.origin
	if blockRecvFrom.present {
		filter.ReceiveFromNone = blockRecvFrom.none
		filter.ReceiveFromPaths = slices.Clone(blockRecvFrom.paths)
	} else {
		filter.ReceiveFromAll = true
	}

	// Compile per-message receive rules.
	var allowedRecvs []string
	dormantMsgs := make(map[string]string)
	for _, msg := range declaredRecvNames {
		decl := recvDecls[msg]
		allowed := receiveMessageAdmitted(msg, recvAllow)
		if allowed {
			allowedRecvs = append(allowedRecvs, msg)
		}

		fromSet := resolveMessageTargetsFromLayers(layers, false, msg)
		rule := &BusCompiledReceiveRule{
			Message:     msg,
			Allowed:     allowed,
			AllowOrigin: recvAllow.origin,
			Direction:   events.DirBoth,
		}
		if fromSet.present {
			rule.FromNone = fromSet.none
			rule.FromPaths = slices.Clone(fromSet.paths)
			rule.FromOrigin = fromSet.origin
			if len(decl.From) > 0 {
				rule.ReplacedDefaults = slices.Clone(decl.From)
			}
		} else {
			rule.FromOrigin = BusRuleOrigin{Origin: BusOriginRegistered}
			if len(decl.From) == 0 {
				rule.FromAll = true
			} else {
				rule.FromPaths = slices.Clone(decl.From)
			}
		}

		cfgTags := resolveMessageTagsFromLayers(layers, false, msg)
		effectiveTags, tagOrigins, err := intersectRegisteredAndConfiguredTags(p.Path, msg, decl.Tags, cfgTags)
		if err != nil {
			return compiledParticipantState{}, err
		}

		// Apply runtime tag narrowing (if any): an empty overlap marks the
		// reception dormant rather than refusing commit (AC-8).
		if rtTags := p.RuntimeReceiveTags[msg]; len(rtTags) > 0 {
			narrowed, dormantReason := intersectWithRuntimeTags(effectiveTags, rtTags)
			if dormantReason != "" {
				rule.Dormant = true
				dormantMsgs[msg] = dormantReason
			} else {
				effectiveTags = narrowed
			}
		}

		if len(effectiveTags) > 0 {
			rule.InboundTagMap = cloneTagValueMap(effectiveTags)
			rule.TagOrigins = tagOrigins
			dir, runtimeOnly := splitDirectionTag(effectiveTags)
			rule.Direction = dir
			if len(runtimeOnly) > 0 {
				rule.InboundTags = NewBusTagMatcher(interner, runtimeOnly)
			}
		}
		filter.ReceiveRules[msg] = rule
	}
	slices.Sort(allowedRecvs)
	filter.AllowedReceiveMessages = allowedRecvs

	return compiledParticipantState{
		idx:         idx,
		input:       p,
		filter:      filter,
		sendDecls:   sendDecls,
		recvDecls:   recvDecls,
		sendAllow:   sendAllow,
		recvAllow:   recvAllow,
		sendRules:   filter.SendRules,
		recvRules:   filter.ReceiveRules,
		dormantMsgs: dormantMsgs,
	}, nil
}

func resolveAllowFromLayers(layers []busRankLayer, isSend bool) resolvedAllowSet {
	for i := range layers {
		dl := &layers[i].receive
		if isSend {
			dl = &layers[i].send
		}
		if !dl.block.AllowPresent {
			continue
		}
		return resolvedAllowSet{
			present: true,
			none:    dl.block.AllowNone,
			allowed: slices.Clone(dl.block.Allow),
			origin: BusRuleOrigin{
				Origin:  layers[i].origin,
				Sources: slices.Clone(dl.allowSources),
			},
			layerIdx: i,
		}
	}
	return resolvedAllowSet{
		origin: BusRuleOrigin{Origin: BusOriginRegistered},
	}
}

func resolveBlockTargetsFromLayers(layers []busRankLayer, isSend bool) resolvedTargetSet {
	for i := range layers {
		dl := &layers[i].receive
		if isSend {
			dl = &layers[i].send
		}
		if !dl.block.TargetsPresent {
			continue
		}
		return resolvedTargetSet{
			present: true,
			none:    dl.block.TargetsNone,
			paths:   slices.Clone(dl.block.Targets),
			origin: BusRuleOrigin{
				Origin:  layers[i].origin,
				Sources: slices.Clone(dl.targetSources),
			},
		}
	}
	return resolvedTargetSet{
		origin: BusRuleOrigin{Origin: BusOriginRegistered},
	}
}

// resolveMessageTargetsFromLayers applies the exact bus target precedence for
// one message: member per-message -> member block -> parent per-message ->
// parent block -> registered default.
func resolveMessageTargetsFromLayers(layers []busRankLayer, isSend bool, msg string) resolvedTargetSet {
	for i := range layers {
		dl := &layers[i].receive
		if isSend {
			dl = &layers[i].send
		}
		if rule := dl.block.Messages[msg]; rule != nil && rule.TargetsPresent {
			return resolvedTargetSet{
				present: true,
				none:    rule.TargetsNone,
				paths:   slices.Clone(rule.Targets),
				origin: BusRuleOrigin{
					Origin:  layers[i].origin,
					Sources: slices.Clone(dl.msgTargetSrcs[msg]),
				},
			}
		}
		if dl.block.TargetsPresent {
			return resolvedTargetSet{
				present: true,
				none:    dl.block.TargetsNone,
				paths:   slices.Clone(dl.block.Targets),
				origin: BusRuleOrigin{
					Origin:  layers[i].origin,
					Sources: slices.Clone(dl.targetSources),
				},
			}
		}
	}
	return resolvedTargetSet{
		origin: BusRuleOrigin{Origin: BusOriginRegistered},
	}
}

func resolveMessageTagsFromLayers(layers []busRankLayer, isSend bool, msg string) resolvedTagSet {
	values := make(map[string][]string)
	origins := make(map[string]BusRuleOrigin)
	for i := range layers {
		dl := &layers[i].receive
		if isSend {
			dl = &layers[i].send
		}
		rule := dl.block.Messages[msg]
		if rule == nil {
			continue
		}
		for tagName, vals := range rule.Tags {
			if _, alreadySet := values[tagName]; alreadySet {
				continue
			}
			values[tagName] = uniqueSortedBusStrings(vals)
			var srcs []string
			if dl.msgTagSrcs != nil {
				srcs = slices.Clone(dl.msgTagSrcs[msg][tagName])
			}
			origins[tagName] = BusRuleOrigin{
				Origin:  layers[i].origin,
				Sources: srcs,
			}
		}
	}
	return resolvedTagSet{
		values:  values,
		origins: origins,
	}
}

func validateDirectionAllowAndMessages(
	participantPath, dirName, targetKey string,
	allow resolvedAllowSet,
	layers []busRankLayer,
	isSend bool,
	declared []string,
	msgTagDefs map[string]map[string][]string,
) error {
	// Refusal 4 on allow-list entries.
	if allow.present && !allow.none {
		for _, msg := range allow.allowed {
			if !messageDeclaredInDirection(msg, declared, isSend) {
				return busRefusal4Error(participantPath, dirName, busKeyAllow, msg, declared)
			}
		}
	}

	// Refusal 4, 5, 9, 10 on per-message rules across layers.
	seenMsgs := make(map[string]bool)
	seenMsgTags := make(map[string]map[string]bool)
	for i := range layers {
		dl := &layers[i].receive
		if isSend {
			dl = &layers[i].send
		}
		for _, msg := range slices.Sorted(maps.Keys(dl.block.Messages)) {
			rule := dl.block.Messages[msg]
			if rule == nil {
				continue
			}
			if !seenMsgs[msg] {
				seenMsgs[msg] = true
				if !messageDeclaredInDirection(msg, declared, isSend) {
					return busRefusal4Error(participantPath, dirName, busLabelMsg+msg, msg, declared)
				}
				// Refusal 5: per-message rule written at the same or more specific rank
				// for a message excluded by allow. An inherited parent per-message rule
				// for a message a child's narrower allow-list dropped is superseded,
				// not a contradiction.
				if allow.present {
					admitted := receiveMessageAdmitted(msg, allow)
					if isSend {
						admitted = sendMessageAdmitted(msg, allow)
					}
					if !admitted {
						if i > allow.layerIdx {
							continue
						}
						fieldKey := targetKey
						if !rule.TargetsPresent && len(rule.Tags) > 0 {
							fieldKey = "tag"
						}
						return busRefusal5Error(participantPath, dirName, fieldKey, msg, allow.origin.Sources)
					}
				}
			}
			// Refusals 9 & 10: tag name and closed value set checks on every
			// unoverridden tag across layers when the message's tag schema is known.
			if len(rule.Tags) > 0 {
				if tagSchema, known := msgTagDefs[msg]; known {
					if seenMsgTags[msg] == nil {
						seenMsgTags[msg] = make(map[string]bool, len(rule.Tags))
					}
					unseenTags := make(map[string][]string, len(rule.Tags))
					for tagName, vals := range rule.Tags {
						if seenMsgTags[msg][tagName] {
							continue
						}
						seenMsgTags[msg][tagName] = true
						unseenTags[tagName] = vals
					}
					if len(unseenTags) > 0 {
						if err := validateMessageTagRules(participantPath, dirName, msg, unseenTags, tagSchema); err != nil {
							return err
						}
					}
				}
			}
		}
	}
	return nil
}

func messageDeclaredInDirection(msg string, declared []string, isSend bool) bool {
	if slices.Contains(declared, msg) {
		return true
	}
	if !isSend {
		return false
	}
	if !strings.HasPrefix(msg, busCLIPrefix) {
		return false
	}
	for _, decl := range declared {
		if strings.HasPrefix(decl, busCLIPrefix) && cliPrefixMatches(decl, msg) {
			return true
		}
	}
	return false
}

func sendMessageAdmitted(msg string, allow resolvedAllowSet) bool {
	if !allow.present {
		return true
	}
	if allow.none {
		return false
	}
	if slices.Contains(allow.allowed, msg) {
		return true
	}
	if !strings.HasPrefix(msg, busCLIPrefix) {
		return false
	}
	for _, entry := range allow.allowed {
		if strings.HasPrefix(entry, busCLIPrefix) && cliPrefixMatches(entry, msg) {
			return true
		}
	}
	return false
}

func receiveMessageAdmitted(msg string, allow resolvedAllowSet) bool {
	if !allow.present {
		return true
	}
	if allow.none {
		return false
	}
	return slices.Contains(allow.allowed, msg)
}

func busRefusal4Error(participantPath, dirName, field, msg string, declared []string) *opererr.Error {
	var problem, why, fix textbuf.Buffer
	problem.Str(participantPath).Str(busSepArrow).Str(dirName).Str(busSepArrow).
		Str(field).Str(": message ").Quoted(msg).Str(" is not declared for ").Str(dirName)
	if len(declared) == 0 {
		why.Str(participantPath).Str(" declares no messages in direction ").Str(dirName)
		fix.Str("remove ").Quoted(msg).Str(" or use allow none")
	} else {
		why.Str(participantPath).Str(" declared ").Str(dirName).Str(" messages: ").Join(declared, ", ")
		fix.Str("choose from the declared messages: ").Join(declared, ", ")
	}
	return opererr.New(problem.String(), why.String(), fix.String())
}

func busRefusal5Error(participantPath, dirName, targetKey, msg string, allowSources []string) *opererr.Error {
	allowLine := participantPath + busSepArrow + dirName + busSepArrow + busKeyAllow
	if len(allowSources) > 0 {
		allowLine = allowSources[0] + busSepArrow + dirName + busSepArrow + busKeyAllow
	}
	msgLine := participantPath + busSepArrow + dirName + busSepArrow + busLabelMsg + msg + busSepArrow + targetKey
	var problem, fix textbuf.Buffer
	problem.Str("contradictory bus rule: ").Str(msgLine).
		Str(" configures message ").Quoted(msg).
		Str(", which is excluded by ").Str(allowLine)
	fix.Str("either add ").Quoted(msg).Str(" to ").Str(allowLine).
		Str(" or remove ").Str(msgLine)
	return opererr.New(
		problem.String(),
		"a per-message rule cannot reference a message that the same direction's allow-list excludes",
		fix.String())
}

func validateMessageTagRules(
	participantPath, dirName, msg string,
	configured, schema map[string][]string,
) error {
	declaredNames := slices.Sorted(maps.Keys(schema))
	for _, tagName := range slices.Sorted(maps.Keys(configured)) {
		closedVals, ok := schema[tagName]
		if !ok {
			var problem, why, fix textbuf.Buffer
			problem.Str(participantPath).Str(busSepArrow).Str(dirName).Str(busSepArrow).
				Str(busLabelMsg).Str(msg).Str(busSepArrow).Str(busLabelTag).Str(tagName).
				Str(": undeclared tag ").Quoted(tagName)
			if len(declaredNames) == 0 {
				why.Str("message ").Quoted(msg).Str(" declares no tags")
				fix.Str("remove tag ").Str(tagName)
			} else {
				why.Str("message ").Quoted(msg).Str(" declares tags: ").Join(declaredNames, ", ")
				fix.Str("use one of the declared tags: ").Join(declaredNames, ", ")
			}
			return opererr.New(problem.String(), why.String(), fix.String())
		}
		if len(closedVals) == 0 {
			continue
		}
		for _, val := range configured[tagName] {
			if !slices.Contains(closedVals, val) {
				var problem, why, fix textbuf.Buffer
				problem.Str(participantPath).Str(busSepArrow).Str(dirName).Str(busSepArrow).
					Str(busLabelMsg).Str(msg).Str(busSepArrow).Str(busLabelTag).Str(tagName).
					Str(": invalid value ").Quoted(val)
				why.Str("tag ").Quoted(tagName).Str(" on ").Quoted(msg).
					Str(" accepts only closed values: ").Join(closedVals, ", ")
				fix.Str("choose a value from: ").Join(closedVals, ", ")
				return opererr.New(problem.String(), why.String(), fix.String())
			}
		}
	}
	return nil
}

func intersectRegisteredAndConfiguredTags(
	participantPath, msg string,
	registered []rpc.MessageTagDecl,
	configured resolvedTagSet,
) (map[string][]string, map[string]BusRuleOrigin, error) {
	if len(registered) == 0 && len(configured.values) == 0 {
		return nil, nil, nil
	}
	effective := make(map[string][]string)
	origins := make(map[string]BusRuleOrigin)

	regByName := make(map[string][]string, len(registered))
	for i := range registered {
		t := &registered[i]
		if len(t.Values) == 0 {
			continue
		}
		regByName[t.Name] = uniqueSortedBusStrings(t.Values)
	}

	for _, tagName := range slices.Sorted(maps.Keys(regByName)) {
		regVals := regByName[tagName]
		cfgVals, hasCfg := configured.values[tagName]
		if !hasCfg {
			effective[tagName] = regVals
			origins[tagName] = BusRuleOrigin{Origin: BusOriginRegistered}
			continue
		}
		overlap := intersectSortedStrings(regVals, cfgVals)
		if len(overlap) == 0 {
			var problem, why, fix textbuf.Buffer
			problem.Str(participantPath).Str(busSepArrow).Str(busDirReceive).Str(busSepArrow).
				Str(busLabelMsg).Str(msg).Str(busSepArrow).Str(busLabelTag).Str(tagName).
				Str(": tag filter [ ").Join(cfgVals, " ").
				Str(" ] shares no value with registered filter [ ").Join(regVals, " ").Str(" ]")
			why.Str("an empty intersection between configured and registered tag filters would never admit any message")
			fix.Str("write a value admitted by the registered filter [ ").Join(regVals, " ").
				Str(" ] or remove tag ").Str(tagName)
			return nil, nil, opererr.New(problem.String(), why.String(), fix.String())
		}
		effective[tagName] = overlap
		origins[tagName] = configured.origins[tagName]
	}

	for _, tagName := range slices.Sorted(maps.Keys(configured.values)) {
		if _, exists := effective[tagName]; exists {
			continue
		}
		effective[tagName] = slices.Clone(configured.values[tagName])
		origins[tagName] = configured.origins[tagName]
	}

	return effective, origins, nil
}

func intersectWithRuntimeTags(effective, runtime map[string][]string) (map[string][]string, string) {
	out := cloneTagValueMap(effective)
	if out == nil {
		out = make(map[string][]string, len(runtime))
	}
	for _, tagName := range slices.Sorted(maps.Keys(runtime)) {
		rtVals := uniqueSortedBusStrings(runtime[tagName])
		if len(rtVals) == 0 {
			continue
		}
		effVals, hasEff := out[tagName]
		if !hasEff {
			out[tagName] = rtVals
			continue
		}
		overlap := intersectSortedStrings(effVals, rtVals)
		if len(overlap) == 0 {
			var tb textbuf.Buffer
			reason := tb.Str("tag ").Quoted(tagName).
				Str(" filter [ ").Join(effVals, " ").
				Str(" ] shares no value with runtime filter [ ").Join(rtVals, " ").Str(" ]").String()
			return nil, reason
		}
		out[tagName] = overlap
	}
	return out, ""
}

func splitDirectionTag(tags map[string][]string) (events.Direction, map[string][]string) {
	dir := events.DirBoth
	var runtimeOnly map[string][]string

	for tagName, vals := range tags {
		if tagName == events.TagDirection {
			dir = directionFromValues(vals)
			continue
		}
		if runtimeOnly == nil {
			runtimeOnly = make(map[string][]string, len(tags))
		}
		runtimeOnly[tagName] = slices.Clone(vals)
	}
	return dir, runtimeOnly
}

func directionFromValues(vals []string) events.Direction {
	if len(vals) == 0 {
		return events.DirBoth
	}
	var recv, sent bool
	for _, v := range vals {
		switch v {
		case events.DirectionReceived:
			recv = true
		case events.DirectionSent:
			sent = true
		case events.DirectionBoth:
			recv = true
			sent = true
		}
	}
	if recv && sent {
		return events.DirBoth
	}
	if recv {
		return events.DirReceived
	}
	if sent {
		return events.DirSent
	}
	return events.DirBoth
}

func populateReplacedSendDefaults(
	states []compiledParticipantState,
	participants []BusParticipantInput,
	pathToIdx map[string]uint32,
) {
	for i := range states {
		s := &states[i]
		for msg, rule := range s.sendRules {
			if rule.ToOrigin.Origin == BusOriginRegistered {
				continue
			}
			decl := s.sendDecls[msg]
			if decl != nil && len(decl.DefaultDestinations) > 0 {
				rule.ReplacedDefaults = slices.Clone(decl.DefaultDestinations)
				continue
			}
			// Broadcast default was overridden: record registered receivers of
			// msg that are no longer covered by the written to list.
			var replaced []string
			for j := range states {
				if i == j {
					continue
				}
				d := &states[j]
				if _, receives := d.recvDecls[msg]; !receives {
					continue
				}
				if !rule.ToNone && anyPathCoversParticipant(rule.ToPaths, &participants[j], uint32(j), pathToIdx) {
					continue
				}
				replaced = append(replaced, participants[j].Path)
			}
			slices.Sort(replaced)
			rule.ReplacedDefaults = replaced
		}
	}
}

func compileDeliveryAndSendIndices(
	bus *CompiledBus,
	states []compiledParticipantState,
	participants []BusParticipantInput,
	hasSenderForMsg map[string]bool,
) {
	for i := range states {
		s := &states[i]
		sInput := &participants[i]

		for _, msg := range slices.Sorted(maps.Keys(s.sendRules)) {
			sendRule := s.sendRules[msg]
			isCLI := strings.HasPrefix(msg, busCLIPrefix)
			entry := &compiledSendEntry{
				status:       SendAllowed,
				isCLI:        isCLI,
				toNone:       sendRule.ToNone,
				allowedDests: make(map[uint32]struct{}),
				outboundTags: sendRule.OutboundTags,
			}
			if !sendRule.Allowed {
				entry.status = SendRefusedAllowList
			}

			if isCLI {
				populateCLISendDestinations(entry, sendRule, participants, bus.pathToIdx, i)
				sendRule.ResolvedPaths = slices.Clone(entry.allowedPaths)
				bus.sendIndex[busSendKey{senderIdx: s.idx, message: msg}] = entry
				bus.cliSends[s.idx] = append(bus.cliSends[s.idx], compiledCLISend{
					prefix: msg,
					entry:  entry,
				})
				continue
			}

			nsID, etID, hasEventID := resolveMessageEventIDs(msg)
			for j := range states {
				if i == j {
					continue
				}
				d := &states[j]
				dInput := &participants[j]

				// Condition 4: D's kind declared receiving msg.
				recvRule := d.recvRules[msg]
				if recvRule == nil {
					continue
				}
				// Condition 5: D's receive allow-list admits msg.
				if !recvRule.Allowed {
					continue
				}
				// Skip receptions dormant due to empty runtime tag overlap.
				if recvRule.Dormant {
					continue
				}
				// Condition 3: S's to for msg covers D.
				if sendRule.ToNone {
					continue
				}
				if !sendRule.ToAll && !anyPathCoversParticipant(sendRule.ToPaths, dInput, d.idx, bus.pathToIdx) {
					continue
				}
				// Condition 6: D's from for msg covers S.
				if recvRule.FromNone {
					continue
				}
				if !recvRule.FromAll && !anyPathCoversParticipant(recvRule.FromPaths, sInput, s.idx, bus.pathToIdx) {
					continue
				}

				// Record destination in send index even when send allow refuses
				// (status gate refuses before dest lookup), and record resolved
				// targets & edges when Condition 2 passes.
				if !sendRule.Allowed {
					continue
				}

				var effectiveDir events.Direction
				if hasEventID {
					effectiveDir = intersectEdgeDirection(sendRule.OutboundTagMap, recvRule.Direction)
					// Disjoint outbound/inbound direction tags (e.g. sender only
					// sends received while receiver only accepts sent) produce no
					// active delivery edge.
					if recvRule.Direction != events.DirUnspecified && effectiveDir == events.DirUnspecified {
						continue
					}
				}

				entry.allowedDests[d.idx] = struct{}{}
				entry.allowedPaths = append(entry.allowedPaths, dInput.Path)
				entry.allowedTargets = append(entry.allowedTargets, BusResolvedTarget{
					Path:           dInput.Path,
					EndpointName:   dInput.EndpointName,
					SecondaryPaths: slices.Clone(dInput.SecondaryPaths),
				})
				sendRule.ResolvedPaths = append(sendRule.ResolvedPaths, dInput.Path)
				if dInput.EndpointName != "" {
					sendRule.ResolvedEndpoints = append(sendRule.ResolvedEndpoints, dInput.EndpointName)
				}
				recvRule.ResolvedPaths = append(recvRule.ResolvedPaths, sInput.Path)

				edgeOrigin := computeEdgeOrigin(sInput.Required || dInput.Required, sendRule, recvRule)
				bus.edges = append(bus.edges, BusCompiledEdge{
					SenderPath:       sInput.Path,
					ReceiverPath:     dInput.Path,
					ReceiverEndpoint: dInput.EndpointName,
					Message:          msg,
					Origin:           edgeOrigin,
					TagSummary:       formatTagSummary(recvRule.InboundTagMap),
					ParentPath:       sInput.ParentPath,
				})

				if hasEventID {
					rec := compiledReceiver{
						EndpointName: dInput.EndpointName,
						Path:         dInput.Path,
						TagFilter:    recvRule.InboundTags,
						Origin:       edgeOrigin,
					}
					indexDeliveryReceiver(bus.deliveryIndex, s.idx, nsID, etID, effectiveDir, rec)
				}
			}

			slices.Sort(entry.allowedPaths)
			slices.Sort(sendRule.ResolvedPaths)
			slices.Sort(sendRule.ResolvedEndpoints)
			bus.sendIndex[busSendKey{senderIdx: s.idx, message: msg}] = entry
		}

		// Handle narrower cli: allow-list sub-prefixes that narrow a declared
		// cli: prefix (e.g. declared "cli:show route", allow ["cli:show route detail"]).
		if s.sendAllow.present && !s.sendAllow.none {
			for _, allowedMsg := range s.sendAllow.allowed {
				if !strings.HasPrefix(allowedMsg, busCLIPrefix) {
					continue
				}
				if _, exists := bus.sendIndex[busSendKey{senderIdx: s.idx, message: allowedMsg}]; exists {
					continue
				}
				// Find the covering declared cli: prefix to copy its destination set.
				var baseRule *BusCompiledSendRule
				for declMsg, r := range s.sendRules {
					if strings.HasPrefix(declMsg, busCLIPrefix) && cliPrefixMatches(declMsg, allowedMsg) {
						baseRule = r
						break
					}
				}
				if baseRule == nil {
					continue
				}
				subEntry := &compiledSendEntry{
					status:       SendAllowed,
					isCLI:        true,
					toNone:       baseRule.ToNone,
					allowedDests: make(map[uint32]struct{}),
					outboundTags: baseRule.OutboundTags,
				}
				populateCLISendDestinations(subEntry, baseRule, participants, bus.pathToIdx, i)
				bus.sendIndex[busSendKey{senderIdx: s.idx, message: allowedMsg}] = subEntry
				bus.cliSends[s.idx] = append(bus.cliSends[s.idx], compiledCLISend{
					prefix: allowedMsg,
					entry:  subEntry,
				})
			}
		}

		// Sort cliSends longest-prefix-first so narrower allow-list entries win
		// over broader refused parent prefixes.
		if cliList := bus.cliSends[s.idx]; len(cliList) > 1 {
			slices.SortFunc(cliList, func(a, b compiledCLISend) int {
				if len(a.prefix) != len(b.prefix) {
					return len(b.prefix) - len(a.prefix)
				}
				return strings.Compare(a.prefix, b.prefix)
			})
		}
	}

	// Finalize fast-path endpointOnly slices for delivery entries where no
	// receiver carries a runtime TagFilter, and precompute the distinct tag
	// names and IDs tested by receivers that do carry a runtime TagFilter.
	for k, entry := range bus.deliveryIndex {
		hasFilter := false
		for i := range entry.receivers {
			if entry.receivers[i].TagFilter != nil {
				hasFilter = true
				break
			}
		}
		if !hasFilter {
			names := make([]string, len(entry.receivers))
			for i := range entry.receivers {
				if entry.receivers[i].EndpointName != "" {
					names[i] = entry.receivers[i].EndpointName
				} else {
					names[i] = entry.receivers[i].Path
				}
			}
			entry.endpointOnly = names
			bus.deliveryIndex[k] = entry
			continue
		}
		entry.filterTags = collectDeliveryFilterTags(bus.interner, entry.receivers)
		bus.deliveryIndex[k] = entry
	}
	compileInboundTagIndex(bus)

	// Collect dormant receptions (missing sender, empty runtime tag overlap,
	// or explicit from path with no matching active sender).
	for i := range states {
		d := &states[i]
		dInput := &participants[i]
		for _, msg := range slices.Sorted(maps.Keys(d.recvRules)) {
			recvRule := d.recvRules[msg]
			slices.Sort(recvRule.ResolvedPaths)
			if !recvRule.Allowed {
				continue
			}
			if recvRule.FromNone {
				continue
			}
			if reason, isRuntimeDormant := d.dormantMsgs[msg]; isRuntimeDormant {
				recordDormantReception(bus, d.filter, dInput, msg, "", reason)
				continue
			}
			if !hasSenderForMsg[msg] {
				recvRule.Dormant = true
				recordDormantReception(bus, d.filter, dInput, msg, "", "no sender registered for message")
				continue
			}
			if len(recvRule.ResolvedPaths) == 0 && !recvRule.FromAll && len(recvRule.FromPaths) > 0 {
				recvRule.Dormant = true
				senderHint := textbuf.Join(recvRule.FromPaths, ", ")
				recordDormantReception(bus, d.filter, dInput, msg, senderHint, "no matching sender currently active")
			}
		}
	}

	slices.SortFunc(bus.edges, func(a, b BusCompiledEdge) int {
		if a.SenderPath != b.SenderPath {
			return strings.Compare(a.SenderPath, b.SenderPath)
		}
		if a.ReceiverPath != b.ReceiverPath {
			return strings.Compare(a.ReceiverPath, b.ReceiverPath)
		}
		if a.Message != b.Message {
			return strings.Compare(a.Message, b.Message)
		}
		if a.Origin != b.Origin {
			if a.Origin < b.Origin {
				return -1
			}
			return 1
		}
		return strings.Compare(a.TagSummary, b.TagSummary)
	})
	bus.edges = slices.Compact(bus.edges)
}

func recordDormantReception(
	bus *CompiledBus,
	filter *CompiledParticipantFilter,
	dInput *BusParticipantInput,
	msg, senderHint, reason string,
) {
	filter.DormantReceptions = append(filter.DormantReceptions, BusDormantReception{
		ParticipantPath: dInput.Path,
		Message:         msg,
		Reason:          reason,
	})
	bus.edges = append(bus.edges, BusCompiledEdge{
		SenderPath:       senderHint,
		ReceiverPath:     dInput.Path,
		ReceiverEndpoint: dInput.EndpointName,
		Message:          msg,
		Origin:           BusOriginDormant,
		TagSummary:       reason,
	})
}

func populateCLISendDestinations(
	entry *compiledSendEntry,
	sendRule *BusCompiledSendRule,
	participants []BusParticipantInput,
	pathToIdx map[string]uint32,
	senderIdx int,
) {
	if sendRule.ToNone {
		return
	}
	for j := range participants {
		if j == senderIdx {
			continue
		}
		dInput := &participants[j]
		if !sendRule.ToAll && !anyPathCoversParticipant(sendRule.ToPaths, dInput, uint32(j), pathToIdx) {
			continue
		}
		entry.allowedDests[uint32(j)] = struct{}{}
		entry.allowedPaths = append(entry.allowedPaths, dInput.Path)
		entry.allowedTargets = append(entry.allowedTargets, BusResolvedTarget{
			Path:           dInput.Path,
			EndpointName:   dInput.EndpointName,
			SecondaryPaths: slices.Clone(dInput.SecondaryPaths),
		})
	}
	slices.Sort(entry.allowedPaths)
}

func resolveMessageEventIDs(msg string) (events.NamespaceID, events.EventTypeID, bool) {
	ns, name, ok := events.SplitMessageName(msg)
	if !ok {
		return events.NamespaceUnknown, events.EventTypeUnknown, false
	}
	nsID := events.LookupNamespaceID(ns)
	if nsID == events.NamespaceUnknown {
		return events.NamespaceUnknown, events.EventTypeUnknown, false
	}
	etID := events.LookupEventTypeID(name)
	if etID == events.EventTypeUnknown {
		return events.NamespaceUnknown, events.EventTypeUnknown, false
	}
	return nsID, etID, true
}

func indexDeliveryReceiver(
	index map[busDeliveryKey]busDeliveryEntry,
	senderIdx uint32,
	ns events.NamespaceID,
	et events.EventTypeID,
	dir events.Direction,
	rec compiledReceiver,
) {
	appendDeliveryReceiver(index, busDeliveryKey{senderIdx: senderIdx, ns: ns, et: et, dir: events.DirUnspecified}, rec)
	if dir == events.DirReceived || dir == events.DirBoth {
		appendDeliveryReceiver(index, busDeliveryKey{senderIdx: senderIdx, ns: ns, et: et, dir: events.DirReceived}, rec)
	}
	if dir == events.DirSent || dir == events.DirBoth {
		appendDeliveryReceiver(index, busDeliveryKey{senderIdx: senderIdx, ns: ns, et: et, dir: events.DirSent}, rec)
	}
	if dir == events.DirBoth {
		appendDeliveryReceiver(index, busDeliveryKey{senderIdx: senderIdx, ns: ns, et: et, dir: events.DirBoth}, rec)
	}
}

func intersectEdgeDirection(outboundTags map[string][]string, recvDir events.Direction) events.Direction {
	vals := outboundTags[events.TagDirection]
	if len(vals) == 0 {
		return recvDir
	}
	var sendDir events.Direction
	for _, v := range vals {
		switch v {
		case events.DirectionReceived:
			sendDir |= events.DirReceived
		case events.DirectionSent:
			sendDir |= events.DirSent
		}
	}
	if sendDir == events.DirUnspecified {
		return recvDir
	}
	if recvDir == events.DirUnspecified {
		return sendDir
	}
	return sendDir & recvDir
}

func appendDeliveryReceiver(index map[busDeliveryKey]busDeliveryEntry, key busDeliveryKey, rec compiledReceiver) {
	entry := index[key]
	entry.receivers = append(entry.receivers, rec)
	index[key] = entry
}

func anyPathCoversParticipant(
	patterns []string,
	target *BusParticipantInput,
	targetIdx uint32,
	pathToIdx map[string]uint32,
) bool {
	for _, pat := range patterns {
		if pathCoversParticipant(pat, target, targetIdx, pathToIdx) {
			return true
		}
	}
	return false
}

func pathCoversParticipant(
	pat string,
	target *BusParticipantInput,
	targetIdx uint32,
	pathToIdx map[string]uint32,
) bool {
	if resolvedIdx, ok := pathToIdx[pat]; ok && resolvedIdx == targetIdx {
		return true
	}
	if plugin.PathCovers(pat, target.Path) {
		return true
	}
	for _, sp := range target.SecondaryPaths {
		if plugin.PathCovers(pat, sp) {
			return true
		}
	}
	if target.ParentPath != "" && plugin.PathCovers(pat, target.ParentPath) {
		return true
	}
	return false
}

func computeEdgeOrigin(required bool, sendRule *BusCompiledSendRule, recvRule *BusCompiledReceiveRule) BusEdgeOrigin {
	if required {
		return BusOriginRequired
	}
	if ruleOriginIs(BusOriginWritten, sendRule, recvRule) {
		return BusOriginWritten
	}
	if ruleOriginIs(BusOriginInherited, sendRule, recvRule) {
		return BusOriginInherited
	}
	return BusOriginRegistered
}

func ruleOriginIs(want BusEdgeOrigin, sendRule *BusCompiledSendRule, recvRule *BusCompiledReceiveRule) bool {
	if sendRule.AllowOrigin.Origin == want {
		return true
	}
	if sendRule.ToOrigin.Origin == want {
		return true
	}
	if recvRule.AllowOrigin.Origin == want {
		return true
	}
	if recvRule.FromOrigin.Origin == want {
		return true
	}
	for _, orig := range sendRule.TagOrigins {
		if orig.Origin == want {
			return true
		}
	}
	for _, orig := range recvRule.TagOrigins {
		if orig.Origin == want {
			return true
		}
	}
	return false
}

func formatTagSummary(tags map[string][]string) string {
	if len(tags) == 0 {
		return ""
	}
	var tb textbuf.Buffer
	first := true
	for _, tagName := range slices.Sorted(maps.Keys(tags)) {
		if !first {
			tb.Str(", ")
		}
		first = false
		tb.Str(tagName).Str("=[").Join(tags[tagName], " ").Byte(']')
	}
	return tb.String()
}

func uniqueSortedBusStrings(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := slices.Clone(in)
	slices.Sort(out)
	return slices.Compact(out)
}

func intersectSortedStrings(a, b []string) []string {
	var out []string
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		switch {
		case a[i] < b[j]:
			i++
		case a[i] > b[j]:
			j++
		default:
			out = append(out, a[i])
			i++
			j++
		}
	}
	return out
}

func cloneTagValueMap(in map[string][]string) map[string][]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string][]string, len(in))
	for k, v := range in {
		out[k] = slices.Clone(v)
	}
	return out
}
