# Local Go Tests, Fuzzing, and gomu

Use this page for the local Go test layer: named `_test.go` cases, race runs, coverage, fuzz targets, and gomu mutation testing. These checks are one system. They all ask whether package-level behavior is proved well enough before the slower gates run.

<!-- source: ../main/internal/le/testunit/actions.go -- unit test actions -->
<!-- source: ../main/internal/le/fuzz/actions.go -- fuzz actions -->
<!-- source: ../main/.gomuignore -- gomu exclusions -->
<!-- source: ../main/internal/le/mutation/actions.go -- mutation report actions -->
<!-- source: ../main/internal/le/mutation/combine.go -- mutation report combination -->
<!-- source: ../main/internal/le/mutation/history.go -- mutation history -->

## The local test loop

Start with a normal Go test. It names the behavior and fixes the expected result. If the input space is too large for a few named cases, add a fuzz target for the same behavior. If the code is covered but the assertion may still be weak, run gomu and see whether a deliberate code change survives.

<table>
<thead><tr><th>Mode</th><th>Question</th><th>Command</th></tr></thead>
<tbody>
<tr><td>Example test</td><td>Does this named input produce the exact expected behavior?</td><td><code>go test -race -run TestName ./path/...</code></td></tr>
<tr><td>Fuzz target</td><td>Does the same rule hold for generated inputs and saved corpus entries?</td><td><code>FUZZ=FuzzParseNLRI PKG=./internal/component/bgp/wire/ TIME=30s ./le fuzz run</code></td></tr>
<tr><td>gomu mutation run</td><td>Would the tests fail if the implementation made a small wrong decision?</td><td><code>go run github.com/sivchari/gomu/cmd/gomu run --incremental --base-branch=main --fail-on-gate=false</code></td></tr>
<tr><td>Race run</td><td>Does the behavior still hold when goroutines are scheduled differently?</td><td><code>./le test-unit</code></td></tr>
<tr><td>Coverage report</td><td>Which branches ran without a strong assertion attached?</td><td><code>go test -coverprofile coverage.out ./path/...</code></td></tr>
</tbody>
</table>

Fuzzing without a clear rule is just random input. gomu without real assertions only proves that code was executed. The normal example test gives both tools something precise to extend.

## Example tests

A normal unit test is the right tool when the behavior sits inside one package or a small group of production types. Good targets are wire encoders, parsers, state machines, validation helpers, route selection, command formatting, and error paths. If the behavior only exists after a daemon starts, a browser renders, or the Linux kernel answers, use `.ci`, `.wb`, `.et`, or QEMU instead.

<table>
<thead><tr><th>Scope</th><th>Command</th><th>When to use it</th></tr></thead>
<tbody>
<tr><td>One test</td><td><code>go test -race -run TestName ./path/...</code></td><td>Fast edit loop for one named behavior.</td></tr>
<tr><td>BGP group</td><td><code>./le test-unit bgp</code></td><td>Wire, FSM, peer, and BGP component changes.</td></tr>
<tr><td>Core group</td><td><code>./le test-unit core</code></td><td>Core libraries and shared infrastructure.</td></tr>
<tr><td>Plugin group</td><td><code>./le test-unit plugins</code></td><td>Runtime plugin logic and plugin boundaries.</td></tr>
<tr><td>Config group</td><td><code>./le test-unit config</code></td><td>YANG, config parsing, validation, and rendering.</td></tr>
<tr><td>CLI group</td><td><code>./le test-unit cli</code></td><td>Command parsing and user-visible formatting.</td></tr>
<tr><td>All unit groups</td><td><code>./le test-unit</code></td><td>Local unit gate.</td></tr>
</tbody>
</table>

## Fuzz targets are still tests

A Go fuzz target is a test function with generated inputs. It should start from useful seed cases, call the same parser or decoder a normal unit test would call, and assert a stable rule. For Ze, good fuzz targets are BGP attributes, communities, capabilities, AS paths, L2TP control packets, TACACS packets, and other parsers that must survive malformed external input.

```bash
./le fuzz run
FUZZ=FuzzParseNLRI PKG=./internal/component/bgp/wire/ TIME=30s ./le fuzz run
```

Keep the target deterministic and small. Be strict about accepted errors and round trips. When fuzzing finds a crash or semantic bug, keep the corpus entry. That saved input becomes the named regression case that explains the failure.

## gomu checks assertion strength

gomu is the mutation-testing tool Ze uses to test the tests. It changes Go code in small ways and reruns the same test suite. If the tests fail, the mutation is killed. If the tests still pass, the mutation survived. A survived mutation usually means the changed code was equivalent or the test did not check the decision tightly enough.

```bash
go run github.com/sivchari/gomu/cmd/gomu run --incremental --base-branch=main --fail-on-gate=false
go run github.com/sivchari/gomu/cmd/gomu run --incremental=false --fail-on-gate=false ./internal/core/textbuf/
go run github.com/sivchari/gomu/cmd/gomu run --incremental=false --fail-on-gate=false
./le mutation record-history report mutation-report.json
```

This complements fuzzing. Fuzzing changes the inputs and keeps the implementation fixed. gomu changes the implementation and keeps the tests fixed. Together they show whether a test is broad enough and sharp enough.

<table>
<thead><tr><th>gomu result</th><th>Meaning</th><th>Response</th></tr></thead>
<tbody>
<tr><td>Killed</td><td>The test suite noticed the changed behavior.</td><td>No action needed.</td></tr>
<tr><td>Survived</td><td>The tests still passed after a code mutation.</td><td>Add a stronger assertion, add a functional test, or classify the mutation as equivalent.</td></tr>
<tr><td>Timed out</td><td>The package or test is too slow for the current mutation settings.</td><td>Narrow the package or skip mutation where it does not add signal.</td></tr>
</tbody>
</table>

gomu runs through `go run`, so no separate install is needed. `.gomuignore` excludes paths where mutation testing is noisy or not useful. Full mutation runs are slower than unit tests and advisory in release evidence, but a survived mutation in changed code deserves a real decision.

## Choosing the proof

<table>
<thead><tr><th>Change</th><th>First proof</th><th>Strengthen it with</th></tr></thead>
<tbody>
<tr><td>Pure parser or encoder</td><td>Normal test with exact input and output.</td><td>Fuzz target for malformed inputs and gomu for assertion strength.</td></tr>
<tr><td>State machine or route decision</td><td>Normal test for transition or selected result.</td><td>Race run if goroutines are involved.</td></tr>
<tr><td>Error handling</td><td>Normal test that asserts the error shape.</td><td>gomu if changing a condition could silently pass.</td></tr>
<tr><td>Malformed external input</td><td>Fuzz target seeded with known examples.</td><td>Corpus regression when a failure is found.</td></tr>
<tr><td>Process or UI behavior</td><td>Functional transcript.</td><td>Normal tests only for helper logic behind the surface.</td></tr>
</tbody>
</table>
