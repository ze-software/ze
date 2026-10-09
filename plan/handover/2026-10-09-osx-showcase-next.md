# Next on macOS: the showcase video, then nothing else first

Handover for work not yet started. Written 2026-10-09 on Linux, for the owner's
next session on macOS.

**Owner instruction (2026-10-09): the next session picks these two specs, in
this order, before any other work.**

| Order | Spec | Status | Why this order |
|-------|------|--------|----------------|
| 1 | `plan/immediate/spec-appliance-kernel-vpn-modules.md` | skeleton | the showcase IPsec chapter needs esp4 and xfrm_interface, and the appliance kernel must carry them too |
| 2 | `plan/spec-terminal-demo-showcase.md` | skeleton | the video; it `Depends` on spec 1 |

Each spec holds its own owner decisions, storyboard, work plan and risks. Start
each one with `./le spec claim spec <path>` and run it through `/ze-spec`, then
`/ze-implement`. One spec per session (`.claude/rules/session-start.md`).

## RATIONALE (verify this matches what we agreed)

- The showcase is one long terminal session for the site hero and a 60 to 90 s
  README cut, showing RPKI, IRR, BFD, OSPF, IPsec, VRRP tracking, eBPF traffic
  counting and commit-confirmed, each configured live in the SSH editor and then
  demonstrated. Decisions are in the showcase spec, "Owner decisions".
- A feature that does not enable on a live commit is a Ze defect to fix, never a
  tape workaround.
- WireGuard is NOT a chapter until `plan/spec-wireguard-runtime-proof.md` lands;
  that spec comes after these two. PPP and L2TP are skipped.

## What macOS changes

- The demo renderer runs Linux containers (`--privileged`, kernel modules,
  network namespaces). Rendering on a Mac goes through the Docker VM, whose
  kernel decides whether esp4 and xfrm_interface exist: check that first, it is
  showcase risk R-2.
- A demo daemon that never starts under host load was seen twice on 2026-10-09:
  `plan/journal/startup-wait-expires-under-load.md`. A long session meets it
  more often; showcase risk R-3 says to fix it if it recurs.

## Also waiting on macOS, not part of this order

- `plan/immediate/spec-osx-interface-listing-check.md`: the stdlib interface
  listing (commit `52eb8b4f77`) was only compiled for darwin, never run. The
  owner did not place it in the order above; ask before starting it ahead of
  spec 1.

## Delete this handover

When spec 2 is claimed, this file has done its job: remove it in that session's
first commit.
