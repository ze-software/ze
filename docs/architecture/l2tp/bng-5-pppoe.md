# PPPoE access concentrator

A PPPoE access concentrator, as an alternative to L2TP for a direct-attach
broadband network gateway. Discovery wire format, AC-Cookie, session tables,
kernel sockets, the discovery reader, and the CLI surface.

The shared `ze-ppp-lcp` grouping declares the server's LCP Restart timer and
Max-Terminate, Max-Configure and Max-Failure limits. `ExtractParameters` reads
those schema defaults into an immutable `ppp.LCPConfig`. A server generation
copies that snapshot into every locally terminated `StartSession`; a reload
changes newly admitted sessions and preserves sessions already handed to PPP.
The same grouping serves the L2TP server. A Service-Name relay bypasses local
PPP and therefore does not negotiate LCP with these settings.

<!-- source: internal/component/l2tp/pppoe/config.go -- ExtractParameters -->
<!-- source: internal/component/l2tp/pppoe/subsystem_reload.go -- applyInterfaceConfig -->
<!-- source: internal/component/l2tp/ppp/yang/ze-ppp-lcp.yang -- shared settings -->

<!-- source: internal/component/l2tp/pppoe/discovery.go -- ParseDiscovery, Builder, MatchServiceName -->
<!-- source: internal/component/l2tp/pppoe/cookie.go -- GenerateCookie, VerifyCookie, CookieKey -->
<!-- source: internal/component/l2tp/pppoe/session.go -- SessionTable, AllocSID, Add, Remove, Lookup -->
<!-- source: internal/component/l2tp/pppoe/kernel_linux.go -- AF_PACKET and AF_PPPOX sockets -->
<!-- source: internal/component/l2tp/pppoe/subsystem.go -- Subsystem, discoveryReader, eventConsumer -->
<!-- source: internal/component/l2tp/pppoe/server.go -- InterfaceServer, handlePADI, handlePADR, handlePADT -->

## RFC obligations carried by this code

RFC 2516 defines the PPPoE discovery stage. The five packet types are PADI,
PADO, PADR, PADS and PADT, and the tag set is the standard one. The reference
summary is `rfc/short/rfc2516.md`.

`BuildPADI` limits the PPPoE header and discovery payload to 1484 octets,
not counting the Ethernet header. The complete frame can therefore reach
1498 octets. A larger request returns nil rather than emitting a truncated
tag. The remaining 16 octets under the Ethernet payload limit hold a
Relay-Session-Id tag with a 12-octet value.
<!-- source: internal/component/l2tp/pppoe/discovery.go -- BuildPADI, PADIMaxLen -->

RFC 2516 Section 5.5 forbids further PPP traffic after either direction of
PADT. A received PADT must match the session ID and both MAC addresses.
`handlePADT` closes the captured transport before canceling its original PPP
attempt. `handleSessionDown` closes that transport before
sending PADT, because the session-down notification can precede the PPP
goroutine's final cleanup. The first wire-end handler retains its original
admission through a completion receipt. A concurrent received PADT or genuine
PPP terminal callback cannot release that SID while the outbound PADT writer
is still running. The original handler completes the receipt and then checks
the current PPP terminal or relay-release observation before cleanup, so a
terminal event during the write is neither lost nor allowed to free the SID
early. Duplicate callbacks return without waiting for the writer.
The driver retains ownership of its transferred resources.
<!-- source: internal/component/l2tp/pppoe/server.go -- handlePADT, handleSessionDown -->

The AC sets `StartSession.PPPoE` so its shared LCP negotiator rejects ACCM,
ACFC and FCS Alternatives under Section 7. FCS Alternatives is option Type 9.
PFC is NOT RECOMMENDED rather than prohibited and keeps its existing
behaviour. L2TP leaves this transport policy unset and continues to
negotiate ACCM and ACFC.
<!-- source: internal/component/l2tp/ppp/lcp_options.go -- LCPNegPolicy, negotiatePeerOption -->

## Decisions

**A session data plane is selected by registration and exact name.** Each
provider calls `RegisterSessionDataplane` from its own registration package.
`LookupSessionDataplane` returns a sealed `Backend` for that name or refuses it;
it never tries another provider. `SessionDataplaneNames` derives a sorted copy
of the installed names for configuration diagnostics. The registry imports no
backend, and adding a provider needs no central list of backend names. An open
receives the access name, device, namespace, session ID, peer MAC and proxy-LCP
capture choice unchanged in `Request`.
<!-- source: internal/component/l2tp/pppoe/dataplane/registry.go -- RegisterSessionDataplane, LookupSessionDataplane, SessionDataplaneNames, Request -->

**Access preparation separates the session device from its Linux control
device.** A provider's `Prepare` resolves both identities before discovery can
admit a subscriber. Its immutable `Access` carries the Linux discovery index,
control MAC, MTU and namespace alongside the session-side device name. The
control MAC supplies discovery and cookie processing. The VPP provider requires
it to equal the data-plane device's MAC: VPP advertises the latter and preserves
the destination MAC on ingress, while Linux's PPPoE receiver discards frames
classified for another host. Preparation refuses a mismatch before installing
the map or starting discovery; it never changes a device MAC. `NewAccess`
consumes the preparation owner on every outcome. `Backend.Prepare` closes
partial or canceled results; if their cleanup fails, its error retains the
original cleanup owner. The caller retains each usable lease until its sessions
stop and discovery detaches, and retains failed preparation errors until their
original cleanup is settled.
<!-- source: internal/component/l2tp/pppoe/dataplane/access.go -- Access, AccessInfo, NewAccess -->
<!-- source: internal/component/l2tp/pppoe/dataplane/registry.go -- sessionBackend.Prepare -->

Each access owns session admission through its required `AccessOwner.Open`.
The registry selects preparation, and the returned access opens sessions through
that original owner. The caller MUST stop admission and join its sessions before
`Access.Close`. Close returns its cached first result, and a completed Close
refuses later opens. Concurrent opens follow the provider's contract.
<!-- source: internal/component/l2tp/pppoe/dataplane/access.go -- AccessOwner, Access, preparedAccess.Open, preparedAccess.Close -->

