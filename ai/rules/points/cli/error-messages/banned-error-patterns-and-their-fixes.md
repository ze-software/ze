---
kind: table
level:
stage:
---
| Pattern | Fix |
|---------|-----|
| `errors.New("failed")`, `"invalid input"`, `"unexpected error"` | Name what, the value, and the expected |
| Dropping the cause inside `if err != nil` (`return errors.New("parse failed")`) | Wrap: `fmt.Errorf("parse %s: %w", name, err)` |
| Reporting a value as invalid without printing it | Include `%q` of the offending value |
| Rewording a stable error phrase per call site | Keep one phrase so it stays greppable |
| Returning `nil`/skip when a check cannot run | Return an error; fail closed |
| A user-facing failure with no diagnostic code or remediation | Register a `doctor-*` code, make it `ze explain`-able |
