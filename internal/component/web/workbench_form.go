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
	"id": "ID", "dns": "DNS", "mac": "MAC", "vlan": "VLAN", "url": "URL",
	"mtu": "MTU", "ttl": "TTL", "tcp": "TCP", "udp": "UDP", "rfc": "RFC",
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
