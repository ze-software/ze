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
| `#` | Comment (rest of line ignored) |

### Whitespace

- Space (` `) and tab (`\t`) separate tokens
- Newlines are significant only for line continuation
- Multiple spaces collapse to single separator

---

## Line Continuation

Lines ending with `\` continue to the next line:

```
peer 192.168.1.2 { \
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
peer 192.168.1.2 {
    router-id 1.1.1.1;
    local-as 65001;
    family {
        ipv4/unicast;
    }
}
```

### Output Tokens

```python
['neighbor', '192.168.1.2', '{']
['router-id', '1.1.1.1', ';']
['local-as', '65001', ';']
['family', '{']
['ipv4', 'unicast', ';']
['}']
['}']
```

---

## Ze Implementation Notes

### Go Tokenizer

```go
type Tokenizer struct {
    tokens   []string
    next     []string  // lookahead
    consumed int
}

func (t *Tokenizer) Peek() string {
    if len(t.next) > 0 {
        return t.next[0]
    }
    // ...
}

func (t *Tokenizer) Next() string {
    t.consumed++
    // ...
}
```

### State Machine

```go
type TokenState int

const (
    StateNormal TokenState = iota
    StateQuoted
    StateComment
)

func tokenize(line string) ([]string, error) {
    var tokens []string
    var current strings.Builder
    state := StateNormal
    quoteChar := byte(0)

    for i := 0; i < len(line); i++ {
        c := line[i]
        switch state {
        case StateNormal:
            // Handle normal characters
        case StateQuoted:
            // Handle quoted strings
        case StateComment:
            // Ignore rest of line
        }
    }
    return tokens, nil
}
```

---

**Last Updated:** 2025-12-19
