---
kind: directive
level: MUST
stage:
---
- **A test, a sweep, an `le` tool or a lab MUST call the one function, or read the one file, that declares a runner, harness or lab rule, and MUST NOT carry a second copy of it.** A copy of the runner's placeholder expansion, a per-scenario copy of a lab-wide daemon setting, or a fixture classification kept inside one test file drifts from its source while every gate stays green. When a second reader needs a rule that lives in a test file, move the rule to a package both import. When every scenario needs a setting, put it in the lab-wide file and give it a test that refuses a scenario copy.
