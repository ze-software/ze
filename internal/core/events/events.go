// Design: docs/architecture/api/process-protocol.md -- event bus namespace and type registry
// Related: ids.go -- typed NamespaceID/EventTypeID/Direction populated by registration here
// Related: typed.go -- Event[T]/SignalEvent generic handles that call RegisterNamespace
//
// Package events defines the event namespace and type constants used by ze's
// event bus, plus the runtime registry for namespace/event validation.
// Domain-specific event types are defined in separate files (bgp.go,
// interface.go, sysctl.go, etc.) so ownership is clear at the file level.
// Each domain file declares its constants and valid-event map; the aggregate
// ValidEvents map is assembled here from those per-domain maps.
package events

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/ze-software/ze/internal/core/textbuf"
)

var (
	errNamespaceMustNotBeEmpty     = errors.New("namespace must not be empty")
	errNamespaceIdOverflowMax255   = errors.New("namespace ID overflow (max 255)")
	errEventTypeIdOverflowMax65535 = errors.New("event type ID overflow (max 65535)")
	errEventTypeMustNotBeEmpty     = errors.New("event type must not be empty")
	errMessageNameInvalid          = errors.New("message name must be namespace:name with no control characters")
)

// Direction constants for event filtering.
const (
	DirectionReceived = "received"
	DirectionSent     = "sent"
	DirectionBoth     = "both"
)

// NamespaceCLI is the namespace for command-path prefix messages (e.g.
// "cli:show bgp peer"), where single spaces separate command words after the
// namespace separator.
const NamespaceCLI = "cli"

// NamespaceBus is the namespace for generic bus participant lifecycle messages.
const NamespaceBus = "bus"

// EventParticipantExit is emitted when a bus participant exits without
// keep-on-exit policy, so subscribers holding per-participant state can
// release it.
const EventParticipantExit = "participant-exit"

// MessageParticipantExit is the canonical "namespace:name" identifier for
// participant-exit lifecycle notifications.
const MessageParticipantExit = NamespaceBus + ":" + EventParticipantExit

// ParticipantExitEvent carries the identity of a bus participant that exited.
type ParticipantExitEvent struct {
	Type        string `json:"type"`
	Endpoint    string `json:"endpoint"`
	Participant string `json:"participant"`
}

// BusParticipantExit is the typed handle for bus:participant-exit lifecycle
// notifications. Initialized in register.go after package-level maps are ready.
var BusParticipantExit *Event[ParticipantExitEvent]

// TagDirection is the canonical tag name for bidirectional protocol messages.
const TagDirection = "direction"

// Bounds on registered bus message metadata. Every input arriving from
// registrations or wire declarations is checked against these limits.
const (
	MaxMessageNamespaceLen        = 64
	MaxMessageNameLen             = 192
	MaxMessageFullLen             = 256
	MaxMessageTagNameLen          = 64
	MaxMessageTagValueLen         = 128
	MaxMessageTags                = 32
	MaxMessageTagValues           = 64
	MaxMessageDefaultDestinations = 64
)

// TagDef describes one tag a bus message carries.
// An empty Values slice denotes an open tag (any valid value token is accepted);
// a non-empty Values slice names the closed set of allowed values.
type TagDef struct {
	Name   string
	Values []string
}

// MessageDef describes one registered bus message kind, canonically identified
// as Namespace + ":" + Name.
type MessageDef struct {
	Namespace           string
	Name                string
	Tags                []TagDef
	DefaultDestinations []string
}

// FullName returns the canonical "namespace:name" identifier for m.
func (m MessageDef) FullName() string {
	var tb textbuf.Buffer
	return tb.Str(m.Namespace).Byte(':').Str(m.Name).String()
}

// DirectionTagDef returns the closed TagDef for bidirectional messages
// ("received" and "sent").
func DirectionTagDef() TagDef {
	return TagDef{
		Name:   TagDirection,
		Values: []string{DirectionReceived, DirectionSent},
	}
}

// eventsMu protects ValidEvents from concurrent read/write.
// Writes happen during RegisterEventType and RegisterNamespace (startup).
// Reads happen during subscribe-events and emit-event validation (runtime).
var eventsMu sync.RWMutex

