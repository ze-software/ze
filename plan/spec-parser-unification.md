# Spec: Parser Unification

## Goal

Unify API and config parsing for `update` commands. Same syntax, shared parser, different tokenizers.

## Prerequisite

Depends on unified syntax from `spec-announce-family-first.md`.

## Current State

| | Config | API |
|---|---|---|
| Tokenizer | `pkg/config/tokenizer.go` | inline in `command.go` |
| Tokens | `Token{Type, Value, Line, Col}` | `[]string` |
| Parser | recursive descent | linear scan |
| Validation | schema-driven | per-handler |

## Target Architecture

```
Input (API or Config)
        ↓
Format-specific Tokenizer
        ↓
Common Token Stream: []Token
        ↓
Shared Parser: parseUpdate()
        ↓
UpdateCommand struct
```

## Token Interface

```go
// pkg/parse/token.go

type TokenType int

const (
    TokenEOF TokenType = iota
    TokenWord      // unquoted word
    TokenString    // quoted string
    TokenLBrace    // {
    TokenRBrace    // }
    TokenLBracket  // [
    TokenRBracket  // ]
    TokenSemicolon // ;
)

type Token struct {
    Type  TokenType
    Value string
    Line  int
    Col   int
}

type Tokenizer interface {
    Next() Token
    Peek() Token
}
```

## Tokenizer Modes

### API Tokenizer

- Splits on whitespace
- Newline = end of command
- No `{ }` or `;` tokens (implicit structure)
- Quotes handled (strips quotes, processes escapes)

```go
// Input: peer 10.0.0.1 update text attr next-hop 10.0.0.1 nlri ipv4/unicast add 1.0.0.0/24
// Tokens: [peer, 10.0.0.1, update, text, attr, next-hop, 10.0.0.1, nlri, ipv4/unicast, add, 1.0.0.0/24, EOF]
```

### Config Tokenizer

- Splits on whitespace + delimiters
- `{ }` and `;` are tokens
- Block structure explicit
- Quotes handled (same as API)

```go
// Input: update { text { attr { next-hop 10.0.0.1; nlri ipv4/unicast add 1.0.0.0/24; } } }
// Tokens: [update, {, text, {, attr, {, next-hop, 10.0.0.1, ;, nlri, ipv4/unicast, add, 1.0.0.0/24, ;, }, }, }]
```

## Shared Parser

```go
// pkg/parse/update.go

type UpdateCommand struct {
    Encoding   Encoding        // text, hex, b64, cbor
    Attributes AttributeData   // parsed or raw bytes
    NLRISections []NLRISection
    Watchdog   string
}

type Encoding int

const (
    EncodingText Encoding = iota
    EncodingHex
    EncodingB64
    EncodingCBOR
)

type AttributeData struct {
    Encoding Encoding
    // For text encoding
    Parsed *ParsedAttributes
    // For wire encodings
    Raw []byte
}

type NLRISection struct {
    Family   Family
    Add      []NLRI
    Del      []NLRI
    Override *ParsedAttributes // text mode only
}

// Parser consumes tokens, doesn't care about source
func ParseUpdate(tok Tokenizer) (*UpdateCommand, error) {
    cmd := &UpdateCommand{}

    // Expect: update <encoding> attr ...
    if err := cmd.parseEncoding(tok); err != nil {
        return nil, err
    }
    if err := cmd.parseAttr(tok); err != nil {
        return nil, err
    }
    if err := cmd.parseNLRISections(tok); err != nil {
        return nil, err
    }
    if err := cmd.parseWatchdog(tok); err != nil {
        return nil, err
    }

    return cmd, nil
}
```

## Parser Functions

### parseEncoding

```go
func (cmd *UpdateCommand) parseEncoding(tok Tokenizer) error {
    t := tok.Next()
    switch t.Value {
    case "text":
        cmd.Encoding = EncodingText
    case "hex":
        cmd.Encoding = EncodingHex
    case "b64":
        cmd.Encoding = EncodingB64
    case "cbor":
        cmd.Encoding = EncodingCBOR
    default:
        return fmt.Errorf("line %d: expected encoding (text|hex|b64|cbor), got %q", t.Line, t.Value)
    }
    return nil
}
```

### parseAttr

```go
func (cmd *UpdateCommand) parseAttr(tok Tokenizer) error {
    t := tok.Next()
    if t.Value != "attr" {
        return fmt.Errorf("line %d: expected 'attr', got %q", t.Line, t.Value)
    }

    switch cmd.Encoding {
    case EncodingText:
        return cmd.parseTextAttributes(tok)
    default:
        return cmd.parseWireAttributes(tok)
    }
}
```

