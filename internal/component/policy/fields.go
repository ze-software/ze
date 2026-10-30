// Design: plan/post-release/features/spec-route-policy-0-umbrella.md -- Route Field Vocabulary
// Related: registry.go -- the filter instances a chain names
// Related: register.go -- the vocabulary registrations
//
// The route field vocabulary. A filter type registers the fields it reads and
// the fields it sets; an attachment point registers the fields its routes
// carry and the fields it lets a filter set. A chain is refused at load when a
// filter in it reads a field the attachment does not carry or sets one it does
// not allow, so a filter never silently matches a zero.

package policy

import (
	"fmt"
	"slices"
	"strings"
	"sync"
)

// Field is one registered route field (prefix, tag, community, ...). Its zero
// value names no field and is never registered, so it cannot be mistaken for
// one.
type Field struct{ name string }

// String returns the field's config name, for messages.
func (f Field) String() string { return f.name }

// fieldUse is what a filter type or an attachment declares.
type fieldUse struct {
	owner  string  // an attachment: the plugin whose config holds the chain; a filter type: ""
	first  []Field // a filter type: fields read; an attachment: fields carried
	second []Field // a filter type: fields set; an attachment: fields settable
}

// vocabulary holds the registered fields, filter types and attachments. The
// registrations run from init(), the checks at config load; the mutex makes
// it safe for concurrent use.
type vocabulary struct {
	mu          sync.RWMutex
	fields      map[string]Field
	filterTypes map[string]fieldUse
	attachments map[string]fieldUse
	// perInstance holds, for a filter type whose instances each use a subset
	// of its fields (match, modify), the function that reads one instance's
	// fields from its config. The type's own declaration stays the superset.
	perInstance map[string]InstanceFields
}

func newVocabulary() *vocabulary {
	return &vocabulary{
		fields:      make(map[string]Field),
		filterTypes: make(map[string]fieldUse),
		attachments: make(map[string]fieldUse),
		perInstance: make(map[string]InstanceFields),
	}
}

// vocab is the process vocabulary every registration writes to.
var vocab = newVocabulary()

// MustRegisterField registers a route field. A protocol or filter plugin that
// needs a new field registers it here; nothing enumerates fields elsewhere.
// A second registration of one name is a Ze defect and panics at init.
func MustRegisterField(name string) Field { return vocab.mustRegisterField(name) }

// MustRegisterFilterType declares the fields a filter type reads and sets.
func MustRegisterFilterType(typeName string, reads, sets []Field) {
	vocab.mustRegisterFilterType(typeName, reads, sets)
}

// InstanceFields answers the fields one instance of a filter type reads and
// sets, from the instance's own config subtree. It MUST answer a subset of
// the type's MustRegisterFilterType declaration and MUST NOT change any state:
// a refused verify and a CLI validate run it.
type InstanceFields func(instance map[string]any) (reads, sets []Field)

// MustRegisterInstanceFields declares that the instances of typeName each use
// only the fields their own config names, so an instance of a broad type (a
// `match` naming only prefix-list and tag) attaches wherever those fields are
// carried. typeName MUST already be declared with MustRegisterFilterType. A
// second registration is a Ze defect and panics.
func MustRegisterInstanceFields(typeName string, fields InstanceFields) {
	vocab.mustRegisterInstanceFields(typeName, fields)
}

// MustRegisterAttachment declares the fields an attachment point's routes
// carry and the fields a filter there may set. owner is the registration name
// of the plugin whose config holds the chain: it MUST list PluginName in its
// Dependencies, which a test derives from AttachmentOwners. name is how the
// operator reads the attachment in an error ("isis import", "rib import").
func MustRegisterAttachment(owner, name string, carries, settable []Field) {
	vocab.mustRegisterAttachment(owner, name, carries, settable)
}

// AttachmentOwners answers, sorted and once each, the plugins that registered
// an attachment point.
func AttachmentOwners() []string { return vocab.attachmentOwners() }

// DeclaredFilterTypes answers, sorted, the filter types that declared the
// route fields they read and set. A ze:filter list missing from it cannot be
// checked against an attachment, so a test derives the list from the schema
// and compares.
func DeclaredFilterTypes() []string { return vocab.declaredFilterTypes() }

// CheckChain refuses a chain at an attachment when any instance it names
// reads a field the attachment does not carry or sets one it does not allow.
// A name the registry does not hold is left to Registry.ValidateName.
func CheckChain(attachment string, reg *Registry, names []string) error {
	return vocab.checkChain(attachment, reg, names)
}