Kernel preparation opens one independent `netlink` backend through the iface
registry and captures its original `UnitCreator` before discovery or transport
allocation. Every child keeps that same backend for interface operations and
ordered unit creation. Shared setup returns the original typed channel and unit;
`unit.SessionConfig(channel)` prepares their non-consuming transfer and the real
PPP constructor consumes both fresh owners on every outcome. Until that call,
the factory retains and closes every partial owner, including failed or panicked
configuration. Counters come from the created unit's original generation-bound
reader; no interface-name callback or active-backend lookup replaces it. Closing
the access closes this backend after its dependent sessions and reads stop.
<!-- source: internal/component/l2tp/pppoe/dataplane/kernel/prepare.go -- Prepare, prepareWithCreator, kernelAccessOwner.Open, kernelAccessOwner.Close -->
<!-- source: internal/component/l2tp/pppoe/dataplane/kernel/factory.go -- openWithOps, preparedFactoryOps -->

A provider can register an optional lifecycle subscription alongside its
preparation factory. `Backend.WatchLifecycle` reports explicit absence
for a stable provider. A changing provider's watch fences admission on loss,
coalesces wakeups and preserves the loss sequence across readiness hints.
Only actual preparation for an unchanged observation can admit a new permit;
readiness notifications carry no native cleanup authority. The caller must
close the watch before joining its worker.
<!-- source: internal/component/l2tp/pppoe/dataplane/lifecycle.go -- LifecycleWatch, sessionBackend.WatchLifecycle -->

The first terminal observation keeps its original identity, loss sequence and
cause. Normal Close or loss exhaustion uses live CAS to freeze its exact loss
and cause, then binds that receipt before exposure; an already bound terminal
receipt wins. Every snapshot, admitted-generation lookup and permit check uses
the same terminal owner. A callback that exhausts its sixty-four live CAS
attempts binds one contention refusal or observes an existing terminal winner;
it cannot replace an earlier closed observation. A late live Admit CAS cannot
reopen admission after that terminal receipt wins. The additional terminal work
uses constant atomic operations, with no native I/O or subsystem lock.
<!-- source: internal/component/l2tp/pppoe/dataplane/lifecycle.go -- LifecycleWatch, sessionBackend.WatchLifecycle -->

`NewRecoverableAccess` binds settlement to the original preparation owner.
Its `RecoverableAccessOwner` supplies both live admission and cleanup.
Failed preparation receipts retain the cleanup-only `AccessRecovery` contract.
<!-- source: internal/component/l2tp/pppoe/dataplane/access_recovery.go -- AccessRecovery, RecoverableAccessOwner, NewRecoverableAccess -->

`SettleAccess` requires that exact access's first `Close` to have returned;
it never replays or rewrites the cached close result. Failed constructors and
provider refusals can instead return an error-owned cleanup receipt through
`RefuseAccess`. Transparent wrapping preserves it, and `SettleAccessFailure`
validates the complete bounded error graph before settling every original
receipt. Neither an embedded access, a replacement owner nor missing
bookkeeping supplies settlement authority. Native work observes its bounded
context; mandatory original physical-owner joins still complete before success.
<!-- source: internal/component/l2tp/pppoe/dataplane/access_recovery.go -- AccessRecovery, RecoverableAccessOwner, NewRecoverableAccess, SettleAccess -->
<!-- source: internal/component/l2tp/pppoe/dataplane/access_refusal.go -- RefuseAccess, SettleAccessFailure -->

**VPP preparation owns a reciprocal control map before discovery starts.**
The VPP provider reads the selected backend's actual LCP pair and checks its
Linux device in that pair's namespace. Discovery uses the Linux control
device's index and MAC; session creation retains the VPP device's index, MAC
and API connection generation. The native Linux readback runs in a dedicated
locked setup worker because the netlink library does not report a failed
namespace restoration; discarding that worker's thread prevents unrelated
work from inheriting its namespace.

Both map indices are queried through the patched targeted binding API before
an add. A conflicting pair is refused. An exact pair already present in VPP
is borrowed and never deleted. Ze-created pairs are reference counted, and
the last lease verifies the exact reciprocal pair before deleting it. A
confirmed add is rolled back when later publication fails, including a stream
cleanup error after the successful reply. Failed deletion or rollback retains
that confirmed provenance, so a later lease still removes Ze's own map. A lost
delete reply requires successful targeted reads of both indices before absence
can release that ownership; the original error remains visible to its caller.
A mutation without a successful add reply reserves both indices without claiming
deletion authority. A later matching readback cannot turn that unknown result
into a borrowed or owned map. Those reservations block overlapping admissions
across API reconnections and count toward the 4096-entry bound
alongside active leases and failed cleanup. Reaching the bound refuses another
access rather than forgetting unresolved native state.

The provider retains an immutable preparation for each active logical access.
The access lease opens sessions through that exact preparation and its captured API epoch.
It does not resolve a new device or switch to a replacement VPP connection.
Access close waits for an in-flight constructor before removing the map.
The caller must still stop every session and detach discovery before ending
its preparation lease.
<!-- source: internal/component/l2tp/pppoe/dataplane/vpp/prepare.go -- Prepare, accessPreparationLease.Open, accessPreparation.borrow, accessPreparations -->
<!-- source: internal/component/l2tp/pppoe/dataplane/vpp/access_map.go -- controlMapOwners.acquire, controlMapLease.Close -->
<!-- source: internal/component/l2tp/pppoe/dataplane/vpp/prepare_linux.go -- readControlLink -->

The VPP composition wrapper registers `Prepare` under the
exact `vpp` name. Its manifest row is nested under the L2TP feature, so the
generated import requires both `ze_l2tp` and `ze_vpp`. The common PPPoE
registry imports neither implementation.
<!-- source: internal/component/l2tp/plugins/pppoe-vpp/register.go -- init -->

The common original-tree gate checks the installed provider registry against
the candidate's interface backend and its schema default; it never probes the
host or constructs a session. The VPP wrapper adds its own prerequisite
validator, requiring VPP and LCP to be enabled. The same pure predicate selects
its registered startup feature: `pppoe_plugin.so` and the
`enable-pass-nd-and-dhcpv6` flag under `pppoe`. Read-root declarations reach both
in-process verification and SDK configuration delivery through the VPP startup
registry and `registry.AddConfigRead`. No feature-specific branch in VPP
selects PPPoE.
<!-- source: internal/component/l2tp/pppoe/backend_gate.go -- validateBackendGate, ConfiguredSessionBackendName -->
<!-- source: internal/component/l2tp/plugins/pppoe-vpp/startup.go -- registerStartup, startupRequired -->
<!-- source: internal/component/vpp/startup_feature.go -- RegisterStartupFeature, ParseConfigSections, startupConfigRoots -->

