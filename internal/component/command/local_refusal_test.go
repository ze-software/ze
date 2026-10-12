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
