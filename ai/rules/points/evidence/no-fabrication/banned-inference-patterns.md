---
kind: directive
level: MUST NOT
stage:
---
**These inferences MUST NOT be made, or presented as a finding:**

| Pattern | Why |
|---------|-----|
| Inferring status from position in a list | The file might not encode status by position |
| Inferring done or not-done without an explicit marker | Fabrication dressed as analysis |
| Presenting interpretation as fact | The user asked what the file says, not what you think |
| Guessing what the user meant and presenting the guess as a conclusion | Say you do not know, and ask |
| Inferring a function's return value or behavior from its caller | Read the producer of the value, not the consumer |
| Citing a code comment as the project's design intent | A comment is its author's belief, not a decision record. Read `plan/deferrals/`, `plan/journal/`, and the specs |
| Citing a commit message, or a number in one, as the state of HEAD | It records the moment it was written. A measurement in a body is usually the PRE-fix figure, and a spec row can still read `NOT MET` after the fix landed |
| Inferring a foreign system's semantics from a generated binding stub | The stub documents a field's existence, not what the system does with it. Read that system's source (VPP's C is vendored at `third_party/vpp-linux-cp/`, not `binapi/*.ba.go`) |
| Recommending work premised on an unverified behavioral claim | The premise is itself a claim; trace it to source first |
| Treating a coherent narrative as verified | A self-consistent story is a hypothesis until the keystone fact is read |
