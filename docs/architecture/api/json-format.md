# Ze JSON Output Format

**Purpose:** Document Ze's JSON output format for plugin communication.

---

## Overview

Ze outputs JSON messages to external processes via stdout. All messages follow the IPC Protocol format with a top-level `type` field indicating which key contains the payload.

---

## Message Structure

All messages have a top-level `type` field. Event type is in the `message` object:

```json
{
  "type": "<namespace>",
  "<namespace>": {
    "message": {"type": "<event-type>", "id": <n>, "direction": "<dir>"},
    "peer": {"local": {"address": "<local-ip>", "as": <local-asn>}, "remote": {"address": "<ip>", "as": <n>}},
    ...event-specific fields...
  }
}
```

This structure keeps message metadata (type, id, direction) together in the `message` object.
<!-- source: internal/core/ipc/message.go -- MapResponse -->

### Namespaces

| Namespace | Description |
|-----------|-------------|
| `bgp` | BGP protocol events (UPDATE, OPEN, etc.) |
| `rib` | RIB events (cache, route changes) |
| `response` | API command responses |

### The IPC envelope

```json
{"type":"bgp","bgp":{"peer":{"group":"...","name":"...","local":{"address":"...","as":N},"remote":{"address":"...","as":N}},"message":{"id":N,"direction":"received","type":"open"},"open":{...}}}
```

Envelope and value conventions:

- Error: `{"error":"description","parsed":false}`
- CLI: `{"status":"ok","data":{...}}` or `{"status":"error","error":"msg"}`
- Raw hex: uppercase, no `0x`. `"parsed":false` plus `"raw":"DEADBEEF"`
- Numbers decode as `float64` in Go, so integer display goes through `formatNumber()`

---

## Address families

A family is written `afi/safi`. Plugins register families at startup, so this
is not a static list: `./le repo inventory` prints what the build actually carries,
and the registry is the authority.

| AFI | SAFIs |
|-----|-------|
| ipv4 | `unicast`, `multicast`, `mpls-label`, `mvpn`, `sr-policy`, `mup`, `mpls-vpn`, `rtc`, `flow`, `flow-vpn` |
| ipv6 | `unicast`, `multicast`, `mpls-label`, `mvpn`, `sr-policy`, `mup`, `mpls-vpn`, `flow`, `flow-vpn` |
| l2vpn | `evpn`, `vpls` |
| bgp-ls | `bgp-ls`, `bgp-ls-vpn` |

Unicast and multicast are built into the engine. Every other family arrives
with a `bgp-nlri-*` plugin, so removing that plugin removes the family.
<!-- source: internal/core/family/family.go -- SAFI constants -->
<!-- source: internal/core/family/registry.go -- MustRegister -->

---

## Field naming

JSON keys are lowercase kebab-case, never camelCase and never snake_case. The
key matches the YANG leaf name or config tree key it carries, so the config key
`remove-private-as` is `json:"remove-private-as"`.

| Wrong | Right | Why |
|-------|-------|-----|
| `json:"remove-private"` | `json:"remove-private-as"` | Truncated; the key must match the full config key |
| `json:"policyAttrASPathRemovePrivate"` | `json:"remove-private-as"` | camelCase is not kebab-case |
| No `json` tag on an exported field | `json:"kebab-name"` | Go's default marshaling leaks the PascalCase field name |

`internal/component/lg/handler_api.go` is the one exemption: the looking-glass
API keeps birdwatcher-convention `snake_case` for Alice-LG compatibility.

---

## BGP Events

All BGP events have the same structure: type in `message`, `peer` and event-specific data at `bgp` level:

```json
{
  "type": "bgp",
  "bgp": {
    "message": {"type": "<event-type>", "id": <n>, "direction": "<dir>"},
    "peer": {"local": {"address": "<local-ip>", "as": <local-asn>}, "remote": {"address": "<ip>", "as": <n>}},
    ...event-specific fields...
  }
}
```

### Common Fields

