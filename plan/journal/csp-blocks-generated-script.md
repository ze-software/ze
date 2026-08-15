# CSP blocks the script the markup needs

`setSecurityHeaders` (`internal/component/web/auth.go`) sends
`default-src 'self'; script-src 'self'; style-src 'self'` on every response. No
`unsafe-eval` and no `unsafe-inline`. A browser therefore refuses an inline
`<script>`, an inline event handler, and any source a page compiles at run time
with `eval` or `Function`.

A refusal is silent to the operator. The feature does not fail closed. It
either does nothing or, worse, falls back to a wider behavior than the markup
asked for. Read a page feature against this header before you write it.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-08-15 | - | web inline editors and the CLI terminal | four controls carried a bracketed htmx trigger. Three editors used `keyup[key=='Enter']`, in `input_text.templ`, `input_number.templ` and `component_list_table.templ`. The terminal used `keydown[key=='Enter']`, in `component_cli_terminal.templ`. htmx builds a bracketed filter as SOURCE and runs it through `Function()` (`nt`, `assets/htmx.min.js`). The call throws under this header. `nt` catches it, fires `htmx:syntax:error` and returns null. Its caller assigns only a truthy filter. `gt` then reads no filter and ignores no event. The trigger degraded to a BARE key event rather than failing closed. Measured in a browser with this exact header. Three keystrokes in a config field sent three POSTs to `/config/set/`, carrying `londona`, `londonb` and `londonc`. Three keystrokes in the terminal ran `command=`, `command=s` and `command=sh`. The editor case also swapped the field away on every keystroke, which is where the caret went | FIXED 2026-08-15. The four triggers name the custom event `ze-enter`. `initEnterSubmit` (`assets/cli.js`) reads the element's own `hx-trigger` and dispatches it. The markup states the contract and the script compiles nothing. A control whose own handler already acted leaves `defaultPrevented` set, which keeps the listener off. That is what stops the terminal posting twice. Measured after the fix. Three keystrokes send ONE POST carrying `londonabc`, and Enter still commits 400ms later, inside the 1s debounce. `TestNoTriggerFilterNeedsEval` (`markup_contract_test.go`) refuses a `[` in any `hx-trigger` in the package. It also checks the listener is defined, called, and named by a component |

## Earlier casualties of the same header

Three, all found in the same week. A list, not a table. The row register above
is this page's only table, because the gate reads every table line as a row.

- IS-IS and OSPF live views. The header refused an inline `<script>` that
  opened the EventSource, so the view never updated in any browser. See
  `portSnapshotScript` (`internal/component/web/port_check_test.go`) and
  `assets/snapshot-live.js`.
- Looking glass graph mode. The header refused an inline event handler on the
  mode button. See `internal/component/lg/assets/graph-mode.js`.
- Error drawer toggle. The header refused an inline handler, which is why the
  toggle carries `data-action`. See `plan/journal/rendered-markup-invalid.md`.
