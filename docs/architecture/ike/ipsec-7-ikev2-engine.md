# The IKEv2 engine

The native IKEv2 state machine. It sits above the wire codec and the crypto
layer and below the Child SA and dataplane layer. It owns the per-peer
goroutines, the IKE_SA_INIT and IKE_AUTH exchanges, config reconciliation, and
the SA lifecycle events. Every change it makes to the peer roster advances the
dataplane generation: `reconcilePeers` when it starts or stops a peer, and
`setActivePeers` when it publishes a new map. A kernel read that spans one of
those answers unknown rather than drift.

<!-- source: internal/component/ike/engine/fsm.go -- runOnce, runInitiator, runResponder, handleInbound, handleSAInitResponse, handleAuthResponse -->
<!-- source: internal/component/ike/engine/sa.go -- SA, SAState, GenerateSPI, GenerateNonce -->
<!-- source: internal/component/ike/engine/table.go -- SATable -->
<!-- source: internal/component/ike/engine/reconcile.go -- PeerSession, reconcilePeers, startPeerSession -->
<!-- source: internal/component/ike/engine/register.go -- runEngine, dispatchInbound, newInboundRateLimiter -->

## Decisions

**The engine registers as a named plugin, it is not wired directly.** It
registers as `ike` over the SDK protocol and claims the `vpn` and `pki` config
roots, so it receives config through the standard plugin pipeline instead of a
bespoke hook. Config-path auto-loading runs it as an internal plugin goroutine
in the daemon, with a `DirectBridge` after the SDK handshake. The command
handlers read the same `ActiveTable`, peer map and dataplane backend that this
engine updates. The XFRM interface id comes from peer config.

<!-- source: internal/component/plugin/server/startup_autoload.go -- getConfigPathPlugins -->
<!-- source: internal/component/plugin/process/process.go -- startInternal -->

**The engine publishes its live tunnels through a registered inventory, so a
feature outside this component reads them without importing it.** `init()`
registers `inventorySnapshot` with `internal/core/ipsecinventory`, a
standard-library-only leaf. The snapshot is built from the same `PeerInfoMap`
that `show vpn ipsec sa` reads, under the same locks, so a reader sees what the
operator sees: one value-type `Tunnel` per active peer session, sorted by name,
carrying the configured and the installed endpoints, the interface id, the
encapsulation, the mode and the NEGOTIATED ESP transform, as its name and as the
RFC 7296 transform ids. A peer whose Child SA is down still appears with `Up`
false, so a reader tells "down" from "absent". A build that does not link this
package makes `ipsecinventory.Tunnels` answer `ErrNotRegistered`, which is a
named outcome distinct from a registered engine holding no tunnel. The leaf was
added for the path MTU diagnostic (spec-path-mtu-diagnostic), its first reader.

<!-- source: internal/component/ike/engine/inventory.go -- inventorySnapshot, tunnelOf -->
<!-- source: internal/core/ipsecinventory/registry.go -- Register, Tunnels, Tunnel -->

**Startup opens the sockets before it starts peers.** `OnConfigure` applies the
configuration and starts the IKE and NAT-T receive loops, then stages the peer
configuration for `OnAllPluginsReady`. On successful startup, that callback runs
after the startup phases finish and their subscriptions are registered.
A failed phase also triggers the callback, so it does not prove that every
configured plugin became ready. It starts peer goroutines without waiting for
an exchange.

`ike engine configured` therefore proves config delivery, and `ike peers started`
proves the startup reconciliation ran. Neither line proves an SA established.
An initiator inserts its SA before sending IKE_SA_INIT. A responder inserts one
only when `tryResponderSAInit` admits a received request from a configured peer.
The establishment lines come from `runInitiator` and `runResponder` after the
handshake reaches `StateEstablished`.

Shutdown runs the reverse. `runEngine` stops each peer session, removes the peer
from the map, and advances the dataplane generation for each removal.

<!-- source: internal/component/ike/engine/register.go -- runEngine, tryResponderSAInit -->
<!-- source: internal/component/ike/engine/fsm.go -- runInitiator, runResponder -->
<!-- source: internal/component/plugin/server/startup.go -- runPluginStartup, signalStartupComplete -->

