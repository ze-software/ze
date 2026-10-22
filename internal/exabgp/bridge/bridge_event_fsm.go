// Design: docs/architecture/exabgp-bridge.md -- the connected, fsm and signal events
// Related: bridge_event.go -- ReadEvent and ExabgpJSON call these
// Related: bridge_event_state.go -- the down reason of the same session

package bridge

import "strconv"

// zeKindConnected is ze's `connected` event (internal/component/bgp/format,
// AppendConnected): the peer's TCP connection is established, before OPEN.
const zeKindConnected = "connected"

// readConnected turns ze's `connected` event into the state event ExaBGP
// writes for it. ExaBGP files a connection under `neighbor-changes` as a
// state, `{"state": "connected", "type": "state"}` (Response.JSON.connected,
// src/exabgp/reactor/api/response/json.py), and its text encoder writes
// `neighbor <ip> connected` (Response.Text.connected). Ze delivers it as an
// event of its own, so a plugin that reads `state` never meets a third value.
func (e *Event) readConnected() {
	e.Kind = msgTypeState
	e.Data = e.Payload
	e.Payload["state"] = zeKindConnected
}

// fsmState is the state an `fsm` event moved to, in ExaBGP's words. Ze's
// fsm.State names are ExaBGP's FSM names already (IDLE, ACTIVE, CONNECT,
// OPENSENT, OPENCONFIRM, ESTABLISHED; src/exabgp/bgp/fsm.py), and ExaBGP
// writes the new state alone (Response.JSON.fsm, `{'state': fsm.name()}`).
func (e Event) fsmState() string {
	to, _ := e.Data["to"].(string)
	return to
}

// writeSignal writes the members ExaBGP's signal line carries inside
// `neighbor`: the signal number as a string and its C name, `"code": "1",
// "name": "SIGHUP"` (Response.JSON.signal, src/exabgp/reactor/api/response/json.py,
// `{'code': str(signal), 'name': Signal.name(signal)}`). Ze's `signal` event
// carries the number as a JSON number (internal/component/bgp/format,
// AppendSignal). ExaBGP's text encoder writes no line for a signal
// (Response.Text.signal answers None), and neither does the bridge.
func (e Event) writeSignal(neighbor map[string]any) {
	number, _ := e.Data["code"].(float64)
	neighbor["code"] = strconv.Itoa(int(number))
	neighbor["name"], _ = e.Data["name"].(string)
}
