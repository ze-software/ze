# Documentation First

**When:** before you spawn an agent or open a search to learn how a surface works, and whenever an edit changes behavior a page describes
**Severity:** blocking
**Related:** completion, evidence, context-economy, repo-maintenance, writing

## Directives

**You MUST read the page that covers a surface before you spawn an agent, open a broad search, or read wide code to learn how it works.** `ai/CODE-TO-DOCS.md` names the pages for a file, `ai/DOCS-TO-CODE.md` the files for a page, and `ai/INDEX.md` the page for a keyword. The investigation is then authorized by a gap you NAME: the page is SILENT on the question, or the page DISAGREES with the code.
**A change that makes a page wrong MUST carry its page edit in the SAME piece of work, before the next code edit starts.** A page that disagrees with the code is a defect repaired here rather than reported, and you MUST read the producing function to learn which side is wrong. You MUST NOT defer a page to a final pass, a closing commit, a review gate, or a follow-up spec.
