# PPPoE Docker Interop Lab

This peer-isolated Docker lab proves both Ze PPPoE roles against independent
implementations. Ze acts as a client against
[accel-ppp](https://accel-ppp.org/), then as an access concentrator against
`pppd` with the `rp-pppoe` plugin.

## Overview

Each scenario runs its containers on an isolated bridge because PPPoE uses
Ethernet discovery and session frames (EtherTypes 0x8863 and 0x8864). No
container runs privileged: Ze, accel-ppp and the pppd client each hold
`NET_ADMIN` and the host's `/dev/ppp` device, and the discovery sockets use the
`NET_RAW` Docker grants by default. The host kernel must provide `/dev/ppp` and
the `pppoe` pppox module, built in or loaded by the host: no container loads a
kernel module.

The client scenario requires PADI/PADO/PADR/PADS discovery, LCP, CHAP-MD5,
IPCP, one kernel device named after the `pppoe-client` key (`pppoe0`),
traffic to the access concentrator, the
accel-ppp session view, and clean teardown. The access-concentrator scenario
requires the same discovery and NCP path from an independent pppd trace. It
also dials a wrong credential and requires CHAP refusal before proving that all
session state disappeared.

### Two clients on one native VLAN

`pppoe-shared-access-two-clients` runs the real Ze daemon and two independent
pppd clients on VLAN 100. Both clients use the maintained pppd 2.5.5 image,
with separate credentials, container namespaces, PPP interface names, PID
files and logs. The checker requires each client's discovery exchange to
carry its own MAC and admitted session ID, both directions of LCP
acknowledgment, CHAP success, distinct IPCP addresses, matching kernel PPP
devices and routes, and traffic bound to each PPP interface in both directions.

The checker terminates the first client's exact PID from its own pppd PID
file. It requires that client's daemon row and native PPP interfaces to
disappear while the second client's original row, assignment and traffic
remain. It then terminates the second client and requires both sessions to
be gone while the configured access VLAN remains unchanged. Suite teardown
owns all three containers and their capture/log files, including failed runs.
This is a classless fixture foundation; it does not test CoS maps, original
Snapshot counts, native fences or retirement receipts.

On macOS, select this scenario through the registered lab route:

```sh
CGO_ENABLED=0 ./le --name session test qemu docker-lab lab 'test deployment docker-pppoe-accel' env 'ZE_PPPOE_INTEROP_SCENARIO=pppoe-shared-access-two-clients' timeout 3600s
```

This route resolves Ze's own runtime kernel and image and checks their
identity. Its kernel must provide `/dev/ppp`, PPPoE and VLAN support; the
native preflight is mandatory. Explicit `kernel` and `image` keywords belong
to the separate `command` mode and are refused with `lab`.
<!-- source: internal/le/interoplab/pppoe/shared_access_plan.go -- sharedAccessPlan -->
<!-- source: internal/le/interoplab/pppoe/shared_access_check.go -- checkSharedAccess -->

## Layout

```
test/interop-pppoe/
  Dockerfile.ze        Ze image used in both roles (copies the ze-linux the
                       suite preflight cross-compiles)
  ze-linux             the staged daemon, git-ignored and rewritten each run
  Dockerfile.accel     accel-ppp access-concentrator image
  Dockerfile.client    pppd and rp-pppoe client image
  scenarios/
    01-pppoe-chap-ipv4/         Ze client, accel-ppp concentrator
    02-ze-ac-pppd-client/       Ze concentrator, pppd client
    pppoe-omitted-mru-ze-ac/    Ze concentrator, peer MRU omitted, measured Ze transmit cap
    pppoe-chap-rechallenge-ze-client/  Ze client, native accel-ppp reauthentication
    pppoe-named-device-reconnect-ze-client/  Ze client as a CPE: pppoe0, static default, masquerade, reconnect
    pppoe-peer-terminate-ze-ac/        Ze concentrator, pppd LCP termination without incoming PADT
    pppoe-chap-only-pap-client/       Ze concentrator, CHAP-only default, PAP-only pppd client
    pppoe-empty-service-name/  Ze concentrator, pppd client, no service-name configured
    pppoe-padr-replay/         Ze concentrator, pppd client, max-sessions-per-mac 1
    ipv6cp-zero-identifier/    Ze concentrator, pppd session, zero IPv6CP identifier replayed
    ipv6cp-missing-option/     Ze concentrator, pppd client whose IPv6CP request carries no identifier option
internal/le/interoplab/pppoe/
  pppoe.go             Native images, preflight, selection, and lifecycle
  scenarios.go         Role-selected container plans and mounts
  check_client.go      Ze client assertions
  check_ac.go          Ze access-concentrator assertions
  check_named_device.go  pppoe-named-device-reconnect-ze-client: device name, route and NAT across a reconnect
  check_service_name.go  pppoe-empty-service-name: wire-level Service-Name proof
  check_auth_allow.go    pppoe-chap-only-pap-client: session-correlated policy refusal and resource cleanup
  check_padr_replay.go   pppoe-padr-replay: wire-level replay and per-MAC cap proof
  check_ipv6cp.go        ipv6cp-zero-identifier, ipv6cp-missing-option: wire-level
                         IPv6CP proof, BLOCKED until plan/post-release/features/spec-l2tp-ipv6-subscriber.md
                         lands (see the file's own header comment)
```

The Dockerfiles keep their small amount of peer initialisation in the image
entrypoint command. There are no separate runner or entrypoint scripts.

## Prerequisites

The native preflight probes `/dev/ppp` and the `pppoe` module from a temporary
container that holds the peers' grants and nothing more. It exits non-zero when
either is absent, names what is missing, and prints the host command that loads
it:

```
sudo modprobe -a ppp_generic pppoe
```

Ze's kernel builds both in, so `./le test qemu docker-lab` needs neither. Setting
`ZE_PPPOE_SKIP_KERNEL_PROBE` or `ze.pppoe.skip-kernel-probe` causes an immediate
refusal.

The probe decides whether Docker can carry the proof. Docker Desktop on macOS
was measured on 2026-08-01 and its Linux VM supplied both requirements on that
machine. The QEMU action remains available for a host where the probe refuses.

<!-- source: internal/le/interoplab/pppoe/pppoe.go -- preflight -->

The client image pins pppd **2.5.5** from the official GitHub tag archive with
a SHA256 checksum. The archive contains autotools inputs, not `configure`.
Upstream `autogen.sh` generates the build system and runs `configure` with
the image's existing installation paths. Build-only autotools dependencies
do not enter the runtime image. Native runs against pppd 2.5.5 passed
`pppoe-chap-only-pap-client`, `pppoe-pap-ze-ac` and
`02-ze-ac-pppd-client` on 2026-11-04. The CHAP-only scenario also failed
against a rebuilt producer with the old broad authentication default and
passed again after restoration.
<!-- source: test/interop-pppoe/Dockerfile.client -- PPP_VERSION, PPP_SHA256, build stage -->

## Running

### VPP mechanism image

`./le --name session build vpp run` builds the pinned, patched VPP image for
the PPPoE data-plane mechanism lab. Its recipe and patch provenance live in
[`third_party/vpp-patches`](../../third_party/vpp-patches/README.md). The
image includes the `af_packet`, Linux CP and PPPoE plugins, with one compile
worker for the small Docker VM. The answer names its immutable image ID.
The optional `variant missing-frame-metadata` keyword builds a separate proof
image that omits parsed metadata; the answer records its variant and immutable
ID. This counterfactual must fail the same runtime oracle the baseline passes.
This builds a lab dependency; the existing scenarios below still exercise
the kernel backend. The complete native mechanism proof passed on 2026-11-16,
including its separate missing-metadata image.
<!-- source: internal/le/build/vpp/build.go -- buildImage, imageRecipe -->
<!-- source: internal/le/build/vpp/register.go -- native build action and image registration -->

### VPP mechanism proof

The native Phase 0 proof is `./le --name session test deployment vpp-pppoe run`.
It starts three separate peers: patched VPP with a scratch control helper,
pppd 2.5.5 from the existing client image, and an independent IP traffic host.
The helper opens a PPPoX datagram socket without opening `/dev/ppp` or
creating a PPP unit. Its deliberately fixed, unauthenticated LCP/IPCP exchange
tests the transport mechanism; it is not Ze's subscriber control plane.

The proof runs plain, named-namespace and VLAN access topologies. It requires
a VPP session before PADS, reciprocal LCP over the raw socket, IPCP service,
positive traffic through VPP, a measured 1492-byte session MTU boundary,
increases in both VPP session packet counters across the controlled ping, and
RS/DHCPv6 delivery to the control socket. The VPP access MAC is set explicitly
and checked against the injection destination. The checker waits for pppd's
actual PPP interface and assigned address, then installs and reads back a
specific return route through that interface, so the shared Docker Ethernet
segment cannot carry subscriber replies around VPP.
VPP omits zero counters from this CLI output, so one small warm-up round trip
makes both counter rows observable before the separate measured ping burst.
An IP data frame appearing on the control TAP fails the run. Namespace sockets
are opened in the LCP namespace and then used after returning to the caller's
namespace. The VPP container receives the network and namespace grants and
TUN/vhost devices it needs. Its memlock soft and hard limits are 256 MiB:
the pinned AF_PACKET driver's one interface locks a 10 MiB RX ring and a
66 MiB TX ring. This limit retains headroom without an `IPC_LOCK` grant or
privileged mode.
<!-- source: internal/le/test/deployment/vpppppoe/plan.go -- mechanismPlan -->

The MTU check keeps the 1492-byte positive packet and 1493-byte refusal.
It then reads the client's actual PPP MTU and requires more than 1400 bytes.
Only VPP's session IPv4 MTU is lowered to 1400 and read back. A 1400-byte
DF packet must pass; a 1401-byte DF packet must fail with ping exit 1 and
an explicit measured MTU of 1400. The client's larger limit cannot produce
that boundary. VPP's MTU is restored to 1492 and read back, including after
failure or cancellation, with the existing bounded command budget.
<!-- source: internal/le/test/deployment/vpppppoe/mtu.go -- checkIndependentMTU -->

Before starting pppd, the checker requires an empty VPP session table and
records the `pppoe-input` malformed-length counter. The wire peer sends physically padded IPv6 packets whose
PPPoE declared length contains only the PPP protocol. A valid discovery frame
follows as a receive-path barrier. The malformed packets must not reach the
control TAP. After the barrier the session table must remain empty, and the
same receiving-node counter must increase by at least two. A stale count,
another node's matching description or an unreadable CLI answer fails.
This check runs before the
optional `test pppoe frame-bounds` buffer test, so an original image fails on
its packet behavior rather than simply lacking the new test command.

The unchanged 31 parser cases are followed by `test pppoe receive-bounds`.
That separate command submits real graph frames to exercise scalar input,
both paired input lanes and control dispatch. It checks native drops, counter
increments, initialized trace metadata and unchanged session allocation. Only
the complete answer `PPPoE receive bounds: 4 paths passed` permits dialing.
These checks passed in all three native topologies on 2026-11-16.

Discovery diagnostics capture at most 32 packets for at most 60 seconds on
the VPP peer, after confirming tcpdump has started. Failure evidence includes
its text output and the helper's first sixteen received/sent discovery
envelopes. The suite removes the container and capture on every cleanup path.
The scratch helper retries interrupted socket calls within its existing
context; blocking reads and writes each have one-second socket timeouts.
The VLAN topology creates an LCP pair for the data subinterface as well as
its parent, then maps the actual CP child returned by `show lcp`. Creating
only a Linux VLAN device leaves VPP without the matching control-side VLAN
interface and drops returning discovery before PPPoE dispatch.
<!-- source: internal/le/test/deployment/vpppppoe/capture.go -- startDiscoveryCapture -->
<!-- source: internal/le/test/deployment/vpppppoe/control_io.go -- controlIO -->

`run image sha256:<complete-image-id>` selects a prebuilt immutable VPP image
for that counterfactual while preparing the helper and independent client
from the current source. The normal run builds the canonical patched recipe.
Every action probes Docker, freshly stages its Ze/helper binaries and checks
the Docker host's runtime kernel before building its first source image. This
also applies to the combined proof's retained baseline, standalone binding and
metadata images, and the client image built beside a supplied VPP image.
An unsuitable host therefore refuses before the VPP source compilation starts.

The proof's Docker host must run Ze's kernel. The existing QEMU carrier boots
Ze's Docker-host appliance and forwards its Docker socket to this native lab:

```text
./le --name session test qemu docker-lab \
  lab "test deployment vpp-pppoe run" timeout 3600s
```

The `lab` branch selects and validates the appliance's own runtime kernel.
The separate `command`/`harness` branch starts no Docker daemon and is not this
carrier. Source image compilation on Colima establishes artifact buildability;
it does not establish these kernel-dependent packet results. This proof is
manually invoked while the post-release feature is developed. On 2026-11-16,
plain, named-namespace and VLAN access all passed on baseline image
`sha256:a0aac6ad351b37311384caa5fab112630d3a733875777faffc5689a61a6085bc`.
This uses the scratch control helper, not Ze's subscriber authentication and
session provider.

On failure, the shared suite captures bounded tails of VPP's startup/runtime
logs, the control helper's launch log and error/state markers, and pppd's log
before removing the containers and
temporary files. A failed Docker or permission probe of the error marker stays
an error; only a completed missing-file answer permits reading the state.
Each control helper VPP CLI command has the same 30-second budget as a
mechanism readiness query, including cleanup after the parent is canceled.
Image release failures are reported alongside the original setup or staging
refusal and any earlier cleanup errors.
<!-- source: internal/le/test/deployment/vpppppoe/plan.go -- Run, mechanismPlan -->
<!-- source: internal/le/test/deployment/vpppppoe/check.go -- checkMechanism -->
<!-- source: internal/le/test/deployment/vpppppoe/control_linux.go -- runControl, openInNamespace -->

The manually invoked `proof` action runs that entire positive mechanism,
then starts one separate VPP peer using its exact immutable image to run the
control-binding API regression described below. Only after both pass does it
build the `missing-frame-metadata` variant on the same Docker daemon. Its late
Docker layer reuses the completed positive compilation. Run all three stages
under one Ze-kernel guest with:

```text
./le --name session test qemu docker-lab \
  lab "test deployment vpp-pppoe proof" timeout 3600s
```

The final stage starts a separate VPP peer without a PPP client or control
helper. It reads back `/usr/share/doc/vpp/proof-variant`, then requires the
buffer test to answer `empty PADT: wrong session ID 0, expected 17`. Pinned
`vppctl` returns zero for CLI assertion errors, so the exact diagnostic proves
the intended failure; a process error, missing command, different assertion or
31-case success marker fails the proof. The result records each stage's image
identity, variant and outcome. A failed baseline or binding regression stops
before the counterfactual runs. The binding stage follows all three mechanism
topologies, preserving the malformed-wire-first check against an old receiver
that does not provide the optional binding command.

The isolated parser step has a 90-second bound and each CLI query has a
30-second bound. The shared suite removes its peer before the image lease is
released and task scratch is removed; image cleanup remains bounded after
cancellation. The combined proof passed on 2026-11-16: three baseline
topologies, same-image binding/ownership checks, and the exact metadata
assertion from image
`sha256:92da1e02c179f5731d3a6fbe9010333f50fe17bafc419252b589ab298f1d04d0`.
Its Ze-kernel guest reported check, command and lab exits of zero, stopped the
VM and left nothing behind. Product-provider and ownership-discrimination
tests remain separate.
<!-- source: internal/le/test/deployment/vpppppoe/proof.go -- runProof, runMetadataProof, checkMetadataOracle -->
<!-- source: internal/le/test/deployment/vpppppoe/register.go -- proof action and signal context -->

The manually invoked `bindings` action runs the normal baseline VPP image
alone and executes `test pppoe cp-binding`, followed by `test pppoe ownership`.
The first command exercises the C map API with
software and hardware interface indices that differ, invalid, absent and
mismatched deletions, preservation of unrelated bindings, and exact-pair
removal. Run it through the same Ze-kernel carrier:

```text
./le --name session test qemu docker-lab lab "test deployment vpp-pppoe bindings" timeout 3600s
```

The checker reads back the `baseline` variant and accepts only a completed
CLI call whose complete answer is `PPPoE cp binding: 10 cases passed`, with
no error output. In particular, `unmapped deletion was accepted` is a failed
API regression even though pinned `vppctl` exits zero for that assertion.
The report preserves the C assertion text; an unknown command, process error
or transport failure cannot count as a pass. This uses no deliberately broken
image and adds no proof variant. On 2026-11-15 the unchanged regression failed
with `unmapped deletion was accepted` before the separate producer repair,
then passed all ten cases with that repair in the same Ze-kernel Docker guest.
This establishes the binding API behavior; the packet mechanism proof is
separate. The checker then requires the complete answer
`PPPoE ownership: 55 cases passed`, with no error output, from the same isolated
VPP instance. This second command exercises process, session and policer object
tokens, guarded interface, route, gate and policer mutations, and index reuse.
It includes a stock IPv6 session whose FIB index has no corresponding IPv4
table. Foreign sessions remain present with their actual owner token; their
undecidable table ID is `~0` rather than an IPv4 lookup. A missing command,
the earlier 30- or 31-case answer, an assertion or cleanup error refuses the
stage. On 2026-11-16, both complete answers passed in the current Ze-kernel
guest on image
`sha256:a017bfa233b228b5e486a2ec74fc7a2c5d52c7cf078af66dead01e36139d4d06`.
The preceding ownership run exposed an IPv4 route left after withdrawal.
Preserving the requested path before native normalization repaired that failure
while retaining the assertion. Ownership discrimination and the full packet
mechanism proof remain separate checks.

This action shares the isolated parser step's fresh staged Ze, runtime-kernel
validation, 90-second scenario bound and bounded cleanup. The standalone
action builds one image lease and removes its peer before releasing that
lease and scratch; the shared image cache tag is preserved. Inside `proof`,
the binding stage reuses the completed baseline's immutable image directly,
without rebuilding it or acquiring a second build lease. The combined proof
owns the baseline's single build lease across the full mechanism, binding and
metadata stages. It releases that lease only after the last reached stage has
removed its peers, including on failure or cancellation. Moving the shared
cache tag during another build cannot remove this retained baseline image.
<!-- source: internal/le/test/deployment/vpppppoe/proof.go -- runBindingProof, runSingleVPPProof, checkBindingOracle, checkOwnershipOracle -->
<!-- source: internal/le/test/deployment/vpppppoe/register.go -- bindings action and answerBindings -->

### Kernel scenarios

```
./le test deployment docker-pppoe-accel
ZE_PPPOE_INTEROP_SCENARIO=01-pppoe-chap-ipv4 ./le test deployment docker-pppoe-accel
ZE_PPPOE_INTEROP_SCENARIO=02-ze-ac-pppd-client ./le test deployment docker-pppoe-accel
ZE_PPPOE_INTEROP_SCENARIO=pppoe-empty-service-name ./le test deployment docker-pppoe-accel
ZE_PPPOE_INTEROP_SCENARIO=pppoe-padr-replay ./le test deployment docker-pppoe-accel
ZE_PPPOE_INTEROP_SCENARIO=ipv6cp-zero-identifier ./le test deployment docker-pppoe-accel
ZE_PPPOE_INTEROP_SCENARIO=ipv6cp-missing-option ./le test deployment docker-pppoe-accel
ZE_PPPOE_INTEROP_SCENARIO=pppoe-pap-ze-ac ./le test deployment docker-pppoe-accel
ZE_PPPOE_INTEROP_SCENARIO=pppoe-pap-ze-client ./le test deployment docker-pppoe-accel
ZE_PPPOE_INTEROP_SCENARIO=pppoe-chap-rechallenge-ze-client ./le test deployment docker-pppoe-accel
ZE_PPPOE_INTEROP_SCENARIO=pppoe-named-device-reconnect-ze-client ./le test deployment docker-pppoe-accel
ZE_PPPOE_INTEROP_SCENARIO=pppoe-peer-terminate-ze-ac ./le test deployment docker-pppoe-accel
ZE_PPPOE_INTEROP_SCENARIO=pppoe-chap-only-pap-client ./le --name session test deployment docker-pppoe-accel
```

`01-pppoe-chap-ipv4` and `02-ze-ac-pppd-client` predate the naming rule and keep
their numeric prefixes. A scenario added since carries none:
`interoplab.Discover` and `ZE_PPPOE_INTEROP_SCENARIO` both match a scenario by
its directory name, and a number is a reservation a later scenario can take, or
a hole nothing tells apart from one (`ai/rules/interop-and-goal-validation.md`).

`NO_BUILD` skips image builds, `SESSION_TIMEOUT` changes the default 90-second
scenario bound, and `ZE_PPPOE_INTEROP_SUFFIX` provides parallel-run isolation.

The QEMU proof for Ze as a client against accel-ppp is:

```
./le test qemu run kernel tmp/kernel/build/vmlinuz \
  packages "iproute2 iputils accel-ppp ppp kmod" \
  command "./le test qemu pppoe-accel"
```

The host `qemu run` action boots the runtime kernel and installs the guest
packages. The guest `pppoe-accel` action then runs Ze and accel-ppp in
two network namespaces joined by a veth. Invoking the guest action on the
host does not boot a VM. This proof covers the client role only. The Docker
suite carries both roles.

<!-- source: internal/le/test/qemu/pppoe_accel_linux.go -- runPPPoEAccelGuest -->

## Scenarios

### 01-pppoe-chap-ipv4

Ze authenticates as `alice` with CHAP-MD5. IPCP assigns `10.11.0.2` with
`10.11.0.1` as its peer. The checker requires one Ze PPP device, `pppoe0`, with that
point-to-point address, a route through it, ICMP to the peer, an accel-ppp
session for `alice`, and removal of that session after Ze stops.

accel-ppp's `[dns]` section offers `192.0.2.53` and `192.0.2.54`, which it
Naks into Ze's IPCP request. While the session is up, and before it stops Ze,
the checker requires Ze's `/tmp/resolv.conf` to list them in that order after
the generated header. It then runs `show interface pppoe-client` through Ze's
REST execute endpoint (enabled in the scenario's `ze.conf` on `127.0.0.1:9099`,
so the `curl` runs inside the Ze container) and requires `pppoe0`'s
`dns-servers` to be `["192.0.2.53", "192.0.2.54"]`. It reads the raw JSON, so
an absent key or a `null` is refused. `pppoe-ipcp-dns-rejected-ze-client` runs
the same checks against an accel-ppp that Rejects both DNS options, so the
session leases no server and `dns-servers` must be an empty list. With no lease
listing a server, Ze falls back to the host's own resolvers, so the checker
reads the Ze container's `/etc/resolv.conf` (Docker's, which Ze never writes)
and requires Ze's resolv.conf to be the generated header followed by exactly
those servers, in that order. It refuses a host list that holds accel-ppp's
`10.11.0.1`, because that server would match whether or not the client dropped
the Rejected options.
<!-- source: internal/le/interoplab/pppoe/check_client.go -- checkZeClient, checkZeClientDNSRejected, zeHostResolvers, checkZeClientShowDNS, pppoeClientDNSServers -->

