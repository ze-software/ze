package cli

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"go.fd.io/govpp/binapi/fib_types"
	"go.fd.io/govpp/binapi/ip"
	"go.fd.io/govpp/binapi/ip_types"
)

func TestVPPStubNegotiatesRegisteredMessagesOverTheRealFrameProtocol(t *testing.T) {
	state, err := newVPPStubState(filepath.Join(t.TempDir(), "stub.jsonl"), false)
	if err != nil {
		t.Fatalf("new stub state: %v", err)
	}
	defer state.log.Close() //nolint:errcheck // test cleanup
	client, server := net.Pipe()
	defer client.Close() //nolint:errcheck // test cleanup
	go func() {
		_ = serveVPPClient(state, server)
		_ = server.Close()
	}()

	request := make([]byte, 6)
	binary.BigEndian.PutUint16(request, sockCreateMessageID)
	binary.BigEndian.PutUint32(request[2:6], 77)
	if err := writeVPPFrame(client, request); err != nil {
		t.Fatalf("write create frame: %v", err)
	}
	reply, err := readVPPFrame(bufio.NewReader(client))
	if err != nil {
		t.Fatalf("read create reply: %v", err)
	}
	message := state.messages["sockclnt_create_reply"]
	if got := binary.BigEndian.Uint16(reply); got != message.id {
		t.Fatalf("reply id = %d, want %d", got, message.id)
	}
	if got := binary.BigEndian.Uint32(reply[6:10]); got != 77 {
		t.Fatalf("reply context = %d, want 77", got)
	}
	count := int(binary.BigEndian.Uint16(reply[18:20]))
	if count != len(state.messages) {
		t.Fatalf("message table count = %d, want %d", count, len(state.messages))
	}
	last := reply[20+66*(count-1) : 20+66*count]
	if !strings.HasPrefix(strings.TrimRight(string(last[2:]), "\x00"), "sockclnt_delete_") {
		t.Fatalf("last message table entry = %q, want sockclnt_delete request", last[2:])
	}
}

func TestVPPStubAllocatesDistinctLoopbacksAndClassifyTables(t *testing.T) {
	state, err := newVPPStubState(filepath.Join(t.TempDir(), "stub.jsonl"), false)
	if err != nil {
		t.Fatalf("new stub state: %v", err)
	}
	defer state.log.Close() //nolint:errcheck // test cleanup
	for _, name := range []string{"create_loopback", "classify_add_del_table"} {
		first, _, handled, err := state.handle(name, 1, []byte{1, 2, 3, 4, 5, 6})
		if err != nil || !handled {
			t.Fatalf("first %s: handled=%v err=%v", name, handled, err)
		}
		second, _, handled, err := state.handle(name, 2, []byte{1, 2, 3, 4, 5, 6})
		if err != nil || !handled {
			t.Fatalf("second %s: handled=%v err=%v", name, handled, err)
		}
		if bytes.Equal(first, second) {
			t.Fatalf("%s returned the same allocated index twice", name)
		}
	}
}

