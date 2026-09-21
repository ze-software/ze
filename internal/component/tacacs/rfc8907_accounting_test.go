// Design: (none -- new TACACS+ component)
// Detail: accounting.go -- CommandStart and CommandStop, the record builders
// Detail: authorizer.go -- splitTacacsTokens, the Section 8.2 argument builder
//
// VALIDATES: RFC 8907 Section 7.1 (STOP never carries WATCHDOG), Section 8
// (the argument dictionary), Section 8.1 (start_time is UTC epoch seconds)
// and Section 8.3 (accounting arguments first, one task_id per event, no
// task_id reuse, service and cmd on every command record).
// PREVENTS: a record whose accounting arguments trail the authorization
// ones, a local-zone timestamp, or a STOP whose task_id is not the START's.
//
// The accountant is never started: each record is read back from its queue
// so the test sees the exact AcctRequest the worker would send.

package tacacs

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// dequeue returns the next record the accountant queued, or fails.
func dequeue(t *testing.T, acct *TacacsAccountant) *AcctRequest {
	t.Helper()
	select {
	case msg := <-acct.queue:
		return msg.req
	default:
		t.Fatal("no accounting record was queued")
		return nil
	}
}

func newQueuedAccountant() *TacacsAccountant {
	return NewTacacsAccountant(NewTacacsClient(TacacsClientConfig{}), nil)
}

// argValue returns the value of the first argument named name, and whether
// one exists.
func argValue(args []string, name string) (string, bool) {
	for _, arg := range args {
		if strings.HasPrefix(arg, name+"=") {
			return arg[len(name)+1:], true
		}
	}
	return "", false
}

// argIndex returns the position of the first argument named name, or -1.
func argIndex(args []string, name string) int {
	for i, arg := range args {
		if argName(arg) == name {
			return i
		}
	}
	return -1
}

// RFC requirement: RFC8907-7.2-1 positive — the STOP record's flags octet is exactly TAC_PLUS_ACCT_FLAG_STOP.
func TestRFC8907StopRecordCarriesStopFlagOnly(t *testing.T) {
	acct := newQueuedAccountant()
	acct.CommandStop("7", "alice", "192.0.2.1", "show version")
	stop := dequeue(t, acct)
	require.Equal(t, uint8(AcctFlagStop), stop.Flags)
}

// RFC requirement: RFC8907-7.2-1 negative — the WATCHDOG bit is clear on a STOP record; STOP and WATCHDOG never appear together.
func TestRFC8907StopRecordNeverCarriesWatchdog(t *testing.T) {
	acct := newQueuedAccountant()
	acct.CommandStop("7", "alice", "192.0.2.1", "show version")
	stop := dequeue(t, acct)
	require.NotZero(t, stop.Flags&AcctFlagStop, "the record is a STOP")
	require.Zero(t, stop.Flags&AcctFlagWatchdog, "WATCHDOG must not be set with STOP")
}

// section8Names is the RFC 8907 Section 8.2 and 8.3 argument dictionary Ze
// draws from for the shell-command use case.
var section8Names = map[string]bool{
	"service": true, "cmd": true, "cmd-arg": true,
	"task_id": true, "start_time": true, "stop_time": true,
}

// RFC requirement: RFC8907-8-1 positive — the shell-command use case is expressed with the Section 8 arguments service, cmd, cmd-arg, task_id and start_time / stop_time.
func TestRFC8907CommandRecordsUseDictionaryArguments(t *testing.T) {
	acct := newQueuedAccountant()
	id := acct.CommandStart("alice", "192.0.2.1", "show bgp summary")
	start := dequeue(t, acct)
	acct.CommandStop(id, "alice", "192.0.2.1", "show bgp summary")
	stop := dequeue(t, acct)

	for _, name := range []string{"service", "cmd", "cmd-arg", "task_id", "start_time"} {
		require.NotEqual(t, -1, argIndex(start.Args, name), "START lacks %s", name)
	}
	for _, name := range []string{"service", "cmd", "cmd-arg", "task_id", "stop_time"} {
		require.NotEqual(t, -1, argIndex(stop.Args, name), "STOP lacks %s", name)
	}
}

