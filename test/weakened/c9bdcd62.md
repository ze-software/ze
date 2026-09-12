# Test weakenings this commit accepts

| Test | Reason |
|------|--------|
| TestDispatchRefusesUnknownAndHandlesHelp | One assertion left it: `Dispatch("le", nil)` answered 1, and a bare invocation is no longer a refusal. The root now answers the manifest as a payload on stdout with exit 0 (AC-1 of `plan/spec-le-publishes-its-command-surface.md`), so the behavior the line asserted does not exist to assert. The new contract is a NEW test rather than this one repurposed: `TestBareRootAnswersTheManifestAsAPayload` drives `Dispatch("le", nil)` through the same entry point and asserts the code, the stream and the bytes. The two facts this test is named for, an unknown command answering 1 and a bare help word answering 0, both stay. |
