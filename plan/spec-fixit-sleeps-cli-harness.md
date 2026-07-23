# spec-fixit-sleeps-cli-harness

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | spec-fixit-migrate-sleeps-infra (harness-gated carve-out) |
| Phase | 0/N (research) |
| Updated | 2026-07-17 |

Update (2026-07-22 plan review; body corrected in-body 2026-07-22): (a) the
sibling this spec recorded as "BLOCKED ... design, NOT approved" --
`spec-fixit-static-interface-nexthops` -- has LANDED (learned 1185, the "make
interface next-hops work" branch), so `005` is unblocked rather than needing
a rewrite; every in-body "005 is blocked" claim is now struck through with a
note (Related Specs, Problem A, AC-3, R-4/R-8, Open Questions, Autonomous
Resolutions, Future); (b) the sleep ratchet baseline is now **125**
(`test/.ci-sleep-baseline`, composable-delta format), not the 132/132->129
originally cited; every in-body occurrence is annotated with the current
value (forward-looking arithmetic: 3 conversions take 125 -> 122).

## Task

Two buckets of `.ci` sleeps are blocked on missing test harness or support, not on
the sleep itself. In both, the sleep is a symptom: the test cannot reach the state it
wants to assert, so a blind hold papers over the gap. **Problem A:** `test/static/004-show.ci`
and `test/static/005-table-interface.ci` query `ze cli -c "show static"`, but `ze cli`
is SSH-only, and neither test's config declares an SSH server or a user, so there is
nothing for the client to connect to. **Problem B:** three `test/vpp` tests are reported
to fail for reasons deeper than their sleeps. The goal: give these tests the harness they
need (the SSH show-CLI pattern already proven by `test/firewall/004-cli-show.ci`; the vpp
blockers resolved), then convert their sleeps and ratchet `test/.ci-sleep-baseline` down.

Group rationale: both buckets are "the sleep is not the problem, the missing harness is".
Neither is QEMU-gated (both suites are host-runnable), which is what separates this spec
from `spec-fixit-sleeps-qemu-bulk`.

> **Scope correction (verified 2026-07-15).** The brief said "3 sleeps each" for the
> static tests and implied all three vpp tests have convertible sleeps. Reading the files:
> the raw counts are right, but only **3 blind sleeps exist across all five files**
> (`static/004:22`, `static/005:24`, `vpp/007:51`). Every other sleep in them is an
> already-deterministic bounded poll annotated "bounded wait not a blind sleep", which must
> NOT be converted. `vpp/005` and `vpp/006` have **no blind sleep at all**. See Problem / Evidence.

## Origin

Carved out of `plan/spec-fixit-migrate-sleeps-infra.md` (the umbrella), which records
that the remaining sleeps were each left for a per-test reason that only surfaces on
attempting the conversion. Skeleton written 2026-07-15 alongside `spec-fixit-sleeps-qemu-bulk`.

## Required Reading

<!-- NEVER tick [ ] to [x]. Capture insights as → Decision: / → Constraint: annotations. -->

### Source (read before designing)
- [ ] `internal/component/cli/client/main.go` - the `ze cli` entry path: loads `sshclient.Credentials` (:274-280), builds the SSH client (:295), and dispatches every command through it.
  → Constraint: there is NO non-SSH transport. `SendCommand` (:385-387) calls `sshclient.ExecCommand`; `StreamMonitor` (:431) calls `sshclient.StreamCommand`. The struct doc (:334) states it plainly: "cliClient handles communication with the daemon via SSH exec".
- [ ] `internal/core/ssh/client/client.go` - `ExecCommand` (:66), the producer: builds an `ssh.ClientConfig` (:71-78) and calls `ssh.Dial("tcp", addr, config)` (:82).
  → Constraint: this is the ground truth for the SSH-only claim. A real SSH server must be listening or `ze cli` returns "connect to <addr>: ..." and exits non-zero.
- [ ] `internal/component/cli/client/main.go` - `runOfflineFallback` (:242-253): the ONLY escape from SSH, consulted when credentials fail to load (:286) or the connection fails (:308).
  → Constraint: the fallback is registry-driven and nearly empty. Only `show host` (`internal/plugins/host/register.go:19`) and `show crashes` (`internal/plugins/crashes/register.go:19`) register one. `show static` does NOT, so it MUST traverse SSH. An offline fallback is therefore an alternative to consider, not a path that already exists.
- [ ] `test/firewall/004-cli-show.ci` - the working SSH show-CLI reference pattern (confirmed).
  → Decision: this is the template to copy for Problem A. Its header (:9-15) documents the recipe; see Current Behavior for the full parts list.
- [ ] `test/plugin/ssh-user-login-yang.ci` - the auth setup `firewall/004` says it mirrors (:9).
- [ ] `feature-gates.txt:31` (`ze_ssh internal/component/ssh`) and `Makefile:51,121` (`ZE_FEATURES`, `bin/ze` build).
  → Constraint: `ze_ssh` is a DEFAULT-ON feature gate, so the host `bin/ze` already has the SSH server compiled in. The build tag is not a blocker natively; it only matters for stripped/cross-compiled binaries (see `mk/test-integration.mk:336-345`).
- [ ] `internal/test/cli/register.go:33` (`registerCIRoot("static", ...)`) and `:47` (`registerRoot("vpp", cmdVpp, ...)`).
  → Constraint: both suites are host-runnable and neither appears in `scripts/evidence/qemu-all-tests.sh`, so QEMU never runs them. This spec's work is verifiable natively, unlike its sibling.
- [ ] `test/vpp/007-fib-route-lookup.ci:51` - the only blind sleep in the vpp bucket: `time.sleep(2.0)` with "Give the VPP backend a moment to acquire its channel", right after `api.wait_for_post_startup(timeout=10.0)` (:48).
- [ ] `scripts/dev/verify_wiring_docs.py:196` (`check_ci_sleep_ratchet`), `:258` (`check_ci_sleep_justification`), and `test/.ci-sleep-baseline` (cited `132`; now `125`, composable-delta sum, 2026-07-22).
  → Constraint: the baseline is tight (the tree contained exactly 132 when written). Converting the 3 blind sleeps here takes it down by 3, in the same change (132 -> 129 as cited; from the current 125 that is 125 -> 122, 2026-07-22).

### Related Specs
- [ ] `plan/spec-fixit-static-interface-nexthops.md` - the sibling skeleton owning `005`'s interface-only next-hop dependency.
  → Constraint: ~~`005` is blocked on its outcome.~~ (2026-07-22: the sibling LANDED as "make them work" -- learned 1185; `005` is UNBLOCKED and needs only the harness transplant.) Its A-4 asked whether interface-only next-hops should be supported at all or rejected at config-verify; it resolved as "supported", so `005`'s config is valid.

### Architecture Docs
- [ ] `docs/architecture/testing/ci-format.md` - the `.ci` directive catalog; where an SSH show-CLI harness pattern would be documented for reuse.
- [ ] `ai/rules/ci-sleep-justification.md` - every sleep in a CHANGED `.ci` needs a justifying comment; kept sleeps must keep theirs.
- [ ] `ai/rules/no-workarounds-for-missing-behavior.md` - directly on point: the missing harness must be built, not worked around by keeping the blind sleep or weakening the assertion.

## Current Behavior (MANDATORY)

**Source files (cite file:line):**
- [ ] `internal/component/cli/client/main.go` - loads SSH credentials (:274-280) with no alternative transport; `cliClient` doc says "communication with the daemon via SSH exec" (:334); `SendCommand` calls `sshclient.ExecCommand` (:386); `StreamMonitor` calls `sshclient.StreamCommand` (:431); `runOfflineFallback` (:249) is the only non-SSH path.
- [ ] `internal/core/ssh/client/client.go` - `ExecCommand` (:66) dials a real SSH server: `ssh.Dial("tcp", addr, config)` (:82), password auth (:74), host-key callback (:67).
- [ ] `internal/plugins/host/register.go` - `registry.MustRegisterOfflineFallback("show host", RunShow)` (:19); `internal/plugins/crashes/register.go:19` registers `show crashes`. These two are the complete offline set; no `show static` entry exists.
- [ ] `internal/test/cli/register.go` - `registerCIRoot("static", ...)` (:33); `registerRoot("vpp", cmdVpp, ...)` (:47).
- [ ] `scripts/dev/verify_wiring_docs.py` - `check_ci_sleep_ratchet` (:196) against `test/.ci-sleep-baseline`; `check_ci_sleep_justification` (:258) scoped to changed `.ci` files (:268).
- [ ] `test/static/004-show.ci` - driver polls for `daemon.pid`/`daemon.ready` (:10-16, bounded), then a blind `time.sleep(0.5)` (:22) annotated "blind settle: give the config a moment before the retrying CLI query below; no post-apply readiness signal to poll in this standalone driver", then retries `ze cli -c "show static"` up to 50x (:25-34, bounded) and exits "ze cli never became ready" if all fail (:35-36). Config (:53-67) declares ONLY a `static { }` block.
- [ ] `test/static/005-table-interface.ci` - the same shape: bounded poll (:12-18), blind settle (:24), bounded CLI retry (:27-36). Config (:52-73) declares `routing-table { table lns { id 100 } }` and `static { }` with an interface-only next-hop `next { interface tun100 { } }` (:70).
- [ ] `test/firewall/004-cli-show.ci` - the reference: `option=skip-os:value=darwin` (:23); provisions client creds with `ze init` into a temp `ze.config.dir` (:52-63); sets `ze.config.dir`/`ze.ssh.password`/`ze.ssh.insecure` (:65-68); runs `ze cli --user operator -c "show firewall ruleset fw10_004"` (:75); config declares `system { authentication { user operator { password "<bcrypt>" } } }` (:120-126) and `environment { ssh { enabled true; server main { ip 127.0.0.1; port 2222 } } }` (:128-136); a fixed port is safe because the per-test netns isolates it (:12-13); a drop-policy chain needs an explicit `allow-loopback` term or it drops the SSH return SYN-ACK (:102-111).
- [ ] `test/vpp/007-fib-route-lookup.ci` - `time.sleep(2.0)` (:51) after `api.wait_for_post_startup(timeout=10.0)` (:48); header cites `plan/spec-vpp-fib-query.md` (:1) and describes dispatching `show route lookup 10.20.0.1` through a `lookup-test` external process plugin (:117-124).
- [ ] `test/vpp/005-mpls-push.ci` (:95, :121, :192) and `test/vpp/006-iface-create.ci` (:33, :118) - every sleep is annotated "bounded wait not a blind sleep": socket-path appearance, stub-log polling, ze-peer listening line, backend-loaded log line.

**Behavior to preserve:**
- Every test keeps its exact `expect=` assertions. `static/004` asserts exit 0 plus stdout containing `"prefix"`, `"weight"`, `0.0.0.0/0`, `192.0.2.0/24`, `blackhole` (:74-79). `static/005` asserts exit 0, stderr `static routes loaded` / `routing-table registry loaded`, stdout `10.0.0.0/8`, `0.0.0.0/0`, `"table"` (:81-86). The point of these tests is the structured JSON shape of `show static`; the harness must let them assert MORE, never less.
- The bounded polls in all five files stay unchanged: they are already deterministic.
- `test/firewall/004-cli-show.ci` keeps working; it is the reference and must not regress.
- No production behavior change unless research proves one is needed (an offline fallback for `show static` would be a real production surface, and needs its own justification).

**Behavior to change:**
- Give `static/004` and `static/005` a working SSH show-CLI harness so `ze cli -c "show static"` can actually connect.
- Resolve the vpp blockers, then convert `vpp/007:51`.
- Lower the baseline by the number of blind sleeps removed.
- Exact recipe: None yet, research first.

## Problem / Evidence

### Problem A: `ze cli` is SSH-only and the static tests provide no SSH (CONFIRMED)

`ze cli` has exactly one transport. `main.go:274-280` loads `sshclient.Credentials`;
every command path funnels into `sshclient.ExecCommand` (`main.go:386`) or
`sshclient.StreamCommand` (`main.go:431`); the producer `internal/core/ssh/client/client.go:66-82`
ends in `ssh.Dial("tcp", addr, config)`. The struct doc (`main.go:334`) says so directly.

The single escape hatch, `runOfflineFallback` (`main.go:249`), is registry-driven and has
exactly two entries tree-wide: `show host` (`internal/plugins/host/register.go:19`) and
`show crashes` (`internal/plugins/crashes/register.go:19`). `show static` is not among them,
so `ze cli -c "show static"` MUST reach a live SSH server.

Neither static test provides one. Grepping `test/static/004-show.ci` and
`test/static/005-table-interface.ci` for `environment`, `ssh`, `authentication`, or `system {`
returns **zero matches**: their configs declare only `static` (and `routing-table` in 005).
So the missing parts, measured against the `test/firewall/004-cli-show.ci` reference, are:
an SSH server block, a user with a password hash, client credential provisioning via `ze init`
into a sandbox `ze.config.dir`, and the `--user` / `ze.ssh.password` / `ze.ssh.insecure` wiring.

The `ze_ssh` build tag is NOT a blocker for the native path: `feature-gates.txt:31` lists it as
a default-on gate, and `bin/ze` builds with `$(ZE_FEATURES)` (`Makefile:51,121`), so the host
binary has the SSH server compiled in. It only matters for cross-compiled or stripped binaries;
`mk/test-integration.mk:336-345` documents adding `ze_ssh` to a QEMU build specifically so
`004-cli-show` works there.

**Sleep accounting (CONFIRMED, corrects the brief's "3 sleeps each"):** each file has 3 raw
`time.sleep(` but only ONE blind sleep. `004:14` and `004:34` (and `005:16`, `005:36`) are
bounded polls annotated "bounded wait not a blind sleep". The convertible sleeps are
`004:22` and `005:24`, both "blind settle" holds before the CLI query. Converting both
lowers the baseline by 2.

**Interface next-hops (CONFIRMED):** `005:70` does use an interface-only next-hop
(`next { interface tun100 { } }`), so the dependency is real, and it is tracked by the
sibling skeleton `plan/spec-fixit-static-interface-nexthops.md` (Status: skeleton,
Depends: - at the time; that spec has since landed and closed, learned 1185, 2026-07-22). That spec's Task states the interface-only next-hop fails to program on BOTH
data-plane backends, and it reads `test/static/005-table-interface.ci:53-73` (its :78-80) as
the config that surfaced the problem. Its A-4 even questions whether interface-only next-hops
should be supported at all rather than rejected at config-verify.

→ Constraint: ~~005 is blocked on that spec's outcome, and the outcome is not yet decided.~~
(2026-07-22: the sibling LANDED -- learned 1185 -- resolving as "make interface-only
next-hops work", NOT config-verify rejection. 005's config is VALID and 005 is UNBLOCKED;
it needs the SSH harness transplant only.) ~~If it
resolves as "reject at config-verify with a clear message", 005's config is INVALID and the
test needs rewriting, not just a harness. Do not assume 005 only needs SSH.~~

(Timing note: that file did not exist when this spec's citations were first checked and was
created minutes later by a concurrent session. Anything asserted here about sibling specs is
a snapshot; re-check before relying on it.)

### Problem B: the vpp bucket (blockers UNVERIFIED, sleep claims CORRECTED)

**CORRECTED (by reading the files):** `test/vpp/005-mpls-push.ci` (3 sleeps at :95, :121, :192)
and `test/vpp/006-iface-create.ci` (2 sleeps at :33, :118) contain **no blind sleeps at all**.
Every one is a bounded poll annotated "bounded wait not a blind sleep" (waiting for the stub
socket path, the stub's JSONL log line, the ze-peer listening line, the backend-loaded log
line). By this spec's own rule they must NOT be converted. So even if their blockers are fixed,
there is no sleep in them to convert, and they contribute **0** to the ratchet.

The only blind sleep in the bucket is `test/vpp/007-fib-route-lookup.ci:51`: `time.sleep(2.0)`
with "Give the VPP backend a moment to acquire its channel", sitting immediately after
`api.wait_for_post_startup(timeout=10.0)` (:48). That is the sole conversion target here.

**UNVERIFIED (reported in the brief as observed symptoms; NOT reproduced here, no tests were
run, and none of these strings appear in the files):**
- `005-mpls-push.ci` fails with ze-peer "no test data".
- `006-iface-create.ci` needs a real kernel interface.
- `007-fib-route-lookup.ci` fails with "lookup-test: no peers match selector".

If these symptoms are real, they are test-harness bugs worth fixing on their own merit, but
only 007 carries a sleep, so only 007 affects the ratchet. Whether 005 and 006 belong in a
sleep-migration spec at all is an open question: fixing them is valuable, but it is not
sleep-migration work.

→ AUTONOMOUS DEFAULT (2026-07-17): the sleep-migration spec IS warranted (per project
practice, migrating blind sleeps to real synchronization exposes real races), but `vpp/005`
and `vpp/006` carry NO blind sleep, so they are RE-SCOPED OUT of this spec into a separate
vpp-harness spec (AC-6's re-scope path). This spec keeps only `vpp/007` from the vpp bucket.
Full rationale under "### Autonomous Resolutions (2026-07-17)" in Open Questions. Thomas:
override if wrong.

**Also noted (drifted citation, pre-existing):** `test/vpp/007-fib-route-lookup.ci:1` cites
`plan/spec-vpp-fib-query.md` as its Design reference, and that file does not exist.

### Ratchet impact (CONFIRMED)

Total blind sleeps in scope: **3** (`static/004:22`, `static/005:24`, `vpp/007:51`).
`test/.ci-sleep-baseline` was `132` when written and the tree contained exactly 132, so full
success here was cited as taking it to **129** (baseline now **125**, 2026-07-22, so full
success takes it 125 -> **122**). The harness work is large; the ratchet credit is small. That asymmetry
is the point of the group rationale, and it should be stated plainly rather than discovered
mid-implementation.

### The question that gates everything (UNVERIFIED)

Do `static/004` and `static/005` pass today? They are ungated (no `option=skip-os`, no
`option=needs-linux`), so `bin/ze-test static` runs them natively on darwin. Yet by the
analysis above, `ze cli -c "show static"` cannot connect to a daemon with no SSH server, the
50x retry loop would exhaust, and the driver would exit "ze cli never became ready" (`004:35-36`).
That predicts a red test. No quarantine, skip, or xfail marker was found for either file, and
the `static` suite is not in `scripts/evidence/qemu-all-tests.sh`, so QEMU never runs it either.
Either these tests are currently failing, or `ze cli` is succeeding through a path this analysis
has not traced. Research must run them first and find out, because the answer changes the shape
of the spec.

## Data Flow (MANDATORY)

### Entry Point
- A `.ci` driver process invokes `ze cli --user <u> -c "show static"` as a subprocess, expecting structured JSON on stdout.

### Transformation Path
1. `ze cli` loads SSH credentials from the sandbox `ze.config.dir` provisioned by `ze init` (`main.go:274-280`).
2. `sshclient.ExecCommand` dials the daemon's SSH server (`internal/core/ssh/client/client.go:82`) and authenticates with the password from `ze.ssh.password`.
3. The daemon's SSH CLI server dispatches `show static` to the registered command handler.
4. The handler returns JSON; `ze cli` prints it (`printFormatted`, `main.go:390`).
5. The driver parses the JSON, asserts its shape, and writes it to stdout for the runner's `expect=` directives.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| driver ↔ `ze cli` | subprocess with `ze.config.dir` / `ze.ssh.password` / `ze.ssh.insecure` env | [ ] |
| `ze cli` ↔ daemon | SSH exec over TCP (`ssh.Dial`, `client.go:82`); needs a listening server + a valid user | [ ] |
| daemon ↔ static plugin | the registered `show static` command handler | [ ] |
| driver ↔ vpp stub | Unix socket + JSONL log (bounded polls, already deterministic) | [ ] |

### Integration Points
- `test/static/004-show.ci`, `test/static/005-table-interface.ci` (the harness rewrite).
- `test/vpp/007-fib-route-lookup.ci` (blocker + sleep).
- `test/.ci-sleep-baseline` (ratchet).
- `docs/architecture/testing/ci-format.md` if the SSH show-CLI pattern is documented for reuse.
- No production integration point unless an offline fallback for `show static` is chosen over the SSH harness.

### Architectural Verification
- [ ] No bypassed layers (the test exercises the real `ze cli` -> SSH -> handler path, as `firewall/004` does).
- [ ] No unintended coupling (test-side harness only; no production change without its own justification).
- [ ] No duplicated functionality (copy the proven `firewall/004` recipe rather than inventing a second one).
- [ ] Registration over hardcoding: if an offline fallback is added, it registers via `registry.RegisterOfflineFallback` from the static plugin, exactly as `show host` and `show crashes` do; no `show static` spelling in a core or shared package.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| `ze cli -c "show static"` over SSH | -> | SSH server + user config + `ze init` provisioning in the test | `test/static/004-show.ci` |
| `ze cli -c "show static"` with a named table + interface next-hop | -> | the same harness, plus interface next-hop support | `test/static/005-table-interface.ci` |
| driver waits for the CLI to answer | -> | deterministic wait replacing the blind settle | `test/static/004-show.ci`, `test/static/005-table-interface.ci` |
| `show route lookup` through the vpp backend | -> | vpp channel-acquired signal replacing the blind 2.0s hold | `test/vpp/007-fib-route-lookup.ci` |
| SSH show-CLI harness pattern is discoverable | -> | `docs/architecture/testing/ci-format.md` | `test/firewall/004-cli-show.ci` (the reference it documents) |
| a sleep is removed from any `.ci` | -> | `check_ci_sleep_ratchet` (`scripts/dev/verify_wiring_docs.py:196`) | `test/.ci-sleep-baseline` lowered to 129 (from the current 125: to 122, 2026-07-22); `make ze-verify-changed` green |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `bin/ze-test static` run before any change | The current pass/fail state of `004-show.ci` and `005-table-interface.ci` is recorded. If red, this is a test-repair spec first and a sleep spec second |
| AC-2 | `test/static/004-show.ci` after the harness lands | Declares an SSH server and a user, provisions client credentials via `ze init` into a sandbox `ze.config.dir`, and `ze cli -c "show static"` returns the JSON its existing `expect=` directives assert (exit 0, `"prefix"`, `"weight"`, `0.0.0.0/0`, `192.0.2.0/24`, `blackhole`) |
| AC-3 | `test/static/005-table-interface.ci` after the harness lands | Same harness; its existing assertions (`10.0.0.0/8`, `0.0.0.0/0`, `"table"`, the two stderr lines) pass~~, OR the interface-next-hop dependency is confirmed blocking and 005 is deferred against `plan/spec-fixit-static-interface-nexthops.md` with the blocking relationship recorded in both specs~~ (2026-07-22: the sibling LANDED, learned 1185; the deferral branch is retired -- 005 is unblocked) |
| AC-4 | The blind settle at `004:22` and `005:24` | Replaced by a deterministic wait on a real post-apply signal. The bounded CLI retry loop is not a substitute: research must decide whether it already suffices once the CLI can connect, in which case the settle is simply deleted |
| AC-5 | `test/vpp/007-fib-route-lookup.ci` | The "lookup-test: no peers match selector" blocker is root-caused and fixed, then the blind `time.sleep(2.0)` (:51) is replaced by a wait on the vpp backend's channel-acquired state |
| AC-6 | `test/vpp/005-mpls-push.ci`, `test/vpp/006-iface-create.ci` | Their blockers are either fixed or explicitly re-scoped out. Neither contains a blind sleep, so neither changes the ratchet; no bounded poll in them is converted |
| AC-7 | Any commit that removes sleeps | `test/.ci-sleep-baseline` lowered by exactly the number removed, same change (132 -> 129 as cited; from the current 125 baseline: 125 -> 122 if all three land, 2026-07-22) |
| AC-8 | Every sleep left in a touched `.ci` file | Still carries a justifying comment; `check_ci_sleep_justification` green on the changed files |
| AC-9 | The SSH show-CLI harness | Documented so the next test needing a show-CLI assertion finds the pattern instead of rediscovering it (`ai/rules/discovery-updates.md`) |
| AC-10 | Full suite after the changes | `bin/ze-test static` and `bin/ze-test vpp` green; `test/firewall/004-cli-show.ci` (the reference) still green |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | `static/004` and `005` are currently RED | `ze cli` needs SSH (`client.go:82`), their configs declare none, `show static` has no offline fallback | if green, the SSH-only analysis is missing a path and the whole premise needs rework before any change | run `bin/ze-test static` | ~~unvalidated~~ CONFIRMED RED (2026-07-17, sibling A-5a) |
| A-2 | The `firewall/004` recipe transplants to `test/static` | it is the proven working reference for exactly this shape | the harness needs per-suite work (netns, ports, caps) | apply it to 004 first, run it | unvalidated |
| A-3 | The fixed port 2222 is safe because a per-test netns isolates it | `firewall/004:12-13` says so explicitly | the static suite needs a dynamic port or a log-scrape to find one | check whether the static suite runs under netns like firewall does | ~~unvalidated~~ RESOLVED (2026-07-17): no netns in default `ze-static-test`; use distinct ports |
| A-4 | `static` tests need no `option=skip-os` | they are ungated today and SSH-to-loopback is not linux-specific; `ze_ssh` is default-on (`feature-gates.txt:31`) | adding SSH may force a gate, and the `static` suite is absent from `qemu-all-tests.sh`, so a gated static test would run NOWHERE | run the converted test on darwin | ~~unvalidated~~ RESOLVED (2026-07-17): `needs-linux` required by the backend, not by SSH |
| A-5 | The vpp backend exposes (or can expose) a channel-acquired signal | the 2.0s hold is described as waiting for exactly that | AC-5 needs a new signal, or 007 stays sleep-bound and is deferred | read the vpp backend's connect path | unvalidated |
| A-6 | An offline fallback for `show static` is NOT the intended fix | the fallback set is deliberately tiny (2 read-only host-local commands) and would assert the CLI's own view, not the daemon's applied state | a much smaller change exists and the SSH harness is wasted effort | ask; check the offline-fallback design intent | ~~unvalidated~~ CONFIRMED (2026-07-17): SSH harness, not fallback |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Large harness effort for 3 sleeps of ratchet credit | noticing mid-implementation that 132 -> 129 (now 125 -> 122, 2026-07-22) is the entire numeric payoff | stated up front in Problem / Evidence; the real value is the tests actually testing `show static`, not the ratchet. Judge the spec on that |
| R-2 | The static tests are red today and the "conversion" silently becomes a repair | AC-1 comes back red | AC-1 is the first action; re-scope openly rather than folding a repair into a migration |
| R-3 | An SSH-enabled static test locks itself out or hangs (the `firewall/004` loopback trap, :102-111) | the CLI times out with no error | copy the `allow-loopback` reasoning; static declares no firewall, so the trap likely does not apply, but confirm rather than assume |
| R-4 | ~~`005`'s interface-next-hop dependency blocks it regardless of the harness~~ (RETIRED 2026-07-22: the sibling landed, learned 1185; the dependency is satisfied) | ~~005 red while 004 green after the same change~~ | ~~split: land 004 alone, defer 005 against `plan/spec-fixit-static-interface-nexthops.md`~~ |
| R-8 | ~~`spec-fixit-static-interface-nexthops` resolves as "reject interface-only next-hops at config-verify" (its A-4)~~ (RETIRED 2026-07-22: it landed as "make them work", learned 1185; 005's config is valid) | ~~that spec's design decision lands against the feature~~ | ~~005's config becomes invalid and the test needs rewriting, not a harness; watch that spec before investing in 005~~ |
| R-5 | Fixing the vpp blockers balloons into vpp feature work | 005/006 need a kernel interface or test-data plumbing | AC-6 allows explicit re-scope; neither has a sleep, so neither is load-bearing for this spec |
| R-6 | Converting a bounded poll for ratchet credit | the diff touches sleeps whose comments say "bounded wait" | AC-6/AC-8; only 3 sleeps in this spec are legitimate targets |
| R-7 | Touching a `.ci` makes the session own every sleep in it | the justification gate fails on neighbours | expected; justify or convert them in the same change |

## 🧪 TDD Test Plan

Migration-adapted: each `.ci` IS its own functional test and keeps its exact assertions.
Unit tests apply only where research adds Go-side support.

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| offline-fallback registry lookup (only if A-6 flips and `show static` gains a fallback) | `internal/component/command/registry/registry_test.go` | a registered fallback is found by `LookupOfflineFallback`; an unregistered command is not | pending |
| vpp channel-acquired signal (only if A-5 needs a new signal) | `internal/component/vpp/` (file per research) | the backend reports channel-acquired; bounded, no unbounded wait | pending |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Notes |
|-------|-------|-------|
| any new wait timeout | bounded, > 0 | times out naming the awaited signal |
| `test/.ci-sleep-baseline` | non-negative integer, monotonically decreasing | 132 -> 129 as cited (now 125 -> 122 if all three conversions land, 2026-07-22) |
| SSH port in the test config | fixed 2222 per the reference, or dynamic | depends on A-3 (netns isolation) |

### Functional Tests
| Test | Location | Scenario | Status |
|------|----------|----------|--------|
| `004-show.ci` | `test/static/` | `show static` JSON over the real `ze cli` -> SSH path; blind settle removed | pending |
| `005-table-interface.ci` | `test/static/` | named tables + interface next-hop over the same path | pending (may block on R-4) |
| `007-fib-route-lookup.ci` | `test/vpp/` | `show route lookup` via the vpp stub; blocker fixed, 2.0s hold converted | pending |
| `005-mpls-push.ci`, `006-iface-create.ci` | `test/vpp/` | blockers fixed; no sleep change (no blind sleep exists) | pending (AC-6 may re-scope out) |
| `004-cli-show.ci` | `test/firewall/` | the SSH reference must not regress | regression guard |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Notes |
|----------|-------|
| N/A | test harness work; no wire-protocol behavior changes |

### Future (if deferring any tests)
- ~~`005-table-interface.ci` may defer behind the interface-next-hop work if R-4 materializes; it needs a real spec file to defer against.~~ (RETIRED 2026-07-22: the interface-next-hop work landed, learned 1185; no deferral path remains.)
- The vpp 005/006 blockers may split into a vpp-harness spec, since neither carries a sleep.

## Files to Modify

- `docs/architecture/testing/ci-format.md` - document the SSH show-CLI harness pattern (server + user + `ze init` provisioning + env wiring) so it is discoverable per `ai/rules/discovery-updates.md` (AC-9).
- `ai/rules/testing.md` - point at the pattern from the testing rules if the doc alone is not discoverable enough (candidate, pending research).
- `test/static/004-show.ci` - SSH harness + blind settle at :22 converted.
- `test/static/005-table-interface.ci` - SSH harness + blind settle at :24 converted (may block on interface next-hops).
- `test/vpp/007-fib-route-lookup.ci` - blocker fixed + blind sleep at :51 converted; the stale `plan/spec-vpp-fib-query.md` Design ref (:1) corrected while in the file.
- `test/vpp/005-mpls-push.ci`, `test/vpp/006-iface-create.ci` - blockers only; no sleep change.
- `test/.ci-sleep-baseline` - lowered by the number of sleeps removed (132 -> 129 as cited; now 125 -> 122 if all land, 2026-07-22).
- Production files: NONE expected. An offline fallback for `show static` (A-6) or a vpp channel-acquired signal (A-5) would each add one, and each needs its own justification before being written.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| Test infra docs | yes (AC-9) | `docs/architecture/testing/ci-format.md` |
| Discovery updates | yes (a reusable harness pattern) | `ai/INDEX.md` per `ai/rules/discovery-updates.md` |
| Ratchet | yes | `test/.ci-sleep-baseline` |
| QEMU verification | no | neither `static` nor `vpp` appears in `scripts/evidence/qemu-all-tests.sh`; both are host-runnable |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File |
|---|----------|----------|------|
| 8 | Plugin SDK/test-support changed? | yes (a documented harness pattern) | `ai/rules/testing.md`, `docs/functional-tests.md` |
| 10 | Test infrastructure changed? | yes | `docs/architecture/testing/ci-format.md` |

## Files to Create
- None planned. ~~The interface-next-hop dependency is already tracked by the sibling skeleton `plan/spec-fixit-static-interface-nexthops.md`; this spec cross-references it rather than duplicating it.~~ (2026-07-22: that sibling landed and its spec file is closed -- learned 1185; the dependency is satisfied, nothing to track.)

## Implementation Steps

### /implement Stage Mapping
| Stage | Section |
|-------|---------|
| Audit | AC-1 (run the tests first), then re-verify this spec's citations against the tree |
| Implement | the phases below, Problem A before Problem B |
| Verify | `bin/ze-test static`, `bin/ze-test vpp`, `bin/ze-test firewall` (reference regression) |
| Close | ratchet lowered; two-commit closure |

### Implementation Phases
1. **Establish ground truth (AC-1).** Run `bin/ze-test static` and `bin/ze-test vpp`. Record pass/fail per test. If the static tests are red, decide openly whether this stays one spec or splits into repair + migration.
2. **Resolve A-6.** Confirm the SSH harness (not an offline fallback) is the intended direction before building it. The offline fallback is far cheaper but asserts the wrong thing; get that decided, not assumed.
3. **Transplant the reference recipe to `static/004` (AC-2).** Copy `firewall/004`'s server + user + `ze init` + env wiring. Run it. Resolve A-3 (fixed vs dynamic port) from what actually happens.
4. **Convert `004:22` (AC-4).** With a connecting CLI, decide whether the bounded retry loop already makes the settle redundant (delete it) or a real post-apply signal is needed (wait on it).
5. **Apply to `static/005` (AC-3).** If the interface next-hop blocks it, find or create the real tracking spec (R-4) and split.
6. **vpp 007 (AC-5).** Root-cause "no peers match selector", fix, then convert `:51`.
7. **vpp 005/006 (AC-6).** Fix the blockers or re-scope them out explicitly. No sleep work here.
8. **Document the pattern (AC-9), lower the ratchet (AC-7), full suite green (AC-10).**

### Critical Review Checklist (/implement stage 6)
| Check | For this spec |
|-------|---------------|
| Assertions preserved | every test keeps its `expect=` checks; the harness lets them assert more, never less |
| No workaround for missing behavior | the harness is built, not papered over by keeping the sleep or weakening the assertion (`ai/rules/no-workarounds-for-missing-behavior.md`) |
| Non-vacuous | the converted test proves `show static` returned real applied state, not that the CLI merely exited 0 |
| Bounded polls untouched | the 7 bounded polls across the five files are unchanged (AC-6) |
| Reference intact | `test/firewall/004-cli-show.ci` still green |
| Registration over hardcoding | any offline fallback registers from the static plugin like `show host`/`show crashes`; no plugin spelling in a shared package |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification |
|-------------|--------------|
| static tests actually exercise `show static` | `bin/ze-test static` green; the assertion is on real daemon output |
| vpp 007 blocker fixed + sleep converted | `bin/ze-test vpp` green |
| Ratchet lowered | `cat test/.ci-sleep-baseline` shows 129 (baseline now 125, so 122 if all three land, 2026-07-22; or the count actually achieved); `scripts/dev/verify_wiring_docs.py` green |
| Harness documented | the pattern is findable from `ai/INDEX.md` / `docs/architecture/testing/ci-format.md` |
| No regressions | `bin/ze-test firewall` green |

### Security Review Checklist (/implement stage 11)
| Check | Notes |
|-------|-------|
| Credential handling | test-only bcrypt hash + `ze.ssh.password` in a sandbox `ze.config.dir`, matching the `firewall/004` precedent; never a real credential |
| `ze.ssh.insecure` | test-only host-key bypass to loopback; must not leak into any production path or doc example |
| Input validation | any new predicate bounded (timeout/attempts) |
| Resource exhaustion | no unbounded wait loop |

### Failure Routing
| Failure | Route To |
|---------|----------|
| AC-1 shows the static tests green | the SSH-only analysis missed a path; STOP and re-derive before changing anything |
| A-6 flips (offline fallback is intended) | drop the SSH harness for Problem A; rescope to a registered `show static` fallback |
| A-2 false (recipe does not transplant) | investigate the per-suite difference (netns, caps, ports) before hand-rolling a second recipe |
| ~~005 blocked on interface next-hops~~ (RETIRED 2026-07-22: dependency landed, learned 1185) | ~~find or create the real tracking spec; land 004 alone~~ |
| A-5 false (no vpp channel signal) | defer 007's sleep with the missing signal named; do not fake the wait |
| 3 fix attempts fail | mark DEFER in `plan/deferrals.md`, move on, report |

## Open Questions (research before design)

- Do `static/004` and `static/005` pass today (A-1)? The analysis predicts red; no quarantine marker exists; the `static` suite never runs under QEMU. This gates the spec's shape and must be answered first.
- Is the SSH harness the intended fix, or should `show static` register an offline fallback like `show host` and `show crashes` (A-6)? The fallback is far cheaper but asserts the CLI's own view rather than the daemon's applied state, which is probably not what these tests are for. This is a design decision, not a research detail.
- Does the `firewall/004` fixed-port-2222 approach depend on netns isolation the `static` suite does not have (A-3)? If so, dynamic port or log-scrape.
- Does adding SSH force an `option=skip-os` on the static tests (A-4)? If it does, note that the `static` suite is absent from `scripts/evidence/qemu-all-tests.sh`, so a gated static test would run in NO environment. That would need fixing too.
- ~~Will `plan/spec-fixit-static-interface-nexthops.md` resolve as "make interface-only next-hops work" or as "reject them at config-verify" (its A-4)? The second makes `005`'s config invalid and turns 005 into a rewrite, not a harness transplant. Answer this before investing in 005 (R-8).~~ (ANSWERED 2026-07-22: it landed as "make them work" -- learned 1185; 005's config is valid and 005 is unblocked.)
- Are the reported vpp blockers real and current ("no test data", "needs a real kernel interface", "no peers match selector")? None appear in the files; they were not reproduced here.
- Given that `vpp/005` and `vpp/006` contain no blind sleeps, do they belong in a sleep-migration spec at all, or should they move to a vpp-harness spec (AC-6)?
- Should `test/vpp/007-fib-route-lookup.ci:1`'s dead Design reference to `plan/spec-vpp-fib-query.md` be fixed here, and does `check_design_refs` (`scripts/dev/verify_wiring_docs.py`) already flag it?

### Autonomous Resolutions (2026-07-17)

Readiness pass, no user available: every Open Question above is resolved here with the
conservative default, grounded in source. Append-only; Thomas overrides any line that is wrong.

→ **A-1 — Do `004`/`005` pass today? RESOLVED: NO, both are RED.** Grounded in sibling
`plan/spec-fixit-static-interface-nexthops.md` A-5a, which ran `make ze-static-test`
2026-07-16 and observed `005` dying on darwin at the platform guard ("static routes: not
supported on this platform (Linux required)" x3, `backend_other.go` `unsupportedStaticBackend`),
`OnConfigure` failing, and the daemon aborting startup before `iface.Resolve`, `tun100`, or any
`ze cli` call is reached. A-5b records the suite is release-evidence-only
(`mk/test-functional.mk:20,49`, `mk/test-release.mk:71`), which is why both `004` and `005`
rotted unnoticed. Independently, Problem A holds on linux: `ze cli` is SSH-only
(`internal/core/ssh/client/client.go:83 ssh.Dial`) and neither config declares a server, so the
50x retry exhausts and the driver exits "ze cli never became ready". Conclusion: RED on both
platforms, different producers. This spec is therefore a test-REPAIR spec first and a
sleep-migration spec second (R-2 realized). Not re-run here per the readiness instruction; the
sibling ran it 2026-07-16.

→ **A-6 — SSH harness or offline fallback for `show static`? RESOLVED: SSH harness.**
Test-infra scope question, so the smaller self-contained option wins: the SSH harness is
test-only (copies `firewall/004`), whereas an offline fallback adds a NEW production surface (a
`registry.RegisterOfflineFallback` for `show static`) that needs its own justification. It also
asserts the wrong thing — the CLI's own view, not the daemon's applied route state, which is the
whole point of these tests. The offline set is deliberately tiny (two read-only host-local
commands, `internal/plugins/host/register.go:19`, `internal/plugins/crashes/register.go:19`).
A-6 confirmed: fallback is NOT the fix.

→ **A-3 — Does fixed port 2222 depend on netns the static suite lacks? RESOLVED: yes, and the
default static run has NO netns.** `ZE_TEST_NETNS` is armed only in `mk/test-integration.mk:132`
and `scripts/evidence/netns_qemu.py:107`; `netnsModeActive()` is off by default
(`internal/test/runner/runner_exec_util.go:29-33`), and netns is a global run-mode env var, not
a per-`.ci` directive. The normal `ze-static-test` target (`mk/test-functional.mk:157-158`,
`bin/ze-test static --all`) does NOT set it. `firewall/004`'s fixed 2222 survives the default
host-netns run only because it is the SOLE firewall test binding an SSH server. Two SSH-binding
static tests (`004`, `005`) sharing a hardcoded 2222 would collide if the runner schedules them
concurrently. Default: give `004` and `005` DISTINCT fixed ports (do NOT copy 2222 into both),
or run the static suite under `ZE_TEST_NETNS`. Provisional — revisit if the runner serializes
SSH tests.

→ **A-4 — Does adding SSH force a platform gate? RESOLVED: the tests need `option=needs-linux`,
but because the static BACKEND is linux-only, not because of SSH.**
`internal/plugins/static/backend_other.go` (`//go:build !linux`) returns "static routes: not
supported on this platform (Linux required)" for every method, so on darwin the daemon aborts
before any `show static`. Sibling C-6 already prescribes `option=needs-linux` for `005`; the same
reasoning applies to `004`. SSH-to-loopback is not platform-specific and `ze_ssh` is default-on
(`feature-gates.txt:31`), so SSH adds NO new gate. The "gated static test runs NOWHERE" hazard is
real (static is release-evidence-only and absent from `scripts/evidence/qemu-all-tests.sh`), so
the implementer must wire the linux-gated static suite into an actually-run linux path
(discovery/wiring, AC-9), NOT drop the gate. A-4's original "no `skip-os` needed" is superseded:
a `needs-linux` gate IS needed.

→ **Sibling next-hop decision — make interface-only next-hops work, or reject at config-verify?
RESOLVED: MAKE THEM WORK.** Thomas decided 2026-07-16 (sibling lines 471-540): config-validate
the interface reference where possible AND handle runtime resolution failure gracefully (WIDEN
`WantsConfig` to `["static","interface"]`, C-8; actionable runtime error + doctor check,
D-2 (a)+(b)). It did NOT resolve as config-verify rejection — A-4a confirms the interface-only
form is deliberate design intent (`internal/plugins/static/yang/ze-static-conf.yang:121-131`).
So `005`'s config is VALID; `005` needs a harness transplant plus the sibling's linux fixes
(C-1 ordering, C-6 `needs-linux` + `interface { backend netlink }` + create `tun100`), NOT a
rewrite (R-8 retires on the "reject" branch). ~~CAVEAT: the sibling is still `design`, NOT approved
(its line 30 — promotion is Thomas's gate). So `005` stays BLOCKED on the sibling landing. Per
R-4: land `004` alone; defer `005` behind the sibling with the block recorded in both specs.~~
(SUPERSEDED 2026-07-22: the sibling LANDED -- learned 1185,
`plan/spec-fixit-static-interface-nexthops.md` closed. `005` is UNBLOCKED; land it with the
harness transplant, no deferral needed.) This
spec does NOT re-open the next-hop question — the sibling owned it and settled it.

→ **vpp blockers real and current? RESOLVED: UNVERIFIED, reproduce first.** None of "no test
data" / "needs a real kernel interface" / "no peers match selector" appear in the files; none
were reproduced. Default: AC-1's "run the suite and record actual failures" is the FIRST vpp
action; do not design against an unreproduced symptom. Only `vpp/007:51` carries a blind sleep,
so only 007's blocker is load-bearing for the ratchet.

→ **vpp/005, vpp/006 in a sleep-migration spec? RESOLVED: the spec IS warranted; 005/006 are
RE-SCOPED OUT.** Migrating blind sleeps to real synchronization is valuable (it exposes real
races), so the overall spec proceeds. But `vpp/005` (`:95,121,192`) and `vpp/006` (`:33,118`)
contain NO blind sleep — every one is a bounded poll annotated "bounded wait not a blind sleep" —
so they add 0 to the ratchet and are NOT sleep-migration work. Smaller/self-contained default:
move their blocker fixes to a separate vpp-harness spec (AC-6's re-scope path); THIS spec keeps
only `vpp/007` from the vpp bucket. No bounded poll is converted for ratchet credit (R-6). Net
scope: exactly 3 blind sleeps (`static/004:22`, `static/005:24`, `vpp/007:51`); baseline
132 -> 129 as written (now 125 -> 122, 2026-07-22).

→ **vpp/007:1 dead Design ref — fix here? does `check_design_refs` flag it? RESOLVED: NOT
gate-flagged; fix opportunistically.** `check_doc_links`/`check_design_refs` matches only
`// Design:` (Go comment) in `.go` non-test files (`scripts/dev/check_doc_links.py:104`, `:183`);
the vpp/007 ref is `# Design: plan/spec-vpp-fib-query.md` (Python comment in a `.ci`), invisible
to the gate. `plan/spec-vpp-fib-query.md` does not exist (confirmed). Since `007` is edited anyway
for the sleep conversion, repoint or drop the dead ref then — cheap hygiene, not a scope driver.

## Checklist

### Goal Gates
- [ ] Tests written -- each `.ci` in scope IS its own functional test and keeps its exact assertions; Go-side support (if any) gets red-first unit tests.
- [ ] Tests FAIL -- the pre-change state is RECORDED (AC-1), not assumed; any new unit test is red-first; a converted `.ci` that fails surfaces a real problem, fixed at the source, never with a re-added sleep.
- [ ] Tests PASS -- `bin/ze-test static` and `bin/ze-test vpp` green; `test/firewall/004-cli-show.ci` still green.
- [ ] make ze-test / `bin/ze-test <suite> --all` -- affected suites green before each commit.

### Quality Gates
- [ ] `test/.ci-sleep-baseline` lowered by exactly the number of sleeps removed, same change.
- [ ] `make ze-verify-changed` green (ratchet + justification gates).
- [ ] `make ze-lint-changed` green.
- [ ] Every sleep left in a touched file still carries a justifying comment.
- [ ] No bounded poll converted for ratchet credit.
- [ ] The SSH show-CLI harness pattern is documented and discoverable.

## Review Gate
### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
