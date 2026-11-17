package main

import (
	"bufio"
	"errors"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// fakeBehavior is how the fake server answers.
type fakeBehavior int

const (
	fakeBehaviorUnspecified fakeBehavior = iota
	// fakeKeeps answers as a Redis server does: GET reads what SET stored.
	fakeKeeps
	// fakeForgets answers SET with +OK and stores nothing, so GET misses.
	fakeForgets
	// fakeNeedsAuth answers every command with the error a server that
	// requires a password writes.
	fakeNeedsAuth
	// fakeRefusesSave answers as fakeKeeps does, and SAVE with an error.
	fakeRefusesSave
)

// The fake's replies that are not a stored value.
const (
	// fakeNoAuth is the reply of a server that requires a password.
	fakeNoAuth = "-NOAUTH Authentication required.\r\n"
	// fakeSaveRefused is the reply of a server that cannot save now.
	fakeSaveRefused = "-ERR Background save already in progress\r\n"
	fakeOK          = "+OK\r\n"
)

// The run ids of the fake's two processes: the one that saves, and the one
// started since.
const (
	fakeRunFirst  = "1f0a2b3c4d5e6f708192a3b4c5d6e7f801234567"
	fakeRunSecond = "9e8d7c6b5a4f3e2d1c0b9a8f7e6d5c4b3a291807"
)

// fakeDisk is what the fake server's processes share: the data a save
// wrote. Each fakeServer over it stands for one process. Safe for concurrent
// use: the fake's goroutine writes it and the test reads it.
type fakeDisk struct {
	mu     sync.Mutex
	stored map[string]string
	// sent is every command the fake was asked, in order.
	sent []string
}

// newFakeDisk answers an empty disk.
func newFakeDisk() *fakeDisk {
	return &fakeDisk{stored: map[string]string{}}
}

// value answers what the disk holds under key.
func (d *fakeDisk) value(key string) (string, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	value, found := d.stored[key]
	return value, found
}

// asked answers every command the fake was asked, joined by "; ".
func (d *fakeDisk) asked() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return strings.Join(d.sent, "; ")
}

// fakeServer is one process of the fake: its behavior, its run id and the
// disk it reads and writes.
type fakeServer struct {
	behavior fakeBehavior
	runID    string
	disk     *fakeDisk
}

// fakeRedis answers the commands of one connection accepted on listener
// until the client closes it. It parses each request as the RESP array of
// bulk strings a client sends, with its own reader, so a request the probe
// encodes wrong is not understood: the fake closes the connection.
func fakeRedis(t *testing.T, listener net.Listener, server fakeServer) {
	t.Helper()
	connection, err := listener.Accept()
	if err != nil {
		return // the listener closed at the test's end
	}
	defer connection.Close() //nolint:errcheck // the test judges the client's verdict
	reader := bufio.NewReader(connection)
	for {
		args, err := fakeReadCommand(reader)
		if err != nil {
			return
		}
		if _, err := connection.Write([]byte(server.reply(args))); err != nil {
			t.Errorf("fake write: %v", err)
			return
		}
	}
}

// reply answers one command.
func (s fakeServer) reply(args []string) string {
	switch s.behavior {
	case fakeNeedsAuth:
		return fakeNoAuth
	case fakeKeeps, fakeForgets, fakeRefusesSave:
		// All answer as a server does; they differ at SET and at SAVE.
	case fakeBehaviorUnspecified:
		panic("BUG: the fake was given no behavior")
	}
	s.disk.mu.Lock()
	defer s.disk.mu.Unlock()
	command := strings.Join(args, " ")
	s.disk.sent = append(s.disk.sent, command)
	if command == respCommandPing {
		return "+PONG\r\n"
	}
	if command == respCommandSave {
		if s.behavior == fakeRefusesSave {
			return fakeSaveRefused
		}
		return fakeOK
	}
	if command == respCommandInfo+" "+respInfoSection {
		return fakeBulk("# Server\r\nredis_version:8.0.0\r\nrun_id:" + s.runID + "\r\ntcp_port:6379\r\n")
	}
	if len(args) == 3 && args[0] == respCommandSet {
		if s.behavior != fakeForgets {
			s.disk.stored[args[1]] = args[2]
		}
		return fakeOK
	}
	if len(args) == 2 && args[0] == respCommandGet {
		value, found := s.disk.stored[args[1]]
		if !found {
			return "$-1\r\n"
		}
		return fakeBulk(value)
	}
	return "-ERR unknown command " + strconv.Quote(command) + "\r\n"
}

