# Path MTU Diagnostic

`show mtu` measures the path MTU the router's IPsec tunnels ride on, derives the
ESP ceiling of each tunnel from the transform it negotiated, and reports whether
each tunnel interface is oversized, tight, under-utilized or correct, with the
configuration commands that fix it. The run changes nothing on the router: no
path from the handler reaches a configuration write, and the remediation
commands are strings for the operator to apply.

This page is the owner of the `mtu` component (`internal/component/mtu/cmd`) and
of the `mtu-cmd` YANG plugin (`internal/plugins/mtu-cmd/yang`). It states the
command grammar, the payload shape, the reference-address leaf, the run, the
search, the arithmetic, the verdicts and the local state.

The probes are the in-daemon ICMP sockets of `internal/core/probe`, in the
Don't Fragment mode `docs/architecture/diagnostics/active-probes.md` describes.

## The module is removable

The feature is one component plus one YANG plugin, each reached by a blank
import in the generated composition root `internal/component/plugin/all/all.go`.
Dropping the two imports removes `show mtu`, its grammar and its configuration
leaf, and nothing else changes. The component imports neither the IKE engine nor
the interface backend: IPsec state reaches it through a registered inventory
snapshot, and interface state through the `iface` dispatch calls.

<!-- source: internal/component/mtu/cmd/register.go -- init, the RPC registration and the answer shape -->
<!-- source: internal/component/mtu/cmd/doc.go -- the blank imports of the two YANG modules -->

## The grammar

```
show mtu [host <address>] [exhaustive] [detail]
```

| Word | Form | Meaning |
|------|------|---------|
| `host` | keyword before one IPv4 or IPv6 address | measure that address alone, and skip the peers and the reference address |
| `exhaustive` | bare word | a slower run whose figure comes from probing every size on the wire: the kernel's cached path MTU and the router's reported value are both discarded |
| `detail` | bare word | add, per target, the size of every probe sent and the answer each one received |

The grammar is `internal/plugins/mtu-cmd/yang/ze-mtu-cmd.yang`, which
container-merges its `show mtu` node onto the show verb root. `host` is a
`zt:ip-address` leaf, so the dispatcher refuses a value outside the two address
patterns before the handler runs, and `parseMTUArgs` parses it a second time
into a `netip.Addr`. The two bare words are the enumeration values of the
`search` and `view` leaves, which is how the loaded model offers them.
A token that is none of the three is refused by name, and so is a dash-leading
value, which is an option and never data.

<!-- source: internal/plugins/mtu-cmd/yang/ze-mtu-cmd.yang -- the show mtu node -->
<!-- source: internal/component/mtu/cmd/mtu.go -- parseMTUArgs, mtuRequest -->

## The run

`runMTU` (`run.go`) owns the control flow, in this order:

1. **Targets.** A `host` run has one target and never consults the inventory.
   A default run asks `ipsecinventory.Tunnels()`: `ErrNotRegistered` is the
   `inventory: not-registered` outcome with a caution note, distinct from a
   registered inventory holding no tunnel (`verdict: no-tunnels`). Each tunnel
   contributes one target: its installed remote while the Child SA is up
   (AC-17), else its configured remote when that is an address. A hostname or
   `any` on a down tunnel is not probed; the tunnel is still listed. One
   address reached through two tunnels is measured once.
2. **Underlay.** The interface the route to the first target leaves by
   (`iface.RouteLookup`, then `iface.GetInterface` for its MTU), or the route
   to the reference address when there is no target. The row names its
   `source` as `route to <address>`. A route or an interface that cannot be
   read is a fault note and no `underlay` row.
3. **Measurements.** Each target in order, one prober each
   (`openWireProber` in `DFHonorCache`, or `DFBypassCache` under `exhaustive`),
   the kernel's cached PMTU (`probe.KernelPathMTU`) read BEFORE the probe.
   A prober that cannot be opened, or a search that errors, leaves that target
   with `outcome: error` and a fault note while the others are still measured
   (AC-7). A target that answers the 10000-octet gate ends the run as
   `df-gate-failed`: the later targets are not probed.
   A measured peer target whose tunnel is up is then offered to the IKE
   prober (`measureByIKE`), which confirms the ICMP figure first and descends
   below it only when the figure is too big: "The IKE prober" below. The IKE
   figure replaces the ICMP one only when IKE refuted it; a fit at the ask
   confirms the ICMP figure and the row's `ike-confirmed` names the size
   proven. It
   reaches the engine through the `internal/core/ikeprobe` leaf (`ikeProber`,
   `search.go`) and never builds an IKE message. A prober that declines, by a
   refusal the engine names, by an SA the exchange failed, by the exchange
   budget, or by no engine being registered, leaves the ICMP figure and
   `prober: icmp` in place, and the row's `ike-declined` and a caution note
   say why the figure is unconfirmed.