**Config uses the canonical plugin-map inverse.** The SDK delivers JSON, and
`ParseIPsecConfig` expects a `config.Tree`. `parseIPsecFromJSON` calls
`config.TreeFromPluginMap`, preserving delivered list order and repeated values.
The wire shape does not distinguish a container of maps from a keyed list, so
the inverse exposes both views rather than guessing from the IPsec schema.
PKI sections go through `pki.ParseJSON`, which uses the same inverse.

<!-- source: internal/component/ike/engine/config.go -- parseIPsecFromJSON, loadPKIFromJSON -->
<!-- source: internal/component/config/plugin_map.go -- TreeFromPluginMap -->

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
the difference into `ikeEngineState.applyConfig`.

The refusal runs FIRST, above every mutation the apply makes. A refused reload
rolls the transaction back, so anything applied ahead of the refusal survives a
commit the operator was told had failed. The cookie threshold and the operator's
SPD entries were applied above the check until 2026-09-20 and did exactly that.

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

**A reload that moves the listen address rebinds both sockets, and it stops the
peers before it closes them.** `ikeListeners` records the configured listen host.
IKE binds that host; NAT-T binds a wildcard on a migration-capable backend, except
under the test-port override. Every SA selects its own local source for writes.
Replies instead use the request's received destination: this includes the initial
IKE_SA_INIT success, its cached retransmission, COOKIE and INVALID_KE responses.
The wildcard socket must not let route selection substitute another local address.
When the configured host changes, `rebindListeners` stops
every session, closes both sockets, opens a new pair, and lets the reconcile
start every peer against it.

The order is the whole of it. `PeerSession.ike` and `PeerSession.natt` are
immutable after `startPeerSession`, so a session that survives a rebind holds a
closed file descriptor and every message it sends is refused. The case that
needs the sequence is the one `peerConfigChanged` cannot see: the operator edits
`vpn ipsec interface`, every peer carries its own `local-address`, and the peer
half of the configuration compares equal while the socket under it moves.
`TestReloadRestartsAnUneditedPeerWhoseSocketMoved` holds it.

Both sockets were created only while their pointer was nil until 2026-09-20, and
nothing rebuilt them. A reload that moved the address half-applied: the peers
restarted, because `peerConfigChanged` saw `LocalAddress` change, and the socket
under them stayed bound to the address the daemon started with.

The remote-access address pool is rebuilt on the same rule, comparing the whole
`ipsec.VirtualIPPool` rather than its range alone, because the DNS servers and
the search domain are pushed to a client too. Nothing reads that pool yet:
`eap.Pool.Allocate` has no non-test caller, so ze negotiates no
INTERNAL_IP4_ADDRESS and a client receives no address from it. What the rebuild
changes today is that an edited range is validated and reported on the commit
that makes it.

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
<!-- source: internal/component/ike/engine/apply.go -- ikeEngineState, applyConfig, rebindListeners, stopAllPeers, reloadPool -->
<!-- source: internal/component/ike/engine/register.go -- applyPhase, ikeConfigStaging, unbindablePeers, peersNeedInterfaceAddress -->
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

## The padded path probe on the wire

`show mtu` measures a live tunnel's path with an INFORMATIONAL request the
owner loop pads to an exact datagram size (`engine/probe.go`; the loop, the
window and the outcomes are on `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md`).
The request carries exactly one payload: a status Notify of the private-use
type `ZE_PATH_PROBE_PADDING` (65280, `wire.NotifyZePathProbePadding`) with an
empty SPI, whose zeroed Notification Data is the padding. RFC 7296 Section 3.10.1
has a peer that does not know the type ignore it and answer with an empty
INFORMATIONAL response; SK padding is not the carrier, because its Pad Length
is one octet (Section 3.14) and an AEAD suite pads nothing (RFC 5282).

The Notification Data length is the requested size minus every other octet of
the datagram. `probeNotifyOctets` computes it from the SA's suite and send path:

| Octets | Part | Present |
|--------|------|---------|
| 20 | IPv4 header | always (the transport is `udp4`; IPv6 is refused `family`) |
| 8 | UDP header | always |
| 4 | non-ESP marker (RFC 3948 Section 2.2) | when the SA sends from port 4500 (`SA.sendPath`) |
| 28 | IKE header | always |
| 4 | SK payload generic header | always |
| 16 or 8 | IV | 16 under CBC, 8 under an AEAD suite (RFC 5282 Section 3.1) |
| 4 + 4 | Notify generic header, then Protocol ID, SPI Size, Notify Message Type | always |
| N | Notification Data | the padding |
| 0..15 | SK Padding | CBC only, to the 16-octet block |
| 1 | Pad Length | always |
| 16 or 12, or the AEAD tag | ICV | the negotiated integrity truncation under CBC; inside the ciphertext under AEAD |

Under CBC the encrypted span is a multiple of 16, so the reachable datagram
sizes sit on a 16-octet grid. A request between two grid points is rounded
DOWN to the grid point below it, never up: a datagram larger than the one asked
for is the one thing a path probe must never send, and a smaller one that fits
is a true lower bound. The size sent travels back in the answer
(`ikeprobe.Result.WireOctets`), so the caller reads the outcome at that size:
asked 1400 on a suite whose grid reaches 1392, the datagram is 1392 octets and
the fit is a fit at 1392. Under AEAD every size from the smallest datagram to
the 3000-octet ceiling (Section 2, `transport.MaxMsgSize`) is reachable and the
size sent is the size asked. The first copy leaves with Don't Fragment set; the
retransmission is the same bytes from the IKE header on (Section 2.1) with DF
clear. A probe is one IKE message and is never IKE-fragmented: Ze negotiates no
RFC 7383 fragmentation today (`wire.NotifyFragmentationSupported` is declared
and never sent), and an implementation of it MUST leave the probe exchange whole
whatever threshold it negotiates, because RFC 7383 Section 2.5.2 searches
fragmentation thresholds downward and a fragmented probe would measure the
threshold rather than the path (`plan/immediate/spec-ike-fragmentation-rfc7383.md`).

<!-- source: internal/component/ike/engine/probe.go -- probeNotifyOctets -->
<!-- source: internal/component/ike/wire/payload_notify.go -- NotifyZePathProbePadding -->

## RFC obligations carried by this code

- RFC 7296 Section 2.6 defines the COOKIE mechanism. The responder issues a
  cookie challenge under load and the initiator retries the IKE_SA_INIT with the
  cookie. The inbound rate limiter, 100 packets per second with a burst of 200,
  is a floor under that, not a replacement for it.
- RFC 7296 Section 2.15 governs the AUTH payload, and the signed octets differ
  by role. See `docs/architecture/ike/ipsec-14-responder.md`.
- RFC 7296 Section 2.23 detects a NAT by comparing the peer's NAT_DETECTION
  hashes against the addresses this node runs on. The SA records the verdict in
  three fields, not one. `NATDetected` answers "is there a NAT", and it selects
  UDP encapsulation and starts the keepalive. `BehindNAT` says THIS node's
  address was translated, and `PeerBehindNAT` says the peer's was. Both can be
  true at once, which is the two-NAT case of Section 2.23.1.
- The two side fields are what Section 2.23.1's transport-mode selector
  substitution is written in terms of, so neither is derivable from
  `NATDetected`: both detection branches set that one. Each is written at all
  four NAT_DETECTION branches, two on `fsm.go` for the initiator and two on
  `detectResponderNAT` for the responder, and both are carried across an IKE SA
  rekey by the two producers in `rekey.go`.
- `OriginalTSiAddr` and `OriginalTSrAddr` hold the traffic-selector addresses as
  they arrived, before that substitution replaced them. Section 2.23.1 requires
  the originals kept for the [UDPENCAPS] "real source and destination address"
  and for the TCP/UDP checksum fixup.