// RFC requirement: RFC8907-8-1 negative — no argument name outside the Section 8 dictionary appears in a START or STOP record.
func TestRFC8907CommandRecordsCarryNoForeignArgument(t *testing.T) {
	acct := newQueuedAccountant()
	id := acct.CommandStart("alice", "192.0.2.1", "show bgp summary")
	start := dequeue(t, acct)
	acct.CommandStop(id, "alice", "192.0.2.1", "show bgp summary")
	stop := dequeue(t, acct)

	for _, arg := range append(start.Args, stop.Args...) {
		require.True(t, section8Names[argName(arg)], "argument %q is not in the Section 8 dictionary", arg)
	}
}

// RFC requirement: RFC8907-8.1-2 positive — start_time is the number of seconds since the epoch, the UTC reading of the clock.
func TestRFC8907StartTimeIsEpochSeconds(t *testing.T) {
	acct := newQueuedAccountant()
	before := time.Now().Unix()
	acct.CommandStart("alice", "192.0.2.1", "show version")
	after := time.Now().Unix()
	start := dequeue(t, acct)

	value, ok := argValue(start.Args, "start_time")
	require.True(t, ok, "start_time missing")
	seconds, err := strconv.ParseInt(value, 10, 64)
	require.NoError(t, err)
	require.GreaterOrEqual(t, seconds, before)
	require.LessOrEqual(t, seconds, after)
}

// RFC requirement: RFC8907-8.1-2 negative — with the process zone set five hours east of UTC the start_time does not move by that offset, and no timezone argument is emitted.
func TestRFC8907StartTimeIgnoresLocalZone(t *testing.T) {
	saved := time.Local
	t.Cleanup(func() { time.Local = saved })
	time.Local = time.FixedZone("east", 5*60*60)

	acct := newQueuedAccountant()
	before := time.Now().UTC().Unix()
	acct.CommandStart("alice", "192.0.2.1", "show version")
	after := time.Now().UTC().Unix()
	start := dequeue(t, acct)

	value, ok := argValue(start.Args, "start_time")
	require.True(t, ok, "start_time missing")
	seconds, err := strconv.ParseInt(value, 10, 64)
	require.NoError(t, err)
	require.Less(t, seconds, before+5*60*60, "start_time shifted by the local zone offset")
	require.GreaterOrEqual(t, seconds, before)
	require.LessOrEqual(t, seconds, after)
	_, hasZone := argValue(start.Args, "timezone")
	require.False(t, hasZone, "no timezone argument is emitted, so the value must be UTC")
}

// RFC requirement: RFC8907-8.3-1 positive — every Section 8.3 argument (task_id, start_time or stop_time) sits before the first Section 8.2 argument in START and STOP records.
func TestRFC8907AccountingArgumentsPrecedeAuthorizationOnes(t *testing.T) {
	acct := newQueuedAccountant()
	id := acct.CommandStart("alice", "192.0.2.1", "show bgp summary")
	start := dequeue(t, acct)
	acct.CommandStop(id, "alice", "192.0.2.1", "show bgp summary")
	stop := dequeue(t, acct)

	firstAuthz := argIndex(start.Args, "service")
	require.NotEqual(t, -1, firstAuthz)
	require.Less(t, argIndex(start.Args, "task_id"), firstAuthz, "task_id after service in START")
	require.Less(t, argIndex(start.Args, "start_time"), firstAuthz, "start_time after service in START")

	firstAuthz = argIndex(stop.Args, "service")
	require.NotEqual(t, -1, firstAuthz)
	require.Less(t, argIndex(stop.Args, "task_id"), firstAuthz, "task_id after service in STOP")
	require.Less(t, argIndex(stop.Args, "stop_time"), firstAuthz, "stop_time after service in STOP")
}

// RFC requirement: RFC8907-8.3-1 negative — no Section 8.2 argument (service, cmd, cmd-arg) precedes a Section 8.3 argument in a START record.
func TestRFC8907NoAuthorizationArgumentPrecedesAccountingOne(t *testing.T) {
	acct := newQueuedAccountant()
	acct.CommandStart("alice", "192.0.2.1", "show bgp summary")
	start := dequeue(t, acct)

	seenAuthz := false
	for _, arg := range start.Args {
		switch argName(arg) {
		case "service", "cmd", "cmd-arg":
			seenAuthz = true
		case "task_id", "start_time":
			require.False(t, seenAuthz, "accounting argument %q follows an authorization argument", arg)
		}
	}
}

