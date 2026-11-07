// Design: docs/architecture/plugin/feature-gates.md -- selected fields need an exact retained owner.
package repoinventory

import "testing"

// TestCommittedExternalCollectionKeysCannotBorrowMethodOwner distinguishes a
// real bound method from a fabricated field named by a collection key. A
// direct literal receiver resolves its real method as its named form does:
// the method's own body decides, so an installing Fire refuses as a
// registration reaching external code and an inert Fire publishes, which a
// key projected as an inert field could not tell apart.
func TestCommittedExternalCollectionKeysCannotBorrowMethodOwner(t *testing.T) {
	for _, shape := range []struct{ name, declarations, literal string }{
		{"array", "type parcelCollection [1]uint32\nconst Fire = 0\n", "parcelCollection{Fire: 0}"},
		{"slice", "type parcelCollection []uint32\nconst Fire = 0\n", "parcelCollection{Fire: 0}"},
		{"map", "type parcelCollection map[string]uint32\nconst Fire = \"key\"\n", "parcelCollection{Fire: 0}"},
	} {
		for _, kind := range []string{"plugin", "yang"} {
			t.Run(shape.name+"/"+kind, func(t *testing.T) {
				fixture := externalEffectsFixture(t, kind)
				externalPhantomMethodControl(t, fixture, shape.declarations, shape.literal)
				externalLiteralMethodOwner(t, fixture, shape.declarations, shape.literal)
			})
		}
	}
}

// externalLiteralMethodOwner selects Fire directly on the literal: the
// installing method refuses as a registration, the inert one publishes.
func externalLiteralMethodOwner(t *testing.T, fixture *asmInventoryFixture, shapes, literal string) {
	t.Helper()
	for _, method := range []struct {
		name, body string
		installs   bool
	}{{"installer", "install(0)", true}, {"inert", "", false}} {
		t.Run(method.name, func(t *testing.T) {
			declarations := shapes + "func (parcelCollection) Fire() { " + method.body + " }\n"
			call := "store.Callback = (" + literal + ").Fire; parcel, fault := remote.Stat(\".\"); _ = parcel; _ = fault"
			fixture.goFile(t, callableOwnerCaller, externalEffectsSource(declarations, call))
			commitInventoryFixture(t, fixture.root)
			if method.installs {
				externalEffectsRefusal(t, fixture, "registration")
				return
			}
			externalEffectsPositive(t, fixture)
		})
	}
}

// TestCommittedExternalAggregateFieldsCannotBorrowMethodOwner covers type and
// value aggregation of nested fields. A collection element or named child field
// does not add that child's fields to its holder's actual selector namespace:
// Fire selected on the holder literal is the holder's own method, whose body
// decides (externalLiteralMethodOwner).
func TestCommittedExternalAggregateFieldsCannotBorrowMethodOwner(t *testing.T) {
	const element = "type parcelElement struct { Fire uint32 }\n"
	for _, shape := range []struct{ name, declarations, literal string }{
		{"array element", element + "type parcelCollection [1]parcelElement\n", "parcelCollection{parcelElement{Fire: 0}}"},
		{"slice element", element + "type parcelCollection []parcelElement\n", "parcelCollection{parcelElement{Fire: 0}}"},
		{"map value", element + "type parcelCollection map[string]parcelElement\n", "parcelCollection{\"key\": parcelElement{Fire: 0}}"},
		{"named child", element + "type parcelCollection struct { Inner parcelElement }\n", "parcelCollection{Inner: parcelElement{Fire: 0}}"},
	} {
		for _, kind := range []string{"plugin", "yang"} {
			t.Run(shape.name+"/"+kind, func(t *testing.T) {
				fixture := externalEffectsFixture(t, kind)
				externalPhantomMethodControl(t, fixture, shape.declarations, shape.literal)
				externalLiteralMethodOwner(t, fixture, shape.declarations, shape.literal)
			})
		}
	}
}

// TestCommittedExternalAssertedOwnerCannotBorrowField keeps a legitimate field
// projection in one interface alternative. The asserted receiver's actual type
// has a method with that spelling, so its callback cannot use the other field.
func TestCommittedExternalAssertedOwnerCannotBorrowField(t *testing.T) {
	const shapes = "type parcelElement struct { Fire uint32 }\ntype parcelCollection struct{}\n"
	for _, kind := range []string{"plugin", "yang"} {
		t.Run(kind, func(t *testing.T) {
			fixture := externalEffectsFixture(t, kind)
			externalPhantomMethodControl(t, fixture, shapes, "parcelCollection{}")
			for _, method := range []struct{ name, body string }{{"installer", "install(0)"}, {"inert", ""}} {
				t.Run(method.name, func(t *testing.T) {
					declarations := shapes + "func (parcelCollection) Fire() { " + method.body + " }\n"
					call := "var boxed any = parcelElement{Fire: 0}; boxed = parcelCollection{}; store.Callback = (boxed.(parcelCollection)).Fire; parcel, fault := remote.Stat(\".\"); _ = parcel; _ = fault"
					fixture.goFile(t, callableOwnerCaller, externalEffectsSource(declarations, call))
					commitInventoryFixture(t, fixture.root)
					externalEffectsRefusal(t, fixture, "ownership")
				})
			}
		})
	}
}

// TestCommittedExternalNestedFieldsKeepScalarSiblingIndependent keeps actual
// keyed fields precise when a named child has a different field of the same
// spelling. A child callback cannot become the root scalar's capability.
func TestCommittedExternalNestedFieldsKeepScalarSiblingIndependent(t *testing.T) {
	const child = "type parcelInner struct { Label func() }\n"
	const box = "type parcelBox struct { Inner parcelInner; Label any }\n"
	for _, shape := range []struct{ name, declarations, datatype string }{
		{"named", child + box, "parcelBox"},
		{"alias", child + box + "type parcelAlias = parcelBox\n", "parcelAlias"},
		{"defined", child + box + "type parcelDefined parcelBox\n", "parcelDefined"},
		{"generic", child + "type parcelGeneric[T any] struct { Inner parcelInner; Label T }\n", "parcelGeneric[any]"},
	} {
		for _, kind := range []string{"plugin", "yang"} {
			t.Run(shape.name+"/"+kind, func(t *testing.T) {
				fixture := externalEffectsFixture(t, kind)
				before := "order.Slice([]any{(" + shape.datatype + "{Inner: parcelInner{Label: func() { install(0) }}, Label: "
				after := "}).Label}, func(i, j int) bool { return false })"
				fixture.goFile(t, callableOwnerCaller, externalEffectsSource(shape.declarations, before+"\"copy\""+after))
				commitInventoryFixture(t, fixture.root)
				externalEffectsPositive(t, fixture)

				fixture.goFile(t, callableOwnerCaller, externalEffectsSource(shape.declarations, before+"func() { install(0) }"+after))
				commitInventoryFixture(t, fixture.root)
				externalEffectsRefusal(t, fixture, "registration")
			})
		}
	}
}

func externalPhantomMethodControl(t *testing.T, fixture *asmInventoryFixture, shapes, literal string) {
	t.Helper()
	declarations := shapes + "func (parcelCollection) Fire() {}\n"
	call := "value := " + literal + "; store.Callback = value.Fire; parcel, fault := remote.Stat(\".\"); _ = parcel; _ = fault"
	fixture.goFile(t, callableOwnerCaller, externalEffectsSource(declarations, call))
	commitInventoryFixture(t, fixture.root)
	externalEffectsPositive(t, fixture)
}
