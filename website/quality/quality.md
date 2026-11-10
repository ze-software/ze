# Code Quality

Ze uses different tests for package logic, running daemons, and behavior that depends on Linux or another router. This page explains which test fits a change and how a failure points to the next check.

<!-- source: ../main/internal/le/testunit/actions.go -- unit and race actions -->
<!-- source: ../main/internal/le/functional/actions.go -- functional suite actions -->
<!-- source: ../main/docs/architecture/testing/ci-format.md -- .ci and .et formats -->
<!-- source: ../main/docs/architecture/testing/runner-architecture.md -- runner architecture and .wb format -->
<!-- source: ../main/internal/le/qemu/actions.go -- QEMU actions -->
<!-- source: ../main/internal/le/integration/gates.go -- integration and interop actions -->
<!-- source: ../main/internal/le/fuzz/actions.go -- fuzz actions -->
<!-- source: ../main/internal/le/mutation/actions.go -- mutation reporting actions -->
<!-- source: ../main/internal/le/evidence/actions.go -- release evidence -->
<!-- source: ../main/internal/le/verify/engine/run.go -- staged verify runner -->
<!-- source: ../main/internal/le/verify/lock/register.go -- shared verify lock -->
<!-- source: ../main/internal/le/rfc/actions.go -- RFC requirement gate -->
<!-- source: ../main/internal/le/hookruntime/writeedit.go -- RFC-tagged test edit guard -->
<div class="quality-hero">
  <div class="quality-hero-card">
    <span class="quality-hero-kicker concept-evidence">Quality model</span>
    <p>A package test can check a parser without starting Ze, but it cannot show that the daemon applies a new configuration or sends a route to another router. Functional transcripts exercise those operations through the running program. QEMU provides the Linux environment for kernel-dependent tests, while interoperability tests exchange messages with other implementations.</p>
    <p>Fuzzing extends the inputs a Go test tries, and mutation testing with gomu checks whether its assertions detect a deliberate code change. A parser failure can then become a saved regression case. If a mutation survives, the reviewer must establish whether the changed code behaves equivalently or whether the test missed a defect.</p>
  </div>
  <div class="quality-meter" aria-label="Quality layers">
    <div class="quality-meter-row"><strong class="concept-evidence">Local</strong><span>Go tests, race runs, coverage, fuzz targets, and gomu run before the expensive gates.</span></div>
    <div class="quality-meter-row"><strong class="concept-evidence">Process</strong><span><code>.ci</code>, <code>.wb</code>, and <code>.et</code> files run Ze the way an operator, peer, browser, or editor would use it.</span></div>
    <div class="quality-meter-row"><strong class="concept-evidence">Linux</strong><span>QEMU runs the same tests where netlink, nftables, eBPF, PPP, and namespaces exist.</span></div>
    <div class="quality-meter-row"><strong class="concept-evidence">Release</strong><span>Interop, deployment, performance, and chaos runs check behavior that package tests cannot establish.</span></div>
  </div>
</div>

## The flow

<div class="quality-flow">
  <article class="quality-stage">
    <span class="quality-stage-number">1</span>
    <h3>Prove package behavior</h3>
    <p>Start with a small Go test that names the case and expected result. Use fuzzing when one or two examples cannot cover the input space. Use gomu when the code is covered but the assertion may still be too loose.</p>
  </article>
  <article class="quality-stage">
    <span class="quality-stage-number">2</span>
    <h3>Prove visible behavior</h3>
    <p>When the behavior crosses a process boundary, use a functional transcript. <code>.ci</code> drives commands, peers, files, HTTP, and daemons. <code>.wb</code> drives the browser. <code>.et</code> drives the editor.</p>
  </article>
  <article class="quality-stage">
    <span class="quality-stage-number">3</span>
    <h3>Prove Linux behavior</h3>
    <p>Tests that depend on the Linux kernel run on Linux. From macOS, a functional file marked <code>option=needs-linux</code> can run in QEMU using Ze's kernel and system image.</p>
  </article>
  <article class="quality-stage">
    <span class="quality-stage-number">4</span>
    <h3>Prove release behavior</h3>
    <p>Release checks include peer daemons, deployment tests, performance measurements, and chaos scenarios. Together these exercise interactions and failure conditions beyond the fixtures used in package tests.</p>
  </article>
</div>

## Local tests are one layer

