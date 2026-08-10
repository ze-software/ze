---
kind: directive
level: MUST
stage:
---
**You MUST NOT `rm -rf gokrazy/modcache`.** 60 tracked files live inside it (the gokrazy init source, whitelisted by `gokrazy/modcache/.gitignore`). You MUST delete named `@version` directories plus their `cache/download/<module>/@v/<version>.*` files, and confirm with `git status --porcelain gokrazy/` that nothing tracked moved.