// ValidEvents maps namespace to its set of valid event types.
// This is the single source of truth for namespace/event validation.
// Populated dynamically by components calling RegisterNamespace from init().
// Protected by eventsMu for concurrent access.
var ValidEvents = map[string]map[string]bool{}

// ID lookup maps below are protected by eventsMu (same as ValidEvents).
// All writes happen at init() time via RegisterNamespace/RegisterEventType;
// at runtime both ID lookups and ValidEvents reads share the RLock with
// no writer contention.
var (
	namespaceIDs               = map[string]NamespaceID{}
	namespaceNames             = map[NamespaceID]string{}
	nextNS         NamespaceID = 1

	eventTypeIDs               = map[string]EventTypeID{}
	eventTypeNames             = map[EventTypeID]string{}
	nextET         EventTypeID = 1
)

// IsValidEvent returns true if the event type is valid in the given namespace.
// Safe for concurrent use.
func IsValidEvent(namespace, eventType string) bool {
	eventsMu.RLock()
	defer eventsMu.RUnlock()
	evts, ok := ValidEvents[namespace]
	if !ok {
		return false
	}
	return evts[eventType]
}

// IsValidNamespace returns true if the namespace exists. Safe for concurrent use.
func IsValidNamespace(namespace string) bool {
	eventsMu.RLock()
	defer eventsMu.RUnlock()
	_, ok := ValidEvents[namespace]
	return ok
}

// ValidEventNames returns a sorted, comma-separated list of valid event types
// for the given namespace. Used in error messages so the list stays in sync
// with the registration maps. Safe for concurrent use.
func ValidEventNames(namespace string) string {
	eventsMu.RLock()
	defer eventsMu.RUnlock()
	m := ValidEvents[namespace]
	if len(m) == 0 {
		return ""
	}
	names := make([]string, 0, len(m))
	for k := range m {
		names = append(names, k)
	}
	slices.Sort(names)
	return textbuf.Join(names, ", ")
}

// ValidNamespaceNames returns a sorted, comma-separated list of valid namespaces.
// Safe for concurrent use.
func ValidNamespaceNames() string {
	eventsMu.RLock()
	defer eventsMu.RUnlock()
	names := make([]string, 0, len(ValidEvents))
	for k := range ValidEvents {
		names = append(names, k)
	}
	slices.Sort(names)
	return textbuf.Join(names, ", ")
}

// IsValidEventAnyNamespace returns true if the event type is valid in any namespace.
// Safe for concurrent use.
func IsValidEventAnyNamespace(eventType string) bool {
	eventsMu.RLock()
	defer eventsMu.RUnlock()
	for _, evts := range ValidEvents {
		if evts[eventType] {
			return true
		}
	}
	return false
}

// AllEventTypes returns all valid event types grouped by namespace.
// Safe for concurrent use.
func AllEventTypes() map[string][]string {
	eventsMu.RLock()
	defer eventsMu.RUnlock()
	result := make(map[string][]string, len(ValidEvents))
	for ns, evts := range ValidEvents {
		types := make([]string, 0, len(evts))
		for et := range evts {
			types = append(types, et)
		}
		result[ns] = types
	}
	return result
}

// allValidEventNames returns a sorted, comma-separated list of valid event types
// across all namespaces (deduped). Safe for concurrent use.
func allValidEventNames() string {
	eventsMu.RLock()
	defer eventsMu.RUnlock()
	seen := make(map[string]bool)
	for _, evts := range ValidEvents {
		for k := range evts {
			seen[k] = true
		}
	}
	names := make([]string, 0, len(seen))
	for k := range seen {
		names = append(names, k)
	}
	slices.Sort(names)
	return textbuf.Join(names, ", ")
}

