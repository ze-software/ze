# Spec: interop image copies a prebuilt ze

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | tooling |
| Depends | - |
| Phase | 1/6 |
| Handoff | - |
| Updated | 2026-09-06 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`test/interop/Dockerfile.ze` compiles `ze` and `ze-test` inside the container.
It does `COPY . .` from the repository root and runs two `go build` invocations
with no cache mount, so every change under `internal/` recompiles both binaries
from scratch. Several sessions edit `internal/` continuously, so the `COPY`
layer is invalidated between one run and the next and the cache never helps.

Two independent costs were measured, and they have one cause.

| Date | Host | Measurement |
|------|------|-------------|
| 2026-09-04 | colima VM, 2 CPUs, 2 GB, host disk full | the same `docker build` run to a private tag with no deadline took 40m39s; two harness runs died at `exit 124` |
| 2026-09-04 | same VM, healthy | 2m48s |
| 2026-09-06 | this workstation, `docker info` reports 32 CPUs and 31.34GiB | `INTEROP_SCENARIO=as-path-prepend-two-octet-peer ./le integration interop` killed by the kernel for low memory during the image build |
| 2026-09-06 | same, retried under contention | second kill, same step |
| 2026-09-06 | same, retried deliberately: load average 0.61, 23G available of 31G, no other heavy work, no peer session building | third kill, same step |

The third run is the one that settles it. The build does not fit this machine
**on its own**, so waiting for a quiet moment is not a mitigation: there is no
quiet moment on this machine that fits this build. Three attempts produced three
OOM kills and zero scenario verdicts.

Rows: `plan/journal/gate-verdict-depends-on-the-machine.md`, 2026-09-04
(`fixit-filter-subject-drops-five-attributes`, the `./le integration interop`
row) and 2026-09-06 (the two
`as-path-prepend-encodes-at-the-negotiated-width` rows).

The goal is that the interop image stops carrying a Go compiler. The daemon and
the test personality are cross-compiled on the host, once, into the build
context, and the image copies them in. `test/interop-ipsec/Dockerfile.ze` and
`test/interop-radius/Dockerfile.ze` already have that shape, so this spec
extends a shipped pattern to the three labs that still compile inside the image
and gives all five one producer.

