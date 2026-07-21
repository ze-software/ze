# RSVP-TE

RSVP-TE (RFC 3209, on top of RFC 2205 RSVP) signals explicitly-routed MPLS LSP
tunnels with bandwidth reservation. It runs directly over IP as protocol 46 and
requires `CAP_NET_RAW`.

> **Status: experimental.** The control plane (signaling, admission, reroute)
> and the dataplane wiring are implemented and unit-tested: the engine emits
> push/swap/pop forwarding entries on the `mpls-fib` event bus and `fib-kernel`
> programs them into the kernel (IP route + label for push, AF_MPLS routes for
> swap/pop). End-to-end and FRR interop validation on a live kernel is still
> pending. Use for evaluation, not production forwarding.

## Configuration

```
rsvp-te {
    router-id 10.0.0.1

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

- `router-id` -- this LSR's address; also the raw-socket source and the SESSION
  extended-tunnel-id.
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
  `node-protection true` asks for a backup around the next node (not just the
  next link); `bandwidth-protection` and `hop-limit` (default 16) tune the
  backup. Presence of the container is what enables protection.
- `bypass` -- a facility-backup bypass LSP from this node (a Point of Local
  Repair) to a `merge-point` along an `explicit-route` that avoids the protected
  resource. ze has no IGP/CSPF, so bypass paths are explicit. `node-protection
  true` marks a bypass that merges at a next-next hop (for node protection).

## How signaling works

1. **Ingress** sends a PATH along the ERO toward the egress.
2. **Transit** nodes consume their own ERO hop, relay the PATH downstream, and on
   the returning RESV allocate a local label, program a swap, and relay the RESV
   upstream.
3. **Egress** allocates a label and returns a RESV.
4. **Ingress** records the downstream label and the LSP comes up.
5. Soft-state is maintained by periodic PATH refresh; `PathTear` removes it.

Admission control reserves the requested bandwidth per interface; an
oversubscribed link is rejected with a `PathErr` (admission-control failure).
Make-before-break reroute signals a replacement LSP (new LSP-ID, SHARED-EXPLICIT
style) and tears the old one down only once the new one is up.

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
4. The PLR sends a `PathErr` "Notify" (code 25, value 3 "Tunnel locally
   repaired") toward the head-end and **keeps the LSP up** (it does not tear it
   down). For node protection it pushes the next-next hop's recorded label (label
   recording is requested automatically).
5. The head-end re-optimizes onto a fresh path (make-before-break) and tears the
   locally-repaired LSP once the replacement is up.

An LSP with no matching bypass keeps the base behavior (tear down + `PathErr` on
a link failure). One-to-one (detour) backup is tracked separately
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
