---
kind: directive
level: MUST
stage:
---
**A citation into another project MUST name the file and the SYMBOL too, and a forge permalink's `#L` anchor is a line number wearing a URL.** You MUST link the file at a pinned tag and put the function in the link text: `[bgp_io.c \`bgp_write\`](https://.../bgp_io.c)`. `c_line_number_ref` in `.claude/hooks/pretool-writeedit.py` refuses a bare anchor.