**Bucket: `plan/` (top level).** The test in `plan/README.md` is what the
undone work costs the FIRST RELEASE. No operator meets this: it is a
development gate on a developer's workstation, and it changes no shipped
behavior, no wire byte and no CLI answer. It is not `pre-release/` either,
because the interop evidence the release owes is still produced: the `interop`
job in `.github/workflows/evidence-nightly.yml` runs `./le integration interop`
on `ubuntu-latest` and is unaffected by whether one workstation can run it.
What is blocked is a person's ability to run a scenario before pushing it,
which is a development cost rather than a release one.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/testing/interop.md` - the lab this spec changes, and
      the page that publishes the build-time claim
  → Decision: each image build is bounded at 90 minutes (`BUILD_TIMEOUT` in
    whole seconds overrides it), and `ImageBuild.Timeout` only ever LENGTHENS a
    bound, so no suite declares one. This spec must not reintroduce a per-image
    cap.
  → Constraint: the "Running" section states `Dockerfile.ze` "copies the whole
    tree and compiles ze twice with no cache mount" and quotes 2m48s and
    40m39s. That paragraph is a factual claim this change falsifies, so it is
    edited in the same work (`ai/rules/documentation.md`).
- [ ] `docs/contributing/rfc-conformance-gates.md` - the discrimination gate,
      which special-cases the interop carrier
  → Constraint: the interop break travels in the WORKING TREE rather than in a
    Go overlay, and the page justifies that by "the image build compiles ze
    INSIDE Docker". The route is still correct after this change and its stated
    reason is not, so the reason is rewritten rather than the route.
- [ ] `CLAUDE.md`, "Binary naming convention" - host versus target binaries
  → Constraint: a target binary is cross-compiled `GOOS=linux
    GOARCH=<arch> CGO_ENABLED=0`, and a host binary is NEVER cross-compiled. The
    lab binary is a target binary. `bin/le` and `ze-host` are untouched.

### RFC Summaries (Scope: protocol)
N-A. This spec changes no protocol code, no wire encoding and no RFC-enforcing
function. The only RFC-adjacent surface it touches is the discrimination gate's
prose, and that is covered above.

**Key insights:**
- The failure is a MEMORY spike, not a slow build. A cache mount addresses time
  on a warm cache and does not remove the compiler from the container, so it
  does not fix what was measured three times.
- `CGO_ENABLED=0` is the whole libc answer: a static Go binary references no
  libc, which is why `test/interop-radius/ze-linux` runs on `alpine:3.21`
  today.
- The bgp lab needs TWO binaries in its image, not one: scenarios invoke
  `ze-test interop-bgp process ...` from inside the container.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `test/interop/Dockerfile.ze` - `FROM golang:1.27-alpine AS builder`,
      `COPY . .`, then two `go build` calls (`-mod=vendor`, tags `ze_core
      ze_distro $ZE_FEATURES` to `/ze` and `ze_test $ZE_FEATURES` to
      `/ze-test`), then an `alpine:3.21` stage that `apk add`s `tini nftables`
      and copies both binaries out of the builder.
- [ ] `internal/le/interoplab/bgp/run.go` - `RunAt` reads the environment,
      discovers scenarios, calls `featuretags.DaemonTags`, and declares the
      `ze` image with `Dockerfile: <producer>/Dockerfile.ze`, `Context: root`,
      `BuildArgs: ["ZE_FEATURES=" + the joined tags]`, `Required: true`. It
      sets no `Preflight`.
- [ ] `internal/le/interoplab/docker.go` - `Docker.Build` runs `docker build -t
      <tag> [--build-arg ...] -f <dockerfile> <context> -q` under
      `build.Timeout` or the machine default (`dockerBuildTimeoutDefault`,
      overridden by `BUILD_TIMEOUT`). It passes **no** `--platform`, so the
      image is built for the Docker daemon's own default platform.
- [ ] `internal/le/interoplab/lab.go` - `Suite` carries a `Preflight` of type
      `PreflightCheck`, which `Suite.Run` calls before any image is built.
- [ ] `internal/le/interoplab/radius/radius.go` - `buildZe` cross-compiles the
      lab daemon: `featuretags.DaemonBuildTags` with base `ze_core ze_distro`,
      `gotoolchain.New`, output `test/interop-radius/ze-linux`, a 5-minute
      bound, and the environment from `Toolchain.Environment` with
      `EnvOptions{GOOS: "linux"}`. It is wired as the suite `Preflight` and
      returns early under `NO_BUILD`.
- [ ] `internal/le/interoplab/ipsec/ipsec.go` - a second, near-identical
      `buildZe` writing `test/interop-ipsec/ze-linux`, wired the same way.
- [ ] `internal/le/gotoolchain/gotoolchain.go` - `EnvOptions` carries `CGO`,
      `GOOS` and `GOARCH`. `Toolchain.Overrides` emits `CGO_ENABLED=0` unless
      `CGO` is set, pins `GOTOOLCHAIN` to the one `go.mod` names, and puts
      `GOCACHE` inside the checkout.
- [ ] `test/interop-l2tp/Dockerfile.ze`, `test/interop-pppoe/Dockerfile.ze` -
      both compile inside the image: `COPY go.mod go.sum ./`, `go mod
      download`, `COPY . .`, then a `go build` whose tags are derived by an
      inline `awk` over `feature-gates.txt`.
- [ ] `.dockerignore` - excludes `test/`, then re-admits
      `test/interop-ipsec/ze-linux` and `test/interop-radius/ze-linux` by
      negation. `bin/` and `cache/` are excluded outright.
- [ ] `.gitignore` - both staged lab binaries, under a comment naming `buildZe`
      as the producer and saying they are regenerated each run.
- [ ] `internal/le/goversion/dockerfile.go` - `copiesModule` returns false for
      a `COPY --from=` and for any COPY whose sources do not include `go.mod`,
      `.` or `./`. `stagesOf` therefore marks only module-copying stages, and
      `Result.judgeDockerfile` counts exactly those as carriers.
- [ ] `internal/le/rfc/discriminate_observe.go` - `requireRed` routes
      `kindInterop` to `requireRedInTree`, which writes the break into the
      working tree and restores it byte for byte. Its comment justifies the
      exception with "The BGP interop lab compiles ze INSIDE Docker".
- [ ] `.github/workflows/evidence-nightly.yml` - five interop jobs, all
      `runs-on: ubuntu-latest`, each invoking a `./le integration interop*`
      action.

**Behavior to preserve:**
- The image's contents: `/usr/local/bin/ze` and `/usr/local/bin/ze-test` on an
  `alpine:3.21` base with `tini` and `nftables`, entrypoint `tini -- ze`.
- The feature set. Both binaries carry the tags `feature-gates.txt` declares,
  through `featuretags`, so the lab daemon cannot drift from the shipped one.
- `NO_BUILD=1` skips every build, preflight included.
- `BUILD_TIMEOUT` still bounds each `docker build`.
- The interop discrimination route: a working-tree break still reddens the
  tagged interop unit.
- Every scenario's observable behavior. No scenario file changes.

**Behavior to change:**
- The image build no longer runs a Go compiler. The binaries are produced on
  the host, before the build, and copied in.
- The bgp lab gains a `Preflight`, and stops passing `ZE_FEATURES` as a build
  argument.
- `test/interop-l2tp/Dockerfile.ze` and `test/interop-pppoe/Dockerfile.ze` take
  the same shape, and their inline `awk` over `feature-gates.txt` is deleted in
  favour of `featuretags`.
- `ipsec.buildZe` and `radius.buildZe` are deleted and both labs call the one
  shared producer.

## Data Flow (MANDATORY)

### Entry Point
`./le integration interop` (and `interop-l2tp`, `interop-pppoe`,
`interop-ipsec`, `interop-radius`), plus the same actions run by the five jobs
in `.github/workflows/evidence-nightly.yml`.

### Transformation Path
1. The lab's `RunAt` reads `feature-gates.txt` through `featuretags` and asks
   the Docker daemon which architecture it builds for.
2. `Suite.Run` calls `Preflight`, which cross-compiles each declared binary
   with `GOOS=linux`, the daemon's `GOARCH`, and `CGO_ENABLED=0`, writing it
   into the lab directory inside the build context.
3. `Docker.Build` runs `docker build` on a Dockerfile whose only stage is
   `alpine:3.21` plus `apk add` plus `COPY <staged binary>`.
4. Scenarios run unchanged.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Host toolchain ↔ container image | a static `GOOS=linux` binary staged into the build context and copied by the image | No |
| `le` ↔ Docker daemon | `docker version --format '{{.Server.Arch}}'` read once, to choose `GOARCH` | No |
| `feature-gates.txt` ↔ lab binary | `featuretags.DaemonBuildTags`, replacing an inline `awk` in two Dockerfiles and a `--build-arg` in one lab | No |

### Integration Points
- `interoplab.Suite.Preflight` - the hook the shared producer is wired to,
  already used by the ipsec and radius labs.
- `featuretags.DaemonBuildTags` - the single declaration of the daemon feature
  set.
- `gotoolchain.Toolchain.Environment` - the pinned toolchain, the in-checkout
  `GOCACHE`, and `CGO_ENABLED=0`.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | No | the build goes through `Suite.Preflight`, the hook the two converted labs already use, rather than a step bolted onto `Docker.Build` |
| No unintended coupling | No | `interoplab` gains a dependency on `featuretags` and `gotoolchain`, both of which two of its sub-packages already import |
| No duplicated functionality | No | two copies of `buildZe` become one; three inline tag derivations become one call |
| Zero-copy preserved where applicable | No | N-A: no encoding path is touched |
| Registration over hardcoding | No | each lab DECLARES the binaries it needs beside the images it needs; the shared producer holds no per-lab branch and no list of labs |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `CGO_ENABLED=0` makes the binary independent of the container's libc | `Toolchain.Overrides` sets it by default; `test/interop-radius/Dockerfile.ze` runs such a binary on `alpine:3.21` today | the image execs and fails on musl | the converted lab starts a container and its ready probe passes | unvalidated |
| A-2 | `docker version --format '{{.Server.Arch}}'` returns a Go `GOARCH` spelling (`amd64`, `arm64`) | Docker reports the daemon architecture in Go's own naming | `go build` refuses an unknown `GOARCH`, loudly, before any image is built | a unit test over the parser plus one real run on this workstation | unvalidated |
| A-3 | The image needs both `ze` and `ze-test` | the current Dockerfile copies `/ze-test` into the image, and 14 scenario `ze.conf` files `run "ze-test interop-bgp process ..."` | scenarios fail with "not found" | `TestBGPPreflightDeclaresBothPersonalities` and a real scenario run | unvalidated |
| A-4 | Removing the builder stage leaves the Go-version gate with carriers | `docker/Dockerfile`, `docker/Dockerfile.lab`, `internal/le/interoplab/l2tp/radiusmock/Dockerfile` and `tools/kernel-builder/Dockerfile` still copy the module, and `Result.judgeGoSource` counts Go string literals as carriers too | `goversion` errors with "the walk judged no build carrier" | `./le verify current mode full`, which runs the gate | unvalidated |
| A-5 | The nightly runners still pass: they already build `bin/le` with Go, so the host toolchain is present | every interop step in `.github/workflows/evidence-nightly.yml` runs a `./le` action | five nightly jobs go red | read the workflow's setup steps; then one nightly cycle | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A daemon whose architecture differs from the host (a remote Docker context, `DOCKER_DEFAULT_PLATFORM`) gets a binary it cannot exec, with the confusing `exec format error` | the container exits immediately and the ready probe times out | the `GOARCH` comes from the daemon rather than the host, and the preflight refuses with the two architectures named when it cannot read one |
| R-2 | The staged binary is stale from a previous run and a scenario tests yesterday's daemon | a fix that is in the tree does not change the verdict | the preflight rebuilds on every run that is not `NO_BUILD`; `go build` is incremental against the checkout's `GOCACHE`, so this costs seconds |
| R-3 | `.dockerignore` excludes the staged path, and `docker build` fails with "file not found" | the first converted build fails at the `COPY` | a test derives the negation list from the labs' declared staging paths rather than reading a hand-written list |
| R-4 | The image stops being buildable by a bare `docker build`, and a person meets a confusing missing-file error | a hand-run outside `./le` | each converted Dockerfile's header comment names the exact producing action, as `test/interop-ipsec/Dockerfile.ze` already does |
| R-5 | The three prose sites that justify the interop discrimination exception go stale and a later reader concludes the route is wrong | none: prose does not fail | they are edited in this same change, and the route itself is re-proved by re-recording `RFC1997-Well-1` |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | five development gates and five nightly jobs. Nothing user-visible: no shipped binary, no wire byte, no config surface and no CLI answer changes |
| How is it reverted? | a single-commit revert. The change is three Dockerfiles, one new producer, two deleted copies of it, `.dockerignore`, `.gitignore` and prose |
| Who else touches this path? | `plan/spec-perf-next-0-umbrella.md` names a stale `Dockerfile.ze` in its AC-1/AC-3 discussion, about the perf harness rather than this lab. The RFC discrimination gate reads the interop carrier's build shape and is addressed here |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le integration interop` | → | `bgp.RunAt` sets `Suite.Preflight` | `TestBGPSuiteDeclaresAPreflightBuild` |
| `Suite.Preflight` invoked by `Suite.Run` | → | the shared `interoplab` producer | `TestPreflightBuildsEveryDeclaredBinary` |
| the producer's `go build` environment | → | `gotoolchain.Environment` | `TestLabCrossBuildIsStaticLinuxAtTheDaemonArch` |
| `docker build` context | → | `.dockerignore` negations | `TestDockerIgnoreAdmitsEveryStagedLabBinary` |
| a working-tree break on an interop carrier | → | `requireRedInTree` | re-recorded `RFC1997-Well-1` in `rfc/discrimination/rfc1997.json` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `grep -c 'go build' test/interop/Dockerfile.ze test/interop-l2tp/Dockerfile.ze test/interop-pppoe/Dockerfile.ze` | zero in each. No `Dockerfile.ze` in the tree runs a compiler or names a `golang:` base |
| AC-2 | `INTEROP_SCENARIO=as-path-prepend-two-octet-peer ./le integration interop` on this 31G workstation, run once | reaches a scenario verdict. Three attempts before this change produced three OOM kills and no verdict |
| AC-3 | the image build step of AC-2 | its wall time is recorded, beside the 40m39s and 2m48s the page publishes today, and the page's paragraph is rewritten to the new shape |
| AC-4 | the preflight `go build` of AC-2, measured with `/usr/bin/time -v` | its peak resident set is recorded. This is the number that replaces an unmeasurable in-container compiler |
| AC-5 | a container from the built image | `ze` and `ze-test` both run inside it, and the scenario's ready probe passes, on an `alpine:3.21` (musl) base |
| AC-6 | a Docker daemon whose architecture the preflight cannot read | the run fails before any image is built, with a message naming what it asked and what it got. It never guesses a `GOARCH` |
| AC-7 | `./le rfc discriminate-record` re-run for `RFC1997-Well-1` | observes the green, applies the working-tree break, observes a red that names the interop unit, and writes a record `./le rfc check` accepts |
| AC-8 | `grep -rn 'func buildZe' internal/le/interoplab/` | one producer, in `internal/le/interoplab/`. The ipsec and radius copies are gone and both labs call it |
| AC-9 | `./le integration interop-ipsec` and `./le integration interop-radius` | still pass, unchanged in behavior, on the shared producer |
| AC-10 | `./le verify current mode full` | passes, `goversion` included: the Go-version gate still judges at least one carrier |

