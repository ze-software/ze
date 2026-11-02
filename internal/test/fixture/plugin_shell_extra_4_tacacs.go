// Design: docs/functional-tests.md -- an observer waits for the state its assertions need
// Related: plugin_shell_extra_4.go -- the TACACS+ observers that wait here
// Related: internal/test/mock/tacacs/records.go -- the records they wait on

package fixture

import (
	"context"
	"errors"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	tacacsmock "github.com/ze-software/ze/internal/test/mock/tacacs"
)

// tacacsExpect4 is one record a TACACS+ observer waits for the in-process mock
// to have taken, count times, before the observer ends. The record's Line is
// the stderr line the .ci asserts, which TestTacacsObserversWaitForWhatTheirCIExpects
// checks, so the wait and the assertion name the same thing.
type tacacsExpect4 struct {
	record tacacsmock.Record
	count  int
}

// matches reports whether the mock record is one this expectation counts. The
// fields compared are the ones that identify the record; ordinals, sequence
// numbers and addresses vary from run to run.
func (expect tacacsExpect4) matches(record tacacsmock.Record) bool {
	if record.Kind != expect.record.Kind {
		return false
	}
	switch expect.record.Kind {
	case tacacsmock.KindAuthen:
		return record.User == expect.record.User && record.Pass == expect.record.Pass
	case tacacsmock.KindAuthor:
		if record.User != expect.record.User {
			return false
		}
		return record.Command == expect.record.Command && record.Pass == expect.record.Pass
	case tacacsmock.KindConnection, tacacsmock.KindSingleConnect,
		tacacsmock.KindAcctStart, tacacsmock.KindAcctStop,
		tacacsmock.KindAcctWatchdog, tacacsmock.KindAcctOther:
		return true
	case tacacsmock.KindUnspecified:
		return false
	default:
		panic("BUG: tacacs expectation of an unknown kind")
	}
}

// tacacsLogin4 is what one TACACS+ login leaves at the mock: the PAP
// authentication, then the session authorization ze asks for before it grants
// profiles (tacacsAuthenticator.handlePass). ze logs "auth success" and the
// granted profiles only once both are answered.
func tacacsLogin4(user string) []tacacsExpect4 {
	return []tacacsExpect4{
		{record: tacacsmock.Record{Kind: tacacsmock.KindAuthen, User: user, Pass: true}, count: 1},
		{record: tacacsmock.Record{Kind: tacacsmock.KindAuthor, User: user, Pass: true}, count: 1},
	}
}

// tacacsExpectations4 names, per TACACS+ observer driven by the mock, the
// records it waits for before it ends. Observe stops ze as soon as the scenario
// returns, so a record ze sends after the command's answer (accounting is
// queued to a background worker, TacacsAccountant) is waited for here, never
// slept for.
var tacacsExpectations4 = map[string][]tacacsExpect4{
	"tacacs-auth": tacacsLogin4("admin"),
	// Two commands ran, so two START and two STOP records: the .ci asserts
	// one of each, and waiting for both commands' records leaves nothing
	// in flight at shutdown.
	"tacacs-acct": append(tacacsLogin4("admin"),
		tacacsExpect4{record: tacacsmock.Record{Kind: tacacsmock.KindAcctStart}, count: 2},
		tacacsExpect4{record: tacacsmock.Record{Kind: tacacsmock.KindAcctStop}, count: 2},
	),
	"tacacs-author": {
		{record: tacacsmock.Record{Kind: tacacsmock.KindAuthor, User: "admin", Command: "show bgp", Pass: true}, count: 1},
		{record: tacacsmock.Record{Kind: tacacsmock.KindAuthor, User: "admin", Command: "clear interface counters all"}, count: 1},
	},
	"tacacs-readonly": tacacsLogin4("noc"),
	// Four commands with accounting on: the .ci rejects a second connection,
	// and the accounting records are what would open one, so all eight are
	// waited for before the observer ends.
	"tacacs-singleconnect": {
		{record: tacacsmock.Record{Kind: tacacsmock.KindSingleConnect}, count: 1},
		{record: tacacsmock.Record{Kind: tacacsmock.KindConnection, Ordinal: 1}, count: 1},
		{record: tacacsmock.Record{Kind: tacacsmock.KindAcctStart}, count: 4},
		{record: tacacsmock.Record{Kind: tacacsmock.KindAcctStop}, count: 4},
	},
}

// tacacsAwaitDelay is the pause between two reads of the mock's records.
const tacacsAwaitDelay = 50 * time.Millisecond

// pluginShellExtra4AwaitTacacs waits, within the test's budget, until the mock
// has taken every record the named observer expects.
func pluginShellExtra4AwaitTacacs(ctx context.Context, name string) error {
	expectations, known := tacacsExpectations4[name]
	if !known {
		return errors.New("BUG: TACACS+ observer " + name + " names no records to wait for in tacacsExpectations4")
	}
	var missing []string
	Poll(ctx, WaitAttempts(80, tacacsAwaitDelay, 400), tacacsAwaitDelay, func() bool {
		missing = tacacsMissing4(expectations, tacacsmock.Received())
		return len(missing) == 0
	})
	if len(missing) == 0 {
		return nil
	}
	var tb textbuf.Buffer
	tb.Str("the TACACS+ mock never took what the .ci expects: ").Join(missing, "; ")
	return errors.New(tb.String())
}

// tacacsMissing4 answers, for each expectation the records do not yet meet,
// the line it waits for and how many of it the mock has taken.
func tacacsMissing4(expectations []tacacsExpect4, records []tacacsmock.Record) []string {
	var missing []string
	for _, expect := range expectations {
		seen := 0
		for _, record := range records {
			if expect.matches(record) {
				seen++
			}
		}
		if seen >= expect.count {
			continue
		}
		var tb textbuf.Buffer
		tb.Str(expect.record.Line()).Str(" (").Int(int64(seen)).Str(" of ").Int(int64(expect.count)).Byte(')')
		missing = append(missing, tb.String())
	}
	return missing
}
