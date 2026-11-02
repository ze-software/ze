// Design: docs/architecture/testing/ci-format.md -- mock TACACS+ server for AAA testing
// Related: tacacs.go -- the server that writes these records

package tacacs

import (
	"sync"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// Kind names what one mock record is.
type Kind uint8

const (
	KindUnspecified Kind = iota
	// KindConnection is an accepted TCP connection.
	KindConnection
	// KindSingleConnect is a connection's first packet asking for single-connect mode.
	KindSingleConnect
	// KindAuthen is an AUTHEN reply.
	KindAuthen
	// KindAuthor is an AUTHOR reply.
	KindAuthor
	// KindAcctStart is an ACCT request carrying the START flag.
	KindAcctStart
	// KindAcctStop is an ACCT request carrying the STOP flag.
	KindAcctStop
	// KindAcctWatchdog is an ACCT request carrying the WATCHDOG flag.
	KindAcctWatchdog
	// KindAcctOther is an ACCT request carrying none of the three.
	KindAcctOther
)

// Record is one thing a mock server took or answered. Line renders it as the
// stderr line the mock prints, which is what .ci files assert, so a caller in
// the same process can wait on the record instead of on a clock.
type Record struct {
	Kind      Kind
	Remote    string // connection and single-connect: the client address
	Ordinal   uint64 // connection: its number; accounting: the running total
	User      string // authen and author
	Command   string // author: the command, arguments joined by spaces
	Pass      bool   // authen and author: the reply passed
	Status    uint8  // authen: the reply status
	Privilege string // author: the priv-lvl argument returned, or empty
	Seq       uint8  // accounting: the packet sequence number
}

// mockPrefix opens every line the mock prints.
const mockPrefix = "tacacs-mock: "

// Line answers the stderr line the mock prints for the record, with no newline.
func (r Record) Line() string {
	var tb textbuf.Buffer
	tb.Str(mockPrefix)
	switch r.Kind {
	case KindConnection:
		tb.Str("connection #").Uint(r.Ordinal).Str(" from ").Str(r.Remote)
	case KindSingleConnect:
		tb.Str("single-connect accepted on ").Str(r.Remote)
	case KindAuthen:
		tb.Str("AUTHEN reply user=").Quoted(r.User).Str(" status=0x").Hex([]byte{r.Status})
	case KindAuthor:
		reply := "FAIL"
		if r.Pass {
			reply = "PASS_ADD"
		}
		tb.Str("AUTHOR cmd=").Quoted(r.Command).Str(" reply=").Str(reply)
		tb.Str(" user=").Quoted(r.User).Str(" privilege=").Quoted(r.Privilege)
	case KindAcctStart, KindAcctStop, KindAcctWatchdog, KindAcctOther:
		tb.Str("ACCT ").Str(acctKindName(r.Kind)).Str(" seq=").Uint8(r.Seq).Str(" total=").Uint(r.Ordinal)
	case KindUnspecified:
		tb.Str("unspecified record")
	default:
		panic("BUG: tacacs mock record of an unknown kind")
	}
	return tb.String()
}

func acctKindName(kind Kind) string {
	switch kind {
	case KindAcctStart:
		return "START"
	case KindAcctStop:
		return "STOP"
	case KindAcctWatchdog:
		return "WATCHDOG"
	case KindAcctOther:
		return "OTHER"
	case KindUnspecified, KindConnection, KindSingleConnect, KindAuthen, KindAuthor:
		panic("BUG: acctKindName called for a record that is not accounting")
	default:
		panic("BUG: tacacs mock record of an unknown kind")
	}
}

// recordsMax bounds the journal. A fixture's run takes a few dozen records;
// past the bound, records are no longer kept, and a caller waiting on one
// fails on its own deadline naming what it did not see.
const recordsMax = 4096

// journal holds every record of the mock servers of this process, in order.
// Safe for concurrent use.
var journal struct {
	mu      sync.Mutex
	records []Record
}

// remember prints the record's line when logging is on, then keeps it. The
// line is written first, so a caller that sees the record knows its line is
// already on stderr.
func remember(record Record, log bool) {
	if log {
		var tb textbuf.Buffer
		tb.Str(record.Line()).Byte('\n').StdErr() //nolint:errcheck // stderr is the log itself; a failed write has nowhere to be reported
	}
	journal.mu.Lock()
	defer journal.mu.Unlock()
	if len(journal.records) < recordsMax {
		journal.records = append(journal.records, record)
	}
}

// Received answers a copy of every record this process's mock servers have
// kept, in the order they were written. Safe for concurrent use.
func Received() []Record {
	journal.mu.Lock()
	defer journal.mu.Unlock()
	out := make([]Record, len(journal.records))
	copy(out, journal.records)
	return out
}
