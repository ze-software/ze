# OS-Specific Tests

A test that cannot run on every OS MUST either carry a build tag
(`//go:build linux`) on its file, or skip (`t.Skip`) with a reason on
the OSes where it cannot run. Never weaken the assertion to accept both
outcomes.

| Situation | Do |
|-----------|-----|
| Whole file is OS-specific | `//go:build linux` on the file |
| One test in a mixed file | `if runtime.GOOS != "linux" { t.Skip(...) }` at the top of that test |
| `.ci` / `.et` test | Split or gate in the runner; do not land an always-failing .ci |

A darwin `FAIL` caused by a `_other.go` stub returning `ErrUnsupported`
is a test-setup bug, not a real failure. Keep the failure list
meaningful.
