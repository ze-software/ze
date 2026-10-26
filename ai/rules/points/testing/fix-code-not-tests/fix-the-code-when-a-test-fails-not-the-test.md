---
kind: directive
level: MUST
stage:
---
- **When a test fails, MUST show the observed failure, trace its cause to the producing code, fix that code, and rerun the existing failing test.** The test's expectations MUST NOT be weakened, simplified, or retargeted to match the code. When the mechanism underneath changes, the expectation stays and the replacement mechanism satisfies it.
- **Test data is covered too: a golden file, an expected output, a fixture, a `.ci` expectation MUST NOT be updated to turn a red run green without a `ze-test-judge` approval**, however plausible the new output looks.
- **If the test itself is wrong, MUST explain its exact defect and why its expectation is incorrect before changing or replacing it.** That explanation is what the `ze-test-judge` agent judges, and it MUST NOT replace the judgement.
- **Test and verification maintenance MUST be reported separately from product fixes.** Moving or replacing tests and refreshing snapshots MUST NOT be presented as fixing the product.