The wrapper also declares every generated PPPoE API name and CRC as a runtime
requirement. `doctor-vpp-pppoe-plugin` queries the configured VPP API socket.
It reports a missing or incompatible patched API as an error, and an
unavailable probe as a warning. API registration comes from the loaded plugin,
so an installed `.so` file alone cannot satisfy this check.
<!-- source: internal/component/l2tp/plugins/pppoe-vpp/startup.go -- registerStartup -->
<!-- source: internal/component/vpp/startup_doctor.go -- checkStartupDoctor, probeStartupRuntime -->

**A factory result has two explicit owners.** `NewOpenedSession` is the only
constructor for the sealed `OpenedSession` result. It validates the borrowed
frame streams, data-plane handle and transport owner, including typed nils.
The PPP driver borrows the frame channel and owns `Dataplane`; the access
concentrator ends the transport with `CloseTransport`. That operation invokes
the transport close once and returns the same result on every call. Construction
consumes both owners on failure as well as success. If a factory returns a
session with an error, `Access.Open` closes both owners before returning the
failure, preserving cleanup errors. A backend that uses one owner for both
roles must meet the handle's idempotent close contract.
<!-- source: internal/component/l2tp/pppoe/dataplane/session.go -- NewOpenedSession, OpenedSession, openedSession.CloseTransport -->
<!-- source: internal/component/l2tp/pppoe/dataplane/access.go -- preparedAccess.Open -->

A missing provider or refused factory also raises a historical
`doctor-ppp-session-admission` event. Runtime doctor shows the access interface,
full access index, control device and session ID; `ze show errors` retains the
original cause, including a joined rollback error. An invalid discovery request
does not invent an admission failure. The discovery owner still rolls back the
failed session before returning.

<!-- source: internal/component/l2tp/pppoe/doctor_admission.go -- InterfaceServer.reportSessionAdmissionFailure -->
<!-- source: internal/component/l2tp/pppoe/server.go -- InterfaceServer.handlePADR -->

**A PPP admission refusal distinguishes an existing key from a new attempt.**
`EventSessionRejected.ExistingSession` preserves the already admitted session
and its transport. A refusal of a new key releases its pending admission,
publishes subscriber teardown and sends PADT through the normal teardown path.

**A family release changes the registry entry, not the session.** When one
NCP leaves Opened while LCP stays up, PPP withdraws that family from pppN and
emits `ppp.EventSessionIPReleased`. `onSessionIPReleased` clears that family's
fields from the registry entry (`subevents.ClearReleasedFamily`) and publishes
`(subscriber, session-ip-released)`, which the shared route observer reads to
withdraw that family's routes. No PADT is sent. The next
`EventSessionIPAssigned` of the family reinstalls it.

**Every `ppp.Event` type has a case.** `EventEchoRTT`, which only the L2TP
reactor relays for CQM, has an explicit no-op case, because one arrives per
LCP Echo-Reply. Any other type the switch does not name reaches the default
arm, `reportUnhandledPPPEvent`: a Warn with `CodeDoctorInternalDefect`, its
fix and the Go type, beside the matching runtime doctor finding. Only a Ze
defect produces one: the sealed event set gained a type this switch lacks.
<!-- source: internal/component/l2tp/pppoe/subsystem.go -- handlePPPEvent, onSessionDown, onSessionIPReleased, reportUnhandledPPPEvent -->

**Kernel relay borrows its descriptor from the transport owner.** A transport
may implement the optional `RelayTransport` capability. The opened session's
`RelayDescriptor` checks that capability while excluding transport close, and
refuses unsupported, closed or invalid owners. The descriptor remains borrowed
until `CloseTransport`; the relay owner must serialize its use against that
close. Neither the session wrapper nor the frame channel caches a descriptor.
<!-- source: internal/component/l2tp/pppoe/dataplane/session.go -- RelayTransport, openedSession.RelayDescriptor, openedSession.CloseTransport -->

**The admitted identity retains the complete access index.** PADR computes
the canonical subscriber ID once from the server's complete interface index
and PPPoE session ID. The session table and PPP start request retain that same
opaque ID and the same validated `SessionAttempt`. PPP's 16-bit tunnel and
session fields are diagnostic labels; neither selects an access interface or
an admitted worker.

Cancellation follows that retained attempt before and after admission.
`Attempt.Cancel` cancels a queued attempt or its once-bound worker, while
`Attempt.Control` returns only that original worker. PADT gates the captured
transport and signals that attempt; it never closes a transferred PPP lease.
A queued or admitted transfer keeps its table row, original Opened borrower and
reserved SID until the genuine Down/refusal consumer completes that admission.
A refusal consults its exact attempt cleanup status; a missing worker Control
is never evidence of lease release.
A delayed request, event or realm callback must match the exact attempt and
admission before changing transport state.
<!-- source: internal/component/l2tp/pppoe/server.go -- handlePADR, handlePADT -->
<!-- source: internal/component/l2tp/pppoe/admission_control.go -- sessionForAttempt, sessionForControl, stopPPPAdmission -->

**Access preparation precedes discovery.** The selected provider's `Prepare`
resolves both the data-plane device and the Linux control device and owns any
control mapping as an access lease. Its immutable snapshot carries the control
device's namespace, Linux index, source MAC and MTU separately from the
data-plane device. Startup prepares every configured access before opening
discovery; a failed preparation closes every unpublished lease. A server
captures this access in its session factory and calls its `Open` method.
<!-- source: internal/component/l2tp/pppoe/dataplane/access.go -- NewAccess, Access -->
<!-- source: internal/component/l2tp/pppoe/subsystem_access.go -- prepareAccessConfiguration, closeAccessConfiguration -->
<!-- source: internal/component/l2tp/pppoe/subsystem_reload.go -- newInterfaceServer -->

