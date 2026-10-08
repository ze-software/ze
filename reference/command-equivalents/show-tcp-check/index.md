# `show tcp-check`

Test TCP connectivity to a remote host and port\.

## Ze command

- Registry path: `show tcp-check`
- Usage: `show tcp-check <host> <port> [source <source>] [timeout <timeout>]`
- Mode: Read-only
- Wire method: `ze-diag:show-tcp-check`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: none: this command takes no subcommand
- Answer shape: not declared
- Address fields: none
- Column order: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, when the answer has rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Tries to open a TCP connection and reports success or failure with the connection time\. Use \'source \<IP\>\' to bind a specific local address\. Quick way to verify a peer\'s BGP port is reachable\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `host` | string | yes | any value of this type | Target host\. | An IP address or a DNS name of at most 253 characters\, which the dialer resolves\. The answer echoes it in the host key\. |
| `port` | uint | yes | any value of this type | Target port\. | The TCP port to connect to\, from 1 to 65535\. Ze joins it to the host and dials that one endpoint\. |
| `source` | string | no | any value of this type | Source IP address\. | A local IP address the connection binds to before it dials\. An address the host does not hold makes the dial fail\. Absent\, the kernel picks the source\. |
| `timeout` | string | no | any value of this type | Connection timeout\. | How long the dial waits before it reports timeout\, from 1s to 30s\. The default is 5s\. |

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
