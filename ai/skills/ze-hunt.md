---
name: ze-hunt
description: Hunt Known Bug Classes
---

# Hunt Known Bug Classes

Sweep the whole tree (or a scoped path) for the recurring defect classes this
codebase has recorded about itself, then triage every hit into real bug /
cleared / intentional. Read-only: reports candidates, changes nothing.

**Source of truth:** `plan/learned/RECURRING-PATTERNS.md` (the *Correctness
traps*, *Testing traps*, and *Multi-source-of-truth traps* sections). Each hunt
below maps to one recorded trap. When that file gains a new pattern, add a
matching hunt here -- the document is the registry, this skill is the detector.

See also: `/ze-review` and `/ze-review-deep` (diff review, not whole-tree),
`/ze-find-alloc` (allocation audit), `make ze-mutation-test-changed` (grade the suite).

## When to use

- Periodic latent-bug sweep of an existing subsystem, not just the diff.
- Right after a new recurring trap is recorded: harvest existing instances tree-wide.
- Before a release or a deep review, to seed the candidate list.

## Arguments

Optional `$ARGUMENTS` = scope path (default scope: `internal/ cmd/ pkg/`).
Example: `/ze-hunt internal/component/bgp/`.

## Method

1. Use ULTRATHINK for thorough triage.
2. Run each hunt below as a grep over the scope (exclude `vendor/`, `tmp/`,
   generated `*_generated.go` / `*.pb.go`).
3. **A grep hit is a CANDIDATE, never a finding.** For every hit, open the file
   and read the surrounding code before classifying. Do not report a grep line
   as a bug.
4. Rule out the known false positives listed per hunt.
5. For surviving candidates, verify against callers: does any caller handle the
   bad value (e.g. check `.IsValid()`, the error, the lock)? If yes, downgrade;
   if no, it is a real candidate. Resolve callers with the LSP tool's
   `findReferences` where your registry carries it, and with
   `gopls references <file>:<line>:<col>` from Bash where it does not. A
   `ze-read` agent has no LSP tool, so it takes the second route
   (`ai/rules/context-economy.md`). Never fall back to grep for this: it matches
   comments and string literals too.
6. Report in the format below. Honest negatives matter: state which classes
   came back clean.

## Hunts (recorded trap -> detector)

Each entry: the trap it targets, the grep, the directories worth focusing on,
and the false positives already known from prior sweeps.

### H1 -- Silent fall-through in parser/dispatch (highest value: recorded 20+ times)

Unknown wire code/keyword silently routed to a default value instead of an error.

- Grep `default:` in wire-facing packages, then inspect the branch body:
  `internal/component/bgp/{wireu,message}`, `internal/core/bgp/{attribute,capability}`,
  `internal/component/bgp/plugins/nlri`.
- **Real shape:** `default:` returns a *value* (`return nil`, `return netip.Addr{}`,
  `return false`, or assigns the most common enum) with no error and no log,
  in an *encoder* or in a *decoder whose caller does not handle the sentinel*.
- **Known false positives (do NOT report):**
  - `default:` that returns an error naming the unknown value (e.g.
    `nlri/evpn/encode.go`, `nlri/vpls/encode.go`, `attribute/builder_parse.go`).
  - `flowspec/config_builder.go` `default: op = FlowOpEqual` -- intentional:
    bare protocol/number implies equality.
  - `wireu/mpwire.go` default wrapping unknown families as opaque `WireNLRI` --
    intentional family preservation.
  - `wireu/mpwire.go` `NextHop()` / `Prefixes()` defaults returning
    `netip.Addr{}` / `nil` for non-IPv4/IPv6 AFI -- verified benign: read
    accessors returning documented sentinels, not encoders. Callers check
    `IsValid()` (`server/codec.go`) or omit non-CIDR prefixes (`appendMPBlock`);
    `netip.Prefix` cannot represent VPN/EVPN/FlowSpec, which use `NLRIs()`.

### H2 -- Signed subtraction for sequence-number ordering

```
int(8|16|32|64)\([^()]*-[^()]*\)\s*[<>]\s*0
```

