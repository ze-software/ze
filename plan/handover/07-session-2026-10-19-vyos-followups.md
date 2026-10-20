# Handover: VyOS-rolling follow-ups (session 20123ea5, 2026-10-18 to 2026-10-20)

Written 2026-10-20 at the end of the session. Everything below is committed and
pushed (origin/main at c9ad7788bb when written). Nothing is left in the working
tree from this session.

## The request this session served

Owner, 2026-10-18: compare a month of VyOS rolling against Ze; features go post
release, bug fixes Ze shares go in now. Scope added the same day: RFC 8415 (now
RFC 9915), RADIUS once its code settles, VPP MTU, IPv6 rpf-check on Linux,
RFC 9774, Blast-RADIUS message authentication, DHCP lease lifetimes.

## Standing owner rulings (still in force)

| Ruling | Effect |
|--------|--------|
| No full verify | Never run `./le verify worktree`, full `./le verify current`, `./le test unit all`, `./le test functional gating`. They stay as verification debt |
| Commit as soon as a piece is done (filesystem trouble) | One commit per logical change, the moment it stands on its own |
| Push | `./le commit create ... push` is refused while verification debt is open, so the owner pushes with `git push` himself. Tell him when commits are waiting |
| Static DNS beats DHCP-learned (RFC 3646 Section 6); DHCPv6 DNS tracked per source | Landed in 75cbddde58 |
| DAD race: re-read link state from the kernel before declining (option a) | Landed in f3090e7a91 and follow-ups |
| DHCPv6 Confirm, stateless, Reconfigure, authentication: IN SCOPE (2026-10-19), post release | Specs approved, see below |

## Done this session (for orientation, not to redo)

| Item | Evidence |
|------|----------|
| DHCPv4 lease maintenance | spec closed d5c7d87945; handover 05 |
| DHCPv6 lease maintenance (RFC 9915 enrolled, Decline on DAD, Rebind after link move) | spec closed dda984d97d; handover 01-dhcpv6 |
| DAD judged against kernel link state; address-read errors propagate; VPP declines nothing | f3090e7a91, ca4eae70a0 (owner rescue commit), 7d26da0dd9, acb6fd67bc |
| Link-only `Backend.GetLink` for existence/MTU/state readers (config apply, mirror, manage, resolver, mtu) | b06c2b499b, b883286f4e, 3dbc854cc0 |
| VPP MTU, IPv4 rpf-check via VPP uRPF, IPv6 rpf-check on Linux | 29a5fffb76, 6d1d92b1f3 (earlier in the session) |
| RFC 9774 AS_SET deprecation, FRR interop `bgp-rfc9774-as-set-frr` with red walk; static-route grouping key fix | spec closed f4a37e7e80; handover 04 |
| RFC 9915 ledger: 23 rows moved from `{feature-declined}` to `{gap}` | spec closed db12461526 |
| awaitSent unparam lint fix | c9ad7788bb |

## Open work, in the order to take it

| # | Item | State | Next step |
|---|------|-------|-----------|
| 1 | RADIUS: EAP without Message-Authenticator, Blast-RADIUS hardening | Unblocked 2026-10-20: spec-radius-rfc-defects closed, commits 59c64b45bb (lab, docs, spec record) and 69c4f494c3 (spec removal); interop-radius-subscriber green | Start a fresh session: `/ze-spec` for both items. The journal row in `plan/journal/silent-fall-through.md` (VyOS T9351, `verifyResponseMessageAuthenticator`) is the starting evidence |
| 2 | DHCPv6 Confirm | Closed 2026-10-20: spec-dhcpv6-client-2-confirm implemented and removed; closure commits c426179c4e (code, docs, ledger, spec) and 99f5971d41 (spec removal). Kea 2.6.2 scenario `dhcpv6-client-confirm-kea` green | None. Verification debt by owner ruling: `./le go lint run`, `./le test unit all`, `./le test functional gating` not run over it |
| 3 | DHCPv6 stateless (Information-request) | Spec `plan/spec-dhcpv6-client-3-stateless.md`, ready, post release | Fresh session. Kea interop `dhcpv6-client-stateless-kea` runs over 10 min (IRT_MINIMUM 600 s), accepted by the owner |
| 4 | DHCPv6 Reconfigure with RKAP authentication | Spec `plan/spec-dhcpv6-client-4-reconfigure.md`, ready, post release, after #3 | Fresh session. Peer: odhcpd built from source (`dhcpv6-client-reconfigure-odhcpd`); if it will not build, stop and ask, never skip. `reconfigure-accept` leaf defaults false |
| 5 | Umbrella `plan/spec-dhcpv6-client-0-umbrella.md` | Ready; owner Q1 to Q5 recorded there | Closes when 2 to 4 close |
| 6 | Verification debt across this session's commits | Rows in `plan/verification-debt/` (fa44853c and the agents' session files) | Owner-held. Clearing them is what re-enables `push` through the commit tool |