4. **Reference.** On a default run only, the `reference-address` leaf is
   measured unless it is already a target, in which case that measurement is
   the reference. A silent reference makes the underlay advice undecidable
   (AC-14).
5. **Tunnels.** Per tunnel, in the verdict order: down when no Child SA is
   installed or the bound interface is down; not sized when the SA is
   policy-based (`if_id` 0), when no xfrm interface carries its `if_id`, when
   the child holds no traffic selector (the inner family comes from `TSRemote`,
   then `TSLocal`), when `deriveESPOverhead` refuses the transform (a fault
   note, never a default), or when no peer path was measured. Otherwise the
   path MTU is the tunnel's own measurement, or the tightest measured peer
   path marked `assumed` when its own target did not answer; then `ceiling`,
   `recommended` (absent when none), `mss`, `classifyTunnel`, and a
   `tunnelCommand` for every verdict that earns one.
6. **Underlay advice.** `adviseUnderlay` runs once a peer path was measured,
   writes `underlay.advice`, a note with the matrix's sentence, and the
   `underlayCommand` when the matrix produced one.
7. **Local state.** The fragmentation counters' findings and `tcp_mtu_probing`.
8. **Status.** `df-gate-failed` beats everything; `ok` when any target or the
   reference answered; `nothing-measured` otherwise.

The run answers ONE payload when every target has been measured. The RPC
contract is one Response per command; the streaming path the ping component
uses (`NewPingSession`, reached through the `monitor` verb's session
factories) is a different verb, so per-peer progress would be `monitor mtu`,
which this spec does not add. The probe budget per target is bounded
(`runProbeBudget`), so the worst case is computable.

Every read outside the package goes through `mtuDeps` (`liveDeps` in
production): the inventory, the prober, the cached PMTU, the route lookup, the
interface, the xfrm interface list, the counters and the sysctls. A test
replaces the struct and drives the whole run through `handleShowMTU`.

<!-- source: internal/component/mtu/cmd/run.go -- runMTU, mtuDeps, liveDeps -->

## The payload

The answer is ONE document, declared `ShapeDoc`, so `| json`, `| yaml` and
`| table` render the same payload and every row operator is refused by name
before the command runs. Every key is kebab-case. A verdict is a field and
never a marker glued to another field's text.

| Key | Holds | Present |
|-----|-------|---------|
| `status` | one of `ok`, `nothing-measured`, `df-gate-failed` | always |
| `inventory` | `registered` or `not-registered` | on a default run |
| `verdict` | the run-level ladder, first match wins: `action-needed`, `check`, `no-tunnels`, `ok` | on a default run with a registered inventory |
| `measurements` | one row per target: `target`, `label` (`host`, `peer`), `outcome` (`measured`, `unmeasurable`, `df-gate-failed`, `error`), `probes` (the ICMP probes sent, on every prober), `lossy`, `prober` (`icmp`, or `ike` when the peer's live SA confirmed or measured the figure), `ike-confirmed` under `prober: ike` (the largest wire size the padded exchange proved the path carries whole, the size the engine sent; equal to `path-mtu` when IKE refuted the ICMP figure, below it when a cipher grid sent a smaller datagram than the ask), `ike-declined` when the IKE prober was asked and produced no figure (the refusal by name, `sa-failed at <size> octets`, the budget, or `not in this build`), `cached-path-mtu` when the kernel held one, `path-mtu`, `method` (how the ICMP search found its figure) and `caveats` (the row's own, by its prober: the ICMP caveat, the different-path caveat, or none) under `measured`, `exchanges` (the IKE exchanges spent, 0 when it declined before sending) under `measured` for a target with a live SA, `reason` under `error`, and `probes-sent` (`payload`, `wire`, `outcome`, `reported-mtu`) under `detail` | always |
| `reference` | `host`, then `path-mtu`, `method` and `prober` when measured, else `outcome` | on a default run |
| `underlay` | `interface`, `mtu`, `source`, `kind` when the link type has an interface list in the iface YANG (`iface.CanonicalInterfaceType`), and `advice` once the matrix ran | once the underlay was read |
| `tunnels` | one row per tunnel: `peer`, `remote`, `interface`, `mode`, `encapsulation`, `transform`, `path-mtu`, `assumed`, `current-mtu`, `ceiling`, `recommended`, `mss`, `verdict`, `octets` (the excess, spare or gain the verdict names), `sized`, `reason` when not sized | on a default run |
| `commands` | the remediation commands, one string each, in Ze's own config syntax | always |
| `notes` | rows of `severity` (`info`, `caution`, `fault`) and `text` | always |
| `caveats` | the run's summary of what its figures cannot see: the ICMP caveat while any figure in the payload, a measurement or the reference, is ICMP-measured; empty when every figure is IKE-measured. Each measurement row carries its own | always |
| `tcp-mtu-probing` | `disabled`, `on-blackhole`, `always` | once the sysctl was read |