// fakeBulk encodes value as a bulk string.
func fakeBulk(value string) string {
	return "$" + strconv.Itoa(len(value)) + respCRLF + value + respCRLF
}

// fakeReadCommand reads one RESP array of bulk strings.
func fakeReadCommand(reader *bufio.Reader) ([]string, error) {
	header, err := fakeLine(reader)
	if err != nil {
		return nil, err
	}
	countText, isArray := strings.CutPrefix(header, "*")
	if !isArray {
		return nil, errors.New("not an array: " + header)
	}
	count, err := strconv.Atoi(countText)
	if err != nil {
		return nil, err
	}
	args := make([]string, 0, count)
	for range count {
		lengthLine, err := fakeLine(reader)
		if err != nil {
			return nil, err
		}
		lengthText, isBulk := strings.CutPrefix(lengthLine, "$")
		if !isBulk {
			return nil, errors.New("not a bulk string: " + lengthLine)
		}
		length, err := strconv.Atoi(lengthText)
		if err != nil {
			return nil, err
		}
		body := make([]byte, length+2)
		if _, err := io.ReadFull(reader, body); err != nil {
			return nil, err
		}
		args = append(args, string(body[:length]))
	}
	return args, nil
}

// fakeLine reads one line and drops its CRLF.
func fakeLine(reader *bufio.Reader) (string, error) {
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	text, ended := strings.CutSuffix(line, respCRLF)
	if !ended {
		return "", errors.New("a line not ended by CRLF")
	}
	return text, nil
}

// talkToFake starts one process of the fake and answers what respTalk says
// of script over one connection to it.
func talkToFake(t *testing.T, server fakeServer, script respScript) error {
	t.Helper()
	listener, port := listenLoopback(t)
	go fakeRedis(t, listener, server)
	connection, err := dialOnce(net.JoinHostPort("127.0.0.1", port))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer connection.Close() //nolint:errcheck // the verdict is respTalk's
	return respTalk(connection, script)
}

// requireFailure fails the test unless err is a failure whose text starts
// with prefix.
func requireFailure(t *testing.T, err error, prefix string) {
	t.Helper()
	if err == nil {
		t.Fatalf("respTalk passed, want a failure that starts with %q", prefix)
	}
	if !strings.HasPrefix(err.Error(), prefix) {
		t.Fatalf("respTalk failed for another reason than %q: %v", prefix, err)
	}
}

// requireOnDisk fails the test unless the disk holds want under key.
func requireOnDisk(t *testing.T, disk *fakeDisk, key, want string) {
	t.Helper()
	value, found := disk.value(key)
	if !found {
		t.Fatalf("the disk holds nothing under %s, want %q", key, want)
	}
	if value != want {
		t.Fatalf("the disk holds %q under %s, want %q", value, key, want)
	}
}

// TestRespSpeaksToAServer: against a server that keeps what SET stores,
// resp connects, and PING, SET and GET each get the reply it requires. A
// command encoded wrong is not understood by the fake, and a GET before the
// SET misses, so either fails this test.
func TestRespSpeaksToAServer(t *testing.T) {
	listener, port := listenLoopback(t)
	go fakeRedis(t, listener, fakeServer{behavior: fakeKeeps, runID: fakeRunFirst, disk: newFakeDisk()})
	if !resp("127.0.0.1", port, respStores) {
		t.Fatal("resp failed against a server that answers the three commands")
	}
}

