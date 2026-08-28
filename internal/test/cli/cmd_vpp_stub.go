package cli

import (
	"bufio"
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"slices"
	"strings"
	"sync"
	"syscall"
	"time"

	"go.fd.io/govpp/api"
	_ "go.fd.io/govpp/binapi/classify"
	_ "go.fd.io/govpp/binapi/interface"
	_ "go.fd.io/govpp/binapi/ip"
	_ "go.fd.io/govpp/binapi/memclnt"
	_ "go.fd.io/govpp/binapi/mpls"

	"github.com/ze-software/ze/internal/core/cliio"
)

const (
	sockCreateMessageID = 15
	negotiatedIDBase    = 100
)

type vppStubMessage struct {
	crc  string
	kind api.MessageType
	id   uint16
}

type vppStubState struct {
	messages      map[string]vppStubMessage
	names         map[uint16]string
	log           io.WriteCloser
	verbose       bool
	mu            sync.Mutex
	classifyIndex uint32
	loopbackIndex uint32
}

func cmdVPPStub(args []string) int {
	flags := flag.NewFlagSet("vpp-stub", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	socketPath := flags.String("socket", "", "Unix socket path")
	logPath := flags.String("log", "", "JSONL log path")
	deadlineSeconds := flags.Float64("deadline", 30, "maximum lifetime in seconds")
	verbose := flags.Bool("verbose", false, "log requests to stderr")
	flags.BoolVar(verbose, "v", false, "log requests to stderr")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *socketPath == "" || *logPath == "" || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "vpp-stub: --socket and --log are required")
		return 2
	}
	if err := runVPPStub(*socketPath, *logPath, time.Duration(*deadlineSeconds*float64(time.Second)), *verbose); err != nil {
		fmt.Fprintf(os.Stderr, "vpp-stub: %v\n", err)
		return 1
	}
	return 0
}

func newVPPStubState(logPath string, verbose bool) (*vppStubState, error) {
	messages := make(map[string]vppStubMessage)
	for _, packageMessages := range api.GetRegisteredMessages() {
		for _, message := range packageMessages {
			messages[message.GetMessageName()] = vppStubMessage{crc: message.GetCrcString(), kind: message.GetMessageType()}
		}
	}
	if _, ok := messages["sockclnt_create_reply"]; !ok {
		return nil, errors.New("registered binapi lacks sockclnt_create_reply")
	}
	names := make([]string, 0, len(messages))
	for name := range messages {
		names = append(names, name)
	}
	slices.Sort(names)
	byID := make(map[uint16]string, len(names)+1)
	byID[sockCreateMessageID] = "sockclnt_create"
	next := uint16(negotiatedIDBase)
	for _, name := range names {
		if name == "sockclnt_create" {
			message := messages[name]
			message.id = sockCreateMessageID
			messages[name] = message
			continue
		}
		message := messages[name]
		message.id = next
		messages[name] = message
		byID[next] = name
		next++
	}
	logFile, err := cliio.Create(logPath)
	if err != nil {
		return nil, err
	}
	return &vppStubState{messages: messages, names: byID, log: logFile, verbose: verbose, loopbackIndex: 1}, nil
}

func runVPPStub(socketPath, logPath string, deadline time.Duration, verbose bool) error {
	_ = os.Remove(socketPath)
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return err
	}
	defer func() { listener.Close(); os.Remove(socketPath) }()
	if err := os.Chmod(socketPath, 0o600); err != nil {
		return err
	}
	state, err := newVPPStubState(logPath, verbose)
	if err != nil {
		return err
	}
	defer state.log.Close()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if deadline > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, deadline)
		defer cancel()
	}
	go func() { <-ctx.Done(); listener.Close() }()
	if verbose {
		fmt.Fprintf(os.Stderr, "vpp-stub: listening on %s (log=%s, deadline=%s)\n", socketPath, logPath, deadline)
	}
	for {
		connection, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		if err := serveVPPClient(state, connection); err != nil && verbose {
			fmt.Fprintf(os.Stderr, "vpp-stub: client error: %v\n", err)
		}
		connection.Close()
	}
}

func (state *vppStubState) writeLog(name string, context uint32, fields map[string]any) error {
	entry := map[string]any{"ts": time.Now().UTC().Format(time.RFC3339Nano), "msg": name, "context": context, "fields": fields}
	encoded, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if _, err := state.log.Write(append(encoded, '\n')); err != nil {
		return err
	}
	if state.verbose {
		fmt.Fprintf(os.Stderr, "vpp-stub: %s %v\n", name, fields)
	}
	return nil
}

func readVPPFrame(reader *bufio.Reader) ([]byte, error) {
	header := make([]byte, 16)
	if _, err := io.ReadFull(reader, header); err != nil {
		return nil, err
	}
	length := binary.BigEndian.Uint32(header[8:12])
	body := make([]byte, length)
	_, err := io.ReadFull(reader, body)
	return body, err
}

