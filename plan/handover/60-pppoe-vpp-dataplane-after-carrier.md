# Handover 60: PPPoE VPP data plane, after the unit-bound IPv6 carrier

Written 2026-11-22 on the server (`claude`), main checkout. Read this instead
of handover 59: its items 1 to 5 landed and its item 6 is carried below. The
commit tooling keeps handover 59 until the spec closes. Nothing is pushed.

The owning spec is `plan/post-release/features/spec-pppoe-vpp-dataplane.md`
(in-progress, phase 2/7). Claim it before editing:
`./le spec claim spec-pppoe-vpp-dataplane.md`. Its Checkpoint table and
Resume checkpoint carry today's evidence in full.

## Landed on 2026-11-22 (this session)

| Commit | What |
|--------|------|
| `ceca4252b0` | Six never-run ppp/kernel roots reach their assertions (user namespace for `/dev/ppp`, 15-byte link name) |
| `2f045d3c0a` | PPPoE counter fixture forwards `BindNativePPPUnit`; five counter roots pass (judged) |
| `865f67540e` | `./le setup` ppp-access reports the dip grant, not an open it cannot make |
| `f1fee74f36` | Integration-only helpers leave production files (lint) |
| `ad00b94b4e` | Bridge owner-guard roots match the kernel without bridge VLAN filtering (judged); `spec-iface-bridge-vlan-filtering` AC-6 restores the VLAN assertion |
| `5ab4769de0` | Kernel 0013b: owned UDP6 sockets keep the PPP receive receipt through UDP queueing |
| `c63d11d484` | The IPv6 carrier takes its sockets from the original unit; no name lookup, no fallback (judged) |
| `169eccfa2c` | Five IPv6 fragment ownership roots promoted from the draft; draft removed |
| `16dc85122b`, `71ba97df3e` | Spec records the above, the retirement cuts and the carrier cut |
| `567f8c4eb3` | Review fixes: carrier open serialized and its branches tested, MSG_PEEK root, kernel-profiles receipt wording |

Baseline now: `internal/component/l2tp/ppp/kernel` 342/0/0 and
`internal/component/l2tp/pppoe/dataplane/kernel` 47/0/0 in Ze's guest on
runtime tree `2b23af13`
(`~/.cache/ze/runtime-kernel/7.3-runtime-amd64-runtime-qemu-2b23af13-5d7f6651/vmlinuz`).
On the host, ppp/kernel passes except the two UDP6 receive roots, because the
host still boots a kernel without the 0013b fix.

## Remaining, in the spec's order

| # | Work | First action |
|---|------|--------------|
| 1 | Server/client wiring that resume step 2 once gave "the other PPP session": no evidence today covers it | Read step 2 and the spec's Integration Points; find whether PPPoE server and client sessions reach the unit-bound carrier and native release through `Access.Open`, then test it through the real entry point |
| 2 | Accounting and route receipt (resume step 5; the L2TP session owned counter work) | Check `./le spec wip` for a live L2TP claim first; if none, it is this spec's work |
| 3 | Final gates: real VPP forwarding, restart, accounting integration, interop, scale | Spec "Final gates" row |

## Waiting on Thomas

- The push of everything above.
- `./le setup docker kernel install` and a reboot, so the host runs a kernel
  with the 0013b fix and the netdev option. Until then the two UDP6 receive
  roots are red on the host.
- The Docker store on the server: Docker kernel builds need 40 GB free and
  `/home/thomas/.cache` has about 33 GB. Today's kernel was built with the
  QEMU builder (`./ze-host appliance kernel --builder qemu --target runtime --arch amd64 --profile runtime`).

## Traps met on 2026-11-22

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
