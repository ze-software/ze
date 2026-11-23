# `show firewall irr prefix`

Show the prefixes the firewall enforces for a reference\.

## Ze command

- Registry path: `show firewall irr prefix`
- Usage: `show firewall irr prefix <name>`
- Mode: Read\-only
- Wire method: `ze-firewall:show-irr-prefix`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: none: this command takes no subcommand
- Answer shape: not declared
- Address fields: none
- Column order: none
- Pipes, always: json\, ndjson\, table\, text\, yaml\, raw\, no\-more\, save
- Pipes, when the answer has rows: match\, count\, first\, last\, display\, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Lists all IPv4 and IPv6 prefixes the firewall enforces for the given ASN or AS\-SET\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `name` | string | yes | any value of this type | ASN or AS\-SET name | The reference as the firewall config names it\: an ASN in the form AS64500\, or an AS\-SET name\. A name no rule references\, or one with nothing stored\, is an error\. |

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
