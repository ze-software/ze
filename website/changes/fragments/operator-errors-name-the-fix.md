---
spec: operator-errors-name-the-fix
topic: CLI
---

## Before

Many refusals said only what failed. A reload of a broken file printed
`reload: parse config: parse config: line N`, and `ze config validate` could
name the line twice. A stray `}` was refused as `expected keyword, got
RBRACE`. When every TACACS+ server failed, the message dropped each server's
cause. `ze cli` with no daemon asked "is the daemon running?" without naming a
command. An API announce whose next hop did not fit the route family said
`next-hop incompatible with family`. An NTP server that was not synchronised
was reported as a kiss of death. `unit 9999` was refused by a generic range
check before the VLAN message could speak, and a command with several faults
named only the first, or answered `missing selector`.

## After

These refusals now say what went wrong, why, and the command that fixes it, on
a line of its own starting `fix:`. Each prefix and line number appears once.
The stray `}` says it closes no section and to delete it. TACACS+ names every
server with its cause (bad secret, refused, timeout). The next-hop refusal
names the route family, the next hop, the session family and three fixes.
`ze cli` names the address it tried, then `ze status`, or `ze start` when no
daemon runs. A config that needs a feature this build left out names the
`ze_` build tag. NTP reports an unsynchronised server as unsynchronised. The
VLAN range and the IS-IS system-id format speak their own message. A command
with several faults lists each one with its usage. The editor help for
`discard` and `clear dns cache` describes what they do today.

## Corrects

none

## Exposure

none

## Upstream

none

## Asked by

none