### parseTextAttributes

```go
func (cmd *UpdateCommand) parseTextAttributes(tok Tokenizer) error {
    attrs := &ParsedAttributes{}

    for {
        t := tok.Peek()
        if t.Type == TokenEOF || t.Value == "nlri" || t.Value == "watchdog" {
            break
        }

        // Skip block delimiters in config mode
        if t.Type == TokenLBrace || t.Type == TokenSemicolon {
            tok.Next()
            continue
        }

        if err := parseAttribute(tok, attrs); err != nil {
            return err
        }
    }

    cmd.Attributes.Parsed = attrs
    return nil
}

func parseAttribute(tok Tokenizer, attrs *ParsedAttributes) error {
    key := tok.Next()

    switch key.Value {
    case "next-hop":
        v := tok.Next()
        addr, err := netip.ParseAddr(v.Value)
        if err != nil {
            return fmt.Errorf("line %d: invalid next-hop: %w", v.Line, err)
        }
        attrs.NextHop = addr

    case "origin":
        v := tok.Next()
        switch v.Value {
        case "igp":
            attrs.Origin = OriginIGP
        case "egp":
            attrs.Origin = OriginEGP
        case "incomplete":
            attrs.Origin = OriginIncomplete
        default:
            return fmt.Errorf("line %d: invalid origin: %s", v.Line, v.Value)
        }

    case "med":
        v := tok.Next()
        n, err := strconv.ParseUint(v.Value, 10, 32)
        if err != nil {
            return fmt.Errorf("line %d: invalid med: %w", v.Line, err)
        }
        med := uint32(n)
        attrs.MED = &med

    case "local-preference":
        v := tok.Next()
        n, err := strconv.ParseUint(v.Value, 10, 32)
        if err != nil {
            return fmt.Errorf("line %d: invalid local-preference: %w", v.Line, err)
        }
        lp := uint32(n)
        attrs.LocalPref = &lp

    case "community":
        return parseCommunities(tok, attrs)

    case "large-community":
        return parseLargeCommunities(tok, attrs)

    case "extended-community":
        return parseExtendedCommunities(tok, attrs)

    case "as-path":
        return parseASPath(tok, attrs)

    case "rd":
        return parseRD(tok, attrs)

    case "label":
        return parseLabel(tok, attrs)

    default:
        return fmt.Errorf("line %d: unknown attribute: %s", key.Line, key.Value)
    }

    return nil
}
```

### parseNLRISections

```go
func (cmd *UpdateCommand) parseNLRISections(tok Tokenizer) error {
    for {
        t := tok.Peek()
        if t.Type == TokenEOF || t.Value == "watchdog" {
            break
        }
        if t.Type == TokenRBrace {
            tok.Next() // consume in config mode
            continue
        }
        if t.Value != "nlri" {
            return fmt.Errorf("line %d: expected 'nlri', got %q", t.Line, t.Value)
        }
        tok.Next() // consume "nlri"

        section, err := parseNLRISection(tok, cmd.Encoding)
        if err != nil {
            return err
        }
        cmd.NLRISections = append(cmd.NLRISections, section)
    }

    if len(cmd.NLRISections) == 0 {
        return fmt.Errorf("missing nlri section")
    }

    return nil
}

func parseNLRISection(tok Tokenizer, enc Encoding) (NLRISection, error) {
    section := NLRISection{}

    // Parse family
    t := tok.Next()
    family, err := ParseFamily(t.Value)
    if err != nil {
        return section, fmt.Errorf("line %d: %w", t.Line, err)
    }
    section.Family = family

    // Parse add/del groups
    for {
        t := tok.Peek()
        if t.Type == TokenEOF || t.Value == "nlri" || t.Value == "watchdog" {
            break
        }
        if t.Type == TokenRBrace || t.Type == TokenSemicolon {
            tok.Next()
            break
        }

        switch t.Value {
        case "add":
            tok.Next()
            nlris, override, err := parseNLRIList(tok, enc, family)
            if err != nil {
                return section, err
            }
            section.Add = append(section.Add, nlris...)
            if override != nil {
                section.Override = override
            }

        case "del":
            tok.Next()
            nlris, _, err := parseNLRIList(tok, enc, family)
            if err != nil {
                return section, err
            }
            section.Del = append(section.Del, nlris...)

        default:
            return section, fmt.Errorf("line %d: expected 'add' or 'del', got %q", t.Line, t.Value)
        }
    }

    return section, nil
}
```

## Integration

### API Integration

