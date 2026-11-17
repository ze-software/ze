// Design: docs/architecture/container/runtime.md -- the test store and its fixture image
// Related: main.go -- the resp mode's dispatch
// Related: serve.go -- the bounded dial the resp mode shares with published
// Related: stages.go -- the fetch, running and diagnose modes run before it

package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// The bounds of the resp mode. A real image's service starts some time after
// its container does, so the first attempts can meet a container whose
// service does not listen yet: the connection is retried, and the mode says
// how long it waited.
const (
	respDialStep  = 500 * time.Millisecond
	respDialSteps = 120 // a refusal answers at once: about 60 s; with dialTimeout, at most 180 s when every attempt times out
	// respReplyTimeout bounds one exchange: the write and its reply.
	respReplyTimeout = 5 * time.Second
	// respLineMax bounds one reply line, and respBulkMax one bulk string. The
	// replies the mode asks for are a few bytes, so anything longer is not
	// the answer.
	respLineMax = 512
	respBulkMax = 64 << 10
)

// respCRLF ends every RESP line.
const respCRLF = "\r\n"

// respNullBulkLength is the length the null bulk string carries: a GET of a
// key the server does not hold.
const respNullBulkLength = -1

// The key and value the exchanges write and read back.
const (
	respKey   = "ze-key"
	respValue = "ze-value"
)

// respKind is the type byte that starts a RESP reply.
type respKind byte

// The reply kinds of RESP2, the protocol a Redis server speaks to a client
// that has not asked for another.
const (
	respSimple  respKind = '+'
	respError   respKind = '-'
	respInteger respKind = ':'
	respBulk    respKind = '$'
)

// respReply is one reply read off a connection: its kind and its text, the
// bytes of the string for a bulk. null marks the null bulk string, which has
// no text.
type respReply struct {
	kind respKind
	text string
	null bool
}

// String answers the reply as it reads on the wire, for the probe's lines.
func (r respReply) String() string {
	if r.kind != respBulk {
		return string(rune(r.kind)) + r.text
	}
	if r.null {
		return "$" + strconv.Itoa(respNullBulkLength) + " (the null bulk string)"
	}
	return "$" + strconv.Itoa(len(r.text)) + " " + strconv.Quote(r.text)
}

// respExchange is one command the mode sends and the reply it requires.
type respExchange struct {
	args []string
	want respReply
}

// respExchanges are the exchanges, in order: the server answers, takes a
// write, and reads it back, so the reply to GET proves the server kept what
// SET stored rather than echoing.
var respExchanges = []respExchange{
	{args: []string{"PING"}, want: respReply{kind: respSimple, text: "PONG"}},
	{args: []string{"SET", respKey, respValue}, want: respReply{kind: respSimple, text: "OK"}},
	{args: []string{"GET", respKey}, want: respReply{kind: respBulk, text: respValue}},
}

// resp runs on the client's side: it dials ADDRESS:PORT until a connection
// opens, within a bounded wait it reports, then speaks the Redis protocol
// over that one connection. Every failure line names its stage: network when
// no attempt was answered, port when the last attempt was refused, protocol
// when the connection opened and a reply was not the one required.
func resp(address, port string) bool {
	target := net.JoinHostPort(address, port)
	connection, connected := respDial(target)
	if !connected {
		return false
	}
	defer func() {
		if err := connection.Close(); err != nil {
			fmt.Println("probe: the connection did not close cleanly: " + err.Error())
		}
	}()
	if err := respTalk(connection); err != nil {
		fmt.Println("probe: FAIL protocol: " + err.Error())
		return false
	}
	fmt.Println("probe: the service answered every exchange")
	return true
}

// respDial dials target until a connection opens or the attempts run out,
// and prints which.
func respDial(target string) (net.Conn, bool) {
	start := time.Now()
	var failure error
	for range respDialSteps {
		connection, err := dialOnce(target)
		if err == nil {
			fmt.Println("probe: connected to " + target + " after " + waited(start))
			return connection, true
		}
		failure = err
		time.Sleep(respDialStep)
	}
	if errors.Is(failure, syscall.ECONNREFUSED) {
		fmt.Println("probe: FAIL port: every connection to " + target + " was refused for " + waited(start) +
			": a reset came back, so nothing listened where the port leads: " + failure.Error())
		return nil, false
	}
	fmt.Println("probe: FAIL network: no connection to " + target + " was answered for " + waited(start) + ": " + failure.Error())
	return nil, false
}

