---
kind: note
level:
stage:
---
A derived parent needs to pass its `builddir` to `gok`; an instance with no
`builddir` discards every pin
(`vendor/github.com/gokrazy/tools/packer/gotool.go`, `getPkg`/`getIncomplete`).
