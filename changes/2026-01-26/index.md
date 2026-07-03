# Week of 2026-01-26

Config work, the ExaBGP migration path, and correctness fixes across the wire.

## 🧩 Family plugins

BGP-LS, VPN, EVPN and FlowSpec are now standalone family plugins with their own CLI decode/encode mode, human-readable output by default, and JSON or text input. A two-phase startup (explicit plugins first, then auto-load by family) avoids conflicts when more than one plugin could claim the same NLRI type. Capability decoding is plugin-driven too: unknown capabilities now show as `code=N` with raw hex instead of being opaque, and a new hostname (FQDN) capability plugin injects per-peer hostname/domain into the OPEN message.

## 🔄 ExaBGP migration

The ExaBGP config migration tool now converts `announce`/`static` blocks to Ze's native `update {}` syntax across all 38 test configs (up from 2). Legacy ExaBGP config syntax (`announce`/`static`/`flow`/`l2vpn`) has been removed from Ze entirely; route announcements now go exclusively through native `update { attribute {} nlri {} }` blocks, which also gained watchdog support for route withdrawal control.

## 🐛 Correctness fixes

- AS_PATH prepending is now RFC 4271 compliant: Ze no longer double-prepends the local AS when the configured path already starts with it
- Connection collision resolution (RFC 4271 §6.8) no longer risks a panic under load; it uses proper channel synchronization for session teardown instead of a timed sleep
- An explicit `hold-time 0` (disable keepalives, RFC 4271) is preserved instead of being silently reset to the 90s default
- Route Distinguisher is now parsed inline with the NLRI (as RFC 8955/4364 define it) instead of as a path attribute, consistent across API, config, and VPN families
- FlowSpec's OR-of-AND filter encoding now round-trips correctly

## 🧰 Config editor and CLI

The editor gained mandatory peer-as validation with template inheritance, a prompt to resume pending edits left from a previous session, and a new `ze config edit` command. `config check`, `dump`, `migrate`, `validate` and `schema` all accept input on stdin now, useful for scripting.

## 🛠️ Under the hood

RIB storage moved to per-attribute deduplication, cutting memory use for peers carrying large overlapping route sets.