| Field | Type | Description |
|-------|------|-------------|
| bgp.message.type | string | Event type (update, state, open, etc.) |
| bgp.message.id | int | Message identifier (omitted if 0) |
| bgp.message.direction | string | "received" or "sent" (omitted if empty) |
| bgp.source-policy-epoch | uint64 | Original received source-policy admission on UPDATE events; omitted when unrecorded or locally originated. Independent of message ID, dispatch session generation and source-DOWN revocation |
| bgp.peer.remote.address | string | Peer IP address |
| bgp.peer.remote.as | int | Peer AS number. Omitted while a dynamic peer's AS is not yet negotiated: the OPEN Ze sends it, a NOTIFICATION sent in OpenSent, and the `connected` and `fsm` events before OpenConfirm carry none. AS 0 is never written, because no peer can hold it (RFC 7607 Section 2) |
| bgp.peer.remote.router-id | string | The PEER's BGP Identifier, dotted quad. Omitted before the peer's OPEN is read |
| bgp.peer.router-id | string | THIS speaker's BGP Identifier for the session, dotted quad. Omitted when the peer sets none |

The two `router-id` keys name two different speakers, and a consumer that
compares BGP Identifiers MUST read `remote.router-id`. RFC 4271 Section 9.1.2.2
step f) prefers the route from the peer with the lowest BGP Identifier, so a
consumer that reads the top-level key compares this speaker's identifier against
itself, ties on every pair, and falls through to step g).
<!-- source: internal/component/bgp/format/text.go -- appendPeerJSON -->

Received UPDATEs capture `source-policy-epoch` before ingress policy evaluation.
Forwarded sent events retain the actual advertised owner's original epoch.
Retained-route replay MUST preserve it; a zero or missing epoch does not grant
received-origin announcement authority. Ordinary reconnect does not change the
policy epoch. This is a typed transport field outside `route-meta`, not a
user-configurable `meta/*` key. Consumers MUST decode it as an exact uint64 rather
than round it through a floating-point value.

The parsed text encoding carries the same value as the reserved header pair
`source-policy-epoch <uint64>` immediately after the UPDATE message ID and before
attributes. The raw and empty text UPDATE lines have no message ID and carry no
epoch, so a text reader records zero there (`text-parser.md`). The in-process
`StructuredEvent` and raw message carry `SourcePolicyEpoch` directly.
<!-- source: internal/component/bgp/format/text_update.go -- appendSourcePolicyEpochJSON, appendSourcePolicyEpochText -->
<!-- source: internal/component/bgp/server/events.go -- getStructuredEvent -->
<!-- source: internal/component/bgp/event.go -- Event, ParseEvent -->

### Event Types

| Type | Description |
|------|-------------|
| state | Peer state change (up, down) |
| update | UPDATE message |
| open | OPEN message |
| keepalive | KEEPALIVE message |
| notification | NOTIFICATION message |
| refresh | ROUTE-REFRESH message |
| negotiated | Capabilities negotiated |

---

## State Events

State events have the state value at `bgp` level:

```json
{
  "type": "bgp",
  "bgp": {
    "message": {"type": "state"},
    "peer": {"local": {"address": "192.0.2.2", "as": 65000}, "remote": {"address": "192.0.2.1", "as": 65001}},
    "state": "up",
    "initial-replay": "42"
  }
}
```

State values: `"up"`, `"down"`. A TCP connection that is established before
any OPEN is the `connected` event below, not a third state value, so a plugin
that reads every state other than `up` as a down never meets it.
<!-- source: internal/component/bgp/format/text_json.go -- appendStateChangeJSON -->

When a Session receipt exists, `initial-replay` is its opaque nonzero uint64
encoded as a decimal string, not a JSON number. A readiness-reporting process
keeps the receipt from the original event and sends
`request peer <address> plugin session ready session <receipt>` after its work,
including an empty replay. The engine validates that receipt against the live
Session and the process's still-open readiness share atomically. A late event
must not acknowledge a replacement Session. Native text events carry the same
receipt as an `initial-replay <receipt>` suffix; the ExaBGP compatibility
encoders do not expose it.

