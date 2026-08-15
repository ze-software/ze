// Related: page_system.go -- buildHostHardwareHTML and writeHardwareKV, the builder this capture renders
// Related: handler_golden_test.go -- the HANDLER capture, which normalizes this panel because its content follows the machine

package web

import (
	"html/template"
	"path/filepath"
	"testing"

	"github.com/ze-software/ze/internal/test/golden"
)

// webMarkupFixtures is where the captured markup lives.
const webMarkupFixtures = "markup"

// webMarkupCase is one markup builder and the bytes it must keep producing.
type webMarkupCase struct {
	// Name is the fixture stem.
	Name string
	// Build renders the markup over input the case fixes, so the bytes are the
	// same on every machine and on every run.
	Build func() template.HTML
}

// webMarkupGoldenCases are the builders that write HTML in Go rather than in a
// template. A template capture cannot reach them and the handler capture cannot
// pin them, because their live input is whatever the machine holds.
var webMarkupGoldenCases = []webMarkupCase{
	{
		Name:  "host-hardware",
		Build: func() template.HTML { return buildHostHardwareHTML(&Renderer{}, webGoldenHardwareSections()) },
	},
	{
		// buildHostHardwareData never returns an empty list: it substitutes an
		// Info section. The empty branch is still markup an operator can be
		// served, so it is captured rather than left to a reader to derive.
		Name:  "host-hardware-empty",
		Build: func() template.HTML { return buildHostHardwareHTML(&Renderer{}, nil) },
	},
}

// webGoldenHardwareSections is the hardware inventory this capture renders. It
// carries one item per branch writeHardwareKV holds. A row with no class, the
// two carrier classes a NIC row takes, and the alarm class a thermal row takes.
//
// A key, a value and a section title each carry HTML specials, because
// writeHardwareKV escapes all three and this capture is what pins that. The
// value cell matters most: buildHostHardwareData puts err.Error(), dev.Model
// and the DMI vendor, product and version strings there, and firmware supplies
// those. Before this fixture carried a special in the value, deleting the
// escape at that call site left every capture green.
func webGoldenHardwareSections() []hardwareSection {
	return []hardwareSection{
		{Title: "CPU", Items: []HardwareItem{
			{Key: "Model", Value: `Ze Reference CPU <rev2> & "turbo"`},
			{Key: "Logical CPUs", Value: "4"},
			{Key: "Core 0 (pkg 0)", Value: "cpu0, 2400 MHz"},
		}},
		{Title: `DMI <vendor> & "co"`, Items: []HardwareItem{
			{Key: "System Vendor", Value: `Acme <Systems> & Co`},
		}},
		{Title: "NIC", Items: []HardwareItem{
			{Key: "eth0", Value: "ze-driver, 02:00:00:00:00:01, 10000 Mbps, up", CSSClass: "up"},
			{Key: "eth1", Value: "ze-driver, 02:00:00:00:00:02, -, down", CSSClass: "down"},
		}},
		{Title: "Thermal", Items: []HardwareItem{
			{Key: "package", Value: "48.5°C"},
			{Key: "core <hot> & \"loud\"", Value: "95.0°C [ALARM]", CSSClass: "alarm"},
		}},
	}
}

// TestWebMarkupGoldenOutput captures the bytes of every HTML builder the web
// package writes in Go.
//
// The handler capture cannot do this. buildHostHardwareData reads host.Detect,
// so the live panel's section list, row count, keys and values follow the
// machine. The handler fixture normalizes them away. This capture fixes the
// input instead. Every byte of the panel is then compared: the section title,
// the table, each row and the class a row carries.
//
// VALIDATES: the markup the Go builders write, over input this test fixes.
// PREVENTS: a port rewriting a panel that no template holds, with the template
// capture green because it owns no template for it and the handler capture
// green because it normalizes the machine-derived content away.
func TestWebMarkupGoldenOutput(t *testing.T) {
	names := make([]string, 0, len(webMarkupGoldenCases))
	for _, c := range webMarkupGoldenCases {
		names = append(names, c.Name)
	}

	golden.AssertUniqueNames(t, "fixture", "webMarkupGoldenCases", names)

	root := filepath.Join("testdata", webMarkupFixtures)
	written := make([]string, 0, len(webMarkupGoldenCases))

	for _, c := range webMarkupGoldenCases {
		fixture := filepath.Join(root, c.Name+".html")
		written = append(written, fixture)

		t.Run(c.Name, func(t *testing.T) {
			got := c.Build()
			if got == "" {
				t.Fatalf("case %q rendered nothing; its fixture would pin an empty file", c.Name)
			}

			golden.Compare(t, fixture, []byte(got))
		})
	}

	golden.AssertCoversDir(t, root, "webMarkupGoldenCases", written)
}