func writeVPPFrame(writer io.Writer, payload []byte) error {
	header := make([]byte, 16)
	binary.BigEndian.PutUint32(header[8:12], uint32(len(payload))) //nolint:gosec // frame allocation already bounds length to int.
	if _, err := writer.Write(header); err != nil {
		return err
	}
	_, err := writer.Write(payload)
	return err
}

func (state *vppStubState) reply(name string, context uint32, body []byte) ([]byte, error) {
	message, ok := state.messages[name]
	if !ok {
		return nil, fmt.Errorf("reply %s is not registered", name)
	}
	offset := 2
	if message.kind == api.RequestMessage {
		offset = 10
	} else if message.kind == api.ReplyMessage || message.kind == api.EventMessage {
		offset = 6
	}
	payload := make([]byte, offset+len(body))
	binary.BigEndian.PutUint16(payload, message.id)
	if offset == 10 {
		binary.BigEndian.PutUint32(payload[2:6], 1)
		binary.BigEndian.PutUint32(payload[6:10], context)
	}
	if offset == 6 {
		binary.BigEndian.PutUint32(payload[2:6], context)
	}
	copy(payload[offset:], body)
	return payload, nil
}

func (state *vppStubState) requestHeader(id uint16) int {
	name := state.names[id]
	message, ok := state.messages[name]
	if !ok {
		return 10
	}
	if message.kind == api.RequestMessage {
		return 10
	}
	if message.kind == api.ReplyMessage || message.kind == api.EventMessage {
		return 6
	}
	return 2
}

func serveVPPClient(state *vppStubState, connection net.Conn) error {
	reader := bufio.NewReader(connection)
	for {
		frame, err := readVPPFrame(reader)
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if len(frame) < 2 {
			return fmt.Errorf("short request body: %d bytes", len(frame))
		}
		id := binary.BigEndian.Uint16(frame)
		headerLength := state.requestHeader(id)
		if len(frame) < headerLength {
			return fmt.Errorf("short request body for header: %d < %d", len(frame), headerLength)
		}
		var contextID, clientIndex uint32
		if headerLength == 10 {
			clientIndex = binary.BigEndian.Uint32(frame[2:6])
			contextID = binary.BigEndian.Uint32(frame[6:10])
		}
		if headerLength == 6 {
			contextID = binary.BigEndian.Uint32(frame[2:6])
		}
		name := state.names[id]
		if name == "" {
			name = fmt.Sprintf("unknown_%d", id)
		}
		body, closeConnection, handled, err := state.handle(name, contextID, frame[headerLength:])
		if err != nil {
			return err
		}
		if !handled {
			if err := state.writeLog(name, contextID, map[string]any{"client_index": clientIndex, "unhandled": true}); err != nil {
				return err
			}
			replyName := name + "_reply"
			if _, ok := state.messages[replyName]; ok {
				body, err = state.reply(replyName, contextID, []byte{0, 0, 0, 0})
			} else {
				body = nil
			}
			if err != nil {
				return err
			}
		}
		if body != nil {
			if err := writeVPPFrame(connection, body); err != nil {
				return err
			}
		}
		if closeConnection {
			return nil
		}
	}
}

func (state *vppStubState) handle(name string, contextID uint32, body []byte) ([]byte, bool, bool, error) {
	fields := make(map[string]any)
	replyName := name + "_reply"
	replyBody := []byte{0, 0, 0, 0}
	closeConnection := false
	switch name {
	case "sockclnt_create":
		replyName = "sockclnt_create_reply"
		replyBody = state.sockCreateReplyBody()
	case "sockclnt_delete":
		closeConnection = true
	case "control_ping":
		replyName = "control_ping_reply"
		replyBody = make([]byte, 12)
		binary.BigEndian.PutUint32(replyBody[4:8], 1)
		binary.BigEndian.PutUint32(replyBody[8:12], uint32(os.Getpid())) //nolint:gosec // process ids fit uint32 on supported systems.
	case "ip_route_add_del":
		fields = decodeIPRoute(body)
		replyBody = make([]byte, 8)
	case "mpls_route_add_del":
		fields = decodeMPLSRoute(body)
		replyBody = make([]byte, 8)
	case "sw_interface_set_mpls_enable":
		if len(body) >= 5 {
			fields["sw_if_index"] = binary.BigEndian.Uint32(body)
			fields["enable"] = body[4] != 0
		}
	case "ip_route_lookup_v2":
		fields, replyBody = decodeRouteLookup(body)
	case "classify_add_del_table":
		isAdd := len(body) == 0 || body[0] != 0
		index := uint32(0xffffffff)
		if isAdd {
			index = state.classifyIndex
			state.classifyIndex++
		} else if len(body) >= 6 {
			index = binary.BigEndian.Uint32(body[2:6])
		}
		fields = map[string]any{"is_add": isAdd, "new_table_index": index}
		replyBody = make([]byte, 8)
		binary.BigEndian.PutUint32(replyBody[4:8], index)
	case "create_loopback":
		index := state.loopbackIndex
		state.loopbackIndex++
		mac := ""
		if len(body) >= 6 {
			mac = strings.Join(hexBytePairs(body[:6]), ":")
		}
		fields = map[string]any{"mac_address": mac, "sw_if_index": index}
		replyBody = make([]byte, 8)
		binary.BigEndian.PutUint32(replyBody[4:8], index)
	case "sw_interface_add_del_address":
		index := uint32(0)
		isAdd := false
		if len(body) >= 4 {
			index = binary.BigEndian.Uint32(body)
		}
		if len(body) >= 5 {
			isAdd = body[4] != 0
		}
		fields = map[string]any{"sw_if_index": index, "is_add": isAdd}
	default:
		return nil, false, false, nil
	}
	if err := state.writeLog(name, contextID, fields); err != nil {
		return nil, false, true, err
	}
	reply, err := state.reply(replyName, contextID, replyBody)
	return reply, closeConnection, true, err
}