For `"down"` events, a `reason` field is included:

```json
{
  "type": "bgp",
  "bgp": {
    "message": {"type": "state"},
    "peer": {"local": {"address": "192.0.2.2", "as": 65000}, "remote": {"address": "192.0.2.1", "as": 65001}},
    "state": "down",
    "reason": "hold timer expired",
    "session-generation": "3"
  }
}
```

A down also carries `session-generation`: the forward generation of the
session the down ended, a nonzero uint64 encoded as a decimal string and
captured when that session closed, so a down read after a reconnect still
names the old session. A forwarding plugin stamps the departure it sends for
the down with the same value as `recovery-generation` route metadata, and the
engine owes only routes that session or an older one of the source wrote
(`docs/architecture/plugin/rib-storage-design.md`, "Source-DOWN replacement and sent ordering").
Native text events carry it as a ` session-generation <n>` suffix, and the
structured (DirectBridge) form as `StructuredEvent.SessionGeneration`. An up
carries none.
<!-- source: internal/component/bgp/format/text_json.go -- appendStateChangeJSON -->
<!-- source: pkg/plugin/rpc/bridge.go -- StructuredEvent -->

When the reason is `notification`, the down also names the NOTIFICATION that
closed the session: which way it went (`sent` or `received`) and its code and
subcode. A NOTIFICATION ze built and could not write told the peer nothing, so
it is not named. Every other down carries no `notification` member.

```json
{"type":"bgp","bgp":{"message":{"type":"state"},"peer":{...},"state":"down","reason":"notification",
 "notification":{"direction":"sent","code":6,"subcode":4}}}
```

The text form appends ` notification <direction> <code> <subcode>` after the
reason: `peer 192.0.2.1 remote as 65001 state down reason notification notification sent 6 4`.
<!-- source: internal/component/bgp/format/session_events.go -- appendCloseNotificationJSON -->

### Connected and FSM Events

A process subscribed to `connected` hears that a peer's TCP connection is
established, before any OPEN is read on it (RFC 4271 Section 8.2.2, Event 17
into OpenSent). A process subscribed to `fsm` hears every FSM state change,
with the state it left and the state it entered, named IDLE, CONNECT, ACTIVE,
OPENSENT, OPENCONFIRM or ESTABLISHED. The `connected` event comes before the
`fsm` event of the same change.

```json
{"type":"bgp","bgp":{"message":{"type":"connected"},"peer":{...}}}
{"type":"bgp","bgp":{"message":{"type":"fsm"},"peer":{...},"fsm":{"from":"CONNECT","to":"OPENSENT"}}}
```

Text: `peer 192.0.2.1 remote as 65001 connected` and
`peer 192.0.2.1 remote as 65001 fsm CONNECT OPENSENT`. Neither has a
structured (DirectBridge) form: every process takes the line.

The engine queues `connected`, `fsm` and `signal` on each process's delivery
queue and does not wait for the process to answer, because it emits them from
the FSM before the OPEN goes out and from the hub before a stop or reload
starts. A slow or wedged subscriber therefore delays neither. Each process
still receives its events in the order they were emitted; a failed write is
logged.
<!-- source: internal/component/bgp/server/events.go -- deliverPeerLine -->
<!-- source: internal/component/bgp/format/session_events.go -- AppendConnected -->
<!-- source: internal/component/bgp/format/session_events.go -- AppendFSM -->
<!-- source: internal/component/bgp/reactor/peer_fsm_events.go -- emitFSMEvents -->

A process subscribed to `signal` hears each operating-system signal the daemon
takes (SIGHUP, SIGINT, SIGTERM), once for every configured peer. The event is
queued before the daemon reloads or stops; a stop signal is announced once
the stop has begun and a second signal can already skip the drain, so a
subscriber that does not read it cannot hold the stop. The hub publishes it on the EventBus
(`daemonevents.Signal`, namespace `daemon`) and the BGP reactor tells each
peer's processes. A stop asked for by `daemon shutdown` or by the end of stdin
is not a signal and is not told.

