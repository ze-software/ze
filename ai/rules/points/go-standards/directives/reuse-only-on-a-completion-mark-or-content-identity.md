---
kind: directive
level: MUST
stage:
---
**A cache, a built artifact or a copied file MUST be reused only when a completion mark written after a successful run, or an identity of its content, proves it complete and current. Size, modification time and "holds an entry" MUST NOT decide reuse.** A run killed partway leaves what it wrote so far, so the mark MUST be removed before the run starts and written back only when the run reaches its verdict. A file replaced with the same size and time (`cp -p`, `rsync -t`) passes a metadata key. Where a content check costs what the rebuild costs, rebuild every time.
