---
kind: directive
level: MUST
stage:
---
- **A test MUST exist and MUST go red before the code that satisfies it is written.** A test that passes the moment it is written proves nothing about the code, so it MUST be strengthened until it fails.
- **Before implementation, acceptance criteria MUST name positive observable behavior and the permitted survivors with the reason they remain permitted.** If there are none, the criterion MUST state why. An absence-only assertion that passes when every input is dropped does not establish useful output. The harness-model design judgment in `ai/skills/ze-spec.md`, "Design admission judgment", checks this meaning; a native source-read marker or review artifact does not.
- **A red test means the code is wrong. MUST NOT weaken, skip, retarget, or delete a test to reach green, and MUST get an approval from an independent `ze-test-judge` agent before deleting or weakening any `*_test.go`, `.ci`, or `.et` content.**
- **A change MUST NOT be claimed done on unit tests alone.** A unit test proves the logic; only a `.ci` or `.et` proves the daemon exposes the behavior through the entry point an operator uses. Which suite runs which format, and what each one asserts, is `docs/functional-tests.md`.
- **A test that cannot run everywhere MUST carry `//go:build linux` on its file or `t.Skip` with a reason, and its assertion MUST NOT be widened to accept both outcomes.**
