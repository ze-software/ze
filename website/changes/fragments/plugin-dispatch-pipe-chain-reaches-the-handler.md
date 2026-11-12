---
spec: plugin-dispatch-pipe-chain-reaches-the-handler
topic: API
---

## Before

A command line sent whole to the daemon with a pipe chain in it, such as
`show bgp rib received | count`, was answered as if the chain were not there.
This hit a plugin's `dispatch-command` and the MCP, REST and gRPC execute
calls. The command received the pipe and its operators as extra arguments:
it either refused them with an error about the wrong thing, or answered its
whole result, unfiltered, with a success status.

## After

Such a line is refused before any command runs. The error names the command
and the chain, says that the chain is applied by the surface that shows the
answer, and gives the line to send instead. When the chain only uses filters
the command owns, that line is the command with those filters as arguments
(`show bgp rib received count`). Otherwise it is the command alone, and the
error also names the `ze cli -c` form that applies the chain. A pipe inside
double quotes is still an ordinary argument. SSH, the web CLI and `ze cli`
are unchanged.

## Corrects

none

## Exposure

none

## Upstream

none

## Asked by

none
