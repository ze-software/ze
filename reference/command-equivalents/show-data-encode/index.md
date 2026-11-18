# `show data encode`

Encode supplied bytes as a ZeFS frame\, header or checksum\.

## Ze command

- Registry path: `show data encode`
- Usage: `show data encode <content> [part <full|header|crc>] [capacity <capacity>]`
- Mode: read\-only\, offline
- Wire method: `ze-config-storage:show-data-encode`
- Backends: any backend
- Task support: optional\: the MCP call is synchronous\, which is the default
- Subcommands: none: this command takes no subcommand
- Answer shape: blob
- Address fields: none
- Column order: none
- Pipes, always: json\, ndjson\, table\, text\, yaml\, raw\, no\-more\, save
- Pipes, on its rows: none
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Encode supplied bytes with the same netcapstring producer as ze data encode\. No store is read or changed\. The answer is the exact frame\, header or CRC bytes\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `content` | string | yes | any value of this type | Padded base64 input bytes | The bytes to encode\, in padded base64\. Empty input encodes an empty frame\. The encoded bound leaves space for the command within one SSH request\. |
| `part` | enum | no | `full`, `header`, `crc` | Frame part to return | Choose the complete frame\, its header or its checksum\. Header and checksum answers end with a newline\, as the standalone command does\. |
| `capacity` | uint | no | any value of this type | Reserved frame capacity in bytes | Reserved frame bytes\, at least the input length\. The default adds ten percent to the input length\. The maximum leaves room for a header in one remote answer\. |

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
