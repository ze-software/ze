# Handoff: continue on Linux (2026-10-12)

Written by the macOS session 5620b26f at HEAD `c79501292b` after 163 commits on
2026-10-12. Start here, then `./le spec current` (this session claimed
`spec-bgp-live-receive-binding-and-bfd-keep-session`; the
Linux session claims it again with `./le spec claim spec <path>`).

Full verify over `c79501292b` (log `tmp/verify-worktree-logs/20261012T181856.805560000Z-p37820-r1-c79501292be4/`):
9 red stages (down from 30 at the start of the day). Fixed after the run, up to
`5ff2edde6c`: lint, compound guards, doc drift and wiring, site facts, plugin 353,
appliance modcache link (`ebfb4931b3`), an editor validation tick (`5ff2edde6c`).
Stopped mid-work at the owner's request, still owed (rerun these stages first):

| Owed | Detail |
|------|--------|
| Unit reds in cmd/ze, cmd/ze/hub, bgp/config, bgp/plugins/filter_path_asn, gr, llnh, cli/testing (.et sessions), l2tp, component/plugin | most are test configs embedded in Go lacking the now-mandatory bgp router-id / local AS (TestGRCapability*, TestRFC4724GRCapability*, TestLinkLocalCapability*, TestOldSyntaxHint, ...). RFC-tagged ones: record `./le rfc approve` with the owner's "3 yes enforce" words. The cached full pass also showed a wave of "build failed" from a Go build-cache fault, not code |
| plugin 743 rpki-decorator-merge | load flake (1 in 20); uncommitted debug printing in `internal/test/fixture/plugin_fixture_14.go` from the stopped agent: keep or finish, journal row open |
| encode, ui, exabgp-test suites | not yet rerun alone; known host reds encode 41, exabgp 17/23/24 (fd00::2), ui 221 (ownership gate) |
| RFC ledger | RFC4659-3.2.1.1-1 and two more SHIFTED (re-judge with `audit-stamp mode rejudge`, not reseal); RFC8955-7.2-3 STALE and its 2 revert records need a Linux kernel (`TestSelectedFlowSpecZeroPacketRateKernel`) |
| Full `./le go lint run` | scoped runs clean |

## 1. Linux-only runs owed (macOS cannot run them)

macOS has no Ze interface backend (`docs/features/interfaces.md`, "macOS / Darwin:
missing"), and a BFD session cannot come Up there (the transport reads no TTL).

| # | What | How | Why owed |
|---|------|-----|----------|
| L1 | `test/reload/bfd-pinned-profile-edit-up-live.ci` green, then its discrimination red | `./le test functional reload` on Linux | keep-session AC-25 live profile edit on an Up session |
| L2 | FRR bfdd interop for a live Poll Sequence (timer change on an Up session) | interop lab, see the spec's Goal Validation | keep-session Phase 4 goal validation |
| L3 | Static-route client variant of `bgp-bfd-shared-session-widens.ci` (spec wiring row 482) | write and run on Linux | macOS version uses BGP + pinned clients instead |
| L4 | `plugin 428 lg-graph-lab` (needs port 53 for the cymru DNS stub, `needs-linux:caps=net-bind`) | `./le test qemu all-tests` | fixed in `543c3884d9`, never run since |
| L5 | `plugin 287 evpn-config-self` (`needs-linux`, `ba8dd10332`) | Linux functional run | never passed on macOS |
| L6 | `test/reload/config-apply-ordering-mixed-rollback.ci` | Linux functional run | swept for mandatory fields, not run |
| L7 | OSPF `TestRFC5187InterfaceIDPreservedAcrossRestart` (now `_linux_test.go`) and `TestOSPFTransportRawSocketCap` | `go test` on Linux | moved/strengthened today |
| L8 | `internal/component/config/archive` `TestArchiveMatcher_InvalidUTF8` Linux branch | `go test` on Linux | macOS runs only the refused-name branch |
| L9 | RSVP-TE functional suite (5 tests, `needs-linux`) | passed 5/5 on the claude host at `19c0f9f026`; rerun at HEAD | regression check |

## 2. Specs waiting

| Spec | State | Next |
|------|-------|------|
| `spec-bgp-live-receive-binding-and-bfd-keep-session` | Phases 1-4 committed; I5 race fix reviewed clean (round 7); Phase 4 reviewed clean (round 3, `tmp/review/...`) | L1-L3, then the RFC5882-4.4-1 summary marker (`rfc/short/rfc5882.md` lacks `{partial}` for the two partly met configurations; then re-judge), then `/ze-close` |
| `plan/immediate/spec-bgp-link-local-only-next-hop.md` (+ readvertise, unowned-rows companions) | design; round 13 fixes committed (`f9ad38e04c`); re-send shape is FRR's in place (owner standing decisions) | review round 14 needs owner authorisation (past the cap), then owner approval |
| `plan/pre-release/spec-plugin-ownership-imports.md` | design; owner answered Q-1 one spec, Q-2 accept the three SR-Policy refusals; design review round 2 fixes committed | one more design review round, then owner approval |
| `plan/spec-bsd-family-interface-backend.md` + six BSD skeletons | design / skeleton, not started | owner answers the five Open Owner Questions; do not start before approval |

## 3. Owner items still open

- Add the IPv6 loopback addresses on the macOS host (encode 41, exabgp-test 17/23/24, plugin 647):
  `sudo ifconfig lo0 inet6 fd00::2/128 alias`, `... fd00::3/127 alias`, `... fe80::1/128 alias`.
- `ze doctor` listener fallback (`collectSchemaListeners` silently falls back to a hard-coded list): removing it deletes AC-6 test `TestDoctorGnmiListenerIsProbed`; needs a ruling (journal `plan/journal/silent-fall-through.md`).

## 4. Journal rows added today, still open

- Overlapping reloads can leave a peer on the first file's BFD setting, no log line (`plan/journal/reload-rolls-back-instead-of-applying.md`).
- A committed BFD plugin disable then re-enable leaves clients without BFD until their next Established (`plan/journal/lifetime-ends-before-its-last-reader.md`).
- A teardown with no target drained after its session has gone clears the queue (`plan/journal/silent-fall-through.md`, N1).
- The mandatory walk skips bgp peer/group entries and data under a choice case (`plan/journal/gate-excludes-part-of-its-population.md`).
- MPLS `dumpMPLSRoutes` returns nil, nil off Linux; raw-socket doctor checks report true where unsupported.

## 5. What landed today (for orientation)

Verification reds from the morning run fixed (lint, arch gates, docs, RFC ledger,
functional suites, Go 1.27.2 + x/net bump, staticcheck from the vendored module);
BFD and routing verdict specs closed; owner decisions applied: EAP refuses TLS
1.2 resumption, YANG `mandatory` enforced (inherited bgp values count, absent
sections not validated, both recorded deviations), five CLI renames to verb-first
forms, second config commit waits its turn, FreeBSD/OpenBSD build compiles and
`./le go vet-platforms` covers openbsd; keep-session I5 race fix and Phase 4
(BFD opt-in hot swap, live timer edits via Poll Sequence, fastest detection,
transactional BFD apply with an SDK `OnConfigCommitted` callback). `./le rfc
check` passes (4147 requirements, 175 RFCs).
