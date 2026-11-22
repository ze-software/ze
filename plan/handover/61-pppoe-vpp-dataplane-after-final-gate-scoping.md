# Handover 61: PPPoE VPP data plane, after final-gate scoping

Written 2026-11-22 on the server (`claude`), main checkout. Read this instead
of handover 60: its items 1 and 2 landed, and its item 3 is now scoped into
the steps below. Its traps are carried. Nothing is pushed.

The owning spec is `plan/post-release/features/spec-pppoe-vpp-dataplane.md`
(in-progress, phase 2/7). Claim it before editing:
`./le spec claim spec-pppoe-vpp-dataplane.md`. Its Checkpoint table ("Final
gates"), Resume checkpoint, "Real-VPP product lab — 2026-11-22" and
"Implementation designs — 2026-11-22 (awaiting owner decisions)" carry today's
evidence and both designs in full.

## Landed on 2026-11-22 (this session)

| Commit | What |
|--------|------|
| `2c9fbd7770` | A real PPPoE server session proves the unit-bound IPv6 carrier and the native release (two cuts) |
| `38267f5e0c` | RADIUS accounting counts the original PPP unit, never a name |
| `0a92e78161` | A real session's Framed-IPv6-Route is owned by its original unit (AC-24, one cut) |
| `45abb5326e` | Spec records the server wiring, unit-bound accounting and route receipt; accounting spec amended to count the unit |
| `ef5b6423fd` | Spec test tables name the tests that exist and those owed |
| `b6388ee211` | PADS follows the session's creation, and PADT its deletion (five server tests, two cuts) |
| `c879124c37` | Spec records the IPv4 product lab pass and the PADS/PADT order; PPPoE operator clear row |
| this handover's commit | The `product-framed` pass, both implementation designs, the unpoliced-subscriber row, this handover |

## Baseline now

- Real-VPP IPv4 product lab `./le test deployment vpp-pppoe product`: 4 of 4
  (pppd client, namespace, VLAN, 100 subscribers),
  `job-vpp-product-fc5ac019.log`.
- `./le test deployment vpp-pppoe product-framed`: 1 of 1,
  `job-vpp-framed-52302217.log`.
- The three server native tests (carrier, teardown, framed route) pass in
  Ze's guest on runtime tree `2b23af13`.
- The `authradius` package passes.

Logs are under `tmp/session/2026-11-22-203554/scratch/`.

## Remaining, in order

| # | Work | First action |
|---|------|--------------|
| 1 | Owner decisions D-1 (external-mode VPP API loss with the process alive: A, B or C) and C-1 (where a configured VPP rate lives) | Present both from the spec's design section to Thomas |
| 2 | Implement OD-2 loss handling per the spec design | Needs no decision except D-1's external-mode branch; managed mode can go first |
| 3 | Implement the per-session VPP policer wiring | The Filter-Id/RADIUS path needs no decision; the configured rate waits on C-1. This fixes the open defect: VPP subscribers forward unpoliced today |
| 4 | Lab discrimination (revert, rebuild, red) for `product` and `product-framed` | Spec "Real-VPP product lab" |
| 5 | VPP accounting and idle detection (AC-17) | After `spec-pppoe-subscribers-produce-no-accounting-or-telemetry` |
| 6 | AC-8 (IA_PD) | After `spec-l2tp-ipv6-subscriber` wires the prefix allocator |
| 7 | Scale on a known host | Thomas |

## Waiting on Thomas

- The push of everything above.
- `./le setup docker kernel install` and a reboot, so the host runs a kernel
  with the 0013b fix and the netdev option.
- The Docker store on the server: Docker kernel builds need 40 GB free.
- Decisions D-1 and C-1.

## Traps

Carried from handover 60:

- A guest command resolves its image from the current kernel recipe. To run
  an older kernel, pass both `kernel` and its matching `image` from
  `~/.cache/ze/docker-host/amd64/<key>/docker-host.img`.
- A guest test binary must be built with `CGO_ENABLED=0` (no race): the guest
  has no libc.
- `ppp_open` needs CAP_NET_ADMIN in the opener's user namespace; a test that
  opens `/dev/ppp` must call `userns.Enter` first.
- Scoped lint refuses to plan while any session edits a package the plan
  depends on; rerun when the edits stop.
- A fixture split that moves assertions into a helper trips the weakening
  gate; it needs a ze-test-judge approval even when nothing is lost.

Met on 2026-11-22:

- `auth-method pap` alone still offers CHAP first. For a PAP-only test peer,
  set `authentication-allow [pap]`.
- A PADT cannot prove route withdrawal: deleting the unit flushes its routes.
  Withdraw through an IPv6CP close instead.
- A packet socket bound to a down device returns ENETDOWN on its first read.
- Overlay cuts (`go test -overlay`) avoid touching the shared tree.
- `le test unit pkg` needs the `path` keyword.
- `le test l2tp` needs `-a`.
- Run a net-admin `.ci` through `./le test qemu ci test <path>`.
- `rfc-approval` on `./le commit create` is only for changes already landed;
  pending judge ledgers are picked up automatically.