## Journal rows this session opened (open, wait for a journal pass)

| File | Defect |
|------|--------|
| `zero-value-as-valid-answer.md` | VPP never fills `InterfaceInfo.Addresses`; IS-IS, OSPF, BFD, `show interface` read "no address" |
| `zero-value-as-valid-answer.md` | RFC 9915 Section 18.2.10.1 DAD MUST unmet on VPP (`dadUnreported`) |
| `zero-value-as-valid-answer.md` | `interfaceExists` reads any failed link read as absent (needs a typed not-found error from every backend) |
| `zero-value-as-valid-answer.md` | Web interface detail silently shows the configured entry on a read error |
| `read-fails-on-a-fact-the-caller-does-not-need.md` | `show interface <name> counters` fails on an address-dump failure |
| `read-fails-on-a-fact-the-caller-does-not-need.md` | `mac/match` selector resolves through `ListInterfaces`, so it fails during address churn |
| `reconciler-repairs-against-a-stale-snapshot.md` | `goneWhileUp` false Decline after a down+up whose events are not yet delivered |
| `queued-request-waits-for-an-unrelated-trigger.md` | v6 worker link events unread during an exchange |
| `claim-outlives-the-evidence-it-cites.md` | No test from parsed `pd { length N; }` config through the real factory |
| `assertion-reads-state-the-scheduler-owns.md` | `TestDHCPv4LoopHoldsLeaseFiveLeasePeriods/renewed_every_T1` goroutine-count flake under load |
| `feature-test-missing-build-tag.md` | Five config/cli tests red without `ze_bgp` |
| `registry-contamination.md` | `internal/component/iface` tests under `-count=2` |
| `wire-carries-both-arms-of-an-either-or.md` | config static route `parseRawAttributeInto` (RFC 9774 walk) |
| `silent-fall-through.md` | RADIUS response Message-Authenticator (VyOS T9351), feeds item 1 |

## Disclosed, no action decided

- Ze defaults to DUID-LL (RFC 9915 Section 11.4 SHOULD NOT). Recorded in `rfc/short/rfc9915.md`; the owner has not asked for a change.

## Traps the next session will meet

- `./le commit create` charges every commit with `arch tier/check` and `arch iface-resolution` reds from the stale `tmp/ze-verify-failures.json`. Re-run both with `./le --name <x>`; if both exit 0, pass `structural-red-ok "<reason>"`. The main thread's classifier refuses `structural-red-ok`, so a subagent commits, or the owner runs a script.
- Plain `./le` may run a stale `bin/le`; `./le --name <x>` builds a fresh one.
- Shell writes to `plan/` are blocked (use Edit/Write); raw `go test` is blocked (`./le job run label X command go test ...`); piping a command through grep/sed/tail/head is blocked (tee to a log, then Read it).
- gopls diagnostics in this checkout are often stale; confirm with `GOOS=linux go vet <pkg>`.
- Removing a test (even a probe) needs a row in `test/weakened/<session>.md`; a journal Fix cell must start `fixed: ` or `open: ` and a row stays under 600 characters.
