// Design: docs/architecture/aaa-tacacs.md -- bounded command accounting delivery.
package tacacs

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"
)

// newAccountingSink answers a bounded number of accounting requests. Each
// request is read and de-obfuscated by the same socket helper as client tests.
func newAccountingSink(t *testing.T, count int) (*TacacsClient, <-chan []byte) {
	t.Helper()
	key := []byte("accounting-test-shared-secret-32")
	received := make(chan []byte, count)
	srv := &testTacacsServer{
		listener: listenTCP(t),
		key:      key,
		replyFn: func(hdr PacketHeader, body []byte) []byte {
			if hdr.Type != typeAccounting {
				return nil
			}
			received <- append([]byte(nil), body...)
			return []byte{0, 0, 0, 0, AcctStatusSuccess}
		},
	}
	t.Cleanup(srv.close)
	go func() {
		for range count {
			srv.serve()
		}
	}()
	client := NewTacacsClient(TacacsClientConfig{
		Servers: []TacacsServer{{Address: srv.addr(), Key: key}},
		Timeout: time.Second,
	})
	t.Cleanup(client.Close)
	return client, received
}

// RFC requirement: RFC8907-8.3-4 positive -- queue saturation retains every entered command and Stop sends all accepted START packets to the configured server.
func TestRFC8907AccountingSaturationRetainsEveryStart(t *testing.T) {
	const commands = 65
	client, received := newAccountingSink(t, commands)
	acct := NewTacacsAccountant(client, nil)
	t.Cleanup(acct.Stop)
	for i := range commands - 1 {
		acct.CommandStart("alice", "192.0.2.1", fmt.Sprintf("show record-%d", i))
	}
	entered := make(chan struct{})
	accepted := make(chan struct{})
	go func() {
		close(entered)
		acct.CommandStart("alice", "192.0.2.1", "show record-64")
		close(accepted)
	}()
	<-entered
	select {
	case <-accepted:
		t.Fatal("a full queue returned before a worker could accept the record")
	case <-time.After(20 * time.Millisecond):
	}
	acct.Start()
	select {
	case <-accepted:
	case <-time.After(5 * time.Second):
		t.Fatal("the queued command never resumed")
	}
	acct.Stop()
	for i := range commands {
		select {
		case body := <-received:
			if body[0] != AcctFlagStart || !bytes.Contains(body, []byte(fmt.Sprintf("cmd-arg=record-%d", i))) {
				t.Fatalf("command %d was reordered or omitted: %x", i, body)
			}
		default:
			t.Fatalf("only %d of %d commands reached the server", i, commands)
		}
	}
	if acct.DropCount() != 0 {
		t.Fatalf("active accountant dropped %d records", acct.DropCount())
	}
}

// RFC requirement: RFC8907-8.3-4 negative -- shutdown cannot discard accepted START records, even when the worker had not been started yet.
func TestRFC8907AccountingStopDrainsAcceptedStarts(t *testing.T) {
	client, received := newAccountingSink(t, 3)
	acct := NewTacacsAccountant(client, nil)
	for range 3 {
		acct.CommandStart("alice", "192.0.2.1", "show version")
	}
	acct.Stop()
	acct.Stop()
	acct.Start()
	for i := range 3 {
		select {
		case body := <-received:
			if body[0] != AcctFlagStart || !bytes.Contains(body, []byte("cmd=show")) {
				t.Fatalf("record %d is not a command START: %x", i, body)
			}
		default:
			t.Fatalf("shutdown discarded record %d", i)
		}
	}
}