- Correct form is unsigned distance: `uintN(b-a) <= maxN/2`.
- **Known clean:** `internal/component/l2tp/reliable_seq.go` already uses the
  unsigned form (this trap's origin, #595). Treat a hit there as the fix, not a bug.
- Broaden by hand for multi-variable comparisons on any modular counter (BGP
  update-id, BMP, L2TP Ns/Nr) -- the regex only catches the inline cast form.

### H3 -- Fallible function returns `(nil, nil)` / `sync.Once` caching an error

```
return nil, nil          # in fallible (T, error) functions
OnceValue|OnceValues     # check whether the wrapped callback can fail
```

- `nilnil` linter already flags bare `return nil, nil`; this hunt targets the
  variant it misses: `sync.Once`/`OnceValue` wrapping a `(value, error)` callback,
  which caches the first call's `(nil, err)` and loses the error on later calls.
- **Known clean:** `internal/core/version/version.go` `OnceValue` wraps an
  infallible info read.

### H4 -- Synchronization comments with no enforced lock

```
(?i)externally synchronized|caller (must )?holds?|not thread.?safe|assumes? .*lock|must be called with.*lock|single.?threaded
```

- Each "Caller MUST hold X" comment is aspirational unless a runtime assertion
  enforces it; the race detector cannot see the contract otherwise.
- **Triage:** for each, grep every caller and confirm the lock is actually held.
  Prioritise concurrent hot paths: `reactor/`, `config/tree.go`,
  `config/transaction/`, `core/rib/locrib/`, `core/events/`, `bfd/engine/`.
- Fix shape: add an assertion (or the lock); delete a false comment.

### H5 -- Hardcoded enumeration counts in tests

```
len\([^()]*\)\s*[=!]=\s*[0-9]{2,}        # in *_test.go
(assert|require)\.(Len|Equal)\([^,]*,\s*[0-9]+
[=!]=\s*len\(                            # reversed form
```

- **Real shape:** a literal asserted against a *registry* count (RPCs, commands,
  plugins, peers) -- breaks on every feature add and conflicts across sessions.
- **Known false positives:** exact wire/key sizes (`ike/crypto`, `radius`, `ppp`,
  `bufpool`) -- those literals are correct and must stay.
- Fix shape: read the count from the registry, or use `>= floor` with a comment
  naming the intent.

### H6 -- Sleep-based test synchronization (hides races)

```
time\.Sleep        # in *_test.go, count per file
```

- Replacing `time.Sleep` with real synchronization is a known race-discovery
  technique here (`feedback_sleep_hides_races`).
- Rank by count; highest-value targets are concurrency-sensitive suites
  (`reactor/`, `healthcheck/`, `bgp/plugins/bmp/`, `l2tp/`).

### H7 -- Multi-source-of-truth drift (structural)

Two places that must agree but can silently diverge: YANG module vs
`yang_schema.go`, an env var registered twice, a plugin/command listed in two
files. For each registry pair, confirm a build-time consistency test diffs the
two sources. Where none exists, that gap is the finding. See
`ai/rules/evidence.md`.

### H8 -- Unwired feature (structural; the project's most recurring defect)

A production (non-`_test.go`) exported symbol with zero non-test callers is
either dead code or a feature not reachable from any user entry point. Resolve
callers on suspect exported symbols by the LSP tool's `findReferences`, or by
`gopls references` from Bash when the tool is absent, never by grep (step 5).
Cross-check that registered CLI dispatch keys, web routes, and config
validators are reachable.
See `ai/rules/completion.md`.

## Report format

### Candidates (need a fix or a decision)

| Hunt | Site (file:line) | Trap | Why suspect | Caller-verified? |
|------|------------------|------|-------------|------------------|

### Cleared classes (swept, came back clean)

| Hunt | Result |
|------|--------|
| H2 signed seqnum | only the documented fix site |

### Ruled out (matched a grep but intentional)

| Site | Why it is correct |
|------|-------------------|

### Recommended next detector

Name the cheapest mechanical gate that would retire the top class permanently
(e.g. a `go/analysis` pass in `internal/analyze/` for H1, a consistency test for H7).

## Beyond grep (escalation, only when asked or scope is "thorough")

Grep catches the recorded *shapes*. To find unknown bugs, escalate:

- `make ze-mutation-pkg-test PKG=<dir>` -- surviving mutants reveal untested logic
  (where bugs hide). Start with wire codec, FSM, RIB.
- `make ze-fuzz-test` on wire/NLRI/attribute decoders, long duration, MRT-seeded
  corpus -- the highest-severity untrusted-input surface.
- `test/interop/` + `ze-functional-exabgp-test` -- differential testing vs FRR/BIRD/ExaBGP.
- `make ze-unit-reactor-test-race` + `ze-chaos` seed sweep -- concurrency.

For a large scope, run the independent hunts as parallel subagents (one per
subsystem or per hunt) and have a second agent adversarially verify each
surviving candidate before it reaches the report. Both are read-only work, so
spawn them with `subagent_type: ze-read`, which costs about 6k fewer startup
tokens per agent than the default (`ai/rules/context-economy.md`).

## Do NOT

- DO NOT modify any file -- this is a read-only sweep.
- DO NOT report a grep hit without reading the code around it.
- DO NOT report the known false positives above.
- DO NOT claim a class is clean without actually running its hunt over the scope.
