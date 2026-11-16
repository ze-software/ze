# `clear irr as-set`

Remove the stored IRR dataset of an AS\-SET\.

## Ze command

- Registry path: `clear irr as-set`
- Usage: `clear irr as-set <as-set>`
- Mode: Daemon
- Wire method: `ze-irr:clear-irr-as-set`
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

Removes the stored dataset and pointer of one AS\-SET\. The answer names every consumer still referencing it\; those consumers stop enforcing it\, and it is not fetched again until update irr or a commit that newly names it\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `as-set` | string | yes | any value of this type | AS\-SET name\. | The RPSL set name the update stored\, for example AS\-EXAMPLE\. |

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
