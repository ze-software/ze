// Design: docs/contributing/ze-go-style.md -- unit tests for the unified Go style engine
//
// VALIDATES: CheckFormat enforces gofmt and local github.com/ze-software/ze
// import grouping; CheckAdded catches newly added forbidden Go patterns via
// exact AST analysis (and token fallback for partial fragments) while ignoring
// comments, string literals, and untouched legacy lines.
// PREVENTS: divergence between edit hooks and commit gates, regex false
// positives on comments/strings, and regressions in go-style enforcement.
package gostyle

import (
	"strings"
	"testing"
)

func TestCheckFormatEnforcesGofmtAndLocalImportGrouping(t *testing.T) {
	t.Parallel()

	badFmt := "package demo\n\nfunc   x() {}\n"
	if hit := CheckFormat("internal/demo/a.go", badFmt); hit == nil || !strings.Contains(hit.Err.Error(), "is not gofmt-formatted") {
		t.Fatalf("badFmt = %+v, want gofmt finding", hit)
	}

	mixedImports := "package demo\n\nimport (\n\t\"github.com/ze-software/ze/internal/core/textbuf\"\n\t\"strings\"\n)\n\nvar _ = strings.TrimSpace\nvar _ textbuf.Buffer\n"
	if hit := CheckFormat("internal/demo/a.go", mixedImports); hit == nil || !strings.Contains(hit.Err.Error(), "imports are not grouped") {
		t.Fatalf("mixedImports = %+v, want local import grouping finding", hit)
	}

	localBeforeStd := "package demo\n\nimport (\n\t\"github.com/ze-software/ze/internal/core/textbuf\"\n\n\t\"strings\"\n)\n\nvar _ = strings.TrimSpace\nvar _ textbuf.Buffer\n"
	if hit := CheckFormat("internal/demo/a.go", localBeforeStd); hit == nil || !strings.Contains(hit.Err.Error(), "imports are not grouped") {
		t.Fatalf("localBeforeStd = %+v, want local import grouping finding", hit)
	}

	clean := "package demo\n\nimport (\n\t\"strings\"\n\n\t\"github.com/ze-software/ze/internal/core/textbuf\"\n)\n\nvar _ = strings.TrimSpace\nvar _ textbuf.Buffer\n"
	if hit := CheckFormat("internal/demo/a.go", clean); hit != nil {
		t.Fatalf("clean = %+v, want nil", hit)
	}
}

