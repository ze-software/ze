# Handover: RADIUS admin auth, subscriber accounting, and two fixit specs

Written 2026-09-04, when five agents were killed mid-flight by an account rate
limit (resets 03:00 Europe/London). Nothing here failed on its merits. The tree
compiles: `go vet ./internal/le/interoplab/radius/...` is clean, and the
uncommitted work below is intact.

Read this before restarting any of the five. Each one stopped at a named point,
and restarting from scratch would repeat work that is already on disk.

## What landed

| Commit | What |
|--------|------|
| `c54d97dcdb` | `internal/component/ike/eap` moved to `internal/core/eap`, 114 files, `./le tier check` clean. Unblocks RADIUS using the EAP peer |
| `0971cff50d` | RADIUS admin CHAP: `auth-method` enum leaf, `pap` default, CHAP branch, 4 discrimination records |
| `3182090331` | RFC 3579 enrolled, 50 requirement ids, 31 MUST-level, all `{gap}` until EAP lands |
| `f38bb0e2da`, `547910de0d`, `1262ea2cae` | Subscriber accounting: Event-Timestamp, Calling-Station-Id, Acct-Terminate-Cause |
| `ead2e374eb` | Loc-RIB emulated peer takes its identity from config, not the OPEN cache |
| `63808baa71`, `c7d44e8823` | `spec-fixit-rfc-drain-quota-never-armed` closed |
| `d7dcd382e` | Removed a resurrected copy of an already-closed spec |
| `64464097c0`, `f0abc7cfb5` | `spec-radius-subscriber-attributes` closed, plus a NAS-Port-Id resolution-length fix |

## Where each killed agent stopped

### 1. FreeRADIUS interop suite (`plan/spec-radius-admin-interop-freeradius.md`, Phase 1/5)

**Its last words: "Now wiring the suite into `./le integration`."** That is the
next step, and it is the only step between what exists and a runnable suite.

On disk, untracked and compiling:

```
internal/le/interoplab/radius/{radius.go,checkers.go,radius_test.go}
test/interop-radius/{Dockerfile.ze,clients.conf,site-default,mods-ze-request-log}
test/interop-radius/scenarios/radius-admin-pap-freeradius/{users,ze.conf}
test/interop-radius/scenarios/radius-admin-chap-freeradius/{users,ze.conf}
test/interop-radius/scenarios/radius-admin-chap-hashed-freeradius/{users,ze.conf}
```

All three scenarios of the spec exist. `Discover`
(`internal/le/interoplab/discover.go`) refuses a scenario directory the suite's
registry does not name, so the wiring is what makes these run rather than break
the run. AC-9's forced REDs and AC-10's CI job are not done.

### 2. Tunnel teardown emits no session-down (no spec, sibling of `1262ea2cae`)

**Its last words: "Now I write the test first, against the current (unfixed)
code, to observe the red."** So no fix is on disk. `internal/component/l2tp/teardown_cause_test.go` is modified.

The defect, verified at the producer: `(*l2tpReactor).teardownTunnelByID`
(`internal/component/l2tp/teardown.go`) collects the live session IDs into
`torn` and calls `routeObserver.OnSessionDown` for each, but never
`emitSessionDown`. So a tunnel teardown publishes no `(l2tp, session-down)`, and
every subscriber on that tunnel gets no Accounting-Stop, no pool release and no
shaper cleanup. An LAC disconnecting is the ordinary case. `teardownSessionByID`
and `teardownSessionOnTunnel` had the same defect and were fixed in `1262ea2cae`;
this is the half that remains. Row in
`plan/journal/guard-added-to-one-half-of-a-pair.md`.

The one non-obvious part: `emitSessionDown` needs a username, and `torn` holds
session IDs only. The username must be collected in the same pass while
`t.sessions` is populated and `tunnelsMu` is held, because `teardownStopCCN`
clears the map straight after. An empty username for every session is the
failure mode and it looks like a working fix. Also check
`TeardownAllTunnels`, `TeardownAllSessions` and `drainPendingKernelTeardowns`
for the same question.

### 3. Directional tunnel traffic proof (`plan/spec-fixit-tunnel-traffic-proof-is-one-directional.md`, Phase 7/7)

**Its last words: "Lint has queued ~15 minutes behind other sessions. Running it
scoped to my two packages."** So the work is done and unlinted, not unwritten.
Nine files uncommitted under `internal/le/interoplab/ipsec/` and
`test/interop-ipsec/`. Check what it produced for AC-9, the report of which
scenarios were passing on one direction only, before assuming anything.

### 4. Loc-RIB RFC 9069 (`plan/spec-fixit-locrib-peer-fields-contradict-rfc9069.md`, Phase 1/1)

**Its last words: "Now the mandatory discrimination walk for the scenario."**
`ead2e374eb` landed the headline fix. `internal/component/bgp/plugins/bmp/bmp_locrib.go` is modified beyond it.

### 5. Subscriber accounting phases 4-5 (`plan/spec-radius-acct-session-attributes.md`, Phase 3/7)

Not killed mid-step; it reported cleanly. Phases 4 and 5 are Acct-Delay-Time and
the client change, and they were deliberately not started because
`internal/component/radius/` was occupied.