<div class="quality-routes">
  <article class="quality-route">
    <h3>Example tests</h3>
    <p>A normal Go test gives the case a name and states the exact result. It is the first proof for a parser rule, encoder rule, state transition, validation path, or error shape.</p>
  </article>
  <article class="quality-route">
    <h3>Fuzz targets</h3>
    <p>A fuzz target is a Go test with generated inputs. It starts from useful seed cases, tries more shapes, and keeps any crash or semantic failure as a corpus entry.</p>
  </article>
  <article class="quality-route">
    <h3>gomu mutation checks</h3>
    <p>gomu changes production code in small ways and runs the same tests. If the tests still pass, the changed code was equivalent or the assertions did not check that behavior tightly enough.</p>
  </article>
</div>

## Which guide to open

<table>
<thead><tr><th>What you are testing</th><th>Use this</th><th>Guide</th></tr></thead>
<tbody>
<tr><td>Package behavior, parser rules, encoders, races, fuzzable inputs, or weak assertions</td><td><code>go test</code>, race, fuzz, coverage, gomu mutation checks</td><td><a href="unit-fuzz-mutation/">Local Go tests, fuzzing, and gomu</a></td></tr>
<tr><td>Daemon startup, CLI output, BGP wire output, HTTP, syslog, files, or process exits</td><td><code>.ci</code> under <code>test/&lt;suite&gt;/</code></td><td><a href="functional-ci/">Functional <code>.ci</code> tests</a></td></tr>
<tr><td>Rendered web UI behavior or interactive editor behavior</td><td><code>.wb</code> under <code>test/web/</code>, <code>.et</code> under <code>test/editor/</code></td><td><a href="browser-editor/">Browser and editor tests</a></td></tr>
<tr><td>Linux kernel behavior, real peer compatibility, deployment, or release evidence</td><td>QEMU, Docker interop, deployment scripts, perf gates</td><td><a href="qemu-interop-release/">QEMU, interop, and release evidence</a></td></tr>
<tr><td>A failing verify run that needs a clear rerun command</td><td>Verify stages, failure groups, trace output, debug logs</td><td><a href="verify-debugging/">Verify and debugging workflow</a></td></tr>
<tr><td>Whether the suite would catch a regression</td><td>Proof density, tests that cannot fail, tests nothing runs, ratchets, KPI history</td><td><a href="health/">Testing health</a></td></tr>
<tr><td>RFC requirement coverage, public gap disclosure, or AI agent test-change guards</td><td><code>./le rfc check</code>, RFC test tags, status-ledger agreement, audit freshness</td><td><a href="rfc-compliance/">RFC compliance gate</a></td></tr>
</tbody>
</table>

## Commands that matter

<div class="quality-command-grid">
  <article class="quality-command">
    <h3>Edit loop</h3>
    <p>Use one focused command while changing code.</p>
    <pre><code>go test -race -run TestName ./internal/component/config/...
FUZZ=FuzzParseNLRI PKG=./internal/component/bgp/wire/ TIME=30s ./le test fuzz run
go run github.com/sivchari/gomu/cmd/gomu run --incremental --base-branch=main --fail-on-gate=false
./le test bgp plugin 42 -v</code></pre>
  </article>
  <article class="quality-command">
    <h3>Release verification</h3>
    <p>GitHub CI runs full verification on pushed commits. The maintainer can also schedule a release check; day-to-day changes use the tests they reach.</p>
    <pre><code>./le verify current
./le verify current fast
./le repo check</code></pre>
  </article>
  <article class="quality-command">
    <h3>Linux and release</h3>
    <p>Use the wider gates only when the behavior needs Linux, real peers, or release evidence.</p>
    <pre><code>./le test qemu netns
./le test qemu run command '...' keep-alive
./le test integration interop
./le verify evidence release-candidate</code></pre>
  </article>
</div>

## How a failure becomes useful

`./le verify current` writes per-stage logs under `tmp/`, groups related failures, and prints the rerun commands. Its lock prevents overlapping verification runs. The functional runner adds per-step traces for `.ci`, `.wb`, and `.et` files. BGP expectations decode wire messages before showing differences, so a failed UPDATE is reported as protocol fields that can be compared.

<div class="quality-panel">
  <h3>The rule</h3>
  <p>A failure needs a test that can observe the required behavior. After correcting the cause, rerun that test while preserving its assertions. Wider verification belongs in CI and the release check.</p>
</div>

## File formats

Ze currently has three functional formats. Use `.ci` for process, protocol, command, HTTP, syslog, file, and daemon behavior. Use `.wb` for rendered browser behavior. Use `.et` for the headless interactive editor. There is no `.wt` parser in the current tree; notes using that extension should be read as web tests only if a new parser is later added.