func TestCheckAddedCatchesPatternsAndIgnoresCommentsAndStrings(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		rel      string
		before   string
		after    string
		opts     Options
		wantHit  string
		wantSev  Severity
		wantPass bool
	}{
		{
			name:    "fmt.Sprintf in internal package is blocked",
			rel:     "internal/component/bgp/a.go",
			before:  "package bgp\n",
			after:   "package bgp\n\nimport \"fmt\"\n\nfunc f(n int) string { return fmt.Sprintf(\"%d\", n) }\n",
			wantHit: "newly added fmt.Sprintf",
			wantSev: SeverityBlock,
		},
		{
			name:     "fmt.Sprintf in comment or string literal passes",
			rel:      "internal/component/bgp/a.go",
			before:   "package bgp\n",
			after:    "package bgp\n\n// Never call fmt.Sprintf or go func() or log.Printf here.\nconst msg = \"fmt.Sprintf os.Exit(1) switch args[0]\"\n",
			wantPass: true,
		},
		{
			name:    "legacy log.Printf is blocked",
			rel:     "internal/component/bgp/a.go",
			before:  "package bgp\n",
			after:   "package bgp\n\nimport \"log\"\n\nfunc f() { log.Printf(\"hi\") }\n",
			wantHit: "legacy log.Printf",
			wantSev: SeverityBlock,
		},
		{
			name:    "os.Exit in handler is blocked",
			rel:     "internal/component/bgp/a.go",
			before:  "package bgp\n",
			after:   "package bgp\n\nimport \"os\"\n\nfunc f() { os.Exit(1) }\n",
			wantHit: "os.Exit() in handler",
			wantSev: SeverityBlock,
		},
		{
			name:    "os.Exit in register.go is blocked",
			rel:     "internal/component/bgp/register.go",
			before:  "package bgp\n",
			after:   "package bgp\n\nimport \"os\"\n\nfunc init() { os.Exit(1) }\n",
			wantHit: "os.Exit() in registration file",
			wantSev: SeverityBlock,
		},
		{
			name:    "os.Exit in internal/component/config/doctor.go is blocked",
			rel:     "internal/component/config/doctor.go",
			before:  "package config\n\nfunc init() {\n}\n",
			after:   "package config\n\nimport \"os\"\n\nfunc init() {\n\tos.Exit(1)\n}\n",
			wantHit: "os.Exit() in registration file",
			wantSev: SeverityBlock,
		},
		{
			name:     "panic BUG in register.go is allowed",
			rel:      "internal/component/bgp/register.go",
			before:   "package bgp\n",
			after:    "package bgp\n\nfunc init() { panic(\"BUG: bgp: registration failed\") }\n",
			wantPass: true,
		},
		{
			name:     "os.Exit in main.go is allowed",
			rel:      "cmd/ze/main.go",
			before:   "package main\n",
			after:    "package main\n\nimport \"os\"\n\nfunc main() { os.Exit(1) }\n",
			wantPass: true,
		},
		{
			name:    "os.Getenv on ZE_ key in product code is blocked",
			rel:     "internal/component/bgp/a.go",
			before:  "package bgp\n",
			after:   "package bgp\n\nimport \"os\"\n\nfunc f() string { return os.Getenv(\"ZE_BGP_DEBUG\") }\n",
			wantHit: "Ze environment key",
			wantSev: SeverityBlock,
		},
		{
			name:     "os.Getenv on system PATH passes",
			rel:      "internal/component/bgp/a.go",
			before:   "package bgp\n",
			after:    "package bgp\n\nimport \"os\"\n\nfunc f() string { return os.Getenv(\"PATH\") }\n",
			wantPass: true,
		},
		{
			name:    "String() equality comparison is blocked",
			rel:     "internal/component/bgp/a.go",
			before:  "package bgp\n",
			after:   "package bgp\n\ntype k int\nfunc (k) String() string { return \"x\" }\nfunc f(a k) bool { return a.String() == \"x\" }\n",
			wantHit: "comparison on .String()",
			wantSev: SeverityBlock,
		},
		{
			name:    "fake BufHandle backed by make is blocked",
			rel:     "internal/component/bgp/a.go",
			before:  "package bgp\n",
			after:   "package bgp\n\ntype BufHandle struct{ Buf []byte }\nfunc f() BufHandle { return BufHandle{Buf: make([]byte, 16)} }\n",
			wantHit: "BufHandle backed by make()",
			wantSev: SeverityBlock,
		},
		{
			name:    "bare nolint with comment is blocked",
			rel:     "internal/component/bgp/a.go",
			before:  "package bgp\n",
			after:   "package bgp\n\nfunc f() {\n\t_ = 1 //nolint // broad ignore\n}\n",
			wantHit: "names no linter",
			wantSev: SeverityBlock,
		},
		{
			name:    "exported And function warns when enabled",
			rel:     "internal/component/bgp/a.go",
			before:  "package bgp\n",
			after:   "package bgp\n\nfunc ParseAndValidate() {}\n",
			opts:    Options{CheckWarnNames: true},
			wantHit: "joins two responsibilities",
			wantSev: SeverityWarn,
		},
		{
			name:    "new shipped file without Design header is blocked when enabled",
			rel:     "internal/component/bgp/newfile.go",
			before:  "",
			after:   "package bgp\n\nfunc f() {}\n",
			opts:    Options{CheckDesignHeader: true},
			wantHit: "missing a // Design: header",
			wantSev: SeverityBlock,
		},
		{
			name:     "new register.go is exempt from Design header requirement",
			rel:      "internal/component/bgp/register.go",
			before:   "",
			after:    "package bgp\n\nfunc init() {}\n",
			opts:     Options{CheckDesignHeader: true},
			wantPass: true,
		},
		{
			name:    "fragment token fallback catches fmt.Sprintf without package decl",
			rel:     "internal/probe/probe.go",
			before:  "",
			after:   `return fmt.Sprintf("%d", n)`,
			wantHit: "newly added fmt.Sprintf",
			wantSev: SeverityBlock,
		},
		{
			name:     "fragment token fallback ignores comment mentioning fmt.Sprintf",
			rel:      "internal/probe/probe.go",
			before:   "",
			after:    `// avoid fmt.Sprintf("%d", n)`,
			wantPass: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			hits := CheckAdded(tc.rel, tc.before, tc.after, tc.opts)
			if tc.wantPass {
				if len(hits) != 0 {
					t.Fatalf("unexpected findings: %+v", hits)
				}
				return
			}
			if len(hits) == 0 {
				t.Fatalf("got 0 findings, want finding mentioning %q", tc.wantHit)
			}
			if !strings.Contains(hits[0].Err.Error(), tc.wantHit) {
				t.Fatalf("finding = %q, want substring %q", hits[0].Err.Error(), tc.wantHit)
			}
			if tc.wantSev != SeverityUnspecified && hits[0].Severity != tc.wantSev {
				t.Fatalf("severity = %v, want %v", hits[0].Severity, tc.wantSev)
			}
		})
	}
}
