# Handover: in-progress spec triage, 2026-10-09 (resume on macOS)

The Linux session `12d06ccf` triaged the 34 in-progress specs and closed or
reset what it could. Per-spec state files under `tmp/session/2026-10-09-12d06ccf-*/state/`
do NOT travel with git: everything needed to resume is here or in the specs.

## Owner decisions still open

| Decision | Context |
|----------|---------|
| WIP cap | 19 in progress vs cap 12; `./le spec claim` refuses `spec-appliance-ships-ze-kernel`. Raise (`ZE_SPEC_WIP_CAP=20`) or park one (stalest unowned: `spec-verify-scope-5-suite-coverage-map`, `spec-ipsec-rfc9190`) |
| Kernel route | `spec-appliance-ships-ze-kernel` (ready): option (b) `ze appliance build` builds ze's kernel via the cache-or-build resolver, assumed; (a) a published pinned kernel module instead. Also: is a ~30 min cold first build acceptable, GPLv2 source-offer sign-off, N100 hardware boot |
| feature-maturity design | Owner direction given (not yet written into the spec): each page section declares what it relies on (tests by content hash, YANG resolved definition per schema path, CLI commands); the commit that changes a dependency re-stamps or fixes the citing sections; known-false sentence still refuses; feature Doc review derived. Rewrite `spec-feature-maturity-declared` around this before code |

## Closed today (10)

ci-parser-refuses-an-assertion-key, bmp-statistics-timeout, pppoe-lcp-option-reject,
pppoe-padt-ends-session, ppp-pap-reanswer-after-auth, pppoe-client-pap-retry,
rsvpte-frr-backup-path-signaling, bgp-local-as-options, bgp-as-migration,
announce-grammar-stated-and-enforced (`be3b08dd73`, `9946d11d38`).
The owner wanted the coverage-gate blind spot as its own spec; it exists only as a journal row so far.
Reset: six unstarted specs to ready/design (`cb4276f8c7`); yang-loader-structural-checks to design (`1ed7d59966`).

## Open, with the next step

| Spec | State | Next |
|------|-------|------|
| `spec-appliance-ships-ze-kernel` | ready, owner decision in Task (`e1280ff02d`): ze's kernel ONLY, rtr7 and `ze.gok.kernel-package` deleted | claim (needs WIP cap), implement; two cached 7.2 amd64 builds exist on the Linux host only |
| `spec-kernel-capability-gate` | AC-1..14 evidenced (`76a6b87730` real MPLSInUse bug fixed, `90f567bdb2`, `21ab1b1666`); AC-15 depends on the kernel spec | after the kernel switch: MPLS seed boot lab, review, close |
| `spec-crash-capture` | Depends on the kernel spec: rtr7 has no CONFIG_PSTORE, so its QEMU labs cannot pass | rerun labs after the switch |
| `spec-announce-build-key-is-the-builder-argument-set` | scenario sends the route by command (`daea9ec553`), but the forced red (prepend zeroed in the key, HEAD export) still PASSES on the current scenario: `local-as-replace-as-partition` does not discriminate. A pass means FRR and BIRD never share a build group (the builder takes the first peer's facts for the group); suspected differing field: extended message or 4-byte AS (unproven). Green last died on a Docker timeout | log each peer's announce key in an export to find the differing field, align the two sessions on everything but the prepend, rerun red then green; the 3 "NOT YET OBSERVED" cells stay empty until a real red |
| `spec-interop-image-copies-a-prebuilt-ze` | AC-1..6, 8 evidenced (`ec67061d71`); doc paragraph landed inside `3d43920131`. AC-9 IPsec run 3: 16 pass / 21 fail, 18 of them a Docker removal cascade at load 40-60; `delete-while-window-held`, `mobike-initiator`, `mobike-responder` timed out before the peer was ready (delete-while-window-held also in run 1). AC-10 `verify current mode full` was left running on the Linux host (its lint already flags errcheck/errorlint in HEAD files outside this spec) | AC-7 `bgp-wellknown-noexport-frr` re-discrimination (172.30.0.0/24 must be free); rerun the three IPsec scenarios alone at low load before calling them environment; rerun AC-10; record the doc paragraph; /ze-close |
| `spec-fixit-flap-test-cannot-build-its-own-stimulus` | already handed to the macOS showcase handover (`20a4e455be`); a stopped agent's half fixture edit was undone, saved as `backups/flap-fixture-20261009-195228.patch` on the Linux host | read `netlinkDrops08` every round, force the drops red, close |
| `spec-feature-maturity-declared` | blocked on the design rewrite above | rewrite spec, then the verify stage |
| `spec-fixit-plugin-concurrency-is-pinned-to-a-ci-constant` | measurement only, gated on the BGP session landing and a quiet box | owner-gated |
| PADR replay lab (`pppoe-padr-replay`) | fix landed `3d43920131`: `SendFrameInNamespace` deleted (uid 1000 cannot open a root container's netns, setns needs CAP_SYS_ADMIN), replaced by `interoplab.SendFrameInContainer` (tcpreplay via docker exec) for every caller incl. the IS-IS purge. Not yet green: the one run that got through failed "per-MAC cap did not hold: 0 sessions" (guess, unverified: the checker's `pkill -x pppd` kills the first session's pppd) | diagnose that, run `pppoe-padr-replay` green, forced red in an export, rerun `pppoe-pap-ze-ac` and `isis-purge-reorig-frr`, scoped lint of `internal/le/interoplab` |

## Hazards found today

- Another session's uncommitted refactor stubs command-argument validation in `internal/component/plugin/server` (`ensure.go`, `command.go`, `handler.go`) and `cmd/ze/ze_core_dispatch.go`: it must not be committed in that state.
- Forced-red breaks belong in a `git archive HEAD` export, never the shared tree; run a broken build with `ZE_BIN` plus `LE_TEST_NO_BUILD=1`, else the runner rebuilds over it (journaled).
- Plain `go test` without the gate's tags (`ze_core` plus `feature-gates.txt`) shows false reds in bgp/config and doctor.
- `ze.test.doctor.procfs-root` keeps its hyphen; the underscore env spelling is silently ignored.
- Full-tree verify is owed over every commit of this session (`plan/verification-debt/68b8aac3.md`).
