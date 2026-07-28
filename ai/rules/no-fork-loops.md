# No Fork Loops

**When:** writing a shell loop, or any Bash command that could fork one process per file
**Severity:** blocking

## Directives

Never write a shell for-loop that forks an external command per
iteration when a single invocation can process all inputs.

On macOS, each `fork+exec` costs ~4-5 ms. A loop over 400 files × one `grep`
per iteration = ~2 seconds of pure fork overhead before any real work. Add a
second command per iteration (pipe to `sed`, call `awk`) and it doubles. Nested
loops make it quadratic.

## Bad

```bash
for f in test/plugin/*.ci; do grep -n 'pattern' "$f"; done       # 400 forks
for f in *.go; do grep -l 'Foo' "$f" | xargs sed -n '1p'; done  # 800 forks
```

## Good

```bash
grep -rn 'pattern' test/plugin/ --include='*.ci'                 # 1 fork
grep -n 'pattern' test/plugin/*.ci                                # 1 fork (glob)
```

## When a loop is unavoidable

If the loop body genuinely needs per-file logic that a single command cannot
express, batch with `xargs` or `find -exec +` instead of per-file forks:

```bash
find test/plugin -name '*.ci' -exec grep -l 'pattern' {} +
```

## Scope

Applies to every `Bash` tool call and every shell script written for this
project. The rule complements `bash-output.md` (no pipes on expensive commands).
