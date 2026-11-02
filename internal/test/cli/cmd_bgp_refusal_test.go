package cli

import "testing"

// TestBgpUnknownVerbFails checks the exit status exposed by the suite entry point.
func TestBgpUnknownVerbFails(t *testing.T) {
	if code := CmdBgp([]string{"ac1-unknown-verb"}); code == 0 {
		t.Fatal("unknown suite verb returned success")
	}
}

func TestBgpHelpSucceeds(t *testing.T) {
	for _, args := range [][]string{nil, {"--help"}, {"help"}} {
		if code := CmdBgp(args); code != 0 {
			t.Fatalf("help %v returned %d", args, code)
		}
	}
}

// A valid suite must preserve its selector and flags, rather than taking the
// successful-help path that deliberately returns no runnable request.
func TestBgpValidSuiteRetainsSelection(t *testing.T) {
	flags, err := zeTestParseRunCLI([]string{"encode", "--list", "--pattern", "ac1-legacy-peer"})
	if err != nil {
		t.Fatal(err)
	}
	if flags == nil {
		t.Fatal("valid suite returned no runnable request")
	}
	if flags.command != "encode" || !flags.list || flags.pattern != "ac1-legacy-peer" {
		t.Fatalf("valid suite lost its selection: %+v", flags)
	}
}
