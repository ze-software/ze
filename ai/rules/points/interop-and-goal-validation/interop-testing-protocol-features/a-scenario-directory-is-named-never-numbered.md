---
kind: directive
level: MUST
stage:
---
**An interop scenario directory MUST be NAMED and MUST NOT carry a numeric prefix, and a spec planning a future scenario MUST name it too** (owner directive, 2026-08-24). The directory name is the scenario's identity: `internal/le/interoplab.Discover` matches it exactly, the native `./le integration` action accepts it through its scenario selector, and specs, journal rows and code comments cite it.

A number adds nothing a name does not carry, and it goes stale in two ways a
name cannot: a deleted scenario leaves a hole no reader can tell from a
reservation, and a planned number is a reservation a second spec can take,
which nothing detects because neither directory exists yet. Before the prefixes
were removed from `test/interop-ipsec/`, several numbers were claimed by more
than one planned scenario, and some had already been built under a different
name. The count is left out because it moves with how you count, which is the
brittleness the numbering itself has.

Run order is `sorted(os.listdir(scenarios_dir))` and no scenario depends on it:
each gets its own `setup()`, `run_check()` and `teardown()` in that loop, so
they are independent by construction and the number encoded a sequence that was
never a dependency.
