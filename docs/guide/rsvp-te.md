# RSVP-TE

RSVP-TE (RFC 3209, on top of RFC 2205 RSVP) signals explicitly-routed MPLS LSP
tunnels with bandwidth reservation. It runs directly over IP as protocol 46 and
requires `CAP_NET_RAW`.

> **Status: experimental.** The engine emits push/swap/pop entries on the
> `mpls-fib` event bus. `fib-kernel` installs IP routes with labels for push and
> AF_MPLS routes for swap/pop. The native Linux carrier exercises these paths
> with three Ze daemons. Independent-peer coverage is limited to freeRouter
> ingress with Ze egress, as described below. These checks do not establish
> complete RFC conformance. Use for evaluation, not production forwarding.

<!-- source: internal/plugins/rsvpte/producer_integration_linux_test.go -- TestRSVPNativeProducer -->

## Configuration

```
rsvp-te {
    router-id          10.0.0.1
    refresh-period     30
    refresh-multiplier 3

    interface eth0 {
        max-bandwidth            10e9
        max-reservable-bandwidth 8e9
        address                  10.0.0.4/30
    }

    tunnel to-egress {
        destination 10.0.0.9
        tunnel-id   1
        bandwidth   1e9
        explicit-route 1 {
            address 10.0.0.5/32
        }
        explicit-route 2 {
            address 10.0.0.9/32
        }
        fast-reroute {
            backup          facility
            node-protection false
        }
    }

    bypass around-eth0 {
        merge-point 10.0.0.5
        explicit-route 1 {
            address 10.1.0.5/32
        }
    }
}
```

- `router-id` identifies this LSR and sets the SESSION extended-tunnel-id.
  RESV replies use an IPv4 address assigned to the outgoing interface.
  <!-- source: internal/plugins/rsvpte/register.go -- tunnelKey -->
  <!-- source: internal/plugins/rsvpte/transport_linux.go -- rawTransport.Send, rawTransport.outgoingSource -->
- `refresh-period` -- how often ze re-sends PATH and RESV, 1 to 65535 seconds,
  default 30. Ze advertises this value in TIME_VALUES, and a neighbor derives the
  lifetime of the state ze created from it (RFC 2205 Section 3.7).
- `refresh-multiplier` -- how many missed refreshes expire state, 1 to 255,
  default 3. State expires when its last refresh is older than the period
  multiplied by this number, which is 90 seconds at the defaults. Expiry releases
  the reserved bandwidth, the MPLS forwarding entry and the label, then sends an
  `lsp-down` event.
- `interface` -- per-link `max-bandwidth` / `max-reservable-bandwidth` used by
  admission control. `address` (the local link prefix, e.g. `10.0.0.4/30`) lets
  admission map an LSP to this interface when more than one is configured: the
  neighbor's address is matched against each interface's prefix. With a single
  interface `address` is optional. An interface without it is not
  admission-enforced in a multi-interface config (a startup warning lists how
  many).
- `tunnel` -- an ingress (head-end) LSP. Each `explicit-route <index>` names a
  hop `address` (IPv4 prefix) from this node toward the `destination` (the egress
  tunnel endpoint); `type strict|loose` defaults to strict.
- `fast-reroute` (on a tunnel) -- request RFC 4090 local protection. `backup`
  is `facility` (one bypass protects many LSPs, the default) or `one-to-one`;
  `node-protection true` asks for a backup around the next node, so the backup
  avoids the next link and the next router; `bandwidth-protection` and `hop-limit` (default 16) tune the
  backup. Presence of the container is what enables protection.
- `bypass` -- a facility-backup bypass LSP from this node (a Point of Local
  Repair) to a `merge-point` along an `explicit-route` that avoids the protected
  resource. Configure the bypass path explicitly. `node-protection true` marks
  a bypass that merges at a next-next hop (for node protection).
  <!-- source: internal/plugins/rsvpte/register.go -- setupBypass -->

### What a commit changes

A `commit` applies the rsvp-te block to the running daemon. `refresh-period` and
`refresh-multiplier` take effect on the next refresh tick, so the new cadence and
the new expiry deadline start within one old period. An interface you remove stops
being serviced at the commit: it leaves `show rsvp-te interface` and its
reservation records are dropped. The LSPs that reserved bandwidth on that link
stay up, because a withdrawn bandwidth declaration is not a teardown request
(RFC 2205 Section 2.4). An interface removed and added back accounts from zero
until those LSPs drain.

An added tunnel or bypass signals, and removing one tears down its LSPs.
A tunnel removal includes every make-before-break generation, even a replacement
whose reservation is still pending. Other tunnels and retained bypasses stay up.
Changing a tunnel's explicit route reroutes its current generation
make-before-break. A second route change supersedes a pending replacement while
the established LSP continues forwarding until the latest replacement is up.

`router-id` is the one leaf a commit does not change. The engine keeps the
running value and logs a warning, because the LSP keys and the message encoders
are built from it. Restart ze to change it.

<!-- source: internal/plugins/rsvpte/register.go -- reconcileInterfaces, refreshTick, the OnConfigApply hook -->

## How signaling works

