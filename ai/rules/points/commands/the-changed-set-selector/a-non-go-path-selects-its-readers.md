---
kind: note
level:
stage:
---
A non-Go path seeds the packages whose tests READ it, so a `.ci`, a rules page or
the `Makefile` selects the tooling packages rather than nothing. `--paths-from`
asks about a path list you supply, which is how to see the answer for a change
you have not made yet.
