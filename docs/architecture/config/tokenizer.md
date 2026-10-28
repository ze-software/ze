# Configuration Tokenizer

**Source:** ExaBGP `configuration/core/format.py`, `configuration/core/parser.py`
**Purpose:** Document configuration file tokenization

---

## Overview

ExaBGP uses a custom tokenizer for configuration files. The format is similar to JUNOS configuration but with some differences.

---

## Token Types

### Delimiters

| Character | Meaning |
|-----------|---------|
| `{` | Section start |
| `}` | Section end |
| `;` | Statement end |
| `,` | List separator |
| `[` | List start |
| `]` | List end |

### Strings

| Character | Meaning |
|-----------|---------|
| `"` | Double-quoted string (preserves spaces) |
| `'` | Single-quoted string (preserves spaces) |
| `#` | Comment to the end of the line. ExaBGP drops it; Ze keeps it as trivia for the parser to attach (see "Comments" below) |

### Whitespace

- Space (` `) and tab (`\t`) separate tokens
- Newlines are significant only for line continuation
- Multiple spaces collapse to single separator

---

## Line Continuation

Lines ending with `\` continue to the next line:

```
peer peer-east { \
    remote { ip 192.168.1.2; as 65002; } \
    router-id 1.1.1.1; \
}
```

---

## Escape Sequences

In quoted strings:

| Sequence | Meaning |
|----------|---------|
| `\b` | Backspace |
| `\f` | Form feed |
| `\n` | Newline |
| `\r` | Carriage return |
| `\t` | Tab |
| `\uXXXX` | Unicode codepoint |
| `\\` | Literal backslash |

---

## Tokenization Algorithm

```python
def tokens(stream):
    spaces = [' ', '\t', '\r', '\n']
    strings = ['"', "'"]
    syntax = [',', '[', ']']
    eol = [';', '{', '}']
    comment = ['#']

    for line in stream:
        line = unescape(line)
        parsed = []
        quoted = ''
        word = ''

        for char in line:
            if char in comment and not quoted:
                if word:
                    parsed.append(word)
                    word = ''
                break  # Ignore rest of line

            elif char in eol and not quoted:
                if word:
                    parsed.append(word)
                    word = ''
                parsed.append(char)
                yield parsed  # End of statement
                parsed = []

            elif char in syntax and not quoted:
                if word:
                    parsed.append(word)
                    word = ''
                parsed.append(char)

            elif char in spaces and not quoted:
                if word:
                    parsed.append(word)
                    word = ''

            elif char in strings:
                if quoted == char:
                    quoted = ''
                    parsed.append(word)
                    word = ''
                elif not quoted:
                    quoted = char

            else:
                word += char

        # Error if unclosed quote or incomplete line
        if word or parsed:
            raise ValueError('incomplete line')
```

---

## Pre-processing

Before tokenization, the `formated()` function normalizes input:

```python
def formated(line):
    # Strip and convert tabs to spaces
    line = line.strip().replace('\t', ' ')

    # Add spaces around brackets/parens
    line = line.replace(']', ' ]').replace('[', '[ ')
    line = line.replace(')', ' )').replace('(', '( ')
    line = line.replace(',', ' , ')

    # Collapse multiple spaces
    while '  ' in line:
        line = line.replace('  ', ' ')

    return line
```

---

## Tokenizer Class

```python
class Tokeniser:
    def __init__(self):
        self.next = deque()      # Lookahead buffer
        self.tokens = []         # Current line tokens
        self.generator = iter([])
        self.consumed = 0        # Tokens consumed

    def replenish(self, content):
        """Set new token list for parsing."""
        self.next.clear()
        self.tokens = content
        self.generator = iter(content)
        self.consumed = 0

    def peek(self):
        """Look at next token without consuming."""
        if self.next:
            return self.next[0]
        try:
            peaked = next(self.generator)
            self.next.append(peaked)
            return peaked
        except StopIteration:
            return ''

    def __call__(self):
        """Get next token, consuming it."""
        if self.next:
            self.consumed += 1
            return self.next.popleft()
        try:
            tok = next(self.generator)
            self.consumed += 1
            return tok
        except StopIteration:
            return ''
