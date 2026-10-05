// VALIDATES: RFC 4301 Section 4.4.1.2 asks users not to put several selector sets in
// one SPD entry until IKE can convey them. Ze's entry holds ONE local prefix, ONE
// remote prefix, one protocol and one port pair, so an operator cannot write a
// second set: the structure meets the obligation, and this pins that structure.

package ipsec

import (
	"net"
	"reflect"
	"slices"
	"testing"
)

// selectorTypes are the Go types a selector of Section 4.4.1.1 is held in: an address
// prefix and a port. A second selector set in one entry would have to be carried by a
// field that holds more than one of them.
var selectorTypes = []reflect.Type{
	reflect.TypeFor[net.IPNet](),
	reflect.TypeFor[PortSelector](),
}

// holdsSelector reports whether a value of type ty is, points to, or directly contains
// a field of one of the selectorTypes.
func holdsSelector(ty reflect.Type) bool {
	ty = derefType(ty)
	if slices.Contains(selectorTypes, ty) {
		return true
	}
	if ty.Kind() != reflect.Struct {
		return false
	}
	for f := range ty.Fields() {
		field := derefType(f.Type)
		//exhaustive:ignore // Only collection kinds need element projection to find selector-bearing fields.
		switch field.Kind() {
		case reflect.Slice, reflect.Array, reflect.Map:
			field = derefType(field.Elem())
		default:
		}
		if slices.Contains(selectorTypes, field) {
			return true
		}
	}
	return false
}

// derefType strips every pointer level from ty.
func derefType(ty reflect.Type) reflect.Type {
	for ty.Kind() == reflect.Pointer {
		ty = ty.Elem()
	}
	return ty
}

// RFC requirement: RFC4301-4.4.1.2-1 positive -- the SPD entry ParseIPsecConfig returns has no field that can hold more than one selector: no slice, array or map of a prefix, a port, or a struct holding one, so the model cannot carry a second selector set.
func TestRFC4301SPDEntryCannotCarryASecondSelectorSet(t *testing.T) {
	cfg, err := ParseIPsecConfig(spdTree("one", map[string]string{"action": "bypass", "protocol": "17"},
		map[string]string{"prefix": "10.0.0.0/24", "port": "53"},
		map[string]string{"prefix": "10.1.0.0/24", "port": "any"}))
	if err != nil {
		t.Fatalf("ParseIPsecConfig: %v", err)
	}
	entry, ok := cfg.Policies["one"]
	if !ok {
		t.Fatalf("entry one was not read; got %d entries", len(cfg.Policies))
	}
	selectors := 0
	for field := range reflect.TypeOf(entry).Fields() {
		//exhaustive:ignore // Only collection kinds can carry multiple selector sets; other kinds are single fields.
		switch field.Type.Kind() {
		case reflect.Slice, reflect.Array, reflect.Map:
			if holdsSelector(field.Type.Elem()) {
				t.Errorf("SPDPolicy.%s is a %s of selectors: an entry could carry more than one selector set", field.Name, field.Type)
			}
		default:
			if holdsSelector(field.Type) {
				selectors++
			}
		}
	}
	// Two prefixes and two ports: one local and one remote of each.
	if selectors != 4 {
		t.Errorf("SPDPolicy holds %d single selectors, want 4 (local and remote prefix, local and remote port)", selectors)
	}
}

// RFC requirement: RFC4301-4.4.1.2-1 positive -- an operator SPD entry is read as exactly
// one selector set: one local prefix, one remote prefix, one protocol and one port pair.
func TestRFC4301SPDEntryHoldsOneSelectorSet(t *testing.T) {
	cfg, err := ParseIPsecConfig(spdTree("one", map[string]string{"action": "bypass", "protocol": "17"},
		map[string]string{"prefix": "10.0.0.0/24", "port": "53"},
		map[string]string{"prefix": "10.1.0.0/24", "port": "any"}))
	if err != nil {
		t.Fatalf("ParseIPsecConfig: %v", err)
	}
	p, ok := cfg.Policies["one"]
	if !ok {
		t.Fatalf("entry one was not read; got %d entries", len(cfg.Policies))
	}
	if p.LocalPrefix == nil || p.LocalPrefix.String() != "10.0.0.0/24" {
		t.Errorf("local prefix %v, want the one written, 10.0.0.0/24", p.LocalPrefix)
	}
	if p.RemotePrefix == nil || p.RemotePrefix.String() != "10.1.0.0/24" {
		t.Errorf("remote prefix %v, want the one written, 10.1.0.0/24", p.RemotePrefix)
	}
	if p.Protocol != 17 || p.LocalPort.Port != 53 || !p.RemotePort.IsAny() {
		t.Errorf("protocol %d local port %d remote any=%v, want 17, 53, true", p.Protocol, p.LocalPort.Port, p.RemotePort.IsAny())
	}
}
