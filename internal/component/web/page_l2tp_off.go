// Design: ai/rules/feature-gate-registration.md -- ze_l2tp-off L2TP page stub
// Related: page_l2tp.go -- the real BNG-backed pages (ze_l2tp builds)

//go:build !ze_l2tp

package web

import (
	"html/template"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
)

// renderL2TPPageContent is the ze_l2tp-off counterpart of the BNG workbench
// pages: the workbench "l2tp" route stays valid in every ze_web build, but a
// build without the BNG states plainly that L2TP is not included instead of
// importing internal/component/l2tp (which would pin the whole subtree into
// the binary). The /l2tp session routes are absent too: register_l2tp.go
// carries the same tag, so nothing registers them.
func renderL2TPPageContent(_ *Renderer, _ []string, _ *config.Tree) (template.HTML, bool) {
	return template.HTML(`<div class="workbench-empty">L2TP/PPPoE (BNG) is not included in this build (ze_l2tp off).</div>`), true
}