// TestRespFailsWhenGetMisses: a server that answers SET and stores nothing
// fails the GET, so the GET's reply is judged, not only its kind.
func TestRespFailsWhenGetMisses(t *testing.T) {
	err := talkToFake(t, fakeServer{behavior: fakeForgets, runID: fakeRunFirst, disk: newFakeDisk()}, respStores)
	requireFailure(t, err, "GET "+respKey+" answered $-1")
}

// TestRespFailsOnAnErrorReply: a server that answers PING with an error
// reply fails at PING, and the failure quotes the server's error.
func TestRespFailsOnAnErrorReply(t *testing.T) {
	err := talkToFake(t, fakeServer{behavior: fakeNeedsAuth, runID: fakeRunFirst, disk: newFakeDisk()}, respStores)
	requireFailure(t, err, "PING answered -NOAUTH")
}

// TestRespSavesThenReadsBackFromAnotherProcess: the save script stores the
// value and the run id of the process it spoke to, and ends with SAVE; the
// kept script, against a second process over the same disk, reads both back
// and sends no write. The disk is read after each script, so what the test
// requires is what the fake was asked, not what the probe printed.
func TestRespSavesThenReadsBackFromAnotherProcess(t *testing.T) {
	disk := newFakeDisk()
	if err := talkToFake(t, fakeServer{behavior: fakeKeeps, runID: fakeRunFirst, disk: disk}, respSaves); err != nil {
		t.Fatalf("the save script failed against a server that saves: %v", err)
	}
	requireOnDisk(t, disk, respKey, respValue)
	requireOnDisk(t, disk, respRunIDKey, fakeRunFirst)
	saved := "PING; SET ze-key ze-value; GET ze-key; INFO server; SET ze-run-id " + fakeRunFirst + "; SAVE"
	if asked := disk.asked(); asked != saved {
		t.Fatalf("the save script asked %q, want %q", asked, saved)
	}
	if err := talkToFake(t, fakeServer{behavior: fakeKeeps, runID: fakeRunSecond, disk: disk}, respKept); err != nil {
		t.Fatalf("the kept script failed against another process over the same disk: %v", err)
	}
	kept := saved + "; PING; GET ze-key; GET ze-run-id; INFO server"
	if asked := disk.asked(); asked != kept {
		t.Fatalf("after the kept script the fake was asked %q, want %q", asked, kept)
	}
}

// TestRespSaveFailsWhenSaveIsRefused: a server that answers SAVE with an
// error fails the save script at SAVE, so the script proves the write to
// disk and not only the SET.
func TestRespSaveFailsWhenSaveIsRefused(t *testing.T) {
	err := talkToFake(t, fakeServer{behavior: fakeRefusesSave, runID: fakeRunFirst, disk: newFakeDisk()}, respSaves)
	requireFailure(t, err, "SAVE answered -ERR Background save already in progress")
}

// TestRespKeptFailsWhenTheValueIsGone: a process whose disk holds nothing
// fails the kept script at its GET, which no SET precedes.
func TestRespKeptFailsWhenTheValueIsGone(t *testing.T) {
	err := talkToFake(t, fakeServer{behavior: fakeKeeps, runID: fakeRunSecond, disk: newFakeDisk()}, respKept)
	requireFailure(t, err, "GET "+respKey+" answered $-1")
}

// TestRespKeptFailsWhenTheRunIDWasNotStored: a disk that holds the value
// and no run id fails the kept script, which cannot then tell the two
// processes apart.
func TestRespKeptFailsWhenTheRunIDWasNotStored(t *testing.T) {
	disk := newFakeDisk()
	if err := talkToFake(t, fakeServer{behavior: fakeKeeps, runID: fakeRunFirst, disk: disk}, respStores); err != nil {
		t.Fatalf("the store script failed: %v", err)
	}
	err := talkToFake(t, fakeServer{behavior: fakeKeeps, runID: fakeRunSecond, disk: disk}, respKept)
	requireFailure(t, err, "GET "+respRunIDKey+" answered $-1")
}

