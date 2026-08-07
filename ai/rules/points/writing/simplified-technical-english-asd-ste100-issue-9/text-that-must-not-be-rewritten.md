---
kind: table
level: MUST NOT
stage:
---
| Surface | Why it does not change |
|---------|------------------------|
| RFC 2119 keywords (MUST, MUST NOT, SHOULD, MAY) when they name an RFC's obligation level | The keyword IS the requirement. `rfc-compliance.md` and every `RFC requirement:` test tag read the exact word. The dictionary substitution `should -> MUST` never applies to a quoted normative term |
| Quoted external text: RFC prose, vendor output, peer daemon log lines, third-party documentation | A quotation is evidence. A changed quotation is false evidence (`evidence.md`) |
| Go identifiers, YANG leaf names, JSON keys, env var keys, CLI tokens, command grammar | `go-standards.md`, `config.md`, `config.md`, and `cli.md` own these. STE governs prose, never identifiers |
| Technical nouns and technical verbs of this subject field: `peer`, `prefix`, `NLRI`, `teardown`, `netlink`, `deenergize` | STE Rules 1.5, 1.6, and 1.12 permit them. `request peer teardown` is a technical noun and is correct. `tear down the peer` is a phrasal verb and is not |
| Test fixture data, hex dumps, config samples, and fenced code blocks | These are data, not prose |
