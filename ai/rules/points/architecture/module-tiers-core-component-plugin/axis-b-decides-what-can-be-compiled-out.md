---
kind: note
level:
stage:
---
Axis B also decides whether a feature can be **compiled out** of the binary. A feature is compile-out-able exactly when nothing always-compiled depends on it: it is reached ONLY through build-tag-gated registration. A direct functional `import` from always-on (untagged) code pins the package into every binary and defeats the compile-out; only a blank/gated registration import can be dropped by a build tag.