// TestRespKeptFailsAgainstTheProcessThatSaved: the kept script against the
// same process that saved, the case of a restart that restarted nothing,
// fails on the run id although the value reads back.
func TestRespKeptFailsAgainstTheProcessThatSaved(t *testing.T) {
	disk := newFakeDisk()
	if err := talkToFake(t, fakeServer{behavior: fakeKeeps, runID: fakeRunFirst, disk: disk}, respSaves); err != nil {
		t.Fatalf("the save script failed: %v", err)
	}
	err := talkToFake(t, fakeServer{behavior: fakeKeeps, runID: fakeRunFirst, disk: disk}, respKept)
	requireFailure(t, err, "the server's run id is "+fakeRunFirst+", the run id of the server that saved")
}

// TestRespScriptOfReadsTheWord: each word of the command line names its
// script, and any other word is an error, never the default script.
func TestRespScriptOfReadsTheWord(t *testing.T) {
	words := map[string]respScript{respWordSave: respSaves, respWordKept: respKept}
	for word, want := range words {
		got, err := respScriptOf(word)
		if err != nil {
			t.Errorf("respScriptOf(%q): %v", word, err)
			continue
		}
		if got != want {
			t.Errorf("respScriptOf(%q) = %d, want %d", word, got, want)
		}
	}
	for _, word := range []string{"", "stores", "SAVE"} {
		if got, err := respScriptOf(word); err == nil {
			t.Errorf("respScriptOf(%q) = %d, want an error", word, got)
		}
	}
}

// TestRespCommandEncodes: the request is the RESP array of bulk strings,
// byte for byte.
func TestRespCommandEncodes(t *testing.T) {
	got := string(respCommand([]string{"SET", respKey, respValue}))
	want := "*3\r\n$3\r\nSET\r\n$6\r\nze-key\r\n$8\r\nze-value\r\n"
	if got != want {
		t.Fatalf("respCommand = %q, want %q", got, want)
	}
}

// TestReadRespJudgesTheReply: each reply kind reads as its kind and text,
// and each malformed reply is an error rather than a reply.
func TestReadRespJudgesTheReply(t *testing.T) {
	read := []struct {
		wire string
		want respReply
	}{
		{"+PONG\r\n", respReply{kind: respSimple, text: "PONG"}},
		{"-ERR no\r\n", respReply{kind: respError, text: "ERR no"}},
		{":7\r\n", respReply{kind: respInteger, text: "7"}},
		{"$8\r\nze-value\r\n", respReply{kind: respBulk, text: "ze-value"}},
		{"$0\r\n\r\n", respReply{kind: respBulk}},
		{"$-1\r\n", respReply{kind: respBulk, null: true}},
	}
	for _, c := range read {
		got, err := readResp(bufio.NewReaderSize(strings.NewReader(c.wire), respLineMax))
		if err != nil {
			t.Errorf("readResp(%q): %v", c.wire, err)
			continue
		}
		if got != c.want {
			t.Errorf("readResp(%q) = %v, want %v", c.wire, got, c.want)
		}
	}
	refused := []string{
		"",
		respCRLF,
		"+PONG\n",
		"*1\r\n",
		"$x\r\n",
		"$-2\r\n",
		"$" + strconv.Itoa(respBulkMax+1) + "\r\n",
		"$8\r\nze-val",
		"$8\r\nze-valueXX",
		"+" + strings.Repeat("a", respLineMax) + "\r\n",
	}
	for _, wire := range refused {
		if got, err := readResp(bufio.NewReaderSize(strings.NewReader(wire), respLineMax)); err == nil {
			t.Errorf("readResp(%q) = %v, want an error", wire, got)
		}
	}
}