The checker also reads accel-ppp's verbose trace and requires, in order, its
LCP Configure-Request with `<auth CHAP-md5>`, Ze's Configure-Ack carrying that
option, a CHAP Challenge, Ze's Response naming `alice`, and a CHAP Success.
A PAP exchange is refused even if IPCP and ping succeed. The shared session
checks remain the same for the PAP scenario; each scenario checks its own
authentication evidence.

These spellings come from accel-ppp 1.14.0's
[`ppp_lcp.c`](https://github.com/accel-ppp/accel-ppp/blob/1.14.0/accel-pppd/ppp/ppp_lcp.c),
[`auth_print`](https://github.com/accel-ppp/accel-ppp/blob/1.14.0/accel-pppd/ppp/ppp_auth.c),
and [CHAP-MD5 handler](https://github.com/accel-ppp/accel-ppp/blob/1.14.0/accel-pppd/auth/auth_chap_md5.c),
not from Ze's configured method. Both client fixtures set `[log] copy=1`:
accel-ppp's
[`general_log`](https://github.com/accel-ppp/accel-ppp/blob/1.14.0/accel-pppd/logs/log_file.c)
otherwise discards session messages from the general log exposed by Docker.
<!-- source: internal/le/interoplab/pppoe/check_client.go -- checkZeClient, checkZeClientSession, accelCHAPEvidence -->

### 02-ze-ac-pppd-client

The independent pppd client requests service `internet`, authenticates as
`alice`, negotiates IPCP, and pings Ze through its PPP interface. The checker
reads pppd's discovery, bidirectional LCP, CHAP, and IPCP trace and compares it
with Ze's REST session table. It then requires PADT and empty state before
repeating the dial with `wrong-secret`; the second trace must reach CHAP and
receive a refusal without creating a session.

### pppoe-omitted-mru-ze-ac

The real pppd client omits its MRU option. The checker correlates its decoded
Configure-Request with Ze's matching Configure-Ack and requires reciprocal
LCP acknowledgment, CHAP success and assigned IPv4 service. It measures Ze's
own `pppN` interface with `ip -j link show` and requires MTU 1492. A client
interface capped at 1492 cannot substitute for this Ze-side measurement.
Both peers must send traffic through the session, followed by PADT, empty
session tables and release of Ze's kernel PPP resources.
<!-- source: internal/le/interoplab/pppoe/check_mru_omitted.go -- checkOmittedPeerMRU, omittedMRULCPEvidence, omittedMRUZeLink -->

The checker appends pppd's `default-mru` option to the normal typed dial.
In pinned pppd 2.5.5, `lcp_addci` omits MRU when `neg_mru` is false.
The PPPoE plugin otherwise lowers the requested MRU to 1492 for a normal
Ethernet link, so leaving out a command-line `mru` value alone does not
exercise omission. `default-mru` also refuses Ze's initial MRU option;
successful replacement negotiation remains part of this proof. The test
retains pppd's full trace and Ze's measured link JSON in the native job log.
<!-- source: internal/le/interoplab/pppoe/check_mru_omitted.go -- checkOmittedPeerMRU -->
<!-- source: test/interop-pppoe/Dockerfile.client -- PPP_VERSION -->

Run `ZE_PPPOE_INTEROP_SCENARIO=pppoe-omitted-mru-ze-ac ./le --name session test deployment docker-pppoe-accel`.
The scenario is authored; a passing run and rebuilt defective-artifact
discrimination must be recorded before its behavior is claimed as proved.
<!-- source: internal/le/interoplab/pppoe/check_mru_omitted.go -- init -->

### pppoe-chap-rechallenge-ze-client

After IPCP traffic succeeds, the checker observes two native accel-ppp CHAP
reauthentication exchanges. It checks each Response's Identifier, CHAP-MD5
digest and username, and each corresponding Success. The kernel PPPoE session,
PPP interface, address and route must remain unchanged, and traffic must still
cross the session. This separates network-phase CHAP from initial login.

<!-- source: internal/le/interoplab/pppoe/check_lifecycle.go -- checkCHAPRechallenge -->
<!-- source: internal/le/interoplab/pppoe/check_lifecycle_wire.go -- rechallengeEvidence -->
<!-- source: test/interop-pppoe/scenarios/pppoe-chap-rechallenge-ze-client/accel-ppp.conf -- native auth interval -->

### pppoe-named-device-reconnect-ze-client

Ze runs the home CPE configuration the guides show. The `pppoe-client pppoe0`
entry sets `no-default-route`, a static route `0.0.0.0/0` has
`next { interface pppoe0 }`, and a NAT postrouting term masquerades
`output-interface "pppoe0"`. A dummy `lan0` at `198.51.100.1/24` stands in for
the LAN. The checker first deletes the default route Docker gives the
container through `eth0`. A CPE has no such route, and at metric 0 it would
collide with fib-kernel's add of the static default. If Ze already holds a PPP
device at that point, the checker stops and asks for a rerun. It also adds
`192.0.2.1` to accel-ppp's loopback: only the static default covers that
address on Ze's side.

On the first session the checker requires:

- `pppoe0` as the only PPP device in Ze's container, so no `pppN` comes from
  the client
- its IPCP address
- the default route at protocol 250 (fib-kernel) through `pppoe0`
- `ip route get 192.0.2.1` through `pppoe0`
- a ping from `198.51.100.1` to `192.0.2.1` that is answered, which needs the
  masquerade, because accel-ppp holds no route back to `198.51.100.0/24`
- a higher packet count on the masquerade rule's counter, read with
  `nft list chain ip ze_cpenat postrouting`

The checker then runs `accel-cmd terminate username alice`. In accel-ppp,
`terminate_exec2` (`accel-pppd/cli/std_cmd.c`) matches the session by
username and calls `ap_session_terminate`, and the PPPoE `disconnect` sends
PADT. The checker waits for a new `pppoe0` with another ifindex. It then
requires the same evidence again: the default route back through the new
device with no reload, the masqueraded ping, and a higher counter. The second
session's address is `10.11.0.3`, because accel-ppp's pool hands out the head
of its free list (`get_ip` in `accel-pppd/extra/ippool.c`) and `put_ip` returns
`10.11.0.2` to the tail.

The accel-ppp functions were read at `06f64b1` of the upstream tree. The lab
runs Alpine's 1.14.0 package.
<!-- source: internal/le/interoplab/pppoe/check_named_device.go -- checkNamedDeviceReconnect, removeDockerDefaultRoute, namedMasqueradeThroughSession -->
<!-- source: test/interop-pppoe/scenarios/pppoe-named-device-reconnect-ze-client/ze.conf -- CPE configuration -->

The scenario passed on 2026-11-08 (`pppoe-named-device-reconnect-ze-client:
pass (9s)`) on the unbroken tree, and it discriminates. Two rebuilt defective
producers each turned it red on the same day:

| Break | Red |
|-------|-----|
| `pppoeclient.Dialer.Dial` passes `""` instead of `cfg.Name` to `ppp.DevPPPSetup`, so the kernel names the unit `pppN` | `first session: wait for PPP device pppoe0 in ze timed out` |
| static's link watch (`internal/plugins/static/link.go`) hears the device's first up and its loss, then drops every later event | `session after accel-ppp terminated the first (ifindex 11): the static default route did not reach the kernel through pppoe0` |
<!-- source: internal/component/l2tp/pppoeclient/dialer.go -- Dialer.Dial -->
<!-- source: internal/plugins/static/link.go -- ifaceLinks.subscribe -->

The second red is the reconnect half of the claim: the route came back only
because static heard `pppoe0` return.

### pppoe-peer-terminate-ze-ac

The checker first establishes authenticated traffic with pppd. It then filters
only pppd's outgoing PADT and asks pppd to terminate. Capture at Ze must show the
peer's LCP Terminate-Request, the matching Ze Terminate-Ack, and Ze's own PADT
after the Restart timer. No client PADT may reach Ze. The checker records Ze's
live PPP interface before termination, then requires the REST session and both
peers' PPP interfaces to disappear. Ze's `/proc/net/pppoe` must retain only its
header, independently proving kernel session cleanup. The filter requires the
host's `clsact`, `u32`, `mirred` and `dummy` support and fails closed if
installation fails. It redirects only PADT into a dummy interface owned by the
client container; removing that container removes the sink too. Redirect consumes
the matched frame rather than mirroring it, and the dummy driver frees it without
transmission. These facilities already ship for Ze's interface and mirroring
features, so this fault injection needs no test-only `gact` kernel requirement.

The action semantics come from iproute2's
[`parse_direction`](https://github.com/iproute2/iproute2/blob/main/tc/m_mirred.c)
and Linux's
[`tcf_mirred_to_dev`](https://github.com/torvalds/linux/blob/master/net/sched/act_mirred.c)
and [`dummy_xmit`](https://github.com/torvalds/linux/blob/master/drivers/net/dummy.c).

<!-- source: internal/le/interoplab/pppoe/check_lifecycle.go -- checkPeerTerminate -->
<!-- source: internal/le/interoplab/pppoe/check_lifecycle_wire.go -- terminateEvidence -->

### pppoe-chap-only-pap-client

Ze's AC has neither `auth-method` nor `authentication-allow` configured, so
its effective policy is CHAP-MD5 only. The real pppd client refuses CHAP,
EAP and both MS-CHAP variants, leaving PAP as its only acceptable method.
The fixture supplies valid credentials and an address pool: an accidental
PAP downgrade cannot hide behind a credential or address-allocation failure.

Before dialing, the checker waits for tcpdump's capture-activation marker at
Ze. It requires Ze's successful PADS, its CHAP-MD5 Configure-Request, the
client's matching-Identifier PAP Configure-Nak, Ze's LCP Terminate-Request,
and exactly one **Ze-originated** PADT, all on the PADS session ID and the
same Ethernet endpoints. A client PADT cannot substitute for Ze's PADT.
No PAP or other authentication/NCP traffic may appear, and a client
Configure-Ack that could let Ze's LCP reach Opened is refused. The checker
also reads pppd's own Configure-Nak and Terminate-Request trace, and requires
Ze's refusal record to name the PAP suggestion and the CHAP-only allow-list.

The checker does not kill pppd or sleep for a guessed teardown interval.
It waits for the dial to exit, Ze's REST session table to empty, both peers'
PPP interfaces to disappear, and Ze's `/proc/net/pppoe` to retain only its
header. Capture remains active through those observations; the final flushed
capture is judged again so duplicate Ze PADTs count.

The independent peer behavior comes from pppd 2.5.5's
[`lcp_reqci`](https://github.com/ppp-project/ppp/blob/v2.5.5/pppd/lcp.c),
which Naks refused CHAP with PAP when EAP is refused, and
[`fsm_rtermreq`](https://github.com/ppp-project/ppp/blob/v2.5.5/pppd/fsm.c),
which acknowledges termination before Opened but keeps negotiating until
Ze's lower-layer teardown. This negative scenario complements, rather than
changes, the explicit `auth-method pap` success proof below.

<!-- source: internal/le/interoplab/pppoe/check_auth_allow.go -- checkCHAPOnlyPAPClient, chapOnlyRefusalEvidence -->

### pppoe-empty-service-name

Ze's AC carries no `service-name` leaf. The independent pppd client dials with
no requested service (RFC 2516 Section 5.1's "any service is acceptable"). The
checker captures the discovery exchange on the wire (tcpdump inside the client
container, decoded with `internal/core/pcap` and
`internal/component/l2tp/pppoe.ParseDiscovery`) and requires exactly one
Service-Name tag on the PADO and exactly one on the PADS, alongside the same
LCP/CHAP/IPCP proof `02-ze-ac-pppd-client` performs. A session coming up is not
enough evidence here: the unit tests already pin `BuildPADO`/`BuildPADS` always
writing one tag, so this scenario's job is proving a real peer reads the same
bytes off Ze's own wire, not proving the encoder again.

### pppoe-padr-replay

Ze's AC carries `max-sessions-per-mac 1`. The checker dials one session with the
independent pppd client, waits until it is live in PPP, then replays the exact
captured PADR frame from inside the client container with `tcpreplay`
(`interoplab.SendFrameInContainer`, the one frame sender every lab uses, the BGP
lab's IS-IS purge injector included). The send runs in the container because an
unprivileged checker cannot enter a rootful Docker container's network
namespace. It requires the PADS that replay provokes to
carry the SAME session id, never a second one. It then dials a second,
genuinely independent session from the same MAC and requires the PADS it
provokes to carry session id `0x0000` and an AC-System-Error tag, read off the
captured frame -- a session count that stayed at one is not enough evidence on
its own for either claim (`ai/rules/interop-and-goal-validation.md`, "Prove a
scenario discriminates"). The first dial's pppd is still running during the
second dial and owns the session the cap counts, so the second pppd carries
`ipparam ze-over-cap-dial` and the checker stops only that one (`pkill -f`).
Ze's table must still list the first session afterwards.
`TestOverCapDialCleanupSparesTheLiveSession` reproduces the failure a stop of
every pppd caused. `spec-pppoe-padr-replay-allocates-unbounded-sessions`.

### pppoe-pap-ze-ac

Ze's AC carries `auth-method pap`, and pppd dials with `refuse-chap`, so PAP is
the only method both ends accept. The checker requires pppd's trace to show
Ze's Configure-Request demanding `<auth pap>`, the client's named
Authenticate-Request and Ze's Authenticate-Ack, with no CHAP anywhere, then
IPCP, ICMP to Ze, and PADT teardown. It captures the session-stage frames
(0x8864) on the client's wire and requires exactly one Authenticate-Request and
one Authenticate-Ack carrying the same Identifier, both on the PPPoE SESSION_ID
Ze's REST table reports.

It then resends that captured Authenticate-Request from inside the client
container with `tcpreplay`, after IPCP has opened, with its Identifier advanced
by one, because a peer retransmitting MUST change it (RFC 1334 Section 2.2.1:
"The Identifier field MUST be changed each time an Authenticate-Request packet
is issued"). It requires Ze to put exactly one Authenticate-Ack and no Nak on
the wire, on the same SESSION_ID, carrying the new Identifier (Section 2.2.2:
"The Identifier field MUST be copied from the Identifier field of the
Authenticate-Request which caused this reply"), so a cached Ack resent with the
old Identifier fails. Afterwards Ze's REST table must still list that one
session, under the same SESSION_ID, and the client must still hold one PPP
interface. That is RFC 1334 Section 2.2.1: "the authenticator MUST allow
repeated Authenticate-Request packets after completing the Authentication
phase", answered with the same Code (`internal/component/l2tp/ppp/pap.go`,
`reanswerPAP`). `TestCheckPAPReanswerJudgesTheReply` drives the same judgement
over scripted replies.
`spec-ppp-pap-reanswer-after-auth`.

### pppoe-pap-ze-client

Ze dials accel-ppp as in `01-pppoe-chap-ipv4`, but accel-ppp loads `auth_pap`
and no CHAP module, so the session comes up only if Ze's client authenticates
with PAP (`internal/component/l2tp/pppoeclient/session.go`, `runClientAuth`).
The checker runs the shared session proof (link, address, route, accel-ppp
session, ICMP, removal after Ze stops), then reads accel-ppp's own
trace and requires `recv [PAP AuthReq` and `send [PAP AuthAck`, with no CHAP.
The lab cannot lose a frame on demand, so the retry itself stays proven by the
discriminated unit tests in `pppoeclient`; this scenario proves the PAP path
interoperates. `spec-pppoe-client-pap-retry`.

### pppoe-ipv6cp-ze-client

Ze dials accel-ppp with `ipv6 enabled true`, `accept-ra 2`, `autoconf true`
and `forwarding true` on its `pppoe-client`. accel-ppp runs `ipv6=prefer`,
so the session starts only once IPv6CP opened. Its own identifier is
`0:0:0:1`, and `ipv6-peer-intf-id=0:0:0:5a` with
`ipv6-accept-peer-intf-id=0` makes it Nak any other identifier the client
offers with `0:0:0:5a` (`ipaddr_recv_conf_req` in accel-ppp's
`ppp/ipv6cp_opt_intfid.c`). Ze starts from its source MAC's EUI-64, so the
Nak is always exercised. `ipv6pool` hands the session the first /64 of
`2001:db8:100::/56`, and `ipv6_nd` advertises it with the autonomous flag.

The checker reads accel-ppp's own trace for, in order: Ze's Configure-Request
with one non-zero Interface-Identifier and no other option, accel-ppp's
Configure-Nak of that request suggesting `0:0:0:5a`, Ze's next request
carrying exactly `0:0:0:5a` (RFC 5072 Section 4.1), and accel-ppp's
Configure-Ack of it, plus Ze's Configure-Ack of accel-ppp's own request.
In Ze's container it then requires the device to be `pppoe0`, `fe80::5a/64`
on it, `accept_ra` 2 and `forwarding` 1 on it, a default route via `fe80::1`
(accel-ppp's link-local) through it, a global address SLAAC formed in
`2001:db8:100::/64`, and an IPv6 ping to `fe80::1` across the session. The
IPv4 session proof of `01-pppoe-chap-ipv4` follows.

### pppoe-ipv6cp-rejected-ze-client

The same Ze client against an accel-ppp with `ipv6=deny`, which answers every
IPv6CP packet with an LCP Protocol-Reject (`ipv6cp_recv` in accel-ppp's
`ppp/ppp_ipv6cp.c`). The checker waits for that reject, pings the IPv4
gateway five times (longer than Ze's 3-second IPv6CP Restart timer), then
requires from accel-ppp's trace exactly one Protocol-Reject of IPv6CP, so the
client stopped sending it (RFC 1661 Section 5.7), no LCP Terminate-Request
either way, and exactly one CHAP success, so there was no second session.
`pppoe0` must hold no link-local address. The IPv4 session proof follows
(RFC 7084 WLL-3).

Both scenarios carry an `ipv6-session` file. Its presence makes the lab set
`net.ipv6.conf.default.disable_ipv6=0` in both containers, so the PPP devices
they create at run time keep IPv6, and run Ze under the lab's net-sysctl
grant, because the client writes its session device's sysctls each time a
session creates it (`scenarios.go`, `ipv6SessionScenario`). accel-ppp has no
command that closes IPv6CP alone, and once IPv6CP has opened accel-ppp ends
the whole session when it closes (`ipv6cp_layer_finished`), so an IPv6CP
close under a live IPCP stays proven by the `pppoeclient` unit tests.

### ipv6cp-zero-identifier and ipv6cp-missing-option

Written and registered against RFC 5072 Section 4.1's zero-identifier Nak
and its missing-option one-shot Nak, driven by a real pppd client and
read off the wire (`check_ipv6cp.go`), the same shape `pppoe-padr-replay`
established. pppd cannot be made to send either request: its `ipv6`
option refuses the zero identifier ("Illegal interface identifier (local)",
`setifaceid` in pppd's `ipv6cp.c`), and it always sends the option. So both
checkers capture pppd's own IPv6CP Configure-Request and replay rebuilt
copies of it: the zero-identifier checker sends the zero value, then the
value Ze's Configure-Nak suggested, and wants a Configure-Nak and then a
Configure-Ack from Ze's MAC. **Neither can pass today.** `poolPlugin.handle`
(`internal/component/l2tp/plugins/pool/register.go`) answers every
`EventIPRequest` whose family is not IPv4 with `Accept: false`, and
`runNCPPhase` (`internal/component/l2tp/ppp/ncp.go`) reads that decline
before the session reads a single client frame, so Ze's IPv6CP FSM never
starts, on any configuration. Both scenarios use the pinned pppd peer: this
lab's client image carries pppd and rp-pppoe, and accel-ppp-as-client is not a
role its images support. Both checkers return a citing error at the first
missing wire evidence rather than passing vacuously.
`plan/post-release/features/spec-l2tp-ipv6-subscriber.md` is what switches them on.

## Relationship to other evidence

| Evidence | Ze role | Independent peer | Kernel PPPoE |
|----------|---------|------------------|--------------|
| `test/pppoe/pppoe-basic.ci` | Access concentrator | Functional fixture | No |
| `test/pppoe/pppoe-vlan.ci` | Access concentrator on VLAN | Functional fixture | No |
| `test/pppoe/pppoe-service-name.ci` | Access concentrator | Functional fixture | No (`option=netns-link`, `./le test qemu pppoe`) |
| `./le test deployment docker-pppoe-accel` | Client and access concentrator | accel-ppp and pppd | Host kernel |
| `./le test qemu pppoe-accel` | Client | accel-ppp | Runtime kernel |

## Configured Ze with the native VPP provider

The separate `product` action runs the staged Ze daemon as the access
concentrator. It uses the existing VPP source image, independent pppd 2.5.5
image, Docker Suite and Ze-kernel carrier. The VPP peer runs Ze and holds the
actual VPP API/statistics sockets. No API proxy, scratch PPP server or
replacement daemon answers for either process.
<!-- source: internal/le/test/deployment/vpppppoe/product_plan.go -- RunProduct, runProductSuite, productPlan -->

```text
./le --name session test qemu docker-lab \
  lab "test deployment vpp-pppoe product" timeout 3600s
```

For a source counterfactual using an already built native VPP image, the
optional `image` keyword accepts its complete immutable `sha256:` identity:

```text
./le --name session test deployment vpp-pppoe product image sha256:<complete-image-ID>
```

That direct form requires the already selected Docker daemon to run Ze's
kernel. Both routes refuse an unsuitable Docker host before they build an
image. Both routes also stage the daemon and le helper from current source.
<!-- source: internal/le/test/deployment/vpppppoe/product_plan.go -- runProductSuite -->

The checker writes the daemon's configuration to the scenario's evidence
directory, `/evidence/product.conf` in the VPP peer, so a failed run leaves it
to read. Ze opens its config store beside the configuration it starts on and
refuses a store another uid owns. Under `test qemu docker-lab` the evidence
directory reaches the container through the guest's 9p share, where every
file carries the host user's uid, even one the container's root creates. Ze,
running as root, would refuse its own `database.lock` there. So every product
action (`product`, `product-ipv6`, `product-framed`) copies the configuration
to `/var/lib/ze-product` in the VPP peer's own filesystem and starts Ze on
that copy. Its log stays at `/evidence/product-ze.log`.
<!-- source: internal/le/test/deployment/vpppppoe/product_check.go -- startProductDaemon, productStartCommand -->

The loopback REST observer is a registered, bounded le action. It uses the
existing authenticated execute endpoint and shared renderer. Its command
allowlist contains only the session and interface show commands. An error,
warning, missing data, null data or oversized reply cannot prove that the
table is empty.
<!-- source: internal/le/test/deployment/vpppppoe/product_observe.go -- observeProduct, answerProductObserve -->

Each plain, named-namespace and VLAN topology configures the VPP backend and
CHAP-MD5 against Ze. A bounded Ethernet capture must contain every successful
client's PADI/PADO/PADR/PADS exchange, with Ze's configured AC-Name and
Service-Name. The physical access source MAC and SID must agree with Ze and
native VPP.
<!-- source: internal/le/test/deployment/vpppppoe/product_discovery.go -- productDiscovery, productDiscoveryTags -->

The lab prepares only AF_PACKET and LCP infrastructure. Ze must create its
own PPPoE map, native session, peer host route and session forwarding state.
The server has no `/dev/ppp` device, and the checker reads both relevant
namespaces for Linux PPP interfaces.
<!-- source: internal/le/test/deployment/vpppppoe/product_check.go -- prepareProduct, noProductKernelPPP -->

Before traffic starts, the peer's CHAP trace and negotiated kernel PPP
address must agree with Ze's native interface identity and session dump.
The `ze-pppoe` FIB owner must match that same session. Its exact source path
must attach to that native session interface. Another source's path or the
forwarding display cannot supply that association.
<!-- source: internal/le/test/deployment/vpppppoe/product_state.go -- productAuth, matchProductNative, productOwner, productHostRoute, productOwnedPath -->

Traffic travels in both directions between the subscriber and an
independent host. Source routes on the peer force replies through the
corresponding PPP interface. Both native packet counters must increase for
each subscriber.
<!-- source: internal/le/test/deployment/vpppppoe/product_check.go -- productTraffic -->

A capture on the actual LCP control TAP starts before those traffic
barriers. The checker joins its exact capture process and requires a complete
zero-capture, zero-kernel-drop summary. A missing summary is a failure.
<!-- source: internal/le/test/deployment/vpppppoe/product_check.go -- startProductCapture, finishProductCapture -->
<!-- source: internal/le/test/deployment/vpppppoe/product_discovery.go -- joinProductCapture -->

Single-client topologies require a successful 1492-byte IP packet and
measured refusal of a 1493-byte DF packet. They also require the native-only
1400/1401 boundary while the peer's MTU remains larger. The checker restores
the smaller native MTU even if its probe fails.
<!-- source: internal/le/test/deployment/vpppppoe/product_check.go -- productTraffic -->
<!-- source: internal/le/test/deployment/vpppppoe/mtu.go -- checkIndependentMTU -->

After graceful peer closure, Ze and native VPP must report session absence.
The peer's PPP interface must disappear, and the owned host route must be
withdrawn. Another accepted dial exercises the released pool capacity.
A wrong secret must produce a real CHAP Challenge/Response/Failure and never
IPCP success.
<!-- source: internal/le/test/deployment/vpppppoe/product_check.go -- checkProduct, waitProductEmpty, productRoutesGone -->
<!-- source: internal/le/test/deployment/vpppppoe/product_state.go -- productAuth -->

The hundred-client topology uses one independent client container with a
hundred macvlan devices and distinct locally administered unicast MACs.
All hundred sessions must have unique native identities and negotiated peer
addresses, owned host routes and successful bidirectional traffic.
<!-- source: internal/le/test/deployment/vpppppoe/product_check.go -- checkProduct, productClient, productTraffic -->
<!-- source: internal/le/test/deployment/vpppppoe/product_state.go -- validateProductSessions, productPopulation -->

A 101st MAC must complete PADI/PADO/PADR and receive a zero-SID
AC-System-Error PADS from the configured physical AC. Its explicit Host-Uniq
must match all four frames. The actual daemon must report session-ID
exhaustion, and the joined pppd must log that resource refusal. A discovery
timeout or unrelated process failure cannot satisfy the limit check.
<!-- source: internal/le/test/deployment/vpppppoe/product_capacity.go -- productCapacityRefusal, finishProductCapacity, productCapacityReason -->

All hundred existing identities must remain unchanged after the refusal.
One exact admitted pppd process then ends. All 99 surviving identities and
their bidirectional traffic must remain intact. The pool has 101 addresses,
so this limit check cannot succeed through pool exhaustion alone.
<!-- source: internal/le/test/deployment/vpppppoe/product_check.go -- checkProduct, signalProductDial -->
<!-- source: internal/le/test/deployment/vpppppoe/product_state.go -- sameProductOwners -->

The checker then requires removal of all hundred native sessions and their
routes. This is a scoped `max-sessions 100` acceptance check. It provides no
throughput or performance measurement.
<!-- source: internal/le/test/deployment/vpppppoe/product_check.go -- checkProduct, waitProductEmpty, productRoutesGone -->

The native-create-before-PADS claim requires the server/provider unit, the
native C proof and the real-product wire/traffic run. The unit holds actual
native factory completion and requires no successful PADS before its
acknowledgment. The C proof establishes the owned-create acknowledgment and
readback. A later native dump alone cannot establish this ordering.
<!-- source: internal/le/test/deployment/vpppppoe/product_check.go -- checkProduct -->

A successful product run establishes the observed final session and packet
path. It does not establish timestamp ordering between independent clocks.
The peer trace vocabulary comes from pppd 2.5.5 `chap.c`
(`chap_handle_status`, `chap_print_pkt`). The checker reads negotiated IP
state from the peer kernel rather than log text.
<!-- source: internal/le/test/deployment/vpppppoe/product_state.go -- productAuth, parseProductLinks -->

This action is the IPv4 product base. Separate required acceptance packages
cover RADIUS/accounting, framed routes and VRFs, policers and CoA. They also
cover IPv6 control and delegated prefixes, physical restart/reconnect
recovery, and all access-index/SID reuse and foreign-object cases. A pass
here cannot close those acceptance criteria. The `product-framed` action
below covers one RADIUS Framed-Route with a non-peer gateway, not the rest of
RADIUS, accounting or VRFs.
<!-- source: internal/le/test/deployment/vpppppoe/product_check.go -- checkProduct -->

Source alone does not record a product-lab pass. Retain the actual named
scenario results and negative counterfactuals before you claim a pass.
Suite owns all containers, networks, staged binaries and captures. The QEMU
carrier owns the guest and must report completed cleanup as part of the
result.
<!-- source: internal/le/test/deployment/vpppppoe/product_plan.go -- RunProduct, productPlan, productConfig -->
<!-- source: internal/le/test/deployment/vpppppoe/product_check.go -- checkProduct, prepareProduct, productTraffic, finishProductCapture -->
<!-- source: internal/le/test/deployment/vpppppoe/product_state.go -- validateProductSessions, matchProductNative, productPopulation -->
<!-- source: internal/le/test/deployment/vpppppoe/product_observe.go -- observeProduct, answerProductObserve -->
<!-- source: internal/le/test/deployment/vpppppoe/product_discovery.go -- productDiscovery, joinProductCapture -->

<!-- source: internal/le/test/deployment/vpppppoe/product_capacity.go -- productCapacityRefusal, finishProductCapacity -->

### A RADIUS Framed-Route through a VPP session

The separate `product-framed` action reuses the product lab's staging,
immutable VPP image selection, pinned pppd 2.5.5 image and Suite cleanup in
the plain topology, and adds pinned FreeRADIUS 3.2.10 as a fourth peer. Ze
authenticates through `l2tp { auth { radius { ... } } }` instead of a local
user, so a CHAP success is FreeRADIUS's Access-Accept:
<!-- source: internal/le/test/deployment/vpppppoe/product_framed.go -- productFramedPlan, framedAuth -->

```sh
./le --name session test qemu docker-lab lab "test deployment vpp-pppoe product-framed" timeout 3600s
```

FreeRADIUS answers `alice` with `Framed-Route = "203.0.113.128/25
203.0.113.77 1"`. In 3.2.10, rlm_files' `file_common` moves that reply item
into the Access-Accept and `rad_vp2rfc` encodes it as its text. The gateway is
neither `0.0.0.0` nor the subscriber's pool address, so Ze installs a device
route through the session (`newFramedRouteReceipt`), and its VPP provider
sends a path with only the session interface and the metric as preference.
The server reuses the subscriber lab's `site-default` and `ze_detail` module;
the lab writes its `clients.conf`, one entry per candidate network, and its
users file. VPP owns the peer's `eth0` address, so Ze's RADIUS client binds a
macvlan with its own MAC and address on the same segment, which VPP's L3
input drops.
<!-- source: internal/le/test/deployment/vpppppoe/product_framed.go -- framedUsers, framedClients, framedNAS -->

After IPCP, `show ip fib 203.0.113.128/25` must hold one `ze-pppoe` source
whose one path is `attached` to the session interface at `pref=1`, and the
entry must not name `203.0.113.77` anywhere. VPP v25.10 `fib_path_create`
makes a path with a next hop `attached-nexthop`, so a route kept via the
gateway fails both checks. The subscriber holds `203.0.113.129/32` on its
loopback and returns traffic through its PPP link; the traffic host routes
the /25 through VPP and pings that address, and both native session counters
must rise. After pppd's graceful close, the session, its host route and the
/25 must all leave VPP.
<!-- source: internal/le/test/deployment/vpppppoe/product_framed.go -- checkProductFramed, productFramedRoute, framedTraffic -->
<!-- source: internal/le/test/deployment/vpppppoe/product_state.go -- productOwnedPath -->

This action is authored; a passing run and the counterfactual with the
non-peer gateway kept must be recorded before the behavior is claimed as
proved.

### Actual Ze IPv6 service over VPP

The separate `product-ipv6` acceptance action reuses the product lab's staging,
immutable VPP image selection, pinned pppd 2.5.5 image and Suite cleanup. It
runs the real daemon in plain, named-control-namespace and VLAN topologies:
<!-- source: internal/le/test/deployment/vpppppoe/product_plan.go -- runProductSuite -->
<!-- source: internal/le/test/deployment/vpppppoe/product_ipv6_check.go -- productIPv6Plan -->

```sh
./le --name session test qemu docker-lab lab "test deployment vpp-pppoe product-ipv6" timeout 3600s
```

This source-first scenario requires the configured `ipv6-na` address pool and
RA prefix `2001:db8:20::/64`, independently of the existing
`2001:db8:100::/56` prefix-delegation pool. A small CPE client binds only the
independent pppd's negotiated PPP link. It sends RS and DHCPv6, then checks
the received RA source, hop limit, managed flags and live prefix option.
<!-- source: internal/le/test/deployment/vpppppoe/product_ipv6_peer.go -- productIPv6Peer -->
<!-- source: internal/le/test/deployment/vpppppoe/product_ipv6_wire.go -- productIPv6RA -->

The client also checks the actual DHCPv6 Advertise/Reply bytes. Both IA_NA
and IA_PD must carry the requested IAIDs, matching client/server identities,
transaction IDs and valid lifetimes. Wrong-server Request and truncated
Solicit controls must stay unanswered. Successful acquisition immediately
after those controls proves that the service still works. The helper never
creates Ze's session or native paths.
<!-- source: internal/le/test/deployment/vpppppoe/product_ipv6_peer.go -- productIPv6Peer -->
<!-- source: internal/le/test/deployment/vpppppoe/product_ipv6_wire.go -- productIPv6RA, productIPv6Lease -->

The checker reads the exact `ze-pppoe` source paths for the leased /128 and
delegated /64. Both paths must contain the negotiated link-local next hop
and native session interface. Both addresses carry bidirectional traffic to
an independent host, with native receive/transmit counter increases.
<!-- source: internal/le/test/deployment/vpppppoe/product_ipv6_check.go -- checkProductIPv6, productIPv6TrafficBoth -->
<!-- source: internal/le/test/deployment/vpppppoe/product_ipv6_wire.go -- productIPv6Route -->

A joined bounded capture must see zero IPv4 and global IPv6 subscriber
payloads on the Linux control TAP. The TAP must own no IPv6 address,
including a link-local address. The checker suppresses automatic TAP
address generation before Ze starts. Ze must create no Linux PPP unit in
either namespace.
<!-- source: internal/le/test/deployment/vpppppoe/product_ipv6_check.go -- checkProductIPv6, productIPv6TrafficBoth, checkProductIPv6TAP, startProductIPv6Capture -->
<!-- source: internal/le/test/deployment/vpppppoe/product_ipv6_wire.go -- productIPv6Route -->

Two independently established dual-stack lifetimes then close one NCP each.
An exact-session Terminate-Request must receive its matching Terminate-Ack.
The CPE's tc filter holds only that family's subsequent Configure-Requests.
pppd's `fsm_rtermack` leaves Opened, calls the protocol's down
callback and starts negotiation again. The other family remains usable.
<!-- source: internal/le/test/deployment/vpppppoe/product_ipv6_check.go -- checkProductIPv6 -->

Raw echo probes on the same negotiated MAC/SID must succeed before closure
and receive no response after their family closes. The independent traffic
host's joined captures must first see the exact positive request, then zero
requests after closure. This traffic observation prevents a missing return
route from masking an ingress gate left open. It observes VPP's gate
independently of the CPE kernel's own NCP drop mode.
<!-- source: internal/le/test/deployment/vpppppoe/product_ipv6_check.go -- productFamilyCheck, productFamilyCapture -->
<!-- source: internal/le/test/deployment/vpppppoe/product_ipv6_raw_linux.go -- productFamilyProbe -->

Routes of the closed family must disappear while the same native subscriber
remains and the other family still forwards. Final peer termination must
remove both bindings and the native session before the second lifetime
starts.
<!-- source: internal/le/test/deployment/vpppppoe/product_ipv6_check.go -- checkProductIPv6, productFamilyCheck, productFamilyCapture, productIPv6WaitWithdrawal -->
<!-- source: internal/le/test/deployment/vpppppoe/product_ipv6_raw_linux.go -- productFamilyProbe -->

A reply is accepted only with the exact reverse Ethernet/PPPoE tuple and
complete IP/ICMP envelope. IPv4 version, IHL, TotalLength and header checksum
must agree with the received bytes. IPv6 version and PayloadLength must
also agree. The original addresses, marker, identifier, sequence and ICMP
checksum must match.
<!-- source: internal/le/test/deployment/vpppppoe/product_ipv6_response.go -- productFamilyResponse -->

Only a full healthy receive window establishes response absence. Early
packet-budget exhaustion and native reader failures are unknown
observations and fail the scenario. They cannot establish a successful
negative result.
<!-- source: internal/le/test/deployment/vpppppoe/product_ipv6_response.go -- productFamilyResponse, productFamilyObserve -->

These are acceptance requirements in source. Runtime evidence requires a
passing named action against the integrated production service and the
corresponding behavioral failures under its production semantic cuts.
<!-- source: internal/le/test/deployment/vpppppoe/product_ipv6_register.go -- answerProductIPv6 -->
