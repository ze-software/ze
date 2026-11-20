// Design: docs/architecture/testing/ci-format.md -- mock TACACS+ server for AAA testing
// Related: records.go -- the ACCT line this test reads

package tacacs

import (
	"testing"

	protocol "github.com/ze-software/ze/internal/component/tacacs"
)

// VALIDATES: an accounting REQUEST's cmd and cmd-arg arguments reach the
// mock's record and its ACCT line, joined by spaces as the AUTHOR line joins
// them, past the accounting arguments that precede them (RFC 8907 Section 8.3).
// PREVENTS: a .ci asserting what Ze accounts having nothing to read: the ACCT
// line printed the sequence and the total only.
// Method: Ze's own encoder (AcctRequest.MarshalBinary) writes the body, the
// parser reads it, and Line renders the record; a body with no cmd argument
// and a truncated body record no command, and the empty command prints cmd="".
func TestAcctRecordCarriesTheCommand(t *testing.T) {
	encode := func(t *testing.T, args []string) []byte {
		t.Helper()
		body, err := (&protocol.AcctRequest{
			Flags: protocol.AcctFlagStart, User: "admin", Port: "ssh", RemAddr: "127.0.0.1", Args: args,
		}).MarshalBinary()
		if err != nil {
			t.Fatal(err)
		}
		return body
	}
	accounted := []string{"task_id=7", "start_time=1", "service=shell", "cmd=show", "cmd-arg=bgp", "cmd-arg=rib", "cmd-arg=|", "cmd-arg=count"}
	for _, tc := range []struct {
		name        string
		body        []byte
		command     string
		commandSent bool
		line        string
	}{
		{"command and arguments", encode(t, accounted), "show bgp rib | count", true,
			`tacacs-mock: ACCT START seq=1 total=3 cmd="show bgp rib | count"`},
		{"empty command", encode(t, []string{"task_id=8", "start_time=1", "service=shell", "cmd="}), "", true,
			`tacacs-mock: ACCT START seq=1 total=3 cmd=""`},
		{"no cmd argument", encode(t, []string{"task_id=9", "start_time=1", "service=shell"}), "", false,
			"tacacs-mock: ACCT START seq=1 total=3"},
		{"truncated body", encode(t, accounted)[:20], "", false,
			"tacacs-mock: ACCT START seq=1 total=3"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			command, commandSent := parseAcctCommand(tc.body)
			if command != tc.command || commandSent != tc.commandSent {
				t.Fatalf("parseAcctCommand = %q, %v; want %q, %v", command, commandSent, tc.command, tc.commandSent)
			}
			record := Record{Kind: KindAcctStart, Seq: 1, Ordinal: 3, Command: command, CommandSent: commandSent}
			if got := record.Line(); got != tc.line {
				t.Fatalf("Line() = %q, want %q", got, tc.line)
			}
		})
	}
}