```json
{"type":"bgp","bgp":{"message":{"type":"signal"},"peer":{...},"signal":{"code":15,"name":"SIGTERM"}}}
```

The text line is `peer 192.0.2.1 remote as 65001 signal 15 SIGTERM`.
<!-- source: internal/component/bgp/format/session_events.go -- AppendSignal -->
<!-- source: internal/component/bgp/reactor/reactor_signal_events.go -- onDaemonSignal -->
<!-- source: cmd/ze/hub/signal_announce.go -- announceSignal -->

---

## UPDATE Events

### Structure

Attributes and NLRIs are inside `bgp.update`. The peer's address is
`bgp.peer.remote.address`, not `bgp.peer.address`. Full-format raw bytes are
inside `bgp.raw`, alongside `bgp.update`.

```json
{
  "type": "bgp",
  "bgp": {
    "message": {"type": "update", "id": 1, "direction": "received"},
    "peer": {"local": {"address": "192.0.2.2", "as": 65000}, "remote": {"address": "192.0.2.1", "as": 65001}},
    "update": {
      "attr": {
        "origin": "igp",
        "as-path": [65001, 65002]
      },
      "nlri": {
        "ipv4/unicast": [
          {"next-hop": "192.0.2.1", "action": "add", "nlri": ["10.0.0.0/24", "10.0.1.0/24"]},
          {"action": "del", "nlri": ["172.16.0.0/16"]}
        ]
      }
    }
  }
}
```

### With Raw Wire Bytes (format=full)

```json
{
  "type": "bgp",
  "bgp": {
    "message": {"type": "update", "id": 1, "direction": "received"},
    "peer": {"local": {"address": "192.0.2.2", "as": 65000}, "remote": {"address": "192.0.2.1", "as": 65001}},
    "update": {
      "attr": {
        "origin": "igp",
        "as-path": [65001]
      },
      "nlri": {
        "ipv4/unicast": [
          {"next-hop": "192.0.2.1", "action": "add", "nlri": ["10.0.0.0/24"]}
        ]
      }
    },
    "raw": {
      "attributes": "40010100400200040001fde8",
      "nlri": {"ipv4/unicast": "180a0000"},
      "update": "..."
    }
  }
}
```

The OPEN, NOTIFICATION, KEEPALIVE and ROUTE-REFRESH events of a `full`
subscriber carry their bytes too, beside the parsed object, under the name the
`raw` format gives them: `"raw": {"message": "<hex>"}`, the body after the
19-octet header. A KEEPALIVE has no body, so its member is the empty string; a
subscriber of any other format is written no `raw` member. An End-of-RIB marker
of a `full` subscriber is the `eor` object, as for a `parsed` one, with
`"raw": {"update": "<hex>"}` beside it. The header is not delivered: RFC 4271
Section 4.1 fixes it as the Marker, the Length and the Type, which the body and
the event state. The ExaBGP bridge subscribes `full` whenever a script
receives an UPDATE or a packet form: it writes ExaBGP's `packets` and
`consolidate` lines from these members, and reads every UPDATE's AS_PATH,
extended community values and unknown attribute flags from `raw.update`
(`docs/architecture/exabgp-bridge.md`).

<!-- source: internal/component/bgp/format/json_message.go -- JSONEncoder.Message -->
<!-- source: internal/component/bgp/format/text_update.go -- appendEORUpdateJSON -->
<!-- source: internal/component/bgp/format/text_json.go -- appendFilterResultJSON -->
<!-- source: internal/component/bgp/format/text_update.go -- appendFullFromResult -->

### Operation Format

Each address family has an array of operations under `nlri`:

```json
"nlri": {
  "<family>": [
    {"next-hop": "<ip>", "action": "add", "nlri": [...]},
    {"action": "del", "nlri": [...]}
  ]
}
```

