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

**A reload restarts a peer whose configuration differs in ANY member, and
leaves every other peer alone.** `peerConfigChanged` compares three whole values
rather than a list of member names: the peer (`ipsec.SiteToSitePeer.Equal`) and
the two RESOLVED crypto groups (`ipsec.IKEGroup.Equal`, `ipsec.ESPGroup.Equal`).

The groups are there because a peer holds their NAMES and none of the crypto.
`startPeerSession` copies the resolved groups onto the `PeerSession` and nothing
refreshes them, so an operator rotating a cipher edits no peer block at all: the
peer half compares equal, and the tunnel would keep negotiating the algorithm
that was replaced. `reconcilePeers` resolves both groups against the new config
before it asks, and a group that is gone resolves to the zero value, which stops
the peer. That is the same answer a fresh daemon gives: the start loop refuses a
peer whose groups do not resolve.

Two more properties follow, and both are load-bearing.

A member added to `SiteToSitePeer` forces a restart from the day it is added, so
an operator's edit cannot be ignored because nobody remembered to list it. A
member that MUST NOT force a restart is subtracted by name, with the reason
recorded on `Equal`. Omission is not how the two are told apart.

A reload that edits nothing restarts nothing. That needs every member to be
stable across two parses of one file, which `TestSiteToSitePeerEqualAcrossTwoParses`
holds: a peer whose selectors and allow list arrive behind fresh pointers on
each parse still compares equal. Without it, a total comparison would bounce
every tunnel on every commit.

**A reload that cannot be applied is REFUSED, not half-applied. Startup applies
the same configuration and says what cannot bind.** `interface` supplies the
local address of every peer that names none, so a failed interface read leaves
those peers unbindable. `unbindablePeers` names that condition, and it is the
ONLY thing the two deliveries answer differently: `applyPhase` is what carries
the difference into `applyIPsecConfig`.

A RELOAD returns the error, `OnConfigApply` propagates it, the transaction rolls
back and the running tunnels are untouched. Without it the peers would carry an
empty `LocalAddress`, differ from the sessions that resolved one at startup, and
every one of them would be stopped and restarted into a state that cannot bind.

STARTUP logs the condition and applies the configuration anyway. There is no
running tunnel to protect and no previous configuration to keep, so a refusal
would start no peer, no IKE socket and no NAT-T socket at all, including for the
peers that carry their own `local-address` and cannot be affected by the
interface. An interface that comes up after the daemon does is ordinary at boot.
`test/ipsec/ipsec-startup-serves-bindable-peers.ci` holds that asymmetry.

The restart is also the only way an edit reaches the wire. `startPeerSession` is
the only writer of `ps.peerCfg`, `initiator.go` and `responder.go` copy that
value into `sa.PeerCfg`, and `proposeChildTSPayloads` reads it to build the TSi
and TSr of the next CREATE_CHILD_SA. A session left running keeps proposing the
selectors it was born with. RFC 7296 Section 2.9.2 asks for the restart in the
narrowing case: a rekey that would need a narrower scope means the policy
changed, and the SA "should have been already deleted after the policy change
took effect".

**A reload reaches the engine through `OnConfigApply`, and an apply with nothing
staged is refused.** The plugin protocol splits a reload into a verify phase and
an apply phase, and the apply request carries diff sections rather than the
configuration, so `ikeConfigStaging` carries what verify parsed across to apply.
An apply that finds nothing staged returns `errIKEApplyWithoutVerify`: both
phases pick their participants with one predicate (`filterDiffs`), so that state
is a protocol violation, and answering OK would report the commit landed while
the engine kept the configuration it was already running.

The engine registered no `OnConfigApply` at all until
spec-fixit-ipsec-peer-reload-ignored. The SDK answers a config-apply with OK when
no handler is registered, so every reload verified the operator's edit, reported
success, and applied nothing: `reconcilePeers` was reachable from startup and
from operator `clear` alone.

<!-- source: internal/component/ike/ipsec/types.go -- SiteToSitePeer.Equal, IKEGroup.Equal, ESPGroup.Equal, IPsecConfig.Changed -->
<!-- source: internal/component/ike/engine/register.go -- applyIPsecConfig, applyPhase, ikeConfigStaging, unbindablePeers, peersNeedInterfaceAddress -->
<!-- source: pkg/plugin/sdk/sdk_callbacks.go -- OnConfigApply -->
<!-- source: internal/component/ike/engine/rekey.go -- proposeChildTSPayloads -->

**PSK AUTH comparison is constant time.** A byte-at-a-time comparison leaks
timing that lets an attacker brute-force the derived key one byte at a time.

**One function holds the AUTH construction, whatever the secret is.** RFC 7296
Section 2.15 gives one formula, and `computeAuthFromSharedSecret` is the only
place its pad string, its PRF order and its operand order are written. A
pre-shared key, an EAP MSK, and SK_pi or SK_pr each enter it as the secret
argument. `verifyPSKAuth` and the EAP verifier share `verifyAuthFromSharedSecret`
for the same reason, so a change to the construction moves the sender and the
receiver together.

<!-- source: internal/component/ike/engine/auth.go -- verifyPSKAuth, computePSKAuth -->
<!-- source: internal/component/ike/engine/eap_auth.go -- computeAuthFromSharedSecret, verifyAuthFromSharedSecret, constantTimeEqualAuth -->

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

**A discarded EAP packet is not a dead SA.** `handleEAPResponse` puts the SA in
`StateDead` for any non-nil `PeerResult.Err`, so reporting an error for a packet
RFC 3748 makes the peer drop would let one forged packet end the exchange. A
discard is `PeerResult.Discarded` instead. Nothing is sent, the SA stays in
`StateEAPInProgress`, and the retransmit timer is re-armed before the handler
returns, which is the state a peer that never received the packet would be in.
`maxEAPRounds` still counts the round, so a flood ends the exchange rather than
holding it open. The engine writes `ike: EAP packet discarded` with the Code, the
Type and the Identifier, because the silence RFC 3748 asks for is owed to the
authenticator and not to the operator.

**A Notification message reaches the operator through the log.** RFC 3748 Section
5.2 says the peer "SHOULD display this message to the user or log it if it cannot
be displayed". A daemon has no user to display it to, so the log line is the
display: `ike: EAP notification from the authenticator` at Info, with the peer
name and the message. The message is unauthenticated and is chosen by whoever
sent the packet, so it is passed as a slog value and is never built into a format
string, a path or a command. Ze sends the Notification Response on the same
round, because `PeerResult` carries `Notified` beside `Response`.

<!-- source: internal/component/ike/engine/fsm.go -- handleEAPResponse -->
<!-- source: internal/core/eap/peer.go -- PeerResult, peerDiscard, notificationResponse -->

<!-- source: internal/component/ike/engine/remote_id.go -- remote identity matching -->
<!-- source: internal/component/ike/engine/notify_error.go -- error notification emission -->