// RFC requirement: RFC8907-8.3-2 positive — the STOP record for a command carries the same task_id as its START record.
func TestRFC8907StartAndStopShareTaskID(t *testing.T) {
	acct := newQueuedAccountant()
	id := acct.CommandStart("alice", "192.0.2.1", "show version")
	start := dequeue(t, acct)
	acct.CommandStop(id, "alice", "192.0.2.1", "show version")
	stop := dequeue(t, acct)

	startID, ok := argValue(start.Args, "task_id")
	require.True(t, ok)
	stopID, ok := argValue(stop.Args, "task_id")
	require.True(t, ok)
	require.Equal(t, id, startID, "START task_id is the id CommandStart returned")
	require.Equal(t, startID, stopID, "STOP task_id must match START")
}

// RFC requirement: RFC8907-8.3-2 negative — the START record of a second command does not carry the first command's task_id, so a STOP for the first never matches the second.
func TestRFC8907SecondCommandCarriesOtherTaskID(t *testing.T) {
	acct := newQueuedAccountant()
	first := acct.CommandStart("alice", "192.0.2.1", "show version")
	dequeue(t, acct)
	second := acct.CommandStart("alice", "192.0.2.1", "show version")
	start := dequeue(t, acct)

	secondID, ok := argValue(start.Args, "task_id")
	require.True(t, ok)
	require.Equal(t, second, secondID)
	require.NotEqual(t, first, secondID, "two events must not share a task_id")
}

// RFC requirement: RFC8907-8.3-3 positive — sixteen START records issued before any STOP carry sixteen distinct task_ids.
func TestRFC8907ActiveTaskIDsAreDistinct(t *testing.T) {
	acct := newQueuedAccountant()
	seen := make(map[string]bool)
	for range 16 {
		id := acct.CommandStart("alice", "192.0.2.1", "show version")
		start := dequeue(t, acct)
		value, ok := argValue(start.Args, "task_id")
		require.True(t, ok)
		require.Equal(t, id, value)
		require.False(t, seen[value], "task_id %s reused while active", value)
		seen[value] = true
	}
}

// RFC requirement: RFC8907-8.3-3 negative — a task_id already used in a START record never reappears in a later START, whether or not its STOP was sent.
func TestRFC8907TaskIDNeverReappears(t *testing.T) {
	acct := newQueuedAccountant()
	first := acct.CommandStart("alice", "192.0.2.1", "show version")
	dequeue(t, acct)
	acct.CommandStop(first, "alice", "192.0.2.1", "show version")
	dequeue(t, acct)

	for range 8 {
		id := acct.CommandStart("alice", "192.0.2.1", "show version")
		start := dequeue(t, acct)
		value, ok := argValue(start.Args, "task_id")
		require.True(t, ok)
		require.NotEqual(t, first, value, "a used task_id reappeared in a START record")
		require.NotEqual(t, first, id)
	}
}

// RFC requirement: RFC8907-8.3-5 positive — START and STOP records for a command carry service=shell, cmd=<verb> and one cmd-arg per further token.
func TestRFC8907CommandRecordsCarryServiceCmdAndArgs(t *testing.T) {
	acct := newQueuedAccountant()
	id := acct.CommandStart("alice", "192.0.2.1", "show bgp summary")
	start := dequeue(t, acct)
	acct.CommandStop(id, "alice", "192.0.2.1", "show bgp summary")
	stop := dequeue(t, acct)

	for _, rec := range []*AcctRequest{start, stop} {
		require.Contains(t, rec.Args, "service=shell")
		require.Contains(t, rec.Args, "cmd=show")
		require.Contains(t, rec.Args, "cmd-arg=bgp")
		require.Contains(t, rec.Args, "cmd-arg=summary")
	}
}

// RFC requirement: RFC8907-8.3-5 negative — an empty command line still yields a START record carrying service=shell and a cmd argument; a command record without them is never queued.
func TestRFC8907EmptyCommandRecordStillCarriesServiceAndCmd(t *testing.T) {
	acct := newQueuedAccountant()
	acct.CommandStart("alice", "192.0.2.1", "")
	start := dequeue(t, acct)
	require.Contains(t, start.Args, "service=shell")
	require.Contains(t, start.Args, "cmd=")
}
