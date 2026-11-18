package command

import (
	"errors"
	"testing"
)

// TestLocalRefusalNamesTheHelpForAMissingArgument proves a missing mandatory
// argument tells the operator the command that lists every argument, and that
// any other refusal is printed alone. Method: the refusal text for one missing
// leaf and for one unrelated error, compared whole.
func TestLocalRefusalNamesTheHelpForAMissingArgument(t *testing.T) {
	t.Parallel()

	missing := &MissingArgumentError{Leaf: "module-name"}
	got := localRefusal("ze set debug module", missing)
	want := "error: required argument missing: module-name\n" +
		"The command definition makes module-name mandatory and supplies no further explanation\n" +
		"fix: supply the required module-name argument\n" +
		"hint: run 'ze set debug module help' to see the arguments it takes\n"
	if got != want {
		t.Fatalf("missing argument refusal:\n got %q\nwant %q", got, want)
	}

	other := errors.New("value too long for name")
	got = localRefusal("ze show env get", other)
	if got != "error: value too long for name\n" {
		t.Fatalf("other refusal carries a hint it does not owe: %q", got)
	}
}

// TestArgumentRefusalPreservesTheMissingCause proves the shared renderer keeps
// both the missing leaf's identity and the complete local diagnostic. Method:
// render the existing missing-argument example, then unwrap the shared error.
func TestArgumentRefusalPreservesTheMissingCause(t *testing.T) {
	t.Parallel()

	missing := &MissingArgumentError{Leaf: "module-name"}
	refusal := ArgumentRefusal("ze set debug module", missing)
	got := LocalErrorLine(refusal)
	want := "error: required argument missing: module-name\n" +
		"The command definition makes module-name mandatory and supplies no further explanation\n" +
		"fix: supply the required module-name argument\n" +
		"hint: run 'ze set debug module help' to see the arguments it takes\n"
	if got != want {
		t.Fatalf("shared missing argument refusal:\n got %q\nwant %q", got, want)
	}
	cause, found := errors.AsType[*MissingArgumentError](refusal)
	if !found {
		t.Fatal("shared refusal lost its typed missing-argument cause")
	}
	if cause != missing {
		t.Fatal("shared refusal replaced the original missing-argument cause")
	}
}

// TestArgumentRefusalLeavesOtherErrorsUnchanged proves a refusal that names no
// missing argument acquires neither wrapping nor a help hint. Method: compare
// the original error identity and complete diagnostic from the existing case.
func TestArgumentRefusalLeavesOtherErrorsUnchanged(t *testing.T) {
	t.Parallel()

	other := errors.New("value too long for name")
	refusal := ArgumentRefusal("ze show env get", other)
	if refusal != other { //nolint:errorlint // Exact identity proves no wrapping was introduced.
		t.Fatal("shared refusal changed an error that needs no argument help")
	}
	got := LocalErrorLine(refusal)
	if got != "error: value too long for name\n" {
		t.Fatalf("other refusal carries a hint it does not owe: %q", got)
	}
}
