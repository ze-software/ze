# Code Quality

Ze's quality work has one rule: when something fails, the output should show what broke and what to run next. This page follows that path from a small Go test to the full release evidence.

**Quality model**

Ze uses several test layers because bugs show up in different places. Local Go tests check package behavior. Fuzz tests are Go tests that try many generated inputs. gomu changes the code and runs the same tests, which shows whether the assertions are strong. Functional transcripts check what an operator, peer, browser, or editor can see. QEMU and interop check the cases that need Linux or real peer daemons.

The layers can overlap, but each one has a job. A parser bug should leave behind a Go test or fuzz corpus entry. A survived gomu mutation means either the changed code was equivalent or the test did not check the behavior tightly enough. A CLI or daemon bug should become a functional transcript. A kernel bug should run in QEMU.

- **Local:** Go tests, race runs, coverage, fuzz targets, and gomu run before the expensive gates.
- **Process:** `.ci`, `.wb`, and `.et` files run Ze the way an operator, peer, browser, or editor would use it.
- **Linux:** QEMU runs the same tests where netlink, nftables, eBPF, PPP, and namespaces exist.
- **Release:** Interop, deployment, performance, chaos, and release evidence compose the slow proof.

## The flow

**Step 1**

### Prove package behavior

Start with a small Go test that names the case and expected result. Use fuzzing when one or two examples cannot cover the input space. Use gomu when the code is covered but the assertion may still be too loose.

**Step 2**

### Prove visible behavior

When the behavior crosses a process boundary, use a functional transcript. `.ci` drives commands, peers, files, HTTP, and daemons. `.wb` drives the browser. `.et` drives the editor.

**Step 3**

### Prove Linux behavior

Linux behavior is tested on Linux, not guessed from macOS. A functional file can mark itself `option=needs-linux` and then run inside the QEMU Alpine image.

**Step 4**

### Prove release behavior

Before broad evidence is claimed, Ze runs real peer daemons, deployment checks, performance gates, chaos scenarios, and release reports instead of trusting local fixtures.

## Local tests are one layer

### Example tests

A normal Go test gives the case a name and states the exact result. It is the first proof for a parser rule, encoder rule, state transition, validation path, or error shape.

### Fuzz targets

A fuzz target is a Go test with generated inputs. It starts from useful seed cases, tries more shapes, and keeps any crash or semantic failure as a corpus entry.

### gomu mutation checks

gomu changes production code in small ways and runs the same tests. If the tests still pass, the changed code was equivalent or the assertions did not check that behavior tightly enough.

## Which guide to open

| What you are testing | Use this | Guide |
| --- | --- | --- |
| Package behavior, parser rules, encoders, races, fuzzable inputs, or weak assertions | `go test`, race, fuzz, coverage, gomu mutation checks | [Local Go tests, fuzzing, and gomu](https://ze-software.net/quality/unit-fuzz-mutation/) |
| Daemon startup, CLI output, BGP wire output, HTTP, syslog, files, or process exits | `.ci` under `test/<suite>/` | [Functional `.ci` tests](https://ze-software.net/quality/functional-ci/) |
| Rendered web UI behavior or interactive editor behavior | `.wb` under `test/web/`, `.et` under `test/editor/` | [Browser and editor tests](https://ze-software.net/quality/browser-editor/) |
| Linux kernel behavior, real peer compatibility, deployment, or release evidence | QEMU, Docker interop, deployment scripts, perf gates | [QEMU, interop, and release evidence](https://ze-software.net/quality/qemu-interop-release/) |
| A failing verify run that needs a clear rerun command | Verify stages, failure groups, trace output, debug logs | [Verify and debugging workflow](https://ze-software.net/quality/verify-debugging/) |
| Whether the suite would actually catch a regression, not how large it is | Proof density, tests that cannot fail, tests nothing runs, ratchets, KPI history | [Testing health](https://ze-software.net/quality/health/) |
| RFC requirement coverage, public gap disclosure, or AI agent test-change guards | `make ze-rfc-check`, RFC test tags, status-ledger agreement, audit freshness | [RFC compliance gate](https://ze-software.net/quality/rfc-compliance/) |

## Commands that matter

### Edit loop

Use one focused command while changing code.

```
go test -race -run TestName ./internal/component/config/...
make ze-fuzz-test-one FUZZ=FuzzParseNLRI PKG=./internal/component/bgp/wire/ TIME=30s
make ze-mutation-test-changed
bin/ze-test bgp plugin 42 -v
```

### Handoff gate

Use the shared gate before handing over normal work.

```
make ze-precommit-verify
make ze-precommit-verify-changed
make ze-repository-check
```

### Linux and release

Use the wider gates only when the behavior needs Linux, real peers, or release evidence.

```
make ze-qemu-needs-linux-test
make ze-qemu-debug RUN='...'
make ze-interop-test
make ze-evidence-release-verify
```

## How a failure becomes useful

`make ze-precommit-verify` is more than a command wrapper. It takes a lock so two heavy runs do not corrupt each other, writes per-stage logs under `tmp/`, groups related failures, and prints the rerun commands. The functional runner adds per-step traces for `.ci`, `.wb`, and `.et` files. BGP expectations decode wire messages before showing differences, so a failed UPDATE is reported as protocol structure instead of a long hex string.

### The rule

Do not hide a failure with a skip or a loose assertion. Move the proof to the layer that can see the real behavior, rerun the narrow command, then rerun the gate that should have caught the regression.

## File formats

Ze currently has three functional formats. Use `.ci` for process, protocol, command, HTTP, syslog, file, and daemon behavior. Use `.wb` for rendered browser behavior. Use `.et` for the headless interactive editor. There is no `.wt` parser in the current tree; notes using that extension should be read as web tests only if a new parser is later added.
