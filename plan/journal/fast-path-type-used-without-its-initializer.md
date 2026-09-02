# Fast-path type used without its initializer

A type is chosen for a cost it only pays after one call. Adopting the type and
skipping that call leaves every measurement where it was, with a diff that looks
like the optimization landed. Ask which call arms the fast path, then check that
each declaration makes it.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-09-02 | - | `internal/core/textbuf` call sites | `textbuf.Buffer` is cheaper than `strings.Builder` only after `Reset`, `New` or `Get` points `b.b` at the struct's 128-byte inline array, and the type's own comment states that rule. Ten of the eleven files edited to remove dead `Reset` calls declare `var b textbuf.Buffer` and then write to it with no arming call, so the first write appends to a nil slice and allocates on the heap exactly as the `strings.Builder` it replaced did. `internal/le/job/registry.go` holds twelve such declarations by itself. Found while removing the `Reset` calls that commit 4404b0b50 made dead | not fixed. The repair is one `b.Reset()` after each declaration, and it allocates nothing. The owner scoped that removal work to the dead calls alone, so the missing arming calls stayed outside it |
