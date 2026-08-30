---
kind: directive
level: MUST
stage:
---
**Any change to `//go:build linux` code MUST run this action.**

| Action | What it runs | When required |
|--------|--------------|---------------|
| `./le qemu all-tests` | Registered Linux integration packages in the runtime-kernel guest | Any change to `//go:build linux` code |
