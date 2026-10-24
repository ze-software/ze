# `request peer notify`

Close a peer session with a NOTIFICATION of any code and subcode\.

## Ze command

- Registry path: `request peer notify`
- Usage: `request peer <selector> notify <code> <subcode> [hex <hex>] [text <text>]`
- Mode: Daemon
- Wire method: `ze-bgp:peer-notify`
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

Ze closes the session with a NOTIFICATION carrying the error code and error subcode given\, any value from 0 to 255\. Under Cease \(6\)\, subcode 2 or 4\, the words after the subcode are the RFC 9003 Shutdown Communication\; under every other pair they are the Data field\. Both are sent as UTF\-8\. With no words the Data field is empty\. A text that does not fit is refused and the session stays up\: more than 128 octets under Cease 2 or 4 \(RFC 9003 Section 3\)\, more than 4075 octets otherwise\, or text that is not UTF\-8\. With the word hex after the subcode\, the next word is the Data field itself in hexadecimal\, sent octet for octet with no encoding\, and refused over 4075 octets\. teardown is the Cease\-only form\; notify is the form for testing another implementation\, and the hex form is the one the ExaBGP bridge sends for teardown with a code\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `selector` | string | yes | any value of this type | An address or peer name\; \* or a pattern for refresh\, borr\, eorr and flush | The value is an IP address\, a peer name\, an AS pattern such as as65001\, a glob\, or \* for every peer\. \'refresh\'\, \'borr\'\, \'eorr\' and \'flush\' act on every peer it matches\. \'teardown\'\, \'notify\'\, \'disable\'\, \'enable\'\, \'pause\'\, \'resume\' and \'clear soft\' act on one session\: they refuse \*\, an exclusion such as \!edge1\, and a selector that matches more than one peer\, and they accept a pattern or glob that matches exactly one peer\. |
| `code` | uint | yes | any value of this type | NOTIFICATION error code | The NOTIFICATION error code\, a whole number from 0 to 255\. |
| `subcode` | uint | yes | any value of this type | NOTIFICATION error subcode | The NOTIFICATION error subcode\, a whole number from 0 to 255\. |
| `hex` | string | no | any value of this type | Data field octets in hexadecimal | The Data field in hexadecimal\, two digits for each octet\, as one word typed after the keyword hex in place of a text\. It is sent as it is\, so it can carry octets that are not UTF\-8 or a Shutdown Communication whose length octet is wrong\. More than 4075 octets\, an odd number of digits\, or a character other than 0\-9\, a\-f or A\-F is refused\. |
| `text` | string | no | any value of this type | Data field text | Every word after the subcode\, or after the keyword text\, joined by one space\. No word in it is read as a keyword\, so a text can hold the words code\, subcode\, text or hex\. Absent\, the Data field is empty\. |

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
