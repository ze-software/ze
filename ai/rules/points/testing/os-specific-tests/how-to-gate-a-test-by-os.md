---
kind: directive
level: MUST
stage:
---
**A test that cannot run on every OS MUST be gated by the row that fits it, and
its assertion MUST NOT be weakened to accept both outcomes.**

| Situation | Do |
|-----------|-----|
| Whole file is OS-specific | `//go:build linux` on the file |
| One test in a mixed file | `if runtime.GOOS != "linux" { t.Skip(...) }` at the top of that test |
| `.ci` / `.et` test | Split or gate in the runner; do not land an always-failing .ci |
