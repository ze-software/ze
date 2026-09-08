# A rule written after the surface it binds

A rule is written for the case that produced it, and the surfaces it also binds
already exist. Nothing sweeps them: the rule's own text names the field that
prompted it, the check its author added covers that field, and every sibling
field on the same surface keeps the behavior the rule now forbids. The gap is
invisible from both ends. A reader of the rule sees a rule, a reader of the
sibling field sees a validator that bounds what it was asked to bound, and only
somebody reading the two together sees that one sentence is enforced in one
place out of several.

This is distinct from a guard added to one half of a pair. There both halves
exist when the guard is written, and the author sees one of them. Here the
second half was written FIRST, and it is the rule that arrived late.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-09-08 | plugin-declares-answer-shape | `validateDeclaredFieldName` (`internal/component/plugin/server/startup.go`), against `validateDeclaredText` in the same file | The bound-and-clean rule for a plugin's declared text (`ai/rules/plugins.md`: a declared text that reaches an operator MUST be refused when it carries a control character its shape does not allow) was written in `e691533a6` on 2026-08-31, for a command's `description` and `long-help`. The answer-shape channel had shipped a week earlier, on 2026-08-24, and its `Columns` and `AddressFields` names reach the operator on the same two surfaces: `completeDisplayFields` (`internal/component/command/completer.go`) offers each declared column as a candidate for the display operator, and the table renderer writes it as a header. `validateDeclaredFieldName` bounded the length and checked for emptiness and nothing else, and `normalizeCommand` collapses only the WHITESPACE control characters, so an ESC in a plugin's declared column name wrote an ANSI sequence to the terminal | fixed in this spec's closure. `validateDeclaredFieldName` calls `validateDeclaredText(name, maxNameLen, textOneLine)`, which replaces its own length check so one bound is stated once. `TestValidateShapeDecls` gained "a column name carrying an escape" and "an address-field name carrying a tab"; both return nil under the old body, so `require.Error` fails on each. Transferable: a rule about a KIND of value (a text an operator reads, a number a peer sends) owes a sweep of every value of that kind already in the tree, and the sweep belongs in the commit that writes the rule |