- `next-hop`: Present only for "add" operations
- `action`: "add" (announce) or "del" (withdraw)
- `nlri`: Array of NLRI values
- Each family is one key. Its array holds every operation the UPDATE carries
  for that family: announces first (the body NLRI field, then MP_REACH_NLRI),
  then withdrawals (the body Withdrawn Routes field, then MP_UNREACH_NLRI). A
  peer may send several of these fields for one family in one UPDATE, because
  RFC 7606 Section 5.1 says "an implementation MUST still be prepared to
  receive these fields in any position or combination". A key stated twice
  would lose its first copy in any reader that decodes the object into a map.
- A consumer applies a family's `del` operations before its `add` operations,
  not in array order. One UPDATE can withdraw and announce the same prefix, and
  RFC 4271 Section 4.3 settles which wins: "A BGP speaker SHOULD treat an
  UPDATE message of this form as though the WITHDRAWN ROUTES do not contain the
  address prefix." Applied in array order, the withdrawal would remove the
  route the same UPDATE announced.
- When an NLRI field (the body NLRI or Withdrawn Routes field, an MP_REACH_NLRI
  or an MP_UNREACH_NLRI) does not split into routes, the operation carries no
  `nlri` member. It carries the error convention instead: `"error"` names why,
  `"parsed": false`, and `"raw"` holds the field's bytes in uppercase hex, for
  example
  `{"action": "del", "error": "parsing NLRI at offset 0: nlri: short read", "parsed": false, "raw": "402001"}`.
  An empty `nlri` list is never written for a field that failed to split, because
  it would read as an operation on no route.
- When a field splits but one NLRI in it is not a route Ze can read (the
  family's plugin decoder refuses its octets, or no loaded plugin decodes an
  opaque family), that element of the `nlri` list uses the same convention:
  `{"error": "<reason>", "parsed": false, "raw": "<hex>"}`, with `path-id` too
  when the NLRI carried one. `raw` holds the NLRI's octets as received, Path
  Identifier included. The element carries no `prefix`, so a consumer cannot
  install it as a route, and the ExaBGP bridge leaves it out with a warning.
  <!-- source: internal/component/bgp/format/text_json.go -- appendNLRIJSONValue, appendUndecodedNLRIJSON -->
  <!-- source: internal/exabgp/bridge/bridge_event.go -- undecodedNLRI -->

Two producers write this member, and both write it the same way. The parsed
JSON event with no attribute or NLRI filter is written straight from the wire
(`appendParsedUpdateJSONDirect`); it covers the body NLRI and Withdrawn
Routes fields and the MP_REACH_NLRI and MP_UNREACH_NLRI fields, an IPv4
unicast MP_REACH_NLRI included, and merges them per family key
(`appendUpdateNLRIJSONDirect`). Every other JSON form, `format=full` and a
selective attribute or NLRI filter included, is written from the filter's
result (`appendFilterResultJSON`), whose families carry the split failure
(`FamilyNLRI.SplitErr`), the IPv4 NLRI and Withdrawn Routes fields of the
UPDATE body included, and merges them per family key (`appendFamiliesJSON`).
The text event form has no spelling for a field that did not split and writes
no route for it.
<!-- source: internal/component/bgp/format/text_json.go -- appendFilterResultJSON, appendFamiliesJSON -->
<!-- source: internal/component/bgp/format/text_update.go -- appendParsedUpdateJSONDirect, appendNLRISplitErrorJSON, appendUpdateNLRIJSONDirect, appendIPv4FieldJSON -->
<!-- source: internal/component/bgp/filter/filter.go -- FamilyNLRI, AnnouncedByFamily, WithdrawnByFamily -->
<!-- source: internal/component/bgp/format/text_human.go -- appendFilterResultText -->

### Attributes

Attributes appear under the `attr` object:

