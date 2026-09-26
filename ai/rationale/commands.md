# Commands rationale

## History moved from rule points (2026-09-26)

- `ai/rules/points/commands/directives/a-run-that-selected-nothing-exits-zero-so-read-what-ran.md`: Measured on 2026-09-08: `./le test stress-repro run suite "bgp ui --draft" test bgp-update-delay-command` answered "not reproduced", which reads as green, and had run nothing at all, because the ui suite is `./le test ui` and takes no `bgp` verb (`internal/le/test/functional/suites.go`, `Suites`).
