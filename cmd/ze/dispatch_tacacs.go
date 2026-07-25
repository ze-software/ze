// Design: ai/rules/feature-gate-registration.md -- ze_tacacs dispatch-root CLI gating
//
// TACACS+ CLI command registration (`ze tacacs ...`) for the ze dispatch
// composition root, gated on ze_tacacs. The other roots are the generated
// internal/component/plugin/all/all_ze_tacacs.go (config schema) and the
// hand-gated internal/component/aaa/all/all_ze_tacacs.go (AAA backend). All
// three must drop the tacacs blank imports when ze_tacacs is off, or the
// package stays linked through the surviving root.

//go:build ze_core && ze_tacacs

package main

import (
	_ "github.com/ze-software/ze/internal/component/tacacs/cli"
)
