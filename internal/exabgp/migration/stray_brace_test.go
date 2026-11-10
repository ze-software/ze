package migration

import "testing"

// TestMigrateStrayBraceNamesTheFix proves `ze exabgp migrate` refuses a '}'
// that closes no section in the operator's words (ExaBGP 6.0 says "this '}'
// closes no section"), with the line and the fix, not as the token RBRACE.
// Method: the qa/old/5.0 conf-extended-attributes shape, a neighbor block
// then one '}' more, through ParseExaBGPConfig, the parse migrate runs.
func TestMigrateStrayBraceNamesTheFix(t *testing.T) {
	_, err := ParseExaBGPConfig("neighbor 127.0.0.1 {\n}\n}\n")
	if err == nil {
		t.Fatal("a stray } parsed")
	}
	want := "line 3: this '}' closes no section\n" +
		"every '}' ends a block that a '{' before it opened, and no block is open at this point\n" +
		"fix: delete this '}'"
	if got := err.Error(); got != want {
		t.Errorf("refusal =\n%s\nwant\n%s", got, want)
	}
}
