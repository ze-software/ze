//go:build linux && ze_ike

package appliance_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	_ "github.com/ze-software/ze/cmd/ze/hub"
	"github.com/ze-software/ze/internal/component/kernelcap"
)

// TestEnrolledKernelSymbolsAreInRuntimeRequire proves Ze's own runtime kernel
// declares every kernel feature Ze enrolls, so the appliance kernel passes the
// Docker-host check by construction (docker-hosts spec, AC-7).
//
// Method: the blank import of the hub, the daemon's composition root, links
// every enrolling package the daemon links under the unit-test tag set, and this
// package's own tests enroll nothing. Each enrolled CONFIG_ symbol must be a
// line of gokrazy/kernel/runtime.require, which the kernel build refuses to
// lose. The ze_ike tag is what links the IPsec enrolments the guard below asks
// for.
func TestEnrolledKernelSymbolsAreInRuntimeRequire(t *testing.T) {
	symbols := kernelcap.KernelSymbols()
	if !slices.Contains(symbols, "CONFIG_XFRM_USER") {
		t.Fatalf("the hub links no IPsec enrolment, so this test compares nothing it should: %v", symbols)
	}

	path := filepath.Join("..", "..", "gokrazy", "kernel", "runtime.require")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	declared := map[string]bool{}
	for line := range strings.SplitSeq(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "CONFIG_") {
			declared[line] = true
		}
	}

	for _, symbol := range symbols {
		if !declared[symbol] {
			t.Errorf("%s is enrolled (internal/component/kernelcap) but not declared in %s", symbol, path)
		}
	}
}