<!-- source: internal/component/ike/engine/sa.go -- NATDetected, BehindNAT, PeerBehindNAT, OriginalTSiAddr, OriginalTSrAddr -->
<!-- source: internal/component/ike/engine/ts_nat_substitute.go -- substituteResponderSelectors, substituteInitiatorSelectors -->

<!-- source: internal/component/ike/engine/cookie.go -- cookie generation and validation -->
<!-- source: internal/component/ike/engine/doctor_cookie.go -- cookie readiness check -->
<!-- source: internal/component/ike/engine/sa_init_retry.go -- retry on COOKIE and on INVALID_KE_PAYLOAD -->

## MOBIKE owner state

`mobike.go` owns the negotiated capability, current local tuple, pending COOKIE2
request and queued address update. The established loop services it on the same
one-second tick and request window as rekey, DPD and Delete. Retransmissions keep
their original bytes and Message ID. A request sent from multiple source addresses
is followed by a fresh INFORMATIONAL request, even if its old COOKIE2 matches.
A missing or mismatched COOKIE2 closes the IKE SA.

The MOBIKE role belongs to the first IKE SA, not the latest IKE rekey's initiator.
`inheritSendPath` retains that role. Only the original initiator selects a new
address pair; a responder accepts an explicit `UPDATE_SA_ADDRESSES` and checks
return routability before changing the Child SA.
Promotion refreshes that path from the old SA: a COOKIE2 check started while an
IKE replacement awaited Delete is reissued under the new keys and Message ID.

Authenticated reply tuples are transient. A valid request receives its answer at
the observed source, but an ordinary packet cannot replace the stored MOBIKE
endpoint. `NO_NATS_ALLOWED` is checked against the actual IP addresses and UDP
ports before address updates, Deletes or other state changes. A mismatch receives
`UNEXPECTED_NAT_DETECTED`; none of the protected tuple is adopted.

`SiteToSitePeer.ProhibitNAT` comes from `nat-traversal prohibit`; its zero value
retains the existing allow policy. The first IKE_AUTH producer and the MOBIKE
address-update producer serialize their chosen source/destination IPs and ports
into `NO_NATS_ALLOWED`. A retransmission retains those authenticated bytes; after
an address change the fresh request protects the new tuple.

The policy is enforced before establishment and migration, not only advertised.
A detected NAT refuses IKE_AUTH under prohibition. A translated NAT-detection
answer cannot enable ESP-in-UDP during migration, even with a correct COOKIE2.
A prohibiting responder rejects unprotected MOBIKE address updates before changing
its endpoint. Allow mode keeps the existing NAT behavior.
If first IKE_AUTH has not demonstrated RFC 4555 support through
`MOBIKE_SUPPORTED` or `NO_NATS_ALLOWED`, a local NAT-policy refusal uses the base
`AUTHENTICATION_FAILED` error. It cannot send a fatal extension error merely
because local configuration requires that extension (RFC 7296 Section 2.21.2).

The configured responder is the address-selection policy. Ze advertises no
additional addresses, and ignores the peer's optional alternatives. Runtime
movement leaves configuration untouched. A configuration edit still follows the
restart path above.

<!-- source: internal/component/ike/engine/mobike.go -- mobikeState, startMobikeRequest, handleMobikeResponse, validateMobikeRequest -->
<!-- source: internal/component/ike/engine/inbound.go -- handleOwnedInbound, handleInformationalOwned -->
<!-- source: internal/component/ike/engine/sa.go -- inheritSendPath -->

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

`PeerResult.Indication` is reported the same way, at Debug, and it is REPORTED
rather than judged: on TLS 1.3 an EAP-TLS exchange that carried no protected
success result indication arrives as an `Err` instead, so the log line names the
octets an accepted exchange carried.

<!-- source: internal/component/ike/engine/fsm.go -- handleEAPResponse -->
<!-- source: internal/core/eap/peer.go -- PeerResult, peerDiscard, notificationResponse -->

<!-- source: internal/component/ike/engine/remote_id.go -- remote identity matching -->
<!-- source: internal/component/ike/engine/notify_error.go -- error notification emission -->
