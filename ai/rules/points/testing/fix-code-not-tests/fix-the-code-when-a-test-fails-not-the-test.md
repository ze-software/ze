---
kind: directive
level: MUST
stage:
---
- **When a test fails, the CODE MUST be fixed, and the test's expectations MUST NOT be weakened, simplified, or retargeted to match it.** When the mechanism underneath changes, the expectation stays and the replacement mechanism satisfies it.
- **Test data is covered too: a golden file, an expected output, a fixture, a `.ci` expectation MUST NOT be updated to turn a red run green without the user's explicit approval**, however plausible the new output looks.
