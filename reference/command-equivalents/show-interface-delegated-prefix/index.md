# `show interface delegated-prefix`

Show the \/64 each LAN unit takes from a delegated prefix\, and its source\.

## Ze command

- Registry path: `show interface delegated-prefix`
- Usage: `show interface delegated-prefix`
- Mode: Read-only
- Wire method: `ze-iface:show-interface-delegated-prefix`
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

One object per delegated\-prefix entry of every enabled unit\, sorted by interface\, unit and source\: interface\, unit\, source \(the pppoe\-client whose lease delegates the prefix\)\, subnet\-id\, and state\. state is installed when the unit holds its address\, which adds device\, address \(the unit\'s address with its \/64 length\)\, prefix \(the derived \/64\)\, delegated \(the whole delegated prefix\)\, valid\-lifetime and preferred\-lifetime \(whole seconds remaining\, 4294967295 meaning infinite\)\. waiting means the source holds no delegated prefix\; too\-small means the delegated prefix cannot hold the subnet\-id\; not\-installed means the address install failed\. Each of those three carries a reason\, and the last two name the delegated prefix\.

## Arguments

No command-specific arguments listed.

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
