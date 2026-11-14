# Ze Go Style: state that goes stale

The detail behind "State that goes stale" in
[the Go style core](../ze-go-style.md), which states each rule in bold. Each
bullet below continues the core's bullet in the same position.

Most defects come from a gap in time or in space between where a value is
checked and where it is used.

1. The same holds across files: a test or a tool that needs a rule the runner
   applies calls the runner's function, and does not write its own copy.
2. The proof is a mark written after the run reached its verdict, or an
   identity of the content. Size, modification time and "holds an entry" are
   not proof: a killed run leaves a partial cache, and `cp -p` keeps the old
   time on new bytes.
3. The constructor takes an out pointer, so the value is written where it lives
   and no intermediate copy exists.
4. Do not declare it early or keep it alive after its last read.
5. Each extra dimension is a branch at every call site.
6. A buffer written short and sent long leaks what the last user left in it,
   and it breaks deterministic test output.
7. A blank line before the acquisition and after the `defer` makes a missing
   `defer` visible in a diff.
8. An index plus one is a count, and a count times the unit is a size. Put the
   unit in the name: `octets`, `entries` and `slots` answer what `n` leaves
   open. When a division can round, write which way.