A key listed as present under a condition is ABSENT otherwise, never zero: an
absent key is a value nobody computed, and a zero would read as an answer. The
Go side of each closed set is a typed enum whose zero value is `Unspecified`,
written to the payload through `String()` and never compared.

<!-- source: internal/component/mtu/cmd/mtu.go -- runStatus, runVerdict, noteSeverity -->
<!-- source: internal/component/mtu/cmd/run.go -- inventoryState, measurementRow, tunnelRow, sizeTunnel -->

## The reference address

`show mtu` measures the reference address beside the peers. A reference outside
the tunnels is what makes the underlay advice decidable: a clamped access
circuit is worth matching, a clamped peer path is not. The address is the
`environment { mtu { reference-address } }` leaf of
`internal/component/mtu/yang/ze-mtu-conf.yang`, one IPv4 or IPv6 address with
the default `1.1.1.1`.

The leaf reaches the handler through the env layer. The config package plumbs
`environment/mtu/reference-address` into the key `ze.mtu.reference-address`
(`envPlumbingTable`), the OS environment wins over the config value, and the
registration's default answers when neither names a value. `referenceAddress`
refuses a value that is not an address by name rather than reading it as no
reference, because no reference is a different outcome: the underlay advice
becomes undecidable.

<!-- source: internal/component/mtu/cmd/mtu.go -- referenceAddress, referenceAddressEntry -->
<!-- source: internal/component/config/apply_env.go -- envPlumbingTable, the mtu row -->
<!-- source: internal/component/mtu/yang/ze-mtu-conf.yang -- the reference-address leaf -->

## The search

`searchPathMTU` (`internal/component/mtu/cmd/search.go`) measures the path
MTU to ONE target. It is a pure algorithm over a `prober`, one operation that
sends a DF echo of a payload size and answers one of: replied, refused with
a reported next-hop MTU, refused with none reported (the three outcomes of
`probe.QueuedError`, each tagged `local` when this host's kernel refused the
send), or silent. The unit tests drive it over a model of a path; the wire
prober at the end of the file binds it to the probe layer. Sizes in the
result are wire octets; the search works in payload octets and converts
with the family's ICMP overhead, 28 on IPv4 and 48 on IPv6.

The search runs in this order, and every step is bounded by a named
constant.

| Step | What it does | Bound |
|------|--------------|-------|
| The DF sanity gate | a 10000-octet payload MUST be refused before any figure is believed. A reply means DF is not honored and the run reports `df-gate-failed`. The gate's own refusal is the first reported figure, from a router (`via ICMP`) or from this host's kernel at send (`via local iface MTU`) | one attempt |
| The follow | a reported wire MTU is CONFIRMED before it is the answer: the size passes and one octet more fails (AC-8). A confirm that fails and hands back a new figure is followed | `reportedFollowsMax` = 6 |
| The ladder | the 31 candidate sizes inside the bracket, bisected by index, largest first | `ladderAttemptsMax` = 5 attempts |
| The floor | when nothing has passed, a 68-octet payload; no answer means `unmeasurable` (AC-7) | one attempt |
| The re-test | a size that passed sitting at or above one that failed is a contradiction only a lossy or lying path produces; the passed size is re-tested and the result is marked lossy | one attempt |
| The number line | the bracket bisected numerically | `bisectionAttemptsMax` = 16 attempts |

Two RFC rules govern every attempt. RFC 8899 Section 5.1.2 sets
`MAX_PROBES` to 3, so one size is sent up to `probesPerSizeMax` = 3 times
before a silence is believed (the Boundary row: 1..3 probes per size), and
an explicit refusal is definitive on the first answer because it is not a
dropped echo. RFC 4821 Section 7.6.4 says a probe lost near other losses
SHOULD NOT update the bounds, so a single silence moves nothing and the
same size is sent again (AC-10). `TestSearchDoesNotMoveBoundsOnSingleSilence`
pins both: two silences at 1400 still answer 1400, a third answers 1399,
and on a filtered path a bound moved on the first silence would contradict
the retry and mark the run lossy. A refusal that reports no value is the
unmodified router of RFC 1191 Section 5: it is a definitive failure that
starts the search, never an MTU and never discarded.

