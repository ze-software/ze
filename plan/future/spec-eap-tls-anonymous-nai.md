# Spec: eap-tls-anonymous-nai

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Deferral shard | `-` |
| Updated | 2026-08-12 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**Ze's EAP peer sends the operator's configured identity in the clear-text
Identity Response.** RFC 9190 Section 2.1.8 asks a peer to send an anonymous
Network Access Identifier there, and to reveal the real identity only inside the
TLS tunnel.

This covers two requirements: `RFC9190-2.1.8-1` and `RFC9190-2.1.8-2`.

### Why this spec is in `plan/future/`

`plan/future/README.md` refuses defects. **These two are unmet MUST-level
requirements, and this spec is here by an owner ruling rather than by
reclassification.**

Owner ruling, 2026-08-12: **"we are very unlikely to run the protocol over the
network and if we do it will be a trusted, secure network, it could be an
improvement but not urgent at all."**

That ruling rests on a property of Ze's transport that is worth writing down,
because it is what makes the deferral reasonable rather than merely convenient:
**in Ze the Identity Response travels inside an already-encrypted, already
authenticated channel.** `verifyRemoteAuth` runs before `startEAPExchange`
(`internal/component/ike/engine/fsm.go`), so the identity goes inside SK{} to a
responder Ze has already authenticated. The clear-text exposure Section 2.1.8
was written for is an 802.1X property and does not exist on this transport.

**The residual risk, stated so a later reader does not have to rediscover it:**
the identity is still exposed onward if the gateway relays it into a RADIUS
chain. Ze as the client cannot control that.

No annotation was written into `rfc/requirements/rfc9190.md`; the rows stay
unproven and honest. RFC 9190 is not enrolled, so no ratchet fires.

### Current behavior

`startEAPExchange` (`internal/component/ike/engine/fsm.go`) takes
`sa.PeerCfg.Auth.LocalID`, falling back to `sa.PeerName`.
`NewPeerSessionTLS` (`internal/core/eap/peer.go`) sends that value
verbatim in the Identity Response.

**The server half is already conformant.** `Session.handleIdentity`
(`internal/core/eap/eap.go`) stores whatever arrives and starts the
method: no lookup, no length check, an empty identity accepted. So
`RFC9190-2.1.8-1` is satisfied on the responder side today.

### The decision this spec must put to the owner before implementation

**The new configuration leaf's name and its default.**

`local-id` cannot be reused. The same leaf drives the IKE IDi payload
(`encodeIKEID`, `internal/component/ike/engine/auth.go`), so changing it changes
Ze's IKE identity. The anonymous NAI needs its own leaf beside it in
`internal/component/ike/ipsec/yang/ze-ipsec-conf.yang`.

**Deriving it from `local-id` does not work.** `encodeIKEID` sends any non-IP
value as ID_FQDN and never as ID_RFC822_ADDR, so operators are pushed toward FQDN
or IP values, and there is usually no `@realm` to keep. RFC 7542 Section 2.2
defines no conformant NAI without a realm. `stripDomain`
(`internal/core/eap/mschapv2.go`) splits on a backslash rather than `@`,
so no existing helper does this.

**A no-config version is possible and is NOT sufficient:** sending `@realm` when
`local-id` happens to carry one, and the value verbatim otherwise, would be
fix-sized, and it would leave `RFC9190-2.1.8-2` violated for every FQDN and IP
`local-id`, which is the majority shape. That is a partial-conformance choice and
it belongs to the owner.

### Interop

Wire-visible, and already covered rather than blocked:
`test/interop-ipsec/scenarios/eap-tls13` sets `eap_id = %any` in its
`swanctl.conf`, so an anonymous NAI still authenticates. The scenario should be
TIGHTENED to assert the octets Ze sends, not replaced.