func (v *vocabulary) mustRegisterField(name string) Field {
	v.mu.Lock()
	defer v.mu.Unlock()
	if name == "" {
		panic("BUG: policy: route field registered with an empty name")
	}
	if _, dup := v.fields[name]; dup {
		panic("BUG: policy: route field " + name + " registered twice")
	}
	f := Field{name: name}
	v.fields[name] = f
	return f
}

func (v *vocabulary) mustRegisterFilterType(typeName string, reads, sets []Field) {
	v.mustRegisterUse(v.filterTypes, "filter type", "", typeName, reads, sets)
}

func (v *vocabulary) mustRegisterInstanceFields(typeName string, fields InstanceFields) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if fields == nil {
		panic("BUG: policy: instance fields of " + typeName + " registered with no function")
	}
	if _, declared := v.filterTypes[typeName]; !declared {
		panic("BUG: policy: instance fields of " + typeName + " registered before its filter type")
	}
	if _, dup := v.perInstance[typeName]; dup {
		panic("BUG: policy: instance fields of " + typeName + " registered twice")
	}
	v.perInstance[typeName] = fields
}

func (v *vocabulary) mustRegisterAttachment(owner, name string, carries, settable []Field) {
	if owner == "" {
		panic("BUG: policy: attachment " + name + " registered with no owner plugin")
	}
	v.mustRegisterUse(v.attachments, "attachment", owner, name, carries, settable)
}

func (v *vocabulary) attachmentOwners() []string {
	v.mu.RLock()
	defer v.mu.RUnlock()
	owners := make([]string, 0, len(v.attachments))
	for _, use := range v.attachments {
		owners = append(owners, use.owner)
	}
	slices.Sort(owners)
	return slices.Compact(owners)
}

func (v *vocabulary) declaredFilterTypes() []string {
	v.mu.RLock()
	defer v.mu.RUnlock()
	types := make([]string, 0, len(v.filterTypes))
	for name := range v.filterTypes {
		types = append(types, name)
	}
	slices.Sort(types)
	return types
}

func (v *vocabulary) mustRegisterUse(into map[string]fieldUse, kind, owner, name string, first, second []Field) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if _, dup := into[name]; dup {
		panic("BUG: policy: " + kind + " " + name + " registered twice")
	}
	for _, f := range slices.Concat(first, second) {
		if _, known := v.fields[f.name]; !known {
			panic("BUG: policy: " + kind + " " + name + " names unregistered field " + f.name)
		}
	}
	into[name] = fieldUse{owner: owner, first: slices.Clone(first), second: slices.Clone(second)}
}

func (v *vocabulary) checkChain(attachment string, reg *Registry, names []string) error {
	v.mu.RLock()
	defer v.mu.RUnlock()
	at, ok := v.attachments[attachment]
	if !ok {
		return fmt.Errorf("policy: no attachment %q is registered, so its chain cannot be checked", attachment)
	}
	for _, name := range names {
		inst, ok := reg.Lookup(name)
		if !ok {
			continue
		}
		ft, ok := v.filterTypes[inst.Type]
		if !ok {
			return fmt.Errorf("%s filter %q (%s): the filter type declares no route fields, "+
				"so Ze cannot tell whether %s routes carry what it reads", attachment, name, inst.Type, attachment)
		}
		reads, sets := ft.first, ft.second
		if fields, ok := v.perInstance[inst.Type]; ok {
			reads, sets = fields(inst.Config)
			if extra := absent(slices.Concat(reads, sets), slices.Concat(ft.first, ft.second)); extra != "" {
				return fmt.Errorf("BUG: policy: %s instance %q names %s, which its filter type never declared", inst.Type, name, extra)
			}
		}
		if missing := absent(reads, at.first); missing != "" {
			return fmt.Errorf("%s filter %q (%s) reads %s, which %s routes do not carry; "+
				"remove it from this chain or use a filter that matches only %s",
				attachment, name, inst.Type, missing, attachment, joinFields(at.first))
		}
		if missing := absent(sets, at.second); missing != "" {
			return fmt.Errorf("%s filter %q (%s) sets %s, which a filter at %s may not set; "+
				"remove it from this chain",
				attachment, name, inst.Type, missing, attachment)
		}
	}
	return nil
}

// absent returns, joined, the fields of want that have does not hold.
func absent(want, have []Field) string {
	var missing []string
	for _, f := range want {
		if !slices.Contains(have, f) {
			missing = append(missing, f.name)
		}
	}
	return strings.Join(missing, ", ")
}

func joinFields(fields []Field) string {
	names := make([]string, len(fields))
	for i, f := range fields {
		names[i] = f.name
	}
	return strings.Join(names, ", ")
}
