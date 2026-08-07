---
kind: note
level:
stage:
---
Benchmarks and fuzz targets are deliberately exempt: a benchmark measures, and a
fuzz target delegates its oracle to the engine. Raising a floor is forbidden:
`make ze-test-health` only lowers one, so a regression cannot be laundered into
the baseline by regenerating.
