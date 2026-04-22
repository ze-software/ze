package reactor

import (
	"bufio"
	"encoding/binary"
	"net"
	"net/netip"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bgpctx "codeberg.org/thomas-mangin/ze/internal/component/bgp/context"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/wireu"
	"codeberg.org/thomas-mangin/ze/internal/core/env"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

// buildUpdateBody constructs a raw UPDATE body (no BGP header) with the
// given attributes and NLRI bytes. Withdrawn routes length is 0.
func buildUpdateBody(attrs, nlri []byte) []byte {
	body := make([]byte, 0, 4+len(attrs)+len(nlri))
	body = append(body, 0, 0, byte(len(attrs)>>8), byte(len(attrs)))
	body = append(body, attrs...)
	return append(body, nlri...)
}

// buildWithdrawalBody constructs a raw UPDATE body with withdrawn routes.
func buildWithdrawalBody(withdrawn []byte) []byte {
	body := make([]byte, 0, 4+len(withdrawn))
	body = append(body, byte(len(withdrawn)>>8), byte(len(withdrawn)))
	body = append(body, withdrawn...)
	body = append(body, 0, 0) // Path attributes length = 0
	return body
}

// sampleAttrs returns a minimal valid path attributes blob.
// ORIGIN(1) + AS_PATH(2) + NEXT_HOP(3).
func sampleAttrs() []byte {
	return []byte{
		// ORIGIN: flags=0x40, code=1, len=1, value=0 (IGP)
		0x40, 0x01, 0x01, 0x00,
		// AS_PATH: flags=0x40, code=2, len=0 (empty)
		0x40, 0x02, 0x00,
		// NEXT_HOP: flags=0x40, code=3, len=4, value=10.0.0.1
		0x40, 0x03, 0x04, 0x0a, 0x00, 0x00, 0x01,
	}
}

// sampleAttrs2 returns a different path attributes blob (different next-hop).
func sampleAttrs2() []byte {
	return []byte{
		0x40, 0x01, 0x01, 0x00,
		0x40, 0x02, 0x00,
		0x40, 0x03, 0x04, 0x0a, 0x00, 0x00, 0x02, // 10.0.0.2
	}
}

// newCoalesceSession creates a Session with coalescing enabled and a
// message callback that records received UPDATE bodies.
func newCoalesceSession(t *testing.T) (*Session, *[][]byte) {
	t.Helper()
	settings := NewPeerSettings(
		netip.MustParseAddr("192.0.2.1"),
		65001, 65002, 0x01020301,
	)
	session := NewSession(settings)
	session.coalesceEnabled = true

	var mu sync.Mutex
	var bodies [][]byte
	session.onMessageReceived = func(
		_ netip.Addr, msgType message.MessageType, rawBytes []byte,
		_ *wireu.WireUpdate, _ bgpctx.ContextID, _ rpc.MessageDirection,
		_ BufHandle, _ map[string]any,
	) bool {
		if msgType == message.TypeUPDATE {
			mu.Lock()
			cp := make([]byte, len(rawBytes))
			copy(cp, rawBytes)
			bodies = append(bodies, cp)
			mu.Unlock()
		}
		return false
	}
	return session, &bodies
}

// writeAllAndClose concatenates all messages into one write then closes.
// Single write ensures all data lands in the reader's buffer together,
// so bufReader.Buffered() > 0 between messages (required for coalescing).
func writeAllAndClose(conn net.Conn, msgs ...[]byte) {
	var combined []byte
	for _, msg := range msgs {
		combined = append(combined, msg...)
	}
	if _, err := conn.Write(combined); err != nil {
		return
	}
	if err := conn.Close(); err != nil {
		return
	}
}

// VALIDATES: Two consecutive UPDATEs with identical attributes are coalesced.
// PREVENTS: Duplicate processing when attributes match.
func TestCoalesceIdenticalAttrs(t *testing.T) {
	session, bodies := newCoalesceSession(t)

	attrs := sampleAttrs()
	nlri1 := []byte{24, 10, 1, 1} // 10.1.1.0/24
	nlri2 := []byte{24, 10, 1, 2} // 10.1.2.0/24
	body1 := buildUpdateBody(attrs, nlri1)
	body2 := buildUpdateBody(attrs, nlri2)
	msg1 := buildUpdateMsg(body1)
	msg2 := buildUpdateMsg(body2)

	server, client := net.Pipe()
	defer func() { _ = server.Close() }()
	defer func() { _ = client.Close() }()

	go writeAllAndClose(client, msg1, msg2)

	reader := bufio.NewReaderSize(server, 65536)

	// Process both messages. The second should coalesce with the first.
	// After the second read, Buffered()==0 triggers flush.
	for range 3 {
		err := session.readAndProcessCoalesced(server, reader)
		if err != nil {
			break
		}
	}

	require.Len(t, *bodies, 1, "should produce exactly one coalesced UPDATE")
	coalesced := (*bodies)[0]

	// Verify the coalesced body has both NLRIs.
	// Structure: wdLen(2)=0 + attrLen(2) + attrs + nlri1 + nlri2
	wdLen := int(coalesced[0])<<8 | int(coalesced[1])
	assert.Equal(t, 0, wdLen)

	attrLen := int(coalesced[2])<<8 | int(coalesced[3])
	assert.Equal(t, len(attrs), attrLen)

	nlriStart := 4 + attrLen
	combinedNLRI := coalesced[nlriStart:]
	assert.Equal(t, append(nlri1, nlri2...), combinedNLRI)
}

// VALIDATES: UPDATEs with different attributes are flushed separately.
// PREVENTS: Incorrect coalescing of mismatched attributes.
func TestCoalesceDifferentAttrs(t *testing.T) {
	session, bodies := newCoalesceSession(t)

	attrs1 := sampleAttrs()
	attrs2 := sampleAttrs2()
	nlri1 := []byte{24, 10, 1, 1}
	nlri2 := []byte{24, 10, 1, 2}
	body1 := buildUpdateBody(attrs1, nlri1)
	body2 := buildUpdateBody(attrs2, nlri2)
	msg1 := buildUpdateMsg(body1)
	msg2 := buildUpdateMsg(body2)

	server, client := net.Pipe()
	defer func() { _ = server.Close() }()
	defer func() { _ = client.Close() }()

	go writeAllAndClose(client, msg1, msg2)

	reader := bufio.NewReaderSize(server, 65536)

	for range 3 {
		err := session.readAndProcessCoalesced(server, reader)
		if err != nil {
			break
		}
	}

	require.Len(t, *bodies, 2, "different attrs must not coalesce")
}

// VALIDATES: Withdrawals are never coalesced, always flushed immediately.
// PREVENTS: Losing withdrawal semantics through coalescing.
func TestCoalesceWithdrawalNotCoalesced(t *testing.T) {
	session, bodies := newCoalesceSession(t)

	attrs := sampleAttrs()
	nlri1 := []byte{24, 10, 1, 1}
	body1 := buildUpdateBody(attrs, nlri1)
	msg1 := buildUpdateMsg(body1)

	withdrawn := []byte{24, 10, 2, 1} // 10.2.1.0/24
	wdBody := buildWithdrawalBody(withdrawn)
	msg2 := buildUpdateMsg(wdBody)

	server, client := net.Pipe()
	defer func() { _ = server.Close() }()
	defer func() { _ = client.Close() }()

	go writeAllAndClose(client, msg1, msg2)

	reader := bufio.NewReaderSize(server, 65536)

	for range 3 {
		err := session.readAndProcessCoalesced(server, reader)
		if err != nil {
			break
		}
	}

	require.Len(t, *bodies, 2, "withdrawal must force separate dispatch")
}

// VALIDATES: Non-UPDATE message types flush the held batch.
// PREVENTS: Held UPDATEs blocking KEEPALIVE processing.
func TestCoalesceKeepaliveFlushesBatch(t *testing.T) {
	session, bodies := newCoalesceSession(t)

	attrs := sampleAttrs()
	nlri := []byte{24, 10, 1, 1}
	updateBody := buildUpdateBody(attrs, nlri)
	updateMsg := buildUpdateMsg(updateBody)

	keepalive := make([]byte, 19)
	for i := range 16 {
		keepalive[i] = 0xff
	}
	binary.BigEndian.PutUint16(keepalive[16:], 19)
	keepalive[18] = byte(message.TypeKEEPALIVE)

	server, client := net.Pipe()
	defer func() { _ = server.Close() }()
	defer func() { _ = client.Close() }()

	go writeAllAndClose(client, updateMsg, keepalive)

	reader := bufio.NewReaderSize(server, 65536)

	for range 3 {
		err := session.readAndProcessCoalesced(server, reader)
		if err != nil {
			break
		}
	}

	require.Len(t, *bodies, 1, "UPDATE must be flushed before KEEPALIVE is processed")
}

// VALIDATES: Single UPDATE with no following data is dispatched immediately.
// PREVENTS: Single updates getting stuck in the coalesce buffer.
func TestCoalesceSingleUpdate(t *testing.T) {
	session, bodies := newCoalesceSession(t)

	attrs := sampleAttrs()
	nlri := []byte{24, 10, 1, 1}
	body := buildUpdateBody(attrs, nlri)
	msg := buildUpdateMsg(body)

	server, client := net.Pipe()
	defer func() { _ = server.Close() }()
	defer func() { _ = client.Close() }()

	go writeAllAndClose(client, msg)

	reader := bufio.NewReaderSize(server, 65536)

	err := session.readAndProcessCoalesced(server, reader)
	require.NoError(t, err)

	require.Len(t, *bodies, 1, "single UPDATE must be dispatched immediately")
}

// VALIDATES: Coalescing is disabled when ze.bgp.reactor.coalesce=false.
// PREVENTS: Coalescing when operator explicitly disables it.
func TestCoalesceDisabledByEnv(t *testing.T) {
	require.NoError(t, env.SetBool("ze.bgp.reactor.coalesce", false))
	defer func() {
		require.NoError(t, env.Set("ze.bgp.reactor.coalesce", ""))
		env.ResetCache()
	}()

	assert.False(t, coalesceEnabled())
}

// VALIDATES: Three consecutive UPDATEs with identical attributes produce one coalesced body.
// PREVENTS: Off-by-one in multi-append path.
func TestCoalesceThreeUpdates(t *testing.T) {
	session, bodies := newCoalesceSession(t)

	attrs := sampleAttrs()
	nlri1 := []byte{24, 10, 1, 1}
	nlri2 := []byte{24, 10, 1, 2}
	nlri3 := []byte{24, 10, 1, 3}
	msg1 := buildUpdateMsg(buildUpdateBody(attrs, nlri1))
	msg2 := buildUpdateMsg(buildUpdateBody(attrs, nlri2))
	msg3 := buildUpdateMsg(buildUpdateBody(attrs, nlri3))

	server, client := net.Pipe()
	defer func() { _ = server.Close() }()
	defer func() { _ = client.Close() }()

	go writeAllAndClose(client, msg1, msg2, msg3)

	reader := bufio.NewReaderSize(server, 65536)

	for range 4 {
		if err := session.readAndProcessCoalesced(server, reader); err != nil {
			break
		}
	}

	require.Len(t, *bodies, 1, "three identical-attr UPDATEs must coalesce into one")
	coalesced := (*bodies)[0]

	attrLen := int(coalesced[2])<<8 | int(coalesced[3])
	nlriStart := 4 + attrLen
	combinedNLRI := coalesced[nlriStart:]

	var expected []byte
	expected = append(expected, nlri1...)
	expected = append(expected, nlri2...)
	expected = append(expected, nlri3...)
	assert.Equal(t, expected, combinedNLRI)
}

// VALIDATES: When the coalesce buffer is full, held batch flushes and
// the current UPDATE starts a new batch.
// PREVENTS: Lost NLRIs when coalesce buffer capacity is exhausted.
func TestCoalesceBufferFullFlushesAndContinues(t *testing.T) {
	session, bodies := newCoalesceSession(t)

	attrs := sampleAttrs()
	// Body cap = min(4096, MaxMsgLen - HeaderLen) = min(4096, 4077) = 4077.
	// Fixed overhead in body: 4 + len(attrs) = 18. NLRI space: 4077 - 18 = 4059.
	// Each /24 NLRI = 4 bytes. 4059/4 = 1014.75, so 1015 NLRIs won't fit.
	// First UPDATE: 1010 NLRIs = 4040 bytes. Remaining: 4059 - 4040 = 19 bytes.
	// Second UPDATE: 5 NLRIs = 20 bytes. Won't fit (20 > 19).

	nlriPerUpdate := 1010
	bigNLRI := make([]byte, 0, nlriPerUpdate*4)
	for i := range nlriPerUpdate {
		bigNLRI = append(bigNLRI, 24, 10, byte(i>>8), byte(i))
	}

	// 5 NLRIs = 20 bytes, exceeds remaining 19.
	smallNLRI := make([]byte, 0, 20)
	for i := range 5 {
		smallNLRI = append(smallNLRI, 24, 172, 16, byte(i))
	}

	msg1 := buildUpdateMsg(buildUpdateBody(attrs, bigNLRI))
	msg2 := buildUpdateMsg(buildUpdateBody(attrs, smallNLRI))

	server, client := net.Pipe()
	defer func() { _ = server.Close() }()
	defer func() { _ = client.Close() }()

	go writeAllAndClose(client, msg1, msg2)

	reader := bufio.NewReaderSize(server, 65536)

	for range 3 {
		if err := session.readAndProcessCoalesced(server, reader); err != nil {
			break
		}
	}

	require.Len(t, *bodies, 2, "buffer-full must flush first batch and start new one")

	// First body has bigNLRI, second has smallNLRI.
	first := (*bodies)[0]
	firstAttrLen := int(first[2])<<8 | int(first[3])
	firstNLRI := first[4+firstAttrLen:]
	assert.Equal(t, bigNLRI, firstNLRI)

	second := (*bodies)[1]
	secondAttrLen := int(second[2])<<8 | int(second[3])
	secondNLRI := second[4+secondAttrLen:]
	assert.Equal(t, smallNLRI, secondNLRI)
}

// VALIDATES: resetCoalesce releases buffers without dispatching.
// PREVENTS: Buffer leaks on session teardown.
func TestResetCoalesceReleasesBuffers(t *testing.T) {
	session, bodies := newCoalesceSession(t)

	attrs := sampleAttrs()
	nlri := []byte{24, 10, 1, 1}
	body := buildUpdateBody(attrs, nlri)

	coalBuf := BufHandle{ID: noPoolBufID, Buf: make([]byte, 4096)}
	copy(coalBuf.Buf[0:], body)

	session.coalesce = coalesceState{
		buf:     coalBuf,
		body:    coalBuf.Buf[:len(body):len(coalBuf.Buf)],
		attrLen: len(attrs),
	}

	session.resetCoalesce()

	assert.Nil(t, session.coalesce.body, "coalesce body must be nil after reset")
	assert.Nil(t, session.coalesce.buf.Buf, "coalesce buf must be zeroed after reset")
	assert.Equal(t, 0, session.coalesce.attrLen, "coalesce attrLen must be zeroed after reset")
	assert.Empty(t, *bodies, "resetCoalesce must not dispatch")
}
