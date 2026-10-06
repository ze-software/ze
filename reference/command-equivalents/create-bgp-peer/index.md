# `create bgp peer`

Add a peer to the running daemon\.

## Ze command

- Registry path: `create bgp peer`
- Usage: `create bgp peer <selector> <asn> [local-as <local-as>] [local-address <local-address>] [router-id <router-id>] [receive-hold-time <receive-hold-time>] [send-hold-time <send-hold-time>] [connect-retry <connect-retry>] [connect <true\|false>] [accept <true\|false>] [family <family>] [graceful-restart <graceful-restart>] [group-updates <true\|false>] [attach <attach>]`
- Mode: Daemon
- Wire method: `ze-bgp:peer-add`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: none: this command takes no subcommand
- Answer shape: not declared
- Address fields: none
- Column order: peer, remote-as, message
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, when the answer has rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

The peer address and \'asn\' are required\. Every other keyword is optional\. Ze builds the peer\, starts it\, and dials the address unless \'connect false\' says otherwise\. The peer lives in the running daemon alone\. Ze writes nothing to the configuration\, so a reload removes the peer\. \'delete bgp peer\' does the same on the way out\, and it does not change the file on disk\. An address that already names a peer is refused\, and so is a keyword Ze cannot honor\. Each name in \'attach\' is bound to the whole surface\: that process receives every message from this peer and can send every type toward it\. Write the receive and send lists in the configuration file when the process needs less\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `selector` | union | yes | any value of this type | Address of the peer to create | The token typed after \'peer\' is an IPv4 or IPv6 address and nothing else\. A name\, a glob or an AS pattern is refused\, because a peer that does not exist yet matches none of them\. An address that already names a peer is refused\. |
| `asn` | union | yes | any value of this type | Remote AS of the peer\, in any notation RFC 5396 names | The value is 1 to 4294967295\, in asplain or asdot notation\. Ze stores the decimal form whichever notation you typed\. AS 0 is refused\, because RFC 7607 reserves it\. |
| `local-as` | union | no | any value of this type | Local AS for this session\, overriding the router\'s\, in any notation RFC 5396 names | The value is 1 to 4294967295\, in asplain or asdot notation\. Absent\, the session uses the router\'s own AS\. AS 0 is refused\. |
| `local-address` | union | no | any value of this type | Local address to bind the session to | The value is an IPv4 or IPv6 address of this router\. Ze binds the TCP session to it\. Absent\, the kernel picks the source address\. |
| `router-id` | string | no | any value of this type | BGP Identifier for this session\, overriding the router\'s | The value is a dotted quad and it MUST NOT be 0\.0\.0\.0\. Ze sends it as the BGP Identifier in the OPEN for this session\. Absent\, the session uses the router\'s own\. |
| `receive-hold-time` | uint | no | any value of this type | Hold time in seconds \(RFC 4271\) | The value is 0 to 65535 seconds and Ze puts it in the Hold Time field of its OPEN\. The two peers then hold the lower of the two proposals\. Absent\, the value is 90\. |
| `send-hold-time` | uint | no | any value of this type | Send hold time in seconds\, 0 for automatic \(RFC 9687\) | The value is 0 to 65535 seconds\. A non\-zero value MUST be at least 480 and more than the receive hold time\, or the peer is refused\. 0 derives it as the larger of 480 and twice the receive hold time\. |
| `connect-retry` | uint | no | any value of this type | ConnectRetry interval in seconds | The value is 0 to 65535 seconds and Ze reads it for a dynamic peer alone\. A disconnected dynamic peer is removed after this delay when no connection comes back\. A static peer reconnects on its own backoff\, which this value does not change\. Absent\, the value is 120\. |
| `connect` | enum | no | `true`, `false` | Dial the peer\, default true | The value is true or false and nothing else\. With false\, the session comes up only when the peer opens the TCP connection and \'accept\' lets it in\. |
| `accept` | enum | no | `true`, `false` | Accept a connection from the peer\, default true | The value is true or false and nothing else\. With false\, the session comes up only when \'connect\' lets Ze dial the peer\. |
| `family` | string | no | any value of this type | Address families to negotiate\, comma separated | Each name is an AFI\/SAFI pair such as ipv4\/unicast\, and a name Ze does not know is refused\. Ze negotiates each family in the OPEN\. Absent\, the session uses the configured default families\. |
| `graceful-restart` | uint | no | any value of this type | Graceful restart time in seconds \(RFC 4724\) | The value is 0 to 4095 seconds\. It is the Restart Time Ze advertises in the graceful restart capability of its OPEN\. Absent\, the session advertises none\. |
| `group-updates` | enum | no | `true`, `false` | Group this peer\'s UPDATEs with other peers\, default true | The value is true or false and nothing else\. Packing reduces the number of UPDATE messages from one per route to one per set of path attributes\. Use false for a peer that requires one prefix per UPDATE\. |
| `attach` | string | no | any value of this type | Plugin processes bound to this peer\, comma separated | Each name is a plugin process the configuration declares\. Ze binds the peer to each one with the whole surface\: the process receives every message from this peer and can send every type toward it\. An empty name is refused\. |

## Mapping intents

No vendor equivalent has been curated yet for this Ze command.

## Vendor equivalents

### Junos MX

No equivalent listed.

### IOS XR

No equivalent listed.

### SR OS

No equivalent listed.

### VyOS

No equivalent listed.
