# A splitter ignores the quoting its tokenizer honors

One line is read by two parsers. The tokenizer honors quotes, so a quoted value
is one argument whatever it holds. A splitter that runs before it cuts at a
separator character without asking whether the character is quoted, so a value
that carries the separator is torn apart before the tokenizer ever sees it.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-11-12 | - (found fixing the plugin dispatch pipe chain) | `ParsePipe`, `parsePipeOps` (`command/pipe.go`), reached by SSH `execMiddleware`, the web CLI and the `ze cli` TUI | The splitter cut at every pipe character, quoted or not, while the dispatcher's `tokenize` honors double quotes, so a quoted argument holding a pipe was split as a chain and its tail refused or applied as an operator | fixed: one grammar reads the command and the chain (`ScanInvocation`; 3d7157b2be), so a quoted pipe stays one argument; `TestChainGrammar`, `test/ui/pipe-quoted-value-is-an-argument.ci` |
