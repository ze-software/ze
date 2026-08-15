// Related: golden_test.go -- the TEMPLATE capture whose fixtures this compares
// Related: handler_golden_test.go -- the HANDLER capture whose fixtures this compares

package web

import (
	"path/filepath"
	"testing"

	"github.com/ze-software/ze/internal/test/golden"
)

// webPortTemplates explains each template fixture that does not match its
// pre-port bytes. Every other difference is a finding.
//
// html/template has no children, so a wrapper is written as a start template
// and an end template, with the content between the two calls. A templ
// component takes its children, so the pair became one component.
//
// The two shapes cannot be compared by path. The pre-port capture rendered the
// start and the end as two units with nothing between them. The ported fixture
// renders the whole wrapper around its children, and covers strictly more.
//
// The wrapper's own frame is still compared. component/detail--fields.html
// renders ten fields through it and has a pre-port counterpart. That fixture
// carries the `class="ze-field" id="field-hold-time"` frame on both sides.
const webPortWrapperPair = "the start and end pair became fieldWrapper, one component taking its children"

// The interface defect pass changes rendered bytes ON PURPOSE. Ten defects the
// port recorded rather than fixed are fixed after it, so each unit below now
// differs from its pre-port form. Each reason names what a reader would see.
//
// The defects and their journal rows are listed in
// plan/spec-web-templ-migration.md. Nothing here excuses a difference the pass
// did not intend. This table is fail-closed, so an entry naming a unit that no
// longer differs is itself a finding.
const (
	portSecretMasked     = "a password field renders the display placeholder, never the stored secret"
	portEmptyRowColspan  = "the empty-state cell spans every column the header drew, where it wrote colspan 0"
	portAddEntryID       = "the add-entry control has a DOM id unique within one document"
	portErrorToggle      = "the error drawer's toggle carries data-action, which is what opens it"
	portErrorSwapRemoved = "the panel element naming a swap htmx does not implement is gone"
	portListRowClass     = "a list-table row names a base class, where it wrote an empty class attribute"
	portSecurityHeaders  = "the response carries the four security headers every response owes"
	portNumberEditor     = "an integer leaf reaches the number editor and shows its schema range"
)

// portEditorFocusID is the eleventh defect the pass fixes, and it changes the
// bytes of every fixture that draws an inline editor.
//
// htmx keeps the focused element across a swap and re-finds it by id
// afterwards. Each editor replaces itself with the response, and no editor
// carried an id. An operator lost the caret on every field of the config
// editor. The id is derived from the leaf path (fieldInputID, view.go), which
// is what makes the page's editor and the response's editor agree.
const portEditorFocusID = "an inline editor carries a DOM id, and htmx puts the caret back in the element that id finds"

// portEnterTriggerCSP is the twelfth defect, and it is what made the eleventh
// visible on every keystroke rather than once a second.
//
// Four controls carried a bracketed htmx trigger. htmx compiles that filter
// with Function(), and setSecurityHeaders (auth.go) forbids it, so htmx left
// the filter unset and the trigger fired on every key. The markup names the
// custom event ze-enter, and initEnterSubmit (assets/cli.js) dispatches it.
const portEnterTriggerCSP = "Enter arrives as an event a listener dispatches, where the trigger filter needed an eval the CSP refuses"

// Phase 5 DELETES six components, so six units stop being captured. Each was
// markup no page rendered, and each entry says what proved it dead.
//
// Two of them were ports of a template file NewRenderer never parsed. Their
// markup had a second spelling in Go, diverged from it, and that spelling is
// the one the operator reached. Deleting the orphan leaves the live one.
const (
	portDeadNeverParsed = "deleted: the port of a template file no renderer parsed, and the live " +
		"markup it duplicated is a component of its own now"
	portDeadNoProducer = "deleted: no producer builds the value it reads, so no page could render it"
	portDeadNoCaller   = "deleted: no page rendered it, and none had since before this migration"
)