// VALIDATES: spec-fib-depth AC-5 -- VPP stub multipath ip_route_add_del_v2 decoding logs every path while preserving path-0 top-level fields.
// PREVENTS: stub decoding dropping secondary ECMP paths or breaking existing single-path stub log consumers.
func TestVPPStubDecodesMultipathIPRouteV2(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "stub.jsonl")
	state, err := newVPPStubState(logPath, false)
	if err != nil {
		t.Fatalf("new stub state: %v", err)
	}
	defer state.log.Close() //nolint:errcheck // test cleanup

	prefix := ip_types.Prefix{
		Address: ip_types.Address{Af: ip_types.ADDRESS_IP4},
		Len:     24,
	}
	prefix.Address.Un.SetIP4(ip_types.IP4Address{10, 20, 0, 0})

	pathA := fib_types.FibPath{
		SwIfIndex: ^uint32(0),
		Weight:    2,
		Proto:     fib_types.FIB_API_PATH_NH_PROTO_IP4,
		NLabels:   1,
	}
	pathA.Nh.Address.SetIP4(ip_types.IP4Address{192, 0, 2, 1})
	pathA.LabelStack[0].Label = 16001

	pathB := fib_types.FibPath{
		SwIfIndex: ^uint32(0),
		Weight:    5,
		Proto:     fib_types.FIB_API_PATH_NH_PROTO_IP4,
	}
	pathB.Nh.Address.SetIP4(ip_types.IP4Address{192, 0, 2, 2})

	req := &ip.IPRouteAddDelV2{
		IsAdd:       true,
		IsMultipath: true,
		Route: ip.IPRouteV2{
			TableID: 0,
			Prefix:  prefix,
			NPaths:  2,
			Src:     vppStubFibSourceID,
			Paths:   []fib_types.FibPath{pathA, pathB},
		},
	}
	body, err := req.Marshal(nil)
	if err != nil {
		t.Fatalf("marshal ip_route_add_del_v2: %v", err)
	}

	_, _, handled, err := state.handle("ip_route_add_del_v2", 7, body)
	if err != nil {
		t.Fatalf("handle ip_route_add_del_v2: %v", err)
	}
	if !handled {
		t.Fatal("ip_route_add_del_v2 was not handled")
	}

	raw, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read stub log: %v", err)
	}
	var logged struct {
		Msg    string         `json:"msg"`
		Fields map[string]any `json:"fields"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(raw), &logged); err != nil {
		t.Fatalf("unmarshal stub log: %v", err)
	}
	if logged.Msg != "ip_route_add_del_v2" {
		t.Fatalf("msg = %q, want ip_route_add_del_v2", logged.Msg)
	}
	if logged.Fields["is_add"] != true {
		t.Fatalf("is_add = %v, want true", logged.Fields["is_add"])
	}
	if logged.Fields["is_multipath"] != true {
		t.Fatalf("is_multipath = %v, want true", logged.Fields["is_multipath"])
	}
	if logged.Fields["prefix"] != "10.20.0.0/24" {
		t.Fatalf("prefix = %v, want 10.20.0.0/24", logged.Fields["prefix"])
	}
	if logged.Fields["n_paths"] != float64(2) {
		t.Fatalf("n_paths = %v, want 2", logged.Fields["n_paths"])
	}
	if logged.Fields["next_hop"] != "192.0.2.1" {
		t.Fatalf("top-level next_hop = %v, want 192.0.2.1", logged.Fields["next_hop"])
	}
	topLabels, ok := logged.Fields["labels"].([]any)
	if !ok {
		t.Fatalf("top-level labels = %T, want []any", logged.Fields["labels"])
	}
	if !slices.Equal(topLabels, []any{float64(16001)}) {
		t.Fatalf("top-level labels = %v, want [16001]", topLabels)
	}

	paths, ok := logged.Fields["paths"].([]any)
	if !ok {
		t.Fatalf("paths = %T, want []any", logged.Fields["paths"])
	}
	if len(paths) != 2 {
		t.Fatalf("len(paths) = %d, want 2", len(paths))
	}
	first, ok := paths[0].(map[string]any)
	if !ok {
		t.Fatalf("paths[0] = %T, want map[string]any", paths[0])
	}
	if first["next_hop"] != "192.0.2.1" {
		t.Fatalf("paths[0].next_hop = %v, want 192.0.2.1", first["next_hop"])
	}
	if first["weight"] != float64(2) {
		t.Fatalf("paths[0].weight = %v, want 2", first["weight"])
	}
	firstLabels, ok := first["labels"].([]any)
	if !ok {
		t.Fatalf("paths[0].labels = %T, want []any", first["labels"])
	}
	if !slices.Equal(firstLabels, []any{float64(16001)}) {
		t.Fatalf("paths[0].labels = %v, want [16001]", firstLabels)
	}

	second, ok := paths[1].(map[string]any)
	if !ok {
		t.Fatalf("paths[1] = %T, want map[string]any", paths[1])
	}
	if second["next_hop"] != "192.0.2.2" {
		t.Fatalf("paths[1].next_hop = %v, want 192.0.2.2", second["next_hop"])
	}
	if second["weight"] != float64(5) {
		t.Fatalf("paths[1].weight = %v, want 5", second["weight"])
	}
	if _, hasLabels := second["labels"]; hasLabels {
		t.Fatalf("paths[1].labels = %v, want absent when n_labels is 0", second["labels"])
	}

	// Truncated second path must stay bounded by remaining body length.
	truncated := decodeIPRoute(body[:ipRoutePathsV2+fibPathSize+10], ipRoutePathsV2)
	truncatedPaths, ok := truncated["paths"].([]map[string]any)
	if !ok {
		t.Fatalf("truncated paths = %T, want []map[string]any", truncated["paths"])
	}
	if len(truncatedPaths) != 1 {
		t.Fatalf("len(truncated paths) = %d, want 1", len(truncatedPaths))
	}
	if truncatedPaths[0]["next_hop"] != "192.0.2.1" {
		t.Fatalf("truncated paths[0].next_hop = %v, want 192.0.2.1", truncatedPaths[0]["next_hop"])
	}
}
