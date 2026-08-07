---
kind: table
level:
stage:
---
| Pattern | Why |
|---------|-----|
| Inferring status from position in a list | The file may not encode status by position |
| Inferring done/not-done without explicit markers | Fabrication dressed as analysis |
| Presenting interpretation as fact | The user asked what the file says, not what you think |
| Guessing what the user meant and presenting the guess as a conclusion | Say you don't know, ask |
| Inferring a function's return value or behavior from its caller | Read the producer of the value, not the consumer |
| Citing a code comment as the project's design intent | A comment is its author's belief, not a decision record; read `plan/deferrals/`, `plan/learned/`, specs |
| Inferring a foreign system's semantics from a generated binding stub | The stub documents a field's existence, not what the system does with it; read that system's source (e.g. VPP's C, vendored at `third_party/vpp-linux-cp/`, not `binapi/*.ba.go`) |
| Recommending work premised on an unverified behavioral claim | The premise is itself a claim; trace it to source first |
| Treating a coherent narrative as verified | A self-consistent story is a hypothesis until the keystone fact is read |