var webPortTemplates = map[string]string{
	"terminal.html":            portDeadNeverParsed,
	"notification_banner.html": portDeadNeverParsed,

	"flex.html--flag.html":  portDeadNoProducer,
	"flex.html--value.html": portDeadNoProducer,
	"flex.html--block.html": portDeadNoProducer,

	"component/cli_bar.html":                    portDeadNoCaller,
	"component/dashboard_overview--full.html":   portDeadNoCaller,
	"component/dashboard_overview--empty.html":  portDeadNoCaller,
	"component/sidebar--nested.html":            portDeadNoCaller,
	"component/sidebar--root.html":              portDeadNoCaller,
	"component/sidebar_section--list.html":      portDeadNoCaller,
	"component/sidebar_section--container.html": portDeadNoCaller,

	"input/field_wrapper_start--bare.html":      webPortWrapperPair,
	"input/field_wrapper_start--annotated.html": webPortWrapperPair,
	"input/field_wrapper_end.html":              webPortWrapperPair,

	"component/detail--fields.html":        portNumberEditor + ", " + portEditorFocusID + ", " + portEnterTriggerCSP,
	"component/detail--list-table.html":    portListRowClass + ", " + portEditorFocusID + ", " + portEnterTriggerCSP,
	"component/error_panel.html":           portErrorToggle,
	"component/finder--columns.html":       portAddEntryID,
	"component/full_content--fields.html":  portAddEntryID + ", " + portNumberEditor + ", " + portEditorFocusID + ", " + portEnterTriggerCSP,
	"component/full_content--monitor.html": portAddEntryID + ", " + portNumberEditor + ", " + portEditorFocusID + ", " + portEnterTriggerCSP,
	"component/list_table--editable.html":  portListRowClass + ", " + portEditorFocusID + ", " + portEnterTriggerCSP,
	"component/list_table--readonly.html":  portListRowClass,
	"component/oob_error.html":             portErrorSwapRemoved,
	"component/oob_response--fields.html":  portAddEntryID + ", " + portNumberEditor + ", " + portEditorFocusID + ", " + portEnterTriggerCSP,
	"component/oob_response--monitor.html": portAddEntryID + ", " + portNumberEditor + ", " + portEditorFocusID + ", " + portEnterTriggerCSP,

	"input/input_bool--false.html":     portEditorFocusID,
	"input/input_bool--true.html":      portEditorFocusID,
	"input/input_bool--unset.html":     portEditorFocusID,
	"input/input_enum--selected.html":  portEditorFocusID,
	"input/input_enum--unset.html":     portEditorFocusID,
	"input/input_number--bare.html":    portEditorFocusID + ", " + portEnterTriggerCSP,
	"input/input_number--default.html": portEditorFocusID + ", " + portEnterTriggerCSP,
	"input/input_number--set.html":     portEditorFocusID + ", " + portEnterTriggerCSP,
	"input/input_text--bare.html":      portEditorFocusID + ", " + portEnterTriggerCSP,
	"input/input_text--default.html":   portEditorFocusID + ", " + portEnterTriggerCSP,
	"input/input_text--set.html":       portEditorFocusID + ", " + portEnterTriggerCSP,

	"component/workbench_form--fields.html":       portSecretMasked,
	"component/workbench_table--add-actions.html": portEmptyRowColspan,
	"component/workbench_table--empty.html":       portEmptyRowColspan,
	"page/layout.html--cli.html":                  portErrorToggle,
	"page/layout.html--finder.html":               portErrorToggle,
	"page/workbench.html--full.html":              portErrorToggle,
	"page/workbench.html--readonly.html":          portErrorToggle,
}

// portSnapshotScript is why the eight IS-IS and OSPF views moved. They are one
// page shell, so one reason covers all eight.
//
// snapshotPageHTML (page_snapshot.go) wrote its EventSource as an inline
// <script>. setSecurityHeaders (auth.go) puts script-src 'self' on every
// response, so a browser refused that script and the live view never updated.
//
// The stream and the event name are data attributes now. assets/snapshot-live.js
// reads them, and the page loads it from /assets/. The two JSEscapeString calls
// went with the script, because neither value is JavaScript any more.
const portSnapshotScript = "the snapshot page's live view is an external script, which script-src 'self' allows"

