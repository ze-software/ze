// Design: docs/architecture/ospf/ospf-4-component-config.md -- live inventory diagnostics.
package ospf

import (
	"slices"
	"strings"
	"testing"
	"testing/synctest"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// VALIDATES: the inventory doctor check reports only enrolled OSPFv2
// interfaces, and its runtime report code is described and admitted as an event.
func TestInterfaceInventoryDoctorOnlyLiveIPv4(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		installPrefixOriginationBackend(t)
		backend, ok := iface.GetBackend().(*prefixOriginationBackend)
		if !ok {
			t.Fatal("prefix origination backend was not loaded")
		}
		const code = "doctor-ospf-interface-addresses-unavailable"
		if diagnostic.Lookup(code) == nil {
			t.Fatal("inventory diagnostic code has no metadata")
		}
		if !slices.Contains(diagnostic.EventCodes(), code) {
			t.Fatal("inventory diagnostic code is not registered for report admission")
		}
		cfg, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1","areas":{"area":{"0":{"area-id":"0"}}},"interfaces":{"interface":{"lan0":{"area":"0","passive":"true"}}}}}`), nil)
		if err != nil {
			t.Fatal(err)
		}
		eng := newEngine(nil)
		defer eng.shutdown()
		eng.setConfig(cfg)
		instances := newInstanceManager(eng, nil, nil)
		backend.mu.Lock()
		backend.failName = "lan0"
		backend.mu.Unlock()
		if got := instances.interfaceAddressDoctorFindings(); len(got) != 0 {
			t.Fatalf("prestart configuration produced runtime failures: %+v", got)
		}
		if err := eng.openInterfaces(); err != nil {
			t.Fatal(err)
		}
		got := instances.interfaceAddressDoctorFindings()
		if len(got) != 1 {
			t.Fatalf("failed running IPv4 inventory findings = %+v, want one", got)
		}
		if got[0].Code != code {
			t.Errorf("doctor code = %q, want %s", got[0].Code, code)
		}
		if got[0].Severity != "warning" {
			t.Errorf("doctor severity = %q, want warning", got[0].Severity)
		}
		if got[0].Fix != "ze show interface name lan0 detail" {
			t.Errorf("doctor fix = %q", got[0].Fix)
		}
		if !strings.Contains(got[0].Message, "test interface lan0 address inventory unavailable") {
			t.Errorf("doctor omitted the inventory read cause: %q", got[0].Message)
		}
		v6 := newEngineWithCodecAF(nil, v6Codec{}, afIPv6Unicast)
		defer v6.shutdown()
		v6.setConfig(cfg)
		if err := v6.openInterfaces(); err != nil {
			t.Fatal(err)
		}
		v6Instances := newInstanceManager(v6, nil, nil)
		if got := v6Instances.interfaceAddressDoctorFindings(); len(got) != 0 {
			t.Errorf("IPv6-only runtime invented an IPv4 inventory failure: %+v", got)
		}
		backend.mu.Lock()
		backend.failName = ""
		backend.mu.Unlock()
		if got := instances.interfaceAddressDoctorFindings(); len(got) != 0 {
			t.Errorf("recovered IPv4 inventory findings = %+v", got)
		}
		backend.mu.Lock()
		backend.failName = "lan0"
		backend.mu.Unlock()
		eng.closeInterface("lan0")
		if got := instances.interfaceAddressDoctorFindings(); len(got) != 0 {
			t.Errorf("removed interface produced runtime failures: %+v", got)
		}
	})
}