// RegisterNamespace adds a new namespace with the given initial event types.
// Idempotent: if the namespace already exists, the event types are merged
// into the existing set. This allows multiple init() functions to contribute
// events to the same namespace without depending on import order.
// Safe for concurrent use.
func RegisterNamespace(namespace string, eventTypes ...string) error {
	if namespace == "" {
		return errNamespaceMustNotBeEmpty
	}
	eventsMu.Lock()
	defer eventsMu.Unlock()
	m, exists := ValidEvents[namespace]
	if !exists {
		m = make(map[string]bool, len(eventTypes))
		ValidEvents[namespace] = m
	}
	for _, e := range eventTypes {
		m[e] = true
	}

	// ID assignment under the same eventsMu already held above.
	if _, ok := namespaceIDs[namespace]; !ok {
		if nextNS == 0 {
			return errNamespaceIdOverflowMax255
		}
		namespaceIDs[namespace] = nextNS
		namespaceNames[nextNS] = namespace
		nextNS++
	}
	for _, e := range eventTypes {
		if _, ok := eventTypeIDs[e]; !ok {
			if nextET == 0 {
				return errEventTypeIdOverflowMax65535
			}
			eventTypeIDs[e] = nextET
			eventTypeNames[nextET] = e
			nextET++
		}
	}
	return nil
}

// RegisterEventType adds a custom event type to the given namespace.
// Plugins call this to register event types they produce (e.g., "update-rpki").
// Duplicate registration is idempotent. The namespace must already exist.
// Event type names must be non-empty and contain no whitespace.
// Safe for concurrent use.
func RegisterEventType(namespace, eventType string) error {
	if eventType == "" {
		return errEventTypeMustNotBeEmpty
	}
	if strings.ContainsAny(eventType, " \t\n\r") {
		return fmt.Errorf("event type %q must not contain whitespace", eventType)
	}
	eventsMu.Lock()
	defer eventsMu.Unlock()
	evts, ok := ValidEvents[namespace]
	if !ok {
		return fmt.Errorf("unknown namespace: %s (valid: %s)", namespace, validNamespaceNamesLocked())
	}
	evts[eventType] = true

	// ID assignment under the same eventsMu already held above.
	if _, ok := eventTypeIDs[eventType]; !ok {
		if nextET == 0 {
			return errEventTypeIdOverflowMax65535
		}
		eventTypeIDs[eventType] = nextET
		eventTypeNames[nextET] = eventType
		nextET++
	}
	return nil
}

// validNamespaceNamesLocked returns namespace names. Caller MUST hold eventsMu.
func validNamespaceNamesLocked() string {
	names := make([]string, 0, len(ValidEvents))
	for k := range ValidEvents {
		names = append(names, k)
	}
	slices.Sort(names)
	return textbuf.Join(names, ", ")
}

// registeredMessages maps canonical "namespace:name" keys to their registered
// MessageDef metadata. Protected by eventsMu alongside ValidEvents.
var registeredMessages = map[string]MessageDef{}

// SplitMessageName splits a canonical "namespace:name" bus message identifier
// on the first ':'. For the "cli" namespace, single spaces between command
// words are permitted (e.g. "cli:show bgp peer"); for all other namespaces,
// whitespace is rejected. Control characters, empty parts, and values past
// their bounds return ok=false.
func SplitMessageName(full string) (namespace, name string, ok bool) {
	if full == "" {
		return "", "", false
	}
	if len(full) > MaxMessageFullLen {
		return "", "", false
	}
	ns, rest, found := strings.Cut(full, ":")
	if !found {
		return "", "", false
	}
	if !validMessageNamespace(ns) {
		return "", "", false
	}
	if !validMessagePartName(ns, rest) {
		return "", "", false
	}
	return ns, rest, true
}

func validMessageNamespace(ns string) bool {
	if ns == "" {
		return false
	}
	if len(ns) > MaxMessageNamespaceLen {
		return false
	}
	for i := range len(ns) {
		c := ns[i]
		if c <= 0x20 {
			return false
		}
		if c == 0x7f {
			return false
		}
		if c == ':' {
			return false
		}
	}
	return true
}

func validMessagePartName(ns, name string) bool {
	if name == "" {
		return false
	}
	if len(name) > MaxMessageNameLen {
		return false
	}
	if ns == NamespaceCLI {
		if strings.TrimSpace(name) != name {
			return false
		}
		for i := range len(name) {
			c := name[i]
			if c < 0x20 {
				return false
			}
			if c == 0x7f {
				return false
			}
		}
		return true
	}
	for i := range len(name) {
		c := name[i]
		if c <= 0x20 {
			return false
		}
		if c == 0x7f {
			return false
		}
	}
	return true
}

