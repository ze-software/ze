// Design: docs/guide/appliance.md -- the kernel host name follows system host on the appliance
// Related: main_system_hostname.go -- reconcileKernelHostname

//go:build ze_appliance

package hub

import (
	"strings"
	"syscall"
	"testing"
)

// TestHostnameApplierOnlyWhenChanged drives the appliance applier over a fake
// kernel.
// VALIDATES: AC-7 -- a differing system host sets the kernel name once, and a
// later reload to another name sets it again; AC-8 -- an unset leaf, or a name
// the kernel already carries, calls no sethostname.
// PREVENTS: the appliance staying "ze" whatever system host says, or a
// sethostname on every reload.
func TestHostnameApplierOnlyWhenChanged(t *testing.T) {
	log, _ := captureLog()
	kernel := &fakeKernel{name: "ze"}

	reconcileKernelHostname("cpe-lab", true, kernel.ops(), log)
	if len(kernel.calls) != 1 || kernel.name != "cpe-lab" {
		t.Fatalf("after cpe-lab: calls %v, kernel %q; want one call and cpe-lab", kernel.calls, kernel.name)
	}

	reconcileKernelHostname("cpe-lab", true, kernel.ops(), log)
	if len(kernel.calls) != 1 {
		t.Fatalf("an unchanged name called sethostname again: %v", kernel.calls)
	}

	reconcileKernelHostname("cpe-lab2", true, kernel.ops(), log)
	if len(kernel.calls) != 2 || kernel.name != "cpe-lab2" {
		t.Fatalf("after cpe-lab2: calls %v, kernel %q", kernel.calls, kernel.name)
	}

	unset := &fakeKernel{name: "ze"}
	reconcileKernelHostname("", false, unset.ops(), log)
	if len(unset.calls) != 0 || unset.name != "ze" {
		t.Fatalf("unset system host changed the kernel: calls %v, kernel %q", unset.calls, unset.name)
	}
}

// TestHostnameApplierSetFailureKeepsNameAndWarns makes the fake sethostname
// answer EPERM.
// VALIDATES: AC-13 -- the kernel keeps its previous name, and exactly one warn
// line carries the value, the error and the fix. The applier returns nothing,
// so it has no way to refuse the reload that called it.
// PREVENTS: a failure logged below WARN, or a warn line with no fix.
func TestHostnameApplierSetFailureKeepsNameAndWarns(t *testing.T) {
	log, buf := captureLog()
	kernel := &fakeKernel{name: "ze", fail: syscall.EPERM}

	reconcileKernelHostname("cpe-lab", true, kernel.ops(), log)

	if kernel.name != "ze" {
		t.Fatalf("kernel name %q after a failed set, want ze", kernel.name)
	}
	if len(kernel.calls) != 1 {
		t.Fatalf("sethostname calls %v, want one", kernel.calls)
	}
	warns := warnLines(buf)
	if len(warns) != 1 {
		t.Fatalf("warn lines %q, want exactly one", warns)
	}
	for _, want := range []string{"cpe-lab", syscall.EPERM.Error(), "CAP_SYS_ADMIN"} {
		if !strings.Contains(warns[0], want) {
			t.Errorf("warn line %q does not carry %q", warns[0], want)
		}
	}
}
