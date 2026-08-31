# Local Go Tests, Fuzzing, and gomu

Use this page for the local Go test layer: named `_test.go` cases, race runs, coverage, fuzz targets, and gomu mutation testing. These checks are one system. They all ask whether package-level behavior is proved well enough before the slower gates run.

## The local test loop

Start with a normal Go test. It names the behavior and fixes the expected result. If the input space is too large for a few named cases, add a fuzz target for the same behavior. If the code is covered but the assertion may still be weak, run gomu and see whether a deliberate code change survives.

| Mode | Question | Command |
| --- | --- | --- |
| Example test | Does this named input produce the exact expected behavior? | `go test -race -run TestName ./path/...` |
| Fuzz target | Does the same rule hold for generated inputs and saved corpus entries? | `FUZZ=FuzzParseNLRI PKG=./internal/component/bgp/wire/ TIME=30s ./le fuzz run` |
| gomu mutation run | Would the tests fail if the implementation made a small wrong decision? | `go run github.com/sivchari/gomu/cmd/gomu run --incremental --base-branch=main --fail-on-gate=false` |
| Race run | Does the behavior still hold when goroutines are scheduled differently? | `./le test-unit` |
| Coverage report | Which branches ran without a strong assertion attached? | `go test -coverprofile coverage.out ./path/...` |

Fuzzing without a clear rule is just random input. gomu without real assertions only proves that code was executed. The normal example test gives both tools something precise to extend.

## Example tests

A normal unit test is the right tool when the behavior sits inside one package or a small group of production types. Good targets are wire encoders, parsers, state machines, validation helpers, route selection, command formatting, and error paths. If the behavior only exists after a daemon starts, a browser renders, or the Linux kernel answers, use `.ci`, `.wb`, `.et`, or QEMU instead.

| Scope | Command | When to use it |
| --- | --- | --- |
| One test | `go test -race -run TestName ./path/...` | Fast edit loop for one named behavior. |
| BGP group | `./le test-unit bgp` | Wire, FSM, peer, and BGP component changes. |
| Core group | `./le test-unit core` | Core libraries and shared infrastructure. |
| Plugin group | `./le test-unit plugins` | Runtime plugin logic and plugin boundaries. |
| Config group | `./le test-unit config` | YANG, config parsing, validation, and rendering. |
| CLI group | `./le test-unit cli` | Command parsing and user-visible formatting. |
| All unit groups | `./le test-unit` | Local unit gate. |

## Fuzz targets are still tests

A Go fuzz target is a test function with generated inputs. It should start from useful seed cases, call the same parser or decoder a normal unit test would call, and assert a stable rule. For Ze, good fuzz targets are BGP attributes, communities, capabilities, AS paths, L2TP control packets, TACACS packets, and other parsers that must survive malformed external input.

```
./le fuzz run
FUZZ=FuzzParseNLRI PKG=./internal/component/bgp/wire/ TIME=30s ./le fuzz run
```

Keep the target deterministic and small. Be strict about accepted errors and round trips. When fuzzing finds a crash or semantic bug, keep the corpus entry. That saved input becomes the named regression case that explains the failure.

## gomu checks assertion strength

gomu is the mutation-testing tool Ze uses to test the tests. It changes Go code in small ways and reruns the same test suite. If the tests fail, the mutation is killed. If the tests still pass, the mutation survived. A survived mutation usually means the changed code was equivalent or the test did not check the decision tightly enough.

```
go run github.com/sivchari/gomu/cmd/gomu run --incremental --base-branch=main --fail-on-gate=false
go run github.com/sivchari/gomu/cmd/gomu run --incremental=false --fail-on-gate=false ./internal/core/textbuf/
go run github.com/sivchari/gomu/cmd/gomu run --incremental=false --fail-on-gate=false
./le mutation record-history report mutation-report.json
```

This complements fuzzing. Fuzzing changes the inputs and keeps the implementation fixed. gomu changes the implementation and keeps the tests fixed. Together they show whether a test is broad enough and sharp enough.

| gomu result | Meaning | Response |
| --- | --- | --- |
| Killed | The test suite noticed the changed behavior. | No action needed. |
| Survived | The tests still passed after a code mutation. | Add a stronger assertion, add a functional test, or classify the mutation as equivalent. |
| Timed out | The package or test is too slow for the current mutation settings. | Narrow the package or skip mutation where it does not add signal. |

gomu runs through `go run`, so no separate install is needed. `.gomuignore` excludes paths where mutation testing is noisy or not useful. Full mutation runs are slower than unit tests and advisory in release evidence, but a survived mutation in changed code deserves a real decision.

## Choosing the proof

| Change | First proof | Strengthen it with |
| --- | --- | --- |
| Pure parser or encoder | Normal test with exact input and output. | Fuzz target for malformed inputs and gomu for assertion strength. |
| State machine or route decision | Normal test for transition or selected result. | Race run if goroutines are involved. |
| Error handling | Normal test that asserts the error shape. | gomu if changing a condition could silently pass. |
| Malformed external input | Fuzz target seeded with known examples. | Corpus regression when a failure is found. |
| Process or UI behavior | Functional transcript. | Normal tests only for helper logic behind the surface. |
