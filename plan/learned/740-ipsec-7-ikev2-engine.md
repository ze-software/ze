# 740: IKEv2 Engine (ipsec-7)

## Context

Implemented the native IKEv2 state machine and engine at `internal/component/ike/engine/`.
This sits between the wire codec (ipsec-5) and crypto layer (ipsec-6) below,
and the child SA / dataplane layer (ipsec-8) above. It manages per-peer goroutines,
IKE_SA_INIT and IKE_AUTH exchanges, config reconciliation, and SA lifecycle events.

## Key Decisions

1. **Plugin registration, not direct wiring.** The IKE engine registers as a named plugin
   ("ike") with `ConfigRoots: ["vpn"]` via the SDK protocol. This means it receives config
   through the standard plugin pipeline rather than a bespoke hook, keeping it consistent
   with other components.

2. **JSON-to-Tree dual storage.** The SDK delivers config as JSON, but `ParseIPsecConfig`
   expects `*config.Tree`. A `treeFromMap` converter stores every nested map as both a
   container AND list entries. This handles the YANG container-vs-keyed-list ambiguity
   without schema knowledge. The heuristic "all-children-are-maps = keyed list" alone
   fails because containers like `site-to-site` can have only sub-map children.

3. **Per-peer goroutine lifecycle from PPPoE.** The `PeerSession` type mirrors `PPPoEClient`:
   Start/Stop, `run()` loop with reconnect backoff, `reconcilePeers` diffs config and
   starts/stops sessions. The peer config is stored directly in PeerSession (not only in SA)
   to avoid a data race between the reconciler reading config and the goroutine setting SA state.

4. **Constant-time PSK verification.** PSK AUTH comparison must use `subtle.ConstantTimeCompare`,
   not a byte-by-byte loop. A byte-at-a-time comparison leaks timing side-channel information
   that allows brute-forcing the derived key one byte at a time.

## Consequences

- The engine registers events (`vpn-ipsec/sa-up`, `vpn-ipsec/sa-down`) at init time.
  Any component can subscribe to IKE SA lifecycle notifications.
- Responder mode is a placeholder (waits on stopCh). Full responder exchange handling
  requires inbound SA creation in the dispatcher, planned for ipsec-8/9.
- `getRemoteCert` returns the CA certificate, not the remote peer's CERT payload.
  Proper X.509 verification requires storing the CERT payload from IKE_AUTH, deferred
  to ipsec-8/9 when the full encrypted payload decryption path is complete.
- The inbound rate limiter (100 pkt/s, burst 200) is a basic defense. RFC 7296 Section 2.6
  cookie mechanism should be added in a future spec for production DoS resistance.

## Gotchas

- `isKeyedList` (now `allMaps`) misclassifies YANG containers whose only children are
  sub-containers. The dual-storage fix works but means `GetContainer` and `GetListOrdered`
  both return data for the same key, which is unusual for config.Tree. Do not rely on the
  absence of one to infer the presence of the other.
- The IKE engine and ipsec component both register for ConfigRoots `["vpn"]`. Both receive
  the same config section. This is by design (ipsec owns the schema, engine consumes the
  parsed config), but watch for ordering issues if ipsec grows its own OnConfigure.

## Files

None recorded.
