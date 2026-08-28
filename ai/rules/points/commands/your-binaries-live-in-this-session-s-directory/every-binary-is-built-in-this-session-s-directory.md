---
kind: note
level:
stage:
---
Native test actions build their binaries inside the current session's private
directory, under bare names. `internal/le/functional/binaries.go` resolves that
location and supplies the pair to the suite. A sibling session therefore cannot
overwrite the binary under test.