Session creation uses the same control device and namespace as discovery.
The access lease stays live
until its sessions stop and discovery no longer dispatches to it.
<!-- source: internal/component/l2tp/pppoe/dataplane/access.go -- NewAccess, Access -->
<!-- source: internal/component/l2tp/pppoe/subsystem_access.go -- prepareAccessConfiguration, closeAccessConfiguration -->
<!-- source: internal/component/l2tp/pppoe/subsystem_reload.go -- newInterfaceServer -->

**One AF_PACKET raw socket serves the selected control namespace.** Every
prepared access must name that namespace and a distinct Linux index within the
shared PPP driver's 16-bit tunnel-key range; discovery dispatches by the index
from the receive call. A named-namespace socket is
opened by a temporary worker on a locked OS thread. It restores the original
namespace and closes the namespace handles before publishing the socket. If
restoration fails, the worker closes the socket and exits while still locked,
so Go discards that thread. The caller's thread never enters the control
namespace. The existing PPPoE client socket API retains its caller-namespace
behavior.
<!-- source: internal/component/l2tp/pppoe/discovery_namespace.go -- openDiscoveryNamespace, openDiscoveryNamespaceThread -->
<!-- source: internal/component/l2tp/pppoe/discovery_namespace_linux.go -- openDiscoverySocketInNamespace -->

**A reload applies the diff live, and refuses only what cannot apply.** The
hub registers the subsystem whatever the boot config says, so a commit that
enables PPPoE has a `Reload` to reach. `Reload` (`subsystem_reload.go`) parses
the `pppoe` root with the same `ExtractParameters` boot uses. A kept interface
gets a new `InterfaceServer` carrying the new settings over the same session
table and cookie key, swapped in while the discovery reader is between frames
(`dispatchMu`), so live sessions and outstanding cookies survive. A removed
interface leaves the map first, then each of its sessions gets a PADT and its
PPP session is stopped, and its access lease is released. Added accesses are
prepared and their servers constructed before the live map changes. A failed
preparation releases only these additions and preserves the running servers,
configuration and leases. A kept access retains its lease and namespace; a
live change to the selected backend or discovery namespace is refused. A
cleanup failure is reported after the removed access stops serving sessions.
Disabling stops discovery and leaves the subsystem started and idle, so a
later reload can start it again.

**Session state is per interface.** Each interface holds its own session table
with an independent session id space over the range 1 to 65534: 0 is the
discovery value and 0xffff is the value RFC 2516 Section 4 reserves and forbids.
There is no global lock to contend on.

**One subscriber MAC address can hold several sessions.** RFC 2516 places no
per-peer session limit, so the session table's MAC index is a set of sessions
per MAC, not a single pointer: adding a second session for a MAC joins the set
rather than replacing the first, and `sessionsByMAC` returns every session the
MAC currently holds. `max-sessions-per-mac` bounds how many sessions one MAC
may hold on an interface, global default 8, with a per-interface override the
same shape as `max-sessions`.

<!-- source: internal/component/l2tp/pppoe/session.go -- SessionTable.byMAC, sessionsByMAC -->