The whole run is bounded by `runProbeBudget`, the sum of every step at its
worst: 3 × (1 + 2 × 6 + 5 + 1 + 1 + 16) = 108 probes, 216 seconds at the
2-second `probeWait` with no answer at all (R-2). `TestSearchUnmeasurable`
counts 21 probes for a dead path, and
`TestSearchBudgetIsTheComputedWorstCase` re-derives the two bisection
bounds from the ladder length and the payload range.

`exhaustive` (AC-11) discards the reported figure and the bracket and searches
the full range; the caller opens the prober in `probe.DFBypassCache` for
it, so the kernel's cached path MTU is never consulted.

The result names how the answer was found, as a typed `searchMethod` with
the seven meanings of the ported tool: `via ICMP`, `via local iface MTU`,
`ICMP under-reported`, `ICMP over-reported`, `ICMP filtered`, `ICMP kept
changing`, `forced full search`. One attribution departs from the tool: a
figure this host's kernel reported at send is not an ICMP report, so a
search that follows it and hears nothing from the wire is `ICMP filtered`,
where the tool printed `ICMP over-reported` for a router that never spoke.
An unmeasurable path and a failed gate are outcomes by name, never a zero.

### The wire prober

`wireProber` opens one DF socket through `probe.OpenICMP` in the mode the
run chose and sends one probe at a time: `probe.BuildICMPEcho` with the
socket's identifier and a sequence per probe, a deadline of `probeWait` set
before the send (it binds the send too, so one left over from a silent
probe cannot fail the next), and a reply matched on identifier, sequence and
source through `probe.ParseEchoReply`. A read that fails is the error-queue
wake: the prober drains the queue and takes the entry whose quoted echo is
this probe (`QueuedError.SizeRefusalOf`), leaving an entry about another
flow alone. A send the kernel refuses with `EMSGSIZE` is answered from the
local entry the kernel queued, or from `probe.KernelPathMTU` when the drain
finds none. Both matchers are the ones the ping session uses.

`search_integration_linux_test.go` (`integration && linux`, run by
`./le qemu all-tests`) proves the prober against the clamped
three-namespace path of `active-probes.md`: the router's 1400 confirmed
`via ICMP`; with the router's ICMP errors dropped on the sender (strict
reverse-path filtering plus a blackhole route for the router's address,
the simplest filter that needs no firewall binary) the ladder and the
number line find 1400 as `ICMP filtered`; and after honor-cache poisoned
the cache with 1400 and the clamp was lifted, `exhaustive` answers 1500 while
`probe.KernelPathMTU` still says 1400.

### The IKE prober

A peer target whose tunnel has a live IKE SA is measured a second time, over
the SA itself: `ikeProber` (`search.go`) is the `prober` whose one operation is
a padded INFORMATIONAL exchange asked of the engine through the
`internal/core/ikeprobe` leaf, so the reply comes back over the port, the
encapsulation and the path the tunnel's control channel rides. The search
speaks in ICMP payload octets and the engine in datagram octets, so the adapter
adds the family's ICMP overhead back: the size on the wire is what a path MTU
is a property of. `measureByIKE` (`run.go`) drives it, in this order.

| Step | What it does | Bound |
|------|--------------|-------|
| The ceiling | an ICMP figure above `ikeWireMax` = 3000, the largest IKE message (RFC 7296 Section 2), is not offered at all and the row says so; the engine refuses a size above its interface MTU by name, so between the two no probe exceeds min(interface MTU, 3000) (AC-6) | none sent |
| Confirm-first | the ICMP figure itself is asked. A fit CONFIRMS it: the path carried it whole for an authenticated request and its reply, the ICMP figure stands as `path-mtu`, the row says `prober: ike`, and `ike-confirmed` carries the size sent. No exchange is ever asked above the ICMP figure (A-6): that figure is what the path carried whole for ICMP, so a larger size is exactly the DF-clear copy a fragment-dropping path loses | `probesPerSizeMax` = 3 exchanges |
| The size sent | the engine sends the largest datagram the SA's cipher suite produces at or below the size asked, never above (a CBC suite sends on a 16-octet grid, an AEAD suite reaches every size), and names it in the answer. `ike-confirmed` is the largest size that FIT as sent (`ikeProber.fitOctets`): asked 1400 on a CBC SA, the exchange leaves at 1392, and a fit keeps `path-mtu` 1400 (the exchange tested nothing above 1392, so it refuted nothing) with `ike-confirmed` 1392 and a caution note saying IKE confirmed the path down to 1392 | none: the rounding costs no exchange |
| A rekey in flight | a probe the peer's IKE rekey retires is answered `rekeyed` with the size it sent (`retirePendingProbe`, `engine/probe.go`): the exchange counts, and the size is asked again on the new SA, up to `probesPerSizeMax` like a silence (AC-10, R-5) | one exchange per rekey |
| The descent | a figure that is too big is REFUTED: `pathSearch.refine`, the same ladder-then-bisect search the ICMP path runs, descends from a bracket whose top is the ICMP figure, and the size it finds replaces the ICMP figure as `path-mtu` (equal to `ike-confirmed`), with an info note naming both figures | `ikeExchangesPerRunMax` = 16 exchanges per run, confirm included (the Boundary row: 1..16) |