```go
// pkg/api/route.go

func (d *Dispatcher) handleUpdate(ctx *CommandContext, args []string) (*Response, error) {
    // Wrap args in API tokenizer
    tok := parse.NewAPITokenizer(args)

    cmd, err := parse.ParseUpdate(tok)
    if err != nil {
        return nil, err
    }

    return d.executeUpdate(ctx, cmd)
}
```

### Config Integration

```go
// pkg/config/parser.go

func (p *Parser) parseUpdateBlock() (*parse.UpdateCommand, error) {
    // Config tokenizer already exists, wrap it
    tok := parse.NewConfigTokenizerAdapter(p.tok)

    return parse.ParseUpdate(tok)
}
```

## File Structure

```
pkg/parse/
├── token.go          # Token types, Tokenizer interface
├── api_tokenizer.go  # API tokenizer ([]string → tokens)
├── config_adapter.go # Adapter for existing config tokenizer
├── update.go         # ParseUpdate() and helpers
├── attributes.go     # Attribute parsing (text mode)
├── nlri.go           # NLRI parsing
├── family.go         # Family parsing/validation
├── wire.go           # Wire bytes parsing (hex/b64/cbor)
└── update_test.go    # Tests
```

## Implementation Steps

### Phase 1: Token Foundation

1. [ ] Create `pkg/parse/token.go` - types and interface
2. [ ] Create `pkg/parse/api_tokenizer.go` - `[]string` → tokens
3. [ ] Test: API tokenizer produces correct tokens

### Phase 2: Shared Parser (TDD)

4. [ ] Write test for `ParseUpdate()` with text encoding → FAIL
5. [ ] Implement `parseEncoding()` → partial
6. [ ] Implement `parseTextAttributes()` → partial
7. [ ] Implement `parseNLRISections()` → PASS
8. [ ] Write test for wire encodings → FAIL
9. [ ] Implement `parseWireAttributes()` → PASS

### Phase 3: API Integration

10. [ ] Update `handleUpdate()` to use shared parser
11. [ ] Test: API commands work with new parser
12. [ ] Remove old parsing code

### Phase 4: Config Adapter

13. [ ] Create `pkg/parse/config_adapter.go`
14. [ ] Update config parser to use shared `ParseUpdate()`
15. [ ] Test: Config parsing works with new parser

### Phase 5: Cleanup

16. [ ] Remove duplicate parsing logic
17. [ ] `make test && make lint && make functional`

## Error Messages

Unified error format with line/col:

```
line 1: expected encoding (text|hex|b64|cbor), got "foo"
line 1: invalid next-hop: invalid IP address
line 1: expected 'attr', got "nlri"
line 1: unknown attribute: foobar
line 1: prefix family mismatch: 2001:db8::/32 is IPv6, expected IPv4
```

## Testing Strategy

```go
func TestParseUpdate_TextEncoding(t *testing.T) {
    // VALIDATES: text encoding parses attributes and NLRI correctly
    // PREVENTS: regression in attribute parsing after unification

    args := []string{"text", "attr", "next-hop", "10.0.0.1", "med", "100",
                     "nlri", "ipv4/unicast", "add", "1.0.0.0/24"}
    tok := NewAPITokenizer(args)

    cmd, err := ParseUpdate(tok)
    require.NoError(t, err)
    require.Equal(t, EncodingText, cmd.Encoding)
    require.Equal(t, netip.MustParseAddr("10.0.0.1"), cmd.Attributes.Parsed.NextHop)
    // ...
}

func TestParseUpdate_HexEncoding(t *testing.T) {
    // VALIDATES: hex wire bytes decode correctly
    // PREVENTS: hex parsing errors

    args := []string{"hex", "attr", "400101", "nlri", "ipv4/unicast", "add", "18010100"}
    tok := NewAPITokenizer(args)

    cmd, err := ParseUpdate(tok)
    require.NoError(t, err)
    require.Equal(t, EncodingHex, cmd.Encoding)
    require.Equal(t, []byte{0x40, 0x01, 0x01}, cmd.Attributes.Raw)
    // ...
}
```

## Migration

1. New parser in `pkg/parse/` - no changes to existing code
2. API switches to new parser (behind flag if needed)
3. Config switches to new parser
4. Old parsing code removed

## Checklist

- [ ] Token types defined
- [ ] API tokenizer works
- [ ] Config adapter works
- [ ] `ParseUpdate()` handles text encoding
- [ ] `ParseUpdate()` handles wire encodings
- [ ] API integrated
- [ ] Config integrated
- [ ] Old code removed
- [ ] `make test` passes
- [ ] `make lint` passes
- [ ] `make functional` passes

---

**Created:** 2025-01-04
**Depends on:** `spec-announce-family-first.md`