**PADR admission is bounded per MAC, and the bound is enforced by counting,
not by rate.** `admitPerMACCap` (`server.go`) refuses a PADR once its source
MAC already holds `max-sessions-per-mac` sessions, replying with a PADS
carrying an AC-System-Error tag and SESSION_ID `0x0000` -- the same tag and
SID `AllocSID` exhaustion already answers with, since both are the AC
declining to spend a resource rather than an objection to the requested
Service-Name. RFC 2516 places no per-peer session limit (Section 9 only names
the concept: an AC using the AC-Cookie "can then limit concurrent sessions for
[a PADI SOURCE_ADDR]"), so the cap is a Ze policy decision. The count includes
a session mid-teardown (`State == StateTeardown`): its kernel resources are
not released until `Remove` runs, so excluding it would let a MAC hold one
more session's worth of descriptors than the cap bounds for the length of
that window.

**The existing `PADILimiter` is not reused for PADR, because its per-MAC arm
is a one-per-second dedup rather than a count.** A subscriber's PADR normally
follows its own PADI inside the same second; running it through the PADI
limiter would refuse that legitimate PADR outright, breaking ordinary session
setup. A rate also does not bound the resource this cap protects: a MAC under
the rate can still grow its session count without limit. The two mechanisms
stay separate: `PADILimiter` bounds how often a MAC may ask, `max-sessions-
per-mac` bounds how much a MAC may hold at once.

**The PADR dedup branch answers a retransmission in every session state, and
the correlator is the AC-Cookie, not the MAC and state alone.** RFC 2516
places no per-peer session limit, so a MAC under the cap can be admitting a
second, genuinely distinct session (its own fresh PADI/PADO cookie) at the
same instant a replay of its first session's PADR arrives; matching on MAC
and state alone would answer that second PADR with the first session's
SESSION_ID instead of admitting it. `SessionTable.matchLiveCookie` compares
an incoming PADR's cookie against each of the MAC's sessions' stored
`Cookie`, runs entirely under the session table's lock, and excludes a
session already marked `StateTeardown`: `handleSessionDown` (the PPP driver's
event-consumer goroutine) marks that state through `SessionTable.markTeardown`
before doing anything else, so a session another goroutine is already tearing
down is never handed back to a PADR the discovery-reader goroutine is
processing at the same moment.

**Every access to `Session.State` goes through the session table's lock, and
that is what the exclusion above rests on.** Two goroutines write the field:
`handleSessionDown` marks `StateTeardown` through `markTeardown`, and
`handlePADR` marks `StateSession` through `markSession` once kernel setup has
succeeded. A bare field write on either side would leave the field
unsynchronized however carefully the other side locked, because a SID freed by
`Remove` is reallocated at once, so a session-down event still queued for the
previous holder of that SID can reach the next one while `handlePADR` is still
setting it up. `markSession` also moves only a session still in
`StateDiscovery`: a session the event consumer has already torn down stays torn
down, because the PADR that admitted a session is never what decides the
session is alive again.

<!-- source: internal/component/l2tp/pppoe/server.go -- admitPerMACCap, the PADR dedup branch in handlePADR -->
<!-- source: internal/component/l2tp/pppoe/session.go -- Session.Cookie, markTeardown, markSession, matchLiveCookie -->

**The AC-Cookie is HMAC-SHA256 with a timestamp.** It is hardware accelerated
and simpler than the MD5 and DES construction accel-ppp uses. The timestamp
bounds replay.

**PADS is sent AFTER the kernel setup succeeds.** Sending it first and then
failing the kernel setup leaves the subscriber waiting for an LCP exchange that
never starts.

**`Remove()` returns the socket descriptor atomically.** The discovery reader
and the event consumer would otherwise both close it.

**PADI is rate limited per client MAC address.**

<!-- source: internal/component/l2tp/pppoe/ratelimit.go -- PADILimiter, Check -->

**A PPPoE session reads its RADIUS rate profile from the shared session
metadata store, not from a PPPoE-specific path.** The RADIUS auth handler stores
the Access-Accept attributes under the `(tunnel id, session id)` pair the PPP
driver carried, and for PPPoE that pair is `(ifindex, PPPoE session id)`, which
`onSessionUp` already holds. So the subscriber's Filter-Id rates reach
`subscriber.Session` with no new plumbing and no second attribute path. The two
rate fields had no producer at all until 2026-09-06: every PPPoE session handed
the shaper a zero pair and was shaped at the configured default whatever the
RADIUS server answered.

<!-- source: internal/component/l2tp/pppoe/subsystem.go -- onSessionUp, the LoadSessionMetadata read -->

**Ze refuses a PADR with no Service-Name tag and serves a PADI with none.**
accel-ppp does exactly this: `pppoe_recv_PADR` discards a tagless PADR, and
`pppoe_recv_PADI` starts its match true when nothing is configured. FreeBSD's
`ng_pppoe` tolerates both packets instead, substituting an empty tag on the
PADI and keying the PADR admission on the cookie alone. RFC 2516 Sections 5.1
and 5.3 bind the sending host, not the AC, so this choice is about robustness
against a malformed peer, not about conformance.

<!-- source: internal/component/l2tp/pppoe/server.go -- requireServiceNameTag, handlePADR -->

**Ze always emits exactly one Service-Name tag in the PADO and the PADS,
including the zero-length case.** RFC 2516 Section 5.2 requires a Service-Name
tag identical to the PADI's in every PADO, and Section 5.4 requires exactly
one in every PADS. `AddTagCopy` skips a nil tag, which is correct for the
genuinely optional Host-Uniq and Relay-Session-Id tags, so `BuildPADO` and
`BuildPADS` write the Service-Name tag through `AddTagString` instead: an
absent or zero-length source tag becomes a zero-length echo, never no tag at
all. Both reference implementations are less conformant here. FreeBSD's
`ng_pppoe` PADO can carry zero tags or two, and its PADS carries none when the
PADR carried none. accel-ppp's PADO carries none when nothing is configured
and the PADI carried none; its PADS already matches Ze.

<!-- source: internal/component/l2tp/pppoe/discovery.go -- BuildPADO, BuildPADS -->

**Every discovery refusal increments `ze_pppoe_discovery_refusals_total`,
labelled by `reason`.** A PADI refused for an unoffered service name gives no
reply on the wire, because RFC 2516 Section 5.2 requires that, so the counter
is the only visibility that refusal has. The reason set is closed:
`rate-limited`, `service-name-mismatch`, `service-name-missing`,
`cookie-invalid`, `session-id-exhausted`, `per-mac-cap-reached`. Where a refusal already logs at
Debug, the log line carries the same reason string, so the log and the
counter are one vocabulary, not two. The rate-limiter refusal counts with no
paired log line: a PADI flood would otherwise turn Debug logging itself into
the resource problem the limiter exists to prevent. One counter set serves
every interface server, so a refusal on any access interface accumulates on
one series per reason.

**`Subsystem.Start` registers the counters with the plugin registry rather
than reading one out of it.** On a PPPoE-only daemon the metrics registry does
not exist yet at that moment: `runYANGConfig` runs `engine.Start`, which runs
`Subsystem.Start`, and only afterwards runs `startStandaloneTelemetry`, which
creates the registry. A daemon that also carries a `bgp` block gets its
registry earlier, from the reactor's config loader during plugin start, so
which event happens first is a property of the operator's configuration.
Reading `registry.GetMetricsRegistry` at `Start` therefore binds nothing on
the PPPoE-only deployment and leaves `ze_pppoe_discovery_refusals_total`
absent for the process lifetime, with no log line saying so.
`registry.InjectPluginMetrics` holds the hook until a registry arrives, so
whichever event happens second does the binding.

**A discovery socket read error increments `ze_pppoe_discovery_read_errors_total`,
a separate counter from the refusal series.** A refusal is a protocol or
configuration decision about a well-formed read; this counter is the socket
itself failing to read at all, so it is not a fourth value on the closed
`reason` label above. `discoveryReader` paces the retry through
`internal/core/pacer` before it reads again, growing the delay from zero up
to a 250ms ceiling across a run of consecutive failures and resetting on
the next success, so a discovery socket that never recovers costs a
bounded slice of a core rather than all of it. Because this one goroutine
reads the single AF_PACKET socket that dispatches by ifindex for every
access interface ("One AF_PACKET raw socket per namespace" above), a
delay here delays discovery on all of them, which is why the ceiling
stays at 250ms rather than growing further: an operator has no
information with which to pick a longer value correctly. The pacer's
wait observes the stop channel. The discovery socket has a 100ms receive
timeout, matching the existing client cancellation mechanism. An idle timeout
checks cancellation without incrementing the error counter or invoking the
error pacer.

`Stop` signals and joins discovery before stopping PPP. It does not hold the
server-map mutex while joining either worker. The discovery descriptor stays
open until PPP events finish, so local session teardown can still send PADT.
A separate lifecycle mutex serializes `Start` and `Stop`. Closing a raw Linux
descriptor alone does not interrupt a `recvfrom` already in progress.

**A panic while one frame is parsed or handled ends only what that frame was
admitting.** `discoveryReader` hands each frame to `dispatchDiscovery`, the
`pppoe/discovery` task of the shared panic guard, which logs, counts and
crash-records the panic and returns to the reader. `handlePADR` records the
session it is admitting in `InterfaceServer.admitting`, so
`endAdmissionAfterPanic` can end it: before the PADS the host holds no session,
and the entry is removed with nothing sent; once the PADS is marked sent, the
host is sent a PADT (RFC 2516 Section 5.5), since PPP never started on that
session. A panic on a frame that admitted nothing drops that frame. The auth
and pool handler calls are the `auth-handler` and `pool-handler` tasks and
refuse the one request. The task table is in `docs/guide/monitoring.md`,
"Contained Panics".

<!-- source: internal/component/l2tp/pppoe/panic_guard.go -- dispatchDiscovery, endAdmissionAfterPanic, admission -->
<!-- source: internal/component/l2tp/pppoe/metrics.go -- registerDiscoveryMetrics, bindPPPoEMetrics, countRefusal, countDiscoveryReadError -->
<!-- source: internal/component/l2tp/pppoe/server.go -- the eight call sites that count a refusal -->

## Traps this code exists to avoid

**`Lookup` returns a live pointer and the caller mutates it.** `handlePADR`
mutates the session state, the unit number and the socket descriptor after
`Add`, without re-acquiring the lock. The snapshot method was safe because it
copies under the lock; the single-session lookup was not. `LookupSnapshot` is
the safe pair.

**Snapshot for the CLI, raw pointer for the hot path.** That is the rule
whenever a table hands out a pointer that is mutated after insertion.

<!-- source: internal/component/l2tp/pppoe/snapshot.go -- Sessions, LookupSnapshot, Snapshot -->

**A struct that crosses the huge-parameter threshold changes every call site.**
Adding four fields to the PPP session start struct pushed it to 304 bytes, and
every pass-by-value site had to become a pointer. The channel type stays a value
because it owns the transfer.

**`SIOCGIFHWADDR` has no Go accessor.** The `unix.Ifreq` type carries no
hardware-address method. Reading it needs pointer arithmetic into the raw
request union at known offsets.

## Patterns worth reusing

- **The YANG triple.** Config schema, API RPCs and the CLI tree are three
  separate YANG modules. The config and API modules are embedded in the
  component's schema package; the command module lives under the CLI handler's
  yang directory. Blank imports in the CLI handler wire all three.
- **Shared PPP integration.** Each transport supplies its session fields,
  including the explicit PPPoE LCP policy. Discovery and transport lifetime
  remain outside the PPP driver. Before native allocation, the transport
  captures a `UnitCreator` from its original retained backend.
  `kernel.DevPPPSetup` borrows the same original PPPoX `*os.File`. It queries
  that socket's capture/connect/bridge ownership ABI, then captures its exact
  native channel into the same freshly opened unbound `/dev/ppp` file. It
  returns that `CreatedChannel` together with its actual `CreatedUnit`. Named and unnamed
  units use the creator's ordered rtnetlink `RTM_NEWLINK` of kind `ppp`, carrying
  the original `IFLA_PPP_DEV_FD` and retaining the actual creation echo and
  generation-bound counter reader. The LNS and access concentrator omit a name;
  the kernel chooses `pppN`. The client supplies its configured key, so its
  device keeps that name (`docs/architecture/l2tp/cpe-1-pppoe-client.md`).
  Neither name nor numeric descriptor metadata establishes ownership. The
  channel retains the original socket namespace from `SIOCGSKNS`; a joined
  startup worker creates the unit there and restores its own caller namespace
  before returning. A failed restore retires that locked OS thread and refuses
  the partial pair. Both file leases end before any backend or network wait.
  Unsupported and Unknown capture results refuse without numeric fallback. The
  original pair supplies `unit.SessionConfig(channel)` for transfer to
  `NewSession`; CPE native operations borrow the same owners through short
  descriptor operations. A failed setup returns its partial original owners,
  closes the unit before the channel and retains every cleanup failure. Both
  naming modes retry a definite native collision at most five times, after
  confirmed original-file cleanup; uncertain outcomes never retry.

<!-- source: internal/component/l2tp/ppp/kernel/devppp_linux.go -- DevPPPSetup -->
<!-- source: internal/component/l2tp/ppp/kernel/devppp_other.go -- non-Linux stub -->
<!-- source: internal/component/l2tp/ppp/kernel/created_channel_linux.go -- CreatePPPChannel -->
<!-- source: internal/component/l2tp/ppp/kernel/devppp_namespace_linux.go -- createPPPSetupUnit, runOriginalPPPNamespace, runOriginalPPPNamespaceThread -->
<!-- source: internal/component/l2tp/ppp/kernel/created_unit_linux.go -- NewUnitCreator, createOriginalPPPUnit -->

The shared `kernel.SetIPv4Mode` operation changes a unit's IPv4 mode to PASS or DROP.
The caller serializes the operation against descriptor closure. The PPPoE client
uses DROP during LCP restart, then PASS after IPCP reaches Opened.
<!-- source: internal/component/l2tp/ppp/kernel/devppp_linux.go -- SetIPv4Mode -->
<!-- source: internal/component/l2tp/pppoeclient/connected_session.go -- restart -->

The shared setup came out of an exact duplicate: L2TP and PPPoE carried
character-for-character identical ioctl sequences that differed only in the
error message prefix. Extracting it removed 148 lines with no behavior change.
The two steps before unit creation (`PPPIOCGCHAN` on the PPPoX socket, then
`PPPIOCATTCHAN` on a newly opened `/dev/ppp` file) are still common to every
kernel-backed transport. Only the client creates its unit by name.

## VPP session ownership

The VPP provider opens one AF_PPPOX datagram socket on the prepared control
device, in that device's namespace. It creates no Linux PPP channel or unit.
Its session add uses the patched `disable_fib` field, reads back the created
interface's name and index, assigns the access interface's actual IPv4 and
IPv6 table IDs, confirms both unicast and multicast receive gates closed
for each family, and sets the initial payload MTU before returning to the
server's PADS path. Session readback also requires the exact ingress device,
SID, client MAC, data-plane source MAC and decapsulation table, so a changed
MAC-only link-table entry cannot publish a session on the wrong access.
A confirmed add remains owned if closing the API stream
fails; construction closes its socket and quarantines forwarding before
removing that exact SID/MAC. If quarantine cannot be confirmed, it retains
the native session and exact owner instead of recycling the interface index.
An add submitted without a confirmed reply reports an unknown outcome and does not
assume ownership of a possibly foreign session. The exact uncertain mutation
and its MAC reservation remain retained, so another admission cannot reuse
the key before reconciliation. Failed forwarding cleanup similarly retains
its owner and remaining route set. Reconciliation retries confirmed cleanup
on its captured connection after checking the complete native session identity.
A completed dump can release an absent uncertain creation; a visible session
does not grant deletion authority. Retained session and control-map failures
appear in `show warnings` and the PPPoE VPP reconciliation doctor check.
<!-- source: internal/component/l2tp/pppoe/dataplane/vpp/factory.go -- openWithOps -->
<!-- source: internal/component/l2tp/pppoe/dataplane/vpp/api.go -- binaryAPI.create, binaryAPI.interfaceName -->

Cancellation or stream-open refusal before `SendMsg` is a different outcome:
the request was never submitted and creates no native ownership obligation.
The exchange preserves that fact as a typed cause, retaining the underlying
error. Session, route, control-map and policer receipt readers consume it.
Once a send has been attempted, a missing reply remains uncertain, including
when the eventual error is cancellation. Rate preparation also checks
cancellation under its ownership mutex before reserving the pending slot.
<!-- source: internal/component/l2tp/pppoe/dataplane/vpp/api_submission.go -- apiRequestNotSentError, requestWasNotSent -->
<!-- source: internal/component/l2tp/pppoe/dataplane/vpp/rate_transaction.go -- policerRates.Prepare -->

PPP borrows the socket through one control reader throughout the session.
`AddAddressP2P` stages the peer path without publishing native egress.
The common PPP owner acknowledges the assignment and its accounting baseline
before calling `Connect`. That call publishes staged peer and framed routes
only for families with an address pair, then opens a family only after all
its staged paths are confirmed. A framed route for an already published
family can take effect immediately. IPv6 can install its negotiated link-local
pair later; another explicit `Connect` publishes that pair independently.
A withdrawing pair leaves the grantable address set before any mutation.
Failed cleanup retains its exact peer path and closing-family cleanup for
retry, so another family's assignment cannot reopen it. Withdrawing the last
pair closes its receive gates and removes every native path of that family.
Generic framed intents wait for a new acknowledged pair; the retired peer
path is discarded. The independent family's paths remain installed.
`Disconnect` closes both families. Every gate operation reads the exact feature state, changes
it only when needed, and reads it again to confirm the result. This avoids
stacking VPP's `ip4-not-enabled` or `ip6-not-enabled` features and avoids
the separate IP-enable APIs' reference counts.
The socket reader preserves datagram length and refuses truncation. Its
`os.File` owns runtime polling and descriptor closure, so close wakes blocked
I/O and deregisters polling before releasing the descriptor. The data-plane
and transport roles share the same idempotent session owner. Its close gates
the control endpoint, confirms all four receive gates closed, and removes
retained routes. It deletes the VPP session only after that cleanup succeeds;
later operations refuse the closed owner.
<!-- source: internal/component/l2tp/pppoe/dataplane/vpp/session.go -- session.Connect, session.Disconnect, session.Close -->
<!-- source: internal/component/l2tp/pppoe/dataplane/vpp/socket_linux.go -- socketFile.Read, socketFile.Close -->

Peer addresses become host routes to the created interface; framed routes
use that same index and the table selected for their address family. These
paths use the `ze-pppoe` FIB source through `ip_route_add_del_v2`, with exact
multipath additions and withdrawals preserving other subscribers' paths
under the same prefix and source. A confirmed add remains owned if stream
closure fails; a missing reply retains the uncertain exact path until its
withdrawal is confirmed. Each session retains at most 1024 paths for
bounded teardown. Counters come from the
created native index in the VPP stats segment. A failed read, a missing row
or a duplicate row is an error and cannot become a zero that idle detection
would read as inactivity. Every session reads the statistics owner's shared
snapshot. `Counters` asks for a dump that starts after the call, which the
accounting baseline and the final read after `Disconnect` need. An interval
reader calls `CountersWithin(interval)`, so all sessions read within one
interval share one VPP dump. Neither accepts a dump taken before the
session's interface was created.
<!-- source: internal/component/l2tp/pppoe/dataplane/vpp/api_routes.go -- binaryAPI.routeSource, binaryAPI.route -->
<!-- source: internal/component/l2tp/pppoe/dataplane/vpp/session_routes.go -- session.AddAddressP2P, session.programRoute -->
<!-- source: internal/component/l2tp/pppoe/dataplane/vpp/session.go -- session.Counters, session.CountersWithin -->

The session policer owner holds at most two native objects: RX for upload and
TX for download. Plans carry explicit burst sizes and reject rates that VPP's
whole-kilobit API cannot represent. A change prepares both plans without
writing, then installs or updates both directions before it can commit. Rate
replacement uses `PolicerUpdate` on the exact name-verified indices, so it
does not unbind traffic or allocate replacement objects. A partial failure
restores both previous plans; failed compensation remains pending. Each owner
retains only one pending change and one terminal result. Ending the session
invalidates both, so a delayed authorization reply cannot restore a departed
subscriber's rate on a reused interface.

Before freeing either policer, cleanup confirms all possibly active features
off: the four patched input arcs, native `device-input`, and the two output
arcs. It then removes each confirmed directional binding and deletes its
name-verified index. Index zero is a valid policer index. A lost deletion reply
requires a completed indexed read proving absence. An unknown creation retains
its exact name without guessing an index from dump order. An unknown first
binding is retained with the session: the pinned VPP unbind function writes a
slot vector that the missing request might never have allocated, so repeating
an unbind would be unsafe. These unknown owners remain quarantined until
ownership-aware readback resolves the operation or process retirement proves
the native state gone. An API reconnection alone cannot release that ownership
or authorize native interface recycling.
The owner decisions of 2026-11-23 ("Design decisions" below) replace it in two
ways. Inside the reconnect-and-verify limit that follows a failed alive test,
a reconnect to the same process, proven by comparing retained pidfs
identities and answering with the data Ze configured, keeps the sessions; a
retained owner then reads back and cleans its exact native identity over the
new connection in the background reconciliation. When that limit expires,
Ze kills the process through its retained pidfd and waits for the exit proof,
which retires these owners, so no owner waits on a VPP that never exits. That is not built
yet; `plan/post-release/features/spec-pppoe-vpp-dataplane.md` owns it.
<!-- source: internal/component/l2tp/pppoe/dataplane/vpp/rate_owner.go -- policerRates, policerRates.Close, policerRates.cleanup -->
<!-- source: internal/component/l2tp/pppoe/dataplane/vpp/rate_transaction.go -- policerRates.Prepare, policerRateTransaction.Apply, policerRateTransaction.Rollback -->
<!-- source: internal/component/l2tp/pppoe/dataplane/vpp/rate_features.go -- binaryAPI.quarantinePolicers -->

The VPP IPv6 carrier borrows that same PPP socket. Its sole reader intercepts
direct IPv6 Router Solicitations and DHCPv6 client datagrams punted by patch
0014; it never passes them into a Linux IP interface. IPv6CP's installed
link-local address pair supplies the local source used for RA and DHCPv6
responses. Before queueing a
control, the reader validates the IPv6 envelope, transport checksum, source,
destination and protocol fields. Solicitation admission also uses the shared
Neighbor Discovery body and Hop Limit validator, accepting unspecified and
unicast sources independently of the negotiated interface identifier.
DHCPv6 admission follows RFC 9915: client sources may use another link-local
or global unicast address and any permitted UDP source port; the destination
must be `ff02::1:2` port 547. Replies use the actual request source address
and destination port 546 through the same session-bound endpoint. This
provider has no legacy Server Unicast leases and discards unicast requests.
Eight fixed slots per
service bound pending controls; a full queue drops the newest control and
increments `ze_pppoe_vpp_ipv6_discards_total` with its reason. Without an open
carrier, IPv6 network payloads are silently discarded while PPP control
protocols continue through the reader.

RA and DHCPv6 writes construct IPv6 packets on the borrowed socket using the
negotiated local link-local source and calculated pseudoheader checksums.
RA uses Hop Limit 255. Cancelling reads preserves this write path: the common
PPP service joins its workers, sends its final RA, then closes the carrier.
Carrier close unregisters and wakes both queues without closing the socket;
the session remains the sole endpoint owner.
<!-- source: internal/component/l2tp/pppoe/dataplane/vpp/ipv6_session.go -- session.OpenIPv6Carrier -->
<!-- source: internal/component/l2tp/pppoe/dataplane/vpp/ipv6_wire.go -- controlSocket.dispatchIPv6, controlSocket.dispatchDHCPv6 -->
<!-- source: internal/component/l2tp/pppoe/dataplane/vpp/ipv6_carrier.go -- ipv6Carrier.WriteRA, ipv6Carrier.WriteDHCPv6, ipv6Carrier.Close -->

PADS progress has one table-owned monotonic receipt. Its zero value is
unspecified; Add explicitly initializes a fresh row as PADSNotAttempted. `PADSAttempted` is recorded
before sending and means the packet may have escaped, including a sender panic
or uncertain result. `PADSSubmitted` records only a successful real kernel send;
it does not claim peer receipt. StateTeardown never erases this history.
A void test send observer proves only an attempt. Shutdown and future terminal
logic use this receipt snapshot instead of deriving publication from State.
<!-- source: internal/component/l2tp/pppoe/session.go -- PADSPublication, SessionTable.PADSPublication -->
<!-- source: internal/component/l2tp/pppoe/server.go -- sendPADS, submitFrame -->

## Design decisions

| Decision | It forbids | Source | Date |
|----------|------------|--------|------|
| D-1, a VPP API loss with the process alive, is resolved by the VPP lifecycle rules: a live VPP that Ze cannot adopt is killed and restarted, so its exit proof always arrives and retires the retained owners | Holding PPPoE service fenced until an external VPP happens to exit, or releasing retained owners on an API reconnection alone | plan/post-release/features/spec-pppoe-vpp-dataplane.md, plan/post-release/features/spec-vpp-failure-lifecycle.md | 2026-11-23 |
| When Ze adopts the VPP, retained owners are read back by exact identity over the new connection of the same pidfd-proven process, and cleaned there (option C) | Using a new connection whose peer is not proven the same process, or releasing an owner without a confirmed deletion or a completed absence read | plan/post-release/features/spec-pppoe-vpp-dataplane.md | 2026-11-23 |
| Adoption happens only inside the single VPP loss timer: a reconnect to the same pidfd-proven process whose full readback matches (option C) keeps the sessions. A declared loss is never adopted: Ze kills the process through the retained pidfd, waits for its exit proof, restarts and reconfigures VPP as at boot, and PPPoE tears its sessions down per OD-2 and lets subscribers redial. The provider holds the process lease, which gives the identity proof, the kill and the exit proof. This narrows the row "When Ze adopts the VPP" to the single timer, and the D-1 row's "cannot adopt" to every declared loss | Option C readback after the timer expired, keeping PPPoE sessions across a forced restart, or a provider that holds no process lease | plan/post-release/features/spec-pppoe-vpp-dataplane.md, plan/post-release/features/spec-vpp-failure-lifecycle.md | 2026-11-23 |
| D-1 (a VPP API loss with the process alive) resolves on the VPP lifecycle's two recovery forms. Form (1), inside the reconnect-and-verify limit that follows the failed alive test: the same pidfd-proven process answering with the data Ze configured is enough to adopt it, and the PPPoE sessions are kept. The retained owners are read back by exact identity over the new connection of that process and cleaned there (option C), as part of the background reconciliation; a mismatch it finds is repaired, never treated as a loss. Form (2), a forced restart: Ze kills the process through the retained pidfd, waits for its exit proof, restarts and reconfigures VPP as at boot, and PPPoE tears its sessions down per OD-2 and lets subscribers redial. The provider holds the process lease, which gives the identity proof, the kill and the exit proof. This supersedes the row "Adoption happens only inside the single VPP loss timer" (the single timer, adoption on a full readback match, and "a declared loss is never adopted"), and narrows the D-1 row and the row "When Ze adopts the VPP" to these two forms | Holding form (1) or the PPPoE sessions until a full readback completes, tearing sessions down over a mismatch the background reconciliation repairs, holding PPPoE service fenced until VPP happens to exit, using a new connection whose peer is not proven the same process, releasing an owner without a confirmed deletion or a completed absence read, keeping PPPoE sessions across a forced restart, or a provider that holds no process lease | plan/post-release/features/spec-pppoe-vpp-dataplane.md, plan/post-release/features/spec-vpp-failure-lifecycle.md | 2026-11-23 |
