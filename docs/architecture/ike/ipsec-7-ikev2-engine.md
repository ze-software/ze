# The IKEv2 engine

The native IKEv2 state machine. It sits above the wire codec and the crypto
layer and below the Child SA and dataplane layer. It owns the per-peer
goroutines, the IKE_SA_INIT and IKE_AUTH exchanges, config reconciliation, and
the SA lifecycle events.

<!-- source: internal/component/ike/engine/fsm.go -- runOnce, runInitiator, runResponder, handleInbound, handleSAInitResponse, handleAuthResponse -->
<!-- source: internal/component/ike/engine/sa.go -- SA, SAState, GenerateSPI, GenerateNonce -->
<!-- source: internal/component/ike/engine/table.go -- SATable -->
<!-- source: internal/component/ike/engine/reconcile.go -- PeerSession, reconcilePeers, startPeerSession -->
<!-- source: internal/component/ike/engine/register.go -- runEngine, dispatchInbound, newInboundRateLimiter -->

## Decisions

**The engine registers as a named plugin, it is not wired directly.** It
registers as `ike` over the SDK protocol and claims the `vpn` and `pki` config
roots, so it receives config through the standard plugin pipeline instead of a
bespoke hook. It runs as a plugin subprocess, which is why it holds no interface
backend and takes the XFRM interface id from config.

**Config arrives as JSON and is stored as both container and list.** The SDK
delivers JSON, and `ParseIPsecConfig` expects a `config.Tree`. `treeFromMap`
stores every nested map as a container AND as list entries. The `allMaps`
heuristic, "every child is a map, so this is a keyed list", fails on its own,
because a container such as `site-to-site` can have only sub-map children. Dual
storage removes the need for schema knowledge at that boundary.

<!-- source: internal/component/ike/engine/config.go -- treeFromMap, allMaps -->

**Per-peer goroutine lifecycle, taken from the PPPoE client.** `PeerSession` has
Start and Stop, a `run` loop with reconnect backoff, and `reconcilePeers` diffs
the config and starts or stops sessions. The peer config is stored on the
`PeerSession`, not only on the SA, so the reconciler reading config cannot race
the goroutine setting SA state.

<!-- source: internal/component/ike/engine/fsm.go -- reconnectDelay -->
<!-- source: internal/component/ike/engine/reconcile.go -- peerConfigChanged -->

**PSK AUTH comparison is constant time.** A byte-at-a-time comparison leaks
timing that lets an attacker brute-force the derived key one byte at a time.

<!-- source: internal/component/ike/engine/auth.go -- verifyPSKAuth, computePSKAuth -->

**The engine emits SA lifecycle events.** `vpn-ipsec/sa-up` and
`vpn-ipsec/sa-down` are registered at init time, so any component can subscribe.

**The two events are a PAIR, and both owner loops produce both.** A path that
emits `sa-up` for an IKE SA emits exactly one `sa-down` when that SA goes down.
It emits that down on every way out, and never a second one for the same SA.

`runInitiator` and `runResponder` each emit `sa-up` at establishment. Each then
calls `emitSADown` when its `runEstablished` returns. Each also clears the
session's SA at that point, so the operator teardown paths find nothing left to
emit a second down for. Subscribers count the two against each other, so an
unpaired emit drifts once per reconnect rather than once per process.

<!-- source: internal/component/ike/engine/events.go -- SA lifecycle event registration -->
<!-- source: internal/component/ike/engine/fsm.go -- runInitiator, runResponder, the emitSADown pair -->

## RFC obligations carried by this code

- RFC 7296 Section 2.6 defines the COOKIE mechanism. The responder issues a
  cookie challenge under load and the initiator retries the IKE_SA_INIT with the
  cookie. The inbound rate limiter, 100 packets per second with a burst of 200,
  is a floor under that, not a replacement for it.
- RFC 7296 Section 2.15 governs the AUTH payload, and the signed octets differ
  by role. See `docs/architecture/ike/ipsec-14-responder.md`.

<!-- source: internal/component/ike/engine/cookie.go -- cookie generation and validation -->
<!-- source: internal/component/ike/engine/doctor_cookie.go -- cookie readiness check -->
<!-- source: internal/component/ike/engine/sa_init_retry.go -- retry on COOKIE and on INVALID_KE_PAYLOAD -->

## Traps this code exists to avoid

**Dual storage means neither accessor implies the other.** Because a nested map
is stored as a container AND as list entries, `GetContainer` and
`GetListOrdered` both return data for the same key. Do not read the absence of
one as evidence about the other.

**Remote identity policy is separate from the certificate check.** The peer's
claimed ID is matched against config, not inferred from the certificate. That
lookup has its own file.

<!-- source: internal/component/ike/engine/remote_id.go -- remote identity matching -->
<!-- source: internal/component/ike/engine/notify_error.go -- error notification emission -->
