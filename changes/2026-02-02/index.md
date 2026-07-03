# Week of 2026-02-02

Mostly spent on the config editor, a new capability, and a round of decode/RIB bug fixes.

## 🔌 Link-local next-hop capability

A new plugin adds the link-local-nexthop capability (code 77, draft-ietf-idr-linklocal-capability), a zero-payload flag signalling willingness to receive IPv6 link-local addresses as BGP next-hops per RFC 2545 §3.

## 🧰 Config editor

The `set` command now actually modifies config (it was a stub before), and the `load` command was redesigned with explicit syntax: `load <file|terminal> <absolute|relative> <replace|merge>`, including a paste-from-terminal mode. Peer names containing spaces, quotes, or backslashes are now handled correctly in `edit`/`set`, and tab completion for peer selectors shows actual configured peers instead of schema field names.

## 🐛 Fixes

- A plugin RPC routing bug sent commands like `bgp watchdog announce` and `bgp cache list` to the wrong handler; both now dispatch correctly
- Decoded IPv6 NLRI now returns plain prefix strings, matching IPv4 output instead of wrapping them in an extra object
- RIB event parsing now correctly reads sent UPDATE events in Ze's JSON format, fixing route replay for reconnect and graceful-restart scenarios
- A capability decoder bug that rejected flag-only capabilities with no payload (breaking the new link-local-nexthop plugin) is fixed
- Early ExaBGP migration work: automatic Extended Message capability (RFC 8654) and hostname/domain-name migration to the hostname capability block