func (state *vppStubState) sockCreateReplyBody() []byte {
	names := make([]string, 0, len(state.messages))
	for name := range state.messages {
		names = append(names, name)
	}
	slices.Sort(names)
	if slices.Contains(names, "sockclnt_delete") {
		names = append(slices.DeleteFunc(names, func(name string) bool { return name == "sockclnt_delete" }), "sockclnt_delete")
	}
	body := make([]byte, 10+66*len(names))
	binary.BigEndian.PutUint32(body[4:8], 1)
	binary.BigEndian.PutUint16(body[8:10], uint16(len(names))) //nolint:gosec // registered message count is bounded well below uint16.
	for index, name := range names {
		at := 10 + 66*index
		message := state.messages[name]
		binary.BigEndian.PutUint16(body[at:at+2], message.id)
		copy(body[at+2:at+66], name+"_"+message.crc)
	}
	return body
}

func decodeIPRoute(body []byte) map[string]any {
	fields := make(map[string]any)
	if len(body) < 29 {
		return fields
	}
	fields["is_add"] = body[0] != 0
	fields["is_multipath"] = body[1] != 0
	fields["table_id"] = binary.BigEndian.Uint32(body[2:6])
	fields["prefix"] = vppAddress(body[10], body[11:27]) + fmt.Sprintf("/%d", body[27])
	fields["n_paths"] = body[28]
	if body[28] > 0 && len(body) >= 71 {
		fields["next_hop"] = vppAddress(byte(binary.BigEndian.Uint32(body[51:55])), body[55:71])
	}
	if body[28] > 0 && len(body) >= 84 {
		fields["labels"] = vppLabels(body, 29)
	}
	return fields
}
func decodeMPLSRoute(body []byte) map[string]any {
	fields := make(map[string]any)
	if len(body) < 14 {
		return fields
	}
	fields["is_add"] = body[0] != 0
	fields["table_id"] = binary.BigEndian.Uint32(body[2:6])
	fields["label"] = binary.BigEndian.Uint32(body[6:10])
	fields["eos"] = body[10]
	fields["n_paths"] = body[13]
	if body[13] > 0 && len(body) >= 56 {
		fields["next_hop"] = vppAddress(byte(binary.BigEndian.Uint32(body[36:40])), body[40:56])
	}
	if body[13] > 0 && len(body) >= 69 {
		fields["out_labels"] = vppLabels(body, 14)
	}
	return fields
}
func vppLabels(body []byte, pathOffset int) []uint32 {
	countAt := pathOffset + 54
	if countAt >= len(body) {
		return nil
	}
	count := min(int(body[countAt]), 16)
	labels := make([]uint32, 0, count)
	for index := range count {
		at := pathOffset + 55 + 7*index
		if at+7 <= len(body) {
			labels = append(labels, binary.BigEndian.Uint32(body[at+1:at+5]))
		}
	}
	return labels
}
func decodeRouteLookup(body []byte) (map[string]any, []byte) {
	fields := make(map[string]any)
	if len(body) >= 23 {
		fields["table_id"] = binary.BigEndian.Uint32(body)
		fields["exact"] = body[4]
		fields["prefix"] = vppAddress(body[5], body[6:22]) + fmt.Sprintf("/%d", body[22])
	}
	reply := make([]byte, 199)
	prefix, _ := fields["prefix"].(string)
	if !strings.HasPrefix(prefix, "10.") {
		binary.BigEndian.PutUint32(reply, uint32(0xffffffff))
		return fields, reply
	}
	copy(reply[13:17], net.ParseIP("10.20.0.0").To4())
	reply[29] = 24
	reply[30] = 1
	reply[31] = 19
	reply[44] = 1
	copy(reply[58:62], net.ParseIP("192.168.1.1").To4())
	return fields, reply
}
func vppAddress(family byte, data []byte) string {
	if family == 0 && len(data) >= 4 {
		return net.IP(data[:4]).String()
	}
	if len(data) >= 16 {
		return net.IP(data[:16]).String()
	}
	return "<invalid>"
}
func hexBytePairs(data []byte) []string {
	encoded := hex.EncodeToString(data)
	pairs := make([]string, 0, len(data))
	for index := range len(data) {
		pairs = append(pairs, encoded[index*2:index*2+2])
	}
	return pairs
}
