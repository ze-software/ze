---
kind: directive
level: MUST
stage:
---
**A journal row MUST reach git.** It is written to be read by a later session,
and an uncommitted row lives in one shared working tree and dies at the next
clean, stash or checkout. Writing it is not the obligation; landing it is.
Commit it with the work that found it, so a reader meets the row beside the diff
and needs no archaeology.

**`/ze-close` sweeps the rows when a spec closes, and most sessions do not close
a spec.** A session that ends any other way MUST commit its own rows first.

**The trap that strands them:** a row naming a spec makes `commit_helper.py`
read the commit as that spec's CLOSURE and demand the Review Gate artifact. The
obvious answer is to drop the rows "for now", and "for now" is the rest of the
session. A rows-only commit that adds no learned summary and removes no spec
closes nothing, and `--review-override` carries that reason: state in it what
the commit does NOT do, so the escape stays auditable.
