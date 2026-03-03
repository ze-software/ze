# 131 — CI Format Cleanup

## Objective

Design a fully key=value `.ci` format (`<action>=<type>:<key>=<value>:...`) replacing the mixed-delimiter old format, and reduce functional test output verbosity to a single summary line.

## Decisions

Mechanical design spec — format was defined but implementation was superseded by the more comprehensive `tmpfs-format` spec (135). The new format specified here became the basis for the unified format.

## Patterns

- Sequence semantics: same `seq` within a `conn` = unordered (any matching message accepted); different `seq` = strict ordering within connection.
- `expect=bgp` and `expect=json` with same `conn`/`seq` represent the same message in different representations (wire bytes vs. decoded JSON).

## Gotchas

None — this spec was a design step, the implementation was done in spec 135.

## Files

- This spec defined the format; actual implementation landed in `internal/test/runner/record.go` and `internal/test/peer/peer.go` via spec 135.