Each exchange holds the SA's request window (RFC 7296 Section 2.3), so no DPD,
Delete or rekey leaves while one is out: exchanges are what a run spends on a
live tunnel, and the budget of 16 is what bounds how long DPD is held off. The
engine keeps its own budget, the ordinary retransmit schedule, which bounds how
long ONE exchange stays outstanding. The two are not one number because they
bound different things.

An IKE `too-big` is the DF copy drawing no answer and the DF-clear copy being
answered. Unlike an ICMP refusal, it can be a lost DF copy (a drop, or a
strongSwan peer in IKE_REKEYED dropping the request), so `attempt` retries it
the way it retries a silence: RFC 4821 Section 7.6.4 and RFC 8899 Section 5.1.3
apply to both probers, the same size is asked again, and only the third
`too-big` is believed (AC-3; `TestIKEProbeRetriesTooBigBeforeBelievingIt`).

A budget spent after a size fitted is a coarser figure, not a missing one: the
largest size the peer answered whole is a floor the path is proven to carry,
which is what a tunnel is sized to. The row reports it as the IKE figure and a
caution note carries the bracket the search stopped inside. A budget spent
before any size fitted leaves the ICMP figure, unconfirmed, with the budget
named in `ike-declined`.

**Stop at the first silence.** An `sa-failed` outcome ends the IKE attempt for
that tunnel at once (`ikeProbeStopAtFirstSilence`, true, read by `ikeProber`
and never by the ICMP path): no smaller size is asked, the row keeps the ICMP
figure with `prober: icmp` and `ike-declined: sa-failed at <size> octets`, and
a caution note says the figure is unconfirmed (AC-13). The reason is stated
plainly: on a path that drops IP fragments, a probe can take the tunnel down
the way any IKE request larger than the path would. The DF copy is too big,
the DF-clear copies are fragmented and lost, and after the full retransmit
budget the SA is deemed failed (RFC 7296 Section 2.1) and re-established by the
owner loop. The engine gains no probe-aware exit, because a request forgotten
while its SA runs is the third exit Section 2.1 does not offer. The mitigation
is this module's: confirm-first means a size the path carries whole is answered
on the DF copy and never reaches a DF-clear copy, and stop-at-first-silence
means one run fails a tunnel at most once. Owner ruling (a), 2026-09-16.

