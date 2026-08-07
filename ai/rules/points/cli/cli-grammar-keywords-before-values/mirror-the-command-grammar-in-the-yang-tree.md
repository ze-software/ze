---
kind: note
level:
stage:
---
The YANG container nesting must mirror the corrected grammar. If the CLI path
changes from `show interface <name>` to `show interface name <name> detail`,
the YANG tree needs a `name` container under `interface`, then `detail`
under that selector. Filter forms like `show interface type <type>` need a
`type` container that consumes the typed selector value.
