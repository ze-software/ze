---
spec: pipe-chain-honored-on-every-command-surface
topic: API
---

## Before

A pipe chain worked only where Ze showed the answer itself: SSH, the web CLI
and `ze cli`. A plugin's `dispatch-command` and the MCP, REST and gRPC execute
calls refused a line such as `show bgp rib | received | count`, and suggested
the argument form `show bgp rib received count`. A REST or gRPC stream of a
monitor ignored its chain. A quoted value after the first pipe was torn at
any pipe or space inside it, so `| match "error|warn"` ran `match "error` and
refused `warn"`, and `| match "link down"` searched for the quotes too.
`monitor ... | match x` refused every event. A `ze cli -c` monitor stopped
with Ctrl+C never wrote its `| save` file. The TACACS+ accounting record held
the rewritten command, not the line the operator typed.

## After

Every surface answers a piped line the same way `ze cli -c` does: a plugin,
MCP, REST and gRPC get the chain applied, a format such as `| table` included,
and REST and gRPC streams apply it to each event. A double-quoted value is one
argument anywhere on the line, and an unfinished quote is refused with its
column. The RIB's scope, filter and count words are pipe operations only:
`show bgp rib received count` is refused with `show bgp rib | received |
count` as the fix, and `show bgp peer <name> rib | received` replaces the
scope typed after `rib`. `show command help <command>` and
`ze help command --json` list every pipe operation a command answers to in one
`pipe-operations` list, which `|` completion offers. REST, gRPC and the MCP
tools also take an ordered `pipe` list. A plugin can declare its own pipe
operations. `monitor ... | match x` keeps or drops each event whole, a
stopped `ze cli -c` monitor writes its `| save` file, and accounting records
the line as typed, with secrets masked.

## Corrects

The earlier note that these surfaces refuse a pipe chain and that the RIB's
filters are sent as arguments (`show bgp rib received count`).

## Exposure

none

## Upstream

none

## Asked by

none