**The caveat rules.** Each measured row carries its own `caveats`. An ICMP
figure carries the ICMP optimism caveat. An IKE figure drops it: the reply is
authenticated and rode the SA's own channel. An IKE figure over an SA without
UDP encapsulation (the inventory's `UDPEncap`) carries the different-path
caveat instead: the exchange rode UDP/500 while ESP rides protocol 50, and a
middlebox can treat the two differently. An IKE figure over a NAT-T SA carries
none: the exchange left from 4500 with the four-octet non-ESP marker, on the
path ESP-in-UDP rides (AC-7). The run's own `caveats` holds the ICMP caveat
while any figure in the payload is ICMP-measured, the reference included.

`probes` keeps its meaning on every row: the ICMP probes sent. The IKE
exchanges are the row's `exchanges`, present for a target with a live SA once
the ICMP search measured, and 0 when the IKE prober declined before sending
(a refusal by name builds nothing, and so does an empty leaf).

`TestIKEProbeConfirmsThenDescends`, `TestIKEProbeSAFailedEndsTheRun`,
`TestIKEProbeNeverExceedsTheCeiling`, `TestIKEProbeUnregisteredIsNotSilence`,
`TestMeasurementRowNamesTheProber` and `TestIKEExchangeBudgetBoundary`
(`run_test.go`) drive the module over a scripted `ikeprobe.Prober` in the leaf;
the engine's exchange is proven on its own page.

<!-- source: internal/component/mtu/cmd/search.go -- searchPathMTU, pathSearch, wireProber, runProbeBudget -->
<!-- source: internal/component/mtu/cmd/search.go -- ikeProber, ikeExchangesPerRunMax, ikeProbeStopAtFirstSilence, ikeWireMax -->
<!-- source: internal/component/mtu/cmd/run.go -- measureByIKE, declineIKE, icmpCaveat, ikePathCaveat -->
<!-- source: internal/component/mtu/cmd/search_test.go -- fakePath, the search tests -->
<!-- source: internal/component/mtu/cmd/search_integration_linux_test.go -- the clamped path, dropRouterICMPErrors -->

## The ESP overhead

Every octet ESP adds to a packet is derived from the Child SA as the inventory
reports it INSTALLED: the negotiated transform, the mode, the encapsulation
and the endpoint family. Nothing is assumed from the configuration, which is
what lets Ze compute what the ported tool had to hardcode.

| Part | Octets | Comes from |
|------|--------|-----------|
| outer IP header | 20 for an IPv4 endpoint, 40 for IPv6 | the family of the installed remote address |
| UDP header | 8 when the SA is UDP-encapsulated, else 0 | `Tunnel.UDPEncap` |
| ESP header | 8 (SPI and Sequence Number) | RFC 4303 Section 2 |
| IV | 8 for AES-GCM (RFC 4106 Section 3.1), 16 for AES-CBC (RFC 3602 Section 3) | `espCipherWires`, keyed by the ENCR transform id |
| ICV | 16 for AES-GCM, or the integrity transform's truncated MAC: 16, 24 or 32 for HMAC-SHA-256, -384, -512 | `crypto.AEADICVOctets` for an AEAD cipher, the crypto integrity registry's `TruncatedLength` otherwise |
| alignment block | 4 for AES-GCM (the ESP rule of RFC 4303 Section 2.4), 16 for AES-CBC (the cipher block) | `espCipherWires` |
| trailer | 2 (Pad Length and Next Header) | RFC 4303 Section 2 |

`espCipherWires` is the one declaration of the ESP IV and block per transform
id. The ICV is never written there: the crypto package already declares it,
per AEAD transform and per integrity transform, so the length a packet really
carries has one source. The population is the set an ESP proposal can carry
into a Child SA, `ipsec.SupportedESPEncryptionNames`: the AES CCM ids IKE can
negotiate are absent because no dataplane installs one, and an inventory row
naming one is refused rather than sized.

AES-GCM-128 over IPv4 without encapsulation is 20 + 8 + 8 + 16 = 52 octets,
and 60 encapsulated. A transform the table does not hold, a child that is not
installed, or a mode nobody set answers `errOverheadRefused`: the tunnel is
reported as not advisable and nothing is computed from a default.

In transport mode the packet keeps its own IP header and ESP is inserted
behind it (RFC 4303 Section 3.1.1), so the outer header is the packet's own
rather than an addition. The overhead carries the mode, and the ceiling
arithmetic branches on it, so tunnel-mode arithmetic is never applied to a
transport-mode SA.

<!-- source: internal/component/mtu/cmd/overhead.go -- espCipherWires, deriveESPOverhead, espICVOctets -->

## The arithmetic

The figures are the ported tool's, with the overhead derived rather than
assumed.

| Figure | Formula | Absent when |
|--------|---------|-------------|
| ceiling | tunnel mode: `alignDown(underlay - outer - fixed, block) - 2`; transport mode: the same over the part after the packet's own header, plus that header | never: 0 when nothing is left |
| recommended | `alignDown(ceiling + 2 - 32, block) - 2` | the ceiling is 0, or the value lands below the inner family's minimum: 1280 for inner IPv6 (RFC 8200 Section 5), 576 for inner IPv4 (a documented default from RFC 791's reassembly minimum, not a conformance claim) |
| MSS | `mtu - inner IP header - 20` | the headers leave nothing |

The margin of 32 is `recommendedMarginOctets`, with the ported tool's reason
beside it. An absent value is `errNoUsableMTU`, never 0: a 0 would read as a
recommendation and reach a `set ... mtu 0` command. The inner family is an
input the caller supplies (`ipFamily`), because the inventory's `Tunnel`
carries no traffic selector.

<!-- source: internal/component/mtu/cmd/arith.go -- ceiling, recommended, mss, ipv6MinimumLinkMTU, recommendedMarginOctets -->

## The verdicts

A tunnel is classified against its interface's current MTU, in this order,
first match wins. The octets column is the figure the payload names beside
the verdict.

| Verdict | Condition | Octets | Command |
|---------|-----------|--------|---------|
| `down` | the interface is down | none | none |
| `no-usable-mtu` | no recommended value exists | none | none |
| `oversized` | `current > ceiling` | the excess, `current - ceiling` | `set interface xfrm <name> mtu <recommended>` |
| `tight` | `current > ceiling - 32` | the spare, `ceiling - current` | the same |
| `under-utilized` | `current != recommended` | the gain, `recommended - current` | the same |
| `ok` | otherwise | none | none |

`no-usable-mtu` is tested before `oversized` on purpose: with no safe value
the current size is beside the point, and falling through would put the
absent value into the remediation list.

