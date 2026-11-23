# Code Quality

Ze uses different tests for package logic, running daemons, and behavior that depends on Linux or another router. This page explains which test fits a change and how a failure points to the next check.

**Quality model**

A package test can check a parser without starting Ze, but it cannot show that the daemon applies a new configuration or sends a route to another router. Functional transcripts exercise those operations through the running program. QEMU provides the Linux environment for kernel-dependent tests, while interoperability tests exchange messages with other implementations.

Fuzzing extends the inputs a Go test tries, and mutation testing with gomu checks whether its assertions detect a deliberate code change. A parser failure can then become a saved regression case. If a mutation survives, the reviewer must establish whether the changed code behaves equivalently or whether the test missed a defect.

- **Local:** Go tests, race checks, coverage, generated-input tests, and gomu run before checks that need more time or resources.
- **Process:** `.ci`, `.wb`, and `.et` files run Ze the way an operator, peer, browser, or editor would use it.
- **Linux:** QEMU runs the same tests where netlink, nftables, eBPF, PPP, and namespaces exist.
- **Release:** Interop, deployment, performance, and chaos runs check behavior that package tests cannot establish.

## The flow

**Step 1**

### Test package behavior

Start with a small Go test that names the case and expected result. Use fuzzing when one or two examples cannot cover the input space. Use gomu when the code is covered but the assertion may still be too loose.

**Step 2**

### Test visible behavior

When the behavior crosses a process boundary, use a functional transcript. `.ci` drives commands, peers, files, HTTP, and daemons. `.wb` drives the browser. `.et` drives the editor.

**Step 3**

### Test Linux behavior

Tests that depend on the Linux kernel run on Linux. From macOS, a functional file marked `option=needs-linux` can run in QEMU using Ze's kernel and system image.

**Step 4**

### Run release checks

Release checks include peer daemons, deployment tests, performance measurements, and chaos scenarios. Together these exercise interactions and failure conditions beyond the fixtures used in package tests.

## Local tests are one layer

### Example tests

A normal Go test gives the case a name and states the exact result. It checks a parser rule, encoder rule, state transition, validation, or error message.

### Fuzz targets

A fuzz target is a Go test with generated inputs. It starts from known examples, tries variations, and saves any input that causes a crash or an incorrect result so it can be tested again.

### gomu mutation checks

gomu changes production code in small ways and runs the same tests. If the tests still pass, the changed code was equivalent or the assertions did not check that behavior tightly enough.

## Which guide to open

| What you are testing | Use this | Guide |
| --- | --- | --- |
| Package behavior, parser rules, encoders, races, fuzzable inputs, or weak assertions | `go test`, race, fuzz, coverage, gomu mutation checks | [Local Go tests, fuzzing, and gomu](https://ze-software.net/quality/unit-fuzz-mutation/) |
| Daemon startup, CLI output, BGP wire output, HTTP, syslog, files, or process exits | `.ci` under `test/<suite>/` | [Functional `.ci` tests](https://ze-software.net/quality/functional-ci/) |
| Rendered web UI behavior or interactive editor behavior | `.wb` under `test/web/`, `.et` under `test/editor/` | [Browser and editor tests](https://ze-software.net/quality/browser-editor/) |
| Linux kernel behavior, compatibility with other routers, deployment, or release evidence | QEMU, Docker interoperability tests, deployment scripts, performance checks | [QEMU, interop, and release evidence](https://ze-software.net/quality/qemu-interop-release/) |
| A failing verify run that needs a clear rerun command | Verify stages, failure groups, trace output, debug logs | [Verify and debugging workflow](https://ze-software.net/quality/verify-debugging/) |
| Whether the suite would catch a regression | What assertions check, tests that cannot fail, tests nothing runs, checks against declining coverage, and results over time | [Testing health](https://ze-software.net/quality/health/) |
| RFC requirement coverage, known omissions, or checks on AI changes to tests | `./le rfc check`, links from requirements to tests, agreement with published status, and current reviews | [RFC compliance](https://ze-software.net/quality/rfc-compliance/) |

## Commands that matter

### Edit loop

Use one focused command while changing code.

```
go test -race -run TestName ./internal/component/config/...
FUZZ=FuzzParseNLRI PKG=./internal/component/bgp/wire/ TIME=30s ./le test fuzz run
go run github.com/sivchari/gomu/cmd/gomu run --incremental --base-branch=main --fail-on-gate=false
./le test bgp plugin 42 -v
```

### Release verification

GitHub CI runs full verification on pushed commits. The maintainer can also schedule a release check; day-to-day changes use the tests they reach.

```
./le verify current
./le verify current fast
./le repo check
```

### Linux and release

Use these checks when the behavior depends on Linux, another router, or the release environment.

```
./le test qemu netns
./le test qemu run command '...' keep-alive
./le test integration interop
./le verify evidence release-candidate
```

## How a failure becomes useful

`./le verify current` writes per-stage logs under `tmp/`, groups related failures, and prints the rerun commands. Its lock prevents overlapping verification runs. The functional runner adds per-step traces for `.ci`, `.wb`, and `.et` files. BGP expectations decode wire messages before showing differences, so a failed UPDATE is reported as protocol fields that can be compared.

### The rule

A failure needs a test that can observe the required behavior. After correcting the cause, rerun that test while preserving its assertions. Wider verification belongs in CI and the release check.

## File formats

Ze currently has three functional formats. Use `.ci` for process, protocol, command, HTTP, syslog, file, and daemon behavior. Use `.wb` for rendered browser behavior. Use `.et` for the headless interactive editor. There is no `.wt` parser in the current tree; notes using that extension should be read as web tests only if a new parser is later added.