## End-to-End User Stories

Deleted: Scope is `tooling`. The users of this path are the five `./le
integration` actions and the five nightly jobs, and the Wiring Test table above
covers each one.

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPreflightBuildsEveryDeclaredBinary` | `internal/le/interoplab/zebuild_test.go` | each declared binary gets its own `go build` with its own tags and output path | |
| `TestLabCrossBuildIsStaticLinuxAtTheDaemonArch` | `internal/le/interoplab/zebuild_test.go` | the build environment carries `GOOS=linux`, `CGO_ENABLED=0`, and the `GOARCH` the daemon reported | |
| `TestPreflightRefusesAnUnreadableDaemonArchitecture` | `internal/le/interoplab/zebuild_test.go` | AC-6: no default, no host fallback, an error naming the query and the answer | |
| `TestPreflightSkippedUnderNoBuild` | `internal/le/interoplab/zebuild_test.go` | `NO_BUILD=1` performs no build | |
| `TestBGPSuiteDeclaresAPreflightBuild` | `internal/le/interoplab/bgp/bgp_test.go` | `RunAt` wires the preflight and no longer passes `ZE_FEATURES` as a build argument | |
| `TestBGPPreflightDeclaresBothPersonalities` | `internal/le/interoplab/bgp/bgp_test.go` | the daemon (tags `ze_core ze_distro` plus gates) and the test personality (tags `ze_test` plus gates) are both declared | |
| `TestZeDockerfilesCarryNoCompiler` | `internal/le/interoplab/zebuild_test.go` | AC-1, over every tracked `test/interop*/Dockerfile.ze` discovered by walk rather than by a hand-written list | |
| `TestDockerIgnoreAdmitsEveryStagedLabBinary` | `internal/le/interoplab/zebuild_test.go` | R-3: every staging path the labs declare has a `.dockerignore` negation, derived from the declarations | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| preflight build bound (seconds) | must be positive | 300, the value both existing copies use | 0, which is rejected before a build starts | N-A: a long bound costs nothing, matching `Docker.Build` |

N-A otherwise: this change introduces no other numeric input. `BUILD_TIMEOUT`
and `ImageBuild.Timeout` are unchanged and already covered.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| N-A | - | this path has no `.ci` surface: its entry point is a `./le` action, exercised by the Interop table below and by the unit tests over its wiring | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `as-path-prepend-two-octet-peer` | `test/interop/scenarios/` | FRR | AC-2: the scenario that could not run three times reaches a verdict | |
| `bgp-wellknown-noexport-frr` | `test/interop/scenarios/` | FRR | AC-7: the interop discrimination route still observes green then red under a working-tree break | |
| `01-pppoe-chap-ipv4` | `test/interop-pppoe/` | accel-ppp | the converted PPPoE image runs the daemon it used to compile | |
| `bgp-redistribute` | `test/interop-l2tp/` | LAC | the converted L2TP image runs the daemon it used to compile | |
| existing suite | `test/interop-ipsec/`, `test/interop-radius/` | strongSwan, FreeRADIUS | AC-9: the two labs already on this shape are unchanged by the consolidation | |

No wire-visible behavior changes, so no NEW scenario is owed
(`ai/rules/interop-and-goal-validation.md`: tooling with no protocol impact).
The rows above are the existing scenarios this change must not break, plus the
two that measure it.

## Files to Modify
- `test/interop/Dockerfile.ze` - delete the `golang` builder stage; copy the two
  staged binaries; header comment names the producing action
- `test/interop-l2tp/Dockerfile.ze` - same, one binary; delete the inline `awk`
  over `feature-gates.txt`
- `test/interop-pppoe/Dockerfile.ze` - same, one binary; delete the inline `awk`
- `internal/le/interoplab/bgp/run.go` - wire `Preflight`, declare the two
  binaries, drop the `ZE_FEATURES` build argument
- `internal/le/interoplab/l2tp/l2tp.go` - wire `Preflight`, declare one binary
- `internal/le/interoplab/pppoe/pppoe.go` - wire `Preflight`, declare one
  binary; the `imageBuilds` comment describing the in-image compile is now
  wrong and is rewritten (`ai/rules/stale-comments.md`)
- `internal/le/interoplab/ipsec/ipsec.go` - delete `buildZe`, call the shared
  producer
- `internal/le/interoplab/radius/radius.go` - delete `buildZe`, call the shared
  producer
- `internal/le/interoplab/docker.go` - the `dockerBuildTimeoutVariable` comment
  describing `test/interop/Dockerfile.ze` as copying the whole tree and
  compiling ze twice is now wrong and is rewritten
- `internal/le/rfc/discriminate_observe.go` - `requireRedInTree`'s comment
  justifies the working-tree route by "the lab compiles ze INSIDE Docker". The
  route is unchanged and its reason becomes: the preflight `go build` reads no
  Go overlay, so a break has nowhere to go but the tree
- `.dockerignore` - negations for the newly staged binaries
- `.gitignore` - the staged binaries, with the comment naming the one producer
- `docs/architecture/testing/interop.md` - the "Running" paragraph quoting
  "compiles ze twice with no cache mount", 2m48s and 40m39s
- `docs/contributing/rfc-conformance-gates.md` - the interop-carrier exception's
  stated reason
- `ai/skills/ze-rfc.md` - the same sentence, in the skill
- `docs/labs/l2tp-interop.md`, `docs/labs/pppoe-interop.md` - the `Dockerfile.ze`
  line in each layout listing
- `plan/journal/gate-verdict-depends-on-the-machine.md` - the `Fix` cell of the
  three rows this spec answers

## Files to Create
- `internal/le/interoplab/zebuild.go` - the one producer, and the declaration a
  lab makes

| Field | Type | Description |
|-------|------|-------------|
| `Name` | string | what the binary is called in a message, for example `ze` or `ze-test` |
| `Tags` | string | the build tags, from `featuretags.DaemonBuildTags` with the lab's own base |
| `Output` | string | the repository-relative staging path, inside the build context and admitted by `.dockerignore` |

The exported function takes a context, the repository root, and the declared
binaries; it reads the Docker daemon's architecture once, then builds each
binary with `gotoolchain.Environment` under `GOOS=linux`, that `GOARCH`, and
the default `CGO_ENABLED=0`.

- `internal/le/interoplab/zebuild_test.go` - the unit tests listed above

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | no operator-facing configuration; the surface is a `./le` development action |
| YANG validation constraints | N-A | no YANG leaf |
| YANG custom validators | N-A | no YANG leaf |
| CLI commands/flags | N-A | `./le integration interop*` already exists and its arguments are unchanged |
| CLI grammar (keyword before value) | N-A | no command added |
| Editor autocomplete | N-A | no YANG leaf |
| Functional test for new RPC/API | N-A | no RPC or API |
| Pipe completeness | N-A | no CLI output added |
| Env var registration | N-A | `BUILD_TIMEOUT`, `NO_BUILD` and `INTEROP_SCENARIO` are the lab's existing environment, read by `interoplab.ReadEnvironment`, and none is added or changed |
| Doctor check for runtime dependencies | No | the host Go toolchain is not a new dependency: `./le` builds `bin/le` with it before any lab runs, and `./le setup` installs it. Docker is already probed by `Docker.Probe` |
| Prometheus counters/metrics | N-A | development tooling, no daemon state |
| BGP family surface | N-A | no SAFI, capability or attribute |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | nothing an operator sees changes |
| 2 | Config syntax changed? | No | no config surface touched |
| 3 | CLI command added/changed? | No | `./le integration interop*` unchanged |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | No | none |
| 6 | Has a user guide page? | No | this is a development gate |
| 7 | Wire format changed? | No | no encoding path touched |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented, changed, or newly proven? | No | `RFC1997-Well-1` is RE-recorded because its carrier's build shape changed; the requirement, the claim and the producer are untouched, so no `rfc/short/` or `docs/features/rfc-status.md` row moves |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/interop.md`, `docs/labs/l2tp-interop.md`, `docs/labs/pppoe-interop.md` |
| 11 | Affects daemon comparison? | No | `docs/comparison.md` describes the daemon |
| 12 | Internal architecture changed? | Yes | `docs/contributing/rfc-conformance-gates.md` (the interop-carrier exception's reason) and `ai/skills/ze-rfc.md` (the same sentence). `docs/architecture/core-design.md` is DECLARED by `internal/le/rfc/discriminate_observe.go` and is UNAFFECTED: that page describes no build shape and no lab, the file's only edit here is one comment's stated reason, and `requireRedInTree` keeps the behavior the page's readers rely on. `docs/architecture/testing/interop.md` is the page declared by every other file in Files to Modify and is edited under row 10 |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | no registry entry changes |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED by `./le spec citation anchors spec plan/spec-interop-image-copies-a-prebuilt-ze.md`, re-run before closure. It names three advisory mentions today, and all three are UNAFFECTED: `docs/features/interoperability-testing.md` anchors `bgp/run.go` and `lab.go` for the target-daemon table and states nothing about how the ze image is built; `docs/features.md` and `docs/guide/ipsec.md` mention `ipsec/ipsec.go` for IKE behavior, which this change does not touch. `internal/le/site/facts/sitefacts.go` derives its interop target list from `git ls-files test/interop/Dockerfile.*` less `Dockerfile.ze`, so that list does not move either |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | the "Running" block in `docs/architecture/testing/interop.md` lists `NO_BUILD=1` and `BUILD_TIMEOUT=7200`; both still hold and are re-checked against the converted path |

### Discovery (`ai/rules/repo-maintenance.md` Mechanical Checklist)
| Question | Answer |
|----------|--------|
| Where does an agent look first? | `ai/INDEX.md` routes "interop" to `docs/architecture/testing/interop.md`, which after this change describes the prebuilt shape and names the producer |
| What rule prevents regression? | `TestZeDockerfilesCarryNoCompiler` walks every tracked `test/interop*/Dockerfile.ze` and refuses a `golang:` base or a `go build`, so a fourth lab cannot be written back into the old shape |
| What registry prevents drift? | the labs' own binary declarations. `TestDockerIgnoreAdmitsEveryStagedLabBinary` derives the required `.dockerignore` negations from those declarations rather than from a second list |
| What verification proves it? | `./le verify current mode full` for the unit and gate side, and the AC-2 and AC-7 runs for the measured side |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the producer exists and the bgp lab
   reaches it
   - Tests: `TestBGPSuiteDeclaresAPreflightBuild`,
     `TestPreflightBuildsEveryDeclaredBinary`
   - Files: `internal/le/interoplab/zebuild.go` (stub returning an error),
     `internal/le/interoplab/bgp/run.go`
   - Verify: the wiring tests fail because the producer is a stub
2. **Phase: the producer** -- cross-compile at the daemon's architecture
   - Tests: `TestLabCrossBuildIsStaticLinuxAtTheDaemonArch`,
     `TestPreflightRefusesAnUnreadableDaemonArchitecture`,
     `TestPreflightSkippedUnderNoBuild`
   - Files: `internal/le/interoplab/zebuild.go`
   - Verify: AC-6 holds before any real build is attempted
3. **Phase: the bgp image** -- the measured one
   - Tests: `TestBGPPreflightDeclaresBothPersonalities`,
     `TestZeDockerfilesCarryNoCompiler`,
     `TestDockerIgnoreAdmitsEveryStagedLabBinary`
   - Files: `test/interop/Dockerfile.ze`, `.dockerignore`, `.gitignore`
   - Verify: AC-2, AC-3, AC-4 and AC-5 measured and recorded on this workstation
4. **Phase: the two remaining compiling labs**
   - Tests: the same walk tests, now covering three files
   - Files: `test/interop-l2tp/Dockerfile.ze`,
     `test/interop-pppoe/Dockerfile.ze`, `l2tp.go`, `pppoe.go`
   - Verify: one scenario from each suite runs
5. **Phase: one producer** -- delete the two copies
   - Tests: AC-8, plus the existing ipsec and radius suite tests
   - Files: `ipsec.go`, `radius.go`
   - Verify: AC-9
6. **Phase: the prose the change falsified**
   - Files: the `docker.go`, `discriminate_observe.go` and `pppoe.go` comments;
     `docs/architecture/testing/interop.md`,
     `docs/contributing/rfc-conformance-gates.md`, `ai/skills/ze-rfc.md`,
     `docs/labs/l2tp-interop.md`, `docs/labs/pppoe-interop.md`
   - Verify: AC-7 re-records `RFC1997-Well-1`, proving the route the rewritten
     prose describes; AC-10 passes

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every AC-N has a file or a recorded measurement behind it |
| Correctness | the staged binary is `GOOS=linux`, `CGO_ENABLED=0`, at the DAEMON's `GOARCH`, and no host binary anywhere in the tree acquired a `GOARCH` |
| Correctness | the feature tags reaching each staged binary are the same set the deleted Dockerfile derived: `ze_core ze_distro` plus gates for the daemon, `ze_test` plus gates for the test personality |
| Naming | the producer is named for what it builds, not for one lab, since five labs call it |
| Data flow | `feature-gates.txt` is read once per lab through `featuretags`, and no `awk`, `--build-arg` or second derivation survives |
| Rule: `ai/rules/no-layering.md` | the in-image compile is DELETED, not kept beside a cache mount or behind a flag |
| Rule: `ai/rules/stale-comments.md` | all four code comments describing the old build shape are rewritten, not just the one in the file being edited |
| Rule: `ai/rules/principles.md` | one `buildZe`, not three; the `.dockerignore` negations are derived from the declarations rather than restated |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| no `Dockerfile.ze` compiles | `grep -l 'go build' $(git ls-files 'test/interop*/Dockerfile.ze')` prints nothing |
| one producer | `grep -rn 'GOOS: *"linux"' internal/le/interoplab/` names one file |
| the blocked scenario runs here | pasted output of `INTEROP_SCENARIO=as-path-prepend-two-octet-peer ./le integration interop` with a verdict |
| the measurement | pasted image-build wall time and `/usr/bin/time -v` peak RSS of the preflight build |
| the discrimination route still works | `rfc/discrimination/rfc1997.json` re-recorded, `./le rfc check` clean |
| the gates | `./le verify current mode full` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | the daemon architecture string reaches `GOARCH` in a `go build` argument list. It is matched against the known Go architecture spellings and refused otherwise, never interpolated into a shell |
| Command construction | the build runs through `exec.CommandContext` with an argument slice, as both existing `buildZe` copies do. No shell, so no `#nosec` beyond the one those copies already carry |
| Artifact provenance | the image now trusts a host-produced file. It is written by the preflight on every non-`NO_BUILD` run, into a git-ignored path, so a stale or foreign file is overwritten rather than shipped |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | fix in the phase that introduced it |
| `exec format error` in a container | R-1: the daemon architecture query, not the Dockerfile |
| `COPY` fails with file not found | R-3: a `.dockerignore` negation is missing for a declared staging path |
| A scenario that passed before now fails | compare the staged binary's tags against the deleted Dockerfile's tags first |
| `goversion` reports no carrier | A-4 was wrong; stop and report, do not weaken the gate |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The measured failure is memory, and that is what decides between the two
  repairs the journal proposed. A `RUN --mount=type=cache` speeds a WARM
  rebuild and leaves a Go compiler and linker running inside the container, so
  the peak stays where the kernel found it three times. Removing the compiler
  removes the peak, and it removes the 40 minutes as a side effect rather than
  the other way round.
- Two of the five labs already solved this, with `.gitignore` and
  `.dockerignore` entries already written for the shape. The work is mostly
  extending a shipped pattern, which is why it can also collapse two copies of
  the producer into one.
- The architecture question has a real answer rather than a convention. Nothing
  in `Docker.Build` names a platform, so the image takes the daemon's. Asking
  the daemon is one command and it makes the guarantee explicit; inheriting the
  host's `GOARCH`, which is what the two converted labs do today, is an
  assumption that holds on every machine anyone has used so far and produces
  `exec format error` on the first one where it does not.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Cross-compile on the host and copy the binary in | add `RUN --mount=type=cache` to the existing in-image build | the measured failure is an OOM kill during compilation, three times, one of them on an idle 31G host. A cache mount keeps the compiler in the container, so it does not address the measurement. It also introduces a Docker-managed cache that `./le scratch cache-clean` does not manage, beside the checkout `GOCACHE` that `gotoolchain` already puts inside the tree |
| Take `GOARCH` from the Docker daemon | inherit the host's, as `ipsec.buildZe` and `radius.buildZe` do today | `Docker.Build` passes no `--platform`, so the image's architecture is the daemon's and only the daemon can be asked. The host answer is right until a remote context or `DOCKER_DEFAULT_PLATFORM` makes it wrong, and the failure mode is an `exec format error` that says nothing about its cause |
| One producer in `internal/le/interoplab`, five callers | leave `ipsec.buildZe` and `radius.buildZe` alone and add a third copy | a third copy is the drift this repository has already paid for once: the untagged-build defect was fixed in `test/interop` first, in `test/interop-l2tp` on 2026-08-03, and the audit that found the second missed the third, which `test/interop-pppoe/Dockerfile.ze`'s own comment records |
| Convert all three compiling labs in one change | convert only `test/interop`, which is the one that was measured | the same defect in three files is one problem, and the unit fixed is the problem rather than the file that was opened (`ai/rules/completion.md`). Leaving two would also leave `TestZeDockerfilesCarryNoCompiler` unable to be written as a walk |
| Keep the working-tree route for interop discrimination | give the preflight a `-overlay` so the Go overlay route applies | the route works unchanged after this spec, and changing a conformance gate is a separate decision with its own evidence. Only the REASON printed beside it is corrected here |

## Known Limitations

- The image stops being buildable from the tree alone. `docker build -f
  test/interop/Dockerfile.ze .` on a clean checkout will fail at the `COPY`
  until the producing action has run. This is deliberate and it is the price of
  the change. What it costs the nightly is nothing: the five jobs in
  `.github/workflows/evidence-nightly.yml` invoke `./le`, which needs a Go
  toolchain to exist before it can build itself. What it costs a person is one
  confusing error, answered by the header comment each converted Dockerfile
  carries.
- `test/interop/Dockerfile.ze` leaves the Go-version gate's carrier set, since
  `goversion.copiesModule` only counts a stage that copies the module. The
  toolchain pin is not lost: the host build takes `GOTOOLCHAIN` from `go.mod`
  through `gotoolchain`, which pins a patch version where the
  `golang:1.27-alpine` tag pinned a minor. The gate keeps carriers
  (`docker/Dockerfile`, `docker/Dockerfile.lab`,
  `internal/le/interoplab/l2tp/radiusmock/Dockerfile`,
  `tools/kernel-builder/Dockerfile`, plus Go image literals), which A-4 states
  and AC-10 checks.
- The build context is still the repository root, because the staged binary
  lives under `test/`. The context TRANSFER is unchanged; only the compile is
  removed. Shrinking the context is separable work and is not in this spec.
- `ImageBuild.Timeout` keeps having no production caller, which the 2026-09-04
  journal row already recorded as the owner's decision to make. This spec does
  not revisit it.

## RFC Documentation (Scope: protocol)

N-A. Scope is `tooling`. No RFC-enforcing code is added or changed. The one RFC
artifact touched is the `RFC1997-Well-1` discrimination record, re-observed
under AC-7 because its carrier's build shape changed, with the requirement, the
claim, the producer and the citation unchanged.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-10 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)
