# Week of 2026-03-02

Best-path selection landed, along with outbound route tracking and a round of CLI/editor polish.

## 🛰️ Routing

- On-demand best-path selection in the RIB (`rib show best`), covering LOCAL_PREF, AS_PATH length, ORIGIN, MED, eBGP/iBGP preference, ORIGINATOR_ID, and peer-address tiebreak per RFC 4271 §9.1.2
- A new persistence plugin tracks outbound routes per peer, retaining cache entries on send and releasing them on withdrawal, then replays the full outbound set with End-of-RIB markers on reconnect
- Graceful Restart Receiving Speaker support (RFC 4724 §4.2), with ADD-PATH state now threaded correctly through the format pipeline (RFC 7911)
- Fixed a watchdog bug where routes configured as withdrawn were being announced anyway on first session
- FSM state transitions now cover the full RFC 4271 §8.2.2 table

## 🖥️ CLI & editor

- New `ze show` and `ze run` split: read-only commands vs. everything including destructive ones, each RPC now declares its own safety level
- Dual-mode config editor (edit/command mode switching), `ze config set` with YANG validation and `--dry-run`, and `ze config edit` now offers to create a missing config file instead of failing
- Command history (up/down arrow), "did you mean?" suggestions across CLI dispatch, `--json` output for schema commands, and shell completion for bash and zsh
- New operational commands: `bgp summary`, peer capabilities, `peer clear soft`, and `config diff` (text and JSON)
- Fixed a capability-negotiation bug where comparisons only checked byte counts instead of actual wire bytes

## 🛠️ Under the hood

Panic recovery now wraps every long-lived BGP goroutine (peer loop, delivery, listener, reactor monitor): a fault inside one peer tears down and re-establishes that session instead of crashing the daemon. Builds now report a version (YY.MM.DD) and build date.
