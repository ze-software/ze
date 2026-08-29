---
kind: directive
level: MUST
stage:
---
A tight loop MAY be used while iterating, but the full pass MUST be the one that
reports the result, because it is the only form that covers the whole population.

- `./le qemu netns-test suites <comma-separated-suites>` runs the selected kernel-dependent functional suites for a tight iteration.
- `./le qemu all-tests` runs every functional suite, the unit pass, and every registered integration package inside the prepared VM.
- `./le qemu all-tests only needs-linux` starts the same suites and narrows each one to the `.ci` tests marked `option=needs-linux`. The unit, installer and integration phases stay whole, and the report names the population it covered.
