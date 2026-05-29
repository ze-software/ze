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

## Inspecting LSPs

The RSVP-TE component exposes three introspection commands that report LSP,
interface, and tunnel state as JSON, available both under the top-level `show`
grammar and as direct plugin commands:

- `show rsvp-te lsp` (= `rsvp-te show-session`) -- all LSPs: state, role,
  bandwidth, in/out labels.
- `show rsvp-te interface` (= `rsvp-te show-interface`) -- per-interface
  reserved / available / max bandwidth.
- `show rsvp-te tunnel` (= `rsvp-te show-tunnel`) -- configured tunnels and
  their state.

The `show` forms proxy to the plugin commands through the dispatcher.

## Metrics

`ze_rsvpte_lsps_active`, `ze_rsvpte_lsps_total`, `ze_rsvpte_path_sent_total`,
`ze_rsvpte_path_recv_total`, `ze_rsvpte_resv_sent_total`,
`ze_rsvpte_resv_recv_total`, `ze_rsvpte_patherr_recv_total`,
`ze_rsvpte_admission_denied_total`.

<!-- source: internal/component/rsvpte/engine.go -- signaling engine -->
<!-- source: internal/component/rsvpte/admission.go -- bandwidth admission -->
<!-- source: internal/component/rsvpte/reroute.go -- make-before-break -->
<!-- source: internal/component/rsvpte/schema/ze-rsvp-te-conf.yang -- config schema -->