The run verdict folds the tunnel verdicts and the notes, first match wins:
any `oversized` or `no-usable-mtu` is `action-needed`; any `tight` or `down`
is `check`; a fault note found outside the tunnel table is `action-needed`,
because a PMTU blackhole in the counters must not sit under an ok; no tunnels
is `no-tunnels`; the rest, `under-utilized` included, is `ok`. The ported
tool's first rung, DO NOT APPLY for a non-standard ESP proposal, has no
counterpart: a transform the arithmetic refuses is a fault note for that one
tunnel, and every other tunnel's figures stay valid.

<!-- source: internal/component/mtu/cmd/verdict.go -- tunnelVerdict, classifyTunnel, runVerdictOf -->

## The underlay advice

The reference measurement is what makes the advice decidable. The matrix is a
pure function of the underlay interface's current MTU, the tightest path
measured to any peer, and the reference measurement when one answered. There
is no `current < tightest` row: a probe larger than the interface MTU is
refused by the local kernel and never leaves the box.

| Outcome | Condition | Severity | Command |
|---------|-----------|----------|---------|
| `unreadable` | no underlay interface, or its MTU could not be read | caution | none |
| `capped-by-interface` | `current == tightest` and `current != 1500` | caution | none |
| `at-standard` | `current == tightest == 1500` | information | none |
| `undecidable` | `current > tightest` and no reference answered | caution | none |
| `not-clamped` | `reference >= current` | information | none: lowering the interface would cost octets on every other destination |
| `circuit-clamped` | `reference == tightest` | caution | `set interface <kind> <name> mtu <reference>` |
| `two-clamps` | otherwise | caution | `set interface <kind> <name> mtu min(reference, tightest)`, to confirm with a third address |

The command is spelled with the underlay's own interface list, `kind`: a physical port is `ethernet`, the functional tests' veth is `veth`, a bridge is `bridge`. A link type the schema holds no list for (`kind` absent, or the loopback) earns the value in the note and no command, and the note says so, because `set interface ethernet br0 mtu 1400` would create an ethernet block rather than size the bridge.

<!-- source: internal/component/mtu/cmd/verdict.go -- underlayOutcome, adviseUnderlay, underlayCommand -->

## The local state

Three facts about the box are reported beside the measurements, each read
through a producer that says so when it cannot answer.

| Fact | Read through | Finding |
|------|--------------|---------|
| the fragmentation counters, as ABSOLUTE values since boot | the vendored `procfs` parse of `/proc/self/net/snmp` and `snmp6`, the same parse the telemetry collector reads as rates | `IpFragOKs` or `Ip6FragOKs` non-zero: caution; `IpFragFails` non-zero: fault, a PMTU blackhole in progress; `IpReasmFails` or `Ip6ReasmFails` non-zero: caution; all zero: one information note |
| `net.ipv4.tcp_mtu_probing` | `sysctl.Read`, the sysctl component's one exported read | 0, the Linux default: information; 1 and 2: nothing |
| the kernel's cached path MTU for a destination | `probe.KernelPathMTU`, before the run probes it | a cached value that differs from the measurement: caution, naming `net.ipv4.route.mtu_expires` when it could be read; one that agrees: information, unless the run was exhaustive |

A counter the kernel does not report is `errCounterAbsent`, a sysctl value
outside `0..2` is an error, and a cache the kernel does not hold is no note:
none of the three is a zero.

<!-- source: internal/component/mtu/cmd/state.go -- readFragmentationCounters, tcpMTUProbing, pathMTUCacheFinding -->
<!-- source: internal/component/sysctl/backend.go -- Read -->

### The local-state dependency check

`ze doctor` runs the check `mtu-local-state` before an operator's first run. It
calls the two readers above, `readFragmentationCounters` over `/proc` and
`readTCPMTUProbing` over the sysctl component, through the same functions the
run takes, so it answers for the code path `show mtu` uses. Each reader that
fails is one warning with the code `doctor-mtu-local-state`, naming the source,
its error and the finding the run then reports as unreadable. Both readers
passing is silence. The check does not stop a measurement: a run on a box that
fails it still probes and still sizes, and only the local-state findings are
missing. The ICMP socket the probes need is the probe layer's own check,
`checkICMPProbeSocket` (`internal/core/probe/doctor.go`), and is not repeated
here. The check is registered by `register.go` beside the RPC, in the
`PreConfig` phase, so it runs on a build with no configuration loaded.

<!-- source: internal/component/mtu/cmd/doctor.go -- checkMTULocalState, localStateMessage -->
<!-- source: internal/component/mtu/cmd/register.go -- diagnostic.RegisterDoctorCheck -->

## Tests

