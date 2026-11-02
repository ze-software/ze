// Design: docs/architecture/core-design.md -- Policy Filter Chain, "The BGP route view"
// Related: view.go -- Session.meta, the map the metadata fields read
// Related: session.go -- the writers the metadata fields share with the session facts

package policyview

import (
	"errors"
	"fmt"

	"github.com/ze-software/ze/internal/component/policy"
)

// metadataMax bounds the registered route metadata fields, so a member's
// metadata sets fit a fixed array (policychain.Sets) and a field's slot fits
// a uint8 mask. Raise it when an owner registers one past it:
// MustRegisterMetadata refuses it at init.
const metadataMax = 8

// MetadataKind is the Go type a route metadata key holds in the route's
// metadata map, the cross-plugin contract of docs/architecture/meta/README.md.
// The zero kind names none.
type MetadataKind uint8

const (
	MetadataUnspecified MetadataKind = iota
	MetadataText                     // a string, such as src-role
	MetadataOctet                    // a uint8, such as stale
	MetadataFlag                     // a bool, such as rebuilt
)

// ErrNoMetadataMap is answered by SetMetadata on a view whose session holds
// no metadata map: an export rail, whose map is the received route's and
// shared by every destination, or a run with no route (a dry run).
var ErrNoMetadataMap = errors.New("the route has no metadata map a filter may set here")

// Metadata is one registered metadata field: its key in the route's metadata
// map, which is the field's name (one name per fact), and its kind.
type Metadata struct {
	field  policy.Field
	kind   MetadataKind
	slot   uint8
	reader Reader // the field's view reader, whose cached answer a set drops
}

// Field answers the vocabulary field.
func (m Metadata) Field() policy.Field { return m.field }

// Kind answers the Go type the key holds.
func (m Metadata) Kind() MetadataKind { return m.kind }

// Slot answers the field's index in registration order, below metadataMax.
func (m Metadata) Slot() uint8 { return m.slot }

// metadataTable holds the registered metadata fields. It is written only from
// init() (MustRegisterMetadata) and read afterwards, so it needs no lock.
var metadataTable struct {
	byName map[string]Metadata
	fields []policy.Field
}

// MustRegisterMetadata registers field as a route metadata key of kind,
// read from and set into the route's metadata map under the field's own
// name. The key's owner calls it from its init(): bgp-role for src-role and
// src-peer-role, bgp-gr for stale. It registers the field's reader and JSON
// writer, so the field is carried by the BGP attachments, and a filter at
// bgp import may set it (spec-filter-wire-6-stage-filters D-8, D-9). A second
// registration, an unspecified kind, or one past metadataMax is a Ze defect
// and panics.
func MustRegisterMetadata(field policy.Field, kind MetadataKind) {
	if kind == MetadataUnspecified || kind > MetadataFlag {
		panic("BUG: policyview: metadata field " + field.String() + " registered with no kind")
	}
	if metadataTable.byName == nil {
		metadataTable.byName = make(map[string]Metadata)
	}
	if _, dup := metadataTable.byName[field.String()]; dup {
		panic("BUG: policyview: metadata field " + field.String() + " registered twice")
	}
	if len(metadataTable.fields) == metadataMax {
		panic("BUG: policyview: more metadata fields than metadataMax; raise it for " + field.String())
	}
	m := Metadata{field: field, kind: kind, slot: uint8(len(metadataTable.fields))}
	MustRegisterReader(field, readMetadata(m))
	reader, err := ReaderFor(field)
	if err != nil {
		panic("BUG: policyview: metadata field " + field.String() + ": " + err.Error())
	}
	m.reader = reader
	metadataTable.byName[field.String()] = m
	metadataTable.fields = append(metadataTable.fields, field)
	mustRegisterWriter(field, metadataWriter(kind), 0, "")
}

// MetadataFields answers every registered metadata field, in registration
// order. The caller MUST NOT modify it.
func MetadataFields() []policy.Field { return metadataTable.fields }

// MetadataNamed answers the metadata field named name, and false for a name
// no owner registered as metadata.
func MetadataNamed(name string) (Metadata, bool) {
	m, ok := metadataTable.byName[name]
	return m, ok
}

// WithImportMetadata answers the session with meta as the ingress pass's
// metadata map, which a metadata set writes. meta MUST be non-nil and MUST be
// the map the received event carries.
func (s Session) WithImportMetadata(meta map[string]any) Session {
	s.meta, s.metaWritable = meta, meta != nil
	return s
}

// WithExportMetadata answers the session with meta as the received route's
// metadata map, which the metadata fields read and no set writes: every
// destination of the route shares it.
func (s Session) WithExportMetadata(meta map[string]any) Session {
	s.meta, s.metaWritable = meta, false
	return s
}

// SetMetadata writes value under m's key into the session's metadata map, so
// a later member of the pass reads it and the received event carries it.
// value MUST be of m's kind: a string, a uint8 or a bool.
func (v *View) SetMetadata(m Metadata, value any) error {
	if !v.session.metaWritable {
		return ErrNoMetadataMap
	}
	v.session.meta[m.field.String()] = value
	// A member that read the field earlier in the run cached the old
	// answer: drop it, so a later read answers the set (read-after-set).
	v.slots[m.reader.slot-1].run = 0
	return nil
}

// MetadataWritable reports whether the session holds a metadata map a filter
// may set (SetMetadata): the ingress pass's.
func (v *View) MetadataWritable() bool { return v.session.metaWritable }

// readMetadata reads m's key from the session's metadata map. An absent key,
// or a session with no map, answers absent; a value of another Go type is a
// writer's defect, answered as an error so the member fails its route.
func readMetadata(m Metadata) readFunc {
	key := m.field.String()
	return func(v *View) (Value, error) {
		raw, ok := v.session.meta[key]
		if !ok {
			return Value{}, nil
		}
		switch m.kind {
		case MetadataText:
			if text, ok := raw.(string); ok {
				return Value{present: true, text: text}, nil
			}
		case MetadataOctet:
			if octet, ok := raw.(uint8); ok {
				return Value{present: true, number: uint64(octet)}, nil
			}
		case MetadataFlag:
			if flag, ok := raw.(bool); ok {
				return Value{present: true, flag: flag}, nil
			}
		case MetadataUnspecified:
		}
		return Value{}, fmt.Errorf("%w: metadata %s holds %T, not its registered kind", ErrMalformed, key, raw)
	}
}

// metadataWriter answers the JSON writer of a kind.
func metadataWriter(kind MetadataKind) writeFunc {
	switch kind {
	case MetadataText:
		return writeText
	case MetadataOctet:
		return writeNumber
	case MetadataFlag:
		return writeFlag
	case MetadataUnspecified:
	}
	panic("BUG: policyview: metadata kind has no writer")
}