```

---

## Parser Class

```python
class Parser:
    def __init__(self, scope, error):
        self.tokeniser = Tokeniser()
        self.line = []
        self.end = ''  # Last token: ';', '{', or '}'

    def set_file(self, filename):
        """Parse from file."""
        with open(filename) as f:
            # Handle line continuation with \
            # Tokenize each line
            pass

    def set_text(self, data):
        """Parse from string."""
        pass

    def __call__(self):
        """Get next line of tokens."""
        self.line = next(self._tokens)
        self.end = self.line[-1]  # ';', '{', or '}'
        self.tokeniser.replenish(self.line[:-1])  # Exclude delimiter
        return self.line
```

---

## Example Tokenization

### Input

```
peer peer-east {
    remote {
        ip 192.168.1.2;
        as 65002;
    }
    router-id 1.1.1.1;
    local-as 65001;
    family {
        ipv4/unicast;
    }
}
```

### Output Tokens

```python
['peer', 'peer-east', '{']
['remote', '{']
['ip', '192.168.1.2', ';']
['as', '65002', ';']
['}']
['router-id', '1.1.1.1', ';']
['local-as', '65001', ';']
['family', '{']
['ipv4/unicast', ';']
['}']
['}']
```

---

## Ze Implementation Notes

<!-- source: internal/component/config/tokenizer.go -- tokenizer, token, comment, scan, readComment, peekTrivia, takeConsumed -->
<!-- source: internal/component/config/parser_comments.go -- commentScope, headCount -->
<!-- source: internal/component/config/tree_comments.go -- blockComments, statementComments -->

### Go Tokenizer

Ze's tokenizer is a single-pass scanner over the whole input, not ExaBGP's
line-based one. It produces word, quoted string, `{`, `}`, `[`, `]`, `(`, `)`
and `;` tokens with their line and column, and the parser reads them with one
token of lookahead (`peek`, `next`).

**Automatic semicolon insertion.** After a value token (a word, a quoted
string, `]` or `)`), a newline, the end of the input or a `}` produces a
synthetic `;`. A comment ends its line, so it ends a statement the same way a
newline does.

### Comments

A `#` comment runs to the end of its line. The tokenizer does not discard it:
it keeps the text after `#` (the CR of a CRLF line ending excluded) as trivia on
the next real token, never on a synthetic `;`. Each comment records its line,
whether it follows a token on that token's line, and whether a blank line comes
before it. Each token records whether a blank line separates it from the
comment or token before it.

The parser attaches each comment to a statement on the parsed `Tree`
(`parser_comments.go`):

| Comment | Belongs to |
|---------|------------|
| On its own line | The next statement in the same block, as a leading comment |
| After a statement on its line, after `{`, after `}` | That statement, that block's opening line, or that block statement, as a trailing comment |
| Before a block's first statement, detached from it by a blank line | The block's head |
| On the first line of the file, starting `# ze-schema: ` | The file's head, blank line or not, so the stamp stays on line 1 |
| After a block's last statement | The block, as closing comments (at the root, the end of the file) |
| Inside a statement (after `[` in a bracket leaf-list) | That statement, as a leading comment |

A leaf statement's comments sit on its parent `Tree` by name, an
`inactive: <leaf-list> <member>` line's by name and member, and a container's or
list entry's on its own `Tree`. A statement written twice keeps the comments of
both, in source order. `Clone`, `MergeContainer` and `RenameListEntry` carry the
comments with the statement; removing a leaf removes its comments. Comments are
not configuration: `ToMap`, the diff and the set printers ignore them.

---

**Last Updated:** 2026-10-28
