package credits

import (
	"errors"
	"strings"
	"testing"
)

// TestPresenceOfAnUndeclaredWorkPanics: a package that declares a work the
// generated data does not hold is a defect, refused at start-up with a BUG
// panic naming the work, never accepted in silence. Method: declare a name no
// declaration will ever carry and recover the panic.
func TestPresenceOfAnUndeclaredWorkPanics(t *testing.T) {
	const name = "a work no declaration names"
	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatalf("DeclarePresent(%q) was accepted", name)
		}
		message, isText := recovered.(string)
		if !isText {
			t.Fatalf("DeclarePresent(%q) panicked with %#v, want a BUG string", name, recovered)
		}
		if !strings.HasPrefix(message, "BUG: ") || !strings.Contains(message, name) {
			t.Fatalf("DeclarePresent(%q) panicked with %q, want a BUG naming the work", name, message)
		}
	}()
	DeclarePresent(name)
}

// TestRowsRefuseWithoutData: with no generated data compiled in, the list is
// an error, never an empty list a caller would read as a binary carrying no
// third-party work. Method: ask for the rows and require ErrNoData and no rows.
func TestRowsRefuseWithoutData(t *testing.T) {
	works, err := Works()
	if !errors.Is(err, ErrNoData) {
		t.Fatalf("Works() error = %v, want ErrNoData", err)
	}
	if works != nil {
		t.Fatalf("Works() answered %d rows beside its error", len(works))
	}
}

// TestLicenceRefusesWithoutData: both licence readers refuse the same way as
// the list, so neither prints an empty document. Method: call each and require
// ErrNoData and no bytes.
func TestLicenceRefusesWithoutData(t *testing.T) {
	document, err := LicenceDocument()
	if !errors.Is(err, ErrNoData) || document != nil {
		t.Fatalf("LicenceDocument() = %q, %v, want no bytes and ErrNoData", document, err)
	}
	one, err := LicenceOf("github.com/miekg/dns")
	if !errors.Is(err, ErrNoData) || one != nil {
		t.Fatalf("LicenceOf() = %q, %v, want no bytes and ErrNoData", one, err)
	}
}
