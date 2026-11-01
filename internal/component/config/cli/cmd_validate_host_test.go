package cli

import (
	"errors"
	"net"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// xfrmConfig uses interface xfrm, a list the schema carries only on Linux
// (ze:os "linux" in ze-iface-conf.yang).
const xfrmConfig = "interface {\n\txfrm xfrm0 {\n\t}\n}\n"

// TestValidateReportsNodeOfAnotherOS validates a Linux-only node with the
// schema built as another OS builds it, and as Linux builds it.
//
// VALIDATES: on an OS whose schema prunes the node, `ze config validate`
// reports config-node-other-os, a code the registry marks a host verdict, with
// a message naming the node, the OS that carries it and the fix (validate on
// that OS); on the OS that carries the node, that code never appears.
// PREVENTS: the operator on a Mac reading "unknown field in interface: xfrm"
// as a defect in a config that is valid on the router it is written for, and
// a host verdict hiding a real config error on the router itself.
func TestValidateReportsNodeOfAnotherOS(t *testing.T) {
	diagnostic.ResetForTest()
	diagnostic.RegisterBuiltinCodes()

	restore := config.SetSchemaOSForTest("darwin")
	result := runValidation(xfrmConfig, "xfrm.conf")
	restore()
	d := findDiagnostic(result, diagnostic.CodeConfigNodeOtherOS)
	if d == nil {
		t.Fatalf("validation on darwin did not report %s: %+v", diagnostic.CodeConfigNodeOtherOS, result.Diagnostics)
	}
	for _, want := range []string{"xfrm", "linux", "darwin", "ze config validate"} {
		if !strings.Contains(d.Message, want) {
			t.Errorf("message %q does not name %q", d.Message, want)
		}
	}
	if d.Line != 2 {
		t.Errorf("line = %d, want 2", d.Line)
	}
	if meta := diagnostic.Lookup(diagnostic.CodeConfigNodeOtherOS); meta == nil || !meta.HostVerdict {
		t.Errorf("%s is not registered as a host verdict", diagnostic.CodeConfigNodeOtherOS)
	}

	restore = config.SetSchemaOSForTest("linux")
	defer restore()
	result = runValidation(xfrmConfig, "xfrm.conf")
	if d := findDiagnostic(result, diagnostic.CodeConfigNodeOtherOS); d != nil {
		t.Errorf("validation on linux reported %s for a node linux carries: %s", d.Code, d.Message)
	}
}

// TestValidateReportsHostBackendVerdict feeds runValidation two refusals with
// the same text from a plugin verifier, one marked a host verdict.
//
// VALIDATES: a verifier refusal the producer marked with diagnostic.HostVerdict
// is reported as config-host-backend, a code the registry marks a host
// verdict, and the same text unmarked as config-plugin-verify: the code comes
// from the producer's mark, never from the message.
// PREVENTS: "no backend configured and no OS default available" on a Mac
// reading as a config defect, and a reader matching message text to tell.
func TestValidateReportsHostBackendVerdict(t *testing.T) {
	diagnostic.ResetForTest()
	diagnostic.RegisterBuiltinCodes()

	refusal := errors.New("interface: no backend configured and no OS default available")
	for _, tc := range []struct {
		name string
		err  error
		want string
	}{
		{"marked", diagnostic.HostVerdict(refusal), diagnostic.CodeConfigHostBackend},
		{"unmarked", refusal, "config-plugin-verify"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			snapshot := registry.Snapshot()
			registry.Reset()
			t.Cleanup(func() { registry.Restore(snapshot) })
			if err := registry.Register(registry.Registration{
				Name:                    "host-verdict-test",
				Description:             "test verifier",
				ConfigRoots:             []string{"interface"},
				RunEngine:               func(net.Conn) int { return 0 },
				CLIHandler:              func([]string) int { return 0 },
				InProcessConfigVerifier: func([]rpc.ConfigSection) error { return tc.err },
			}); err != nil {
				t.Fatalf("register: %v", err)
			}
			result := runValidation("interface {\n}\n", "interface.conf")
			d := findDiagnostic(result, tc.want)
			if d == nil {
				t.Fatalf("no %s diagnostic: %+v", tc.want, result.Diagnostics)
			}
			if !strings.Contains(d.Message, "no OS default available") {
				t.Errorf("message %q lost the verifier's text", d.Message)
			}
		})
	}
	if meta := diagnostic.Lookup(diagnostic.CodeConfigHostBackend); meta == nil || !meta.HostVerdict {
		t.Errorf("%s is not registered as a host verdict", diagnostic.CodeConfigHostBackend)
	}
}

// findDiagnostic answers the first diagnostic with code, nil when none.
func findDiagnostic(result *validationResult, code string) *diagnostic.Diagnostic {
	for i := range result.Diagnostics {
		if result.Diagnostics[i].Code == code {
			return &result.Diagnostics[i]
		}
	}
	return nil
}