func validMessageToken(token string, maxLen int) bool {
	if token == "" {
		return false
	}
	if len(token) > maxLen {
		return false
	}
	for i := range len(token) {
		c := token[i]
		if c <= 0x20 {
			return false
		}
		if c == 0x7f {
			return false
		}
	}
	return true
}

func validateMessageDef(def MessageDef) error {
	if !validMessageNamespace(def.Namespace) {
		return fmt.Errorf("%w: invalid namespace %q", errMessageNameInvalid, def.Namespace)
	}
	if !validMessagePartName(def.Namespace, def.Name) {
		return fmt.Errorf("%w: invalid name %q in namespace %q", errMessageNameInvalid, def.Name, def.Namespace)
	}
	if len(def.Namespace)+1+len(def.Name) > MaxMessageFullLen {
		return fmt.Errorf("%w: full name exceeds %d bytes", errMessageNameInvalid, MaxMessageFullLen)
	}
	if len(def.Tags) > MaxMessageTags {
		return fmt.Errorf("message %s:%s declares %d tags (max %d)", def.Namespace, def.Name, len(def.Tags), MaxMessageTags)
	}
	seenTags := make(map[string]bool, len(def.Tags))
	for _, tag := range def.Tags {
		if !validMessageToken(tag.Name, MaxMessageTagNameLen) {
			return fmt.Errorf("message %s:%s has invalid tag name %q", def.Namespace, def.Name, tag.Name)
		}
		if seenTags[tag.Name] {
			return fmt.Errorf("message %s:%s declares duplicate tag %q", def.Namespace, def.Name, tag.Name)
		}
		seenTags[tag.Name] = true
		if len(tag.Values) > MaxMessageTagValues {
			return fmt.Errorf("message %s:%s tag %q declares %d values (max %d)", def.Namespace, def.Name, tag.Name, len(tag.Values), MaxMessageTagValues)
		}
		seenVals := make(map[string]bool, len(tag.Values))
		for _, v := range tag.Values {
			if !validMessageToken(v, MaxMessageTagValueLen) {
				return fmt.Errorf("message %s:%s tag %q has invalid value %q", def.Namespace, def.Name, tag.Name, v)
			}
			if seenVals[v] {
				return fmt.Errorf("message %s:%s tag %q declares duplicate value %q", def.Namespace, def.Name, tag.Name, v)
			}
			seenVals[v] = true
		}
	}
	if len(def.DefaultDestinations) > MaxMessageDefaultDestinations {
		return fmt.Errorf("message %s:%s declares %d default destinations (max %d)", def.Namespace, def.Name, len(def.DefaultDestinations), MaxMessageDefaultDestinations)
	}
	for _, dst := range def.DefaultDestinations {
		if !validMessageToken(dst, MaxMessageFullLen) {
			return fmt.Errorf("message %s:%s has invalid default destination %q", def.Namespace, def.Name, dst)
		}
	}
	return nil
}

func cloneMessageDef(def MessageDef) MessageDef {
	out := MessageDef{
		Namespace:           def.Namespace,
		Name:                def.Name,
		DefaultDestinations: slices.Clone(def.DefaultDestinations),
	}
	if len(def.Tags) > 0 {
		out.Tags = make([]TagDef, len(def.Tags))
		for i, t := range def.Tags {
			out.Tags[i] = TagDef{
				Name:   t.Name,
				Values: slices.Clone(t.Values),
			}
		}
	}
	return out
}

func messageDefsEqual(a, b MessageDef) bool {
	if a.Namespace != b.Namespace {
		return false
	}
	if a.Name != b.Name {
		return false
	}
	if !slices.Equal(a.DefaultDestinations, b.DefaultDestinations) {
		return false
	}
	if len(a.Tags) != len(b.Tags) {
		return false
	}
	for i := range a.Tags {
		if a.Tags[i].Name != b.Tags[i].Name {
			return false
		}
		if !slices.Equal(a.Tags[i].Values, b.Tags[i].Values) {
			return false
		}
	}
	return true
}

// RegisterMessage registers a bus message with optional tag definitions in the
// unified message registry, populates ValidEvents, and assigns interned
// NamespaceID and EventTypeID values. Re-registering an identical definition is
// idempotent; re-registering with a conflicting tag set or destination list
// returns an error. Safe for concurrent use.
func RegisterMessage(namespace, name string, tags ...TagDef) error {
	return RegisterMessageDef(MessageDef{
		Namespace: namespace,
		Name:      name,
		Tags:      tags,
	})
}

