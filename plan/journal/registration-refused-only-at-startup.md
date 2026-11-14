# registration-refused-only-at-startup

A check over what a package registers runs only when the program starts. A bad
registration therefore compiles, passes the package's tests, and is caught as a
startup failure of the shared program, which stops every session that uses it.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-11-14 | route-policy-1-gate-proof-and-forked-sysrib (walked into) | le help-line length check (`internal/le/le/root/describe.go`) | A help line over 60 characters, written twice in one day, made le panic at startup and stopped `./le` for every session sharing the checkout until it was cut | fixed: `leroot.TextFaults` holds the rule and answers every fault; startup names them all, and `TestEveryWordLeOffersSaysWhatItDoes` (`internal/le/describe_test.go`) fails with one error per fault over every registered root |
