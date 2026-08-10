---
kind: directive
level: MUST
stage:
---
- **A `.py` symbol question MUST go to the LSP tool, which `pyright-langserver` serves once `ze-setup` has run.** 405 measured Read calls named a `.py` path while this machine carried no Python server. Every one of them answered a symbol question by reading the whole file.
- **The Go fall-back has no Python twin. `pyright` is a type checker, not a symbol server: its CLI carries no `symbols`, `definition` or `references` verb (`pyright --help`).** `pyright --outputjson <file>` answers a different question, whether the file type-checks. Where the LSP tool is absent, a `.py` symbol MAY be resolved the other way this rule allows: `grep -n` for the name, then a ranged Read.
- **`make ze-setup CHECK=1` prints a `gopls-answers` row and a `pyright-answers` row. Both RUN their server instead of looking for a binary** (`gopls_status` and `pyright_status`, `scripts/dev/dev-setup.py`). A server on PATH that has never answered is the state both rows exist to catch. gopls was in that state on this machine for weeks.
