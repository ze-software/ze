> **Pre-Alpha.** This page is auto-generated from `ze help command --json`.

# Command Catalog

## Contents

- [clear](#clear) (1)
- [show](#show) (2)

## clear

| Command | Mode | Description |
|---------|------|-------------|
| `clear beta` | offline | Clear \| beta |

## show

| Command | Mode | Description |
|---------|------|-------------|
| `show alpha` | daemon | Alpha |
| `show zeta` | read-only | Zeta \| first |

### `show zeta`

Zeta \| first
Zeta details

Mode: read-only | Wire: `show_zeta`
Answer shape: `tab`
Address fields: `address`

**Requires backend:** `rib`
**Task support:** stream

**Arguments:**

| Name | Type | Required | Values |
|------|------|----------|--------|
| `family` | `enum` | yes | `blue`, `red` |

**Pipes:**
Always: `json`, `save`

When the answer has rows: `match` -- this command has not declared its answer shape, so each of these applies to an answer that carries rows and is refused by name over one that does not.

While the command keeps answering: `log`

Local process only: `save` -- daemon-expanded SSH and web chains refuse these operators.

Named chains:
- `quick` -- short view (`match up | count`)

Command-specific:
- `detail` `<value>` -- show detail

**Subcommands:** `brief`, `detail`

---

*3 commands total.*
