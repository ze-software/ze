---
spec: firewall-prefix-normalize
topic: Firewall
feature: firewall
---

## Before

A firewall source or destination prefix such as `10.10.10.1/30` was accepted
even though the installed rule matched the network `10.10.10.0/30`. The
configuration kept the host bits without showing that distinction.

## After

Validation, startup and configuration commits refuse IPv4 and IPv6 network
prefixes with host bits set. The error names the term and input, explains the
network it would match, and shows the corrected configuration statement. Use
`10.10.10.0/30` for that network, or `/32` or `/128` to match one host.

A refused reload keeps the active configuration and installed rules. Canonical
prefixes, default routes, host routes and named-set references stay accepted.

## Corrects

none

## Exposure

none

## Upstream

none

## Asked by

none
