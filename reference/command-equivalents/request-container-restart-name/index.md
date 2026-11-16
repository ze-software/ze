# `request container restart name`

Stop the named container\, then start it again\.

## Ze command

- Registry path: `request container restart name`
- Usage: `request container restart name <value>`
- Mode: Daemon
- Wire method: `ze-container:request-restart`
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

A running container is stopped\, with its stop signal and then a kill after its stop timeout\, and started again as a new process\. A container in the stopped or failed state is started\. A disabled container and one that waits for its image are refused\, with the statement or command that lets it run\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `value` | string | yes | any value of this type | Name of the container\. | The name of a configured container\, as its instance entry spells it\. The word after the name keyword is the value\. |

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