1. **Ingress** sends a PATH along the ERO toward the egress.
2. **Transit** nodes consume their own ERO hop, relay the PATH downstream, and on
   the returning RESV allocate a local label, program a swap, and relay the RESV
   upstream.
3. **Egress** allocates a label and returns a RESV.
4. **Ingress** records the downstream label and the LSP comes up.
5. Soft-state is maintained by periodic PATH refresh; `PathTear` removes it.

Admission control reserves the requested bandwidth per interface. If a RESV
exceeds the available bandwidth, Ze sends a `ResvErr` (admission-control failure).

<!-- source: internal/plugins/rsvpte/reservation.go -- acceptReservation -->
<!-- source: internal/plugins/rsvpte/reservation_build.go -- rejectReservation, sendResvError -->

Make-before-break reroute signals a replacement LSP (new LSP-ID, SHARED-EXPLICIT
style) and tears the old one down only once the new one is up.

<!-- source: internal/plugins/rsvpte/reservation.go -- acceptReservation -->

## Fast Reroute (RFC 4090)

Fast Reroute pre-signals a backup so an LSP survives a link or node failure with
local repair instead of waiting for the head-end to re-signal from scratch. ze
implements **facility backup** (one bypass LSP protects every LSP crossing a
resource):

1. A tunnel with `fast-reroute` sends a PATH carrying a `FAST_REROUTE` object and
   a `SESSION_ATTRIBUTE` with "local protection desired" set.
2. A transit node that can protect the next resource (a **Point of Local Repair**)
   selects a configured `bypass` whose `merge-point` is the next hop (link
   protection) or the next-next hop (node protection), arms it, and records
   "local protection available" in its RESV `RECORD_ROUTE`.
3. On a link failure the PLR redirects the protected LSP onto the bypass by
   pushing the bypass label over the protected label (a 2-label stack) and
   forwarding via the bypass next hop -- within the link-down event handling, no
   re-signaling round trip. It records "local protection in use".
4. Where an upstream hop exists, the PLR sends a `PathErr` "Notify" (code 25, value 3 "Tunnel locally
   repaired") toward the head-end and **keeps the LSP up** (it does not tear it
   down). For node protection it pushes the next-next hop's recorded label (label
   recording is requested automatically).
5. The head-end re-optimizes onto a fresh path (make-before-break) and tears the
   locally-repaired LSP once the replacement is up.

Without a usable bypass, Ze withdraws the affected LSP. Transit and egress nodes
send `PathErr` to a known upstream hop; the ingress publishes a local path-error
event. One-to-one (detour) backup is tracked separately
(`spec-mpls-9-rsvp-te-one-to-one-backup`).

## Inspecting LSPs

The RSVP-TE component exposes three introspection commands that report LSP,
interface, and tunnel state as JSON, available both under the top-level `show`
grammar and as direct plugin commands:

- `show rsvp-te lsp` (= `show rsvp-te session`) -- all LSPs: state, role,
  bandwidth, in/out labels.
- `show rsvp-te interface` -- per-interface
  reserved / available / max bandwidth.
- `show rsvp-te tunnel` -- configured tunnels and
  their state.
- `show rsvp-te fast-reroute` -- RFC 4090 protection state: each configured
  bypass LSP and each protected LSP with its armed bypass and whether protection
  is available / in use.

The `show` forms proxy to the plugin commands through the dispatcher.

## Metrics

`ze_rsvpte_lsps_active`, `ze_rsvpte_lsps_total`, `ze_rsvpte_path_sent_total`,
`ze_rsvpte_path_recv_total`, `ze_rsvpte_resv_sent_total`,
`ze_rsvpte_resv_recv_total`, `ze_rsvpte_patherr_recv_total`,
`ze_rsvpte_admission_denied_total`, `ze_rsvpte_local_repairs_total`,
`ze_rsvpte_protected_lsps`, `ze_rsvpte_bypass_lsps`.

<!-- source: internal/plugins/rsvpte/engine.go -- signaling engine -->
<!-- source: internal/plugins/rsvpte/admission.go -- bandwidth admission -->
<!-- source: internal/plugins/rsvpte/reroute.go -- make-before-break -->
<!-- source: internal/plugins/rsvpte/frr.go -- RFC 4090 fast reroute -->
<!-- source: internal/plugins/rsvpte/yang/ze-rsvp-te-conf.yang -- config schema -->

## Native Linux carrier

`TestRSVPNativeProducer` runs three Ze daemons in private network namespaces.
It checks installed forwarding state when an LSP reports Up, then sends UDP
through the negotiated push, swap and pop paths. The same UDP tuple and payload
must reach the receiver and both captured links, with the expected labels.

The carrier also checks rejected forwarding installation, strict and loose
paths, two bypass contexts, link-down repair, RESV_CONFIRM and make-before-break.
A temporary policy rule holds ordinary replacement signaling while the
labelled repair path is observed. Removing that rule lets the daemons complete
the replacement. Withdrawal must remove its transit and egress labels before
soft-state expiry. Removing one bypass must preserve the other.