**`buildAcctPacket`'s signature is FROZEN.** Adding a sixth parameter edits 19
RFC-tagged tests, and `./le commit create` refuses that without an owner row in
`test/rfc-changed.md`, which an author may not write for themselves. The previous
agent hit this, reverted, and put the cause on `acctSession` instead. Do the same
for anything phases 4-5 need to carry.

## Open decisions and unproven claims

**Nothing is waiting on Thomas.** Every question put to him this session was
answered: CHAP as a config leaf defaulting to PAP; EAP built with the peer moved
to `internal/core/eap` and MD5-Challenge plus MSCHAPv2 as its methods; all four
accounting attributes emitted unconditionally copying Juniper; a Juniper-shaped
exclusion knob with a curated enum and no numeric form; the RFC-tagged test split
approved; the FreeRADIUS interop spec approved to run.

**One claim is reasoned, not measured**, and it decides whether a fix is
config-side or fixture-side. `test/interop-l2tp/scenarios/04-radius-acct-attrs/ze.conf`
was changed from `auth-method none` to `chap-md5` in `297b790446` by another
session, because `buildAuthAttrs` returns `nil, false` for `AuthMethodNone` since
`6bc7b6063b`, so no Access-Request is sent and the checker waits forever. The
verified half is that `EvaluateProxyLCP` returns `errProxyLCPMissing` when any of
the three AVPs is empty, so the leaf is load-bearing when they are absent. The
UNVERIFIED half is that xl2tpd sends no proxy LCP AVPs.

Settle it with one grep on a Linux host: ze logs `ppp: proxy LCP short-circuit`
in `session_run.go` only when the proxy path is taken. Absent means the leaf is
load-bearing and the change is right. Present means read its `auth-proto` field,
where 0 means the AVPs arrived without an Auth-Protocol option and the method is
`AuthMethodNone` whatever the config says. `authMethodFromAuthProto`
(`internal/component/l2tp/ppp/auth.go`) also returns `AuthMethodNone` for CHAP
with an empty algorithm byte, which is a second way to read a false negative.
The log-level caveat that applies to `.ci` runs does NOT apply here: `zePeer`
sets `ZE_LOG_L2TP=debug` on every ze peer in the interop lab.

## Reds that belong to other sessions

Judge your own change by its own evidence (`ai/rules/principles.md`).

- `internal/component/pki/config.go` imports `"strings"` unused, so the tree-wide
  lint gate is red.
- `internal/component/bgp/plugins/rib/storage/attrparse.go` did not compile
  earlier, so no `ze` binary linked. The CHAP agent worked around it with a
  compiler overlay of that one file from HEAD.
- `ai/PACKAGE-MAP.md`, `ai/RFC-REQUIREMENTS.md`, `docs/features/rfc-status.md`
  and `docs/config-reference.md` carry several sessions' regenerated rows. Every
  agent this session left them unstaged rather than carry another session's work,
  using `stale-index-ok` where the commit gate asked.

## Two fixit specs still open

`spec-fixit-locrib-peer-fields-contradict-rfc9069` and
`spec-fixit-tunnel-traffic-proof-is-one-directional` are both in progress and
both approved by Thomas on 2026-09-03. `spec-fixit-dns-rfc1035-conformance` stays
blocked: he said no DNS work.

## Addendum, later on 2026-09-04: the FreeRADIUS suite is further from done than its last line suggested

Its agent's final message was "Now wiring the suite into `./le integration`",
which reads as one step from runnable. It is not. **Its own unit tests do not
pass**, and they are the polarity harness that proves each checker fails for the
right reason, so the suite proves nothing until they do.

Every subtest of `TestPAPCheckerPolarities`, `TestCHAPCheckerPolarities` and
`TestCHAPHashedCheckerPolarities` fails identically at the FIRST assertion:

```
assertion 1: local control login as localop failed (exit 1):
error: cannot connect to daemon: ssh: handshake failed
```

That string is the stub's own refusal (`runLogin`, `radius_test.go`), not a real
SSH attempt, so the stub IS reached and it is declining to authenticate the local
control user. `conformingPAP` grants that user in both `silentUsers` and
`localPasswords`, and the field extraction is correct: I reproduced
`parseStubLogin` against the exact script `loginScript` builds and it recovers
`localop` and `testpass`. So the cause is NOT the policy fixture and NOT the
parsing, which is where the obvious suspicion falls. Whoever resumes should start
by instrumenting `runLogin`'s branch selection rather than re-reading the policy.

**One real defect was found and fixed on the way, and it is left UNCOMMITTED in
the working tree** (`internal/le/interoplab/radius/radius_test.go`).
`parseStubLogin` read the command with `stubField(script, " -c ")` over the
joined argv, and the checker hands `Exec` an argv of `{"sh", "-c", script}`, so
the first `" -c "` is the SHELL's own and the field after it is the script rather
than the command. The command therefore came back empty on every call, which made
`runLogin`'s denied-command branch unreachable and its polarity untestable. The
fix searches from the `ze cli` invocation onward. It is correct on its own merits
and it is NOT the cause of the failures above; keep it when you resume.

Stopped here under `ai/rules/pre-release.md`: two repairs to test scaffolding in
one stretch is the limit before it becomes the session, and the product work this
suite exists to prove is already committed.