// webPortHandlers explains each response whose content changed on purpose.
// Every other difference is a finding.
//
// Nine responses moved, and each one moved to make a page work. Eight are the
// snapshot views above. The ninth is AC-5, recorded against A-2 in
// plan/spec-web-templ-migration.md.
//
// handleDashboardEventsPage (page_dashboard.go) ran each cell through
// template.HTMLEscapeString and handed the result to markup that escapes
// again. An operator therefore read the JSON payload of an event as &#34;
// rather than as a quote. The pre-port fixture holds that defect. The hand
// escape is deleted, and templ escapes once.
//
// The rest of the table is header and class differences the port declared
// before phase 5, each with its own reason above.
var webPortHandlers = map[string]string{
	"nav-show-events.txt": "AC-5: the event payload was escaped twice before the port",

	"get-isis.txt":                 portSnapshotScript,
	"get-isis-neighbors.txt":       portSnapshotScript,
	"get-isis-database.txt":        portSnapshotScript,
	"get-ospf.txt":                 portSnapshotScript,
	"get-ospf-neighbors.txt":       portSnapshotScript,
	"get-ospf-database.txt":        portSnapshotScript,
	"get-ospf-database-opaque.txt": portSnapshotScript,
	"get-ospfv3-database.txt":      portSnapshotScript,

	"get-admin-subtree.txt":             portErrorToggle,
	"get-admin.txt":                     portErrorToggle,
	"get-assets-missing.txt":            portSecurityHeaders,
	"get-assets-ze-svg.txt":             portSecurityHeaders,
	"get-cli.txt":                       portErrorToggle,
	"get-favicon.txt":                   portSecurityHeaders,
	"get-fragment-detail-page.txt":      portErrorToggle + ", " + portEditorFocusID + ", " + portEnterTriggerCSP,
	"get-fragment-detail.txt":           portEditorFocusID + ", " + portEnterTriggerCSP,
	"get-monitor.txt":                   portErrorToggle,
	"get-root.txt":                      portSecurityHeaders,
	"get-show-bgp-finder.txt":           portErrorToggle + ", " + portListRowClass + ", " + portEditorFocusID + ", " + portEnterTriggerCSP,
	"get-show-finder.txt":               portErrorToggle,
	"nav-show-api.txt":                  portErrorToggle,
	"nav-show-bgp-family.txt":           portEmptyRowColspan + ", " + portErrorToggle,
	"nav-show-bgp-group.txt":            portEmptyRowColspan + ", " + portErrorToggle,
	"nav-show-bgp-peer.txt":             portErrorToggle,
	"nav-show-bgp-policy.txt":           portEmptyRowColspan + ", " + portErrorToggle,
	"nav-show-bgp-summary.txt":          portErrorToggle,
	"nav-show-firewall-chain.txt":       portEmptyRowColspan + ", " + portErrorToggle,
	"nav-show-firewall-connections.txt": portEmptyRowColspan + ", " + portErrorToggle,
	"nav-show-firewall-rule.txt":        portEmptyRowColspan + ", " + portErrorToggle,
	"nav-show-firewall-set.txt":         portEmptyRowColspan + ", " + portErrorToggle,
	"nav-show-firewall.txt":             portEmptyRowColspan + ", " + portErrorToggle,
	"nav-show-health.txt":               portErrorToggle,
	"nav-show-iface-traffic.txt":        portEmptyRowColspan + ", " + portErrorToggle,
	"nav-show-iface-type-bridge.txt":    portErrorToggle,
	"nav-show-iface-type-ethernet.txt":  portErrorToggle,
	"nav-show-iface-type-tunnel.txt":    portEmptyRowColspan + ", " + portErrorToggle,
	"nav-show-iface-type-vlan.txt":      portEmptyRowColspan + ", " + portErrorToggle,
	"nav-show-iface.txt":                portErrorToggle,
	"nav-show-ip-addresses.txt":         portEmptyRowColspan + ", " + portErrorToggle,
	"nav-show-ip-dns.txt":               portErrorToggle,
	"nav-show-ip-routes.txt":            portEmptyRowColspan + ", " + portErrorToggle,
	"nav-show-l2tp-health.txt":          portEmptyRowColspan + ", " + portErrorToggle,
	"nav-show-l2tp-sessions.txt":        portEmptyRowColspan + ", " + portErrorToggle,
	"nav-show-l2tp.txt":                 portErrorToggle,
	"nav-show-lg.txt":                   portErrorToggle,
	"nav-show-logs-errors.txt":          portErrorToggle,
	"nav-show-logs-live.txt":            portErrorToggle,
	"nav-show-logs-warnings.txt":        portErrorToggle,
	"nav-show-mcp.txt":                  portErrorToggle,
	"nav-show-ssh.txt":                  portErrorToggle,
	"nav-show-system-hardware.txt":      portErrorToggle,
	"nav-show-system-identity.txt":      portErrorToggle,
	"nav-show-system-resources.txt":     portErrorToggle,
	"nav-show-system-sysctl.txt":        portEmptyRowColspan + ", " + portErrorToggle,
	"nav-show-tacacs.txt":               portErrorToggle,
	"nav-show-telemetry.txt":            portErrorToggle,
	"nav-show-tools-bgp-decode.txt":     portErrorToggle,
	"nav-show-tools-capture.txt":        portErrorToggle,
	"nav-show-tools-metrics.txt":        portErrorToggle,
	"nav-show-tools-ping.txt":           portErrorToggle,
	"nav-show-users.txt":                portErrorToggle,
	"nav-show-web.txt":                  portErrorToggle,
	"nav-show.txt":                      portErrorToggle,
	"post-login-ok.txt":                 portSecurityHeaders,
}

// TestWebTemplPortFidelity compares every captured unit against the bytes it
// held before the templ port.
//
// It is the evidence for AC-2 of plan/spec-web-templ-migration.md, and it takes
// the ref as a parameter so a reader can run the same comparison. The
// comparison behind it was hand-run once. That left the acceptance criterion on
// a measurement nobody can repeat.
//
// VALIDATES: no unit of the web UI renders different content after the port.
// PREVENTS: a re-baselined fixture hiding a change an operator would see. Once
// a fixture is recaptured, the golden check compares the port against itself.
func TestWebTemplPortFidelity(t *testing.T) {
	ref := golden.PortRef()

	golden.AssertPortFidelity(t, ref, filepath.Join("testdata", "golden"), golden.PortMarkup, webPortTemplates)
	golden.AssertPortFidelity(t, ref, filepath.Join("testdata", webHandlerFixtures), golden.PortResponse, webPortHandlers)
	golden.AssertPortFidelity(t, ref, filepath.Join("testdata", "markup"), golden.PortMarkup, nil)
}