Use the integration test binary built below with a matching Linux daemon that
includes RSVP-TE, OSPF, SSH, the interface component and `fib-kernel`:

```sh
/root/rsvpte-interop.test \
  -test.run '^TestRSVPNativeProducer$' -test.v -test.timeout 4m \
  -rsvp-producer-daemon /root/ze
```

The guest needs iproute2, network namespaces, raw-packet privileges and an
MPLS-capable kernel. No independent RSVP implementation participates in this
carrier; the next section covers that separate boundary.

## Independent-peer Linux carrier

`TestRSVPFreeRouterInterop` runs a supplied Ze daemon against freeRouter in one
private Linux network namespace. The upstream TAP helper connects Linux Ethernet
to freeRouter's Java forwarding stack over namespace-local UDP. Both programs
run in the existing Linux guest; a second router VM is unnecessary.

The carrier covers freeRouter as ingress and Ze as egress for an IPv4
point-to-point LSP. It requires a captured PATH and matching RESV, then uses
freeRouter's tunnel as the return path for ICMP echo replies. Every reply must
carry the negotiated label and match the request's identifier, sequence and
payload. With Linux MPLS input disabled, labelled replies must still reach the
capture but none may reach `ping`. With input enabled, all three replies must be
delivered. Removing the peer tunnel must produce a matching PathTear, remove
Ze's pop route before soft-state expiry, clear the peer's RSVP state and stop
delivery. Echo requests use ordinary IP in the opposite direction.

This carrier is an executable acceptance check, not a recorded pass. It does
not establish transit, Ze-originated interoperability, fast reroute,
make-before-break, admission/preemption or complete RFC conformance.

### Building the peer artifacts

The source pin is freeRouter commit
[`6c295d8ae79c834ef631d3d21d373c335fb05328`](https://github.com/mc36/freeRtr/tree/6c295d8ae79c834ef631d3d21d373c335fb05328).
`test/interop-rsvpte/Dockerfile.freertr` compiles the upstream Java sources for
Java 21 and builds `misc/iface/tapInt.c` with static musl linkage. The exported
directory includes the source archive and upstream CC BY-SA 4.0 notice; these
must remain with redistributed artifacts.

From the repository root:

```sh
scratch=$(./le session scratch ensure)
CGO_ENABLED=0 ./le --name rsvp-peer job run label rsvp-peer-build command \
  docker build -f test/interop-rsvpte/Dockerfile.freertr \
  --output "type=local,dest=$scratch/rsvp-freertr" test/interop-rsvpte

CGO_ENABLED=0 ./le --name rsvp-peer job run label rsvp-carrier-build command \
  env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go test -c -tags integration \
  -o "$scratch/rsvpte-interop.test" ./internal/plugins/rsvpte
```

Copy `$scratch/rsvpte-interop.test`, `$scratch/rsvp-freertr/peer/`, and the
current Linux Ze daemon into the guest. Ze must include RSVP-TE, `fib-kernel`, the interface
component and their dependencies. The guest requires root with network
namespace, network administration and raw-packet privileges, `/dev/net/tun`,
iproute2, `ping`, `sysctl`, and a Java 21 or newer runtime. Its kernel must
provide TAP and MPLS routing. For the Alpine 3.21 guest:

```sh
apk add --no-cache openjdk21-jre-headless iproute2 iputils-ping
modprobe tun
modprobe mpls_router
modprobe mpls_iptunnel

/root/rsvpte-interop.test \
  -test.run '^TestRSVPFreeRouterInterop$' -test.v -test.timeout 4m \
  -rsvp-freertr-peer-dir /root/rsvp-freertr-peer \
  -rsvp-freertr-ze /root/ze \
  -rsvp-freertr-java /usr/bin/java
```

A built-in kernel feature does not require `modprobe`. The test checks the
supplied revision and artifact checksums, creates unique namespace and temporary
directory names, and bounds and reaps its child processes. Without artifact
flags ordinary integration-test discovery skips it. Once any carrier flag is
supplied, absent files, unsupported kernel features and missing privileges fail
the test.

### Ze-ingress limitation of this peer

The pinned freeRouter
[`packRsvp.parseDatPatReq`](https://github.com/mc36/freeRtr/blob/6c295d8ae79c834ef631d3d21d373c335fb05328/src/org/freertr/pack/packRsvp.java)
requires SESSION_ATTRIBUTE on PATH. Ze's ordinary local tunnel emits that object
only when protection is configured. The peer also requires ADSPEC on PathTear,
which Ze's tear builder omits. Those parser requirements prevent this peer from
serving as an ordinary Ze-ingress setup-and-teardown oracle. The carrier does
not enable protection or alter either implementation to bypass them.

The selected direction uses freeRouter's own `clntMplsTeP2p` and
`ipFwdTab.refreshTrfngDel` to originate PATH and PathTear. Ze's existing egress
path handles these messages and programs the kernel label through `fib-kernel`.

<!-- source: internal/plugins/rsvpte/freertr_interop_integration_linux_test.go -- TestRSVPFreeRouterInterop -->
<!-- source: test/interop-rsvpte/Dockerfile.freertr -- pinned Java and static TAP artifacts -->
