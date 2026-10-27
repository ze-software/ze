package rpki

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/crashlog"
	"github.com/ze-software/ze/internal/core/taskguard"
)

// swapRPKIPanics replaces the RPKI panic boundary with one that logs JSON into the
// returned buffer, points crash records at a temporary directory, and restores both
// when t ends.
func swapRPKIPanics(t *testing.T) (*rpkiPanicTasks, *bytes.Buffer) {
	t.Helper()
	restoreCrashDir := crashlog.SetCrashDirForTest(t.TempDir())
	prior := rpkiPanics
	var logs bytes.Buffer
	guard := taskguard.New("rpki", slog.New(slog.NewJSONHandler(&logs, nil)))
	tasks := &rpkiPanicTasks{guard: guard, read: guard.Task("rtr-read", rtrReadRestart)}
	rpkiPanics = func() *rpkiPanicTasks { return tasks }
	t.Cleanup(func() {
		rpkiPanics = prior
		restoreCrashDir()
	})
	return tasks, &logs
}

// cacheAnswer is what a fake cache read back from the router after its End of Data: the
// bytes up to the router's close, and the error that ended the read.
type cacheAnswer struct {
	received []byte
	err      error
}

// syncingCache answers one Reset Query with a Cache Response, one IPv4 Prefix announcing
// prefix for origin, and an End of Data with a 900 s Refresh Interval, then reads what the
// router sends back until the router closes and hands it to answers.
func syncingCache(prefix [4]byte, origin uint32, answers chan<- cacheAnswer) func(net.Conn) {
	return func(conn net.Conn) {
		if err := conn.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
			return
		}
		query := make([]byte, pduHeaderLen)
		if _, err := io.ReadFull(conn, query); err != nil {
			return
		}
		version := query[0]

		resp := make([]byte, pduHeaderLen)
		resp[0], resp[1] = version, pduCacheResp
		binary.BigEndian.PutUint32(resp[4:8], pduHeaderLen)

		vrp := make([]byte, pduIPv4PrefixLen)
		vrp[0], vrp[1] = version, pduIPv4Prefix
		binary.BigEndian.PutUint32(vrp[4:8], pduIPv4PrefixLen)
		vrp[8], vrp[9], vrp[10] = 1, 24, 24 // announce, prefix length, max length
		copy(vrp[12:16], prefix[:])
		binary.BigEndian.PutUint32(vrp[16:20], origin)

		eod := make([]byte, pduEndOfDataLen)
		eod[0], eod[1] = version, pduEndOfData
		binary.BigEndian.PutUint32(eod[4:8], pduEndOfDataLen)
		binary.BigEndian.PutUint32(eod[8:12], 1)     // serial
		binary.BigEndian.PutUint32(eod[12:16], 900)  // refresh interval, seconds
		binary.BigEndian.PutUint32(eod[16:20], 600)  // retry interval, seconds
		binary.BigEndian.PutUint32(eod[20:24], 7200) // expire interval, seconds

		if _, err := conn.Write(bytes.Join([][]byte{resp, vrp, eod}, nil)); err != nil {
			answers <- cacheAnswer{err: err}
			return
		}
		received, err := io.ReadAll(conn)
		answers <- cacheAnswer{received: received, err: err}
	}
}

