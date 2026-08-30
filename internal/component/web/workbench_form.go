// Design: docs/architecture/web-components.md -- Workbench form component
// Related: workbench_table.go -- Table component (sibling)
// Related: workbench_detail.go -- Detail panel (sibling)
// Related: render.go -- Fragment rendering
//
// Spec: plan/spec-web-3-foundation.md (Form component, Phase 5).
//
// WorkbenchFormData drives the workbench_form.html template, rendering a
// singleton configuration form with typed fields, save, and discard actions.

package web

import (
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

//nolint:gochecknoglobals // Shared template helper state; immutable after init.
var formFieldIDReplacer = strings.NewReplacer("/", "-", ".", "-", ":", "-")

//nolint:gochecknoglobals // Immutable acronym lookup for friendly field labels.
var fieldLabelAcronyms = map[string]string{
	"ip": "IP", "ipv4": "IPv4", "ipv6": "IPv6", "asn": "ASN", "as": "AS",
	"id": "ID", "dns": labelDNS, "mac": labelMAC, "vlan": "VLAN", "url": "URL",
	"mtu": labelMTU, "ttl": "TTL", "tcp": "TCP", "udp": "UDP", "rfc": "RFC",
	"bgp": "BGP", "vrf": "VRF", "rd": "RD", "rt": "RT",
}

// humanizeFieldLabel turns a raw YANG field path into a friendly label for the
// add-entry overlay (F16): "connection/remote/ip" -> "Connection Remote IP",
// "session/asn/local" -> "Session ASN Local". Segments split on / - _ are
// title-cased, with common networking acronyms upper-cased.
func humanizeFieldLabel(path string) string {
	parts := strings.FieldsFunc(path, func(r rune) bool {
		return r == '/' || r == '-' || r == '_'
	})
	var tb textbuf.Buffer
	for i, p := range parts {
		if i > 0 {
			tb.Byte(' ')
		}
		if acronym, ok := fieldLabelAcronyms[strings.ToLower(p)]; ok {
			tb.Str(acronym)
			continue
		}
		tb.Str(strings.ToUpper(p[:1])).Str(p[1:])
	}
	return tb.String()
}

// WorkbenchFormData holds the data for a singleton configuration form.
type WorkbenchFormData struct {
	Title      string
	Fields     []WorkbenchFormField
	SaveURL    string
	DiscardURL string
}

// WorkbenchFormField describes one input field in the form.
type WorkbenchFormField struct {
	Name        string
	Path        string
	Label       string
	Type        string // "text", "number", "dropdown", "toggle", "ip", "list", "password"
	Value       string
	Options     []string // for dropdown type
	Items       []string // for list type
	Description string
	Required    bool
	Disabled    bool
}

// wbFormPasswordType is the WorkbenchFormField.Type that renders an
// <input type="password">. It hides the characters on screen and does nothing
// to the response body, so it is a display choice and never the mask.
//
// Masking is the schema's decision, and it happens before a value reaches a
// field. renderPageContent (workbench_pages.go) hands every page a masked
// display tree (secret.go). This constant used to drive a second mask here,
// which was a rule the schema did not write. It hid the defect it looked like
// it was preventing. A form typed a sensitive leaf as text, and the value went
// out in the clear.
const wbFormPasswordType = "password"

// The other WorkbenchFormField.Type values a page sets. component_workbench_form.templ
// switches on these, so a form that spells one differently renders as text.
const (
	wbFormTextType   = "text"
	wbFormNumberType = "number"
	wbFormToggleType = "toggle"
	wbFormListType   = "list"
)

// Field names that more than one service form uses. The name reaches the
// browser as the input name and comes back as the form key the handler reads.
const (
	wbFormEnabledField = "enabled"
	wbFormServersField = "servers"
	wbFormTimeoutField = "timeout"
	wbFormTokenField   = "token"
)

func formFieldName(f WorkbenchFormField) string {
	if f.Path != "" {
		return "field:" + f.Path
	}
	return "field:" + f.Name
}

func formFieldChecked(f WorkbenchFormField) bool {
	return f.Value == htmxRequestTrue
}

// formFieldID returns a stable DOM id suffix for a config path.
func formFieldID(path string) string {
	return formFieldIDReplacer.Replace(path)
}