| Attribute | Format |
|-----------|--------|
| origin | `"origin": "igp"` |
| as-path | `"as-path": [65001, 65002]` |
| med | `"med": 100` |
| local-preference | `"local-preference": 100` |
| atomic-aggregate | `"atomic-aggregate": true` (the attribute has no value; its presence is the statement) |
| aggregator | `"aggregator": "65001:192.0.2.1"` (`<asn>:<address>`, asplain; a two-octet AGGREGATOR is widened at ingest, so the AS number is the aggregating node's whichever width the peer sent) |
| as4-aggregator | `"as4-aggregator": "4200000000:192.0.2.1"` (same form; a received UPDATE never carries it, because ingest folds a received AS4_AGGREGATOR into `aggregator`, RFC 6793 Section 4.2.3) |
| communities | `"communities": ["65001:100", "65001:200"]` |
| large-communities | `"large-communities": ["65001:0:100"]` |
| extended-communities | `"extended-communities": ["target:65000:1", "rate-limit:0"]` (named; a subtype the vocabulary does not name keeps its octets as `0x<type><subtype>:<hex>`) |
| ipv6-extended-communities | `"ipv6-extended-communities": ["000c2a02..."]` (hex) |
| aigp | `"aigp": 123` |
<!-- source: internal/core/bgp/attribute/json.go -- RegisterJSONFormatter, GetJSONFormatter (registry) -->
<!-- source: internal/core/bgp/attribute/register.go -- core attribute formatters -->
<!-- source: internal/component/bgp/plugins/filter_community/json.go -- community attribute formatters -->
<!-- source: internal/component/bgp/plugins/aigp/register.go -- AIGP formatter -->

---

## NLRI Formats

### Simple Prefixes (IPv4/IPv6 Unicast)

Without ADD-PATH:
```json
"nlri": ["10.0.0.0/24", "10.0.1.0/24"]
```

With ADD-PATH:
```json
"nlri": [{"prefix": "10.0.0.0/24", "path-id": 1}]
```

The negotiated layout decides the shape, never the identifier's value. RFC 7911
Section 3 reserves no value for the Path Identifier, so a route whose identifier
is zero is written as `{"prefix": "10.0.0.0/24", "path-id": 0}`. The bare string
form says the session carried no identifier for that route.

<!-- source: internal/component/bgp/format/text_json.go -- appendNLRIJSONValue -->

### Labeled Unicast (MPLS)

```json
"nlri": [{"prefix": "10.0.0.0/24", "labels": [[100, 1600], [200, 3201]]}]
```

Each member of the stack is the PAIR the wire carries: the 20-bit label a reader
matches on, and the three-octet stack entry it came from, which holds the traffic
class and the bottom-of-stack bit. The entry is dropped when it is zero, the one
case where it says nothing the label did not. Label 200 above is the bottom of
the stack, so its entry is `200 << 4 | 1`.

<!-- source: internal/component/bgp/plugins/nlri/labeled/json.go -- AppendJSON -->
<!-- source: internal/component/bgp/plugins/nlri/labeled/encode.go -- labelPairs -->

A withdrawal names the route by its prefix alone, with no `labels`:

```json
"ipv4/mpls-label": [{"action": "del", "nlri": ["10.0.0.0/8"]}]
```

Under ADD-PATH the member is `{"prefix": "10.0.0.0/8", "path-id": 7}`. The
withdrawn NLRI carries a Compatibility field where the announcement carried its
label stack, and RFC 8277 Section 2.4 says "Upon reception, the value of the
Compatibility field MUST be ignored". The event is the same whatever the field
held: 0x800000, zero, or a value that reads as a label. A withdrawal Ze sends
carries 0x800000 in one three-octet field, whatever label or label stack the
command named, because the same section says "Upon transmission, the
Compatibility field SHOULD be set to 0x800000."

<!-- source: internal/component/bgp/reactor/session_egress_normalize.go -- setWithdrawalCompatibility -->
<!-- source: internal/component/bgp/plugins/cmd/update/update_text_nlri.go -- parseNLRISection -->

<!-- source: internal/component/bgp/wireu/mpwire.go -- ParseWithdrawnNLRIs -->
<!-- source: internal/component/bgp/format/text_json.go -- appendNLRIJSONValue -->

### IPVPN (VPNv4/VPNv6)

```json
"nlri": [{"prefix": "10.0.0.0/24", "rd": "0:65000:1", "labels": [[100, 1601]]}]
```

<!-- source: internal/component/bgp/plugins/nlri/vpn/json.go -- AppendJSON -->

### EVPN Type 2 (MAC/IP)

```json
"nlri": [{
  "route-type": "mac-ip-advertisement",
  "rd": "0:65000:1",
  "esi": "00:11:22:33:44:55:66:77:88:99",
  "ethernet-tag": 100,
  "mac": "aa:bb:cc:dd:ee:ff",
  "ip": "10.0.0.1",
  "labels": [200]
}]
```

### FlowSpec

```json
"nlri": {
  "ipv4/flowspec": [
    {
      "next-hop": "192.0.2.1",
      "action": "add",
      "nlri": [{
        "destination-ipv4": ["10.0.0.0/8"],
        "destination-port": ["=80", "=443"],
        "protocol": ["=6"],
        "string": "flow destination-ipv4 10.0.0.0/8 ..."
      }]
    }
  ]
}
```

Next-hop is at the **operation level** (same as all other families), not inside the NLRI object.
<!-- source: internal/component/bgp/format/text.go -- NLRI family formatting -->

### FlowSpec-VPN

```json
"nlri": [{"rd": "0:65000:1", "spec": "destination 10.0.0.0/24 protocol tcp"}]
```

### BGP-LS

```json
"nlri": [{"code": 1, "parsed": false, "raw": "0001..."}]
```

---

## OPEN Events

```json
{
  "type": "bgp",
  "bgp": {
    "message": {"type": "open", "id": 1, "direction": "received"},
    "peer": {"local": {"address": "192.0.2.2", "as": 65000}, "remote": {"address": "192.0.2.1", "as": 65001}},
    "open": {
      "asn": 65001,
      "router-id": "1.1.1.1",
      "timer": {"hold-time": 90},
      "capabilities": [
        {"code": 1, "name": "multiprotocol", "value": "ipv4/unicast"},
        {"code": 65, "name": "asn4", "value": "65001"}
      ]
    }
  }
}
```
<!-- source: internal/component/bgp/format/json.go -- JSONEncoder.Open -->

Capabilities with a structured formatter use that formatter's value, as above.
Other capabilities use lowercase hexadecimal for the value bytes only, without
the capability code and length octets. For example, GR wire TLV
`4006000300010180` has code `64` and value `000300010180`. An empty capability
has no value bytes. This contract applies to both received and sent OPEN events.

A capability that lists one entry per family (Add-Path, Extended Next Hop,
Paths-Limit) gives one list member per family. An Extended Next Hop or a
Paths-Limit capability that lists no family is still one member, with no value,
because the peer sent it. An OPEN in the RFC 9072 extended envelope, whose
parameters carry two-octet lengths, lists its capabilities the same way as one
in the RFC 4271 envelope.
<!-- source: internal/component/bgp/format/decode.go -- DecodedCapability, formatCapability, parseCapabilitiesFromOptParams -->

---

## NOTIFICATION Events

```json
{
  "type": "bgp",
  "bgp": {
    "message": {"type": "notification", "id": 3, "direction": "sent"},
    "peer": {"local": {"address": "192.0.2.2", "as": 65000}, "remote": {"address": "192.0.2.1", "as": 65001}},
    "notification": {
      "code": 6,
      "subcode": 2,
      "code-name": "Cease",
      "subcode-name": "Administrative-Shutdown",
      "data": ""
    }
  }
}
```
<!-- source: internal/component/bgp/format/text.go -- AppendNotification -->

---

## KEEPALIVE Events

```json
{
  "type": "bgp",
  "bgp": {
    "message": {"type": "keepalive", "id": 42, "direction": "sent"},
    "peer": {"local": {"address": "192.0.2.2", "as": 65000}, "remote": {"address": "192.0.2.1", "as": 65001}},
    "keepalive": {}
  }
}
```
<!-- source: internal/component/bgp/format/text.go -- AppendKeepalive -->

---

## Negotiated Capabilities

Sent once the OPENs are exchanged, on entry to OpenConfirm: before the
KEEPALIVE that answers the peer's OPEN, and so before the `state` up the
session's Established brings. A session that ends in OpenConfirm sent it and
never reaches up.

```json
{
  "type": "bgp",
  "bgp": {
    "message": {"type": "negotiated"},
    "peer": {"local": {"address": "192.0.2.2", "as": 65000}, "remote": {"address": "192.0.2.1", "as": 65001}},
    "negotiated": {
      "timer": {"hold-time": 90},
      "asn4": true,
      "message-size": 4096,
      "route-refresh": "enhanced",
      "families": ["ipv4/unicast", "ipv6/unicast"],
      "add-path": {
        "send": ["ipv4/unicast"],
        "receive": ["ipv4/unicast"]
      },
      "extended-next-hop": {"ipv4/unicast": "ipv6"}
    }
  }
}
```

`message-size` is the largest message this speaker may send (65535 once the
peer advertised RFC 8654 Extended Message, 4096 otherwise). `route-refresh` is
`absent`, `normal` or `enhanced` (RFC 7313). `add-path`, `extended-next-hop`
and `paths-limit` appear only when they hold a family. `families` and the two
`add-path` lists keep the order the peer's OPEN listed its Multiprotocol
capabilities in, filtered by what ze sent, the order ExaBGP lists them in; the
order is the same on every event of one session.
<!-- source: internal/component/bgp/format/json.go -- JSONEncoder.Negotiated -->
<!-- source: internal/core/bgp/capability/negotiated.go -- Negotiate, Negotiated.Families -->
<!-- source: internal/component/bgp/reactor/peer_derived_bindings.go -- Peer.announceNegotiated -->

---

## RIB Events

```json
{
  "type": "rib",
  "rib": {
    "type": "cache",
    "action": "new",
    "msg-id": 12345,
    "peer": {"local": {"address": "192.0.2.2", "as": 65000}, "remote": {"address": "192.0.2.1", "as": 65001}}
  }
}
```

---

## Response Format

API command responses:

```json
{
  "type": "response",
  "response": {
    "serial": "1",
    "status": "done",
    "data": {...}
  }
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| response.serial | string | If request had serial | Correlation ID |
| response.status | string | Always | "done", "error", "warning", or "ack" |
| response.partial | bool | If streaming | true for intermediate chunks |
| response.data | any | Optional | Payload or error message |
<!-- source: internal/component/plugin/types.go -- Response -->

---

## Text Format

Text format does not use JSON wrapping. Parsed text events use `peer <ip> remote as <asn> ...`. While a dynamic peer's AS is not yet negotiated, `<asn>` is the word
`unknown`, so the line keeps its token positions: `peer 192.0.2.1 remote as unknown sent notification 2 ...`.

**State:**
```
peer 192.0.2.1 remote as 65001 state up
```

**UPDATE:**
```
peer 192.0.2.1 remote as 65001 received update 1 origin igp path 65001 next 192.0.2.1 nlri ipv4/unicast add 10.0.0.0/24
```

**OPEN:**
```
peer 192.0.2.1 remote as 65001 received open 1 asn 65001 router-id 1.1.1.1 hold-time 90 cap 1 multiprotocol ipv4/unicast
```

**KEEPALIVE:**
```
peer 192.0.2.1 remote as 65001 sent keepalive 42
```

**NOTIFICATION:**
```
peer 192.0.2.1 remote as 65001 sent notification 3 code 6 subcode 2 code-name Cease subcode-name Administrative-Shutdown data
```
<!-- source: internal/component/bgp/format/text_human.go -- appendStateChangeText, appendFilterResultText -->
<!-- source: internal/component/bgp/format/text.go -- AppendOpen, AppendKeepalive, AppendNotification, AppendRouteRefresh -->

---

**Last Updated:** 2026-01-31