| Test | Proves |
|------|--------|
| `TestESPOverheadPerTransform` | every transform pair the registries can negotiate has a hand-computed overhead, over both families with and without UDP (A-2, AC-2, AC-3) |
| `TestESPOverheadUnknownTransformRefuses` | an unknown transform, an uninstallable one, and an uninstalled child are refused, never defaulted |
| `TestCeilingAlignsAndSubtractsTrailer` | the ceiling at hand-computed values in both modes |
| `TestRecommendedAbsentBelowMinimumLinkMTU` | 1280 and 576 are the last valid values; 1279 and 575 are absent |
| `TestVerdictThresholds` | each verdict at its exact boundary (A-5) |
| `TestVerdictOrderNoUsableMTUBeforeOversized` | a tunnel with no safe value is never merely oversized and earns no command (AC-6) |
| `TestUnderlayAdviceMatrix` | every row of the advice matrix (AC-12, AC-13, AC-14) |
| `TestFragmentationCountersAreAbsoluteWithSeverities` | the counters are the file's values, with the severities above (AC-20) |
| `TestReadAnswersTheKernelValueOrSaysWhy` | `sysctl.Read` answers through the one key-to-path mapping and errors on an absent key |
| `TestDoctorMTULocalStateReportsEachUnreadableSource`, `TestDoctorMTULocalStateSilentWhenBothRead` | the `mtu-local-state` check, run through `diagnostic.DoctorChecksForPhase`: one warning with the code `doctor-mtu-local-state` per reader that fails, and none when both read |
| `TestShowMTUReachesTheHandler` | `show mtu` reaches `handleShowMTU` through the registered RPC and answers a document with a status |
| `TestShowMTUHostMeasuresOneAddress` | `show mtu host <address>` measures that address and answers its path MTU (AC-1) |
| `TestShowMTUPayloadRendersAsJSON` | the payload goes through `\| json` unrefused with kebab-case keys |
| `TestShowMTUHostRunHasNoTunnelSection`, `TestShowMTUOversizedTunnelGetsACommand`, `TestShowMTUTightAndOKTunnels`, `TestShowMTUNoUsableMTUHasNoCommand`, `TestShowMTUUnmeasurablePeerIsAssumed`, `TestShowMTUExhaustiveBypassesTheCache`, `TestShowMTUUnderlayAdvice`, `TestShowMTUTransportModeUsesTransportArithmetic`, `TestShowMTUProbesTheInstalledEndpoint`, `TestShowMTUNoInventoryIsNotZeroTunnels`, `TestShowMTUEveryPipeRendersOnePayload`, `TestShowMTUFragmentationCountersAreAbsolute`, `TestShowMTUDownAndUnboundTunnelsAreListed`, `TestShowMTUDFGateFailureStopsTheRun`, `TestShowMTUDetailListsEveryProbe` | the run through the handler over fakes (`run_test.go`): AC-1, AC-2, AC-4 to AC-7, AC-11 to AC-14, AC-16 to AC-20, the assumed path, the not-sized shapes, the DF gate and the detail view |
| `TestResolveIfIDReadsTheBoundInterface` (`ike/engine`) | a peer's `vti bind` installs its Child SA with the bound xfrm interface's `if_id`; a binding to no interface is refused |
| `show-mtu-host.ci`, `show-mtu-exhaustive.ci`, `show-mtu-json.ci`, `show-mtu-no-ipsec-component.ci`, `show-mtu-oversized-tunnels.ci` (`test/plugin/`) | the run over real namespaces: a clamped path measured (AC-1), a poisoned route MTU bypassed under `exhaustive` (AC-11), the document shape (AC-19), a registered inventory with no tunnel (AC-18, registered half), and two bound aes128gcm tunnels sized oversized with the circuit-clamped underlay advice (AC-2, AC-4, AC-13, AC-15, AC-17) |
| `TestShowMTUHostRefusedByTheGrammar` | the host leaf's definition refuses `300.1.1.1` and accepts both families |
| `TestMTUCmdSchemaOwnsShowMTU` | the dedicated module declares the whole surface |
| `TestReportedMTUConfirmedOnTheWire`, `TestSearchLadderThenBisect`, `TestSearchDoesNotMoveBoundsOnSingleSilence`, `TestSearchUnmeasurable`, `TestSearchExhaustiveDiscardsTheReport` | the search, over a model of a path (AC-7 to AC-11) |
| `TestSearchClampedPathViaICMP`, `TestSearchFilteredPathLadderThenBisect`, `TestSearchExhaustiveBypassesPoisonedCache` | the wire prober against a Linux router (AC-1, AC-8, AC-9, AC-11) |

<!-- source: internal/component/mtu/cmd/mtu_test.go -- the wiring tests -->
