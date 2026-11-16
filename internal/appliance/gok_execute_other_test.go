// Design: docs/architecture/appliance/builder.md -- unsupported build hosts refuse before entering gok.

//go:build !linux && !darwin

package appliance

import (
	"errors"
	"runtime"
	"strings"
	"testing"
)

type unsupportedHostHelperCompletion struct{}

func (*unsupportedHostHelperCompletion) Error() string     { return "partitioning helper completed" }
func (*unsupportedHostHelperCompletion) HelperCompletion() {}

func TestGokUnsupportedHostRefusesPacking(t *testing.T) {
	// VALIDATES: a host without gok's block-device support refuses through
	// the appliance boundary, naming the host and the supported alternative.
	// PREVENTS: treating an unsupported build as a completed image or helper.
	err := ExecuteGok(t.Context(), nil)
	if err == nil {
		t.Fatal("unsupported host accepted an image build")
	}
	for _, want := range []string{"not supported on " + runtime.GOOS, "Linux or macOS host"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal %q does not contain %q", err, want)
		}
	}
	for name, result := range map[string]error{
		"ordinary success": nil,
		"ordinary failure": errors.New("packing failed"),
		"helper marker":    &unsupportedHostHelperCompletion{},
	} {
		if IsGokHelperCompletion(result) {
			t.Errorf("unsupported host accepted %s as a completed helper", name)
		}
	}
}
