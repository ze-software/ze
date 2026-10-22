# Held page text: negotiated before the OpenConfirm KEEPALIVE

The reactor fix that announces `negotiated` on entry to OpenConfirm
(`Peer.announceNegotiated`, files `internal/component/bgp/reactor/{peer_derived_bindings,peer_derived_bindings_test,peer_run,peer_settings_apply,reactor_notify}.go`,
plus its `docs/architecture/api/json-format.md` hunk) is uncommitted: two of its
files carry another session's uncommitted policy rename. The paragraph below
describes it, so it was taken out of `docs/architecture/exabgp-bridge.md` to let
the rest of that page land (2026-10-22, main session ed4b8d60).

When the reactor fix lands, put this text back in the same commit, replacing
the paragraph that starts "Ze states the block once" in the section "The
negotiated block rides on every UPDATE line", then delete this file in that
commit.

```
Ze states the block once, in its own `negotiated` event, so each runner keeps it
per peer in a `bridge.Sessions`, from that event to the peer's `down`, and
hands it to every message of the peer: an UPDATE line and a packet line write
it, the other decoded lines do not. The engine emits `negotiated` on entry to
OpenConfirm, before the KEEPALIVE that answers the peer's OPEN
(`Peer.announceNegotiated`), so both OpenConfirm KEEPALIVE lines carry the
session's block, as the pin writes them, and the OPEN lines of a first session
carry UNSET. Which scripts get it follows the script's API
```