// respTalk runs each exchange over connection, printing each reply that
// matched, and answers the first that did not.
func respTalk(connection net.Conn) error {
	reader := bufio.NewReaderSize(connection, respLineMax)
	for _, exchange := range respExchanges {
		sent := strings.Join(exchange.args, " ")
		if err := connection.SetDeadline(time.Now().Add(respReplyTimeout)); err != nil {
			return errors.New(sent + ": " + err.Error())
		}
		if _, err := connection.Write(respCommand(exchange.args)); err != nil {
			return errors.New(sent + " was not sent: " + err.Error())
		}
		reply, err := readResp(reader)
		if err != nil {
			return errors.New(sent + " got no reply: " + err.Error())
		}
		if reply != exchange.want {
			return errors.New(sent + " answered " + reply.String() + ", not " + exchange.want.String())
		}
		fmt.Println("probe: " + sent + " answered " + reply.String())
	}
	return nil
}

// respCommand encodes args as a RESP array of bulk strings, the form a Redis
// client sends: *COUNT, then $LENGTH and the bytes of each argument, every
// line ended by CRLF.
func respCommand(args []string) []byte {
	encoded := strconv.AppendInt([]byte{'*'}, int64(len(args)), 10)
	encoded = append(encoded, respCRLF...)
	for _, arg := range args {
		encoded = append(encoded, byte(respBulk))
		encoded = strconv.AppendInt(encoded, int64(len(arg)), 10)
		encoded = append(encoded, respCRLF...)
		encoded = append(encoded, arg...)
		encoded = append(encoded, respCRLF...)
	}
	return encoded
}

// readResp reads one reply. The kind byte comes from the server, so the set
// is open: a kind this reader does not know is an error naming the line.
func readResp(reader *bufio.Reader) (respReply, error) {
	line, err := readRespLine(reader)
	if err != nil {
		return respReply{}, err
	}
	if line == "" {
		return respReply{}, errors.New("an empty reply line")
	}
	kind, text := respKind(line[0]), line[1:]
	switch kind {
	case respSimple, respError, respInteger:
		return respReply{kind: kind, text: text}, nil
	case respBulk:
		return readRespBulk(reader, text)
	default:
		// The set is open: the byte is the server's.
		return respReply{}, errors.New("a reply that is not RESP2: " + strconv.Quote(line))
	}
}

// readRespLine reads one line ended by CRLF, at most the reader's buffer,
// respLineMax bytes, and answers it without its CRLF.
func readRespLine(reader *bufio.Reader) (string, error) {
	line, err := reader.ReadSlice('\n')
	if errors.Is(err, bufio.ErrBufferFull) {
		return "", errors.New("a reply line longer than " + strconv.Itoa(respLineMax) + " bytes")
	}
	if err != nil {
		return "", err
	}
	ending := len(line) - len(respCRLF)
	if ending < 0 {
		return "", errors.New("a reply line not ended by CRLF: " + strconv.Quote(string(line)))
	}
	if string(line[ending:]) != respCRLF {
		return "", errors.New("a reply line not ended by CRLF: " + strconv.Quote(string(line)))
	}
	return string(line[:ending]), nil
}

// readRespBulk reads the body of a bulk string whose header line carried
// length, bounded by respBulkMax.
func readRespBulk(reader *bufio.Reader, length string) (respReply, error) {
	size, err := strconv.Atoi(length)
	if err != nil {
		return respReply{}, errors.New("a bulk string length that is not a number: " + strconv.Quote(length))
	}
	if size == respNullBulkLength {
		return respReply{kind: respBulk, null: true}, nil
	}
	if size < 0 {
		return respReply{}, errors.New("a negative bulk string length: " + strconv.Itoa(size))
	}
	if size > respBulkMax {
		return respReply{}, errors.New("a bulk string of " + strconv.Itoa(size) + " bytes, past the bound of " + strconv.Itoa(respBulkMax))
	}
	body := make([]byte, size+len(respCRLF))
	if _, err := io.ReadFull(reader, body); err != nil {
		return respReply{}, errors.New("a bulk string cut short: " + err.Error())
	}
	if string(body[size:]) != respCRLF {
		return respReply{}, errors.New("a bulk string not ended by CRLF: " + strconv.Quote(string(body)))
	}
	return respReply{kind: respBulk, text: string(body[:size])}, nil
}

// waited answers the time since start, rounded for the probe's lines.
func waited(start time.Time) string {
	return time.Since(start).Round(100 * time.Millisecond).String()
}
