package main

import (
	"bufio"
	"errors"
	"io"
	"net"
	"strconv"
	"strings"
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
)

// fakeNoAuth is the reply of a server that requires a password.
const fakeNoAuth = "-NOAUTH Authentication required.\r\n"

// fakeRedis answers the commands of one connection accepted on listener
// until the client closes it. It parses each request as the RESP array of
// bulk strings a client sends, with its own reader, so a request the probe
// encodes wrong is not understood: the fake closes the connection.
func fakeRedis(t *testing.T, listener net.Listener, behavior fakeBehavior) {
	t.Helper()
	connection, err := listener.Accept()
	if err != nil {
		return // the listener closed at the test's end
	}
	defer connection.Close() //nolint:errcheck // the test judges the client's verdict
	reader := bufio.NewReader(connection)
	stored := map[string]string{}
	for {
		args, err := fakeReadCommand(reader)
		if err != nil {
			return
		}
		if _, err := connection.Write([]byte(fakeReply(args, stored, behavior))); err != nil {
			t.Errorf("fake write: %v", err)
			return
		}
	}
}

// fakeReply answers one command.
func fakeReply(args []string, stored map[string]string, behavior fakeBehavior) string {
	switch behavior {
	case fakeNeedsAuth:
		return fakeNoAuth
	case fakeKeeps, fakeForgets:
		// Both answer as a server does; they differ only at SET.
	case fakeBehaviorUnspecified:
		panic("BUG: the fake was given no behavior")
	}
	command := strings.Join(args, " ")
	if command == "PING" {
		return "+PONG\r\n"
	}
	if len(args) == 3 && args[0] == "SET" {
		if behavior == fakeKeeps {
			stored[args[1]] = args[2]
		}
		return "+OK\r\n"
	}
	if len(args) == 2 && args[0] == "GET" {
		value, found := stored[args[1]]
		if !found {
			return "$-1\r\n"
		}
		return "$" + strconv.Itoa(len(value)) + respCRLF + value + respCRLF
	}
	return "-ERR unknown command " + strconv.Quote(command) + "\r\n"
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

// talkToFake starts the fake with behavior and answers what respTalk says
// over one connection to it.
func talkToFake(t *testing.T, behavior fakeBehavior) error {
	t.Helper()
	listener, port := listenLoopback(t)
	go fakeRedis(t, listener, behavior)
	connection, err := dialOnce(net.JoinHostPort("127.0.0.1", port))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer connection.Close() //nolint:errcheck // the verdict is respTalk's
	return respTalk(connection)
}

// TestRespSpeaksToAServer: against a server that keeps what SET stores,
// resp connects, and PING, SET and GET each get the reply it requires. A
// command encoded wrong is not understood by the fake, and a GET before the
// SET misses, so either fails this test.
func TestRespSpeaksToAServer(t *testing.T) {
	listener, port := listenLoopback(t)
	go fakeRedis(t, listener, fakeKeeps)
	if !resp("127.0.0.1", port) {
		t.Fatal("resp failed against a server that answers the three commands")
	}
}

// TestRespFailsWhenGetMisses: a server that answers SET and stores nothing
// fails the GET, so the GET's reply is judged, not only its kind.
func TestRespFailsWhenGetMisses(t *testing.T) {
	err := talkToFake(t, fakeForgets)
	if err == nil {
		t.Fatal("respTalk passed a server whose GET answered the null bulk string")
	}
	if !strings.HasPrefix(err.Error(), "GET "+respKey+" answered $-1") {
		t.Fatalf("respTalk failed for another reason: %v", err)
	}
}

// TestRespFailsOnAnErrorReply: a server that answers PING with an error
// reply fails at PING, and the failure quotes the server's error.
func TestRespFailsOnAnErrorReply(t *testing.T) {
	err := talkToFake(t, fakeNeedsAuth)
	if err == nil {
		t.Fatal("respTalk passed a server that answered every command with an error")
	}
	if !strings.HasPrefix(err.Error(), "PING answered -NOAUTH") {
		t.Fatalf("respTalk failed for another reason: %v", err)
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
