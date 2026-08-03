# 1034 - as112-3-bgp-integration

## Context

Completes the AS112 feature: BGP watchdog-driven conditional announcement of
the two AS112 covering prefixes (healthy DNS -> announce, unhealthy ->
withdraw), well-known community selection (NO_EXPORT/NOPEER per RFC
7534/RFC 3765), origin-AS override, and a shared-watchdog-group story for
driving multiple peer-groups from one health signal. Zero new production Go
code for as112 itself (deliverable is docs + tests + interop scenarios) --
but the AC-9 healthcheck-probe design surfaced two real, previously-unknown
bugs in shared, cross-cutting infrastructure used by every plugin, not just
as112.

## Decisions

- The healthcheck probe uses the real `ze cli -c "as112 health target <ip>"`
  command over SSH (not a synthetic/mocked health signal), because AC-9's
  whole point is that the probe validates the actual advertised path, not
  just process liveness. This design choice is WHY the SSH exit-code bug
  below was found at all -- a synthetic probe would never have exercised the
  real dispatch chain.
- `ze cli -c "<command>"` over real SSH always exited 0 regardless of the
  dispatched command's actual outcome (`cmd/ze/hub/service_ssh.go`'s
  executor discarded the Response's `Status` field, only mapping a Go
  error to a nonzero exit) -- but many handlers, including as112's own
  `handleAS112Health`, deliberately return `Status:StatusError` with a nil
  Go error (the established `//nolint:nilerr` pattern). This meant a real
  probe using this exact command would ALWAYS report UP regardless of DNS
  health, directly defeating AC-9's entire premise. Fixed by adding
  `responseExecErr` (maps `Status:StatusError` to a real Go error so the SSH
  exec middleware sets the correct exit code) -- a BLOCKER in shared
  infrastructure (`internal/component/ssh/ssh.go`'s exec middleware,
  `cmd/ze/hub/service_ssh.go`) used by every command dispatched via `ze cli
  -c "..."` over SSH, not as112-specific; any existing script relying on
  this exit code to detect failure was silently broken before this fix.
- `parseCommunityText` (the "update text" runtime command grammar, used by
  watchdog route replay among other paths) hardcoded only 3 of ~15
  registered well-known community names (no-export/no-advertise/
  no-export-subconfed) -- a route configured with `community [ nopeer ]`
  (RFC 3765, a real YANG-accepted config value) failed to re-parse when
  replayed through watchdog announce, silently dropping the route instead of
  announcing it with its configured community. Fixed by delegating to
  `attribute.ParseCommunity` (the canonical, complete parser already used at
  config-time) instead of maintaining a second, partial, duplicate table.
- Two flaky tests were fixed by testing the actual invariant instead of an
  implementation detail: `as112-shared-watchdog-group.ci` pinned exact wire
  hex to a fixed `conn=1`/`conn=2` connection number, but the test
  harness's accept loop is strictly sequential and near-instant, making
  which peer becomes conn=1 vs conn=2 genuinely nondeterministic (confirmed
  by two independent agents' repro runs flip-flopping it) -- fixed by
  matching on the shared NO_EXPORT community substring both peer-groups
  configure (`contains=`), which is order-independent and asserts the thing
  that actually matters (both peer-groups announce the community) rather
  than which TCP connection happened to be accepted first.

## Consequences

- `responseExecErr` is now the correct pattern for any future SSH-dispatched
  command that needs its exit code to reflect operational failure, not just
  Go-level dispatch failure -- a probe/healthcheck/monitoring script calling
  `ze cli -c "..."` over SSH can now trust the exit code.
- `attribute.ParseCommunity` is the single source of truth for well-known
  community names across BOTH the config-time YANG parser and the runtime
  "update text" replay grammar -- a future well-known community addition
  only needs one change, not two kept in sync by hand.
- The BGP worked example in `docs/guide/as112.md` was found structurally
  invalid during this spec's own documentation phase (two `session`
  containers under one `peer` block -- `session` is a single container per
  peer, not a list) -- verified via `ze config validate` empirically rather
  than assumed correct from manual review, a good habit for any doc example
  claiming to be valid config.

## Gotchas

- A test rewritten to fix a nondeterminism bug (matching on community
  substring instead of a fixed conn number) must be checked against
  `ai/rules/testing.md`'s "test rewrite as replacement" concern --
  here it does NOT weaken coverage, since exact byte-for-byte NLRI/AS_PATH
  hex for both AS112 covering prefixes is already asserted elsewhere
  (`as112-healthcheck-announce.ci`/`as112-healthcheck-withdraw.ci`); the
  rewritten test's only job is proving BOTH peer-groups react to the shared
  watchdog group, which the community-substring match still proves.
