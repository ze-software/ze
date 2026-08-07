---
kind: directive
level: MUST
stage:
---
- **Never rewrite a sentence only to satisfy a count.** An edit that changes no meaning for a reader is pure overhead, and it is the thing this guideline exists to remove. A sentence two words over the limit is not a defect.
- **The checker reports. It does not refuse.** `make ze-ste-check` and the commit-time check print findings and let the work through. Apply a finding when it makes the text clearer, and ignore it when it does not.
- **Aim at the six habits, not at the arithmetic.** The word and sentence counts below are a smell test for a run-on, never a target to hit.
- **Write every sentence in STE.** One topic per sentence, active voice, the imperative form for instructions, and an approved verb for every action.
- **The six habits in the next section are the operative list.** A numbered STE rule bans each one. Learn the six first, then read the standard for the remainder.
- **A short, plain, repetitive sentence is correct.** Elegant variation is a defect, not a style.
- **Maximum 20 words in a procedural sentence** (STE Rule 5.1). This covers guide steps, runbooks, remediation lines, and commit messages.
- **Maximum 25 words in a descriptive sentence** (STE Rule 6.3), and maximum 6 sentences in a paragraph (STE Rule 6.6).
- **No semicolons** (STE Rule 8.1). Write two sentences, or use a vertical list (STE Rule 4.3).
- **No contractions, and do not omit words** (STE Rule 4.2). Write "do not", never "don't".
- **When you cannot say it in STE, the text is unclear, not the rule.** Change the sentence construction (STE Rule 9.1).
- **One word, one meaning, one part of speech.** Do not verb a noun, and do not carry two senses of a word through one document (STE Rules 1.2, 1.3, 1.7).
- **No slang, no jargon, no regional words.** `brick the router`, `bounce the daemon`, and `footgun` are not English a reader can look up (STE Rule 1.10).
- **Use the plain word unless the technical one earns its place.** Write for a capable reader who knows computing but not this repository. A good teacher keeps the idea and simplifies the words around it. A technical term is right when it names something exactly. Everywhere else it costs the reader and returns nothing. The test: would you say this sentence out loud to a colleague who has not read our code? `gated` is the standing example of a word to cut. It says that something is restricted, but not by what. Write `N requirements the check enforces`, or `compiled out unless ze_bgp is set`. The opposite mistake is equally real. Terms the code defines (`carrier`, `polarity`, `disposition`) are exact, so keep them and expand them on first use. Owner directive, 2026-08-01, and no checker enforces it.
- **Three words in a noun stack, no more.** Break a longer one with a preposition (STE Rules 2.1, 2.2). Expand an abbreviation on first use.
- **A replacement must keep the meaning.** `should` becomes MUST only for a real obligation, and `must` never sits in front of an imperative (STE Rules 1.2, 5.3).
- **Splitting a sentence is half the job. Connect the pieces.** Give information gradually, repeat the key word, open a paragraph with its topic sentence, and use a connector. `And` and `But` are correct sentence openers (STE Rules 4.4, 6.1, 6.2, 6.4, 6.5).
- **Keep the conjunction `that`,** and never drop an article to shorten a line (STE Rules 4.5, GR-1).
- **No definite article before a noun that carries an alphanumeric identifier.** Write `RFC 4271` and `peer 10.0.0.1` (STE Rule 4.5).
- **No Latin abbreviations.** Write `for example`, `that is`, and `and so on` (STE GR-6). Use gender-neutral language (GR-7).
- **The `-ing` form is permitted as a technical noun or its modifier, and nowhere else.** `routing table` is correct, and `before installing` is not (STE Rule 3.5).
- **Formatting, units, and abbreviation policy are Ze's own.** The standard sets no rules for them, so the uppercase in a safety block is our choice, not a requirement.
