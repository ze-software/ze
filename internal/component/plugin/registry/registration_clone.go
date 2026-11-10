// Design: docs/architecture/plugin/plugin-system.md -- registration fields, the seal after start-up
// Related: registry.go -- Register, Lookup, All, the paths that store and hand out clones
// Related: seal.go -- Seal, the other half of keeping the registry what init() made it

package registry

import "reflect"

// cloneRegistration answers a copy of reg that shares no slice, map or
// pointer with it, nested ones included (a CommandDecl's Args, a
// FilterInsertion's Values, a DoctorCheckDef's Codes), so the holder of one
// copy cannot change what the holder of the other reads. Register stores a
// clone and Lookup and All hand one out, so nothing outside this package can
// change a stored registration. Function values are shared: a function value
// cannot be changed through a copy. Safe for concurrent use while nobody
// writes reg, which the registry guarantees by never writing a stored
// registration in place.
//
// The copy walks the type by reflection rather than naming each field, so a
// field added to Registration or to a type it holds is copied with no edit
// here; TestRegistrationCloneCoversEveryReferenceField proves it.
func cloneRegistration(reg *Registration) *Registration {
	clone := new(Registration)
	deepCopyInto(reflect.ValueOf(clone).Elem(), reflect.ValueOf(reg).Elem())
	return clone
}

// deepCopyInto writes a copy of source into target, which MUST be settable,
// of source's type, and hold its zero value.
//
// The recursion follows the static type of Registration, whose nesting is a
// fixed few levels (a slice of structs holding slices), so the type bounds
// its depth; the data never does, because no type it holds refers to itself.
// An interface, a channel or an unsafe pointer cannot be copied without
// sharing what it refers to, so a Registration that holds one is a Ze defect
// and panics.
func deepCopyInto(target, source reflect.Value) {
	switch source.Kind() {
	case reflect.Slice:
		if source.IsNil() {
			return
		}
		copied := reflect.MakeSlice(source.Type(), source.Len(), source.Len())
		for i := range source.Len() {
			deepCopyInto(copied.Index(i), source.Index(i))
		}
		target.Set(copied)
	case reflect.Map:
		if source.IsNil() {
			return
		}
		copied := reflect.MakeMapWithSize(source.Type(), source.Len())
		entries := source.MapRange()
		for entries.Next() {
			value := reflect.New(entries.Value().Type()).Elem()
			deepCopyInto(value, entries.Value())
			copied.SetMapIndex(entries.Key(), value)
		}
		target.Set(copied)
	case reflect.Pointer:
		if source.IsNil() {
			return
		}
		copied := reflect.New(source.Type().Elem())
		deepCopyInto(copied.Elem(), source.Elem())
		target.Set(copied)
	case reflect.Struct:
		for i := range source.NumField() {
			deepCopyInto(target.Field(i), source.Field(i))
		}
	case reflect.Array:
		for i := range source.Len() {
			deepCopyInto(target.Index(i), source.Index(i))
		}
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64, reflect.Complex64, reflect.Complex128,
		reflect.String, reflect.Func:
		// A scalar, a string and a function value copy by value.
		target.Set(source)
	case reflect.Interface, reflect.Chan, reflect.UnsafePointer, reflect.Invalid:
		panic("BUG: registry: a Registration holds a " + source.Kind().String() +
			", which cloneRegistration cannot copy without sharing")
	}
}
