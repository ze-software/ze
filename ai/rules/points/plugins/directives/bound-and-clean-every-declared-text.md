---
kind: directive
level: MUST
stage:
---
- **A text a plugin DECLARES that reaches an operator MUST be bounded, and MUST be refused when it carries a control character its shape does not allow.** The check runs at Stage 1, before the declaration is stored, because a stored declaration is live for every operator. A ONE-LINE text (a command's `description`, a pipe alias's `description`) refuses every control character: it is written into the tab-separated shell-completion format and into the one-line terminal candidate, so a newline or a tab breaks the format for every row that follows and an ESC writes an ANSI sequence to the terminal. A PARAGRAPH (a command's `long-help`) keeps its newlines and refuses the rest. `validateHelpDecls` and `validateDeclaredText` (`internal/component/plugin/server/startup.go`) are where the next declared text joins them.
