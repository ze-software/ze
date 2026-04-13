# Config Manipulation

**BLOCKING:** Config content MUST be manipulated through one of two methods only.

| Method | When |
|--------|------|
| Parsed YANG tree | When you have a loaded config tree in memory |
| Set command lines | When building or merging config text |

## Forbidden

- Raw text surgery (regex, string replace, brace counting, line insertion)
- Custom merge functions that parse config syntax outside the config system
- Any manipulation that assumes config structure from text patterns

The config format IS set commands. Duplicate blocks are additive. The parser handles merging.
Concatenating two valid config texts produces valid config.
