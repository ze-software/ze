# Code Quality

Ze's quality work has one rule: when something fails, the output should show what broke and what to run next. This page follows that path from a small Go test to the full release evidence.

<!-- source: ../main/internal/le/testunit/actions.go -- unit and race actions -->
<!-- source: ../main/internal/le/functional/actions.go -- functional suite actions -->
<!-- source: ../main/docs/architecture/testing/ci-format.md -- .ci and .et formats -->
<!-- source: ../main/docs/architecture/testing/runner-architecture.md -- runner architecture and .wb format -->
<!-- source: ../main/internal/le/qemu/actions.go -- QEMU actions -->
<!-- source: ../main/internal/le/integration/gates.go -- integration and interop actions -->
<!-- source: ../main/internal/le/fuzz/actions.go -- fuzz actions -->
<!-- source: ../main/internal/le/mutation/actions.go -- mutation reporting actions -->
<!-- source: ../main/internal/le/evidence/actions.go -- release evidence -->
<!-- source: ../main/internal/le/verify/run.go -- staged verify runner -->
<!-- source: ../main/internal/le/verifylock/register.go -- shared verify lock -->
<!-- source: ../main/internal/le/rfc/actions.go -- RFC requirement gate -->
<!-- source: ../main/internal/le/hookruntime/writeedit.go -- RFC-tagged test edit guard -->
<div class="quality-hero">
  <div class="quality-hero-card">
    <span class="quality-hero-kicker">Quality model</span>
    <p>Ze uses several test layers because bugs show up in different places. Local Go tests check package behavior. Fuzz tests are Go tests that try many generated inputs. gomu changes the code and runs the same tests, which shows whether the assertions are strong. Functional transcripts check what an operator, peer, browser, or editor can see. QEMU and interop check the cases that need Linux or real peer daemons.</p>
    <p>The layers can overlap, but each one has a job. A parser bug should leave behind a Go test or fuzz corpus entry. A survived gomu mutation means either the changed code was equivalent or the test did not check the behavior tightly enough. A CLI or daemon bug should become a functional transcript. A kernel bug should run in QEMU.</p>
  </div>
  <div class="quality-meter" aria-label="Quality layers">
    <div class="quality-meter-row"><strong>Local</strong><span>Go tests, race runs, coverage, fuzz targets, and gomu run before the expensive gates.</span></div>
    <div class="quality-meter-row"><strong>Process</strong><span><code>.ci</code>, <code>.wb</code>, and <code>.et</code> files run Ze the way an operator, peer, browser, or editor would use it.</span></div>
    <div class="quality-meter-row"><strong>Linux</strong><span>QEMU runs the same tests where netlink, nftables, eBPF, PPP, and namespaces exist.</span></div>
    <div class="quality-meter-row"><strong>Release</strong><span>Interop, deployment, performance, chaos, and release evidence compose the slow proof.</span></div>
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
    <p>Linux behavior is tested on Linux, not guessed from macOS. A functional file can mark itself <code>option=needs-linux</code> and then run inside the QEMU Alpine image.</p>
  </article>
  <article class="quality-stage">
    <span class="quality-stage-number">4</span>
    <h3>Prove release behavior</h3>
    <p>Before broad evidence is claimed, Ze runs real peer daemons, deployment checks, performance gates, chaos scenarios, and release reports instead of trusting local fixtures.</p>
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
<tr><td>Whether the suite would actually catch a regression, not how large it is</td><td>Proof density, tests that cannot fail, tests nothing runs, ratchets, KPI history</td><td><a href="health/">Testing health</a></td></tr>
<tr><td>RFC requirement coverage, public gap disclosure, or AI agent test-change guards</td><td><code>./le rfc check</code>, RFC test tags, status-ledger agreement, audit freshness</td><td><a href="rfc-compliance/">RFC compliance gate</a></td></tr>
</tbody>
</table>

## Commands that matter

<div class="quality-command-grid">
  <article class="quality-command">
    <h3>Edit loop</h3>
    <p>Use one focused command while changing code.</p>
    <pre><code>go test -race -run TestName ./internal/component/config/...
FUZZ=FuzzParseNLRI PKG=./internal/component/bgp/wire/ TIME=30s ./le fuzz run
go run github.com/sivchari/gomu/cmd/gomu run --incremental --base-branch=main --fail-on-gate=false
bin/ze-test bgp plugin 42 -v</code></pre>
  </article>
  <article class="quality-command">
    <h3>Handoff gate</h3>
    <p>Use the shared gate before handing over normal work.</p>
    <pre><code>./le verify current mode full
./le verify current mode changed
./le repository check</code></pre>
  </article>
  <article class="quality-command">
    <h3>Linux and release</h3>
    <p>Use the wider gates only when the behavior needs Linux, real peers, or release evidence.</p>
    <pre><code>./le qemu netns-test
./le qemu run command '...' keep-alive
./le integration interop
./le evidence release-candidate</code></pre>
  </article>
</div>

## How a failure becomes useful

`./le verify current mode full` is more than a command wrapper. It takes a lock so two heavy runs do not corrupt each other, writes per-stage logs under `tmp/`, groups related failures, and prints the rerun commands. The functional runner adds per-step traces for `.ci`, `.wb`, and `.et` files. BGP expectations decode wire messages before showing differences, so a failed UPDATE is reported as protocol structure instead of a long hex string.

<div class="quality-panel">
  <h3>The rule</h3>
  <p>Do not hide a failure with a skip or a loose assertion. Move the proof to the layer that can see the real behavior, rerun the narrow command, then rerun the gate that should have caught the regression.</p>
</div>

## File formats

Ze currently has three functional formats. Use `.ci` for process, protocol, command, HTTP, syslog, file, and daemon behavior. Use `.wb` for rendered browser behavior. Use `.et` for the headless interactive editor. There is no `.wt` parser in the current tree; notes using that extension should be read as web tests only if a new parser is later added.