// Shutdown must wait for an enqueue already blocked on the full queue, drain
// every accepted record, and count rather than panic on concurrent late calls.
func TestAccountantEnqueueConcurrentStop(t *testing.T) {
	const queued = 64
	const writers = 16
	const perWriter = 8
	const commands = 1 + queued + 1 + writers*perWriter
	key := []byte("concurrent-accounting-shared-key")
	received := make(chan []byte, commands)
	firstReceived := make(chan struct{})
	release := make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	first := true
	srv := &testTacacsServer{
		listener: listenTCP(t),
		key:      key,
		replyFn: func(_ PacketHeader, body []byte) []byte {
			received <- append([]byte(nil), body...)
			if first {
				first = false
				close(firstReceived)
				<-release
			}
			return []byte{0, 0, 0, 0, AcctStatusSuccess}
		},
	}
	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		for range commands {
			srv.serve()
		}
	}()
	t.Cleanup(func() {
		unblock()
		srv.close()
		<-serverDone
	})
	client := NewTacacsClient(TacacsClientConfig{
		Servers: []TacacsServer{{Address: srv.addr(), Key: key}},
		Timeout: 10 * time.Second,
	})
	t.Cleanup(client.Close)
	acct := NewTacacsAccountant(client, slog.New(slog.NewTextHandler(io.Discard, nil)))
	t.Cleanup(func() {
		unblock()
		acct.Stop()
	})
	acct.Start()
	submitted := make(map[string]string, commands)
	guaranteed := make(map[string]bool, queued+2)
	start := func(label string) string {
		return acct.CommandStartArgs("alice", "192.0.2.1", []string{"show", label})
	}
	firstID := start("first")
	submitted[firstID] = "first"
	guaranteed[firstID] = true
	select {
	case <-firstReceived:
	case <-time.After(5 * time.Second):
		t.Fatal("initial accounting request never reached the server")
	}
	for i := range queued {
		label := fmt.Sprintf("queued-%d", i)
		id := start(label)
		submitted[id] = label
		guaranteed[id] = true
	}
	type submission struct {
		id    string
		label string
	}
	completed := make(chan submission, writers*perWriter+1)
	go func() { completed <- submission{start("blocked"), "blocked"} }()
	wait := func(label string, ready func() bool) {
		t.Helper()
		deadline := time.NewTimer(5 * time.Second)
		defer deadline.Stop()
		ticker := time.NewTicker(time.Millisecond)
		defer ticker.Stop()
		for !ready() {
			select {
			case <-ticker.C:
			case <-deadline.C:
				t.Fatalf("timed out waiting for %s", label)
			}
		}
	}
	// Use the lifetime lock only as a scheduling barrier: the sole active
	// caller now holds its reader lock while waiting for queue capacity.
	wait("enqueue blocked on queue capacity", func() bool {
		if acct.mu.TryLock() {
			acct.mu.Unlock()
			return false
		}
		return true
	})
	stopped := make(chan struct{})
	go func() {
		acct.Stop()
		close(stopped)
	}()
	// The pending shutdown writer excludes new readers while the blocked
	// enqueue still owns its reader lock; neither can finish until release.
	wait("Stop overlapping the blocked enqueue", func() bool {
		if acct.mu.TryRLock() {
			acct.mu.RUnlock()
			return false
		}
		return true
	})
	for writer := range writers {
		go func() {
			for i := range perWriter {
				label := fmt.Sprintf("writer-%d-%d", writer, i)
				completed <- submission{start(label), label}
			}
		}()
	}
	unblock()
	for range writers*perWriter + 1 {
		select {
		case result := <-completed:
			if _, duplicate := submitted[result.id]; duplicate {
				t.Fatalf("task ID %q was reused", result.id)
			}
			submitted[result.id] = result.label
			if result.label == "blocked" {
				guaranteed[result.id] = true
			}
		case <-time.After(10 * time.Second):
			t.Fatal("concurrent accounting caller did not return")
		}
	}
	select {
	case <-stopped:
	case <-time.After(10 * time.Second):
		t.Fatal("Stop did not drain accepted requests")
	}
	delivered := make(map[string]bool, len(received))
	for len(received) != 0 {
		body := <-received
		if len(body) == 0 || body[0] != AcctFlagStart {
			t.Fatalf("not an accounting START: %x", body)
		}
		args, err := decodeAuthorRequestArgs(body[1:])
		if err != nil {
			t.Fatal(err)
		}
		id, _ := argValue(args, "task_id")
		label, _ := argValue(args, "cmd-arg")
		want, issued := submitted[id]
		if !issued || label != want || delivered[id] {
			t.Fatalf("unexpected, corrupted or duplicate delivery: task=%q label=%q want=%q", id, label, want)
		}
		delivered[id] = true
	}
	for id := range guaranteed {
		if !delivered[id] {
			t.Fatalf("Stop lost accepted task %q (%s)", id, submitted[id])
		}
	}
	if uint64(len(delivered))+acct.DropCount() != uint64(len(submitted)) {
		t.Fatalf("submitted=%d delivered=%d refused=%d", len(submitted), len(delivered), acct.DropCount())
	}
}

// Retired callers are observable through the drop counter without copying a
// command, which can contain a configuration secret, into the local log.
func TestAccountantRefusesAfterStopWithoutLoggingCommand(t *testing.T) {
	var logs bytes.Buffer
	acct := NewTacacsAccountant(NewTacacsClient(TacacsClientConfig{}), slog.New(slog.NewTextHandler(&logs, nil)))
	acct.Stop()
	command := "set system authentication tacacs server 192.0.2.1 key private-value"
	id := acct.CommandStart("alice", "192.0.2.1", command)
	acct.CommandStop(id, "alice", "192.0.2.1", command)
	if acct.DropCount() != 2 {
		t.Fatalf("post-shutdown refusals = %d, want 2", acct.DropCount())
	}
	if strings.Contains(logs.String(), "private-value") {
		t.Fatal("accounting refusal log exposed a command credential")
	}
}

// Accountant generations coexist during reload. A new generation must not
// reuse an ID while a command on the old generation is active.
func TestAccountantTaskIDsSpanGenerations(t *testing.T) {
	first, second := newQueuedAccountant(), newQueuedAccountant()
	one := first.CommandStart("alice", "192.0.2.1", "show version")
	two := second.CommandStart("alice", "192.0.2.1", "show version")
	if one == two {
		t.Fatalf("concurrent accountant generations reused task_id %s", one)
	}
}

// Typed accounting must keep a value containing whitespace in one cmd-arg.
func TestAccountantTypedArgumentsPreserveBoundaries(t *testing.T) {
	acct := newQueuedAccountant()
	tokens := []string{"set", "description", "two words", `quote"inside`, `slash\inside`}
	id := acct.CommandStartArgs("alice", "192.0.2.1", tokens)
	start := dequeue(t, acct)
	acct.CommandStopArgs(id, "alice", "192.0.2.1", tokens)
	stop := dequeue(t, acct)
	for _, record := range []*AcctRequest{start, stop} {
		for _, arg := range []string{"cmd=set", "cmd-arg=description", "cmd-arg=two words", `cmd-arg=quote\"inside`, `cmd-arg=slash\\inside`} {
			found := false
			for _, actual := range record.Args {
				found = found || actual == arg
			}
			if !found {
				t.Fatalf("accounting lost argument boundary %q: %q", arg, record.Args)
			}
		}
	}
}
