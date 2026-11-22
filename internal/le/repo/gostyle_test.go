package repo

import (
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/commit/changecheck"
)

// VALIDATES: plan/journal/lint-contract-not-applied.md rows dated 2026-08-17
// (un-gofmt'd rfc7606_section51_receive_test.go and verify_run_test.go),
// 2026-08-19 (un-gofmt'd ssh.go //nolint alignment), 2026-08-31 (un-gofmt'd
// rfc2865_response_auth_test.go), plus newly added hook-only Go patterns
// (fmt.Sprintf, strconv.FormatInt, anonymous go func, switch args[0], init()
// outside register*.go, bare //nolint without reason) are refused by
// CheckAddedGoStyle on commit, while pre-existing legacy lines and compliant
// code pass.
// PREVENTS: unformatted Go files and forbidden Go patterns entering main from
// non-Claude harnesses or shell writes.
func TestCheckAddedGoStyleReproducesHistoricalViolations(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		path     string
		before   string
		after    string
		wantHit  string
		wantPass bool
	}{
		{
			name:    "2026-08-17 un-gofmt test file is refused",
			path:    "internal/component/bgp/message/rfc7606_section51_receive_test.go",
			after:   "package message\n\nfunc   TestBadFmt() {\n_ = 1\n}\n",
			wantHit: "is not gofmt-formatted",
		},
		{
			name:    "2026-08-19 un-gofmt nolint alignment in ssh.go is refused",
			path:    "internal/component/ssh/ssh.go",
			after:   "package ssh\n\nfunc run() {\n\t_ = 1   //nolint:errcheck // misaligned spaces\n}\n",
			wantHit: "is not gofmt-formatted",
		},
		{
			name:    "newly added fmt.Sprintf in internal package is refused",
			path:    "internal/component/bgp/example.go",
			before:  "package bgp\n",
			after:   "package bgp\n\nimport \"fmt\"\n\nfunc label(n int) string { return fmt.Sprintf(\"%d\", n) }\n",
			wantHit: "newly added fmt.Sprintf",
		},
		{
			name:    "newly added strconv.FormatInt is refused",
			path:    "internal/component/bgp/example.go",
			before:  "package bgp\n",
			after:   "package bgp\n\nimport \"strconv\"\n\nfunc num(n int64) string { return strconv.FormatInt(n, 10) }\n",
			wantHit: "newly added strconv.FormatInt",
		},
		{
			name:    "newly added anonymous go func is refused",
			path:    "internal/component/bgp/example.go",
			before:  "package bgp\n",
			after:   "package bgp\n\nfunc start() {\n\tgo func() {}()\n}\n",
			wantHit: "newly added anonymous goroutine",
		},
		{
			name:    "newly added switch args[0] is refused",
			path:    "internal/component/bgp/example.go",
			before:  "package bgp\n",
			after:   "package bgp\n\nfunc dispatch(args []string) {\n\tswitch args[0] {\n\tcase \"show\":\n\t}\n}\n",
			wantHit: "newly added switch args[0]",
		},
		{
			name:    "newly added init() outside register.go is refused",
			path:    "internal/component/bgp/example.go",
			before:  "package bgp\n",
			after:   "package bgp\n\nfunc init() {}\n",
			wantHit: "newly added init() outside register*.go",
		},
		{
			name:    "newly added //nolint without reason is refused",
			path:    "internal/component/bgp/example.go",
			before:  "package bgp\n",
			after:   "package bgp\n\nfunc f() {\n\t_ = 1 //nolint:gosec\n}\n",
			wantHit: "carries no reason",
		},
		{
			name:     "pre-existing legacy fmt.Sprintf line untouched by commit passes",
			path:     "internal/component/bgp/legacy.go",
			before:   "package bgp\n\nimport \"fmt\"\n\nfunc legacy(n int) string { return fmt.Sprintf(\"%d\", n) }\n",
			after:    "package bgp\n\nimport \"fmt\"\n\nfunc legacy(n int) string { return fmt.Sprintf(\"%d\", n) }\n\nfunc clean() int { return 1 }\n",
			wantPass: true,
		},
		{
			name:     "init() inside register.go and justified //nolint pass",
			path:     "internal/component/bgp/register.go",
			after:    "package bgp\n\nfunc init() {\n\t_ = 1 //nolint:gosec // bounded constant\n}\n",
			wantPass: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			change := changecheck.Change{
				Paths: []string{tc.path},
				Before: func(rel string) (string, bool, error) {
					if rel == tc.path && tc.before != "" {
						return tc.before, true, nil
					}
					return "", false, nil
				},
				After: func(rel string) (string, bool, error) {
					if rel == tc.path {
						return tc.after, true, nil
					}
					return "", false, nil
				},
			}
			got, err := CheckAddedGoStyle(change)
			if err != nil {
				t.Fatalf("CheckAddedGoStyle: %v", err)
			}
			if tc.wantPass {
				if got != "" {
					t.Fatalf("unexpected refusal:\n%s", got)
				}
				return
			}
			if !strings.Contains(got, tc.wantHit) {
				t.Fatalf("CheckAddedGoStyle = %q, want refusal mentioning %q", got, tc.wantHit)
			}
		})
	}
}

// VALIDATES: CheckAddedGoStyle is registered in changecheck so every
// `./le commit create` runs it.
func TestCheckAddedGoStyleIsRegistered(t *testing.T) {
	t.Parallel()
	registered := changecheck.Registered()
	if !slices.ContainsFunc(registered, func(c changecheck.Check) bool { return c.Name == goStyleCheckName }) {
		t.Fatalf("%q is not registered in changecheck: %+v", goStyleCheckName, registered)
	}
}

// VALIDATES: checkCrossPackageWiring covers newly added exports in pkg/ as
// well as internal/ and cmd/, flagging an uncalled pkg/ export and passing a
// wired one.
// PREVENTS: dead public API exports in pkg/ slipping past the wiring gate.
func TestCrossPackageWiringCoversPkgExports(t *testing.T) {
	t.Parallel()
	root := tree(t, map[string]string{
		"pkg/demo/demo.go":       "package demo\n\nfunc WiredExport() {}\nfunc UnwiredExport() {}\n",
		"internal/caller/use.go": "package caller\n\nimport \"github.com/ze-software/ze/pkg/demo\"\n\nfunc Use() { demo.WiredExport() }\n",
	})

	findings, err := checkCrossPackageWiring(t.Context(), root, []string{"pkg/demo/demo.go"})
	if err != nil {
		t.Fatalf("checkCrossPackageWiring: %v", err)
	}
	if len(findings) != 1 || !strings.Contains(findings[0].Message, "UnwiredExport") {
		t.Fatalf("findings = %+v, want exactly one finding for UnwiredExport", findings)
	}
}