- Setting `ZE_CONFIG_DIR`/`ZE_SSH_PASSWORD` on the daemon's OWN process
  environment (so a healthcheck probe subprocess could inherit them for
  self-referential `ze cli -c` dispatch) also redirects the DAEMON's own
  config-storage backend there, since the daemon itself reads
  `ZE_CONFIG_DIR` too -- causing `ze init` (targeting the same directory) to
  fail with "database already exists" on the next start. Fixed by baking
  the env vars into the probe's `command` string itself (`/bin/sh -c
  "ZE_CONFIG_DIR=... ze cli -c ..."`), so only that one subprocess sees
  them, leaving the daemon's own environment untouched. General lesson: env
  vars set for "just this one subprocess's benefit" can leak into a parent
  process that also reads the same variable name for an unrelated purpose.
- A cross-cutting infrastructure bug (SSH exit-code mapping) can hide behind
  a feature spec that never directly touches the broken file, because the
  bug only manifests when a NEW caller relies on the exit code for
  something the existing callers never needed -- as112's probe was the
  first caller to actually depend on `ze cli -c`'s exit code reflecting
  operational (not just dispatch) success.

## Open / Deferred -- RESOLVED

- **AC-10, AC-11: two advisory doctor checks, originally deferred, were
  subsequently built** (user direction: "2 doctor implement", after this
  entry first recorded the deferral). Both were scoped as OPTIONAL in the
  spec's own Key Design Decisions ("build only if a clean home is found...
  otherwise ship tests + docs and record the deferral") -- primary
  enforcement for both was already in place; this closes the optional
  secondary warning too:
  - `doctor-as112-watchdog-missing-withdraw` -- warns when a BGP `update`
    block announces an AS112 covering prefix without a
    `watchdog{ withdraw true }` marker.
  - `doctor-as112-global-origin-uncoordinated` -- warns when
    `asn.local 112` + `replace-as` (the M5 foot-gun documented in
    [1035](1035-as112-0-umbrella.md)) targets an eBGP session to a
    non-private-use remote ASN (RFC 6996 Section 5).
  - **The "home" decision**, previously the actual blocker (neither the
    as112 plugin nor bgp could own it without violating the spec's own
    no-layering rule -- neither may read the other's config): both checks
    live in `internal/component/doctor` per `ai/rules/repo-maintenance.md`'s
    "dependency with no narrower owner" bucket, reading the whole
    `config.Tree` generically (same pattern as the pre-existing
    `checkConfigReferences`/`checkBGPMD5`), importing neither package.
  - Codes registered in `internal/core/diagnostic/codes.go`; unit tests
    (including RFC 6996 boundary tests) and a full-config-text functional
    test (through `Run()`, the real `ze doctor` entry point) in
    `internal/component/doctor/checks_as112_coordination_test.go`.
  - `docs/guide/as112.md`'s H2/M5 worked-example comments now cross-reference
    the corresponding check by code name.

## Files

- `internal/component/doctor/checks_as112_coordination.go` +
  `checks_as112_coordination_test.go` -- AC-10/AC-11 advisory doctor checks
  (built after initial deferral, see Open/Deferred above)
- `internal/component/doctor/checks_helpers.go` -- `nestedSlice` helper
  (leaf-list counterpart to the pre-existing `nestedValue`)
- `internal/component/doctor/doctor.go` -- wires both checks into `runChecks`
- `internal/core/diagnostic/codes.go` -- registers
  `doctor-as112-watchdog-missing-withdraw`,
  `doctor-as112-global-origin-uncoordinated`
- `docs/guide/as112.md` -- BGP worked example (two peer-groups, watchdog
  groups, community selection, origin-AS override), `allow-from`-vs-firewall
  framing, internal-only requirement (added later, see 1032)
- `cmd/ze/hub/service_ssh.go` + `service_ssh_test.go` -- `responseExecErr`,
  SSH exec exit-code fix (shared infra, not as112-specific)
- `internal/component/bgp/plugins/cmd/update/update_text.go` +
  `update_text_test.go` -- `parseCommunityText` delegates to
  `attribute.ParseCommunity` (full well-known-name table)
- `test/plugin/as112-healthcheck-{announce,withdraw}.ci`,
  `as112-community-choice.ci`, `as112-shared-watchdog-group.ci`,
  `as112-probe-anycast-not-loopback.ci` -- functional proof
- `test/plugin/ssh-cli-status-error-exit-code.ci` -- SSH exit-code regression test
- `test/interop/scenarios/as112-origin-as-frr/`,
  `test/interop/scenarios/as112-community-frr/` -- FRR/BIRD interop,
  AS_PATH-origin-override and community content on the real wire