// TestRTRReadPanicContained proves the RTR read task's panic boundary through the cache
// group's real round (cacheGroup.poll, the call Run makes), against two real TCP caches.
// The preferred cache's sync panics; the round must survive it, send that cache an Error
// Report Internal Error with no encapsulated PDU, close its connection, start its Retry
// Interval, and take the load from the second cache. The panic must be logged, counted
// and crash-recorded.
//
// Two seams, neither added to production code: the session's existing onROAChange
// callback, which the plugin sets to its re-validation trigger and handlePDU calls after
// End of Data; and a nil ROA cache handed to the existing constructor, which panics
// inside applyEndOfData while the session lock and the shared data lease's lock are held.
// The second case also proves both locks are released: the connection close after the
// panic takes the session lock, and the second cache's End of Data takes the lease lock.
func TestRTRReadPanicContained(t *testing.T) {
	cases := []struct {
		name   string
		arm    func(preferred *RTRSession)
		roaNil bool
	}{
		{name: "the ROA change callback panics", arm: func(s *RTRSession) { s.onROAChange = func() { panic("boom") } }},
		{name: "a cache update panics under both locks", arm: func(*RTRSession) {}, roaNil: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tasks, logs := swapRPKIPanics(t)

			preferredAnswers := make(chan cacheAnswer, 1)
			preferredPort, _ := serveRTR(t, syncingCache([4]byte{192, 0, 2, 0}, 65001, preferredAnswers))
			secondAnswers := make(chan cacheAnswer, 1)
			secondPort, _ := serveRTR(t, syncingCache([4]byte{198, 51, 100, 0}, 65002, secondAnswers))

			stopCh := make(chan struct{})
			t.Cleanup(func() { close(stopCh) })
			preferredCache := newROACache()
			if tc.roaNil {
				preferredCache = nil
			}
			preferred := newTestRTRSession(t, "127.0.0.1", preferredPort, 1, "", preferredCache, newASPACache(), stopCh)
			tc.arm(preferred)
			secondCache := newROACache()
			second := newTestRTRSession(t, "127.0.0.1", secondPort, 2, "", secondCache, newASPACache(), stopCh)
			group := newCacheGroup([]*RTRSession{preferred, second}, stopCh)

			polled := make(chan time.Duration, 1)
			go func() { polled <- group.poll() }()
			var delay time.Duration
			select {
			case delay = <-polled:
			case <-time.After(15 * time.Second):
				t.Fatal("the cache group's round never finished: the contained panic left a lock held or the panic was not contained")
			}

			// The other session continues: the second cache took the load and its data is live.
			assert.Same(t, second, group.holder, "the second cache holds the load")
			assert.Equal(t, 900*time.Second, delay, "the next round follows the second cache's Refresh Interval")
			v4, v6 := secondCache.Count()
			assert.Equal(t, [2]int{1, 0}, [2]int{v4, v6}, "the second cache's VRP is installed")
			assert.True(t, time.Now().Before(group.retryAt[0]), "the preferred cache's Retry Interval started")

			// The preferred cache got an Error Report Internal Error, then the close.
			select {
			case answer := <-preferredAnswers:
				require.NoError(t, answer.err, "the router closed the preferred cache's connection")
				require.Len(t, answer.received, 16, "one Error Report with no encapsulated PDU and no text")
				assert.Equal(t, pduErrorRpt, answer.received[1], "PDU type Error Report")
				assert.Equal(t, errInternalError, binary.BigEndian.Uint16(answer.received[2:4]), "error code 1, Internal Error")
				assert.Equal(t, uint32(16), binary.BigEndian.Uint32(answer.received[4:8]), "PDU length")
				assert.Equal(t, uint32(0), binary.BigEndian.Uint32(answer.received[8:12]), "Length of Encapsulated PDU is zero")
			case <-time.After(10 * time.Second):
				t.Fatal("the preferred cache never saw its connection close")
			}
			select {
			case answer := <-secondAnswers:
				require.NoError(t, answer.err)
				assert.Empty(t, answer.received, "the second cache, which synced, got no Error Report")
			case <-time.After(10 * time.Second):
				t.Fatal("the second cache never saw its connection close")
			}

			requireRTRReadContained(t, tasks, logs, "rtr cache 127.0.0.1:"+strconv.Itoa(int(preferredPort)))
		})
	}
}

// requireRTRReadContained asserts exactly one contained rtr-read panic with value "boom" on
// unit: one ERROR line naming the component, the task, the unit and the End of Data in
// flight, one count, and one recovered crash record.
func requireRTRReadContained(t *testing.T, tasks *rpkiPanicTasks, logs *bytes.Buffer, unit string) {
	t.Helper()
	count, declared := tasks.guard.Panics("rtr-read")
	assert.True(t, declared)
	assert.Equal(t, uint64(1), count, "the panic was counted once")

	lines := strings.Split(strings.TrimSpace(logs.String()), "\n")
	require.Len(t, lines, 1, "one log line: %q", logs.String())
	var line map[string]any
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &line))
	want := map[string]string{
		"level": "ERROR", "component": "rpki", "task": "rtr-read", "unit": unit,
		"message-type": "End of Data",
	}
	for key, value := range want {
		assert.Equal(t, value, line[key], "log field %s", key)
	}
	panicValue, _ := line["panic"].(string)
	assert.NotEmpty(t, panicValue, "the log line carries the panic value")

	crashes := crashlog.ListCrashes()
	require.Len(t, crashes, 1, "one crash record")
	assert.Equal(t, crashlog.KindRecovered, crashes[0].Kind)
}

// TestRTRReadPanicSkipsErrorReportForErrorReport proves the Error Report a contained panic
// sends is withheld when the PDU in flight was itself an Error Report, and when no PDU was
// in flight. RFC 8210 Section 5.11: "An Error Report PDU MUST NOT be sent for an Error Report
// PDU."
func TestRTRReadPanicSkipsErrorReportForErrorReport(t *testing.T) {
	cases := []struct {
		name     string
		inFlight pduInFlight
		want     int
	}{
		{name: "End of Data in flight", inFlight: pduInFlight{header: rTRHeader{Type: pduEndOfData}, parsed: true}, want: 16},
		{name: "Error Report in flight", inFlight: pduInFlight{header: rTRHeader{Type: pduErrorRpt}, parsed: true}},
		{name: "no PDU in flight", inFlight: pduInFlight{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			router, cache := net.Pipe()
			s := newTestRTRSession(t, "127.0.0.1", 323, 1, "", newROACache(), newASPACache(), make(chan struct{}))
			received := make(chan []byte, 1)
			go func() {
				got, err := io.ReadAll(cache)
				if err != nil && !errors.Is(err, io.ErrClosedPipe) {
					t.Errorf("cache read: %v", err)
				}
				received <- got
			}()
			s.reportInternalError(router, tc.inFlight)
			require.NoError(t, router.Close())
			assert.Len(t, <-received, tc.want)
		})
	}
}
