# 767 — Command Tokenizer: No Escape Sequences

## Context

Both command tokenizers (`plugin/server/command.go:tokenize` and
`cli/model_commands.go:tokenizeCommand`) scanned every byte/rune for
backslash escape sequences (`\"`, `\\`). Commands never contain
backslashes in practice; the escape logic was defensive code with no
real use case.

## Key Decisions

### Backslash is a normal character

Rather than adding a fast path (`IndexByte` check to skip escape logic),
we removed escape handling entirely. Backslash is treated as a literal
character, the same as any other. This removes two branches per rune
from the hot loop and simplifies the tokenizer to: split on whitespace,
respect quote delimiters.

### joinTokensWithQuotes drops escape encoding

The round-trip encoder `joinTokensWithQuotes` no longer escapes
backslashes or quotes. Since `tokenizeCommand` uses `"` as a delimiter
(never as content), no token from the tokenizer can contain `"`, so
there is nothing to escape.

### Web tokenizer was already correct

`web/cli.go:tokenizeCommand` never had escape handling. The two
"full-featured" tokenizers now match its simplicity.

## Traps

- Removing escape support means there is no way to embed a literal `"`
  inside a quoted value. This is acceptable: Ze command values (peer
  names, descriptions, paths) have no use case for embedded quotes.

## Files

None recorded.
