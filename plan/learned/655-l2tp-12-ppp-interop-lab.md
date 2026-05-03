# Learned: L2TP PPP/NCP Docker Interop Lab

Spec: `spec-l2tp-12-ppp-interop-lab`

## What Was Built

A peer-isolated Docker lab under `test/l2tp-interop/` that proves full L2TP
PPP/NCP/kernel dataplane evidence with Ze LNS, real xl2tpd/pppd LAC, and
FRR as a BGP peer in separate privileged containers on an isolated bridge.

Two scenarios: `01-ppp-ipv4` (full PPP proof with cleanup verification) and
`02-ppp-bgp-redistribute-frr` (subscriber /32 advertised to FRR via real
RouteObserver path and withdrawn on teardown).

## Key Decisions

- **Separate module from test/interop**: the BGP interop harness has
  domain-specific names, images, and daemon helpers. Copying the proven
  pattern into `test/l2tp-interop/` was cleaner than generalizing.

- **Strict preflight from inside a container**: probing the host kernel from
  the runner's Python environment would miss Docker Desktop VM kernel state.
  Running a temporary privileged Alpine container with modprobe/ip checks
  gives the same kernel view the lab containers will get.

- **Ze env vars match native evidence**: ZE_LOG_L2TP=debug, disabled blob
  storage, disabled IPv6CP, and 15s auth/NCP timeouts. Without these the
  container Ze behaves differently from the native proof.

## Mistakes Avoided

- Did not reuse `scripts/evidence/docker-run.py` (single-container, no peer
  isolation).
- Did not put L2TP scenarios into `test/interop/` (would require awkward
  generalization of BGP-specific code).
- Did not use `ze.l2tp.skip-kernel-probe` (preflight rejects it).

## Reusable Patterns

- Docker preflight probe pattern: run a temporary --rm --privileged container
  to probe host kernel capabilities before starting the real lab.
- The `wait_ze_log` / `wait_l2tp_clean` polling helpers are reusable for
  future L2TP interop scenarios (IPv6CP, CHAP-MD5, multi-session).
