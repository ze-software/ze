---
kind: directive
level: MUST NOT
stage:
---
**An unexpected module version in `gokrazy/modcache/` MUST NOT be dismissed as cache noise.** A `github.com/ze-software/ze@v0.0.0-...` directory, or an off-pin copy of a builddir-pinned module, means a build resolved over the network instead of through the pins, so the version it built is not the version this repository chose. You MUST find the path that prepared that instance without the builddir and fix it, rather than deleting the directory. What each finding means, and what growth is expected instead, is `docs/architecture/appliance/gokrazy-build-pins.md`.
