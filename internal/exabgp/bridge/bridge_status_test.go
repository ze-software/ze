package bridge

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// VALIDATES: the bridge answers ExaBGP's `status` with the pinned ExaBGP's bytes.
// PREVENTS: a status line refused, or a peer printed with a state ze never had.

// TestBridgeReadsTheStatusLine holds the two spellings of ExaBGP's daemon
// status: `status` (API 4) and `daemon status` (API 6), with or without the
// trailing `json` (src/exabgp/reactor/api/dispatch/v4.py maps `status` to
// `daemon status`). The bridge performs it itself, so the line carries no
// command.
func TestBridgeReadsTheStatusLine(t *testing.T) {
	for _, line := range []string{"status", "daemon status", "status json", "daemon status json"} {
		translation, err := TranslateLine(line)
		require.NoError(t, err, line)
		assert.Equal(t, LocalStatus, translation.Local, line)
		assert.True(t, translation.Nothing(), line)
	}
}

// TestStatusAnswersInJSONAsExaBGPDecides holds API._answers_in_json at the
// pinned ExaBGP (src/exabgp/reactor/api/__init__.py): a script held to API 6
// always reads JSON, any other only when the line's last word is `json`.
func TestStatusAnswersInJSONAsExaBGPDecides(t *testing.T) {
	assert.True(t, StatusInJSON(APIV6, "daemon status"))
	assert.False(t, StatusInJSON(APIV4, "status"))
	assert.True(t, StatusInJSON(APIV4, "status json"))
	assert.False(t, StatusInJSON(APIVersionAuto, "status"))
	assert.True(t, StatusInJSON(APIVersionAuto, "status   json "))
}

// statusDetail is what `show bgp peer * detail` answers for two peers, cut to
// the fields the status line reads.
const statusDetail = `{"peers":{
 "192.0.2.9":{"remote-as":65002,"local-as":65001,"router-id":"10.0.0.1","fsm-state":"OPENSENT"},
 "127.0.0.1":{"remote-as":65535,"local-as":65535,"router-id":"127.0.0.1","local-ip":"127.0.0.1","fsm-state":"ESTABLISHED"}}}`

// TestStatusPeersNameEachPeerAsExaBGPDoes holds ExaBGP's peer key and state:
// the key of reactor._peers is Neighbor.name() (src/exabgp/bgp/neighbor/neighbor.py,
// `neighbor {ip} local-ip {local} local-as {as} peer-as {as} router-id {id}
// family-allowed in-open`, `auto` for an unset local address), and the state is
// FSM.name(), the RFC 4271 Section 8.2.2 state in upper case. ze's peer detail
// is a map, so the peers are ordered by address.
func TestStatusPeersNameEachPeerAsExaBGPDoes(t *testing.T) {
	peers, err := StatusPeers(json.RawMessage(statusDetail))
	require.NoError(t, err)
	assert.Equal(t, []StatusPeer{
		{Name: "neighbor 127.0.0.1 local-ip 127.0.0.1 local-as 65535 peer-as 65535 router-id 127.0.0.1 family-allowed in-open", State: "ESTABLISHED"},
		{Name: "neighbor 192.0.2.9 local-ip auto local-as 65001 peer-as 65002 router-id 10.0.0.1 family-allowed in-open", State: "OPENSENT"},
	}, peers)
}

// TestStatusPeersRefuseARowWithoutItsState proves a peer row with no FSM state
// is an error, never a peer printed with an empty or invented state.
func TestStatusPeersRefuseARowWithoutItsState(t *testing.T) {
	_, err := StatusPeers(json.RawMessage(`{"peers":{"127.0.0.1":{"remote-as":1,"local-as":1,"router-id":"1.1.1.1"}}}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "fsm-state")
}

// TestStatusReplyIsExaBGPsBytes holds the answer reactor.py `status` writes at
// the pinned ExaBGP, in both forms. Text: one processes.write per line.
// JSON: json.dumps with its default separators, start_time a float.
func TestStatusReplyIsExaBGPsBytes(t *testing.T) {
	start := time.Unix(1760960000, 250000000)
	info := statusInfo{
		uuid:  "0b5c2c8e-7a4e-4f7e-9d2a-3c1e5f6a7b8c",
		pid:   4242,
		start: start,
		now:   start.Add(3*time.Hour + 4*time.Minute + 5*time.Second + 900*time.Millisecond),
		peers: []StatusPeer{{Name: "neighbor 127.0.0.1 local-ip 127.0.0.1 local-as 65535 peer-as 65535 router-id 127.0.0.1 family-allowed in-open", State: "ESTABLISHED"}},
	}

	var text bytes.Buffer
	info.write(&text, false)
	assert.Equal(t, "ExaBGP Daemon Status\n"+
		"====================\n"+
		"Version: "+Version+"\n"+
		"UUID: 0b5c2c8e-7a4e-4f7e-9d2a-3c1e5f6a7b8c\n"+
		"PID: 4242\n"+
		"Uptime: 3h 4m 5s\n"+
		"Peers: 1\n"+
		"  - neighbor 127.0.0.1 local-ip 127.0.0.1 local-as 65535 peer-as 65535 router-id 127.0.0.1 family-allowed in-open: ESTABLISHED\n",
		text.String())

	// start_time is the nanoseconds as a double divided by 1e9, which is how
	// CPython's time.time() reaches its float, so the digits are Python's own:
	// python3 -c "print(repr(float(1760960000250000000)/1e9))" prints
	// 1760960000.2499998.
	var object bytes.Buffer
	info.write(&object, true)
	assert.Equal(t, `{"version": "`+Version+`", "uuid": "0b5c2c8e-7a4e-4f7e-9d2a-3c1e5f6a7b8c", "pid": 4242, "uptime": 11045, "start_time": 1760960000.2499998, `+
		`"peers": {"neighbor 127.0.0.1 local-ip 127.0.0.1 local-as 65535 peer-as 65535 router-id 127.0.0.1 family-allowed in-open": "ESTABLISHED"}}`+"\n",
		object.String())

	// No peer: the text has no peer line and the JSON an empty object.
	info.peers = nil
	info.start = time.Unix(1760960000, 0)
	object.Reset()
	info.write(&object, true)
	assert.Contains(t, object.String(), `"start_time": 1760960000.0, "peers": {}}`)
	text.Reset()
	info.write(&text, false)
	assert.True(t, bytes.HasSuffix(text.Bytes(), []byte("Peers: 0\n")), text.String())
}