// RegisterMessageDef registers a complete MessageDef in the unified message
// registry, populates ValidEvents, and assigns interned NamespaceID and
// EventTypeID values. Re-registering an identical definition is idempotent;
// conflicting duplicate definitions return an error. Safe for concurrent use.
func RegisterMessageDef(def MessageDef) error {
	if err := validateMessageDef(def); err != nil {
		return err
	}
	cloned := cloneMessageDef(def)
	full := cloned.FullName()

	eventsMu.Lock()
	defer eventsMu.Unlock()

	if existing, exists := registeredMessages[full]; exists {
		if !messageDefsEqual(existing, cloned) {
			return fmt.Errorf("conflicting duplicate message registration for %q", full)
		}
		return nil
	}

	if _, ok := namespaceIDs[cloned.Namespace]; !ok {
		if nextNS == 0 {
			return errNamespaceIdOverflowMax255
		}
		namespaceIDs[cloned.Namespace] = nextNS
		namespaceNames[nextNS] = cloned.Namespace
		nextNS++
	}
	if _, ok := eventTypeIDs[cloned.Name]; !ok {
		if nextET == 0 {
			return errEventTypeIdOverflowMax65535
		}
		eventTypeIDs[cloned.Name] = nextET
		eventTypeNames[nextET] = cloned.Name
		nextET++
	}

	evts, exists := ValidEvents[cloned.Namespace]
	if !exists {
		evts = make(map[string]bool)
		ValidEvents[cloned.Namespace] = evts
	}
	evts[cloned.Name] = true
	registeredMessages[full] = cloned
	return nil
}

// IsValidMessage reports whether full ("namespace:name") is a registered bus
// message (either registered via RegisterMessage/RegisterMessageDef or via
// RegisterNamespace/RegisterEventType). Safe for concurrent use.
func IsValidMessage(full string) bool {
	ns, name, ok := SplitMessageName(full)
	if !ok {
		return false
	}
	eventsMu.RLock()
	defer eventsMu.RUnlock()
	if _, exists := registeredMessages[full]; exists {
		return true
	}
	evts := ValidEvents[ns]
	return evts[name]
}

// LookupMessage returns a defensive copy of the MessageDef for full
// ("namespace:name"), or ok=false if not registered. Messages registered only
// via RegisterNamespace/RegisterEventType synthesize a tagless MessageDef.
// Safe for concurrent use.
func LookupMessage(full string) (MessageDef, bool) {
	ns, name, ok := SplitMessageName(full)
	if !ok {
		return MessageDef{}, false
	}
	eventsMu.RLock()
	defer eventsMu.RUnlock()
	if def, exists := registeredMessages[full]; exists {
		return cloneMessageDef(def), true
	}
	if ValidEvents[ns][name] {
		return MessageDef{Namespace: ns, Name: name}, true
	}
	return MessageDef{}, false
}

// AllMessages returns every registered message across all namespaces, sorted
// deterministically by namespace then name. Entries registered via
// RegisterMessage/RegisterMessageDef include their full tag and destination
// metadata; entries registered only via RegisterNamespace/RegisterEventType
// are included as tagless MessageDef records. Safe for concurrent use.
func AllMessages() []MessageDef {
	eventsMu.RLock()
	defer eventsMu.RUnlock()

	seen := make(map[string]MessageDef, len(registeredMessages))
	var tb textbuf.Buffer
	for ns, evts := range ValidEvents {
		for name := range evts {
			full := tb.Reset().Str(ns).Byte(':').Str(name).String()
			seen[full] = MessageDef{Namespace: ns, Name: name}
		}
	}
	for full, def := range registeredMessages {
		seen[full] = cloneMessageDef(def)
	}

	out := make([]MessageDef, 0, len(seen))
	for _, def := range seen {
		out = append(out, def)
	}
	slices.SortFunc(out, func(a, b MessageDef) int {
		if a.Namespace != b.Namespace {
			return strings.Compare(a.Namespace, b.Namespace)
		}
		return strings.Compare(a.Name, b.Name)
	})
	return out
}
