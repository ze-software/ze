# Week of 2026-02-09

A new chaos-testing tool, matured config reload, RFC 7606 enforcement, and a hot-path allocation cleanup.

## 🌪️ Chaos testing tool

A new dedicated tool, `ze-bgp-chaos`, landed this week: a seed-based fault-injection harness for exercising Ze's BGP route propagation. It grew from a single-peer session driver into a multi-peer validation framework with scripted chaos events (drops, resets, malformed messages) and coverage for seven address families (IPv4/IPv6 unicast, IPv4/IPv6 VPN, EVPN, IPv4/IPv6 FlowSpec).

## 🔁 Config reload

Live reload now runs through the hub/orchestrator architecture: plugins get a verify pass before an apply pass, and the reload coordinator is hardened so a crashing plugin can't corrupt the process (any plugin that dies mid-reload aborts the reload cleanly instead of applying a partial config). Committing in the config editor now triggers a live daemon reload over the API socket directly. A new `ze signal` command plus PID-file locking makes it possible to reload, stop, or query a running daemon without hunting down its PID, and prevents two instances from starting against the same config.

## 🛰️ Protocol correctness

- Malformed UPDATE messages are now handled per RFC 7606: depending on the error, Ze treats the route as withdrawn, discards just the bad attribute, or resets the session, instead of one blunt response for every case
- Early support for RFC 9234 BGP Roles: peers can declare a role, and the engine validates role pairs on OPEN, sending a NOTIFICATION on mismatch (OTC handling is still to come)
- `local-address` is now mandatory for every peer; leaving it unset made TCP source IP selection OS-dependent and produced inconsistent next-hop-self behaviour

## 🔄 ExaBGP migration

The migration tool now passes all 37 compatibility test configs, with L2VPN/VPLS migration completed and ExaBGP watchdog processes bridged through Ze's API socket.

## ⚡ Performance

The BGP UPDATE encoding path had its remaining heap allocations eliminated: attribute writes, SRv6 TLVs, withdrawals, and RIB NLRI paths all now write into pooled buffers instead of allocating per message.
