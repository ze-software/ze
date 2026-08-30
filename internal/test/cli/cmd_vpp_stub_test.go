package cli

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"net"
	"path/filepath"
	"strings"
	"testing"
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
